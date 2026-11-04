package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func seedListedRide(t *testing.T, f *fixture, owner *actor, id, city, description string, starts time.Time, status string) string {
	t.Helper()
	if id == "" {
		id = uuid.NewString()
	}
	_, err := f.db.Exec(context.Background(), `INSERT INTO rides
		(id,organizer_id,city,description,title,starts_at,timezone,meeting_point,capacity,status,created_at)
		VALUES($1,$2,$3,$4,'PRIVATE PLAN',$5,'Asia/Almaty','PRIVATE LOCATION',10,$6,$7)`,
		id, owner.id, city, description, starts, status, f.now)
	if err != nil {
		t.Fatal(err)
	}
	seedListedParticipation(t, f, id, owner, "approved")
	return id
}

func seedListedParticipation(t *testing.T, f *fixture, ride string, rider *actor, state string) {
	t.Helper()
	if _, err := f.db.Exec(context.Background(), `INSERT INTO participation(ride_id,user_id,state,updated_at)
		VALUES($1,$2,$3,$4) ON CONFLICT(ride_id,user_id) DO UPDATE SET state=excluded.state`, ride, rider.id, state, f.now); err != nil {
		t.Fatal(err)
	}
}

func listedRideIDs(t *testing.T, r response) []string {
	t.Helper()
	expect(t, r, 200)
	out := []string{}
	for _, row := range r.body["rides"].([]any) {
		out = append(out, row.(map[string]any)["id"].(string))
	}
	return out
}

func TestCitySuggestionsAndWritesShareCanonicalCityValues(t *testing.T) {
	f := newFixture(t)
	public := f.request("GET", "/api/v1/cities", nil, nil)
	expect(t, public, 200)
	found := false
	for _, row := range public.body["cities"].([]any) {
		city := row.(map[string]any)
		if city["value"] != "Aktobe" {
			continue
		}
		names := city["names"].(map[string]any)
		if names["en"] != "Aktobe" || names["ru"] != "Актобе" || names["kk"] != "Ақтөбе" || len(city["aliases"].([]any)) == 0 {
			t.Fatal("city choices lack useful localized labels and aliases", city)
		}
		found = true
	}
	if !found {
		t.Fatal("public city suggestions omitted the localized choice")
	}
	owner := f.login("+77011234567", "Organizer")
	profile := f.request("PATCH", "/api/v1/me/profile", map[string]any{"city": " Нұр-Сұлтан "}, owner)
	expect(t, profile, 200)
	if profile.body["user"].(map[string]any)["profile"].(map[string]any)["city"] != "Astana" {
		t.Fatal("profile alias was not canonicalized", profile.raw)
	}
	club := f.request("POST", "/api/v1/clubs", map[string]any{"name": "Local riders", "city": " Ақтөбе ", "description": "Community rides"}, owner)
	expect(t, club, 201)
	clubRow := club.body["club"].(map[string]any)
	if clubRow["city"] != "Aktobe" {
		t.Fatal("club alias was not canonicalized", club.raw)
	}
	ride := f.request("POST", "/api/v1/rides", map[string]any{
		"city": " Алма-Ата ", "description": "A city ride", "title": "Private plan", "starts_at": "2026-11-07T23:30:00Z",
		"timezone": "Asia/Almaty", "meeting_point": "Private meeting", "capacity": 2,
	}, owner)
	expect(t, ride, 201)
	rideRow := ride.body["ride"].(map[string]any)
	if rideRow["city"] != "Almaty" {
		t.Fatal("ride alias was not canonicalized", ride.raw)
	}
	var profileCity, clubCity, rideCity string
	err := f.db.QueryRow(context.Background(), `SELECT
		(SELECT city FROM users WHERE id=$1),
		(SELECT city FROM clubs WHERE id=$2),
		(SELECT city FROM rides WHERE id=$3)`, owner.id, clubRow["id"], rideRow["id"]).Scan(&profileCity, &clubCity, &rideCity)
	if err != nil || profileCity != "Astana" || clubCity != "Aktobe" || rideCity != "Almaty" {
		t.Fatal("canonical response values did not persist", err)
	}
}

