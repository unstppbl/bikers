package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"bikers.kz/backend/internal/config"
	"bikers.kz/backend/internal/database/postgres"
	"bikers.kz/backend/internal/managers/accounts"
	"bikers.kz/backend/internal/models"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This provider exists only in the test binary. Production always uses Telegram.
type fakeGateway struct {
	mu            sync.Mutex
	phones        map[string]string
	status        string
	phoneOverride string
	sends, checks int
}

func newFakeGateway() *fakeGateway {
	return &fakeGateway{phones: map[string]string{}, status: "code_valid"}
}
func (g *fakeGateway) Send(_ context.Context, phone string) (models.Verification, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.sends++
	id := uuid.NewString()
	g.phones[id] = phone
	return models.Verification{RequestID: id, Phone: phone}, nil
}
func (g *fakeGateway) Verify(_ context.Context, id, code string) (models.Verification, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.checks++
	v := models.Verification{RequestID: id, Phone: g.phones[id]}
	if g.phoneOverride != "" {
		v.Phone = g.phoneOverride
	}
	status := g.status
	if code != "123456" {
		status = "code_invalid"
	}
	if status != "" {
		v.VerificationStatus = &struct {
			Status string `json:"status"`
		}{status}
	}
	return v, nil
}
func (g *fakeGateway) set(status, phone string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.status = status
	g.phoneOverride = phone
}

type fixture struct {
	t       *testing.T
	db      *pgxpool.Pool
	s       *Runtime
	cfg     config.Config
	ts      *httptest.Server
	gateway *fakeGateway
	now     time.Time
}
type actor struct {
	id, csrf string
	cookie   *http.Cookie
}
type response struct {
	status  int
	body    map[string]any
	raw     string
	cookies []*http.Cookie
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("PostgreSQL integration requires TEST_DATABASE_URL; run the integration target before delivery")
	}
	ctx := context.Background()
	admin, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	schema := "bikerskz_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, e = admin.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); e != nil {
		admin.Close()
		t.Fatal(e)
	}
	pc, e := pgxpool.ParseConfig(dsn)
	if e != nil {
		t.Fatal(e)
	}
	pc.ConnConfig.RuntimeParams["search_path"] = schema
	pc.MaxConns = 12
	db, e := pgxpool.NewWithConfig(ctx, pc)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		db.Close()
		_, _ = admin.Exec(ctx, "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
		admin.Close()
	})
	if e = postgres.New(db).Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	web := t.TempDir()
	if e = os.WriteFile(filepath.Join(web, "index.html"), []byte(`<html><head><link rel="stylesheet" href="/assets/app.css"><script type="module" src="/assets/app.js"></script></head><body><div id="root"></div></body></html>`), 0600); e != nil {
		t.Fatal(e)
	}
	g := newFakeGateway()
	f := &fixture{t: t, db: db, gateway: g, now: time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC), cfg: config.Config{DatabaseURL: dsn, MediaDir: t.TempDir(), WebDistDir: web}}
	f.startServer(g)
	return f
}
func (f *fixture) startServer(gateway accounts.Gateway) {
	f.t.Helper()
	if f.ts != nil {
		f.ts.Close()
	}
	ts := httptest.NewUnstartedServer(nil)
	cfg := f.cfg
	cfg.BaseURL = "http://" + ts.Listener.Addr().String()
	runtime, e := New(cfg, postgres.New(f.db), gateway, Options{Now: func() time.Time { return f.now }})
	if e != nil {
		ts.Close()
		f.t.Fatal(e)
	}
	f.cfg = cfg
	f.s = runtime
	f.ts = ts
	ts.Config.Handler = runtime.Handler
	ts.Start()
	f.t.Cleanup(ts.Close)
}

