package app

import (
	"context"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"bikers.kz/backend/internal/database/postgres"
	"bikers.kz/backend/internal/managers/notifications"
	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
)

func TestPreferencesAreIndependentValidatedDurableAndPrivate(t *testing.T) {
	f := newFixture(t)
	a := f.login("+77011234567", "Rider")
	expect(t, f.request("GET", "/api/v1/me/preferences", nil, nil), 401)
	for _, patch := range []map[string]any{{"country_code": "KG"}, {"locale": "kk"}, {"currency_code": "EUR"}} {
		expect(t, f.request("PATCH", "/api/v1/me/preferences", patch, a), 200)
	}
	r := f.request("GET", "/api/v1/me/preferences", nil, a)
	expect(t, r, 200)
	if r.body["country_code"] != "KG" || r.body["currency_code"] != "EUR" || r.body["locale"] != "kk" {
		t.Fatalf("settings coupled unexpectedly: %s", r.raw)
	}
	bad := *a
	bad.csrf = ""
	expect(t, f.request("PATCH", "/api/v1/me/preferences", map[string]any{"country_code": "KZ"}, &bad), 403)
	expect(t, f.request("PATCH", "/api/v1/me/preferences", map[string]any{"currency_code": "BTC", "country_code": "KZ"}, a), 422)
	f.startServer(f.gateway)
	r = f.request("GET", "/api/v1/me/preferences", nil, a)
	expect(t, r, 200)
	if r.body["country_code"] != "KG" || r.body["currency_code"] != "EUR" || r.body["locale"] != "kk" {
		t.Fatal("failed patch or restart changed preferences")
	}
	session := f.request("GET", "/api/v1/session", nil, a)
	if session.body["user"].(map[string]any)["profile"].(map[string]any)["locale"] != "kk" {
		t.Fatal("language preference diverged from session locale")
	}
}

func TestAnnouncementsAndInboxAreAtomicIdempotentAndAccessScoped(t *testing.T) {
	f := newFixture(t)
	o := f.login("+77011234567", "Organizer")
	a := f.login("+77021234567", "Approved rider")
	pending := f.login("+77051234567", "Pending rider")
	id := f.ride(o, 3)
	f.join(id, a)
	f.join(id, pending)
	expect(t, f.decision(id, o, a, "approve"), 200)
	path := "/api/v1/rides/" + id + "/announcements"
	expect(t, f.request("GET", path, nil, nil), 401)
	expect(t, f.request("GET", path, nil, pending), 403)
	expect(t, f.request("POST", path, map[string]any{"client_announcement_id": uuid.NewString(), "body": "Not an organizer"}, a), 403)
	input := map[string]any{"client_announcement_id": uuid.NewString(), "body": "PRIVATE ANNOUNCEMENT: arrive at SECRET MEETING"}
	first := f.request("POST", path, input, o)
	expect(t, first, 201)
	retry := f.request("POST", path, input, o)
	expect(t, retry, 200)
	if retry.body["announcement"].(map[string]any)["id"] != first.body["announcement"].(map[string]any)["id"] {
		t.Fatal("announcement retry changed identity")
	}
	input["body"] = "Changed retry"
	expect(t, f.request("POST", path, input, o), 409)
	inbox := f.request("GET", "/api/v1/me/notifications", nil, a)
	expect(t, inbox, 200)
	rows := inbox.body["notifications"].([]any)
	if len(rows) != 2 || inbox.body["unread_count"] != float64(2) {
		t.Fatalf("inbox should contain approval and one announcement: %s", inbox.raw)
	}
	if strings.Contains(inbox.raw, "PRIVATE") || strings.Contains(inbox.raw, "SECRET") || strings.Contains(inbox.raw, "Organizer") {
		t.Fatal("inbox persisted private plan, author or announcement text")
	}
	noticeID := rows[0].(map[string]any)["id"].(string)
	readPath := "/api/v1/me/notifications/" + noticeID + "/read"
	expect(t, f.request("PUT", readPath, nil, pending), 404)
	read := f.request("PUT", readPath, nil, a)
	expect(t, read, 200)
	f.now = f.now.Add(time.Minute)
	repeatedRead := f.request("PUT", readPath, nil, a)
	expect(t, repeatedRead, 200)
	if read.body["notification"].(map[string]any)["read_at"] != repeatedRead.body["notification"].(map[string]any)["read_at"] {
		t.Fatal("marking an already-read notice changed its read timestamp")
	}
	// A failed notice write must also roll back its announcement.
	if _, err := f.db.Exec(context.Background(), `CREATE FUNCTION reject_test_notice() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'isolated rollback'; END $$; CREATE TRIGGER reject_notice BEFORE INSERT ON notifications FOR EACH ROW EXECUTE FUNCTION reject_test_notice()`); err != nil {
		t.Fatal(err)
	}
	expect(t, f.request("POST", path, map[string]any{"client_announcement_id": uuid.NewString(), "body": "Must roll back"}, o), 503)
	if _, err := f.db.Exec(context.Background(), "DROP TRIGGER reject_notice ON notifications; DROP FUNCTION reject_test_notice()"); err != nil {
		t.Fatal(err)
	}
	f.startServer(f.gateway)
	announcements := f.request("GET", path, nil, a)
	expect(t, announcements, 200)
	if len(announcements.body["announcements"].([]any)) != 1 {
		t.Fatalf("retry or rollback duplicated announcement: %s", announcements.raw)
	}
	expect(t, f.request("PUT", "/api/v1/me/notifications/read", nil, a), 204)
	inbox = f.request("GET", "/api/v1/me/notifications", nil, a)
	if inbox.body["unread_count"] != float64(0) {
		t.Fatal("read-all did not persist")
	}
	expect(t, f.request("POST", "/api/v1/rides/"+id+"/cancel", map[string]any{"plan_version": 1}, o), 200)
	expect(t, f.request("GET", path, nil, a), 200)
	expect(t, f.request("POST", path, map[string]any{"client_announcement_id": uuid.NewString(), "body": "After cancellation"}, o), 409)
	expect(t, f.decision(id, o, a, "remove"), 200)
	expect(t, f.request("GET", path, nil, a), 403)
}