func TestRideListsKeepPersonalViewsExclusiveAndRespectModeration(t *testing.T) {
	f := newFixture(t)
	me := f.login("+77011234567", "Me")
	other := f.login("+77021234567", "Other")
	future, past := f.now.Add(48*time.Hour), f.now.Add(-24*time.Hour)
	own := seedListedRide(t, f, me, "", "Almaty", "Own future", future, "published")
	joined := seedListedRide(t, f, other, "", "Almaty", "Joined", future, "published")
	seedListedParticipation(t, f, joined, me, "approved")
	pending := seedListedRide(t, f, other, "", "Almaty", "Pending", future, "published")
	seedListedParticipation(t, f, pending, me, "requested")
	seedListedRide(t, f, other, "", "Almaty", "Unrelated", future, "published")
	previous := []string{seedListedRide(t, f, me, "", "Almaty", "Own past", past, "published")}
	for _, state := range []string{"approved", "requested", "rejected", "left", "removed"} {
		id := seedListedRide(t, f, other, "", "Almaty", "Past "+state, past, "published")
		seedListedParticipation(t, f, id, me, state)
		if state == "approved" || state == "requested" {
			previous = append(previous, id)
		}
	}
	previous = append(previous, seedListedRide(t, f, me, "", "Almaty", "Own cancelled", future, "cancelled"))
	for _, state := range []string{"approved", "requested"} {
		id := seedListedRide(t, f, other, "", "Almaty", "Cancelled "+state, future, "cancelled")
		seedListedParticipation(t, f, id, me, state)
		previous = append(previous, id)
	}
	hiddenOwned := seedListedRide(t, f, me, "", "Almaty", "Hidden owned", future, "published")
	hiddenJoined := seedListedRide(t, f, other, "", "Almaty", "Hidden joined", future, "published")
	seedListedParticipation(t, f, hiddenJoined, me, "approved")
	hiddenPending := seedListedRide(t, f, other, "", "Almaty", "Hidden pending", future, "published")
	seedListedParticipation(t, f, hiddenPending, me, "requested")
	for _, id := range []string{hiddenOwned, hiddenJoined, hiddenPending} {
		if _, err := f.db.Exec(context.Background(), "INSERT INTO moderation_hidden(target_type,target_id,hidden_at) VALUES('ride',$1,$2)", id, f.now); err != nil {
			t.Fatal(err)
		}
	}
	for view, expected := range map[string][]string{
		"organizing": {own, hiddenOwned}, "joined": {joined, hiddenJoined},
		"pending": {pending}, "past": previous,
	} {
		r := f.request("GET", "/api/v1/rides?view="+view, nil, me)
		actual := listedRideIDs(t, r)
		sort.Strings(actual)
		sort.Strings(expected)
		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("%s: got %v; want %v", view, actual, expected)
		}
		for _, secret := range []string{"starts_at", "organizer_id", "meeting_point", "PRIVATE"} {
			if strings.Contains(r.raw, secret) {
				t.Fatalf("list exposed private plan: %s", r.raw)
			}
		}
		expect(t, f.request("GET", "/api/v1/rides?view="+view, nil, nil), 401)
	}
	guest := f.request("GET", "/api/v1/rides", nil, nil)
	if ids := listedRideIDs(t, guest); len(ids) != 4 || strings.Contains(guest.raw, "Hidden") || strings.Contains(guest.raw, "participation_state") || strings.Contains(guest.raw, "can_manage") {
		t.Fatal("guest discovery visibility changed", guest.raw)
	}
	privateSearch := f.request("GET", "/api/v1/rides?q=PRIVATE", nil, me)
	if len(listedRideIDs(t, privateSearch)) != 0 {
		t.Fatal("discovery search matched participant-only plan text")
	}
	for _, query := range []string{"view=unknown", "from=2026-02-30", "from=2026-11-05&to=2026-11-04", "cursor=broken"} {
		expect(t, f.request("GET", "/api/v1/rides?"+query, nil, me), 422)
	}
}

