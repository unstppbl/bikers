package models

import (
	"encoding/json"
	"testing"
)

// These are the pre-refactor public JSON shapes, including ordering and omissions.
func TestPublicSerializationRemainsStable(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  string
	}{
		{"profile", Profile{}, `{"display_name":"","city":"","languages":null,"interests":"","locale":""}`},
		{"user", User{}, `{"id":"","profile":{"display_name":"","city":"","languages":null,"interests":"","locale":""},"onboarding_complete":false}`},
		{"summary", RideSummary{}, `{"id":"","city":"","date":"","description":"","status":""}`},
		{"ride", Ride{}, `{"id":"","city":"","date":"","description":"","status":"","title":"","starts_at":"0001-01-01T00:00:00Z","timezone":"","meeting_point":"","destination":"","route_url":"","expectations":"","capacity":0,"plan_version":0,"organizer_id":""}`},
		{"message", Message{}, `{"id":"","sequence":0,"sender_id":"","sender_name":"","body":"","created_at":"0001-01-01T00:00:00Z","client_message_id":""}`},
		{"event", Event{}, `{"type":"","ride_id":""}`},
		{"verification", Verification{}, `{"request_id":"","phone_number":"","verification_status":null}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.value)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Fatalf("wire format changed\ngot %s\nwant %s", got, tc.want)
			}
		})
	}
}
