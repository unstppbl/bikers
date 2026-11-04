package app

import (
	"context"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func catalogueBike(t *testing.T, f *fixture, owner *actor, fields map[string]any) map[string]any {
	t.Helper()
	r := f.request("POST", "/api/v1/me/bikes", fields, owner)
	expect(t, r, 201)
	return r.body["bike"].(map[string]any)
}

func TestPublicBikeCataloguePreservesSharingProjectionModerationAndBlocks(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011234801", "Catalogue owner")
	other := f.login("+77011234802", "Catalogue viewer")
	admin := f.login("+77011234803", "Catalogue moderator")
	grantTestAdmin(t, f, admin)
	public := catalogueBike(t, f, owner, map[string]any{"model_id": "yamaha-mt-07", "year": 2020, "visibility": "public"})
	id := public["id"].(string)
	catalogueBike(t, f, owner, map[string]any{"make": "PRIVATE MAKE", "model": "PRIVATE MODEL", "year": 2020})
	hidden := catalogueBike(t, f, owner, map[string]any{"make": "Hidden Make", "model": "Hidden Model", "year": 2020, "visibility": "public"})
	expect(t, trustAction(f, admin, "hide_content", "bike", hidden["id"].(string), "Hide catalogue fixture", ""), 200)
	expect(t, f.request("POST", "/api/v1/me/bikes/"+id+"/maintenance", map[string]any{"service_date": "2026-11-02", "mileage_km": 12345, "work_performed": "PRIVATE SERVICE NOTE"}, owner), 201)
	expect(t, f.request("PUT", "/api/v1/me/blocks/"+owner.id, nil, other), 200)
	var anonymous map[string]any
	for _, viewer := range []*actor{nil, owner, other} {
		r := f.request("GET", "/api/v1/bikes", nil, viewer)
		expect(t, r, 200)
		bikes := r.body["bikes"].([]any)
		if len(bikes) != 1 || r.body["next_cursor"] != "" {
			t.Fatalf("private/hidden bike reached catalogue: %s", r.raw)
		}
		bike := bikes[0].(map[string]any)
		if len(bike) != 5 || bike["id"] != id || bike["make"] != "Yamaha" || bike["model"] != "MT-07" || bike["model_id"] != "yamaha-mt-07" || bike["year"] != float64(2020) {
			t.Fatalf("public catalogue projection changed: %s", r.raw)
		}
		for _, forbidden := range []string{owner.id, "owner_id", "PRIVATE", "version", "visibility", "can_manage", "mileage", "created_at", "updated_at"} {
			if strings.Contains(r.raw, forbidden) {
				t.Fatalf("catalogue exposed %q: %s", forbidden, r.raw)
			}
		}
		if anonymous == nil {
			anonymous = r.body
		} else if !reflect.DeepEqual(anonymous, r.body) {
			t.Fatal("catalogue projection depends on session or bike ownership")
		}
	}
	expect(t, f.request("PATCH", "/api/v1/me/bikes/"+id, map[string]any{"version": 1, "visibility": "private"}, owner), 200)
	empty := f.request("GET", "/api/v1/bikes", nil, nil)
	expect(t, empty, 200)
	if len(empty.body["bikes"].([]any)) != 0 || empty.body["next_cursor"] != "" {
		t.Fatalf("unpublished bike remained discoverable: %s", empty.raw)
	}
	expect(t, f.request("GET", "/api/v1/bikes/"+id, nil, nil), 404)
	expect(t, f.request("PATCH", "/api/v1/me/bikes/"+id, map[string]any{"version": 2, "visibility": "public"}, owner), 200)
	if _, err := f.db.Exec(context.Background(), "UPDATE users SET status='suspended' WHERE id=$1", owner.id); err != nil {
		t.Fatal(err)
	}
	// Existing public basic-bike access survives account restriction. Discovery
	// follows that publication boundary rather than inventing a new owner rule.
	expect(t, f.request("GET", "/api/v1/bikes/"+id, nil, nil), 200)
	restricted := f.request("GET", "/api/v1/bikes", nil, nil)
	expect(t, restricted, 200)
	if len(restricted.body["bikes"].([]any)) != 1 {
		t.Fatal("catalogue changed existing account/public-bike semantics")
	}
}

func TestPublicBikeCatalogueLiteralSearchAndInvalidInput(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011234804", "Search owner")
	wanted := catalogueBike(t, f, owner, map[string]any{"make": "Yamaha", "model": "MT-%_07", "year": 2020, "visibility": "public"})
	catalogueBike(t, f, owner, map[string]any{"make": "Yamaha", "model": "MT-07", "year": 2021, "visibility": "public"})
	for _, query := range []string{"  YAMAHA mt-%_  ", "%", "_"} {
		r := f.request("GET", "/api/v1/bikes?q="+url.QueryEscape(query), nil, nil)
		expect(t, r, 200)
		bikes := r.body["bikes"].([]any)
		if len(bikes) != 1 || bikes[0].(map[string]any)["id"] != wanted["id"] {
			t.Fatalf("search %q was not a literal make/model substring: %s", query, r.raw)
		}
	}
	for _, query := range []string{"not found", "' OR 1=1 --"} {
		r := f.request("GET", "/api/v1/bikes?q="+url.QueryEscape(query), nil, nil)
		expect(t, r, 200)
		if bikes, ok := r.body["bikes"].([]any); !ok || len(bikes) != 0 {
			t.Fatalf("empty query result is not an empty array: %s", r.raw)
		}
	}
	expect(t, f.request("GET", "/api/v1/bikes?q="+strings.Repeat("x", 201), nil, nil), 422)
	for _, cursor := range []string{"bad", strings.Repeat("a", 257), "AAAAAAAA-BBBB-4CCC-8DDD-EEEEEEEEEEEE", "urn:uuid:aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"} {
		expect(t, f.request("GET", "/api/v1/bikes?cursor="+url.QueryEscape(cursor), nil, nil), 422)
	}
}

func TestPublicBikeCatalogueFiltersBeforeKeysetPagination(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011234805", "Page owner")
	ctx := context.Background()
	// Nonmatching, private and hidden records sort before visible matches. If
	// filtering happens after LIMIT, this page will be empty or truncated.
	for i := range 35 {
		id := fmt.Sprintf("00000000-0000-4000-8000-%012x", i)
		visibility, model := "public", "Other model"
		if i%3 == 0 {
			visibility, model = "private", "Matching model"
		} else if i%3 == 1 {
			model = "Matching model"
		}
		if _, err := f.db.Exec(ctx, `INSERT INTO bikes(id,owner_id,make,model,model_id,year,visibility,version,created_at,updated_at) VALUES($1,$2,'Honda',$3,'',2020,$4,1,$5,$5)`, id, owner.id, model, visibility, f.now); err != nil {
			t.Fatal(err)
		}
		if i%3 == 1 {
			if _, err := f.db.Exec(ctx, `INSERT INTO moderation_hidden(target_type,target_id,hidden_at) VALUES('bike',$1,$2)`, id, f.now); err != nil {
				t.Fatal(err)
			}
		}
	}
	wanted := map[string]bool{}
	for i := range 23 {
		id := fmt.Sprintf("10000000-0000-4000-8000-%012x", i)
		if _, err := f.db.Exec(ctx, `INSERT INTO bikes(id,owner_id,make,model,model_id,year,visibility,version,created_at,updated_at) VALUES($1,$2,'Honda','Matching model','',2020,'public',1,$3,$3)`, id, owner.id, f.now); err != nil {
			t.Fatal(err)
		}
		wanted[id] = true
	}
	seen := map[string]bool{}
	cursor := ""
	for page := range 2 {
		r := f.request("GET", "/api/v1/bikes?q=matching&cursor="+url.QueryEscape(cursor), nil, nil)
		expect(t, r, 200)
		bikes := r.body["bikes"].([]any)
		count := 20
		if page == 1 {
			count = 3
		}
		if len(bikes) != count {
			t.Fatalf("catalogue filtered after page bound: %s", r.raw)
		}
		for _, value := range bikes {
			id := value.(map[string]any)["id"].(string)
			if !wanted[id] || seen[id] || id <= cursor {
				t.Fatalf("unexpected, repeated or unordered catalogue item: %s", id)
			}
			seen[id] = true
			cursor = id
		}
		next := r.body["next_cursor"].(string)
		if (page == 0 && next != cursor) || (page == 1 && next != "") {
			t.Fatalf("incorrect public continuation: %s", r.raw)
		}
	}
	if len(seen) != len(wanted) {
		t.Fatal("catalogue pagination lost a published bike")
	}
}
