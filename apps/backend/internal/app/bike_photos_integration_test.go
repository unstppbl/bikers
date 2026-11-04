package app

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

func photoPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 120, 80))
	for y := 0; y < 80; y++ {
		for x := 0; x < 120; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 2), G: uint8(y * 3), B: 140, A: 255})
		}
	}
	var data bytes.Buffer
	if err := png.Encode(&data, img); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}
func photoUpload(t *testing.T, f *fixture, bike string, data []byte, actor *actor, csrf bool) response {
	t.Helper()
	req, err := http.NewRequest("POST", f.ts.URL+"/api/v1/me/bikes/"+bike+"/photos", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", f.ts.URL)
	req.Header.Set("Content-Type", "image/png")
	if actor != nil {
		req.AddCookie(actor.cookie)
		if csrf {
			req.Header.Set("X-CSRF-Token", actor.csrf)
		}
	}
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	body, _ := io.ReadAll(r.Body)
	out := response{status: r.StatusCode, raw: string(body)}
	_ = json.Unmarshal(body, &out.body)
	if r.Header.Get("Cache-Control") != "no-store" {
		t.Error("upload response may be cached")
	}
	return out
}
func photoGallery(t *testing.T, f *fixture, id string, a *actor) map[string]any {
	t.Helper()
	r := f.request("GET", "/api/v1/bikes/"+id+"/photos", nil, a)
	expect(t, r, 200)
	return r.body
}
func photoRead(t *testing.T, f *fixture, path string, a *actor, method string, status int) []byte {
	t.Helper()
	req, _ := http.NewRequest(method, f.ts.URL+path, nil)
	if a != nil {
		req.AddCookie(a.cookie)
	}
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	data, _ := io.ReadAll(r.Body)
	if r.StatusCode != status {
		t.Fatalf("photo read %s:status%d want%d body%s", path, r.StatusCode, status, data)
	}
	if r.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("photo response cache policy:%s", r.Header.Get("Cache-Control"))
	}
	if status == 200 && (r.Header.Get("Content-Type") != "image/jpeg" || r.Header.Get("X-Content-Type-Options") != "nosniff") {
		t.Fatal("unsafe photo content headers")
	}
	if status == 200 && method == "GET" {
		if _, err := jpeg.Decode(bytes.NewReader(data)); err != nil {
			t.Fatalf("photo was not re-encoded JPEG:%v", err)
		}
	}
	if method == "HEAD" && len(data) != 0 {
		t.Fatal("HEAD returned a photo body")
	}
	return data
}
func TestBikePhotosDefaultCoverPrivacyAndAtomicDeletion(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011234611", "Photo owner")
	other := f.login("+77011234612", "Other rider")
	bike := testBike(t, f, owner)
	id := bike["id"].(string)
	base := "/api/v1/me/bikes/" + id
	initial := photoGallery(t, f, id, owner)
	if initial["cover_photo_id"] != "" || initial["version"] != float64(1) || len(initial["photos"].([]any)) != 0 {
		t.Fatal("catalogue is not the initial default")
	}
	source := photoPNG(t)
	expect(t, photoUpload(t, f, id, source, nil, true), 401)
	expect(t, photoUpload(t, f, id, source, owner, false), 403)
	expect(t, photoUpload(t, f, id, source, other, true), 404)
	uploaded := photoUpload(t, f, id, source, owner, true)
	expect(t, uploaded, 201)
	gallery := uploaded.body["gallery"].(map[string]any)
	photo := uploaded.body["photo"].(map[string]any)
	photoID := photo["id"].(string)
	path := photo["url"].(string)
	if gallery["cover_photo_id"] != "" || gallery["version"] != float64(2) {
		t.Fatal("upload changed catalogue default or missed version")
	}
	if len(photo) != 5 || photo["width"] != float64(120) || photo["height"] != float64(80) || !strings.HasPrefix(path, "/api/v1/bikes/"+id+"/photos/") {
		t.Fatalf("unsafe photo metadata:%v", photo)
	}
	photoRead(t, f, path, owner, "GET", 200)
	photoRead(t, f, path, owner, "HEAD", 200)
	photoRead(t, f, path, nil, "GET", 404)
	photoRead(t, f, path, other, "GET", 404)
	expect(t, f.request("GET", "/api/v1/bikes/"+id+"/photos", nil, nil), 404)
	chosen := f.request("PUT", base+"/photo-cover", map[string]any{"version": 2, "photo_id": photoID}, owner)
	expect(t, chosen, 200)
	if chosen.body["gallery"].(map[string]any)["cover_photo_id"] != photoID {
		t.Fatal("explicit cover was not selected")
	}
	expect(t, f.request("PUT", base+"/photo-cover", map[string]any{"version": 2, "photo_id": ""}, owner), 409)
	second := testBike(t, f, owner)
	secondID := second["id"].(string)
	expect(t, f.request("PUT", "/api/v1/me/bikes/"+secondID+"/photo-cover", map[string]any{"version": 1, "photo_id": photoID}, owner), 422)
	photoRead(t, f, "/api/v1/bikes/"+secondID+"/photos/"+photoID+"/content", owner, "GET", 404)
	expect(t, f.request("PATCH", base, map[string]any{"version": 1, "visibility": "public"}, owner), 200)
	for _, viewer := range []*actor{nil, other} {
		public := photoGallery(t, f, id, viewer)
		if len(public) != 2 || public["version"] != nil || strings.Contains(publicString(public), owner.id) {
			t.Fatalf("private gallery metadata leaked:%v", public)
		}
		photoRead(t, f, path, viewer, "GET", 200)
		record := f.request("GET", "/api/v1/bikes/"+id, nil, viewer)
		expect(t, record, 200)
		if len(record.body["bike"].(map[string]any)) != 5 {
			t.Fatal("photos changed public bike projection")
		}
	}
	// Replacing app/pool must retain both content and explicit selection.
	f.startServer(f.gateway)
	photoRead(t, f, path, owner, "GET", 200)
	if photoGallery(t, f, id, owner)["cover_photo_id"] != photoID {
		t.Fatal("restart lost cover")
	}
	expect(t, f.request("PATCH", base, map[string]any{"version": 2, "visibility": "private"}, owner), 200)
	photoRead(t, f, path, nil, "GET", 404)
	photoRead(t, f, path, other, "GET", 404)
	deleted := f.request("DELETE", base+"/photos/"+photoID, map[string]any{"version": 3}, owner)
	expect(t, deleted, 200)
	if deleted.body["gallery"].(map[string]any)["cover_photo_id"] != "" {
		t.Fatal("deleting cover did not restore catalogue")
	}
	photoRead(t, f, path, owner, "GET", 404)
	var count int
	if err := f.db.QueryRow(context.Background(), "SELECT count(*) FROM bike_photos WHERE id=$1", photoID).Scan(&count); err != nil || count != 0 {
		t.Fatal("deleted content retained")
	}
	uploaded = photoUpload(t, f, id, source, owner, true)
	expect(t, uploaded, 201)
	gallery = uploaded.body["gallery"].(map[string]any)
	photo = uploaded.body["photo"].(map[string]any)
	expect(t, f.request("PUT", base+"/photo-cover", map[string]any{"version": gallery["version"], "photo_id": photo["id"]}, owner), 200)
	expect(t, f.request("DELETE", base, map[string]any{"version": 3}, owner), 200)
	if err := f.db.QueryRow(context.Background(), "SELECT count(*) FROM bike_photos WHERE bike_id=$1", id).Scan(&count); err != nil || count != 0 {
		t.Fatal("bike cascade retained image content")
	}
}
func publicString(v any) string { data, _ := json.Marshal(v); return string(data) }
func TestBikePhotoConcurrencyLimitAndInvalidInputs(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011234621", "Photo owner")
	id := testBike(t, f, owner)["id"].(string)
	data := photoPNG(t)
	expect(t, photoUpload(t, f, id, []byte{0x89, 'P', 'N', 'G'}, owner, true), 422)
	for range 5 {
		expect(t, photoUpload(t, f, id, data, owner, true), 201)
	}
	results := make(chan response, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() { results <- photoUpload(t, f, id, data, owner, true) })
	}
	wg.Wait()
	close(results)
	created := 0
	for r := range results {
		if r.status == 201 {
			created++
		} else if r.status != 409 && r.status != 429 {
			t.Fatalf("unexpected concurrent upload:%d %s", r.status, r.raw)
		}
	}
	if created != 1 {
		t.Fatalf("capacity race accepted%d uploads, want1", created)
	}
	if len(photoGallery(t, f, id, owner)["photos"].([]any)) != 6 {
		t.Fatal("photo count limit violated")
	}
	var pending int
	if err := f.db.QueryRow(context.Background(), "SELECT count(*) FROM bike_photos WHERE state='pending'").Scan(&pending); err != nil || pending != 0 {
		t.Fatal("finished or invalid upload left reservation")
	}
}
