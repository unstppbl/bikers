package app

import (
	"context"
	"strings"
	"sync"
	"testing"
)

func countClubNotices(t *testing.T, f *fixture, club, user, kind string, want int) {
	t.Helper()
	var count int
	if err := f.db.QueryRow(context.Background(), "SELECT count(*) FROM notifications WHERE club_id=$1 AND user_id=$2 AND kind=$3", club, user, kind).Scan(&count); err != nil || count != want {
		t.Fatalf("club notice kind=%s count=%d want=%d err=%v", kind, count, want, err)
	}
}

func TestClubMembershipNotificationsAreScopedIdempotentAndDurable(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011234671", "Private owner name")
	applicant := f.login("+77011234672", "Private applicant name")
	outsider := f.login("+77011234673", "Outsider")
	id := testClub(t, f, owner)["id"].(string)
	base := "/api/v1/clubs/" + id
	expect(t, f.request("PUT", base+"/membership", nil, owner), 200)
	countClubNotices(t, f, id, owner.id, "club.membership.requested", 0)

	// The club transaction serializes competing retries of the same transition.
	concurrent := func(action func() response) {
		t.Helper()
		var group sync.WaitGroup
		results := make(chan response, 6)
		for range 6 {
			group.Go(func() { results <- action() })
		}
		group.Wait()
		close(results)
		for result := range results {
			expect(t, result, 200)
		}
	}
	concurrent(func() response { return f.request("PUT", base+"/membership", nil, applicant) })
	countClubNotices(t, f, id, owner.id, "club.membership.requested", 1)
	expect(t, clubDecision(f, id, outsider, applicant, "approve"), 403)
	countClubNotices(t, f, id, applicant.id, "club.membership.approved", 0)
	concurrent(func() response { return clubDecision(f, id, owner, applicant, "approve") })
	countClubNotices(t, f, id, applicant.id, "club.membership.approved", 1)
	concurrent(func() response { return clubDecision(f, id, owner, applicant, "remove") })
	countClubNotices(t, f, id, applicant.id, "club.membership.removed", 1)
	expect(t, clubDecision(f, id, owner, applicant, "reject"), 409)
	countClubNotices(t, f, id, applicant.id, "club.membership.rejected", 0)
	expect(t, f.request("PUT", base+"/membership", nil, applicant), 200)
	concurrent(func() response { return clubDecision(f, id, owner, applicant, "reject") })
	countClubNotices(t, f, id, applicant.id, "club.membership.rejected", 1)
	countClubNotices(t, f, id, owner.id, "club.membership.requested", 2)

	ownerInbox := f.request("GET", "/api/v1/me/notifications", nil, owner)
	expect(t, ownerInbox, 200)
	ownerNotice := ownerInbox.body["notifications"].([]any)[0].(map[string]any)["id"].(string)
	expect(t, f.request("PUT", "/api/v1/me/notifications/"+ownerNotice+"/read", nil, applicant), 404)
	inbox := f.request("GET", "/api/v1/me/notifications", nil, applicant)
	expect(t, inbox, 200)
	if len(inbox.body["notifications"].([]any)) != 3 || inbox.body["unread_count"] != float64(3) {
		t.Fatal("applicant did not receive exactly one notice per actual decision", inbox.raw)
	}
	for _, item := range inbox.body["notifications"].([]any) {
		notice := item.(map[string]any)
		if notice["club_id"] != id || notice["ride_id"] != nil || !strings.HasPrefix(notice["kind"].(string), "club.membership.") {
			t.Fatal("club notice has an incorrect or ambiguous target", notice)
		}
	}
	if strings.Contains(inbox.raw, "Private") || strings.Contains(inbox.raw, "City riders") || strings.Contains(inbox.raw, "Weekend") {
		t.Fatal("inbox serialized actor or club content instead of generic event metadata")
	}
	first := inbox.body["notifications"].([]any)[0].(map[string]any)["id"].(string)
	read := f.request("PUT", "/api/v1/me/notifications/"+first+"/read", nil, applicant)
	expect(t, read, 200)
	if read.body["notification"].(map[string]any)["club_id"] != id {
		t.Fatal("mark-read lost the club target")
	}
	expect(t, f.request("PUT", "/api/v1/me/notifications/read", nil, applicant), 204)
	f.startServer(f.gateway)
	inbox = f.request("GET", "/api/v1/me/notifications", nil, applicant)
	expect(t, inbox, 200)
	if inbox.body["unread_count"] != float64(0) || len(inbox.body["notifications"].([]any)) != 3 {
		t.Fatal("club inbox or read state did not survive restart", inbox.raw)
	}
	other := f.request("GET", "/api/v1/me/notifications", nil, outsider)
	expect(t, other, 200)
	if len(other.body["notifications"].([]any)) != 0 {
		t.Fatal("club events leaked into an unrelated inbox")
	}
}

