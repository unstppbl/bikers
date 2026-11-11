package app

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"bikers.kz/backend/internal/models"
)

func recordedFixture(t *testing.T, f *fixture, owner *actor) map[string]any {
	t.Helper()
	input := models.TrackInput{Name: "Private GPS recording", DurationSeconds: 120,
		Segments: [][][2]float64{{{76.89, 43.24}, {76.891, 43.241}}, {{77.5, 44}, {77.501, 44.001}}}}
	created := f.request("POST", "/api/v1/tracks", input, owner)
	expect(t, created, 201)
	track := created.body["track"].(map[string]any)
	if distance := track["distance_meters"].(float64); distance < 100 || distance > 400 {
		t.Fatal("pause gap was connected or distance was not computed", created.raw)
	}
	return track
}

func routeFromTrack(t *testing.T, f *fixture, owner *actor, track map[string]any) map[string]any {
	t.Helper()
	in := models.CommunityRouteInput{Title: "Mountain route", City: "Almaty", Description: "Public riding notes", Surface: "mixed", TrackID: track["id"].(string), Published: true}
	expect(t, f.request("POST", "/api/v1/routes", in, owner), 422)
	in.PublishConsent = true
	created := f.request("POST", "/api/v1/routes", in, owner)
	expect(t, created, 201)
	return created.body["route"].(map[string]any)
}

func denyRouteAccess(t *testing.T, result response) {
	t.Helper()
	if result.status != 403 && result.status != 404 {
		t.Fatalf("unauthorized object access returned %d: %s", result.status, result.raw)
	}
}