func (f *fixture) request(method, path string, payload any, a *actor) response {
	f.t.Helper()
	var body io.Reader
	if payload != nil {
		b, e := json.Marshal(payload)
		if e != nil {
			f.t.Fatal(e)
		}
		body = bytes.NewReader(b)
	}
	req, e := http.NewRequest(method, f.ts.URL+path, body)
	if e != nil {
		f.t.Fatal(e)
	}
	req.Header.Set("Origin", f.ts.URL)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if a != nil {
		req.AddCookie(a.cookie)
		req.Header.Set("X-CSRF-Token", a.csrf)
	}
	r, e := http.DefaultClient.Do(req)
	if e != nil {
		f.t.Fatal(e)
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	out := response{status: r.StatusCode, raw: string(b), cookies: r.Cookies()}
	_ = json.Unmarshal(b, &out.body)
	if strings.HasPrefix(path, "/api/") && r.Header.Get("Cache-Control") != "no-store" {
		f.t.Error("private API cache headers missing")
	}
	return out
}
func expect(t *testing.T, r response, status int) {
	t.Helper()
	if r.status != status {
		t.Fatalf("status=%d want=%d body=%s", r.status, status, r.raw)
	}
}
func (f *fixture) challenge(phone string) string {
	f.t.Helper()
	r := f.request("POST", "/api/v1/auth/challenges", map[string]any{"phone": phone}, nil)
	expect(f.t, r, 201)
	return r.body["challenge_id"].(string)
}
func (f *fixture) login(phone, name string) *actor {
	f.t.Helper()
	id := f.challenge(phone)
	r := f.request("POST", "/api/v1/auth/challenges/verify", map[string]any{"challenge_id": id, "code": "123456"}, nil)
	expect(f.t, r, 200)
	if len(r.cookies) != 1 {
		f.t.Fatal("session cookie missing")
	}
	a := &actor{id: r.body["user"].(map[string]any)["id"].(string), csrf: r.body["csrf_token"].(string), cookie: r.cookies[0]}
	r = f.request("PATCH", "/api/v1/me/profile", map[string]any{"display_name": name, "city": "Almaty", "languages": []string{"en", "ru"}, "locale": "en"}, a)
	expect(f.t, r, 200)
	return a
}
func (f *fixture) ride(a *actor, capacity int) string {
	f.t.Helper()
	r := f.request("POST", "/api/v1/rides", map[string]any{"city": "Almaty", "description": "A relaxed city ride", "title": "PRIVATE TITLE", "starts_at": "2026-11-07T23:30:00Z", "timezone": "Asia/Almaty", "meeting_point": "SECRET MEETING", "destination": "PRIVATE DESTINATION", "route_url": "https://example.com/route", "expectations": "Relaxed", "capacity": capacity}, a)
	expect(f.t, r, 201)
	return r.body["ride"].(map[string]any)["id"].(string)
}
func (f *fixture) join(id string, a *actor) {
	f.t.Helper()
	expect(f.t, f.request("PUT", "/api/v1/rides/"+id+"/participation", nil, a), 200)
}
func (f *fixture) decision(id string, owner, target *actor, d string) response {
	return f.request("POST", "/api/v1/rides/"+id+"/participants/"+target.id+"/decision", map[string]any{"decision": d}, owner)
}

func TestOTPRejectsInvalidProviderStatesBindingExpiryAndReplay(t *testing.T) {
	for _, mode := range []string{"code_invalid", "", "expired", "code_max_attempts_exceeded", "PHONE_MISMATCH"} {
		t.Run(mode, func(t *testing.T) {
			f := newFixture(t)
			id := f.challenge("+77011234567")
			status, phone := mode, ""
			if mode == "PHONE_MISMATCH" {
				status = "code_valid"
				phone = "+77021234567"
			}
			f.gateway.set(status, phone)
			expect(t, f.request("POST", "/api/v1/auth/challenges/verify", map[string]any{"challenge_id": id, "code": "123456"}, nil), 401)
			var count int
			if e := f.db.QueryRow(context.Background(), "SELECT count(*) FROM sessions").Scan(&count); e != nil || count != 0 {
				t.Fatalf("invalid challenge created session: %d %v", count, e)
			}
			if mode == "expired" || mode == "code_max_attempts_exceeded" {
				f.gateway.set("code_valid", "")
				expect(t, f.request("POST", "/api/v1/auth/challenges/verify", map[string]any{"challenge_id": id, "code": "123456"}, nil), 401)
			}
		})
	}
	t.Run("local expiry", func(t *testing.T) {
		f := newFixture(t)
		id := f.challenge("+77011234567")
		_, e := f.db.Exec(context.Background(), "UPDATE otp_challenges SET expires_at=$2 WHERE id=$1", id, f.now.Add(-time.Second))
		if e != nil {
			t.Fatal(e)
		}
		expect(t, f.request("POST", "/api/v1/auth/challenges/verify", map[string]any{"challenge_id": id, "code": "123456"}, nil), 401)
		if f.gateway.checks != 0 {
			t.Fatal("expired challenge reached provider")
		}
	})
	t.Run("caller cannot substitute phone", func(t *testing.T) {
		f := newFixture(t)
		id := f.challenge("+77011234567")
		expect(t, f.request("POST", "/api/v1/auth/challenges/verify", map[string]any{"challenge_id": id, "code": "123456", "phone": "+77021234567"}, nil), 400)
	})
	t.Run("concurrent consume and replay", func(t *testing.T) {
		f := newFixture(t)
		id := f.challenge("+77011234567")
		codes := make(chan int, 10)
		var wg sync.WaitGroup
		for range 10 {
			wg.Go(func() {
				codes <- f.request("POST", "/api/v1/auth/challenges/verify", map[string]any{"challenge_id": id, "code": "123456"}, nil).status
			})
		}
		wg.Wait()
		close(codes)
		success := 0
		for c := range codes {
			if c == 200 {
				success++
			} else if c != 401 {
				t.Errorf("unexpected concurrent status %d", c)
			}
		}
		if success != 1 {
			t.Fatalf("%d successful consumes", success)
		}
		expect(t, f.request("POST", "/api/v1/auth/challenges/verify", map[string]any{"challenge_id": id, "code": "123456"}, nil), 401)
		var sessions, users int
		_ = f.db.QueryRow(context.Background(), "SELECT count(*) FROM sessions").Scan(&sessions)
		_ = f.db.QueryRow(context.Background(), "SELECT count(*) FROM users").Scan(&users)
		if sessions != 1 || users != 1 {
			t.Fatalf("sessions=%d users=%d", sessions, users)
		}
	})
}

func TestOTPLimitsSpanUTCMidnightAndAttempts(t *testing.T) {
	f := newFixture(t)
	f.now = time.Date(2026, 11, 4, 0, 0, 10, 0, time.UTC)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		created := f.now.Add(-time.Duration(i+2) * time.Minute)
		_, e := f.db.Exec(ctx, `INSERT INTO otp_challenges(id,phone,source_key,state,created_at,expires_at,resend_after) VALUES($1,'+77011234567','old-source','failed',$2,$3,$4)`, uuid.NewString(), created, created.Add(5*time.Minute), created.Add(time.Minute))
		if e != nil {
			t.Fatal(e)
		}
	}
	expect(t, f.request("POST", "/api/v1/auth/challenges", map[string]any{"phone": "+77011234567"}, nil), 429)
	id := f.challenge("+77021234567")
	expect(t, f.request("POST", "/api/v1/auth/challenges", map[string]any{"phone": "+77021234567"}, nil), 429)
	for range 5 {
		expect(t, f.request("POST", "/api/v1/auth/challenges/verify", map[string]any{"challenge_id": id, "code": "000000"}, nil), 401)
	}
	expect(t, f.request("POST", "/api/v1/auth/challenges/verify", map[string]any{"challenge_id": id, "code": "123456"}, nil), 401)
	if f.gateway.checks != 5 {
		t.Fatalf("provider calls=%d want 5", f.gateway.checks)
	}
}

