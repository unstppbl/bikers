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
