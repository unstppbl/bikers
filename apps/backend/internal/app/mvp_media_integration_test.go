package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func mediaUpload(t *testing.T, f *fixture, path string, data []byte, a *actor, csrf bool) response {
	t.Helper()
	req, err := http.NewRequest("POST", f.ts.URL+path, bytes.NewReader(data))
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
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	if r.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("media response may be cached")
	}
	out := response{status: r.StatusCode, raw: string(body)}
	_ = json.Unmarshal(body, &out.body)
	return out
}
func approvedRider(t *testing.T, f *fixture, ride string, owner, member *actor) {
	t.Helper()
	f.join(ride, member)
	expect(t, f.decision(ride, owner, member, "approve"), 200)
}

func TestMVPAvatarSelfAndContextPrivacyReplacementRevocation(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011235011", "Avatar owner")
	member := f.login("+77011235012", "Approved rider")
	outsider := f.login("+77011235013", "Other rider")
	base := "/api/v1/me/avatar"
	source := photoPNG(t)
	expect(t, f.request("GET", base, nil, nil), 401)
	initial := f.request("GET", base, nil, owner)
	expect(t, initial, 200)
	if initial.body["image"] != nil || initial.body["version"] != float64(1) {
		t.Fatal(initial.raw)
	}
	expect(t, mediaUpload(t, f, base+"?version=1", source, owner, false), 403)
	expect(t, mediaUpload(t, f, base+"?version=0", source, owner, true), 422)
	uploaded := mediaUpload(t, f, base+"?version=1", source, owner, true)
	expect(t, uploaded, 201)
	image := uploaded.body["image"].(map[string]any)
	ownURL := image["url"].(string)
	if len(image) != 5 || ownURL != base+"/"+image["id"].(string)+"/content" {
		t.Fatal("avatar metadata has unscoped/private fields")
	}
	photoRead(t, f, ownURL, owner, "GET", 200)
	photoRead(t, f, ownURL, owner, "HEAD", 200)
	photoRead(t, f, ownURL, outsider, "GET", 404)
	expect(t, f.request("GET", "/api/v1/users/"+owner.id+"/avatar", nil, outsider), 404)
	ride := f.ride(owner, 5)
	contextBase := "/api/v1/rides/" + ride + "/avatars/" + owner.id
	f.join(ride, member)
	expect(t, f.request("GET", contextBase, nil, member), 404)
	expect(t, f.decision(ride, owner, member, "approve"), 200)
	view := f.request("GET", contextBase, nil, member)
	expect(t, view, 200)
	if len(view.body) != 1 {
		t.Fatal("context avatar exposed self version")
	}
	contextURL := view.body["image"].(map[string]any)["url"].(string)
	if !strings.HasPrefix(contextURL, contextBase+"/") {
		t.Fatal("context avatar used a global URL")
	}
	photoRead(t, f, contextURL, member, "GET", 200)
	photoRead(t, f, contextURL, member, "HEAD", 200)
	memberUpload := mediaUpload(t, f, base+"?version=1", source, member, true)
	expect(t, memberUpload, 201)
	memberContext := "/api/v1/rides/" + ride + "/avatars/" + member.id
	expect(t, f.request("GET", memberContext, nil, owner), 200)
	expect(t, f.decision(ride, owner, member, "remove"), 200)
	photoRead(t, f, contextURL, member, "GET", 404)
	photoRead(t, f, contextURL, member, "HEAD", 404)
	expect(t, f.request("GET", memberContext, nil, owner), 404)
	club := testClub(t, f, owner)["id"].(string)
	clubBase := "/api/v1/clubs/" + club
	expect(t, f.request("PUT", clubBase+"/membership", nil, member), 200)
	expect(t, f.request("GET", clubBase+"/avatars/"+owner.id, nil, member), 404)
	expect(t, clubDecision(f, club, owner, member, "approve"), 200)
	clubView := f.request("GET", clubBase+"/avatars/"+owner.id, nil, member)
	expect(t, clubView, 200)
	clubURL := clubView.body["image"].(map[string]any)["url"].(string)
	photoRead(t, f, clubURL, member, "GET", 200)
	expect(t, clubDecision(f, club, owner, member, "remove"), 200)
	photoRead(t, f, clubURL, member, "GET", 404)
	expect(t, mediaUpload(t, f, base+"?version=2", []byte{0x89, 'P', 'N', 'G'}, owner, true), 422)
	photoRead(t, f, ownURL, owner, "GET", 200)
	replaced := mediaUpload(t, f, base+"?version=2", source, owner, true)
	expect(t, replaced, 201)
	newURL := replaced.body["image"].(map[string]any)["url"].(string)
	if newURL == ownURL || replaced.body["version"] != float64(3) {
		t.Fatal("avatar replacement did not update revision")
	}
	photoRead(t, f, ownURL, owner, "GET", 404)
	f.startServer(f.gateway)
	photoRead(t, f, newURL, owner, "GET", 200)
	expect(t, f.request("DELETE", base, map[string]any{"version": 2}, owner), 409)
	removed := f.request("DELETE", base, map[string]any{"version": 3}, owner)
	expect(t, removed, 200)
	if removed.body["image"] != nil || removed.body["version"] != float64(4) {
		t.Fatal(removed.raw)
	}
	photoRead(t, f, newURL, owner, "GET", 404)
}

