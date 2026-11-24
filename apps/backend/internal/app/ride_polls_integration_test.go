package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
)

const pollAPI = "/api/v1/ride-polls"

func pollResponse(t *testing.T, response response, status int) models.RidePoll {
	t.Helper()
	expect(t, response, status)
	var result struct{ Poll models.RidePoll }
	if err := json.Unmarshal([]byte(response.raw), &result); err != nil || result.Poll.ID == "" || result.Poll.Timezone != "Asia/Almaty" || result.Poll.MyOptionIDs == nil {
		t.Fatal("invalid poll response", response.raw, err)
	}
	return result.Poll
}

func openPoll(t *testing.T, f *fixture, host *actor) models.RidePoll {
	t.Helper()
	return pollResponse(t, f.request("POST", pollAPI, models.RidePollInput{
		City:        " Астана ",
		Description: " A paved loop for the group ",
		Options:     []time.Time{f.now.Add(48 * time.Hour), f.now.Add(24 * time.Hour)},
	}, host), 201)
}

func guestPollCookie(t *testing.T, f *fixture, path string) *actor {
	t.Helper()
	read := f.request("GET", path, nil, nil)
	pollResponse(t, read, 200)
	if len(read.cookies) != 1 {
		t.Fatal("expected one poll cookie", read.cookies)
	}
	cookie := read.cookies[0]
	if cookie.Path != path || cookie.Domain != "" || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.MaxAge <= 0 {
		t.Fatal("guest cookie is not durable and scoped to the poll", cookie)
	}
	if strings.Contains(read.raw, cookie.Value) {
		t.Fatal("raw voter token included in the response JSON")
	}
	return &actor{cookie: cookie}
}

func checkPollVotes(t *testing.T, p models.RidePoll, want []int64, mine []string) {
	t.Helper()
	votes := make([]int64, len(p.Options))
	for i, option := range p.Options {
		votes[i] = option.Votes
	}
	if !reflect.DeepEqual(votes, want) || !reflect.DeepEqual(p.MyOptionIDs, mine) {
		t.Fatalf("votes = %v and mine = %v, want %v and %v", votes, p.MyOptionIDs, want, mine)
	}
}