func TestRidePastPagesUseDescendingDayAndID(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011234567", "Organizer")
	expected := []string{}
	for i := range 23 {
		id := fmt.Sprintf("00000000-0000-4000-8000-%012d", i+1)
		seedListedRide(t, f, owner, id, "Almaty", "Past ride", f.now.Add(-48*time.Hour), "published")
		expected = append([]string{id}, expected...)
	}
	latest := seedListedRide(t, f, owner, "", "Almaty", "Latest past ride", f.now.Add(-24*time.Hour), "published")
	expected = append([]string{latest}, expected...)
	first := f.request("GET", "/api/v1/rides?view=past", nil, owner)
	actual := listedRideIDs(t, first)
	if len(actual) != 20 || first.body["next_cursor"] == "" {
		t.Fatal("past first page is not bounded")
	}
	second := f.request("GET", "/api/v1/rides?view=past&cursor="+url.QueryEscape(first.body["next_cursor"].(string)), nil, owner)
	actual = append(actual, listedRideIDs(t, second)...)
	if !reflect.DeepEqual(actual, expected) || second.body["next_cursor"] != "" {
		t.Fatal("past pagination skipped or repeated descending date/ID keys", actual)
	}
}

func TestRideDiscoveryFiltersBeforePagingAndUsesPublicDateKeys(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011234567", "Organizer")
	// This UTC instant is the following local day; filters must match the
	// calendar date visible in the public preview, not the UTC date.
	starts := time.Date(2026, 11, 4, 23, 30, 0, 0, time.UTC)
	expected := []string{}
	for i := range 45 {
		city := "Almaty"
		if i%2 == 1 {
			city = "Алматы"
		}
		id := fmt.Sprintf("00000000-0000-4000-8000-%012d", i+1)
		expected = append(expected, seedListedRide(t, f, owner, id, city, "Scenic 100%_ loop", starts.Add(time.Duration(45-i)*time.Second), "published"))
	}
	seedListedRide(t, f, owner, "", "Astana", "Scenic 100%_ loop", starts, "published")
	seedListedRide(t, f, owner, "", "Almaty", "Scenic plain loop", starts, "published")
	seedListedRide(t, f, owner, "", "Almaty", "Scenic 100%_ loop", starts.Add(24*time.Hour), "published")
	query := "city=" + url.QueryEscape(" Алматы ") + "&from=2026-11-05&to=2026-11-05&q=" + url.QueryEscape("100%_")
	actual := []string{}
	cursor := ""
	for page, wantCount := range []int{20, 20, 5} {
		path := "/api/v1/rides?" + query
		if cursor != "" {
			path += "&cursor=" + url.QueryEscape(cursor)
		}
		r := f.request("GET", path, nil, nil)
		ids := listedRideIDs(t, r)
		if len(ids) != wantCount {
			t.Fatalf("page %d got %d rides; expected %d: %s", page, len(ids), wantCount, r.raw)
		}
		actual = append(actual, ids...)
		cursor = r.body["next_cursor"].(string)
		if page < 2 && cursor == "" || page == 2 && cursor != "" {
			t.Fatal("incorrect page continuation", r.raw)
		}
		if page == 0 {
			payload, signature, _ := strings.Cut(cursor, ".")
			decoded, err := base64.RawURLEncoding.DecodeString(payload)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]any
			if err := json.Unmarshal(decoded, &fields); err != nil {
				t.Fatal(err)
			}
			if fields["Date"] != "2026-11-05" || strings.Contains(string(decoded), "23:30") || strings.Contains(string(decoded), "starts_at") {
				t.Fatal("cursor disclosed exact private departure time")
			}
			expect(t, f.request("GET", "/api/v1/rides?city=Astana&cursor="+url.QueryEscape(cursor), nil, nil), 422)
			fields["Now"] = f.now.Add(-365 * 24 * time.Hour).Format(time.RFC3339Nano)
			tampered, err := json.Marshal(fields)
			if err != nil {
				t.Fatal(err)
			}
			forged := base64.RawURLEncoding.EncodeToString(tampered) + "." + signature
			expect(t, f.request("GET", "/api/v1/rides?"+query+"&cursor="+url.QueryEscape(forged), nil, nil), 422)
			// Moving the clock past departure must not change which rides belong
			// to the remaining pages of this already-started discovery snapshot.
			f.now = starts.Add(time.Hour)
		}
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("keyset skipped or repeated same-day rides: %v", actual)
	}
	newDiscovery := f.request("GET", "/api/v1/rides?"+query, nil, nil)
	if len(listedRideIDs(t, newDiscovery)) != 0 {
		t.Fatal("new discovery snapshot retained departed rides")
	}
}
