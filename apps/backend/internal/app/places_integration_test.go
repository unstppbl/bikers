package app

import (
	"bikers.kz/backend/internal/models"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func placeInput(name string) models.PlaceInput {
	return models.PlaceInput{Name: name, Category: "fuel", Lat: 43.24, Lon: 76.89, City: "Almaty", Address: "Public road", Website: "https://example.com/place", OpeningHours: "24/7"}
}
func importPlaceRecords(t *testing.T, f *fixture, records []models.PlaceImportRecord) models.PlaceImportResult {
	t.Helper()
	var body strings.Builder
	for _, r := range records {
		b, e := json.Marshal(r)
		if e != nil {
			t.Fatal(e)
		}
		body.Write(b)
		body.WriteByte('\n')
	}
	text := body.String()
	out, e := f.s.Places.Import(context.Background(), strings.NewReader(text), fmt.Sprintf("%x", sha256.Sum256([]byte(text))))
	if e != nil {
		t.Fatal(e)
	}
	return out
}
func submitPlace(t *testing.T, f *fixture, a *actor, kind, id string, version int64, p *models.PlaceInput) map[string]any {
	t.Helper()
	r := f.request("POST", "/api/v1/places/submissions", models.PlaceSubmissionInput{Kind: kind, PlaceID: id, BaseVersion: version, Place: p, Reason: "PRIVATE contribution evidence"}, a)
	expect(t, r, 201)
	return r.body["submission"].(map[string]any)
}
func reviewPlace(f *fixture, a *actor, s map[string]any, status string) response {
	return f.request("POST", "/api/v1/admin/place-submissions/"+s["id"].(string)+"/review", models.PlaceReviewInput{Version: int64(s["version"].(float64)), Status: status, Reason: "PRIVATE moderator evidence"}, a)
}
func placeList(t *testing.T, f *fixture, path string) response {
	t.Helper()
	r := f.request("GET", "/api/v1/places"+path, nil, nil)
	expect(t, r, 200)
	return r
}

func TestPlacesModeratedPublicationPrivacyStaleReviewClosureAndRestart(t *testing.T) {
	f := newFixture(t)
	rider := f.login("+77011234911", "Contributor")
	other := f.login("+77011234912", "Other rider")
	admin := f.login("+77011234913", "Moderator")
	grantTestAdmin(t, f, admin)
	base := placeInput("OSM service point")
	records := []models.PlaceImportRecord{{PlaceInput: base, SourceRef: "node/1101", SourceDate: "2026-11-02"}}
	if r := importPlaceRecords(t, f, records); r.Inserted != 1 {
		t.Fatal(r)
	}
	public := placeList(t, f, "")
	places := public.body["places"].([]any)
	id := places[0].(map[string]any)["id"].(string)
	expect(t, f.request("GET", "/api/v1/places/config", nil, nil), 200)
	newPlace := placeInput("Pending community workshop")
	newPlace.Category = "workshop"
	pending := submitPlace(t, f, rider, "new", "", 0, &newPlace)
	if strings.Contains(fmt.Sprint(pending), "submitter_id") {
		t.Fatal("create receipt includes contributor identity")
	}
	if len(placeList(t, f, "").body["places"].([]any)) != 1 {
		t.Fatal("pending proposal published")
	}
	expect(t, f.request("GET", "/api/v1/places/"+pending["id"].(string), nil, nil), 404)
	for _, a := range []*actor{nil, other} {
		own := f.request("GET", "/api/v1/me/place-submissions", nil, a)
		if a == nil {
			expect(t, own, 401)
		} else {
			expect(t, own, 200)
			if len(own.body["submissions"].([]any)) != 0 {
				t.Fatal("another rider read pending proposal")
			}
		}
	}
	expect(t, f.request("GET", "/api/v1/admin/place-submissions", nil, rider), 403)
	expect(t, reviewPlace(f, rider, pending, "approved"), 403)
	expect(t, f.request("POST", "/api/v1/places/submissions", models.PlaceSubmissionInput{Kind: "new", Place: &base, Reason: "Reason"}, nil), 401)
	badCSRF := *rider
	badCSRF.csrf = "wrong"
	expect(t, f.request("POST", "/api/v1/places/submissions", models.PlaceSubmissionInput{Kind: "new", Place: &base, Reason: "Reason"}, &badCSRF), 403)
	queue := f.request("GET", "/api/v1/admin/place-submissions", nil, admin)
	expect(t, queue, 200)
	if !strings.Contains(queue.raw, rider.id) || !strings.Contains(queue.raw, newPlace.Name) {
		t.Fatal("administrator queue lacks proposal evidence", queue.raw)
	}
	approved := reviewPlace(f, admin, pending, "approved")
	expect(t, approved, 200)
	expect(t, reviewPlace(f, admin, pending, "approved"), 409)
	communityID := approved.body["submission"].(map[string]any)["place_id"].(string)
	expect(t, f.request("GET", "/api/v1/places/"+communityID, nil, nil), 200)
	changed := base
	changed.Name = "Community corrected station"
	correction := submitPlace(t, f, rider, "correction", id, 1, &changed)
	stale := submitPlace(t, f, other, "correction", id, 1, &base)
	expect(t, reviewPlace(f, admin, correction, "approved"), 200)
	conflict := reviewPlace(f, admin, stale, "approved")
	expect(t, conflict, 409)
	if conflict.body["error"].(map[string]any)["code"] != "stale_version" {
		t.Fatal(conflict.raw)
	}
	detail := f.request("GET", "/api/v1/places/"+id, nil, nil)
	expect(t, detail, 200)
	if detail.body["place"].(map[string]any)["name"] != changed.Name {
		t.Fatal("stale approval overwrote accepted correction")
	}
	expect(t, reviewPlace(f, admin, stale, "rejected"), 200)
	if result := importPlaceRecords(t, f, records); result.Preserved != 1 {
		t.Fatal("OSM replay overwrote community correction", result)
	}
	closure := submitPlace(t, f, rider, "closed", id, 2, nil)
	expect(t, reviewPlace(f, admin, closure, "approved"), 200)
	expect(t, f.request("GET", "/api/v1/places/"+id, nil, nil), 404)
	if result := importPlaceRecords(t, f, records); result.Preserved != 1 {
		t.Fatal("OSM replay reopened accepted closure", result)
	}
	before := placeList(t, f, "")
	f.startServer(f.gateway)
	after := placeList(t, f, "")
	if !reflect.DeepEqual(before.body, after.body) {
		t.Fatal("public places changed after restart")
	}
	own := f.request("GET", "/api/v1/me/place-submissions", nil, rider)
	expect(t, own, 200)
	for _, forbidden := range []string{"submitter_id", "reviewer_id", admin.id} {
		if strings.Contains(own.raw, forbidden) {
			t.Fatalf("own history disclosed %s", forbidden)
		}
	}
	if len(own.body["submissions"].([]any)) != 3 {
		t.Fatal("submission history lost on restart", own.raw)
	}
	for _, forbidden := range []string{"PRIVATE", "submitter", "reviewer", rider.id, admin.id, "proposal", "review_reason"} {
		if strings.Contains(after.raw, forbidden) {
			t.Fatal("public place leaked moderation data", forbidden, after.raw)
		}
	}
}

func TestPlacesImportIdempotencyAtomicValidationFiltersAndRestartPaging(t *testing.T) {
	f := newFixture(t)
	records := []models.PlaceImportRecord{}
	for i := range 64 {
		p := placeInput(fmt.Sprintf("Station %02d", i))
		if i%2 == 0 {
			p.Category = "parking"
		}
		if i >= 40 {
			p.City = "Astana"
			p.Lat = 51.16
			p.Lon = 71.43
		}
		records = append(records, models.PlaceImportRecord{PlaceInput: p, SourceRef: fmt.Sprintf("node/%d", 2000+i), SourceDate: "2026-11-02"})
	}
	records[0].Name = "Literal %_ place"
	out := importPlaceRecords(t, f, records)
	if out.Inserted != 64 || out.Total != 64 {
		t.Fatal(out)
	}
	before := placeList(t, f, "")
	again := importPlaceRecords(t, f, records)
	if again.Unchanged != 64 || again.Inserted != 0 || again.Updated != 0 {
		t.Fatal("import replay changed records", again)
	}
	if !reflect.DeepEqual(before.body, placeList(t, f, "").body) {
		t.Fatal("replay changed IDs, versions or dates")
	}
	page := placeList(t, f, "?limit=7&city=Алматы&category=parking")
	if len(page.body["places"].([]any)) != 7 || page.body["next_cursor"] == "" {
		t.Fatal("city/category filters or pagination failed", page.raw)
	}
	cursor := page.body["next_cursor"].(string)
	firstID := page.body["places"].([]any)[0].(map[string]any)["id"]
	f.startServer(f.gateway)
	next := placeList(t, f, "?limit=7&city=Алматы&category=parking&after="+cursor)
	if len(next.body["places"].([]any)) != 7 || next.body["places"].([]any)[0].(map[string]any)["id"] == firstID {
		t.Fatal("cursor failed after restart", next.raw)
	}
	expect(t, f.request("GET", "/api/v1/places?limit=7&city=Astana&category=parking&after="+cursor, nil, nil), 422)
	area := placeList(t, f, "?west=76&south=43&east=77&north=44&limit=100")
	if len(area.body["places"].([]any)) != 40 {
		t.Fatal("bbox not applied before paging", area.raw)
	}
	literal := placeList(t, f, "?q=%25_&limit=100")
	if len(literal.body["places"].([]any)) != 1 {
		t.Fatal("search used SQL wildcards", literal.raw)
	}
	for _, query := range []string{"?limit=0", "?limit=101", "?limit=bad", "?category=unknown", "?west=76", "?west=77&south=43&east=76&north=44", "?west=NaN&south=43&east=77&north=44", "?west=0&south=0&east=180&north=90", "?after=bad", "?q=" + strings.Repeat("x", 161)} {
		expect(t, f.request("GET", "/api/v1/places"+query, nil, nil), 422)
	}
	// Validate every row before writes: a valid first row cannot partially commit.
	body := `{"source_ref":"node/9999","source_date":"2026-11-02","name":"Should not commit","category":"fuel","lat":43.2,"lon":76.9}` + "\n" + `{"source_ref":"node/9998","source_date":"2026-11-02","name":"Invalid","category":"fuel","lat":0,"lon":0}`
	if _, e := f.s.Places.Import(context.Background(), strings.NewReader(body), fmt.Sprintf("%x", sha256.Sum256([]byte(body)))); e == nil {
		t.Fatal("invalid import accepted")
	}
	if n := len(placeList(t, f, "?limit=100").body["places"].([]any)); n != 64 {
		t.Fatal("failed import partially committed", n)
	}
	oldPublic := placeList(t, f, "?q=Station+01").body["places"].([]any)[0].(map[string]any)
	records[1].Address = "Updated source address"
	updated := importPlaceRecords(t, f, records)
	if updated.Updated != 1 || updated.Unchanged != 63 {
		t.Fatal(updated)
	}
	nowPublic := placeList(t, f, "?q=Station+01").body["places"].([]any)[0].(map[string]any)
	if oldPublic["id"] != nowPublic["id"] || nowPublic["version"] != float64(2) {
		t.Fatal("source update did not preserve ID/increment version", nowPublic)
	}
}

func TestPlacesConcurrentReviewSubmissionQuotaAndRestrictions(t *testing.T) {
	f := newFixture(t)
	rider := f.login("+77011234921", "Contributor")
	admin := f.login("+77011234922", "Moderator")
	grantTestAdmin(t, f, admin)
	p := placeInput("Concurrent pending place")
	submission := submitPlace(t, f, rider, "new", "", 0, &p)
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); statuses <- reviewPlace(f, admin, submission, "approved").status }()
	}
	wg.Wait()
	close(statuses)
	counts := map[int]int{}
	for status := range statuses {
		counts[status]++
	}
	if counts[200] != 1 || counts[409] != 1 {
		t.Fatal("concurrent review published twice", counts)
	}
	if len(placeList(t, f, "").body["places"].([]any)) != 1 {
		t.Fatal("concurrent review duplicated place")
	}
	quota := make(chan int, 21)
	for range 21 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := f.request("POST", "/api/v1/places/submissions", models.PlaceSubmissionInput{Kind: "new", Place: &p, Reason: "Quota check"}, rider)
			quota <- r.status
		}()
	}
	wg.Wait()
	close(quota)
	counts = map[int]int{}
	for status := range quota {
		counts[status]++
	}
	if counts[201] != 19 || counts[429] != 2 {
		t.Fatal("daily quota raced", counts)
	}
	f.startServer(f.gateway)
	expect(t, f.request("POST", "/api/v1/places/submissions", models.PlaceSubmissionInput{Kind: "new", Place: &p, Reason: "Restart quota"}, rider), 429)
	if _, err := f.db.Exec(context.Background(), `UPDATE users SET status='suspended' WHERE id=$1`, rider.id); err != nil {
		t.Fatal(err)
	}
	queue := f.request("GET", "/api/v1/admin/place-submissions", nil, admin)
	expect(t, queue, 200)
	pending := queue.body["submissions"].([]any)[0].(map[string]any)
	expect(t, reviewPlace(f, admin, pending, "approved"), 403)
	if _, err := f.db.Exec(context.Background(), `UPDATE users SET status='active' WHERE id=$1`, rider.id); err != nil {
		t.Fatal(err)
	}
	if err := f.s.Trust.Role(context.Background(), admin.id, false, "Revoke test moderator"); err != nil {
		t.Fatal(err)
	}
	expect(t, reviewPlace(f, admin, pending, "approved"), 403)
	// UTC quota reset is durable history, not a process-local counter.
	f.now = f.now.Add(24 * time.Hour)
	expect(t, f.request("POST", "/api/v1/places/submissions", models.PlaceSubmissionInput{Kind: "new", Place: &p, Reason: "New day"}, rider), 201)
}
