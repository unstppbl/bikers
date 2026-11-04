package httpapi

import (
	"strings"
	"testing"
	"time"

	"bikers.kz/backend/internal/models"
	ics "github.com/arran4/golang-ical"
)

func TestCalendarEscapesPrivateUnicodeTextAndRepresentsOnlyKnownTimes(t *testing.T) {
	ride := models.Ride{
		RideSummary: models.RideSummary{ID: "a7df84ac-3d06-435d-b858-e2392850361f", Description: "Local ride\nwith friends", Status: "published"},
		Title:       "Алматы, evening; ride\nBEGIN:VEVENT", MeetingPoint: "A; B, C\\D",
		StartsAt: time.Date(2026, 11, 8, 4, 30, 0, 0, time.FixedZone("Almaty", 5*3600)),
		Timezone: "Asia/Almaty", PlanVersion: 7,
	}
	at := time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)
	link := "https://bikers.kz/rides/" + ride.ID
	serialized := serializeRideCalendar(ride, link, "ru", at)
	if strings.Contains(strings.ReplaceAll(serialized, "\r\n", ""), "\n") || strings.Contains(serialized, "DTEND") || strings.Contains(serialized, "DURATION") {
		t.Fatal("calendar must use CRLF and not invent a ride end")
	}
	for _, value := range []string{"UID:" + ride.ID + "@bikers.kz", "SEQUENCE:7", "DTSTART:20261107T233000Z", "CLASS:PRIVATE", "STATUS:CONFIRMED", "URL:" + link} {
		if !strings.Contains(serialized, value+"\r\n") {
			t.Fatalf("missing calendar value %q", value)
		}
	}
	parsed, err := ics.ParseCalendar(strings.NewReader(serialized))
	if err != nil || len(parsed.Events()) != 1 {
		t.Fatal("calendar serialization allowed text to create another event", err)
	}
	if parsed.Events()[0].GetProperty(ics.ComponentPropertySummary).Value != ride.Title {
		t.Fatal("calendar did not preserve rider-supplied unicode text")
	}
	ride.Status, ride.PlanVersion = "cancelled", 8
	updated := serializeRideCalendar(ride, link, "kk", at)
	if !strings.Contains(updated, "STATUS:CANCELLED\r\n") || !strings.Contains(updated, "SEQUENCE:8\r\n") || !strings.Contains(updated, "UID:"+ride.ID+"@bikers.kz\r\n") {
		t.Fatal("cancellation lost stable identity or latest plan version")
	}
}