func TestAnnouncementAndInboxPaginationPreserveOrderingWithoutOverlap(t *testing.T) {
	f := newFixture(t)
	o := f.login("+77011234567", "Organizer")
	a := f.login("+77021234567", "Rider")
	id := f.ride(o, 2)
	f.join(id, a)
	expect(t, f.decision(id, o, a, "approve"), 200)
	path := "/api/v1/rides/" + id + "/announcements"
	for i := range 52 {
		expect(t, f.request("POST", path, map[string]any{"client_announcement_id": uuid.NewString(), "body": fmt.Sprintf("Announcement %d", i)}, o), 201)
	}
	for _, endpoint := range []string{path, "/api/v1/me/notifications"} {
		first := f.request("GET", endpoint, nil, a)
		expect(t, first, 200)
		key := "announcements"
		wantRemainder := 2
		if strings.Contains(endpoint, "notifications") {
			key, wantRemainder = "notifications", 3
		}
		rows := first.body[key].([]any)
		cursor := first.body["next_cursor"].(string)
		if len(rows) != 50 || cursor == "" {
			t.Fatalf("bounded first page: %s", first.raw)
		}
		second := f.request("GET", endpoint+"?before="+cursor, nil, a)
		expect(t, second, 200)
		remaining := second.body[key].([]any)
		if len(remaining) != wantRemainder || second.body["next_cursor"] != "" {
			t.Fatalf("incorrect remaining page: %s", second.raw)
		}
		seen := map[string]bool{}
		for _, row := range append(rows, remaining...) {
			id := row.(map[string]any)["id"].(string)
			if seen[id] {
				t.Fatal("cursor pages overlap")
			}
			seen[id] = true
		}
		expect(t, f.request("GET", endpoint+"?before=broken", nil, a), 422)
	}
}