func TestRoutesPrivateRecordingPublicationCollectionsAndSnapshotLifetime(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011235301", "Route owner")
	reader := f.login("+77011235302", "Route reader")
	for _, path := range []string{"/api/v1/tracks", "/api/v1/route-collections"} {
		expect(t, f.request("GET", path, nil, nil), 401)
	}
	badCSRF := *owner
	badCSRF.csrf = "invalid"
	expect(t, f.request("POST", "/api/v1/tracks", models.TrackInput{}, nil), 401)
	expect(t, f.request("POST", "/api/v1/tracks", models.TrackInput{}, &badCSRF), 403)
	track := recordedFixture(t, f, owner)
	trackID := track["id"].(string)
	denyRouteAccess(t, f.request("GET", "/api/v1/tracks/"+trackID, nil, reader))
	denyRouteAccess(t, f.request("DELETE", "/api/v1/tracks/"+trackID, map[string]int{"version": 1}, reader))
	if list := f.request("GET", "/api/v1/tracks", nil, reader); len(list.body["tracks"].([]any)) != 0 {
		t.Fatal("another account's private track entered the list", list.raw)
	}
	route := routeFromTrack(t, f, owner, track)
	id := route["id"].(string)
	for _, actor := range []*actor{nil, reader} {
		public := f.request("GET", "/api/v1/routes/"+id, nil, actor)
		expect(t, public, 200)
		if strings.Contains(public.raw, trackID) || strings.Contains(public.raw, "Private GPS recording") || strings.Contains(public.raw, "source_track") {
			t.Fatal("public snapshot exposed the private source recording", public.raw)
		}
		if len(public.body["route"].(map[string]any)["segments"].([]any)) != 2 {
			t.Fatal("recorded pause segments were lost", public.raw)
		}
		list := f.request("GET", "/api/v1/routes?city=Алматы&q=Mountain&limit=1", nil, actor)
		expect(t, list, 200)
		if len(list.body["routes"].([]any)) != 1 || strings.Contains(list.raw, "segments") || strings.Contains(list.raw, "76.89") {
			t.Fatal("route list lost city matching or leaked full geometry", list.raw)
		}
	}
	collectionResponse := f.request("POST", "/api/v1/route-collections", models.RouteCollectionInput{Name: "Weekend routes"}, reader)
	expect(t, collectionResponse, 201)
	collectionID := collectionResponse.body["collection"].(map[string]any)["id"].(string)
	itemPath := "/api/v1/route-collections/" + collectionID + "/routes/" + id
	expect(t, f.request("PUT", itemPath, nil, reader), 200)
	expect(t, f.request("PUT", itemPath, nil, reader), 200)
	denyRouteAccess(t, f.request("GET", "/api/v1/routes?collection="+collectionID, nil, owner))
	denyRouteAccess(t, f.request("PUT", itemPath, nil, owner))
	denyRouteAccess(t, f.request("DELETE", "/api/v1/route-collections/"+collectionID, map[string]int{"version": 1}, owner))
	collection := f.request("GET", "/api/v1/routes?collection="+collectionID, nil, reader)
	expect(t, collection, 200)
	if len(collection.body["routes"].([]any)) != 1 {
		t.Fatal("collection membership was duplicated or lost", collection.raw)
	}
	patch := models.CommunityRouteInput{Title: "Mountain route", City: "Almaty", Description: "Public riding notes", Surface: "mixed", Version: 1}
	denyRouteAccess(t, f.request("PUT", "/api/v1/routes/"+id, patch, reader))
	private := f.request("PUT", "/api/v1/routes/"+id, patch, owner)
	expect(t, private, 200)
	expect(t, f.request("GET", "/api/v1/routes/"+id, nil, reader), 404)
	if saved := f.request("GET", "/api/v1/routes?collection="+collectionID, nil, reader); len(saved.body["routes"].([]any)) != 0 {
		t.Fatal("private route survived in another account's collection response", saved.raw)
	}
	expect(t, f.request("PUT", "/api/v1/routes/"+id, patch, owner), 409)
	patch.Version, patch.Published = 2, true
	expect(t, f.request("PUT", "/api/v1/routes/"+id, patch, owner), 422)
	patch.PublishConsent = true
	expect(t, f.request("PUT", "/api/v1/routes/"+id, patch, owner), 200)
	expect(t, f.request("DELETE", "/api/v1/tracks/"+trackID, map[string]int{"version": 1}, owner), 200)
	f.startServer(f.gateway)
	expect(t, f.request("GET", "/api/v1/tracks/"+trackID, nil, owner), 404)
	snapshot := f.request("GET", "/api/v1/routes/"+id, nil, nil)
	expect(t, snapshot, 200)
	if len(snapshot.body["route"].(map[string]any)["segments"].([]any)) != 2 {
		t.Fatal("deleting a private source destroyed the published snapshot", snapshot.raw)
	}
	expect(t, f.request("DELETE", "/api/v1/routes/"+id, map[string]int{"version": 3}, owner), 200)
	expect(t, f.request("GET", "/api/v1/routes/"+id, nil, nil), 404)
	if saved := f.request("GET", "/api/v1/routes?collection="+collectionID, nil, reader); len(saved.body["routes"].([]any)) != 0 {
		t.Fatal("deleted route survived in a collection", saved.raw)
	}
}

