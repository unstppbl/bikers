package app

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"testing"
)

func testBike(t *testing.T, f *fixture, owner *actor) map[string]any {
	t.Helper()
	r := f.request("POST", "/api/v1/me/bikes", map[string]any{"make": "Honda", "model": "CB400", "year": 2005}, owner)
	expect(t, r, 201)
	return r.body["bike"].(map[string]any)
}
func testClub(t *testing.T, f *fixture, owner *actor) map[string]any {
	t.Helper()
	r := f.request("POST", "/api/v1/clubs", map[string]any{"name": "City riders", "city": "Astana", "description": "Weekend community rides"}, owner)
	expect(t, r, 201)
	return r.body["club"].(map[string]any)
}
func clubDecision(f *fixture, id string, owner, target *actor, decision string) response {
	return f.request("POST", "/api/v1/clubs/"+id+"/members/"+target.id+"/decision", map[string]any{"decision": decision}, owner)
}

func TestGarageSharingNeverSharesMaintenanceAndOwnerMutationsStayScoped(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011234567", "Owner")
	other := f.login("+77021234567", "Other")
	bike := testBike(t, f, owner)
	id := bike["id"].(string)
	if bike["visibility"] != "private" || bike["version"] != float64(1) || bike["can_manage"] != true {
		t.Fatal("private creation defaults changed")
	}
	expect(t, f.request("GET", "/api/v1/bikes/"+id, nil, nil), 404)
	expect(t, f.request("GET", "/api/v1/bikes/"+id, nil, other), 404)
	privateHTML := f.request("GET", "/bikes/"+id, nil, owner)
	expect(t, privateHTML, 200)
	if strings.Contains(privateHTML.raw, "CB400") || strings.Contains(privateHTML.raw, owner.id) {
		t.Fatal("private bike HTML leaked metadata")
	}
	base := "/api/v1/me/bikes/" + id + "/maintenance"
	entry := f.request("POST", base, map[string]any{"service_date": "2026-11-02", "mileage_km": 12345, "work_performed": "PRIVATE MAINTENANCE NOTE"}, owner)
	expect(t, entry, 201)
	record := entry.body["entry"].(map[string]any)
	entryID := record["id"].(string)
	expect(t, f.request("GET", base, nil, nil), 401)
	expect(t, f.request("GET", base, nil, other), 404)
	expect(t, f.request("PATCH", "/api/v1/me/bikes/"+id, map[string]any{"version": 1, "visibility": "public"}, other), 404)
	shared := f.request("PATCH", "/api/v1/me/bikes/"+id, map[string]any{"version": 1, "visibility": "public"}, owner)
	expect(t, shared, 200)
	for _, viewer := range []*actor{nil, other} {
		r := f.request("GET", "/api/v1/bikes/"+id, nil, viewer)
		expect(t, r, 200)
		public := r.body["bike"].(map[string]any)
		if len(public) != 5 || public["owner_id"] != nil || strings.Contains(r.raw, "PRIVATE") || strings.Contains(r.raw, "mileage") || strings.Contains(r.raw, "version") {
			t.Fatalf("private bike fields leaked: %s", r.raw)
		}
	}
	publicHTML := f.request("GET", "/bikes/"+id, nil, nil)
	expect(t, publicHTML, 200)
	if !strings.Contains(publicHTML.raw, "Honda CB400") || strings.Contains(publicHTML.raw, "PRIVATE MAINTENANCE NOTE") || strings.Contains(publicHTML.raw, owner.id) {
		t.Fatal("public bike HTML is missing useful metadata or leaks private fields")
	}
	expect(t, f.request("GET", base, nil, other), 404)
	expect(t, f.request("DELETE", "/api/v1/me/bikes/"+id, map[string]any{"version": 1}, owner), 409)
	patched := f.request("PATCH", base+"/"+entryID, map[string]any{"version": 1, "work_performed": "Changed private work"}, owner)
	expect(t, patched, 200)
	expect(t, f.request("PATCH", base+"/"+entryID, map[string]any{"version": 1, "work_performed": "Stale work"}, owner), 409)
	second := testBike(t, f, owner)
	secondBase := "/api/v1/me/bikes/" + second["id"].(string) + "/maintenance/" + entryID
	expect(t, f.request("DELETE", secondBase, map[string]any{"version": 2}, owner), 404)
	expect(t, f.request("DELETE", base+"/"+entryID, map[string]any{"version": 2}, other), 404)
	for _, bad := range []map[string]any{
		{"service_date": "2026-02-30", "mileage_km": 1, "work_performed": "Work"},
		{"service_date": "2026-11-04", "mileage_km": 1, "work_performed": "Work"},
		{"service_date": "2026-11-02", "mileage_km": -1, "work_performed": "Work"},
	} {
		expect(t, f.request("POST", base, bad, owner), 422)
	}
	expect(t, f.request("GET", base+"?cursor=bad", nil, owner), 422)
	expect(t, f.request("DELETE", "/api/v1/me/bikes/"+id, map[string]any{"version": 2}, owner), 200)
	expect(t, f.request("GET", "/api/v1/bikes/"+id, nil, owner), 404)
	var count int
	if err := f.db.QueryRow(context.Background(), "SELECT count(*) FROM maintenance_entries WHERE bike_id=$1", id).Scan(&count); err != nil || count != 0 {
		t.Fatal("bike deletion left private entries")
	}
}

