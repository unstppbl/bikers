package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func rideCoverUpload(t *testing.T, f *fixture, rideID, version string, data []byte, a *actor, csrf bool) response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, f.ts.URL+"/api/v1/rides/"+rideID+"/cover?version="+version, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", f.ts.URL)
	req.Header.Set("Content-Type", "image/png")
	if a != nil {
		req.AddCookie(a.cookie)
		if csrf {
			req.Header.Set("X-CSRF-Token", a.csrf)
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
		t.Fatal("ride cover metadata response may be cached")
	}
	return result
}

// rideCard returns the listed summary for one ride, or nil when absent.
func rideCard(t *testing.T, result response, rideID string) map[string]any {
	t.Helper()
	for _, item := range result.body["rides"].([]any) {
		if card := item.(map[string]any); card["id"] == rideID {
			return card
		}
	}
	return nil
}

func coverID(value map[string]any) any {
	cover, _ := value["cover"].(map[string]any)
	return cover["id"]
}

func approvedCoverRide(t *testing.T, f *fixture) (owner, rider, outsider *actor, rideID string) {
	t.Helper()
	owner = f.login("+77011234951", "Cover organizer")
	rider = f.login("+77011234952", "Approved rider")
	outsider = f.login("+77011234953", "Outsider")
	rideID = f.ride(owner, 4)
	f.join(rideID, rider)
	expect(t, f.decision(rideID, owner, rider, "approve"), 200)
	return owner, rider, outsider, rideID
}