func TestRideJourneyPrivacyCapacityMessagesAndRevocation(t *testing.T) {
	f := newFixture(t)
	o := f.login("+77011234567", "Organizer")
	a := f.login("+77021234567", "Rider")
	b := f.login("+77051234567", "Waitlisted")
	id := f.ride(o, 2)
	r := f.request("GET", "/api/v1/rides/"+id, nil, nil)
	expect(t, r, 200)
	summary := r.body["ride"].(map[string]any)
	if len(summary) != 5 || summary["date"] != "2026-11-08" {
		t.Fatalf("public summary=%v", summary)
	}
	if strings.Contains(r.raw, "SECRET") || strings.Contains(r.raw, "PRIVATE") {
		t.Fatal("private preview fields leaked")
	}
	expect(t, f.request("GET", "/api/v1/rides/"+id+"/messages", nil, nil), 401)
	f.join(id, a)
	f.join(id, b)
	expect(t, f.request("GET", "/api/v1/rides/"+id+"/messages", nil, a), 403)
	expect(t, f.request("GET", "/api/v1/rides/"+id+"/participants", nil, a), 403)
	r = f.request("GET", "/api/v1/rides/"+id, nil, a)
	if strings.Contains(r.raw, "meeting_point") || strings.Contains(r.raw, "organizer_id") {
		t.Fatal("pending rider received private plan")
	}
	expect(t, f.decision(id, o, a, "approve"), 200)
	expect(t, f.decision(id, o, b, "approve"), 409)
	r = f.request("GET", "/api/v1/rides/"+id, nil, a)
	if !strings.Contains(r.raw, "SECRET MEETING") {
		t.Fatal("approved participant cannot see plan")
	}
	clientID := uuid.NewString()
	payload := map[string]any{"client_message_id": clientID, "body": "Hello riders"}
	r = f.request("POST", "/api/v1/rides/"+id+"/messages", payload, a)
	expect(t, r, 201)
	expect(t, f.request("POST", "/api/v1/rides/"+id+"/messages", payload, a), 200)
	payload["body"] = "Changed"
	expect(t, f.request("POST", "/api/v1/rides/"+id+"/messages", payload, a), 409)
	var wg sync.WaitGroup
	for i := range 16 {
		wg.Go(func() {
			expect(t, f.request("POST", "/api/v1/rides/"+id+"/messages", map[string]any{"client_message_id": uuid.NewString(), "body": fmt.Sprintf("Message %d", i)}, a), 201)
		})
	}
	wg.Wait()
	r = f.request("GET", "/api/v1/rides/"+id+"/messages?after=0", nil, a)
	expect(t, r, 200)
	messages := r.body["messages"].([]any)
	if len(messages) != 17 {
		t.Fatalf("messages=%d", len(messages))
	}
	for i, m := range messages {
		if m.(map[string]any)["sequence"] != float64(i+1) {
			t.Fatal("cursor order gap")
		}
	}
	r = f.request("GET", "/api/v1/rides/"+id+"/messages?after=17", nil, a)
	if len(r.body["messages"].([]any)) != 0 || r.body["next_sequence"] != float64(17) {
		t.Fatal("empty cursor should remain stable")
	}
	expect(t, f.decision(id, o, a, "remove"), 200)
	expect(t, f.request("GET", "/api/v1/rides/"+id+"/messages", nil, a), 403)
	r = f.request("GET", "/api/v1/rides/"+id, nil, a)
	if strings.Contains(r.raw, "meeting_point") {
		t.Fatal("removed rider retained private plan")
	}
	expect(t, f.decision(id, o, b, "approve"), 200)
}

