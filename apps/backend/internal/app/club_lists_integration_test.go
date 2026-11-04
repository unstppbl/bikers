package app

import (
	"context"
	"fmt"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func seedListedClub(t *testing.T, f *fixture, owner *actor, id, city, name, status string) string {
	t.Helper()
	if id == "" {
		id = uuid.NewString()
	}
	_, err := f.db.Exec(context.Background(), `INSERT INTO clubs(id,owner_id,name,city,description,status,created_at,updated_at) VALUES($1,$2,$3,$4,'Public club description',$5,$6,$6)`, id, owner.id, name, city, status, f.now)
	if err != nil {
		t.Fatal(err)
	}
	seedListedClubMembership(t, f, id, owner, "approved")
	return id
}
func seedListedClubMembership(t *testing.T, f *fixture, club string, rider *actor, state string) {
	t.Helper()
	_, err := f.db.Exec(context.Background(), `INSERT INTO club_memberships(club_id,user_id,state,created_at,updated_at) VALUES($1,$2,$3,$4,$4) ON CONFLICT(club_id,user_id) DO UPDATE SET state=excluded.state`, club, rider.id, state, f.now)
	if err != nil {
		t.Fatal(err)
	}
}
func listedClubIDs(t *testing.T, r response) []string {
	t.Helper()
	expect(t, r, 200)
	ids := []string{}
	for _, raw := range r.body["clubs"].([]any) {
		ids = append(ids, raw.(map[string]any)["id"].(string))
	}
	return ids
}
func TestClubListsSeparateOwnershipMembershipAndPendingWithModeration(t *testing.T) {
	f := newFixture(t)
	me := f.login("+77011234567", "Me")
	other := f.login("+77021234567", "Other")
	own := seedListedClub(t, f, me, "", "Almaty", "My active club", "active")
	ownClosed := seedListedClub(t, f, me, "", "Almaty", "My closed club", "closed")
	joined := seedListedClub(t, f, other, "", "Almaty", "Joined active", "active")
	seedListedClubMembership(t, f, joined, me, "approved")
	joinedClosed := seedListedClub(t, f, other, "", "Almaty", "Joined closed", "closed")
	seedListedClubMembership(t, f, joinedClosed, me, "approved")
	pending := seedListedClub(t, f, other, "", "Almaty", "Pending active", "active")
	seedListedClubMembership(t, f, pending, me, "requested")
	pendingClosed := seedListedClub(t, f, other, "", "Almaty", "Pending closed", "closed")
	seedListedClubMembership(t, f, pendingClosed, me, "requested")
	for _, state := range []string{"rejected", "removed", "left"} {
		id := seedListedClub(t, f, other, "", "Almaty", state, "active")
		seedListedClubMembership(t, f, id, me, state)
	}
	hiddenOwn := seedListedClub(t, f, me, "", "Almaty", "Hidden own", "active")
	hiddenJoined := seedListedClub(t, f, other, "", "Almaty", "Hidden joined", "active")
	seedListedClubMembership(t, f, hiddenJoined, me, "approved")
	hiddenPending := seedListedClub(t, f, other, "", "Almaty", "Hidden pending", "active")
	seedListedClubMembership(t, f, hiddenPending, me, "requested")
	for _, id := range []string{hiddenOwn, hiddenJoined, hiddenPending} {
		if _, err := f.db.Exec(context.Background(), `INSERT INTO moderation_hidden(target_type,target_id,hidden_at) VALUES('club',$1,$2)`, id, f.now); err != nil {
			t.Fatal(err)
		}
	}
	for view, want := range map[string][]string{"organizing": {own, ownClosed, hiddenOwn}, "joined": {joined, joinedClosed, hiddenJoined}, "pending": {pending}} {
		r := f.request("GET", "/api/v1/clubs?view="+view, nil, me)
		got := listedClubIDs(t, r)
		sort.Strings(got)
		sort.Strings(want)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s got %v want %v", view, got, want)
		}
		for _, field := range []string{"owner_id", "version", "member_count"} {
			if strings.Contains(r.raw, `"`+field+`"`) {
				t.Fatalf("list leaked full club field %s: %s", field, r.raw)
			}
		}
		expect(t, f.request("GET", "/api/v1/clubs?view="+view, nil, nil), 401)
	}
	public := f.request("GET", "/api/v1/clubs", nil, nil)
	if len(listedClubIDs(t, public)) != 6 || strings.Contains(public.raw, "Hidden") || strings.Contains(public.raw, "closed") || strings.Contains(public.raw, "can_manage") || strings.Contains(public.raw, "membership_state") {
		t.Fatal("public visibility changed", public.raw)
	}
	for _, query := range []string{"view=unknown", "city=" + strings.Repeat("x", 161), "q=" + strings.Repeat("x", 201), "cursor=bad"} {
		expect(t, f.request("GET", "/api/v1/clubs?"+query, nil, me), 422)
	}
}

func TestClubSearchFiltersBeforePaginationAndBindsCursorToActorAndQuery(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011234567", "Owner")
	other := f.login("+77021234567", "Other")
	for i := 1; i <= 40; i++ {
		seedListedClub(t, f, owner, fmt.Sprintf("00000000-0000-4000-8000-%012d", i), "Astana", "Touring elsewhere", "active")
	}
	want := []string{}
	for i := 41; i <= 65; i++ {
		city := "Almaty"
		if i%2 == 0 {
			city = "Алма-Ата"
		}
		want = append(want, seedListedClub(t, f, owner, fmt.Sprintf("00000000-0000-4000-8000-%012d", i), city, fmt.Sprintf("Touring %d", i), "active"))
	}
	seedListedClub(t, f, owner, "", "Almaty", "A different club", "active")
	path := "/api/v1/clubs?view=discover&city=" + url.QueryEscape("Алматы") + "&q=tOuRiNg"
	first := f.request("GET", path, nil, owner)
	firstIDs := listedClubIDs(t, first)
	if !reflect.DeepEqual(firstIDs, want[:20]) {
		t.Fatalf("filter applied after page limit: got %v", firstIDs)
	}
	cursor := first.body["next_cursor"].(string)
	if cursor == "" {
		t.Fatal("expected second page")
	}
	second := f.request("GET", path+"&cursor="+url.QueryEscape(cursor), nil, owner)
	if got := listedClubIDs(t, second); !reflect.DeepEqual(got, want[20:]) || second.body["next_cursor"] != "" {
		t.Fatalf("incorrect second page %v", got)
	}
	expect(t, f.request("GET", path+"&cursor="+url.QueryEscape(cursor), nil, other), 422)
	expect(t, f.request("GET", path+"&cursor="+url.QueryEscape(cursor), nil, nil), 422)
	expect(t, f.request("GET", "/api/v1/clubs?city=Astana&q=tOuRiNg&cursor="+url.QueryEscape(cursor), nil, owner), 422)
	expect(t, f.request("GET", "/api/v1/clubs?city=Almaty&q=other&cursor="+url.QueryEscape(cursor), nil, owner), 422)
	if got := listedClubIDs(t, f.request("GET", "/api/v1/clubs?q="+url.QueryEscape("%"), nil, nil)); len(got) != 0 {
		t.Fatal("name query interpreted SQL wildcard", got)
	}
	f.startServer(f.gateway)
	expect(t, f.request("GET", path+"&cursor="+url.QueryEscape(cursor), nil, owner), 422)
}
