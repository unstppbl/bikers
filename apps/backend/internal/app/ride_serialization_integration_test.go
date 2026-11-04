package app

import "testing"

func TestCreateReturnsTheStoredRideTimeRepresentation(t *testing.T) {
	for _, stamp := range []string{"2026-11-09T12:15:00Z", "2026-11-09T12:15:00+02:00", "2026-11-09T12:15:00+05:00"} {
		t.Run(stamp, func(t *testing.T) {
			f := newFixture(t)
			owner := f.login("+77011234567", "Organizer")
			created := f.request("POST", "/api/v1/rides", map[string]any{"city": "Almaty", "description": "Timezone round trip", "title": "Plan", "starts_at": stamp, "timezone": "Asia/Almaty", "meeting_point": "Meeting", "capacity": 2}, owner)
			expect(t, created, 201)
			ride := created.body["ride"].(map[string]any)
			stored := f.request("GET", "/api/v1/rides/"+ride["id"].(string), nil, owner)
			expect(t, stored, 200)
			if ride["starts_at"] != stored.body["ride"].(map[string]any)["starts_at"] {
				t.Fatalf("create response bypassed stored-record mapping: create=%s read=%s", ride["starts_at"], stored.body["ride"].(map[string]any)["starts_at"])
			}
		})
	}
}