func TestRideCoverOrganizerUploadsPublicCoverAcrossRideShapes(t *testing.T) {
	f := newFixture(t)
	owner, rider, outsider, rideID := approvedCoverRide(t, f)
	base := "/api/v1/rides/" + rideID
	metadata := base + "/cover"
	for _, viewer := range []*actor{nil, rider, outsider, owner} {
		result := f.request("GET", metadata, nil, viewer)
		expect(t, result, 200)
		if result.body["image"] != nil {
			t.Fatal("new ride has an unexpected cover")
		}
		if viewer == owner {
			if len(result.body) != 2 || result.body["version"] != float64(1) {
				t.Fatalf("organizer initial cover metadata=%s", result.raw)
			}
		} else if len(result.body) != 1 {
			t.Fatalf("public cover metadata leaked the version: %s", result.raw)
		}
	}
	organizer := f.request("GET", base, nil, owner).body["ride"].(map[string]any)
	if organizer["cover_version"] != float64(1) || organizer["cover"] != nil {
		t.Fatalf("organizer ride lacks the initial cover version: %v", organizer)
	}
	if _, leaked := f.request("GET", base, nil, rider).body["ride"].(map[string]any)["cover_version"]; leaked {
		t.Fatal("participant received the cover version")
	}
	data := photoPNG(t)
	clubImageFailure(t, rideCoverUpload(t, f, rideID, "1", data, nil, true), 401, "authentication_required")
	clubImageFailure(t, rideCoverUpload(t, f, rideID, "1", data, owner, false), 403, "invalid_csrf")
	clubImageFailure(t, f.request("DELETE", metadata, map[string]any{"version": 1}, nil), 401, "authentication_required")
	for _, other := range []*actor{rider, outsider} {
		clubImageFailure(t, rideCoverUpload(t, f, rideID, "1", data, other, true), 403, "forbidden")
		clubImageFailure(t, f.request("DELETE", metadata, map[string]any{"version": 1}, other), 403, "forbidden")
	}
	for _, version := range []string{"", "0", "-1", "abc", "1.5", "01"} {
		clubImageFailure(t, rideCoverUpload(t, f, rideID, version, data, owner, true), 422, "invalid_photo_choice")
	}
	clubImageFailure(t, f.request("DELETE", metadata, map[string]any{"version": 0}, owner), 422, "invalid_photo_choice")
	first := rideCoverUpload(t, f, rideID, "1", data, owner, true)
	expect(t, first, 201)
	firstImage := first.body["image"].(map[string]any)
	if len(first.body) != 2 || first.body["version"] != float64(2) || len(firstImage) != 5 || firstImage["width"] != float64(120) || firstImage["height"] != float64(80) {
		t.Fatalf("uploaded cover leaked storage fields or changed safe dimensions: %s", first.raw)
	}
	firstID, firstURL := firstImage["id"].(string), firstImage["url"].(string)
	if firstURL != metadata+"/"+firstID+"/content" {
		t.Fatalf("content URL is not scoped to its ride: %s", firstURL)
	}
	firstBytes := photoRead(t, f, firstURL, nil, "GET", 200)
	photoRead(t, f, firstURL, nil, "HEAD", 200)
	for _, viewer := range []*actor{rider, outsider, owner} {
		if !bytes.Equal(photoRead(t, f, firstURL, viewer, "GET", 200), firstBytes) {
			t.Fatal("cover bytes differ by viewer")
		}
	}

	// Every ride shape carries the same public cover without private details.
	for _, viewer := range []*actor{nil, outsider} {
		feed := f.request("GET", "/api/v1/rides", nil, viewer)
		expect(t, feed, 200)
		card := rideCard(t, feed, rideID)
		if card == nil || coverID(card) != firstID || len(card["cover"].(map[string]any)) != 5 || strings.Contains(feed.raw, "SECRET") {
			t.Fatalf("feed card cover=%v", card)
		}
		preview := f.request("GET", base, nil, viewer)
		expect(t, preview, 200)
		summary := preview.body["ride"].(map[string]any)
		if coverID(summary) != firstID || strings.Contains(preview.raw, "SECRET") || strings.Contains(preview.raw, "cover_version") {
			t.Fatalf("public preview cover=%s", preview.raw)
		}
		if viewer == nil && len(summary) != 6 {
			t.Fatalf("anonymous preview changed beyond the cover: %s", preview.raw)
		}
	}
	participant := f.request("GET", base, nil, rider).body["ride"].(map[string]any)
	if coverID(participant) != firstID || participant["meeting_point"] != "SECRET MEETING" {
		t.Fatalf("participant ride=%v", participant)
	}
	organizer = f.request("GET", base, nil, owner).body["ride"].(map[string]any)
	if coverID(organizer) != firstID || organizer["cover_version"] != float64(2) {
		t.Fatalf("organizer ride=%v", organizer)
	}
	clubID := testClub(t, f, owner)["id"].(string)
	expect(t, f.request("PUT", "/api/v1/clubs/"+clubID+"/rides/"+rideID, nil, owner), 200)
	linked := f.request("GET", "/api/v1/clubs/"+clubID+"/rides", nil, nil)
	expect(t, linked, 200)
	if card := rideCard(t, linked, rideID); card == nil || coverID(card) != firstID || strings.Contains(linked.raw, "SECRET") {
		t.Fatalf("club linked ride cover=%s", linked.raw)
	}

	clubImageFailure(t, rideCoverUpload(t, f, rideID, "1", data, owner, true), 409, "stale_version")
	clubImageFailure(t, rideCoverUpload(t, f, rideID, "2", []byte{0x89, 'P', 'N', 'G'}, owner, true), 422, "invalid_photo")
	clubImageFailure(t, f.request("DELETE", metadata, map[string]any{"version": 1}, owner), 409, "stale_version")
	expect(t, f.request("DELETE", metadata, map[string]any{"version": 2, "unexpected": true}, owner), 400)
	if after := photoRead(t, f, firstURL, nil, "GET", 200); !bytes.Equal(after, firstBytes) {
		t.Fatal("invalid or stale replacement changed the ready cover")
	}
	second := rideCoverUpload(t, f, rideID, "2", data, owner, true)
	expect(t, second, 201)
	secondImage := second.body["image"].(map[string]any)
	if second.body["version"] != float64(3) || secondImage["id"] == firstID {
		t.Fatalf("replacement did not advance cover identity/version: %s", second.raw)
	}
	photoRead(t, f, firstURL, nil, "GET", 404)
	photoRead(t, f, secondImage["url"].(string), nil, "GET", 200)
	var ready, pending int
	if err := f.db.QueryRow(context.Background(), "SELECT count(*) FILTER(WHERE state='ready'),count(*) FILTER(WHERE state='pending') FROM ride_covers WHERE ride_id=$1", rideID).Scan(&ready, &pending); err != nil || ready != 1 || pending != 0 {
		t.Fatalf("replacement retained old or pending content: ready=%d pending=%d err=%v", ready, pending, err)
	}
	removed := f.request("DELETE", metadata, map[string]any{"version": 3}, owner)
	expect(t, removed, 200)
	if removed.body["image"] != nil || removed.body["version"] != float64(4) {
		t.Fatalf("remove did not restore the city card state: %s", removed.raw)
	}
	photoRead(t, f, secondImage["url"].(string), nil, "GET", 404)
	if card := rideCard(t, f.request("GET", "/api/v1/rides", nil, nil), rideID); card == nil || card["cover"] != nil {
		t.Fatalf("feed kept a removed cover: %v", card)
	}
	third := rideCoverUpload(t, f, rideID, "4", data, owner, true)
	expect(t, third, 201)
	thirdImage := third.body["image"].(map[string]any)
	thirdBytes := photoRead(t, f, thirdImage["url"].(string), nil, "GET", 200)
	f.startServer(f.gateway)
	restarted := f.request("GET", metadata, nil, owner)
	expect(t, restarted, 200)
	if restarted.body["version"] != float64(5) || restarted.body["image"].(map[string]any)["id"] != thirdImage["id"] || !bytes.Equal(photoRead(t, f, thirdImage["url"].(string), nil, "GET", 200), thirdBytes) {
		t.Fatal("application restart lost the current cover or version")
	}
	expect(t, f.request("GET", "/api/v1/rides/"+uuid.NewString()+"/cover", nil, nil), 404)
	clubImageFailure(t, rideCoverUpload(t, f, uuid.NewString(), "1", data, owner, true), 404, "not_found")
	photoRead(t, f, metadata+"/"+uuid.NewString()+"/content", nil, "GET", 404)
}