func TestGarageModelsAndCursorPagesDoNotLoseRecords(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011234567", "Owner")
	for _, query := range []string{"ЯМАХА МТ-07", "yamaha mt07"} {
		r := f.request("GET", "/api/v1/bike-models?query="+url.QueryEscape(query), nil, nil)
		expect(t, r, 200)
		found := false
		for _, item := range r.body["models"].([]any) {
			m := item.(map[string]any)
			if m["make"] == "Yamaha" && m["model"] == "MT-07" {
				found = true
			}
		}
		if !found {
			t.Fatal("basic alias lookup missed equivalent model spelling")
		}
	}
	ids := map[string]bool{}
	for range 51 {
		bike := testBike(t, f, owner)
		ids[bike["id"].(string)] = true
	}
	r := f.request("GET", "/api/v1/me/bikes", nil, owner)
	expect(t, r, 200)
	if len(r.body["bikes"].([]any)) != 50 || r.body["next_cursor"] == "" {
		t.Fatal("first bike page not bounded")
	}
	seen := map[string]bool{}
	for _, value := range r.body["bikes"].([]any) {
		seen[value.(map[string]any)["id"].(string)] = true
	}
	r = f.request("GET", "/api/v1/me/bikes?cursor="+url.QueryEscape(r.body["next_cursor"].(string)), nil, owner)
	expect(t, r, 200)
	if len(r.body["bikes"].([]any)) != 1 || r.body["next_cursor"] != "" {
		t.Fatal("last bike page not stable")
	}
	for _, value := range r.body["bikes"].([]any) {
		id := value.(map[string]any)["id"].(string)
		if seen[id] {
			t.Fatal("duplicate bike across cursor pages")
		}
		seen[id] = true
	}
	if len(seen) != len(ids) {
		t.Fatal("bike page lost a record")
	}
	var bikeID string
	for id := range ids {
		bikeID = id
		break
	}
	base := "/api/v1/me/bikes/" + bikeID + "/maintenance"
	for i := range 51 {
		expect(t, f.request("POST", base, map[string]any{"service_date": "2026-11-02", "mileage_km": i, "work_performed": fmt.Sprintf("Work %d", i)}, owner), 201)
	}
	r = f.request("GET", base, nil, owner)
	expect(t, r, 200)
	if len(r.body["entries"].([]any)) != 50 || r.body["next_cursor"] == "" {
		t.Fatal("maintenance cursor missing at date tie")
	}
	entryIDs := map[string]bool{}
	for _, v := range r.body["entries"].([]any) {
		entryIDs[v.(map[string]any)["id"].(string)] = true
	}
	r = f.request("GET", base+"?cursor="+url.QueryEscape(r.body["next_cursor"].(string)), nil, owner)
	expect(t, r, 200)
	if len(r.body["entries"].([]any)) != 1 || r.body["next_cursor"] != "" {
		t.Fatal("maintenance continuation incorrect")
	}
	if entryIDs[r.body["entries"].([]any)[0].(map[string]any)["id"].(string)] {
		t.Fatal("duplicate maintenance entry across pages")
	}
}