func TestRideTransitionsNotifyOnlyTheirAudienceAndCommitWithThePlan(t *testing.T) {
	f := newFixture(t)
	o := f.login("+77011234831", "Organizer")
	a := f.login("+77011234832", "Approved rider")
	pending := f.login("+77011234833", "Pending rider")
	id := f.ride(o, 3)
	f.join(id, a)
	f.join(id, a)
	f.join(id, pending)
	expect(t, f.decision(id, o, a, "approve"), 200)
	expect(t, f.decision(id, o, a, "approve"), 200)
	expect(t, f.decision(id, o, pending, "reject"), 200)
	countKind := func(user, kind string, want int) {
		t.Helper()
		var n int
		if err := f.db.QueryRow(context.Background(), "SELECT count(*) FROM notifications WHERE user_id=$1 AND kind=$2", user, kind).Scan(&n); err != nil || n != want {
			t.Fatalf("user=%s kind=%s count=%d want=%d err=%v", user, kind, n, want, err)
		}
	}
	countKind(o.id, "participation.requested", 2)
	countKind(a.id, "participation.approved", 1)
	countKind(pending.id, "participation.rejected", 1)
	path := "/api/v1/rides/" + id
	expect(t, f.request("PATCH", path, map[string]any{"plan_version": 1, "title": "Cosmetic title"}, o), 200)
	countKind(a.id, "ride.updated", 0)
	expect(t, f.request("PATCH", path, map[string]any{"plan_version": 2, "meeting_point": "Changed meeting point"}, o), 200)
	expect(t, f.request("PATCH", path, map[string]any{"plan_version": 2, "meeting_point": "Stale mutation"}, o), 409)
	countKind(a.id, "ride.updated", 1)
	countKind(o.id, "ride.updated", 0)
	countKind(pending.id, "ride.updated", 0)
	if _, err := f.db.Exec(context.Background(), `CREATE FUNCTION reject_test_plan_notice() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'isolated rollback'; END $$; CREATE TRIGGER reject_notice BEFORE INSERT ON notifications FOR EACH ROW EXECUTE FUNCTION reject_test_plan_notice()`); err != nil {
		t.Fatal(err)
	}
	expect(t, f.request("PATCH", path, map[string]any{"plan_version": 3, "meeting_point": "Must roll back with notice"}, o), 503)
	if _, err := f.db.Exec(context.Background(), "DROP TRIGGER reject_notice ON notifications; DROP FUNCTION reject_test_plan_notice()"); err != nil {
		t.Fatal(err)
	}
	current := f.request("GET", path, nil, a)
	expect(t, current, 200)
	plan := current.body["ride"].(map[string]any)
	if plan["plan_version"] != float64(3) || plan["meeting_point"] != "Changed meeting point" {
		t.Fatal("failed outbox write committed a partial plan change")
	}
	expect(t, f.request("POST", path+"/cancel", map[string]any{"plan_version": 3}, o), 200)
	expect(t, f.request("POST", path+"/cancel", map[string]any{"plan_version": 4}, o), 200)
	countKind(a.id, "ride.cancelled", 1)
	countKind(o.id, "ride.cancelled", 0)
	countKind(pending.id, "ride.cancelled", 0)
}

type fakePushGateway struct {
	mu       sync.Mutex
	statuses []int
	payloads []models.PushPayload
}

func (g *fakePushGateway) Send(_ context.Context, _ models.PushSubscriptionInput, payload models.PushPayload) (int, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.payloads = append(g.payloads, payload)
	status := 201
	if len(g.statuses) > 0 {
		status, g.statuses = g.statuses[0], g.statuses[1:]
	}
	return status, nil
}

func startPushFixture(t *testing.T) (*fixture, *fakePushGateway) {
	t.Helper()
	f := newFixture(t)
	private, x, y, err := elliptic.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	f.cfg.PushPublicKey = base64.RawURLEncoding.EncodeToString(elliptic.Marshal(elliptic.P256(), x, y))
	f.cfg.PushPrivateKey = base64.RawURLEncoding.EncodeToString(private)
	f.cfg.PushSubject = "mailto:isolated@example.com"
	g := &fakePushGateway{}
	f.ts.Close()
	ts := httptest.NewUnstartedServer(nil)
	f.cfg.BaseURL = "http://" + ts.Listener.Addr().String()
	runtime, err := New(f.cfg, postgres.New(f.db), f.gateway, Options{Now: func() time.Time { return f.now }, PushGateway: g})
	if err != nil {
		t.Fatal(err)
	}
	f.s, f.ts = runtime, ts
	ts.Config.Handler = runtime.Handler
	ts.Start()
	t.Cleanup(ts.Close)
	return f, g
}