func TestMVPAlbumParticipantsAuthorshipAndVideoLifecycle(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011235021", "Organizer")
	uploader := f.login("+77011235022", "Album uploader")
	member := f.login("+77011235023", "Other approved")
	outsider := f.login("+77011235024", "Pending rider")
	ride := f.ride(owner, 5)
	approvedRider(t, f, ride, owner, uploader)
	approvedRider(t, f, ride, owner, member)
	f.join(ride, outsider)
	base := "/api/v1/rides/" + ride + "/album"
	expect(t, f.request("GET", base, nil, nil), 404)
	expect(t, f.request("GET", base, nil, outsider), 404)
	expect(t, mediaUpload(t, f, base+"/photos", photoPNG(t), uploader, false), 403)
	upload := mediaUpload(t, f, base+"/photos", photoPNG(t), uploader, true)
	expect(t, upload, 201)
	photo := upload.body["photo"].(map[string]any)
	photoID, contentURL := photo["id"].(string), photo["url"].(string)
	if len(photo) != 7 || photo["uploader_id"] != uploader.id || photo["can_delete"] != true {
		t.Fatal("album photo projection leaked data or lost authority")
	}
	before := photoRead(t, f, contentURL, member, "GET", 200)
	photoRead(t, f, contentURL, member, "HEAD", 200)
	photoRead(t, f, contentURL, outsider, "GET", 404)
	view := f.request("GET", base, nil, member)
	expect(t, view, 200)
	if view.body["photos"].([]any)[0].(map[string]any)["can_delete"] != false {
		t.Fatal("ordinary participant can delete another uploader's photo")
	}
	expect(t, f.request("DELETE", base+"/photos/"+photoID, map[string]any{"version": 2}, member), 403)
	expect(t, f.request("POST", base+"/videos", map[string]any{"url": "http://video.example/watch", "title": "Ride"}, uploader), 422)
	added := f.request("POST", base+"/videos", map[string]any{"url": "https://video.example/watch?v=ride#t=10", "title": " Weekend "}, uploader)
	expect(t, added, 201)
	video := added.body["videos"].([]any)[0].(map[string]any)
	videoID := video["id"].(string)
	if video["title"] != "Weekend" || video["uploader_id"] != uploader.id || added.body["version"] != float64(3) {
		t.Fatal(added.raw)
	}
	f.startServer(f.gateway)
	if after := photoRead(t, f, contentURL, member, "GET", 200); !bytes.Equal(before, after) {
		t.Fatal("restart changed album JPEG")
	}
	expect(t, f.request("DELETE", base+"/videos/"+videoID, map[string]any{"version": 3}, member), 403)
	removed := f.request("DELETE", base+"/photos/"+photoID, map[string]any{"version": 3}, owner)
	expect(t, removed, 200)
	photoRead(t, f, contentURL, uploader, "GET", 404)
	expect(t, f.request("POST", "/api/v1/rides/"+ride+"/cancel", map[string]any{"plan_version": 1}, owner), 200)
	expect(t, mediaUpload(t, f, base+"/photos", photoPNG(t), uploader, true), 409)
	expect(t, f.request("POST", base+"/videos", map[string]any{"url": "https://video.example/another", "title": "Another"}, uploader), 409)
	expect(t, f.request("GET", base, nil, uploader), 200)
	deleted := f.request("DELETE", base+"/videos/"+videoID, map[string]any{"version": 4}, uploader)
	expect(t, deleted, 200)
	if len(deleted.body["videos"].([]any)) != 0 || deleted.body["version"] != float64(5) {
		t.Fatal(deleted.raw)
	}
}

