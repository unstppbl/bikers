package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func clubImageUpload(t *testing.T, f *fixture, clubID, version string, data []byte, owner *actor, csrf bool) response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, f.ts.URL+"/api/v1/clubs/"+clubID+"/image?version="+version, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", f.ts.URL)
	req.Header.Set("Content-Type", "image/png")
	if owner != nil {
		req.AddCookie(owner.cookie)
		if csrf {
			req.Header.Set("X-CSRF-Token", owner.csrf)
		}
	}
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	data, err = io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	result := response{status: r.StatusCode, raw: string(data)}
	_ = json.Unmarshal(data, &result.body)
	if r.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("club image metadata response may be cached")
	}
	return result
}

func clubImageFailure(t *testing.T, result response, status int, code string) {
	t.Helper()
	expect(t, result, status)
	fault, ok := result.body["error"].(map[string]any)
	if !ok || fault["code"] != code {
		t.Fatalf("wanted %s, received %s", code, result.raw)
	}
}

func TestClubImagePublicMetadataOwnerCommandsAndAtomicReplacement(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011234801", "Club image owner")
	member := f.login("+77011234802", "Club member")
	clubID := testClub(t, f, owner)["id"].(string)
	base := "/api/v1/clubs/" + clubID
	metadata := base + "/image"
	for _, viewer := range []*actor{nil, member, owner} {
		result := f.request("GET", metadata, nil, viewer)
		expect(t, result, 200)
		if result.body["image"] != nil {
			t.Fatal("new club has an unexpected image")
		}
		if viewer == owner {
			if len(result.body) != 2 || result.body["version"] != float64(1) {
				t.Fatalf("owner initial image metadata=%s", result.raw)
			}
		} else if len(result.body) != 1 {
			t.Fatalf("public metadata leaked private state: %s", result.raw)
		}
	}
	data := photoPNG(t)
	clubImageFailure(t, clubImageUpload(t, f, clubID, "1", data, nil, true), 401, "authentication_required")
	clubImageFailure(t, clubImageUpload(t, f, clubID, "1", data, owner, false), 403, "invalid_csrf")
	clubImageFailure(t, f.request("DELETE", metadata, map[string]any{"version": 1}, nil), 401, "authentication_required")
	wrongCSRF := *owner
	wrongCSRF.csrf = ""
	clubImageFailure(t, f.request("DELETE", metadata, map[string]any{"version": 1}, &wrongCSRF), 403, "invalid_csrf")
	clubImageFailure(t, clubImageUpload(t, f, clubID, "1", data, member, true), 403, "forbidden")
	expect(t, f.request("PUT", base+"/membership", nil, member), 200)
	expect(t, clubDecision(f, clubID, owner, member, "approve"), 200)
	clubImageFailure(t, clubImageUpload(t, f, clubID, "1", data, member, true), 403, "forbidden")
	clubImageFailure(t, f.request("DELETE", metadata, map[string]any{"version": 1}, member), 403, "forbidden")
	for _, version := range []string{"", "0", "-1", "abc", "1.5"} {
		clubImageFailure(t, clubImageUpload(t, f, clubID, version, data, owner, true), 422, "invalid_photo_choice")
	}
	first := clubImageUpload(t, f, clubID, "1", data, owner, true)
	expect(t, first, 201)
	firstImage := first.body["image"].(map[string]any)
	if len(first.body) != 2 || first.body["version"] != float64(2) || len(firstImage) != 5 || firstImage["width"] != float64(120) || firstImage["height"] != float64(80) {
		t.Fatalf("uploaded metadata leaked storage fields or changed safe dimensions: %s", first.raw)
	}
	firstID, firstURL := firstImage["id"].(string), firstImage["url"].(string)
	if firstURL != metadata+"/"+firstID+"/content" {
		t.Fatalf("content URL is not scoped to its club: %s", firstURL)
	}
	firstBytes := photoRead(t, f, firstURL, nil, "GET", 200)
	photoRead(t, f, firstURL, nil, "HEAD", 200)
	for _, viewer := range []*actor{nil, member} {
		result := f.request("GET", metadata, nil, viewer)
		expect(t, result, 200)
		if len(result.body) != 1 || len(result.body["image"].(map[string]any)) != 5 {
			t.Fatalf("public club image projection changed: %s", result.raw)
		}
		publicClub := f.request("GET", base, nil, nil)
		expect(t, publicClub, 200)
		if len(publicClub.body["club"].(map[string]any)) != 5 {
			t.Fatal("image upload changed existing public club DTO")
		}
	}
	// Conditional requests still authorize and return the processed image.
	req, _ := http.NewRequest("GET", f.ts.URL+firstURL, nil)
	req.Header.Set("If-None-Match", "*")
	req.Header.Set("If-Modified-Since", "Wed, 01 Jan 2100 00:00:00 GMT")
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	conditional, readErr := io.ReadAll(r.Body)
	_ = r.Body.Close()
	if readErr != nil || r.StatusCode != 200 || r.Header.Get("ETag") != "" || r.Header.Get("Last-Modified") != "" || r.Header.Get("Content-Length") != strconv.Itoa(len(firstBytes)) || !bytes.Equal(conditional, firstBytes) {
		t.Fatalf("conditional image response bypassed content contract: status=%d read=%v", r.StatusCode, readErr)
	}
	clubImageFailure(t, clubImageUpload(t, f, clubID, "1", data, owner, true), 409, "stale_version")
	clubImageFailure(t, clubImageUpload(t, f, clubID, "2", []byte{0x89, 'P', 'N', 'G'}, owner, true), 422, "invalid_photo")
	clubImageFailure(t, f.request("DELETE", metadata, map[string]any{"version": 1}, owner), 409, "stale_version")
	expect(t, f.request("DELETE", metadata, map[string]any{"version": 2, "unexpected": true}, owner), 400)
	if after := photoRead(t, f, firstURL, nil, "GET", 200); !bytes.Equal(after, firstBytes) {
		t.Fatal("invalid or stale replacement changed ready image")
	}
	unchanged := f.request("GET", metadata, nil, owner)
	if unchanged.body["version"] != float64(2) || unchanged.body["image"].(map[string]any)["id"] != firstID {
		t.Fatalf("failed replacement changed metadata: %s", unchanged.raw)
	}
	second := clubImageUpload(t, f, clubID, "2", data, owner, true)
	expect(t, second, 201)
	secondImage := second.body["image"].(map[string]any)
	if second.body["version"] != float64(3) || secondImage["id"] == firstID {
		t.Fatalf("replacement did not atomically advance image identity/version: %s", second.raw)
	}
	photoRead(t, f, firstURL, nil, "GET", 404)
	photoRead(t, f, secondImage["url"].(string), nil, "GET", 200)
	var ready, pending int
	if err := f.db.QueryRow(context.Background(), "SELECT count(*) FILTER(WHERE state='ready'),count(*) FILTER(WHERE state='pending') FROM club_images WHERE club_id=$1", clubID).Scan(&ready, &pending); err != nil || ready != 1 || pending != 0 {
		t.Fatalf("replacement retained old or pending content: ready=%d pending=%d err=%v", ready, pending, err)
	}
	removed := f.request("DELETE", metadata, map[string]any{"version": 3}, owner)
	expect(t, removed, 200)
	if removed.body["image"] != nil || removed.body["version"] != float64(4) {
		t.Fatalf("remove did not restore initial state: %s", removed.raw)
	}
	photoRead(t, f, secondImage["url"].(string), nil, "GET", 404)
	third := clubImageUpload(t, f, clubID, "4", data, owner, true)
	expect(t, third, 201)
	thirdImage := third.body["image"].(map[string]any)
	thirdBytes := photoRead(t, f, thirdImage["url"].(string), nil, "GET", 200)
	f.startServer(f.gateway)
	restarted := f.request("GET", metadata, nil, owner)
	expect(t, restarted, 200)
	if restarted.body["version"] != float64(5) || restarted.body["image"].(map[string]any)["id"] != thirdImage["id"] || !bytes.Equal(photoRead(t, f, thirdImage["url"].(string), nil, "GET", 200), thirdBytes) {
		t.Fatal("application replacement lost current club image or version")
	}
	expect(t, f.request("GET", "/api/v1/clubs/"+uuid.NewString()+"/image", nil, nil), 404)
	photoRead(t, f, metadata+"/"+uuid.NewString()+"/content", nil, "GET", 404)
}

