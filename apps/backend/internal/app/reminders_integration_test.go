package app

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"bikers.kz/backend/internal/database/postgres"
	"bikers.kz/backend/internal/managers/reminders"
	"github.com/google/uuid"
)

func reminderPlan(t *testing.T, f *fixture, owner *actor, ride string, starts time.Time) {
	t.Helper()
	current := f.request("GET", "/api/v1/rides/"+ride, nil, owner)
	expect(t, current, 200)
	expect(t, f.request("PATCH", "/api/v1/rides/"+ride, map[string]any{"plan_version": current.body["ride"].(map[string]any)["plan_version"], "starts_at": starts.Format(time.RFC3339)}, owner), 200)
}
func reminderCount(t *testing.T, f *fixture, ride string) int {
	t.Helper()
	var n int
	if err := f.db.QueryRow(context.Background(), "SELECT count(*) FROM notifications WHERE ride_id=$1 AND kind='ride.reminder'", ride).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
func TestPushCategoryPreferencesMuteQueuedDeliveryWithoutLosingInbox(t *testing.T) {
	f, g := startPushFixture(t)
	owner := f.login("+77011234567", "Owner")
	rider := f.login("+77021234567", "Rider")
	ride := f.ride(owner, 2)
	f.join(ride, rider)
	expect(t, f.decision(ride, owner, rider, "approve"), 200)
	subscribePush(t, f, rider, "category")
	expect(t, f.request("POST", "/api/v1/rides/"+ride+"/announcements", map[string]any{"client_announcement_id": uuid.NewString(), "body": "PRIVATE ANNOUNCEMENT"}, owner), 201)
	expect(t, f.request("PATCH", "/api/v1/me/preferences", map[string]any{"push_categories": map[string]bool{"ride_updates": false}}, rider), 200)
	if err := f.s.Notifications.Dispatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(g.payloads) != 0 {
		t.Fatal("muted queued category was sent")
	}
	var state string
	if err := f.db.QueryRow(context.Background(), "SELECT d.state FROM push_deliveries d JOIN notifications n ON n.id=d.notification_id WHERE n.kind='ride.announcement'").Scan(&state); err != nil || state != "skipped" {
		t.Fatal("muted queue was not skipped", state, err)
	}
	preferences := f.request("GET", "/api/v1/me/preferences", nil, rider)
	expect(t, preferences, 200)
	categories := preferences.body["push_categories"].(map[string]any)
	if categories["ride_updates"] != false || categories["participation"] != true || categories["club_membership"] != true || categories["ride_reminders"] != true {
		t.Fatal("sparse category patch changed unrelated defaults")
	}
	inbox := f.request("GET", "/api/v1/me/notifications", nil, rider)
	expect(t, inbox, 200)
	if len(inbox.body["notifications"].([]any)) != 2 {
		t.Fatal("muted category removed inbox records")
	}
	count := f.request("GET", "/api/v1/me/notifications/unread-count", nil, rider)
	expect(t, count, 200)
	if count.body["unread_count"] != inbox.body["unread_count"] || len(count.body) != 1 {
		t.Fatal("lightweight count differs from inbox")
	}
	expect(t, f.request("GET", "/api/v1/me/notifications/unread-count", nil, nil), 401)
	expect(t, f.request("PUT", "/api/v1/me/notifications/read", nil, rider), 204)
	count = f.request("GET", "/api/v1/me/notifications/unread-count", nil, rider)
	expect(t, count, 200)
	if count.body["unread_count"] != float64(0) {
		t.Fatal("read-all left stale unread count")
	}
	expect(t, f.request("PATCH", "/api/v1/me/preferences", map[string]any{"push_categories": map[string]any{"ride_updates": "yes"}}, rider), 400)
}
func TestRideReminderConsentDueDeliveryDeduplicatesAcrossConcurrentPollsAndRestart(t *testing.T) {
	f, g := startPushFixture(t)
	owner := f.login("+77011234567", "Owner")
	rider := f.login("+77021234567", "Rider")
	pending := f.login("+77051234567", "Pending")
	ride := f.ride(owner, 3)
	starts := f.now.Add(25 * time.Hour)
	reminderPlan(t, f, owner, ride, starts)
	f.join(ride, rider)
	f.join(ride, pending)
	expect(t, f.decision(ride, owner, rider, "approve"), 200)
	path := "/api/v1/rides/" + ride + "/reminders"
	expect(t, f.request("GET", path, nil, nil), 401)
	expect(t, f.request("GET", path, nil, pending), 403)
	current := f.request("GET", path, nil, rider)
	expect(t, current, 200)
	if current.body["day_before"] != false || current.body["hour_before"] != false {
		t.Fatal("rider implicitly opted into reminders")
	}
	missingCSRF := *rider
	missingCSRF.csrf = ""
	choice := map[string]bool{"day_before": true, "hour_before": true}
	expect(t, f.request("PUT", path, choice, &missingCSRF), 403)
	expect(t, f.request("PUT", path, map[string]bool{"hour_before": true}, rider), 422)
	expect(t, f.request("PUT", path, choice, rider), 200)
	subscribePush(t, f, rider, "departure")
	if err := f.s.Reminders.Dispatch(context.Background()); err != nil || reminderCount(t, f, ride) != 0 {
		t.Fatal("reminder generated before its due time", err)
	}
	f.now = starts.Add(-24 * time.Hour)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- f.s.Reminders.Dispatch(context.Background()) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	restarted := reminders.New(postgres.New(f.db), func() time.Time { return f.now })
	if err := restarted.Dispatch(context.Background()); err != nil || reminderCount(t, f, ride) != 1 {
		t.Fatal("concurrent/restarted worker duplicated occurrence", err)
	}
	if err := f.s.Notifications.Dispatch(context.Background()); err != nil || len(g.payloads) != 1 {
		t.Fatal("due day reminder not dispatched", err)
	}
	f.now = starts.Add(-time.Hour)
	if err := restarted.Dispatch(context.Background()); err != nil || reminderCount(t, f, ride) != 2 {
		t.Fatal("hour reminder not generated independently", err)
	}
	if err := f.s.Notifications.Dispatch(context.Background()); err != nil || len(g.payloads) != 2 {
		t.Fatal("due hour reminder not dispatched", err)
	}
	if g.payloads[0].NotificationID == g.payloads[1].NotificationID {
		t.Fatal("different reminder occurrences shared push tag")
	}
	inbox := f.request("GET", "/api/v1/me/notifications", nil, rider)
	expect(t, inbox, 200)
	if strings.Contains(inbox.raw, "PRIVATE") || strings.Contains(inbox.raw, "SECRET") || strings.Contains(inbox.raw, "reminder_starts_at") {
		t.Fatal("inbox reminder leaked private plan or schedule metadata")
	}
	var events int
	if err := f.db.QueryRow(context.Background(), "SELECT count(*) FROM ride_reminder_events WHERE ride_id=$1", ride).Scan(&events); err != nil || events != 2 {
		t.Fatal("occurrence receipts not durable", events, err)
	}
}
func TestReminderQueuedPushRechecksOptOutCancellationRescheduleAccessAndSession(t *testing.T) {
	for _, scenario := range []string{"opt-out", "cancel", "reschedule", "remove", "reapprove", "logout", "category", "hidden", "blocked", "restricted", "expired-window"} {
		t.Run(scenario, func(t *testing.T) {
			f, g := startPushFixture(t)
			owner := f.login("+77011234567", "Owner")
			rider := f.login("+77021234567", "Rider")
			ride := f.ride(owner, 2)
			starts := f.now.Add(2 * time.Hour)
			reminderPlan(t, f, owner, ride, starts)
			f.join(ride, rider)
			expect(t, f.decision(ride, owner, rider, "approve"), 200)
			path := "/api/v1/rides/" + ride + "/reminders"
			expect(t, f.request("PUT", path, map[string]bool{"day_before": false, "hour_before": true}, rider), 200)
			subscribePush(t, f, rider, scenario)
			f.now = starts.Add(-time.Hour)
			if err := f.s.Reminders.Dispatch(context.Background()); err != nil || reminderCount(t, f, ride) != 1 {
				t.Fatal("reminder not queued", err)
			}
			var noticeID string
			if err := f.db.QueryRow(context.Background(), "SELECT id::text FROM notifications WHERE ride_id=$1 AND kind='ride.reminder'", ride).Scan(&noticeID); err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "opt-out":
				expect(t, f.request("PUT", path, map[string]bool{"day_before": false, "hour_before": false}, rider), 200)
			case "cancel":
				current := f.request("GET", "/api/v1/rides/"+ride, nil, owner)
				expect(t, current, 200)
				expect(t, f.request("POST", "/api/v1/rides/"+ride+"/cancel", map[string]any{"plan_version": current.body["ride"].(map[string]any)["plan_version"]}, owner), 200)
			case "reschedule":
				reminderPlan(t, f, owner, ride, starts.Add(24*time.Hour))
			case "remove":
				expect(t, f.decision(ride, owner, rider, "remove"), 200)
			case "reapprove":
				expect(t, f.request("DELETE", "/api/v1/rides/"+ride+"/participation", nil, rider), 200)
				f.join(ride, rider)
				expect(t, f.decision(ride, owner, rider, "approve"), 200)
				current := f.request("GET", path, nil, rider)
				expect(t, current, 200)
				if current.body["hour_before"] != false {
					t.Fatal("reapproved participant inherited previous opt-in")
				}
			case "logout":
				expect(t, f.request("DELETE", "/api/v1/session", nil, rider), 200)
			case "category":
				expect(t, f.request("PATCH", "/api/v1/me/preferences", map[string]any{"push_categories": map[string]bool{"ride_reminders": false}}, rider), 200)
			case "hidden":
				if _, err := f.db.Exec(context.Background(), "INSERT INTO moderation_hidden(target_type,target_id,hidden_at) VALUES('ride',$1,now())", ride); err != nil {
					t.Fatal(err)
				}
			case "blocked":
				if _, err := f.db.Exec(context.Background(), "INSERT INTO user_blocks(blocker_id,blocked_id,created_at) VALUES($1,$2,now())", owner.id, rider.id); err != nil {
					t.Fatal(err)
				}
			case "restricted":
				if _, err := f.db.Exec(context.Background(), "UPDATE users SET status='suspended' WHERE id=$1", rider.id); err != nil {
					t.Fatal(err)
				}
			case "expired-window":
				f.now = f.now.Add(11 * time.Minute)
			}
			if err := f.s.Notifications.Dispatch(context.Background()); err != nil {
				t.Fatal(err)
			}
			for _, payload := range g.payloads {
				if payload.NotificationID == noticeID {
					t.Fatal("stale reminder reached push provider")
				}
			}
			var state string
			if err := f.db.QueryRow(context.Background(), "SELECT state FROM push_deliveries WHERE notification_id=$1", noticeID).Scan(&state); err != nil || state != "skipped" {
				t.Fatal("stale reminder not skipped", state, err)
			}
			if reminderCount(t, f, ride) != 1 {
				t.Fatal("dispatch removed durable reminder inbox entry")
			}
		})
	}
}
func TestReminderWorkerDoesNotCatchUpMissedWindowOrLateConsent(t *testing.T) {
	for _, lateConsent := range []bool{false, true} {
		t.Run(map[bool]string{false: "missed window", true: "late consent"}[lateConsent], func(t *testing.T) {
			f := newFixture(t)
			owner := f.login("+77011234567", "Owner")
			ride := f.ride(owner, 1)
			starts := f.now.Add(2 * time.Hour)
			reminderPlan(t, f, owner, ride, starts)
			path := "/api/v1/rides/" + ride + "/reminders"
			if lateConsent {
				f.now = starts.Add(-time.Hour + time.Minute)
			}
			expect(t, f.request("PUT", path, map[string]bool{"day_before": false, "hour_before": true}, owner), 200)
			if !lateConsent {
				f.now = starts.Add(-time.Hour + 11*time.Minute)
			}
			if err := f.s.Reminders.Dispatch(context.Background()); err != nil || reminderCount(t, f, ride) != 0 {
				t.Fatal("late reminder was generated", err)
			}
		})
	}
}

