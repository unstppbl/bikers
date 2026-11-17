package models

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestCityNormalization(t *testing.T) {
	for input, want := range map[string]string{
		"  Алматы ": "Almaty", "almaty": "Almaty", "ALMA ATA": "Almaty",
		"Нұр-Сұлтан": "Astana", "nur sultan": "Astana", "АСТАНА": "Astana",
		"Қарағанды": "Karaganda", "karagandy": "Karaganda", "Усть-Каменогорск": "Oskemen",
		"Өскемен": "Oskemen", "Konaev": "Konayev", "Қонаев": "Konayev",
		"  Bishkek  ": "Bishkek", "Almaty district": "Almaty district", "": "", "  ": "",
	} {
		if got := NormalizeCity(input); got != want {
			t.Errorf("NormalizeCity(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestCityAliasesMatchStoredNamesWithoutChangingUnknownCities(t *testing.T) {
	aliases := CityAliases("Усть-Каменогорск")
	for _, name := range []string{"Oskemen", "Усть-Каменогорск", "Өскемен", "Ust-Kamenogorsk"} {
		if !slices.Contains(aliases, name) {
			t.Errorf("Missing stored-name alias %q in %v", name, aliases)
		}
	}
	if !slices.Equal(CityAliases("  Bishkek "), []string{"Bishkek"}) || len(CityAliases("  ")) != 0 {
		t.Fatal("Unknown cities must remain usable and blank input must not match a city")
	}
	seen := map[string]bool{}
	for _, alias := range aliases {
		key := strings.ToLower(alias)
		if seen[key] {
			t.Errorf("Duplicate case-insensitive alias %q", alias)
		}
		seen[key] = true
	}
}

func TestCityVocabularyIsCompletePerLanguageAndCannotBeMutated(t *testing.T) {
	cities := Cities()
	if len(cities) == 0 {
		t.Fatal("City suggestions must be available")
	}
	seen := map[string]string{}
	for _, city := range cities {
		if city.Value == "" || city.Names.EN == "" || city.Names.RU == "" || city.Names.KK == "" {
			t.Errorf("City names missing: %+v", city)
		}
		for _, alias := range append([]string{city.Value, city.Names.EN, city.Names.RU, city.Names.KK}, city.Aliases...) {
			key := cityKey(alias)
			if previous := seen[key]; previous != "" && previous != city.Value {
				t.Errorf("Alias %q is ambiguous between %s and %s", alias, previous, city.Value)
			}
			seen[key] = city.Value
		}
	}
	first := Cities()[0]
	cities[0].Value = "Changed"
	cities[0].Names.EN = "Changed"
	cities[0].Aliases[0] = "Changed"
	cities[0].Center.Lat = 0
	unchanged := Cities()[0]
	if first.Value != unchanged.Value || first.Names != unchanged.Names || first.Center != unchanged.Center || !slices.Equal(first.Aliases, unchanged.Aliases) {
		t.Fatal("Cities returned mutable vocabulary")
	}
}

func TestCityCataloguePublishesValidMapCenters(t *testing.T) {
	encoded, err := json.Marshal(Cities())
	if err != nil {
		t.Fatal(err)
	}
	var published []struct {
		Value  string `json:"value"`
		Center *struct {
			Lat float64 `json:"lat"`
			Lon float64 `json:"lon"`
		} `json:"center"`
	}
	if err := json.Unmarshal(encoded, &published); err != nil {
		t.Fatal(err)
	}
	for _, city := range published {
		if city.Center == nil {
			t.Errorf("%s has no public map center", city.Value)
			continue
		}
		if !ValidCoordinate(city.Center.Lat, city.Center.Lon) || city.Center.Lat < 40.5 || city.Center.Lat > 55.5 || city.Center.Lon < 46 || city.Center.Lon > 87.5 {
			t.Errorf("%s map center is outside Kazakhstan's supported map envelope: %+v", city.Value, city.Center)
		}
	}
}