func TestMVPAlbumBlockingAndModerationRetainSafeDeletion(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011235031", "Organizer")
	uploader := f.login("+77011235032", "Uploader")
	viewer := f.login("+77011235033", "Viewer")
	ride := f.ride(owner, 5)
	approvedRider(t, f, ride, owner, uploader)
	approvedRider(t, f, ride, owner, viewer)
	base := "/api/v1/rides/" + ride + "/album"
	upload := mediaUpload(t, f, base+"/photos", photoPNG(t), uploader, true)
	expect(t, upload, 201)
	photo := upload.body["photo"].(map[string]any)
	photoID, path := photo["id"].(string), photo["url"].(string)
	added := f.request("POST", base+"/videos", map[string]any{"url": "https://video.example/ride", "title": "Ride"}, uploader)
	expect(t, added, 201)
	videoID := added.body["videos"].([]any)[0].(map[string]any)["id"].(string)
	ctx := context.Background()
	if _, err := f.db.Exec(ctx, "INSERT INTO user_blocks(blocker_id,blocked_id,created_at) VALUES($1,$2,$3)", viewer.id, uploader.id, f.now); err != nil {
		t.Fatal(err)
	}
	blocked := f.request("GET", base, nil, viewer)
	expect(t, blocked, 200)
	if len(blocked.body["photos"].([]any)) != 0 || len(blocked.body["videos"].([]any)) != 0 {
		t.Fatal("blocked author's media leaked")
	}
	photoRead(t, f, path, viewer, "GET", 404)
	photoRead(t, f, path, uploader, "GET", 200)
	for _, target := range []struct{ kind, id string }{{"ride_photo", photoID}, {"ride_video", videoID}} {
		if _, err := f.db.Exec(ctx, "INSERT INTO moderation_hidden(target_type,target_id,hidden_at) VALUES($1,$2,$3)", target.kind, target.id, f.now); err != nil {
			t.Fatal(err)
		}
	}
	for _, a := range []*actor{uploader, owner} {
		view := f.request("GET", base, nil, a)
		expect(t, view, 200)
		p := view.body["photos"].([]any)[0].(map[string]any)
		v := view.body["videos"].([]any)[0].(map[string]any)
		if p["hidden"] != true || p["url"] != "" || p["can_delete"] != true || v["hidden"] != true || v["url"] != "" || v["title"] != "" || v["can_delete"] != true {
			t.Fatal("moderated content exposed pixels/link or removed deletion authority")
		}
		photoRead(t, f, path, a, "GET", 404)
		photoRead(t, f, path, a, "HEAD", 404)
	}
	expect(t, f.request("DELETE", base+"/photos/"+photoID, map[string]any{"version": 3}, uploader), 200)
	expect(t, f.request("DELETE", base+"/videos/"+videoID, map[string]any{"version": 4}, owner), 200)
}

func TestMVPRidePhotoUploadRevocationABAAndCancellation(t *testing.T) {
	for _, change := range []string{"remove", "ABA", "cancel", "plan", "suspend", "expire"} {
		t.Run(change, func(t *testing.T) {
			f := newFixture(t)
			owner := f.login("+77011235041", "Organizer")
			member := f.login("+77011235042", "Uploader")
			ride := f.ride(owner, 4)
			approvedRider(t, f, ride, owner, member)
			release, finished := startGatedImageUpload(t, f, "/api/v1/rides/"+ride+"/album/photos", photoPNG(t), member)
			waitImageReservation(t, f, "ride_photos", "ride_id", ride)
			want := 404
			switch change {
			case "remove", "ABA":
				expect(t, f.decision(ride, owner, member, "remove"), 200)
				if change == "ABA" {
					approvedRider(t, f, ride, owner, member)
				}
			case "cancel":
				expect(t, f.request("POST", "/api/v1/rides/"+ride+"/cancel", map[string]any{"plan_version": 1}, owner), 200)
				want = 409
			case "plan":
				expect(t, f.request("PATCH", "/api/v1/rides/"+ride, map[string]any{"plan_version": 1, "title": "Updated plan"}, owner), 200)
				want = 409
			case "suspend":
				if _, err := f.db.Exec(context.Background(), "UPDATE users SET status='suspended' WHERE id=$1", member.id); err != nil {
					t.Fatal(err)
				}
			case "expire":
				if _, err := f.db.Exec(context.Background(), "UPDATE ride_photos SET expires_at=$2 WHERE ride_id=$1 AND state='pending'", ride, f.now.Add(-time.Second)); err != nil {
					t.Fatal(err)
				}
				want = 409
			}
			expect(t, finishImageUpload(t, release, finished), want)
			var count int
			if err := f.db.QueryRow(context.Background(), "SELECT count(*) FROM ride_photos WHERE ride_id=$1", ride).Scan(&count); err != nil || count != 0 {
				t.Fatal("failed upload retained a reservation or ready bytes")
			}
		})
	}
}

