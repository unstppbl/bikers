package valhalla

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"bikers.kz/backend/internal/models"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestMotorcycleRequestUsesOSRMGeoJSONAndReturnsMeters(t *testing.T) {
	c := New("http://valhalla:8002", Options{HTTPClient: &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if r.Method != "POST" || r.URL.String() != "http://valhalla:8002/route" || payload["costing"] != "motorcycle" || payload["format"] != "osrm" || payload["shape_format"] != "geojson" || payload["directions_type"] != "none" || payload["alternates"] != float64(0) || len(payload["locations"].([]any)) != 2 {
			t.Fatalf("wrong routing payload %+v", payload)
		}
		if _, exists := payload["language"]; exists {
			t.Fatal("unnarrated route passed an unsupported narration locale")
		}
		costing := payload["costing_options"].(map[string]any)["motorcycle"].(map[string]any)
		if costing["use_trails"] != float64(0) || costing["use_tracks"] != float64(0) {
			t.Fatal("road motorcycle profile changed")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"code":"Ok","routes":[{"distance":1234.5,"duration":186.2,"geometry":{"type":"LineString","coordinates":[[76.89,43.24],[76.91,43.25]]}}]}`))}, nil
	})}})
	for _, lang := range []string{"en", "ru", "kk"} {
		plan, err := c.Route(context.Background(), []models.MapPoint{{Label: "A", Lat: 43.24, Lon: 76.89}, {Label: "B", Lat: 43.25, Lon: 76.91}}, lang)
		if err != nil || plan.DistanceMeters != 1234.5 || plan.DurationSeconds != 186.2 || plan.Geometry.Coordinates[0] != [2]float64{76.89, 43.24} {
			t.Fatalf("invalid normalized %s plan %+v %v", lang, plan, err)
		}
	}
}

func TestValhallaRejectsInvalidGeometryAndDistinguishesNoRoute(t *testing.T) {
	for _, test := range []struct {
		status     int
		body, code string
	}{
		{400, `{"error_code":442}`, "route_not_found"},
		{200, `{"code":"NoRoute"}`, "route_not_found"},
		{503, `{}`, ""},
		{200, `{"code":"Ok","routes":[{"geometry":{"type":"LineString","coordinates":[[76,43,5],[76,43]]}}]}`, ""},
		{200, `{"code":"Ok","routes":[]}`, ""},
		{200, strings.Repeat(" ", 8<<20+1), ""},
	} {
		c := New("http://valhalla:8002", Options{HTTPClient: &http.Client{Transport: transport(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: test.status, Body: io.NopCloser(strings.NewReader(test.body))}, nil
		})}})
		_, err := c.Route(context.Background(), nil, "en")
		if err == nil {
			t.Fatal("invalid route response accepted")
		}
		if test.code != "" {
			var f *models.Fault
			if !errors.As(err, &f) || f.Code != test.code {
				t.Fatalf("got %v, want %s", err, test.code)
			}
		}
	}
}
