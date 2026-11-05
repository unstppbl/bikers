package photon

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"bikers.kz/backend/internal/models"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSearchNormalizesPhotonAddressesAndFiltersCountry(t *testing.T) {
	c := New("http://photon:2322", Options{HTTPClient: &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		q := r.URL.Query()
		if r.Method != "GET" || r.URL.Host != "photon:2322" || r.URL.Path != "/api/" || q.Get("q") != "Абай 150" || q.Get("countrycode") != "KZ" || q.Get("lang") != "ru" || q.Get("bbox") != "46,40.5,87.5,55.5" || q.Get("lat") != "43.24" {
			t.Fatalf("unexpected request %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"type":"FeatureCollection","features":[{"type":"Feature","geometry":{"type":"Point","coordinates":[76.89,43.24]},"properties":{"osm_type":"W","osm_id":12,"street":"проспект Абая","housenumber":"150","city":"Алматы","state":"Алматы","countrycode":"KZ","type":"house"}},{"type":"Feature","geometry":{"type":"Point","coordinates":[37,55]},"properties":{"name":"Outside","countrycode":"RU","osm_type":"N","osm_id":13}}]}`))}, nil
	})}})
	places, err := c.Search(context.Background(), models.MapSearch{Query: "Абай 150", Lang: "ru", Near: &models.MapPoint{Lat: 43.24, Lon: 76.89}})
	if err != nil || len(places) != 1 || places[0].Label != "проспект Абая 150, Алматы" || places[0].ID != "w:12" || places[0].Lat != 43.24 || places[0].Lon != 76.89 || places[0].Kind != "house" {
		t.Fatalf("unexpected places %+v: %v", places, err)
	}
}

func TestPhotonRejectsOversizeMalformedAndFailureResponses(t *testing.T) {
	for _, test := range []struct {
		status int
		body   string
	}{{503, `{}`}, {200, `{}`}, {200, `{"features":[]} trailing`}, {200, strings.Repeat(" ", 256<<10+1)}} {
		c := New("http://photon:2322", Options{HTTPClient: &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/reverse" || r.URL.Query().Get("radius") != "2" {
				t.Fatal("reverse request was not bounded")
			}
			return &http.Response{StatusCode: test.status, Body: io.NopCloser(strings.NewReader(test.body))}, nil
		})}})
		if _, err := c.Reverse(context.Background(), models.MapPoint{Lat: 43, Lon: 76}, "kk"); err == nil {
			t.Fatal("invalid photon response accepted")
		}
	}
}