func coverImageID(result response) any {
	image, _ := result.body["image"].(map[string]any)
	return image["id"]
}

// expectCoverReads checks the cover metadata status and both content methods
// for one viewer.
func expectCoverReads(t *testing.T, f *fixture, rideID, coverID string, viewer *actor, metadata, content int) response {
	t.Helper()
	result := f.request("GET", "/api/v1/rides/"+rideID+"/cover", nil, viewer)
	expect(t, result, metadata)
	url := "/api/v1/rides/" + rideID + "/cover/" + coverID + "/content"
	for _, method := range []string{"GET", "HEAD"} {
		photoRead(t, f, url, viewer, method, content)
	}
	return result
}

// expectCoverWritesRefused checks that a viewer who is not the organizer
// cannot upload or remove the cover at its current version.
func expectCoverWritesRefused(t *testing.T, f *fixture, rideID string, viewer *actor, version int) {
	t.Helper()
	status, code := 403, "forbidden"
	if viewer == nil {
		status, code = 401, "authentication_required"
	}
	clubImageFailure(t, rideCoverUpload(t, f, rideID, strconv.Itoa(version), photoPNG(t), viewer, true), status, code)
	clubImageFailure(t, f.request("DELETE", "/api/v1/rides/"+rideID+"/cover", map[string]any{"version": version}, viewer), status, code)
}