func TestClubNoticeFailureRollsBackMembershipAndPushOutbox(t *testing.T) {
	f, _ := startPushFixture(t)
	owner := f.login("+77011234674", "Owner")
	applicant := f.login("+77011234675", "Applicant")
	id := testClub(t, f, owner)["id"].(string)
	base := "/api/v1/clubs/" + id
	subscribePush(t, f, owner, "club-owner-rollback")
	subscribePush(t, f, applicant, "club-applicant-rollback")
	reject := func() {
		t.Helper()
		if _, err := f.db.Exec(context.Background(), `CREATE FUNCTION reject_club_notice() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'isolated club notice failure'; END $$; CREATE TRIGGER reject_club_notice BEFORE INSERT ON notifications FOR EACH ROW EXECUTE FUNCTION reject_club_notice()`); err != nil {
			t.Fatal(err)
		}
	}
	allow := func() {
		t.Helper()
		if _, err := f.db.Exec(context.Background(), "DROP TRIGGER reject_club_notice ON notifications; DROP FUNCTION reject_club_notice()"); err != nil {
			t.Fatal(err)
		}
	}
	state := func(want string, jobs int) {
		t.Helper()
		var got string
		var queued int
		if err := f.db.QueryRow(context.Background(), "SELECT COALESCE((SELECT state FROM club_memberships WHERE club_id=$1 AND user_id=$2),'')", id, applicant.id).Scan(&got); err != nil || got != want {
			t.Fatal("notification failure partially committed membership", got, want, err)
		}
		if err := f.db.QueryRow(context.Background(), "SELECT count(*) FROM push_deliveries").Scan(&queued); err != nil || queued != jobs {
			t.Fatal("notification failure partially committed push jobs", queued, jobs, err)
		}
	}
	reject()
	expect(t, f.request("PUT", base+"/membership", nil, applicant), 503)
	state("", 0)
	allow()
	expect(t, f.request("PUT", base+"/membership", nil, applicant), 200)
	state("requested", 1)
	reject()
	expect(t, clubDecision(f, id, owner, applicant, "approve"), 503)
	state("requested", 1)
	allow()
	expect(t, clubDecision(f, id, owner, applicant, "approve"), 200)
	state("approved", 2)
	countClubNotices(t, f, id, owner.id, "club.membership.requested", 1)
	countClubNotices(t, f, id, applicant.id, "club.membership.approved", 1)
}

func TestIneligibleClubRequestsDoNotCreateAlerts(t *testing.T) {
	f, _ := startPushFixture(t)
	owner := f.login("+77011234676", "Owner")
	applicant := f.login("+77011234677", "Applicant")
	subscribePush(t, f, owner, "club-ineligible")
	id := testClub(t, f, owner)["id"].(string)
	base := "/api/v1/clubs/" + id
	expect(t, f.request("PUT", "/api/v1/me/blocks/"+owner.id, nil, applicant), 200)
	expect(t, f.request("PUT", base+"/membership", nil, applicant), 409)
	expect(t, f.request("DELETE", "/api/v1/me/blocks/"+owner.id, nil, applicant), 200)
	if _, err := f.db.Exec(context.Background(), "INSERT INTO moderation_hidden(target_type,target_id,hidden_at) VALUES('club',$1,$2)", id, f.now); err != nil {
		t.Fatal(err)
	}
	expect(t, f.request("PUT", base+"/membership", nil, applicant), 404)
	if _, err := f.db.Exec(context.Background(), "DELETE FROM moderation_hidden WHERE target_type='club' AND target_id=$1", id); err != nil {
		t.Fatal(err)
	}
	expect(t, f.request("POST", base+"/close", map[string]any{"version": 1}, owner), 200)
	expect(t, f.request("PUT", base+"/membership", nil, applicant), 409)
	countClubNotices(t, f, id, owner.id, "club.membership.requested", 0)
	var jobs int
	if err := f.db.QueryRow(context.Background(), "SELECT count(*) FROM push_deliveries").Scan(&jobs); err != nil || jobs != 0 {
		t.Fatal("failed club requests queued a push", jobs, err)
	}
}