func TestReminderConsentTimestampsAreIndependentForEachLead(t *testing.T) {
	for _, scenario := range []string{"change-other-lead", "toggle-same-lead", "new-participation"} {
		t.Run(scenario, func(t *testing.T) {
			f, g := startPushFixture(t)
			owner := f.login("+77011234567", "Owner")
			rider := f.login("+77021234567", "Rider")
			ride := f.ride(owner, 2)
			starts := f.now.Add(2 * time.Hour)
			reminderPlan(t, f, owner, ride, starts)
			f.join(ride, rider)
			expect(t, f.decision(ride, owner, rider, "approve"), 200)
			path := "/api/v1/rides/" + ride + "/reminders"
			expect(t, f.request("PUT", path, map[string]bool{"day_before": false, "hour_before": true}, rider), 200)
			subscribePush(t, f, rider, scenario)
			var originalHour time.Time
			if err := f.db.QueryRow(context.Background(), "SELECT hour_before_enabled_at FROM ride_reminders WHERE ride_id=$1 AND user_id=$2", ride, rider.id).Scan(&originalHour); err != nil {
				t.Fatal(err)
			}
			f.now = starts.Add(-time.Hour)
			if err := f.s.Reminders.Dispatch(context.Background()); err != nil {
				t.Fatal(err)
			}
			var noticeID string
			if err := f.db.QueryRow(context.Background(), "SELECT id::text FROM notifications WHERE ride_id=$1 AND kind='ride.reminder'", ride).Scan(&noticeID); err != nil {
				t.Fatal(err)
			}
			f.now = f.now.Add(time.Minute)
			switch scenario {
			case "change-other-lead":
				expect(t, f.request("PUT", path, map[string]bool{"day_before": true, "hour_before": true}, rider), 200)
			case "toggle-same-lead":
				expect(t, f.request("PUT", path, map[string]bool{"day_before": false, "hour_before": false}, rider), 200)
				var cleared *time.Time
				if err := f.db.QueryRow(context.Background(), "SELECT hour_before_enabled_at FROM ride_reminders WHERE ride_id=$1 AND user_id=$2", ride, rider.id).Scan(&cleared); err != nil || cleared != nil {
					t.Fatal("opt-out retained lead consent timestamp", err)
				}
				expect(t, f.request("PUT", path, map[string]bool{"day_before": false, "hour_before": true}, rider), 200)
			case "new-participation":
				expect(t, f.request("DELETE", "/api/v1/rides/"+ride+"/participation", nil, rider), 200)
				f.join(ride, rider)
				expect(t, f.decision(ride, owner, rider, "approve"), 200)
				expect(t, f.request("PUT", path, map[string]bool{"day_before": false, "hour_before": true}, rider), 200)
			}
			var afterHour time.Time
			if err := f.db.QueryRow(context.Background(), "SELECT hour_before_enabled_at FROM ride_reminders WHERE ride_id=$1 AND user_id=$2", ride, rider.id).Scan(&afterHour); err != nil {
				t.Fatal(err)
			}
			wantDelivery := scenario == "change-other-lead"
			if wantDelivery && !afterHour.Equal(originalHour) {
				t.Fatal("changing day preference renewed unchanged hour consent")
			}
			if !wantDelivery && !afterHour.Equal(f.now) {
				t.Fatal("new hour opt-in did not capture its own consent time")
			}
			if err := f.s.Notifications.Dispatch(context.Background()); err != nil {
				t.Fatal(err)
			}
			delivered := false
			for _, payload := range g.payloads {
				if payload.NotificationID == noticeID {
					delivered = true
				}
			}
			if delivered != wantDelivery {
				t.Fatalf("queued hour reminder delivery=%v, want %v", delivered, wantDelivery)
			}
		})
	}
}
