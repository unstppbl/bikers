package models

import (
	"encoding/json"
	"net/url"
	"path"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestBikeModelCopiesIsolateAliasesAndReferenceImages(t *testing.T) {
	original := BikeModel{ID: "fixture-model", Make: "Fixture", Model: "Model", Aliases: []string{"Фикстура"}, Images: []BikeModelImage{{URL: "/bike-models/fixture-model-2021.jpg", YearFrom: 2021, YearTo: 2021, Width: 800, Height: 600, SourceURL: "https://example.org/source", Author: "Photographer", License: "CC BY 4.0", LicenseURL: "https://creativecommons.org/licenses/by/4.0/"}}}
	copy := copyBikeModel(original)
	copy.Aliases[0] = "changed"
	copy.Images[0].URL = "changed"
	copy.Images[0].Author = "changed"
	copy.Images[0].YearTo = 2099
	if original.Aliases[0] != "Фикстура" || original.Images[0].URL != "/bike-models/fixture-model-2021.jpg" || original.Images[0].Author != "Photographer" || original.Images[0].YearTo != 2021 {
		t.Fatalf("returned metadata aliases catalogue storage: %+v", original)
	}
	empty := copyBikeModel(BikeModel{})
	data, err := json.Marshal(empty)
	if err != nil {
		t.Fatal(err)
	}
	if empty.Images == nil || !strings.Contains(string(data), `"images":[]`) {
		t.Fatalf("missing images must serialize as an empty array: %s", data)
	}
}

func TestCatalogueAliasesPreserveCanonicalModelAndImages(t *testing.T) {
	entries := MatchBikeModels("")
	if len(entries) == 0 {
		t.Fatal("curated catalogue is empty")
	}
	for _, entry := range entries {
		canonical, ok := FindBikeModel(entry.ID)
		if !ok || canonical.Make != entry.Make || canonical.Model != entry.Model || !reflect.DeepEqual(canonical.Images, entry.Images) {
			t.Fatalf("lookup changed canonical mapping for %s", entry.ID)
		}
		queries := append([]string{entry.Make + " " + entry.Model}, entry.Aliases...)
		for _, query := range queries {
			matches := MatchBikeModels("  " + strings.ToUpper(query) + " -- ")
			found := false
			for _, match := range matches {
				if match.ID == entry.ID {
					found = true
					if match.Make != canonical.Make || match.Model != canonical.Model || !reflect.DeepEqual(match.Images, canonical.Images) {
						t.Fatalf("alias %q changed canonical metadata", query)
					}
				}
			}
			if !found {
				t.Errorf("alias %q missed %s", query, entry.ID)
			}
		}
		// Mutating a public lookup result must not affect later requests.
		if len(canonical.Aliases) > 0 {
			canonical.Aliases[0] = "mutated-lookup"
		}
		if len(canonical.Images) > 0 {
			canonical.Images[0].URL = "mutated-lookup"
		}
		again, _ := FindBikeModel(entry.ID)
		if !reflect.DeepEqual(again, entry) {
			t.Fatalf("FindBikeModel leaked shared slices for %s", entry.ID)
		}
	}
	entries[0].Aliases = append(entries[0].Aliases, "mutated-search")
	if len(entries[0].Images) > 0 {
		entries[0].Images[0].URL = "mutated-search"
	}
	again, _ := FindBikeModel(entries[0].ID)
	if strings.Contains(strings.Join(again.Aliases, ","), "mutated-search") || (len(again.Images) > 0 && again.Images[0].URL == "mutated-search") {
		t.Fatal("MatchBikeModels leaked shared metadata")
	}
}

func TestCuratedReferenceMetadataHasValidUnambiguousBounds(t *testing.T) {
	location, err := time.LoadLocation("Asia/Almaty")
	if err != nil {
		t.Fatal(err)
	}
	maxYear := time.Now().In(location).Year() + 1
	pathPattern := regexp.MustCompile(`^/bike-models/[a-z0-9]+(?:-[a-z0-9]+)*-[0-9]{4}\.(?:jpg|png)$`)
	ids := map[string]bool{}
	for _, model := range MatchBikeModels("") {
		if model.ID == "" || ids[model.ID] {
			t.Fatalf("missing/duplicate catalogue ID %q", model.ID)
		}
		ids[model.ID] = true
		if model.Images == nil {
			t.Errorf("%s images is null", model.ID)
		}
		for i, image := range model.Images {
			if image.YearFrom < 1885 || image.YearTo < image.YearFrom || image.YearTo > maxYear {
				t.Errorf("%s has invalid reference year range %d..%d", model.ID, image.YearFrom, image.YearTo)
			}
			if !pathPattern.MatchString(image.URL) || !strings.HasPrefix(image.URL, "/bike-models/"+model.ID+"-") {
				t.Errorf("%s has non-local or mismatched model image path %q", model.ID, image.URL)
			}
			name := strings.TrimSuffix(image.URL, path.Ext(image.URL))
			photoYear, err := strconv.Atoi(name[strings.LastIndex(name, "-")+1:])
			if err != nil || photoYear < image.YearFrom || photoYear > image.YearTo {
				t.Errorf("%s reference filename year is outside its verified metadata range", model.ID)
			}
			if image.Width <= 0 || image.Height <= 0 {
				t.Errorf("%s reference dimensions must be positive", model.ID)
			}
			for _, raw := range []string{image.SourceURL, image.LicenseURL} {
				parsed, err := url.Parse(raw)
				if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
					t.Errorf("%s needs an HTTPS public source/license URL", model.ID)
				}
			}
			if strings.TrimSpace(image.Author) == "" || strings.TrimSpace(image.License) == "" {
				t.Errorf("%s reference lacks attribution/license", model.ID)
			}
			for j := 0; j < i; j++ {
				other := model.Images[j]
				if image.YearFrom <= other.YearTo && other.YearFrom <= image.YearTo {
					t.Errorf("%s has overlapping reference ranges", model.ID)
				}
			}
		}
	}
}

func TestReferenceImagesDoNotChangeBikeWireProjections(t *testing.T) {
	data, err := json.Marshal(PublicBike{})
	if err != nil {
		t.Fatal(err)
	}
	var public map[string]json.RawMessage
	if err = json.Unmarshal(data, &public); err != nil {
		t.Fatal(err)
	}
	if len(public) != 5 {
		t.Fatalf("public bike projection changed: %s", data)
	}
	for _, key := range []string{"id", "make", "model", "model_id", "year"} {
		if _, ok := public[key]; !ok {
			t.Fatalf("missing public bike field %s", key)
		}
	}
	data, err = json.Marshal(Bike{})
	if err != nil {
		t.Fatal(err)
	}
	var owner map[string]json.RawMessage
	if err = json.Unmarshal(data, &owner); err != nil {
		t.Fatal(err)
	}
	want := []string{"id", "owner_id", "make", "model", "model_id", "year", "visibility", "version", "created_at", "updated_at", "can_manage"}
	if len(owner) != len(want) {
		t.Fatalf("owner bike projection changed: %s", data)
	}
	for _, key := range want {
		if _, ok := owner[key]; !ok {
			t.Fatalf("missing owner bike field %s", key)
		}
	}
}