func TestMVPMediaSharedSlotsAndAvatarPendingRemoval(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011235051", "Media owner")
	ride := f.ride(owner, 3)
	bike := testBike(t, f, owner)["id"].(string)
	avatarRelease, avatarFinished := startGatedImageUpload(t, f, "/api/v1/me/avatar?version=1", photoPNG(t), owner)
	waitImageReservation(t, f, "avatars", "owner_id", owner.id)
	photoRelease, photoFinished := startGatedImageUpload(t, f, "/api/v1/rides/"+ride+"/album/photos", photoPNG(t), owner)
	waitImageReservation(t, f, "ride_photos", "ride_id", ride)
	expect(t, photoUpload(t, f, bike, photoPNG(t), owner, true), 429)
	expect(t, finishImageUpload(t, avatarRelease, avatarFinished), 201)
	expect(t, finishImageUpload(t, photoRelease, photoFinished), 201)
	release, finished := startGatedImageUpload(t, f, "/api/v1/me/avatar?version=2", photoPNG(t), owner)
	waitImageReservation(t, f, "avatars", "owner_id", owner.id)
	expect(t, mediaUpload(t, f, "/api/v1/me/avatar?version=2", photoPNG(t), owner, true), 409)
	expect(t, f.request("DELETE", "/api/v1/me/avatar", map[string]any{"version": 2}, owner), 200)
	expect(t, finishImageUpload(t, release, finished), 409)
	current := f.request("GET", "/api/v1/me/avatar", nil, owner)
	expect(t, current, 200)
	if current.body["image"] != nil || current.body["version"] != float64(3) {
		t.Fatal("stale upload resurrected removed avatar")
	}
}

func TestMVPAlbumLimitsCountPendingAndUploaderShares(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011235061", "Organizer")
	member := f.login("+77011235062", "Uploader")
	ride := f.ride(owner, 4)
	approvedRider(t, f, ride, owner, member)
	base := "/api/v1/rides/" + ride + "/album"
	for i := 0; i < 6; i++ {
		if _, err := f.db.Exec(context.Background(), `INSERT INTO ride_photos(id,ride_id,owner_id,state,reserved_bytes,participation_version,ride_plan_version,created_at,expires_at) VALUES($1,$2,$3,'pending',4194304,1,1,$4,$5)`, uuid.NewString(), ride, member.id, f.now, f.now.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	result := mediaUpload(t, f, base+"/photos", photoPNG(t), member, true)
	clubImageFailure(t, result, 409, "album_photo_limit")
	for i := 0; i < 3; i++ {
		expect(t, f.request("POST", base+"/videos", map[string]any{"url": fmt.Sprintf("https://video.example/ride/%d", i), "title": "Ride"}, member), 201)
	}
	clubImageFailure(t, f.request("POST", base+"/videos", map[string]any{"url": "https://video.example/too-many", "title": "Ride"}, member), 409, "album_video_limit")
	// Existing reservations fill the24-photo ride limit, regardless of
	// which authorized participant submits the next source image.
	if _, err := f.db.Exec(context.Background(), `INSERT INTO ride_photos(id,ride_id,owner_id,state,reserved_bytes,participation_version,ride_plan_version,created_at,expires_at) SELECT gen_random_uuid(),$1,$2,'pending',4194304,1,1,$3,$4 FROM generate_series(1,18)`, ride, member.id, f.now, f.now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	clubImageFailure(t, mediaUpload(t, f, base+"/photos", photoPNG(t), owner, true), 409, "album_photo_limit")
}
