package models

import (
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
	unchanged := Cities()[0]
	if first.Value != unchanged.Value || first.Names != unchanged.Names || !slices.Equal(first.Aliases, unchanged.Aliases) {
		t.Fatal("Cities returned mutable vocabulary")
	}
}