func TestRidePollAnonymousVoteReplacementAndHostSelection(t *testing.T) {
	f := newFixture(t)
	host := f.login("+77011235461", "Private Host Name")
	other := f.login("+77011235462", "Private Guest Name")
	created := openPoll(t, f, host)
	if parsed, err := uuid.Parse(created.ID); err != nil || parsed.Version() != 4 || !created.CanManage || created.City != "Astana" || created.Description != "A paved loop for the group" || len(created.Options) != 2 || !created.Options[0].StartsAt.Before(created.Options[1].StartsAt) {
		t.Fatal("invalid normalized unlisted poll", created, err)
	}
	path := pollAPI + "/" + created.ID
	first, second := created.Options[0].ID, created.Options[1].ID
	expect(t, f.request("GET", pollAPI, nil, nil), 404)
	expect(t, f.request("GET", pollAPI+"/unparseable", nil, nil), 404)
	expect(t, f.request("GET", pollAPI+"/"+uuid.NewString(), nil, nil), 404)
	guest := guestPollCookie(t, f, path)
	guest2 := guestPollCookie(t, f, path)
	if guest.cookie.Value == guest2.cookie.Value {
		t.Fatal("the two guests received the same voter token")
	}
	vote := func(voter *actor, choices ...string) models.RidePoll {
		t.Helper()
		ids := append([]string{}, choices...)
		return pollResponse(t, f.request("PUT", path+"/vote", map[string][]string{"option_ids": ids}, voter), 200)
	}
	checkPollVotes(t, vote(guest, second, first), []int64{1, 1}, []string{first, second})
	checkPollVotes(t, vote(guest, second), []int64{0, 1}, []string{second})
	checkPollVotes(t, vote(guest, second), []int64{0, 1}, []string{second})
	checkPollVotes(t, vote(guest2, second), []int64{0, 2}, []string{second})
	checkPollVotes(t, vote(guest), []int64{0, 1}, []string{})
	checkPollVotes(t, pollResponse(t, f.request("GET", path, nil, guest2), 200), []int64{0, 1}, []string{second})

	for _, wrongOrigin := range []string{"", "https://other.example"} {
		req, err := http.NewRequest("PUT", f.ts.URL+path+"/vote", strings.NewReader(`{"option_ids":[]}`))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if wrongOrigin != "" {
			req.Header.Set("Origin", wrongOrigin)
		}
		req.AddCookie(guest2.cookie)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusForbidden {
			t.Fatal("vote without the exact origin succeeded", res.StatusCode)
		}
	}
	for _, payload := range []any{map[string]any{}, map[string][]string{"option_ids": {second, second}}, map[string][]string{"option_ids": {uuid.NewString()}}} {
		expect(t, f.request("PUT", path+"/vote", payload, guest2), 422)
	}
	selection := func(by *actor, id string) response {
		return f.request("PUT", path+"/selection", map[string]string{"option_id": id}, by)
	}
	expect(t, selection(nil, first), 401)
	expect(t, selection(other, first), 403)
	expect(t, selection(host, uuid.NewString()), 422)
	selected := pollResponse(t, selection(host, first), 200)
	if !selected.CanManage || selected.SelectedOptionID != first {
		t.Fatal("host selection was not retained", selected)
	}
	checkPollVotes(t, selected, []int64{0, 1}, []string{})
	expect(t, selection(host, first), 200)
	expect(t, selection(host, second), 409)
	expect(t, f.request("PUT", path+"/vote", map[string][]string{"option_ids": {first}}, guest2), 409)
	public := f.request("GET", path, nil, nil)
	read := pollResponse(t, public, 200)
	checkPollVotes(t, read, []int64{0, 1}, []string{})
	if read.CanManage || read.SelectedOptionID != first || len(public.cookies) != 0 {
		t.Fatal("chosen poll failed guest-read constraints", public.raw)
	}
	for _, sensitive := range []string{host.id, other.id, "Private Host Name", "Private Guest Name", "organizer_id", "voter_hash", guest.cookie.Value, guest2.cookie.Value} {
		if strings.Contains(public.raw, sensitive) {
			t.Fatal("poll exposed organizer or voter identity", public.raw)
		}
	}
	for _, publicPath := range []string{path, "/ride-polls/" + created.ID} {
		res, err := http.Get(f.ts.URL + publicPath)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.Header.Get("X-Robots-Tag") != "noindex, nofollow" {
			t.Fatal("poll can be indexed", publicPath, res.Header)
		}
	}
	var rideCount int
	if err := f.db.QueryRow(context.Background(), `SELECT count(*) FROM rides`).Scan(&rideCount); err != nil || rideCount != 0 {
		t.Fatal("poll selection automatically published a ride", rideCount, err)
	}
}