func pushInput(t *testing.T, name string) models.PushSubscriptionInput {
	t.Helper()
	_, x, y, err := elliptic.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return models.PushSubscriptionInput{Endpoint: "https://fcm.googleapis.com/fcm/send/isolated-" + name, Keys: models.PushKeys{P256dh: base64.RawURLEncoding.EncodeToString(elliptic.Marshal(elliptic.P256(), x, y)), Auth: base64.RawURLEncoding.EncodeToString(make([]byte, 16))}}
}

func subscribePush(t *testing.T, f *fixture, a *actor, name string) string {
	t.Helper()
	r := f.request("POST", "/api/v1/me/push/subscriptions", pushInput(t, name), a)
	expect(t, r, 201)
	return r.body["subscription"].(map[string]any)["id"].(string)
}

func TestPushConsentRetryGenericPayloadAndDeadSubscription(t *testing.T) {
	f, g := startPushFixture(t)
	o := f.login("+77011234567", "Organizer")
	a := f.login("+77021234567", "Rider")
	id := f.ride(o, 2)
	f.join(id, a)
	expect(t, f.decision(id, o, a, "approve"), 200)
	var count int
	if err := f.db.QueryRow(context.Background(), "SELECT count(*) FROM push_deliveries").Scan(&count); err != nil || count != 0 {
		t.Fatal("notifications queued push without browser subscription", err)
	}
	subscribePush(t, f, a, "retry")
	expect(t, f.request("PATCH", "/api/v1/me/preferences", map[string]any{"locale": "kk"}, a), 200)
	listed := f.request("GET", "/api/v1/me/push/subscriptions", nil, a)
	expect(t, listed, 200)
	if strings.Contains(listed.raw, "endpoint") || strings.Contains(listed.raw, "p256dh") || strings.Contains(listed.raw, "session_digest") {
		t.Fatal("device listing exposed browser secrets")
	}
	path := "/api/v1/rides/" + id + "/announcements"
	expect(t, f.request("POST", path, map[string]any{"client_announcement_id": uuid.NewString(), "body": "SECRET meeting, PRIVATE rider names"}, o), 201)
	g.statuses = []int{503, 201}
	if err := f.s.Notifications.Dispatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	var state string
	var attempts int
	if err := f.db.QueryRow(context.Background(), "SELECT state,attempts FROM push_deliveries").Scan(&state, &attempts); err != nil || state != "pending" || attempts != 1 || len(g.payloads) != 1 {
		t.Fatalf("first transient failure state=%s attempts=%d calls=%d err=%v", state, attempts, len(g.payloads), err)
	}
	if err := f.s.Notifications.Dispatch(context.Background()); err != nil || len(g.payloads) != 1 {
		t.Fatal("retried before due time", err)
	}
	f.now = f.now.Add(11 * time.Second)
	if err := f.s.Notifications.Dispatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := f.db.QueryRow(context.Background(), "SELECT state,attempts FROM push_deliveries").Scan(&state, &attempts); err != nil || state != "delivered" || attempts != 2 {
		t.Fatal("retry was not delivered", state, attempts, err)
	}
	payload := g.payloads[0]
	if payload.Title != "BIKERS.KZ" || payload.Body != "Сізде жаңа жаңарту бар" || payload.URL != "/inbox" || payload.NotificationID == "" || payload != g.payloads[1] {
		t.Fatalf("push text or retry identity incorrect: %+v", payload)
	}
	g.statuses = []int{410}
	expect(t, f.request("POST", path, map[string]any{"client_announcement_id": uuid.NewString(), "body": "Second"}, o), 201)
	if err := f.s.Notifications.Dispatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := f.db.QueryRow(context.Background(), "SELECT count(*) FROM push_subscriptions").Scan(&count); err != nil || count != 0 {
		t.Fatal("dead endpoint retained", err)
	}
}