func TestRideCoverVisibilityFollowsModeratedRideAndHiddenCover(t *testing.T) {
	f := newFixture(t)
	owner, rider, outsider, rideID := approvedCoverRide(t, f)
	admin := f.login("+77011234954", "Moderator")
	grantTestAdmin(t, f, admin)
	pending := f.login("+77011234955", "Pending rider")
	f.join(rideID, pending)
	rejected := f.login("+77011234956", "Rejected rider")
	f.join(rideID, rejected)
	expect(t, f.decision(rideID, owner, rejected, "reject"), 200)
	// riders marks the viewers who keep the cover of a moderated ride.
	viewers := []struct {
		name   string
		actor  *actor
		riders bool
	}{
		{"guest", nil, false}, {"blocked outsider", outsider, false}, {"pending rider", pending, false},
		{"rejected rider", rejected, false}, {"unrelated admin", admin, false},
		{"approved rider", rider, true}, {"organizer", owner, true},
	}
	base := "/api/v1/rides/" + rideID
	uploaded := rideCoverUpload(t, f, rideID, "1", photoPNG(t), owner, true)
	expect(t, uploaded, 201)
	image := uploaded.body["image"].(map[string]any)
	id, url := image["id"].(string), image["url"].(string)
	before := photoRead(t, f, url, nil, "GET", 200)
	expect(t, f.request("PUT", "/api/v1/me/blocks/"+outsider.id, nil, owner), 200)

	// A visible ride shows its cover to everyone; only the organizer edits it.
	for _, viewer := range viewers {
		result := expectCoverReads(t, f, rideID, id, viewer.actor, 200, 200)
		fields := 1
		if viewer.actor == owner {
			fields = 2
		}
		if coverImageID(result) != id || len(result.body) != fields {
			t.Fatalf("%s: visible ride cover metadata=%s", viewer.name, result.raw)
		}
		ride := f.request("GET", base, nil, viewer.actor)
		expect(t, ride, 200)
		if coverID(ride.body["ride"].(map[string]any)) != id {
			t.Fatalf("%s: visible ride shape lacks its cover: %s", viewer.name, ride.raw)
		}
		if viewer.actor != owner {
			expectCoverWritesRefused(t, f, rideID, viewer.actor, 2)
		}
	}
	reported := trustReport(f, outsider, "ride_cover", id)
	expect(t, reported, 201)
	reportID := reported.body["report"].(map[string]any)["id"].(string)
	if reviewed := photoRead(t, f, "/api/v1/admin/reports/"+reportID+"/content", admin, "GET", 200); !bytes.Equal(reviewed, before) {
		t.Fatal("review did not return the reported cover")
	}

	// A moderated ride keeps its cover only for the organizer and approved riders.
	expect(t, trustAction(f, admin, "hide_content", "ride", rideID, "Hide abusive ride", ""), 200)
	for _, viewer := range viewers {
		if viewer.riders {
			if result := expectCoverReads(t, f, rideID, id, viewer.actor, 200, 200); coverImageID(result) != id {
				t.Fatalf("%s: moderated ride hid its cover from its riders: %s", viewer.name, result.raw)
			}
			expect(t, f.request("GET", base, nil, viewer.actor), 200)
		} else {
			expectCoverReads(t, f, rideID, id, viewer.actor, 404, 404)
			expect(t, f.request("GET", base, nil, viewer.actor), 404)
			if viewer.actor != nil {
				expect(t, trustReport(f, viewer.actor, "ride_cover", id), 404)
			}
		}
		if viewer.actor != owner {
			expectCoverWritesRefused(t, f, rideID, viewer.actor, 2)
		}
	}
	if after := photoRead(t, f, url, owner, "GET", 200); !bytes.Equal(after, before) {
		t.Fatal("refused writes changed the moderated ride cover")
	}
	expect(t, trustAction(f, admin, "restore_content", "ride", rideID, "Ride restored", ""), 200)

	// A moderated cover is served to no one; only the organizer keeps its metadata.
	expect(t, trustAction(f, admin, "hide_content", "ride_cover", id, "Hide abusive cover", ""), 200)
	for _, viewer := range viewers {
		result := expectCoverReads(t, f, rideID, id, viewer.actor, 200, 404)
		if viewer.actor == owner {
			if coverImageID(result) != id || result.body["version"] != float64(2) {
				t.Fatalf("organizer cannot replace a moderated cover: %s", result.raw)
			}
		} else if result.body["image"] != nil || len(result.body) != 1 {
			t.Fatalf("%s: moderated cover served to others: %s", viewer.name, result.raw)
		}
		if ride := f.request("GET", base, nil, viewer.actor); ride.body["ride"].(map[string]any)["cover"] != nil {
			t.Fatalf("%s: ride shape kept a moderated cover: %s", viewer.name, ride.raw)
		}
	}
	if card := rideCard(t, f.request("GET", "/api/v1/rides", nil, nil), rideID); card == nil || card["cover"] != nil {
		t.Fatalf("feed kept a moderated cover: %v", card)
	}
	expect(t, trustReport(f, rider, "ride_cover", id), 404)
	var length int
	if err := f.db.QueryRow(context.Background(), `SELECT octet_length(content) FROM ride_covers WHERE id=$1`, id).Scan(&length); err != nil || length != len(before) {
		t.Fatalf("moderation deleted the original cover: %d %v", length, err)
	}
	replaced := rideCoverUpload(t, f, rideID, "2", photoPNG(t), owner, true)
	expect(t, replaced, 201)
	fresh := replaced.body["image"].(map[string]any)
	photoRead(t, f, fresh["url"].(string), nil, "GET", 200)
	if card := rideCard(t, f.request("GET", "/api/v1/rides", nil, nil), rideID); coverID(card) != fresh["id"] {
		t.Fatalf("replacement cover is not public: %v", card)
	}
}

