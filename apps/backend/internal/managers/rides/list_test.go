package rides

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
)

func TestListValidatesFiltersAndPersonalAuthentication(t *testing.T) {
	m, repo, _, owner := rideFixture(t)
	for _, view := range []string{"organizing", "joined", "pending", "past"} {
		_, err := m.List(context.Background(), nil, models.RideListInput{View: view})
		requireFault(t, err, "authentication_required")
	}
	for _, input := range []models.RideListInput{
		{View: "unknown"}, {From: "2026-02-30"}, {To: "2026-2-03"},
		{From: "2026-11-04", To: "2026-11-03"}, {City: strings.Repeat("x", 161)},
		{Search: strings.Repeat("x", 201)},
	} {
		_, err := m.List(context.Background(), &models.Session{User: owner}, input)
		requireFault(t, err, "invalid_query")
	}
	for _, view := range []string{"discover", "organizing", "joined", "pending", "past"} {
		page, err := m.List(context.Background(), &models.Session{User: owner}, models.RideListInput{View: view, City: " Алматы ", Search: " scenic ", From: "2026-11-03", To: "2026-11-03"})
		if err != nil || page.Rides == nil || page.NextCursor != "" {
			t.Fatal("empty page must serialize as an array", err)
		}
		q := repo.listQuery
		if q.UserID != owner.ID || q.Limit != 21 || q.FromDate != "2026-11-03" || q.ToDate != "2026-11-03" || q.Search != "scenic" || !reflect.DeepEqual(q.CityAliases, models.CityAliases("Almaty")) {
			t.Fatalf("filters were not passed to persistence: %+v", q)
		}
		if q.OrganizerOnly != (view == "organizing") || q.ExcludeOrganizer != (view == "joined" || view == "pending") || q.RelatedOnly != (view == "past") || q.Past != (view == "past") {
			t.Fatalf("wrong view policy for %s: %+v", view, q)
		}
		if expected := map[string]string{"joined": "approved", "pending": "requested"}[view]; q.ParticipationState != expected {
			t.Fatalf("wrong participation selection for %s: %+v", view, q)
		}
	}
}

func TestListCursorRetainsSnapshotAndOnlyPublicRideFields(t *testing.T) {
	m, repo, _, owner := rideFixture(t)
	for i := range 21 {
		repo.list = append(repo.list, models.RideListing{Summary: models.RideSummary{ID: fmt.Sprintf("00000000-0000-4000-8000-%012d", i+1), Date: "2026-11-05", Description: "Public", ParticipationState: "approved"}, OrganizerID: owner.ID})
	}
	input := models.RideListInput{City: "Almaty", From: "2026-11-03", Search: "Public"}
	page, err := m.List(context.Background(), nil, input)
	if err != nil || len(page.Rides) != 20 || page.NextCursor == "" {
		t.Fatal("expected a bounded page and next cursor", err)
	}
	for _, ride := range page.Rides {
		if ride.CanManage != nil || ride.ParticipationState != "" {
			t.Fatal("guest page leaked private participation")
		}
	}
	ownerPage, err := m.List(context.Background(), &models.Session{User: owner}, input)
	if err != nil || ownerPage.NextCursor == "" {
		t.Fatal("owner cursor missing", err)
	}
	payload, _, _ := strings.Cut(page.NextCursor, ".")
	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		t.Fatal(err)
	}
	var cursor listCursor
	if err = json.Unmarshal(decoded, &cursor); err != nil {
		t.Fatal(err)
	}
	if cursor.Date != "2026-11-05" || cursor.ID != page.Rides[19].ID || strings.Contains(string(decoded), "StartsAt") || strings.Contains(string(decoded), "starts_at") {
		t.Fatal("cursor must key only on the public day and ID")
	}
	snapshot := repo.listQuery.Now
	m.now = func() time.Time { return snapshot.Add(time.Hour) }
	input.Cursor = page.NextCursor
	repo.list = nil
	last, err := m.List(context.Background(), nil, input)
	if err != nil || len(last.Rides) != 0 || last.NextCursor != "" || !repo.listQuery.Now.Equal(snapshot) || repo.listQuery.AfterDate != cursor.Date || repo.listQuery.AfterID != cursor.ID {
		t.Fatal("next page lost stable snapshot or key", err)
	}
	for _, change := range []func(*models.RideListInput){
		func(i *models.RideListInput) { i.City = "Astana" },
		func(i *models.RideListInput) { i.From = "2026-11-04" },
		func(i *models.RideListInput) { i.To = "2026-11-10" },
		func(i *models.RideListInput) { i.Search = "other" },
	} {
		other := input
		change(&other)
		_, err := m.List(context.Background(), nil, other)
		requireFault(t, err, "invalid_cursor")
	}
	input.Cursor = ownerPage.NextCursor
	otherView := input
	otherView.View = "past"
	_, err = m.List(context.Background(), &models.Session{User: owner}, otherView)
	requireFault(t, err, "invalid_cursor")
	_, err = m.List(context.Background(), &models.Session{User: models.User{ID: uuid.NewString()}}, input)
	requireFault(t, err, "invalid_cursor")
	for _, invalid := range []string{"not-a-cursor", strings.Repeat("x", 2049), base64.RawURLEncoding.EncodeToString([]byte(`{}`))} {
		input.Cursor = invalid
		_, err := m.List(context.Background(), nil, input)
		requireFault(t, err, "invalid_cursor")
	}
}