func TestClubImageSurvivesTransferAndRemainsPublicWhenClosed(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011234811", "Original owner")
	next := f.login("+77011234812", "Next owner")
	clubID := testClub(t, f, owner)["id"].(string)
	base := "/api/v1/clubs/" + clubID
	metadata := base + "/image"
	expect(t, f.request("PUT", base+"/membership", nil, next), 200)
	expect(t, clubDecision(f, clubID, owner, next, "approve"), 200)
	uploaded := clubImageUpload(t, f, clubID, "1", photoPNG(t), owner, true)
	expect(t, uploaded, 201)
	image := uploaded.body["image"].(map[string]any)
	imageURL := image["url"].(string)
	before := photoRead(t, f, imageURL, nil, "GET", 200)
	expect(t, f.request("PUT", base+"/ownership", map[string]any{"version": 1, "user_id": next.id}, owner), 200)
	var chargedTo string
	if err := f.db.QueryRow(context.Background(), "SELECT owner_id::text FROM club_images WHERE id=$1", image["id"]).Scan(&chargedTo); err != nil || chargedTo != owner.id {
		t.Fatalf("transfer changed immutable uploader accounting: owner=%s err=%v", chargedTo, err)
	}
	current := f.request("GET", metadata, nil, next)
	expect(t, current, 200)
	if current.body["version"] != float64(2) || current.body["image"].(map[string]any)["id"] != image["id"] {
		t.Fatal("transfer lost current image or separate image version")
	}
	former := f.request("GET", metadata, nil, owner)
	expect(t, former, 200)
	if len(former.body) != 1 {
		t.Fatal("former owner retained image mutation version")
	}
	clubImageFailure(t, clubImageUpload(t, f, clubID, "2", photoPNG(t), owner, true), 403, "forbidden")
	clubImageFailure(t, f.request("DELETE", metadata, map[string]any{"version": 2}, owner), 403, "forbidden")
	expect(t, f.request("POST", base+"/close", map[string]any{"version": 2}, next), 200)
	for _, viewer := range []*actor{nil, owner, next} {
		result := f.request("GET", metadata, nil, viewer)
		expect(t, result, 200)
		if len(result.body) != 1 || result.body["image"].(map[string]any)["id"] != image["id"] {
			t.Fatalf("closed public image gained private owner access: %s", result.raw)
		}
		if !bytes.Equal(photoRead(t, f, imageURL, viewer, "GET", 200), before) {
			t.Fatal("transfer or closure changed public image bytes")
		}
		photoRead(t, f, imageURL, viewer, "HEAD", 200)
	}
	clubImageFailure(t, clubImageUpload(t, f, clubID, "2", photoPNG(t), next, true), 409, "club_closed")
	clubImageFailure(t, f.request("DELETE", metadata, map[string]any{"version": 2}, next), 409, "club_closed")
	expect(t, f.request("DELETE", base+"/membership", nil, next), 200)
	historical := f.request("GET", base, nil, next)
	expect(t, historical, 200)
	if _, leaked := historical.body["club"].(map[string]any)["owner_id"]; leaked {
		t.Fatal("historical closed owner inherited private club projection")
	}
	if result := f.request("GET", metadata, nil, next); len(result.body) != 1 || !bytes.Equal(photoRead(t, f, imageURL, nil, "GET", 200), before) {
		t.Fatal("leaving closed club changed its public basic-page image")
	}
}

