package app

import (
	"bikers.kz/backend/internal/database/postgres"
	"context"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// TestBrowserHarness is an explicitly requested local UI verification seam. It is
// excluded from the production binary by Go's _test.go compilation rules.
// Run BROWSER_HARNESS=1 TEST_DATABASE_URL=... go test ./internal/app -run '^TestBrowserHarness$' -v -timeout 20m.
func TestBrowserHarness(t *testing.T) {
	if os.Getenv("BROWSER_HARNESS") != "1" {
		t.Skip("opt-in local browser harness")
	}
	f := newFixture(t)
	if os.Getenv("BROWSER_HARNESS_ADMIN") == "1" {
		admin := f.login("+77011239901", "Test Administrator")
		grantTestAdmin(t, f, admin)
		t.Log("TEST-ONLY precreated administrator: phone=+77011239901; OTP=123456")
	}
	f.ts.Close()
	port := os.Getenv("BROWSER_HARNESS_PORT")
	if port == "" {
		port = "8081"
	}
	listener, e := net.Listen("tcp", "127.0.0.1:"+port)
	if e != nil {
		t.Fatal(e)
	}
	base := "http://127.0.0.1:" + port
	if v := os.Getenv("BROWSER_HARNESS_BASE_URL"); v != "" {
		base = v
	}

	web := os.Getenv("WEB_DIST_DIR")
	if web == "" {
		web, e = filepath.Abs("../../../web/dist")
		if e != nil {
			t.Fatal(e)
		}
	}
	cfg := f.cfg
	cfg.BaseURL = base
	cfg.WebDistDir = web
	options := Options{Now: func() time.Time { return time.Now().UTC() }}
	if os.Getenv("BROWSER_HARNESS_PUSH") == "1" {
		private, x, y, err := elliptic.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		cfg.PushPublicKey = base64.RawURLEncoding.EncodeToString(elliptic.Marshal(elliptic.P256(), x, y))
		cfg.PushPrivateKey = base64.RawURLEncoding.EncodeToString(private)
		cfg.PushSubject = "mailto:isolated@example.com"
		options.PushGateway = &fakePushGateway{}
		t.Log("TEST-ONLY push settings enabled with a fake gateway; no external deliveries")
	}
	runtime, e := New(cfg, postgres.New(f.db), f.gateway, options)
	if e != nil {
		t.Fatal(e)
	}
	h := http.Server{Handler: runtime.Handler, ReadHeaderTimeout: 5 * time.Second}

	defer h.Shutdown(context.Background())
	go func() { _ = h.Serve(listener) }()
	t.Logf("TEST-ONLY browser harness: %s; OTP=123456; isolated schema; web=%s", base, web)
	duration := 10 * time.Minute
	if v := os.Getenv("BROWSER_HARNESS_SECONDS"); v != "" {
		n, e := strconv.Atoi(v)
		if e != nil || n < 1 {
			t.Fatal("invalid harness duration")
		}
		duration = time.Duration(n) * time.Second
	}
	select {
	case <-time.After(duration):
	case <-t.Context().Done():
	}
}