func TestSessionsCSRFPlanVersionsCancellationAndWebsocket(t *testing.T) {
	f := newFixture(t)
	o := f.login("+77011234567", "Organizer")
	a := f.login("+77021234567", "Rider")
	id := f.ride(o, 2)
	f.join(id, a)
	expect(t, f.decision(id, o, a, "approve"), 200)
	bad := *o
	bad.csrf = "bad"
	expect(t, f.request("PATCH", "/api/v1/me/profile", map[string]any{"city": "Astana"}, &bad), 403)
	expect(t, f.request("PATCH", "/api/v1/rides/"+id, map[string]any{"plan_version": 0, "title": "Wrong version"}, o), 409)
	expect(t, f.request("PATCH", "/api/v1/rides/"+id, map[string]any{"plan_version": 1, "capacity": 1}, o), 409)
	expect(t, f.request("PATCH", "/api/v1/rides/"+id, map[string]any{"plan_version": 1, "meeting_point": "NEW SECRET"}, o), 200)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	headers := http.Header{"Origin": []string{f.ts.URL}, "Cookie": []string{a.cookie.String()}}
	c, _, e := websocket.Dial(ctx, "ws"+strings.TrimPrefix(f.ts.URL, "http")+"/api/v1/events?ride_id="+id, &websocket.DialOptions{HTTPHeader: headers})
	if e != nil {
		t.Fatal(e)
	}
	defer c.CloseNow()
	expect(t, f.request("POST", "/api/v1/rides/"+id+"/messages", map[string]any{"client_message_id": uuid.NewString(), "body": "Websocket hint"}, o), 201)
	var event models.Event
	if e = wsjson.Read(ctx, c, &event); e != nil {
		t.Fatal(e)
	}
	if event.Type != "message.created" || event.RideID != id {
		t.Fatalf("event=%v", event)
	}
	expect(t, f.request("DELETE", "/api/v1/session", nil, a), 200)
	if e = wsjson.Read(ctx, c, &event); e == nil {
		t.Fatal("logged-out socket remained active")
	}
	r := f.request("GET", "/api/v1/session", nil, a)
	expect(t, r, 200)
	if r.body["user"] != nil {
		t.Fatal("revoked session still active")
	}
	expect(t, f.request("POST", "/api/v1/rides/"+id+"/cancel", map[string]any{"plan_version": 1}, o), 409)
	expect(t, f.request("POST", "/api/v1/rides/"+id+"/cancel", map[string]any{"plan_version": 2}, o), 200)
	expect(t, f.request("POST", "/api/v1/rides/"+id+"/messages", map[string]any{"client_message_id": uuid.NewString(), "body": "Too late"}, o), 409)
	expect(t, f.request("GET", "/api/v1/rides/"+id+"/messages", nil, o), 200)
}