func TestClubPushRechecksOwnerTransferAndTargetsTheCurrentOwner(t *testing.T) {
	f, gateway := startPushFixture(t)
	owner := f.login("+77011234678", "Original owner")
	successor := f.login("+77011234679", "New owner")
	applicant := f.login("+77011234680", "Applicant")
	id := testClub(t, f, owner)["id"].(string)
	base := "/api/v1/clubs/" + id
	expect(t, f.request("PUT", base+"/membership", nil, successor), 200)
	expect(t, clubDecision(f, id, owner, successor, "approve"), 200)
	subscribePush(t, f, owner, "club-old-owner")
	subscribePush(t, f, successor, "club-new-owner")
	expect(t, f.request("PUT", base+"/membership", nil, applicant), 200)
	expect(t, f.request("PUT", base+"/ownership", map[string]any{"version": 1, "user_id": successor.id}, owner), 200)
	if err := f.s.Notifications.Dispatch(context.Background()); err != nil || len(gateway.payloads) != 0 {
		t.Fatal("previous owner received a queued request after transfer", err)
	}
	expect(t, f.request("DELETE", base+"/membership", nil, applicant), 200)
	expect(t, f.request("PUT", base+"/membership", nil, applicant), 200)
	countClubNotices(t, f, id, successor.id, "club.membership.requested", 1)
	if err := f.s.Notifications.Dispatch(context.Background()); err != nil || len(gateway.payloads) != 1 {
		t.Fatal("new owner did not receive a new request", err)
	}
	var recipient, kind string
	if err := f.db.QueryRow(context.Background(), "SELECT user_id::text,kind FROM notifications WHERE id=$1", gateway.payloads[0].NotificationID).Scan(&recipient, &kind); err != nil || recipient != successor.id || kind != "club.membership.requested" {
		t.Fatal("request push was routed to the wrong club owner", recipient, kind, err)
	}
}

func TestClubPushSkipsStaleApprovalsButDeliversGenericMembershipOutcomes(t *testing.T) {
	f, gateway := startPushFixture(t)
	owner := f.login("+77011234681", "Private owner")
	applicant := f.login("+77011234682", "Private applicant")
	id := testClub(t, f, owner)["id"].(string)
	base := "/api/v1/clubs/" + id
	expect(t, f.request("PUT", base+"/membership", nil, applicant), 200)
	subscribePush(t, f, applicant, "club-outcomes")
	expect(t, f.request("PATCH", "/api/v1/me/preferences", map[string]any{"locale": "kk"}, applicant), 200)
	expect(t, clubDecision(f, id, owner, applicant, "approve"), 200)
	expect(t, f.request("DELETE", base+"/membership", nil, applicant), 200)
	if err := f.s.Notifications.Dispatch(context.Background()); err != nil || len(gateway.payloads) != 0 {
		t.Fatal("former member received a stale approval push", err)
	}
	expect(t, f.request("PUT", base+"/membership", nil, applicant), 200)
	expect(t, clubDecision(f, id, owner, applicant, "reject"), 200)
	if err := f.s.Notifications.Dispatch(context.Background()); err != nil || len(gateway.payloads) != 1 {
		t.Fatal("rejected applicant lost their outcome notice", err)
	}
	expect(t, f.request("PUT", base+"/membership", nil, applicant), 200)
	expect(t, clubDecision(f, id, owner, applicant, "approve"), 200)
	expect(t, clubDecision(f, id, owner, applicant, "remove"), 200)
	if err := f.s.Notifications.Dispatch(context.Background()); err != nil || len(gateway.payloads) != 2 {
		t.Fatal("removed member lost their outcome or received a stale approval", err)
	}
	for _, payload := range gateway.payloads {
		if payload.Title != "BIKERS.KZ" || payload.Body != "Сізде жаңа жаңарту бар" || payload.URL != "/inbox" {
			t.Fatal("club push exposed content or changed the safe inbox destination", payload)
		}
	}
}

func TestClubRequestPushSkipsRevokedSessionsAndRestrictedAccounts(t *testing.T) {
	for _, revoke := range []bool{true, false} {
		name := "restricted account"
		if revoke {
			name = "revoked session"
		}
		t.Run(name, func(t *testing.T) {
			f, gateway := startPushFixture(t)
			owner := f.login("+77011234683", "Owner")
			applicant := f.login("+77011234684", "Applicant")
			id := testClub(t, f, owner)["id"].(string)
			subscribePush(t, f, owner, "club-revoked")
			expect(t, f.request("PUT", "/api/v1/clubs/"+id+"/membership", nil, applicant), 200)
			if revoke {
				expect(t, f.request("DELETE", "/api/v1/session", nil, owner), 200)
			} else if _, err := f.db.Exec(context.Background(), "UPDATE users SET status='suspended' WHERE id=$1", owner.id); err != nil {
				t.Fatal(err)
			}
			if err := f.s.Notifications.Dispatch(context.Background()); err != nil || len(gateway.payloads) != 0 {
				t.Fatal("ineligible club owner received push", err)
			}
		})
	}
}