func TestClubApprovalTransferClosureAndPublicRideLinks(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011234567", "Owner")
	member := f.login("+77021234567", "Member")
	outsider := f.login("+77051234567", "Outsider")
	club := testClub(t, f, owner)
	id := club["id"].(string)
	base := "/api/v1/clubs/" + id
	if club["member_count"] != float64(1) || club["owner_id"] != owner.id {
		t.Fatal("club did not create an approved owner")
	}
	public := f.request("GET", base, nil, nil)
	expect(t, public, 200)
	if len(public.body["club"].(map[string]any)) != 5 {
		t.Fatal("public club preview leaked private metadata")
	}
	clubHTML := f.request("GET", "/clubs/"+id, nil, nil)
	expect(t, clubHTML, 200)
	if !strings.Contains(clubHTML.raw, "City riders") || strings.Contains(clubHTML.raw, owner.id) || strings.Contains(clubHTML.raw, "member_count") {
		t.Fatal("club HTML preview leaked private metadata")
	}
	expect(t, f.request("PUT", base+"/membership", nil, member), 200)
	expect(t, f.request("PUT", base+"/membership", nil, member), 200)
	pending := f.request("GET", base, nil, member)
	expect(t, pending, 200)
	if strings.Contains(pending.raw, "owner_id") || strings.Contains(pending.raw, "member_count") || strings.Contains(pending.raw, "version") {
		t.Fatal("pending member received private club fields")
	}
	expect(t, f.request("GET", base+"/members", nil, member), 403)
	expect(t, clubDecision(f, id, owner, member, "approve"), 200)
	expect(t, clubDecision(f, id, owner, member, "approve"), 200)
	expect(t, f.request("GET", base+"/members", nil, member), 200)
	expect(t, f.request("DELETE", base+"/membership", nil, owner), 409)
	expect(t, f.request("PATCH", base, map[string]any{"version": 1, "name": "Hijack"}, member), 403)
	expect(t, f.request("PATCH", base, map[string]any{"version": 0, "name": "Stale"}, owner), 409)
	ride := f.ride(owner, 5)
	expect(t, f.request("PUT", base+"/rides/"+ride, nil, owner), 200)
	expect(t, f.request("PUT", base+"/rides/"+ride, nil, owner), 200)
	linked := f.request("GET", base+"/rides", nil, outsider)
	expect(t, linked, 200)
	if len(linked.body["rides"].([]any)) != 1 || len(linked.body["rides"].([]any)[0].(map[string]any)) != 5 || strings.Contains(linked.raw, "PRIVATE") {
		t.Fatal("linked club ride leaked private plan")
	}
	view := f.request("GET", "/api/v1/rides/"+ride, nil, member)
	expect(t, view, 200)
	if strings.Contains(view.raw, "SECRET") {
		t.Fatal("club membership bypassed ride approval")
	}
	expect(t, f.request("GET", "/api/v1/rides/"+ride+"/messages", nil, member), 403)
	expect(t, f.request("PUT", base+"/ownership", map[string]any{"version": 1, "user_id": outsider.id}, owner), 409)
	transfer := f.request("PUT", base+"/ownership", map[string]any{"version": 1, "user_id": member.id}, owner)
	expect(t, transfer, 200)
	if transfer.body["club"].(map[string]any)["can_manage"] != false {
		t.Fatal("previous owner kept active administration")
	}
	expect(t, f.request("DELETE", base+"/rides/"+ride, nil, member), 200)
	expect(t, f.request("PUT", base+"/rides/"+ride, nil, member), 403)
	expect(t, f.request("DELETE", base+"/membership", nil, owner), 200)
	expect(t, f.request("GET", base+"/members", nil, owner), 403)
	closed := f.request("POST", base+"/close", map[string]any{"version": 2}, member)
	expect(t, closed, 200)
	expect(t, f.request("POST", base+"/close", map[string]any{"version": 2}, member), 200)
	expect(t, f.request("PUT", base+"/membership", nil, outsider), 409)
	expect(t, f.request("DELETE", base+"/membership", nil, member), 200)
	expect(t, f.request("GET", base+"/members", nil, member), 403)
	historical := f.request("GET", base, nil, member)
	expect(t, historical, 200)
	if strings.Contains(historical.raw, "owner_id") || strings.Contains(historical.raw, "member_count") {
		t.Fatal("historical owner inherited private access after leaving")
	}
}

func TestConcurrentClubTransferAndTargetLeaveCannotOrphanActiveClub(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011234567", "Owner")
	member := f.login("+77021234567", "Member")
	club := testClub(t, f, owner)
	id := club["id"].(string)
	base := "/api/v1/clubs/" + id
	expect(t, f.request("PUT", base+"/membership", nil, member), 200)
	expect(t, clubDecision(f, id, owner, member, "approve"), 200)
	results := make(chan response, 2)
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		<-start
		results <- f.request("PUT", base+"/ownership", map[string]any{"version": 1, "user_id": member.id}, owner)
	})
	wg.Go(func() { <-start; results <- f.request("DELETE", base+"/membership", nil, member) })
	close(start)
	wg.Wait()
	close(results)
	for r := range results {
		if r.status != 200 && r.status != 409 {
			t.Fatalf("unexpected concurrent transition: %d %s", r.status, r.raw)
		}
	}
	var approvedOwner bool
	if err := f.db.QueryRow(context.Background(), "SELECT EXISTS(SELECT 1 FROM clubs c JOIN club_memberships m ON m.club_id=c.id AND m.user_id=c.owner_id WHERE c.id=$1 AND c.status='active' AND m.state='approved')", id).Scan(&approvedOwner); err != nil || !approvedOwner {
		t.Fatal("concurrent transfer/leave orphaned an active club")
	}
}
