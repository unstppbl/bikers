package app

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestPrivateCalendarExportTracksPlanVersionsCancellationAndLostAccess(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011234567", "Organizer")
	member := f.login("+77021234567", "Participant")
	outsider := f.login("+77051234567", "Outsider")
	id := f.ride(owner, 5)
	path := "/api/v1/rides/" + id + "/calendar"
	expect(t, f.request("GET", path, nil, nil), 401)
	expect(t, f.request("GET", path, nil, outsider), 403)
	f.join(id, member)
	expect(t, f.request("GET", path, nil, member), 403)
	expect(t, f.decision(id, owner, member, "approve"), 200)
	fetch := func(actor *actor) string {
		t.Helper()
		req, err := http.NewRequest("GET", f.ts.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.AddCookie(actor.cookie)
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		content, err := io.ReadAll(r.Body)
		if err != nil || r.StatusCode != 200 || r.Header.Get("Content-Type") != "text/calendar; charset=utf-8" || r.Header.Get("Cache-Control") != "no-store" || !strings.Contains(r.Header.Get("Content-Disposition"), "attachment;") || !strings.Contains(r.Header.Get("Content-Disposition"), "ride-"+id+".ics") {
			t.Fatalf("private calendar download headers/status invalid: status=%d error=%v", r.StatusCode, err)
		}
		return string(content)
	}
	for _, actor := range []*actor{owner, member} {
		value := fetch(actor)
		for _, field := range []string{"UID:" + id + "@bikers.kz", "SUMMARY:PRIVATE TITLE", "LOCATION:SECRET MEETING", "SEQUENCE:1", "DTSTART:20261107T233000Z", "CLASS:PRIVATE", "URL:" + f.ts.URL + "/rides/" + id} {
			if !strings.Contains(value, field+"\r\n") {
				t.Fatalf("authorized export missing %q", field)
			}
		}
		if strings.Contains(value, "+770") || strings.Contains(value, "ATTENDEE:") || strings.Contains(value, "DTEND:") {
			t.Fatal("export includes account identifiers, roster or invented end time")
		}
	}
	expect(t, f.request("PATCH", "/api/v1/rides/"+id, map[string]any{"plan_version": 1, "starts_at": "2026-11-08T00:30:00Z", "meeting_point": "Updated place"}, owner), 200)
	updated := fetch(member)
	if !strings.Contains(updated, "SEQUENCE:2\r\n") || !strings.Contains(updated, "DTSTART:20261108T003000Z\r\n") || !strings.Contains(updated, "LOCATION:Updated place\r\n") || !strings.Contains(updated, "UID:"+id+"@bikers.kz\r\n") {
		t.Fatal("refreshed download does not identify the updated plan")
	}
	expect(t, f.decision(id, owner, member, "remove"), 200)
	expect(t, f.request("GET", path, nil, member), 403)
	expect(t, f.request("POST", "/api/v1/rides/"+id+"/cancel", map[string]any{"plan_version": 2}, owner), 200)
	cancelled := fetch(owner)
	if !strings.Contains(cancelled, "STATUS:CANCELLED\r\n") || !strings.Contains(cancelled, "SEQUENCE:3\r\n") || !strings.Contains(cancelled, "UID:"+id+"@bikers.kz\r\n") {
		t.Fatal("cancellation cannot update the same exported event")
	}
}