func TestPushRechecksRevocationMembershipAndBrowserAccountBinding(t *testing.T) {
	t.Run("revoked membership", func(t *testing.T) {
		f, g := startPushFixture(t)
		o := f.login("+77011234567", "Organizer")
		a := f.login("+77021234567", "Rider")
		id := f.ride(o, 2)
		f.join(id, a)
		expect(t, f.decision(id, o, a, "approve"), 200)
		subscribePush(t, f, a, "membership")
		expect(t, f.request("POST", "/api/v1/rides/"+id+"/announcements", map[string]any{"client_announcement_id": uuid.NewString(), "body": "Now inaccessible"}, o), 201)
		expect(t, f.decision(id, o, a, "remove"), 200)
		if err := f.s.Notifications.Dispatch(context.Background()); err != nil {
			t.Fatal(err)
		}
		var skipped int
		if err := f.db.QueryRow(context.Background(), "SELECT count(*) FROM push_deliveries d JOIN notifications n ON n.id=d.notification_id WHERE n.kind='ride.announcement' AND d.state='skipped'").Scan(&skipped); err != nil || skipped != 1 || len(g.payloads) != 1 {
			t.Fatal("removed rider received ride-content push, or lost generic removal notice", skipped, len(g.payloads), err)
		}
	})
	t.Run("logout and rebind", func(t *testing.T) {
		f, g := startPushFixture(t)
		o := f.login("+77011234567", "Organizer")
		a := f.login("+77021234567", "First rider")
		b := f.login("+77051234567", "Second rider")
		id := f.ride(o, 3)
		f.join(id, a)
		expect(t, f.decision(id, o, a, "approve"), 200)
		input := pushInput(t, "shared-browser")
		expect(t, f.request("POST", "/api/v1/me/push/subscriptions", input, a), 201)
		expect(t, f.request("POST", "/api/v1/me/push/subscriptions", input, b), 409)
		expect(t, f.request("POST", "/api/v1/rides/"+id+"/announcements", map[string]any{"client_announcement_id": uuid.NewString(), "body": "Private first-account update"}, o), 201)
		expect(t, f.request("DELETE", "/api/v1/session", nil, a), 200)
		expect(t, f.request("POST", "/api/v1/me/push/subscriptions", input, b), 201)
		if err := f.s.Notifications.Dispatch(context.Background()); err != nil || len(g.payloads) != 0 {
			t.Fatal("rebound browser received previous account's queued notice", err)
		}
		var queued int
		if err := f.db.QueryRow(context.Background(), "SELECT count(*) FROM push_deliveries").Scan(&queued); err != nil || queued != 0 {
			t.Fatal("old account outbox survived endpoint rebind", err)
		}
	})
	t.Run("revoked session", func(t *testing.T) {
		f, g := startPushFixture(t)
		o := f.login("+77011234567", "Organizer")
		a := f.login("+77021234567", "Rider")
		id := f.ride(o, 2)
		f.join(id, a)
		expect(t, f.decision(id, o, a, "approve"), 200)
		subscribePush(t, f, a, "logout")
		expect(t, f.request("POST", "/api/v1/rides/"+id+"/announcements", map[string]any{"client_announcement_id": uuid.NewString(), "body": "Queued before logout"}, o), 201)
		expect(t, f.request("DELETE", "/api/v1/session", nil, a), 200)
		if err := f.s.Notifications.Dispatch(context.Background()); err != nil || len(g.payloads) != 0 {
			t.Fatal("logged-out session received push", err)
		}
	})
}