func TestStaticDeepLinksPublicHTMLAndHealth(t *testing.T) {
	f := newFixture(t)
	o := f.login("+77011234567", "Organizer")
	id := f.ride(o, 2)
	for _, p := range []string{"/rides/new", "/rides/" + id + "/edit"} {
		expect(t, f.request("GET", p, nil, nil), 200)
	}
	expect(t, f.request("GET", "/assets/missing.js", nil, nil), 404)
	expect(t, f.request("GET", "/api/v1/missing", nil, nil), 404)
	expect(t, f.request("GET", "/health/ready", nil, nil), 200)
	r := f.request("GET", "/rides/"+id, nil, o)
	expect(t, r, 200)
	for _, secret := range []string{"PRIVATE TITLE", "SECRET MEETING", "PRIVATE DESTINATION", "organizer_id"} {
		if strings.Contains(r.raw, secret) {
			t.Fatalf("preview leaked %s", secret)
		}
	}
	if !strings.Contains(r.raw, "/assets/app.js") {
		t.Fatal("preview lost application assets")
	}
}

func TestConcurrentApprovalsNeverOverbookAndRemovedSocketCloses(t *testing.T) {
	f := newFixture(t)
	o := f.login("+77011234567", "Organizer")
	a := f.login("+77021234567", "Rider A")
	b := f.login("+77051234567", "Rider B")
	id := f.ride(o, 2)
	f.join(id, a)
	f.join(id, b)
	codes := make(chan int, 2)
	var wg sync.WaitGroup
	for _, target := range []*actor{a, b} {
		wg.Go(func() { codes <- f.decision(id, o, target, "approve").status })
	}
	wg.Wait()
	close(codes)
	success, full := 0, 0
	for c := range codes {
		switch c {
		case 200:
			success++
		case 409:
			full++
		default:
			t.Fatalf("unexpected approval status %d", c)
		}
	}
	if success != 1 || full != 1 {
		t.Fatalf("approval race success=%d full=%d", success, full)
	}
	var approved string
	if e := f.db.QueryRow(context.Background(), "SELECT user_id::text FROM participation WHERE ride_id=$1 AND state='approved' AND user_id<>$2", id, o.id).Scan(&approved); e != nil {
		t.Fatal(e)
	}
	member := a
	if approved == b.id {
		member = b
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, e := websocket.Dial(ctx, "ws"+strings.TrimPrefix(f.ts.URL, "http")+"/api/v1/events?ride_id="+id, &websocket.DialOptions{HTTPHeader: http.Header{"Origin": []string{f.ts.URL}, "Cookie": []string{member.cookie.String()}}})
	if e != nil {
		t.Fatal(e)
	}
	defer c.CloseNow()
	expect(t, f.decision(id, o, member, "remove"), 200)
	var event models.Event
	if e = wsjson.Read(ctx, c, &event); e == nil {
		t.Fatal("removed participant received event or retained socket")
	}
	_, e = f.db.Exec(context.Background(), "UPDATE users SET status='suspended' WHERE id=$1", member.id)
	if e != nil {
		t.Fatal(e)
	}
	r := f.request("GET", "/api/v1/session", nil, member)
	expect(t, r, 200)
	if r.body["user"] != nil {
		t.Fatal("suspended account session remains usable")
	}
}