type clubImageGatedReader struct {
	reader *bytes.Reader
	gate   <-chan struct{}
	ctx    context.Context
}

func (r *clubImageGatedReader) Read(p []byte) (int, error) {
	if r.reader.Len() > 0 {
		return r.reader.Read(p)
	}
	select {
	case <-r.gate:
		return 0, io.EOF
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	}
}

type clubImageHTTPResult struct {
	response response
	err      error
}

func startGatedImageUpload(t *testing.T, f *fixture, path string, data []byte, owner *actor) (func(), <-chan clubImageHTTPResult) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	gate := make(chan struct{})
	var once sync.Once
	release := func() { once.Do(func() { close(gate) }) }
	t.Cleanup(func() { release(); cancel() })
	req, err := http.NewRequestWithContext(ctx, "POST", f.ts.URL+path, &clubImageGatedReader{reader: bytes.NewReader(data), gate: gate, ctx: ctx})
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", f.ts.URL)
	req.Header.Set("Content-Type", "image/png")
	req.Header.Set("X-CSRF-Token", owner.csrf)
	req.Header.Set("Expect", "100-continue")
	req.AddCookie(owner.cookie)
	finished := make(chan clubImageHTTPResult, 1)
	go func() {
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			finished <- clubImageHTTPResult{err: err}
			return
		}
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		result := response{status: r.StatusCode, raw: string(body)}
		_ = json.Unmarshal(body, &result.body)
		finished <- clubImageHTTPResult{response: result, err: err}
	}()
	return release, finished
}