func TestPushLeaseRecoveryAndStaleFinisherCannotOverwriteRetry(t *testing.T) {
	f, g := startPushFixture(t)
	o := f.login("+77011234567", "Organizer")
	a := f.login("+77021234567", "Rider")
	id := f.ride(o, 2)
	f.join(id, a)
	expect(t, f.decision(id, o, a, "approve"), 200)
	subscribePush(t, f, a, "lease")
	expect(t, f.request("POST", "/api/v1/rides/"+id+"/announcements", map[string]any{"client_announcement_id": uuid.NewString(), "body": "Durable notification"}, o), 201)
	repo := postgres.New(f.db)
	claimed, err := repo.ClaimPush(context.Background(), f.now, f.now.Add(2*time.Minute), func(models.PushDelivery) bool { return true })
	if err != nil || claimed == nil {
		t.Fatal("initial lease not acquired", err)
	}
	if err = f.s.Notifications.Dispatch(context.Background()); err != nil || len(g.payloads) != 0 {
		t.Fatal("active lease delivered twice", err)
	}
	f.now = f.now.Add(3 * time.Minute)
	if err = f.s.Notifications.Dispatch(context.Background()); err != nil || len(g.payloads) != 1 {
		t.Fatal("expired durable lease was not recovered", err)
	}
	if err = repo.FinishPush(context.Background(), claimed.ID, claimed.LeaseID, models.PushDeliveryResult{State: "failed", NextAttempt: f.now}); err != nil {
		t.Fatal(err)
	}
	var state string
	var attempts int
	if err = f.db.QueryRow(context.Background(), "SELECT state,attempts FROM push_deliveries WHERE id=$1", claimed.ID).Scan(&state, &attempts); err != nil || state != "delivered" || attempts != 2 {
		t.Fatal("stale worker overwrote recovered delivery", state, attempts, err)
	}
}

func TestPushRetriesAreBoundedAndNetworkDoesNotHoldApplicationLocks(t *testing.T) {
	t.Run("bounded failures", func(t *testing.T) {
		f, g := startPushFixture(t)
		o := f.login("+77011234811", "Organizer")
		a := f.login("+77011234812", "Rider")
		id := f.ride(o, 2)
		approvedRider(t, f, id, o, a)
		subscribePush(t, f, a, "exhausted")
		expect(t, f.request("POST", "/api/v1/rides/"+id+"/announcements", map[string]any{"client_announcement_id": uuid.NewString(), "body": "Retry bounded delivery"}, o), 201)
		g.statuses = []int{503, 503, 503, 503, 503, 201}
		for range 7 {
			if err := f.s.Notifications.Dispatch(context.Background()); err != nil {
				t.Fatal(err)
			}
			f.now = f.now.Add(time.Hour)
		}
		var state string
		var attempts int
		if err := f.db.QueryRow(context.Background(), "SELECT state,attempts FROM push_deliveries").Scan(&state, &attempts); err != nil || state != "failed" || attempts != 5 || len(g.payloads) != 5 {
			t.Fatal("transient retries exceeded the bounded delivery budget", state, attempts, len(g.payloads), err)
		}
	})
	t.Run("network outside transaction", func(t *testing.T) {
		f, _ := startPushFixture(t)
		o := f.login("+77011234821", "Organizer")
		a := f.login("+77011234822", "Rider")
		id := f.ride(o, 2)
		approvedRider(t, f, id, o, a)
		subscribePush(t, f, a, "blocked-network")
		expect(t, f.request("POST", "/api/v1/rides/"+id+"/announcements", map[string]any{"client_announcement_id": uuid.NewString(), "body": "Network outside database locks"}, o), 201)
		g := &blockingPushGateway{started: make(chan struct{}), release: make(chan struct{})}
		m := notifications.New(postgres.New(f.db), g, f.cfg.PushPublicKey, func() time.Time { return f.now })
		done := make(chan error, 1)
		go func() { done <- m.Dispatch(context.Background()) }()
		select {
		case <-g.started:
		case <-time.After(3 * time.Second):
			close(g.release)
			t.Fatal("push network call did not start")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		locale := "ru"
		_, err := f.s.Notifications.UpdatePreferences(ctx, models.User{ID: a.id}, models.PreferencesPatch{Locale: &locale})
		cancel()
		close(g.release)
		if dispatchErr := <-done; dispatchErr != nil {
			t.Fatal(dispatchErr)
		}
		if err != nil {
			t.Fatal("push network call retained the aggregate database lock", err)
		}
	})
}

type blockingPushGateway struct {
	started, release chan struct{}
}

func (g *blockingPushGateway) Send(ctx context.Context, _ models.PushSubscriptionInput, _ models.PushPayload) (int, error) {
	close(g.started)
	select {
	case <-g.release:
		return 201, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}
