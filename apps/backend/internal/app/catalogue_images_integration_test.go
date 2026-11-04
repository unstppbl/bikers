package app

import (
	"encoding/json"
	"net/url"
	"reflect"
	"testing"

	"bikers.kz/backend/internal/models"
)

func TestPublicCatalogueKeepsEveryCanonicalModelDiscoverableBeyondSuggestionLimit(t *testing.T) {
	f := newFixture(t)
	canonical := models.MatchBikeModels("")
	const suggestionLimit = 20
	if len(canonical) <= suggestionLimit {
		t.Fatalf("regression requires a catalogue larger than %d suggestions; got %d", suggestionLimit, len(canonical))
	}
	readPublicModels := func(query string) []models.BikeModel {
		t.Helper()
		r := f.request("GET", "/api/v1/bike-models?query="+url.QueryEscape(query), nil, nil)
		expect(t, r, 200)
		var result struct {
			Models []models.BikeModel `json:"models"`
		}
		if err := json.Unmarshal([]byte(r.raw), &result); err != nil {
			t.Fatal(err)
		}
		if result.Models == nil || len(result.Models) > suggestionLimit {
			t.Fatalf("public query %q returned null or unbounded suggestions: %s", query, r.raw)
		}
		return result.Models
	}

	suggestions := readPublicModels("")
	if len(suggestions) != suggestionLimit {
		t.Fatalf("empty public query returned %d suggestions, want %d", len(suggestions), suggestionLimit)
	}
	firstPage := make(map[string]bool, len(suggestions))
	for _, suggestion := range suggestions {
		known, ok := models.FindBikeModel(suggestion.ID)
		if !ok || firstPage[suggestion.ID] || !reflect.DeepEqual(suggestion, known) {
			t.Fatalf("empty public query changed or duplicated canonical metadata for %q", suggestion.ID)
		}
		firstPage[suggestion.ID] = true
	}

	discoveredOutsideSuggestions := 0
	for _, wanted := range canonical {
		found := false
		for _, match := range readPublicModels(wanted.Make + " " + wanted.Model) {
			if match.ID != wanted.ID {
				continue
			}
			if !reflect.DeepEqual(match, wanted) {
				t.Fatalf("narrow public query changed canonical identity, aliases, or reference metadata for %q", wanted.ID)
			}
			if !firstPage[wanted.ID] {
				discoveredOutsideSuggestions++
			}
			found = true
			break
		}
		if !found {
			t.Errorf("canonical model %q became unreachable behind the suggestion limit", wanted.ID)
		}
	}
	if discoveredOutsideSuggestions != len(canonical)-suggestionLimit {
		t.Fatalf("discovered %d models outside the empty-query suggestions, want %d", discoveredOutsideSuggestions, len(canonical)-suggestionLimit)
	}
}