func TestListCursorAuthenticatesSnapshotQueryAndCaller(t *testing.T) {
	m, repo, events, owner := rideFixture(t)
	for range 21 {
		repo.list = append(repo.list, models.RideListing{Summary: models.RideSummary{ID: uuid.NewString(), Date: "2026-11-05"}, OrganizerID: owner.ID})
	}
	input := models.RideListInput{City: "Almaty"}
	page, err := m.List(context.Background(), nil, input)
	if err != nil || page.NextCursor == "" {
		t.Fatal("page cursor missing", err)
	}
	if len(page.NextCursor) > rideCursorLimit {
		t.Fatal("issued cursor exceeds accepted limit")
	}
	payload, signature, _ := strings.Cut(page.NextCursor, ".")
	data, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		t.Fatal(err)
	}
	var cursor listCursor
	if err := json.Unmarshal(data, &cursor); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*listCursor){
		"historical cutoff": func(c *listCursor) { c.Now = c.Now.Add(-365 * 24 * time.Hour) },
		"query binding":     func(c *listCursor) { c.City = "Astana" },
		"caller binding":    func(c *listCursor) { c.User = owner.ID },
		"ordering key":      func(c *listCursor) { c.Date = "2026-01-01" },
	} {
		t.Run(name, func(t *testing.T) {
			changed := cursor
			mutate(&changed)
			encoded, err := json.Marshal(changed)
			if err != nil {
				t.Fatal(err)
			}
			input.Cursor = base64.RawURLEncoding.EncodeToString(encoded) + "." + signature
			before := repo.listQuery
			_, err = m.List(context.Background(), nil, input)
			requireFault(t, err, "invalid_cursor")
			if !reflect.DeepEqual(repo.listQuery, before) {
				t.Fatal("unauthenticated cursor reached persistence")
			}
		})
	}
	for _, token := range []string{payload, payload + ".", payload + "." + base64.RawURLEncoding.EncodeToString([]byte("short")), strings.Repeat("x", rideCursorLimit+1)} {
		input.Cursor = token
		_, err := m.List(context.Background(), nil, input)
		requireFault(t, err, "invalid_cursor")
	}
	input.Cursor = page.NextCursor
	restarted := New(repo, events, m.now)
	_, err = restarted.List(context.Background(), nil, input)
	requireFault(t, err, "invalid_cursor")
	// HTML-sensitive ASCII takes six JSON bytes per input byte. Valid maximum
	// filters must still produce an accepted token after escaping and signing.
	maxInput := models.RideListInput{City: strings.Repeat("<", 160), Search: strings.Repeat(">", 200)}
	maxPage, err := m.List(context.Background(), nil, maxInput)
	if err != nil || maxPage.NextCursor == "" || len(maxPage.NextCursor) > rideCursorLimit {
		t.Fatal("maximum valid filters produced an unusable cursor", err)
	}
	maxInput.Cursor = maxPage.NextCursor
	if _, err := m.List(context.Background(), nil, maxInput); err != nil {
		t.Fatal("maximum valid signed cursor was rejected", err)
	}
}