func TestRideCoverStaysPublicOnCancelledRideAndRejectsChanges(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011234961", "Cancelling organizer")
	rideID := f.ride(owner, 3)
	base := "/api/v1/rides/" + rideID
	metadata := base + "/cover"
	uploaded := rideCoverUpload(t, f, rideID, "1", photoPNG(t), owner, true)
	expect(t, uploaded, 201)
	image := uploaded.body["image"].(map[string]any)
	plan := f.request("GET", base, nil, owner).body["ride"].(map[string]any)["plan_version"]
	expect(t, f.request("POST", base+"/cancel", map[string]any{"plan_version": plan}, owner), 200)
	current := f.request("GET", metadata, nil, owner)
	expect(t, current, 200)
	if len(current.body) != 1 || current.body["image"].(map[string]any)["id"] != image["id"] {
		t.Fatalf("cancelled ride offered cover changes: %s", current.raw)
	}
	organizer := f.request("GET", base, nil, owner).body["ride"].(map[string]any)
	if _, ok := organizer["cover_version"]; ok || coverID(organizer) != image["id"] {
		t.Fatalf("cancelled organizer ride=%v", organizer)
	}
	clubImageFailure(t, rideCoverUpload(t, f, rideID, "2", photoPNG(t), owner, true), 409, "ride_cancelled")
	clubImageFailure(t, f.request("DELETE", metadata, map[string]any{"version": 2}, owner), 409, "ride_cancelled")
	photoRead(t, f, image["url"].(string), nil, "GET", 200)
	past := f.request("GET", "/api/v1/rides?view=past", nil, owner)
	expect(t, past, 200)
	if card := rideCard(t, past, rideID); coverID(card) != image["id"] {
		t.Fatalf("past ride card lost its cover: %s", past.raw)
	}
}

