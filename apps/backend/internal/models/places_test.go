package models

import (
	"math"
	"strings"
	"testing"
)

func TestPlaceValidationBoundsCategoriesAndUnsafeLinks(t *testing.T) {
	base := PlaceInput{Name: "  Service point  ", Category: "workshop", Lat: 43.24, Lon: 76.89, City: "Алматы", Website: "https://example.com/service"}
	out, err := ValidatePlace(base)
	if err != nil || out.Name != "Service point" || out.City != "Almaty" {
		t.Fatal(out, err)
	}
	for _, change := range []func(*PlaceInput){func(p *PlaceInput) { p.Name = "" }, func(p *PlaceInput) { p.Name = strings.Repeat("x", 201) }, func(p *PlaceInput) { p.Category = "all" }, func(p *PlaceInput) { p.Lat = math.NaN() }, func(p *PlaceInput) { p.Lon = math.Inf(1) }, func(p *PlaceInput) { p.Lat = 0 }, func(p *PlaceInput) { p.Website = "javascript:alert(1)" }, func(p *PlaceInput) { p.Website = "https://user:pass@example.com" }, func(p *PlaceInput) { p.Website = "//example.com" }, func(p *PlaceInput) { p.OpeningHours = strings.Repeat("x", 301) }} {
		p := base
		change(&p)
		if _, err := ValidatePlace(p); err == nil {
			t.Fatal("invalid place accepted", p)
		}
	}
}

func TestPlaceCategoriesIncludeSpeedCamerasAndRemainClosed(t *testing.T) {
	for _, category := range []string{"fuel", "workshop", "tires", "motorcycle_shop", "parking", "meeting_spot", "speed_camera"} {
		t.Run(category, func(t *testing.T) {
			place, err := ValidatePlace(PlaceInput{Name: "  Observed place  ", Category: " " + category + " ", Lat: 51.1694, Lon: 71.4491, City: "Астана"})
			if err != nil || place.Category != category || place.Name != "Observed place" || place.City != "Astana" {
				t.Fatal("valid place category rejected or normalization changed", place, err)
			}
		})
	}
	for _, category := range []string{"", "speed-camera", "speed_cameras", "traffic_camera", "Speed_camera"} {
		if PlaceCategory(category) {
			t.Fatal("unknown category accepted", category)
		}
	}
}
