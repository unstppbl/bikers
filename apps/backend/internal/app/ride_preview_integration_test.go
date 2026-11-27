package app

import (
	"html"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestRideSharePreviewUsesOnlyCurrentPublicFields(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011234971", "PRIVATE OWNER")
	rider := f.login("+77011234972", "Approved rider")
	pending := f.login("+77011234973", "Pending rider")
	admin := f.login("+77011234974", "Moderator")
	grantTestAdmin(t, f, admin)
	const description = `A paved & easy loop <script>alert("test")</script>`
	created := f.request("POST", "/api/v1/rides", map[string]any{
		"city": "Almaty", "description": description, "title": "Charyn Canyon Loop",
		"style": "touring", "surface": "asphalt", "distance_km": 180, "publish_public_details": true,
		"starts_at": "2026-11-07T23:30:00Z", "timezone": "Asia/Almaty",
		"meeting_point": "PRIVATE MEETUP", "destination": "PRIVATE DESTINATION",
		"route_url": "https://example.org/private-route", "expectations": "PRIVATE PACE", "capacity": 4,
	}, owner)
	expect(t, created, http.StatusCreated)
	id := created.body["ride"].(map[string]any)["id"].(string)
	f.join(id, rider)
	expect(t, f.decision(id, owner, rider, "approve"), http.StatusOK)
	f.join(id, pending)
	cover := rideCoverUpload(t, f, id, "1", photoPNG(t), owner, true)
	expect(t, cover, http.StatusCreated)
	image := cover.body["image"].(map[string]any)
	pageURL := f.ts.URL + "/rides/" + id
	coverURL := f.ts.URL + image["url"].(string)

	read := func(viewer *actor, want int) string {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, pageURL+"?forwarded=anything", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("User-Agent", "TelegramBot")
		req.Header.Set("X-Forwarded-Host", "unexpected.example.org")
		if viewer != nil {
			req.AddCookie(viewer.cookie)
		}
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != want || response.Header.Get("Cache-Control") != "no-store" {
			t.Fatalf("preview status=%d cache=%q body=%s", response.StatusCode, response.Header.Get("Cache-Control"), body)
		}
		return string(body)
	}

	guest := read(nil, http.StatusOK)
	for _, want := range []string{
		`property="og:url" content="` + pageURL + `"`,
		`property="og:title" content="Charyn Canyon Loop"`,
		`property="og:description" content="Almaty · 2026-11-08 · ` + html.EscapeString(description) + `"`,
		`property="og:image" content="` + coverURL + `"`, "/assets/app.js",
	} {
		if !strings.Contains(guest, want) {
			t.Errorf("preview missing %q: %s", want, guest)
		}
	}
	for _, secret := range []string{"PRIVATE", "23:30", "private-route", "<script>alert", "unexpected.example.org", "?forwarded"} {
		if strings.Contains(guest, secret) {
			t.Errorf("preview contains %q: %s", secret, guest)
		}
	}
	for _, viewer := range []*actor{owner, rider, pending} {
		if got := read(viewer, http.StatusOK); got != guest {
			t.Fatal("session changed the public preview")
		}
	}
	expect(t, f.decision(id, owner, rider, "remove"), http.StatusOK)
	if got := read(rider, http.StatusOK); got != guest {
		t.Fatal("removed participant received a different public preview")
	}
	expect(t, trustAction(f, admin, "hide_content", "ride_cover", image["id"].(string), "Hide cover", ""), http.StatusOK)
	if got := read(owner, http.StatusOK); strings.Contains(got, `property="og:image"`) || strings.Contains(got, coverURL) {
		t.Fatalf("preview kept a moderated cover: %s", got)
	}
	expect(t, f.request("POST", "/api/v1/rides/"+id+"/cancel", map[string]any{"plan_version": 1}, owner), http.StatusOK)
	if got := read(nil, http.StatusOK); !strings.Contains(got, `property="og:title" content="Cancelled · Charyn Canyon Loop"`) {
		t.Fatalf("preview is missing the cancelled state: %s", got)
	}
	expect(t, trustAction(f, admin, "hide_content", "ride", id, "Hide ride", ""), http.StatusOK)
	if got := read(owner, http.StatusNotFound); strings.Contains(got, "Almaty") || strings.Contains(got, "og:") {
		t.Fatalf("moderated ride retained a preview: %s", got)
	}
}