func TestRideCoverStaysEditableAfterPublishedRideStarts(t *testing.T) {
	f := newFixture(t)
	owner, rider, outsider, rideID := approvedCoverRide(t, f)
	base := "/api/v1/rides/" + rideID
	metadata := base + "/cover"
	first := rideCoverUpload(t, f, rideID, "1", photoPNG(t), owner, true)
	expect(t, first, 201)
	firstImage := first.body["image"].(map[string]any)
	// The fixture ride departs at 2026-11-07T23:30Z.
	f.now = time.Date(2026, 11, 8, 6, 0, 0, 0, time.UTC)
	organizer := f.request("GET", base, nil, owner)
	expect(t, organizer, 200)
	if ride := organizer.body["ride"].(map[string]any); ride["status"] != "published" || ride["cover_version"] != float64(2) || coverID(ride) != firstImage["id"] {
		t.Fatalf("departed organizer ride lost its cover version: %s", organizer.raw)
	}
	if current := f.request("GET", metadata, nil, owner); current.body["version"] != float64(2) || coverImageID(current) != firstImage["id"] {
		t.Fatalf("departed ride stopped offering cover changes: %s", current.raw)
	}
	for _, other := range []*actor{nil, rider, outsider} {
		expectCoverWritesRefused(t, f, rideID, other, 2)
	}
	second := rideCoverUpload(t, f, rideID, "2", photoPNG(t), owner, true)
	expect(t, second, 201)
	secondImage := second.body["image"].(map[string]any)
	secondID := secondImage["id"].(string)
	if second.body["version"] != float64(3) || secondID == firstImage["id"] {
		t.Fatalf("departed ride replacement=%s", second.raw)
	}
	photoRead(t, f, firstImage["url"].(string), nil, "GET", 404)
	for _, viewer := range []*actor{nil, outsider, rider, owner} {
		if result := expectCoverReads(t, f, rideID, secondID, viewer, 200, 200); coverImageID(result) != secondID {
			t.Fatalf("departed ride cover metadata=%s", result.raw)
		}
		ride := f.request("GET", base, nil, viewer)
		expect(t, ride, 200)
		if coverID(ride.body["ride"].(map[string]any)) != secondID {
			t.Fatalf("departed ride shape lacks its cover: %s", ride.raw)
		}
	}
	for _, viewer := range []*actor{owner, rider} {
		past := f.request("GET", "/api/v1/rides?view=past", nil, viewer)
		expect(t, past, 200)
		if coverID(rideCard(t, past, rideID)) != secondID {
			t.Fatalf("past ride card lacks the replacement cover: %s", past.raw)
		}
	}
	removed := f.request("DELETE", metadata, map[string]any{"version": 3}, owner)
	expect(t, removed, 200)
	if removed.body["image"] != nil || removed.body["version"] != float64(4) {
		t.Fatalf("departed ride removal=%s", removed.raw)
	}
	photoRead(t, f, secondImage["url"].(string), nil, "GET", 404)
	if card := rideCard(t, f.request("GET", "/api/v1/rides?view=past", nil, owner), rideID); card == nil || card["cover"] != nil {
		t.Fatalf("past ride card kept a removed cover: %v", card)
	}
	if ride := f.request("GET", base, nil, owner).body["ride"].(map[string]any); ride["cover_version"] != float64(4) || ride["cover"] != nil {
		t.Fatalf("departed organizer ride after removal=%v", ride)
	}
}

func TestRideCoverPendingReservationIsBusyAndSharesProcessingSlots(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011234971", "Cover uploader")
	bikeOwner := f.login("+77011234972", "Bike uploader")
	rideID := f.ride(owner, 3)
	otherRide := f.ride(owner, 3)
	clubID := testClub(t, f, owner)["id"].(string)
	bikeID := testBike(t, f, bikeOwner)["id"].(string)
	data := photoPNG(t)
	releaseCover, coverFinished := startGatedImageUpload(t, f, "/api/v1/rides/"+rideID+"/cover?version=1", data, owner)
	waitImageReservation(t, f, "ride_covers", "ride_id", rideID)
	pendingView := f.request("GET", "/api/v1/rides/"+rideID+"/cover", nil, owner)
	if pendingView.body["image"] != nil || pendingView.body["version"] != float64(1) {
		t.Fatalf("pending upload changed the current cover: %s", pendingView.raw)
	}
	clubImageFailure(t, rideCoverUpload(t, f, rideID, "1", data, owner, true), 409, "ride_cover_busy")
	releaseBike, bikeFinished := startGatedImageUpload(t, f, "/api/v1/me/bikes/"+bikeID+"/photos", data, bikeOwner)
	waitImageReservation(t, f, "bike_photos", "bike_id", bikeID)
	clubImageFailure(t, rideCoverUpload(t, f, otherRide, "1", data, owner, true), 429, "photo_upload_busy")
	clubImageFailure(t, clubImageUpload(t, f, clubID, "1", data, owner, true), 429, "photo_upload_busy")
	expect(t, finishImageUpload(t, releaseCover, coverFinished), 201)
	expect(t, finishImageUpload(t, releaseBike, bikeFinished), 201)
	var pending int
	if err := f.db.QueryRow(context.Background(), "SELECT count(*) FROM ride_covers WHERE state='pending'").Scan(&pending); err != nil || pending != 0 {
		t.Fatalf("finished upload left a pending cover: %d %v", pending, err)
	}
}