func TestRidePollRejectsInvalidCreationAndPastChoices(t *testing.T) {
	f := newFixture(t)
	host := f.login("+77011235463", "Poll Host")
	unfinished := f.login("+77011235464", "New Account")
	if _, err := f.db.Exec(context.Background(), `UPDATE users SET onboarding_complete=false WHERE id=$1`, unfinished.id); err != nil {
		t.Fatal(err)
	}
	future := f.now.Add(2 * time.Hour)
	valid := models.RidePollInput{City: "Astana", Description: "Local loop", Options: []time.Time{future, future.Add(time.Hour)}}
	expect(t, f.request("POST", pollAPI, valid, nil), 401)
	expect(t, f.request("POST", pollAPI, valid, unfinished), 409)
	for _, input := range []models.RidePollInput{
		{City: valid.City, Description: valid.Description, Options: []time.Time{future}},
		{City: valid.City, Description: valid.Description, Options: []time.Time{future, future.In(time.FixedZone("KZ", 5*60*60))}},
		{City: valid.City, Description: valid.Description, Options: []time.Time{future, f.now}},
		{City: valid.City, Description: strings.Repeat("a", 501), Options: valid.Options},
		{City: valid.City, Description: strings.Repeat("🏍", 251), Options: valid.Options},
		{City: valid.City, Description: "contains\x00null", Options: valid.Options},
		{City: " ", Description: valid.Description, Options: valid.Options},
	} {
		expect(t, f.request("POST", pollAPI, input, host), 422)
	}
	created := openPoll(t, f, host)
	path := pollAPI + "/" + created.ID
	guest := guestPollCookie(t, f, path)
	f.now = created.Options[0].StartsAt
	option := created.Options[0].ID
	selectPast := f.request("PUT", path+"/selection", map[string]string{"option_id": option}, host)
	votePast := f.request("PUT", path+"/vote", map[string][]string{"option_ids": {option}}, guest)
	for _, result := range []response{selectPast, votePast} {
		expect(t, result, 422)
		if !strings.Contains(result.raw, `"poll_option_past"`) {
			t.Fatal("past time failed with incorrect code", result.raw)
		}
	}
	f.now = created.Options[1].StartsAt
	expect(t, f.request("PUT", path+"/vote", map[string][]string{"option_ids": {}}, guest), 409)
	expect(t, f.request("GET", path, nil, nil), 200)
}

func TestRidePollConcurrentVoteAndSelectionCommitConsistentCounts(t *testing.T) {
	f := newFixture(t)
	host := f.login("+77011235465", "Poll Host")
	created := openPoll(t, f, host)
	path := pollAPI + "/" + created.ID
	guest := guestPollCookie(t, f, path)
	choice := created.Options[0].ID
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	makeRequest := func(suffix, payload string, actor *actor) *http.Request {
		t.Helper()
		req, err := http.NewRequestWithContext(ctx, "PUT", f.ts.URL+path+suffix, strings.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", f.ts.URL)
		req.Header.Set("X-CSRF-Token", actor.csrf)
		req.AddCookie(actor.cookie)
		return req
	}
	voteRequest := makeRequest("/vote", `{"option_ids":["`+choice+`"]}`, guest)
	selectionRequest := makeRequest("/selection", `{"option_id":"`+choice+`"}`, host)
	type result struct {
		response response
		err      error
	}
	start := make(chan struct{})
	invoke := func(req *http.Request) <-chan result {
		finished := make(chan result, 1)
		go func() {
			<-start
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				finished <- result{err: err}
				return
			}
			defer res.Body.Close()
			body, err := io.ReadAll(res.Body)
			finished <- result{response: response{status: res.StatusCode, raw: string(body)}, err: err}
		}()
		return finished
	}
	voteDone, selectionDone := invoke(voteRequest), invoke(selectionRequest)
	close(start)
	vote, selection := <-voteDone, <-selectionDone
	if vote.err != nil || selection.err != nil {
		t.Fatal("concurrent requests failed", vote.err, selection.err)
	}
	selected := pollResponse(t, selection.response, 200)
	if selected.SelectedOptionID != choice {
		t.Fatal("host selection was lost", selection.response.raw)
	}
	var committedVotes int64
	switch vote.response.status {
	case 200:
		committedVotes = 1
		voted := pollResponse(t, vote.response, 200)
		if voted.SelectedOptionID != "" {
			t.Fatal("vote was accepted after the poll closed", vote.response.raw)
		}
	case 409:
	default:
		t.Fatal("unexpected vote status", vote.response.status, vote.response.raw)
	}
	checkPollVotes(t, selected, []int64{committedVotes, 0}, []string{})
	checkPollVotes(t, pollResponse(t, f.request("GET", path, nil, nil), 200), []int64{committedVotes, 0}, []string{})
}
