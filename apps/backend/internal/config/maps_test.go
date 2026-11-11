package config

import "testing"

func TestMapConfigurationUsesFixedOriginsAndVersionedSameOriginArchive(t *testing.T) {
	for _, values := range [][3]string{{"", "", ""}, {"http://bikers-photon:2322", "http://bikers-valhalla:8002", "/map-data/2583113c45ca/kazakhstan.pmtiles"}} {
		if err := ValidateMaps(values[0], values[1], values[2]); err != nil {
			t.Fatal(err)
		}
	}
	for _, values := range [][3]string{
		{"http://photon:2322", "", ""},
		{"file:///tmp/photon", "http://valhalla:8002", ""},
		{"http://user:pass@photon:2322", "http://valhalla:8002", ""},
		{"http://photon:2322/api", "http://valhalla:8002", ""},
		{"http://photon:2322?url=https://example.com", "http://valhalla:8002", ""},
		{"", "", "https://example.com/map.pmtiles"},
		{"", "", "//example.com/map.pmtiles"},
		{"", "", "/map-data/../secret.pmtiles"},
		{"", "", "/map-data/v1/map.pmtiles?url=https://example.com"},
		{"", "", "/map-data/v1/%2e%2e.pmtiles"},
		{"", "", "/map-data/map.pmtiles"},
	} {
		if err := ValidateMaps(values[0], values[1], values[2]); err == nil {
			t.Errorf("unsafe map configuration accepted: %#v", values)
		}
	}
}