func TestRideCoverFailedAttemptsShareTheHourlyLimit(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011234981", "Shared attempt organizer")
	rideID := f.ride(owner, 3)
	clubID := testClub(t, f, owner)["id"].(string)
	malformed := []byte{0x89, 'P', 'N', 'G'}
	for range 6 {
		expect(t, rideCoverUpload(t, f, rideID, "1", malformed, owner, true), 422)
		expect(t, clubImageUpload(t, f, clubID, "1", malformed, owner, true), 422)
	}
	clubImageFailure(t, rideCoverUpload(t, f, rideID, "1", photoPNG(t), owner, true), 429, "photo_rate_limited")
	clubImageFailure(t, clubImageUpload(t, f, clubID, "1", photoPNG(t), owner, true), 429, "photo_rate_limited")
	var attempts, reservations int
	if err := f.db.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM photo_upload_attempts WHERE owner_id=$1),(SELECT count(*) FROM ride_covers WHERE owner_id=$1)`, owner.id).Scan(&attempts, &reservations); err != nil || attempts != 12 || reservations != 0 {
		t.Fatalf("cover attempts=%d reservations=%d err=%v", attempts, reservations, err)
	}
}

func TestRideCoverBytesCountInSharedOwnerQuotaAndExpireAcrossFeatures(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011234991", "Cover storage owner")
	other := f.login("+77011234992", "Separate storage owner")
	ctx := context.Background()
	_, err := f.db.Exec(ctx, `
	 WITH seeded_bikes AS (
	  INSERT INTO bikes(id,owner_id,make,model,year,visibility,created_at,updated_at)
	  SELECT ('d4000000-0000-0000-0000-'||lpad(n::text,12,'0'))::uuid,
	   $1::uuid,'Fixture','Cover quota bike '||n,2020,'private',$2,$2
	  FROM generate_series(1,4) AS source(n)
	  RETURNING id,owner_id
	 )
	 INSERT INTO bike_photos(id,bike_id,owner_id,state,reserved_bytes,created_at,expires_at)
	 SELECT ('d5000000-0000-0000-0000-'||lpad(n::text,12,'0'))::uuid,
	  bike.id,bike.owner_id,'pending',4194304,$2,$3
	 FROM generate_series(1,24) AS source(n)
	 JOIN seeded_bikes AS bike ON bike.id=('d4000000-0000-0000-0000-'||lpad((((n-1)/6)+1)::text,12,'0'))::uuid
	`, owner.id, f.now, f.now.Add(10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	rideID := f.ride(owner, 3)
	clubID := testClub(t, f, owner)["id"].(string)
	otherRide := f.ride(other, 3)
	data := photoPNG(t)
	first := rideCoverUpload(t, f, rideID, "1", data, owner, true)
	expect(t, first, 201)
	oldURL := first.body["image"].(map[string]any)["url"].(string)
	clubImageFailure(t, clubImageUpload(t, f, clubID, "1", data, owner, true), 409, "photo_storage_limit")
	clubImageFailure(t, rideCoverUpload(t, f, rideID, "2", data, owner, true), 409, "photo_storage_limit")
	photoRead(t, f, oldURL, nil, "GET", 200)
	expect(t, rideCoverUpload(t, f, otherRide, "1", data, other, true), 201)
	var total int64
	if err = f.db.QueryRow(ctx, `SELECT COALESCE((SELECT sum(reserved_bytes) FROM bike_photos WHERE owner_id=$1),0)+COALESCE((SELECT sum(reserved_bytes) FROM ride_covers WHERE owner_id=$1),0)`, owner.id).Scan(&total); err != nil || total <= 96<<20 || total > 100<<20 {
		t.Fatalf("combined owner payload accounting=%d err=%v", total, err)
	}
	f.now = f.now.Add(11 * time.Minute)
	expect(t, rideCoverUpload(t, f, rideID, "2", data, owner, true), 201)
	photoRead(t, f, oldURL, nil, "GET", 404)
	var bikePending int
	if err = f.db.QueryRow(ctx, "SELECT count(*) FROM bike_photos WHERE state='pending'").Scan(&bikePending); err != nil || bikePending != 0 {
		t.Fatalf("cover upload did not expire pending bike reservations: %d %v", bikePending, err)
	}
	// A stale cover reservation blocks its ride until any upload expires it.
	if _, err = f.db.Exec(ctx, `INSERT INTO ride_covers(id,ride_id,owner_id,state,reserved_bytes,cover_version,created_at,expires_at) VALUES($1,$2,$3,'pending',4194304,1,$4,$5)`, uuid.NewString(), otherRide, other.id, f.now, f.now.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	clubImageFailure(t, rideCoverUpload(t, f, otherRide, "2", data, other, true), 409, "ride_cover_busy")
	f.now = f.now.Add(11 * time.Minute)
	expect(t, clubImageUpload(t, f, clubID, "1", data, owner, true), 201)
	var coverPending int
	if err = f.db.QueryRow(ctx, "SELECT count(*) FROM ride_covers WHERE state='pending'").Scan(&coverPending); err != nil || coverPending != 0 {
		t.Fatalf("club upload did not expire pending ride covers: %d %v", coverPending, err)
	}
	expect(t, rideCoverUpload(t, f, otherRide, "2", data, other, true), 201)
}

func TestRideCoverRespectsTheGlobalImageBudget(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011235001", "Global cover uploader")
	rideID := f.ride(owner, 3)
	ctx := context.Background()
	// Fill the global budget with pending bike reservations; every synthetic
	// owner and bike stays below its own limit, isolating the global constraint.
	_, err := f.db.Exec(ctx, `
	 WITH seeded_owners AS (
	  INSERT INTO users(id,phone,status,display_name,city,languages,locale,onboarding_complete,created_at)
	  SELECT ('d1000000-0000-0000-0000-'||lpad(n::text,12,'0'))::uuid,
	   '+770197'||lpad(n::text,5,'0'),'active','Cover quota owner '||n,'Almaty',ARRAY['en'],'en',true,$1
	  FROM generate_series(1,11) AS source(n)
	  RETURNING id
	 ), seeded_bikes AS (
	  INSERT INTO bikes(id,owner_id,make,model,year,visibility,created_at,updated_at)
	  SELECT ('d2000000-0000-0000-0000-'||lpad(n::text,12,'0'))::uuid,
	   owner.id,'Fixture','Global cover bike '||n,2020,'private',$1,$1
	  FROM generate_series(1,44) AS source(n)
	  JOIN seeded_owners AS owner ON owner.id=('d1000000-0000-0000-0000-'||lpad((((n-1)/4)+1)::text,12,'0'))::uuid
	  RETURNING id,owner_id
	 )
	 INSERT INTO bike_photos(id,bike_id,owner_id,state,reserved_bytes,created_at,expires_at)
	 SELECT ('d3000000-0000-0000-0000-'||lpad(n::text,12,'0'))::uuid,
	  bike.id,bike.owner_id,'pending',4194304,$1,$2
	 FROM generate_series(1,256) AS source(n)
	 JOIN seeded_bikes AS bike ON bike.id=('d2000000-0000-0000-0000-'||lpad((((n-1)/6)+1)::text,12,'0'))::uuid
	`, f.now, f.now.Add(10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	data := photoPNG(t)
	clubImageFailure(t, rideCoverUpload(t, f, rideID, "1", data, owner, true), 409, "photo_storage_limit")
	f.now = f.now.Add(11 * time.Minute)
	expect(t, rideCoverUpload(t, f, rideID, "1", data, owner, true), 201)
}
