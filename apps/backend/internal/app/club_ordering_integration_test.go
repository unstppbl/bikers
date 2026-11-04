package app

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestClubHistoricalLinksCannotHideUpcomingPublishedRides(t *testing.T) {
	f := newFixture(t)
	// A clock far beyond the wall clock also catches repositories that ignore
	// the manager's supplied time when classifying an upcoming ride.
	f.now = time.Date(2040, 1, 15, 12, 0, 0, 0, time.UTC)
	owner := f.login("+77011234567", "Club organizer")
	created := f.request("POST", "/api/v1/clubs", map[string]any{
		"name": "City riders", "city": "Almaty", "description": "Public club description",
	}, owner)
	expect(t, created, 201)
	clubID := created.body["club"].(map[string]any)["id"].(string)
	base := "/api/v1/clubs/" + clubID + "/rides"

	// Existing linked history is a fixture, while new links exercise the actual
	// HTTP creation/link/cancellation flow. All rows belong to this test schema.
	_, err := f.db.Exec(context.Background(), `
		WITH history AS (
		 INSERT INTO rides(id,organizer_id,city,description,title,starts_at,timezone,meeting_point,capacity,status,created_at)
		 SELECT md5($2::text || ':' || n::text)::uuid,$1::uuid,'Almaty','Historical public description',
		 'PRIVATE_HISTORY_TITLE',$3::timestamptz - n * interval '1 day','Asia/Almaty','PRIVATE_HISTORY_MEETING',10,
		 CASE WHEN n % 2=0 THEN 'cancelled' ELSE 'published' END,$3::timestamptz
		 FROM generate_series(1,105) AS n
		 RETURNING id
		)
		INSERT INTO club_rides(club_id,ride_id,linked_by,linked_at)
		SELECT $2::uuid,id,$1::uuid,$3::timestamptz FROM history`, owner.id, clubID, f.now)
	if err != nil {
		t.Fatal(err)
	}

	createLinked := func(starts time.Time, description string) string {
		t.Helper()
		r := f.request("POST", "/api/v1/rides", map[string]any{
			"city": "Almaty", "description": description, "title": "PRIVATE_FUTURE_TITLE",
			"starts_at": starts.Format(time.RFC3339), "timezone": "Asia/Almaty",
			"meeting_point": "PRIVATE_FUTURE_MEETING", "capacity": 10,
		}, owner)
		expect(t, r, 201)
		id := r.body["ride"].(map[string]any)["id"].(string)
		expect(t, f.request("PUT", base+"/"+id, nil, owner), 200)
		return id
	}
	// Create out of order so insertion order cannot accidentally satisfy the
	// assertion. A cancelled future ride must not outrank published future rides.
	later := createLinked(f.now.Add(3*time.Hour), "Later city ride")
	cancelled := createLinked(f.now.Add(30*time.Minute), "Cancelled city ride")
	expect(t, f.request("POST", "/api/v1/rides/"+cancelled+"/cancel", map[string]any{"plan_version": 1}, owner), 200)
	soon := createLinked(f.now.Add(time.Hour), "Soon city ride")
	middle := createLinked(f.now.Add(2*time.Hour), "Middle city ride")

	for _, actor := range []*actor{nil, owner} {
		result := f.request("GET", base, nil, actor)
		expect(t, result, 200)
		rides := result.body["rides"].([]any)
		if len(rides) != 100 {
			t.Fatalf("linked result size=%d want bounded100", len(rides))
		}
		for index, want := range []string{soon, middle, later} {
			ride := rides[index].(map[string]any)
			if ride["id"] != want || ride["status"] != "published" {
				t.Fatalf("upcoming published ride hidden or misordered at index%d: got=%v want_id=%s", index, ride, want)
			}
		}
		for _, raw := range rides {
			if len(raw.(map[string]any)) != 5 {
				t.Fatal("linked ride response includes more than the public five-field projection")
			}
		}
		if strings.Contains(result.raw, "PRIVATE_") {
			t.Fatal("linked club rides leaked private plan content")
		}
	}
}
