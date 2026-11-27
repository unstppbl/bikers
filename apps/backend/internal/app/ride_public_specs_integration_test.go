package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"bikers.kz/backend/internal/models"
)

func TestExpandedRideDetailsRequireExplicitPublicationAcrossPublicViews(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011239801", "PRIVATE ORGANIZER")
	member := f.login("+77011239802", "Approved rider")
	created := f.request("POST", "/api/v1/rides", map[string]any{
		"city": "Almaty", "description": "A public description", "title": "PRIVATE TITLE",
		"starts_at": f.now.Add(24 * time.Hour).Format(time.RFC3339), "timezone": "Asia/Almaty",
		"meeting_point": "PRIVATE MEETING", "capacity": 5, "style": "touring", "surface": "asphalt", "distance_km": 80,
	}, owner)
	expect(t, created, 201)
	id := created.body["ride"].(map[string]any)["id"].(string)
	path := "/api/v1/rides/" + id
	club := testClub(t, f, owner)["id"].(string)
	clubPath := "/api/v1/clubs/" + club + "/rides"
	expect(t, f.request("PUT", clubPath+"/"+id, nil, owner), 200)
	f.join(id, member)
	expect(t, f.decision(id, owner, member, "approve"), 200)

	check := func(published bool) {
		t.Helper()
		for _, endpoint := range []string{path, "/api/v1/rides", clubPath} {
			result := f.request("GET", endpoint, nil, nil)
			expect(t, result, 200)
			var summary map[string]any
			if endpoint == path {
				summary = result.body["ride"].(map[string]any)
			} else {
				rows := result.body["rides"].([]any)
				if len(rows) != 1 {
					t.Fatal("ride missing from public listing", result.raw)
				}
				summary = rows[0].(map[string]any)
			}
			if published {
				if summary["title"] != "PRIVATE TITLE" || summary["organizer_name"] != "PRIVATE ORGANIZER" || summary["capacity"] != float64(5) || summary["approved_count"] != float64(2) || summary["distance_km"] != float64(80) || summary["style"] != "touring" || summary["time_window"] == nil {
					t.Fatal("explicitly published details missing", result.raw)
				}
			} else if len(summary) != 5 || strings.Contains(result.raw, "PRIVATE") {
				t.Fatal("unpublished ride details leaked", result.raw)
			}
			for _, key := range []string{"public_details_published", "distance_override_km", "starts_at", "map_plan"} {
				if summary[key] != nil {
					t.Fatal("editing or plan metadata leaked", key, result.raw)
				}
			}
		}
		for _, query := range []string{"q=PRIVATE+TITLE", "style=touring", "surface=asphalt"} {
			result := f.request("GET", "/api/v1/rides?"+query, nil, nil)
			want := 0
			if published {
				want = 1
			}
			if got := len(listedRideIDs(t, result)); got != want {
				t.Fatal("public filters exposed unpublished details", query, result.raw)
			}
		}
		preview := f.request("GET", "/rides/"+id, nil, nil)
		expect(t, preview, 200)
		if strings.Contains(preview.raw, "PRIVATE TITLE") != published {
			t.Fatal("crawler preview ignored publication state", preview.raw)
		}
		for _, viewer := range []*actor{owner, member} {
			result := f.request("GET", path, nil, viewer)
			expect(t, result, 200)
			ride := result.body["ride"].(map[string]any)
			if ride["title"] != "PRIVATE TITLE" || ride["meeting_point"] != "PRIVATE MEETING" || ride["capacity"] != float64(5) || ride["public_details_published"] != published || ride["distance_override_km"] != float64(80) {
				t.Fatal("publication state changed approved access", result.raw)
			}
		}
	}
	check(false)
	// An ordinary legacy edit must not implicitly publish anything.
	expect(t, f.request("PATCH", path, map[string]any{"plan_version": 1, "description": "A public description"}, owner), 200)
	check(false)
	expect(t, f.request("PATCH", path, map[string]any{"plan_version": 2, "publish_public_details": true}, member), 403)
	check(false)
	expect(t, f.request("PATCH", path, map[string]any{"plan_version": 2, "publish_public_details": true}, owner), 200)
	check(true)
	expect(t, f.request("PATCH", path, map[string]any{"plan_version": 3, "publish_public_details": false}, owner), 200)
	check(false)
}

func TestAutomaticDistanceStaysAutomaticAcrossPersistenceAndLocationEdits(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011239803", "Distance organizer")
	id := f.ride(owner, 5)
	path := "/api/v1/rides/" + id
	plan := models.MapPlan{Points: []models.MapPoint{{Lat: 43.24, Lon: 76.89, Label: "PRIVATE MEETING"}, {Lat: 43.25, Lon: 76.91, Label: "PRIVATE DESTINATION"}}, DistanceMeters: 100000}
	if _, err := f.db.Exec(context.Background(), "UPDATE rides SET map_plan=$2,public_details_published=true WHERE id=$1", id, plan); err != nil {
		t.Fatal(err)
	}
	read := f.request("GET", path, nil, owner)
	expect(t, read, 200)
	ride := read.body["ride"].(map[string]any)
	if ride["distance_override_km"] != float64(0) || ride["distance_km"] != float64(100) {
		t.Fatal("automatic distance did not retain its source", read.raw)
	}
	expect(t, f.request("PATCH", path, map[string]any{"plan_version": 1, "description": "Ordinary edit"}, owner), 200)
	var stored int
	if err := f.db.QueryRow(context.Background(), "SELECT distance_km FROM rides WHERE id=$1", id).Scan(&stored); err != nil || stored != 0 {
		t.Fatal("ordinary edit persisted the calculated estimate", stored, err)
	}
	plan.DistanceMeters = 200000
	if _, err := f.db.Exec(context.Background(), "UPDATE rides SET map_plan=$2 WHERE id=$1", id, plan); err != nil {
		t.Fatal(err)
	}
	read = f.request("GET", path, nil, owner)
	expect(t, read, 200)
	if ride = read.body["ride"].(map[string]any); ride["distance_km"] != float64(200) || ride["distance_override_km"] != float64(0) {
		t.Fatal("new route retained an old automatic estimate", read.raw)
	}
	expect(t, f.request("PATCH", path, map[string]any{"plan_version": 2, "distance_km": 75}, owner), 200)
	read = f.request("GET", path, nil, nil)
	expect(t, read, 200)
	if read.body["ride"].(map[string]any)["distance_km"] != float64(75) {
		t.Fatal("manual override did not take precedence", read.raw)
	}
	expect(t, f.request("PATCH", path, map[string]any{"plan_version": 3, "distance_km": 0}, owner), 200)
	read = f.request("GET", path, nil, owner)
	expect(t, read, 200)
	if ride = read.body["ride"].(map[string]any); ride["distance_km"] != float64(200) || ride["distance_override_km"] != float64(0) {
		t.Fatal("clearing override failed to restore current estimate", read.raw)
	}
}
