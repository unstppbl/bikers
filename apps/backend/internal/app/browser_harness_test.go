package app

import (
	"bikers.kz/backend/internal/database/postgres"
	"bikers.kz/backend/internal/models"
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

	"github.com/google/uuid"
)

// TestBrowserHarness is an explicitly requested local UI verification seam. It is
// excluded from the production binary by Go's _test.go compilation rules.
// Run BROWSER_HARNESS=1 TEST_DATABASE_URL=... go test ./internal/app -run '^TestBrowserHarness$' -v -timeout 20m.
func TestBrowserHarness(t *testing.T) {
	if os.Getenv("BROWSER_HARNESS") != "1" {
		t.Skip("opt-in local browser harness")
	}
	f := newFixture(t)
	if os.Getenv("BROWSER_HARNESS_RIDE_MENUS") == "1" {
		f.now = time.Now().UTC()
		owner := f.login("+77011239928", "QA Organizer")
		member := f.login("+77011239929", "QA Member")
		other := f.login("+77011239930", "QA Other Rider")
		pending := f.login("+77011239931", "QA Applicant")
		grantTestAdmin(t, f, owner)
		for _, title := range []string{"QA participant menus", "QA second ride"} {
			created := f.request("POST", "/api/v1/rides", map[string]any{
				"city": "Almaty", "description": "Isolated browser fixture only", "title": title,
				"starts_at": f.now.Add(24 * time.Hour).Format(time.RFC3339), "timezone": "Asia/Almaty",
				"meeting_point": "QA meeting point", "destination": "QA destination", "capacity": 10,
			}, owner)
			expect(t, created, 201)
			id := created.body["ride"].(map[string]any)["id"].(string)
			for _, rider := range []*actor{member, other, pending} {
				f.join(id, rider)
				if rider != pending {
					expect(t, f.decision(id, owner, rider, "approve"), 200)
				}
			}
			for i := range 18 {
				sender := []*actor{owner, member, other}[i%3]
				expect(t, f.request("POST", "/api/v1/rides/"+id+"/messages", map[string]any{
					"client_message_id": uuid.NewString(), "body": "QA conversation message " + strconv.Itoa(i+1) + ". This is an isolated fixture for menu and scroll checks.",
				}, sender), 201)
			}
			expect(t, f.request("POST", "/api/v1/rides/"+id+"/announcements", map[string]any{
				"client_announcement_id": uuid.NewString(), "body": "QA announcement for isolated browser checks. Meet at the test point before departure.",
			}, owner), 201)
			t.Logf("TEST-ONLY ride menu fixture: %s; ride=%s", title, id)
		}
		t.Log("TEST-ONLY organizer/admin phone=+77011239928; member=+77011239929; other=+77011239930; applicant=+77011239931; OTP=123456")
	}
	if os.Getenv("BROWSER_HARNESS_ADMIN") == "1" {
		admin := f.login("+77011239901", "Test Administrator")
		grantTestAdmin(t, f, admin)
		t.Log("TEST-ONLY precreated administrator: phone=+77011239901; OTP=123456")
	}
	if os.Getenv("BROWSER_HARNESS_CLUB_NOTICES") == "1" {
		owner := f.login("+77011239902", "Test Club Owner")
		applicant := f.login("+77011239903", "Test Club Applicant")
		club := testClub(t, f, owner)["id"].(string)
		path := "/api/v1/clubs/" + club + "/membership"
		expect(t, f.request("PUT", path, nil, applicant), 200)
		expect(t, clubDecision(f, club, owner, applicant, "approve"), 200)
		expect(t, clubDecision(f, club, owner, applicant, "remove"), 200)
		expect(t, f.request("PUT", path, nil, applicant), 200)
		expect(t, clubDecision(f, club, owner, applicant, "reject"), 200)
		expect(t, f.request("PUT", path, nil, applicant), 200)
		t.Logf("TEST-ONLY club inbox fixture: owner=+77011239902; applicant=+77011239903; OTP=123456; club=%s", club)
	}
	if os.Getenv("BROWSER_HARNESS_PUBLIC_BIKES") == "1" {
		owner := f.login("+77011239904", "Test Catalogue Owner")
		for _, fields := range []map[string]any{
			{"model_id": "yamaha-mt-07", "year": 2021, "visibility": "public"},
			{"model_id": "honda-cb500x", "year": 2019, "visibility": "public"},
			{"model_id": "ktm-390-adventure", "year": 2024, "visibility": "public"},
			{"make": "Private Catalogue Fixture", "model": "Never Public", "year": 2020},
		} {
			catalogueBike(t, f, owner, fields)
		}
		t.Log("TEST-ONLY public catalogue fixture: three public reference-image bikes and one private bike; owner=+77011239904; OTP=123456")
	}
	if os.Getenv("BROWSER_HARNESS_PLACES") == "1" {
		contributor := f.login("+77011239922", "QA Places Contributor")
		admin := f.login("+77011239923", "QA Places Administrator")
		grantTestAdmin(t, f, admin)
		fixtures := []models.PlaceInput{
			{Name: "QA fuel point", Category: "fuel", Lat: 43.24, Lon: 76.91, City: "Almaty", Description: "Test fixture only"},
			{Name: "QA workshop", Category: "workshop", Lat: 43.243, Lon: 76.915, City: "Almaty", Description: "Test fixture only"},
			{Name: "QA meeting point", Category: "meeting_spot", Lat: 43.237, Lon: 76.905, City: "Almaty", Description: "Test fixture only"},
		}
		firstID := ""
		for i, place := range fixtures {
			created := f.request("POST", "/api/v1/places/submissions", models.PlaceSubmissionInput{Kind: "new", Place: &place, Reason: "Test fixture only"}, contributor)
			expect(t, created, 201)
			submission := created.body["submission"].(map[string]any)
			reviewed := f.request("POST", "/api/v1/admin/place-submissions/"+submission["id"].(string)+"/review", models.PlaceReviewInput{Version: 1, Status: "approved", Reason: "Test fixture only"}, admin)
			expect(t, reviewed, 200)
			if i == 0 {
				firstID = reviewed.body["submission"].(map[string]any)["place_id"].(string)
			}
		}
		correction := fixtures[0]
		correction.Name = "QA fuel point correction"
		expect(t, f.request("POST", "/api/v1/places/submissions", models.PlaceSubmissionInput{Kind: "correction", PlaceID: firstID, BaseVersion: 1, Place: &correction, Reason: "Test fixture only"}, contributor), 201)
		pending := models.PlaceInput{Name: "QA pending meeting point", Category: "meeting_spot", Lat: 43.242, Lon: 76.912, City: "Almaty", Description: "Test fixture only"}
		expect(t, f.request("POST", "/api/v1/places/submissions", models.PlaceSubmissionInput{Kind: "new", Place: &pending, Reason: "Test fixture only"}, contributor), 201)
		t.Log("TEST-ONLY places fixture: contributor phone=+77011239922; administrator phone=+77011239923")
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
	if os.Getenv("BROWSER_HARNESS_GIS") == "1" {
		cfg.PhotonBaseURL = os.Getenv("PHOTON_BASE_URL")
		cfg.ValhallaBaseURL = os.Getenv("VALHALLA_BASE_URL")
		cfg.MapsArchiveURL = os.Getenv("MAPS_ARCHIVE_URL")
		if cfg.PhotonBaseURL == "" || cfg.ValhallaBaseURL == "" || cfg.MapsArchiveURL == "" {
			t.Fatal("GIS harness requires explicit PHOTON_BASE_URL, VALHALLA_BASE_URL and MAPS_ARCHIVE_URL")
		}
		t.Log("TEST-ONLY GIS providers and same-origin archive enabled from explicit environment settings")
	}
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