func waitImageReservation(t *testing.T, f *fixture, table, scopeColumn, id string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var count int
		// Table and column names are fixed test literals, never application input.
		err := f.db.QueryRow(context.Background(), fmt.Sprintf("SELECT count(*) FROM %s WHERE %s=$1 AND state='pending'", table, scopeColumn), id).Scan(&count)
		if err != nil {
			t.Fatal(err)
		}
		if count == 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("upload never reserved before reading the blocked raw body")
}

func finishImageUpload(t *testing.T, release func(), finished <-chan clubImageHTTPResult) response {
	t.Helper()
	release()
	select {
	case result := <-finished:
		if result.err != nil {
			t.Fatal(result.err)
		}
		return result.response
	case <-time.After(5 * time.Second):
		t.Fatal("upload did not finish after releasing its raw body")
		return response{}
	}
}

func TestClubImagePendingReplacementRejectsTransferABAAndClosure(t *testing.T) {
	for _, transition := range []string{"transfer", "aba", "close"} {
		t.Run(transition, func(t *testing.T) {
			f := newFixture(t)
			owner := f.login("+77011234821", "Uploading owner")
			next := f.login("+77011234822", "Next owner")
			clubID := testClub(t, f, owner)["id"].(string)
			base := "/api/v1/clubs/" + clubID
			metadata := base + "/image"
			expect(t, f.request("PUT", base+"/membership", nil, next), 200)
			expect(t, clubDecision(f, clubID, owner, next, "approve"), 200)
			data := photoPNG(t)
			initial := clubImageUpload(t, f, clubID, "1", data, owner, true)
			expect(t, initial, 201)
			image := initial.body["image"].(map[string]any)
			before := photoRead(t, f, image["url"].(string), nil, "GET", 200)
			release, finished := startGatedImageUpload(t, f, metadata+"?version=2", data, owner)
			waitImageReservation(t, f, "club_images", "club_id", clubID)
			pendingView := f.request("GET", metadata, nil, owner)
			expect(t, pendingView, 200)
			if pendingView.body["version"] != float64(2) || pendingView.body["image"].(map[string]any)["id"] != image["id"] {
				t.Fatal("pending replacement changed the current image")
			}
			clubImageFailure(t, clubImageUpload(t, f, clubID, "2", data, owner, true), 409, "club_image_busy")
			want := "forbidden"
			if transition == "close" {
				expect(t, f.request("POST", base+"/close", map[string]any{"version": 1}, owner), 200)
				want = "club_closed"
			} else {
				expect(t, f.request("PUT", base+"/ownership", map[string]any{"version": 1, "user_id": next.id}, owner), 200)
				if transition == "aba" {
					expect(t, f.request("PUT", base+"/ownership", map[string]any{"version": 2, "user_id": owner.id}, next), 200)
					want = "stale_version"
				}
			}
			status := 409
			if want == "forbidden" {
				status = 403
			}
			clubImageFailure(t, finishImageUpload(t, release, finished), status, want)
			current := f.request("GET", metadata, nil, nil)
			expect(t, current, 200)
			if current.body["image"].(map[string]any)["id"] != image["id"] || !bytes.Equal(photoRead(t, f, image["url"].(string), nil, "GET", 200), before) {
				t.Fatal("rejected finalization destroyed old ready image")
			}
			var pending, ready int
			if err := f.db.QueryRow(context.Background(), "SELECT count(*) FILTER(WHERE state='pending'),count(*) FILTER(WHERE state='ready') FROM club_images WHERE club_id=$1", clubID).Scan(&pending, &ready); err != nil || pending != 0 || ready != 1 {
				t.Fatalf("race left pending reservation or lost ready content: pending=%d ready=%d err=%v", pending, ready, err)
			}
		})
	}
}