func TestRoutesConditionsFreshnessModerationBlocksAndAuthorBoundaries(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011235311", "Route publisher")
	reporter := f.login("+77011235312", "Condition author")
	admin := f.login("+77011235313", "Route moderator")
	grantTestAdmin(t, f, admin)
	route := routeFromTrack(t, f, owner, recordedFixture(t, f, owner))
	id := route["id"].(string)
	conditionsPath := "/api/v1/routes/" + id + "/conditions"
	in := models.RouteConditionInput{Kind: "gravel", Description: "Loose gravel observed on the ascent", ObservedAt: f.now.Add(-time.Hour)}
	created := f.request("POST", conditionsPath, in, reporter)
	expect(t, created, 201)
	condition := created.body["condition"].(map[string]any)
	conditionID := condition["id"].(string)
	if condition["state"] != "recent" || condition["author_id"] != reporter.id || condition["expires_at"] == nil {
		t.Fatal("condition lacks attribution or expiry", created.raw)
	}
	in.Version, in.Resolved = 1, true
	itemPath := conditionsPath + "/" + conditionID
	denyRouteAccess(t, f.request("PUT", itemPath, in, owner))
	denyRouteAccess(t, f.request("DELETE", itemPath, map[string]int{"version": 1}, owner))
	expect(t, trustReport(f, owner, "route_condition", conditionID), 201)
	expect(t, trustAction(f, admin, "hide_content", "route_condition", conditionID, "Review dated report", ""), 200)
	if visible := f.request("GET", conditionsPath, nil, nil); len(visible.body["conditions"].([]any)) != 0 {
		t.Fatal("hidden condition remained public", visible.raw)
	}
	expect(t, trustAction(f, admin, "restore_content", "route_condition", conditionID, "Restore report", ""), 200)
	resolved := f.request("PUT", itemPath, in, reporter)
	expect(t, resolved, 200)
	if resolved.body["condition"].(map[string]any)["state"] != "resolved" {
		t.Fatal("resolved report still appears current", resolved.raw)
	}
	expect(t, f.request("PUT", itemPath, in, reporter), 409)
	in.Version, in.Resolved, in.ObservedAt = 2, false, f.now.Add(-31*24*time.Hour)
	expired := f.request("PUT", itemPath, in, reporter)
	expect(t, expired, 200)
	if expired.body["condition"].(map[string]any)["state"] != "expired" {
		t.Fatal("old observation remains a permanent current warning", expired.raw)
	}
	expect(t, trustReport(f, reporter, "route", id), 201)
	expect(t, trustAction(f, admin, "hide_content", "route", id, "Review route", ""), 200)
	expect(t, f.request("GET", "/api/v1/routes/"+id, nil, nil), 404)
	expect(t, f.request("GET", conditionsPath, nil, reporter), 404)
	expect(t, trustReport(f, reporter, "route_condition", conditionID), 404)
	expect(t, trustAction(f, admin, "restore_content", "route", id, "Restore route", ""), 200)
	expect(t, f.request("PUT", "/api/v1/me/blocks/"+owner.id, nil, reporter), 200)
	expect(t, f.request("GET", "/api/v1/routes/"+id, nil, reporter), 404)
	expect(t, f.request("GET", conditionsPath, nil, reporter), 404)
	expect(t, trustReport(f, reporter, "route_condition", conditionID), 404)
	expect(t, f.request("DELETE", "/api/v1/me/blocks/"+owner.id, nil, reporter), 200)
	expect(t, f.request("DELETE", itemPath, map[string]int{"version": 3}, reporter), 200)
	var count int
	if err := f.db.QueryRow(context.Background(), `SELECT conditions_created FROM route_daily_limits WHERE owner_id=$1`, reporter.id).Scan(&count); err != nil || count != 1 {
		t.Fatal("deleting a condition reset its daily creation budget", count, err)
	}
	if result := f.request("GET", conditionsPath, nil, nil); result.status != 200 || len(result.body["conditions"].([]any)) != 0 {
		t.Fatal("deleted condition retained public content", result.raw)
	}
}

func TestRoutesRejectInvalidTrackBodiesAndPaging(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011235321", "Recorder")
	for _, in := range []any{
		models.TrackInput{Name: "Empty", DurationSeconds: 60},
		models.TrackInput{Name: "Outside supported region", DurationSeconds: 60, Segments: [][][2]float64{{{0, 0}, {1, 1}}}},
		models.TrackInput{Name: "No elapsed time", Segments: [][][2]float64{{{76.89, 43.24}, {76.891, 43.241}}}},
	} {
		expect(t, f.request("POST", "/api/v1/tracks", in, owner), 422)
	}
	expect(t, f.request("POST", "/api/v1/tracks", map[string]any{"name": "Hidden field", "owner_id": owner.id}, owner), 400)
	expect(t, f.request("POST", "/api/v1/tracks", map[string]any{"name": strings.Repeat("X", 2<<20)}, owner), 413)
	for _, query := range []string{"limit=0", "limit=101", "limit=no", "after=not-a-cursor", "scope=unknown"} {
		t.Run(fmt.Sprintf("invalid_%s", query), func(t *testing.T) {
			expect(t, f.request("GET", "/api/v1/routes?"+query, nil, owner), 422)
		})
	}
}
