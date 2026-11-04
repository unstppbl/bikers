package app

import (
	"net/url"
	"reflect"
	"testing"
)

func TestModelLookupPublishesOnlyCuratedReferenceMetadata(t *testing.T) {
	f := newFixture(t)
	all := f.request("GET", "/api/v1/bike-models", nil, nil)
	expect(t, all, 200)
	items, ok := all.body["models"].([]any)
	if !ok || len(items) == 0 || len(items) > 20 {
		t.Fatal("model lookup did not return a bounded array")
	}
	byID := map[string]map[string]any{}
	for _, item := range items {
		model := item.(map[string]any)
		if len(model) != 5 {
			t.Fatalf("unexpected model lookup fields: %v", model)
		}
		for _, field := range []string{"id", "make", "model"} {
			if value, ok := model[field].(string); !ok || value == "" {
				t.Fatalf("missing model field %s", field)
			}
		}
		if _, ok := model["aliases"].([]any); !ok {
			t.Fatal("aliases must be an array")
		}
		images, ok := model["images"].([]any)
		if !ok {
			t.Fatal("unavailable reference images must be [] rather than null or omitted")
		}
		for _, raw := range images {
			image, ok := raw.(map[string]any)
			if !ok || len(image) != 9 {
				t.Fatal("reference image schema changed")
			}
			for _, field := range []string{"url", "source_url", "author", "license", "license_url"} {
				if value, ok := image[field].(string); !ok || value == "" {
					t.Fatalf("missing reference field %s", field)
				}
			}
			for _, field := range []string{"year_from", "year_to", "width", "height"} {
				if value, ok := image[field].(float64); !ok || value <= 0 {
					t.Fatalf("invalid numeric reference field %s", field)
				}
			}
		}
		byID[model["id"].(string)] = model
	}
	for _, query := range []string{"ЯМАХА МТ-07", "yamaha mt07"} {
		result := f.request("GET", "/api/v1/bike-models?query="+url.QueryEscape(query), nil, nil)
		expect(t, result, 200)
		found := false
		for _, item := range result.body["models"].([]any) {
			model := item.(map[string]any)
			if model["id"] == "yamaha-mt-07" {
				found = true
				if !reflect.DeepEqual(model, byID["yamaha-mt-07"]) {
					t.Fatal("alias lookup changed canonical reference metadata")
				}
			}
		}
		if !found {
			t.Fatal("alias lookup lost canonical model")
		}
	}
}
