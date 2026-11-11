package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestPrivateRideMapsAndAuthenticatedGeoAPIs(t *testing.T) {
	f := newFixture(t)
	var routeCalls atomic.Int32
	var unavailable atomic.Bool
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if unavailable.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"PRIVATE PROVIDER FAILURE 43.241234 76.891234"}`))
			return
		}
		switch r.URL.Path {
		case "/api/", "/reverse":
			_, _ = w.Write([]byte(`{"type":"FeatureCollection","features":[{"type":"Feature","geometry":{"type":"Point","coordinates":[76.89,43.24]},"properties":{"osm_type":"N","osm_id":123,"name":"Алматы","countrycode":"KZ","type":"city"}}]}`))
		case "/route":
			routeCalls.Add(1)
			var request map[string]any
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request["costing"] != "motorcycle" || request["shape_format"] != "geojson" || request["format"] != "osrm" {
				t.Error("incorrect fixed routing request", err)
				w.WriteHeader(500)
				return
			}
			_, _ = w.Write([]byte(`{"code":"Ok","routes":[{"distance":1500,"duration":180,"geometry":{"type":"LineString","coordinates":[[76.89,43.24],[76.91,43.25]]}}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(provider.Close)
	f.cfg.PhotonBaseURL, f.cfg.ValhallaBaseURL = provider.URL, provider.URL
	f.cfg.MapsArchiveURL = "/map-data/2583113c45ca/kazakhstan.pmtiles"
	f.startServer(f.gateway)
	expect(t, f.request("GET", "/api/v1/geo/config", nil, nil), 401)
	for _, path := range []string{"/api/v1/geo/search", "/api/v1/geo/reverse"} {
		expect(t, f.request("POST", path, map[string]any{}, nil), 401)
	}
	owner := f.login("+77011234201", "Map owner")
	guest := f.login("+77011234202", "Map member")
	cfg := f.request("GET", "/api/v1/geo/config", nil, owner)
	expect(t, cfg, 200)
	if cfg.body["enabled"] != true || cfg.body["archive_url"] != f.cfg.MapsArchiveURL {
		t.Fatal("map configuration unavailable", cfg.raw)
	}
	for _, test := range []struct {
		path string
		body map[string]any
	}{
		{"/api/v1/geo/search", map[string]any{"query": "Almaty", "lang": "kk", "near": map[string]any{"lat": 43.24, "lon": 76.89}}},
		{"/api/v1/geo/reverse", map[string]any{"point": map[string]any{"lat": 43.24, "lon": 76.89}, "lang": "ru"}},
	} {
		result := f.request("POST", test.path, test.body, owner)
		expect(t, result, 200)
		if len(result.body["places"].([]any)) != 1 {
			t.Fatal("place normalization failed", result.raw)
		}
	}
	for _, test := range []struct {
		path string
		body map[string]any
	}{
		{"/api/v1/geo/search", map[string]any{"query": "Almaty", "near": map[string]any{"lat": 43.24}}},
		{"/api/v1/geo/search", map[string]any{"query": "Almaty", "lang": "de"}},
		{"/api/v1/geo/reverse", map[string]any{"point": map[string]any{"lat": 1, "lon": 1}}},
		{"/api/v1/geo/reverse", map[string]any{"point": map[string]any{"lat": 43.24}}},
	} {
		expect(t, f.request("POST", test.path, test.body, owner), 422)
	}
	badGeoCSRF := *owner
	badGeoCSRF.csrf = "incorrect"
	for _, test := range []struct {
		path string
		body map[string]any
	}{
		{"/api/v1/geo/search", map[string]any{"query": "Almaty"}},
		{"/api/v1/geo/reverse", map[string]any{"point": map[string]any{"lat": 43.24, "lon": 76.89}}},
	} {
		expect(t, f.request("POST", test.path, test.body, &badGeoCSRF), 403)
		expect(t, f.request("GET", test.path, nil, owner), 404)
		unknown := map[string]any{}
		for key, value := range test.body {
			unknown[key] = value
		}
		unknown["unexpected_private_value"] = "PRIVATE QUERY 76.891234"
		result := f.request("POST", test.path, unknown, owner)
		expect(t, result, 400)
		if strings.Contains(result.raw, "PRIVATE QUERY") || strings.Contains(result.raw, "76.891234") {
			t.Fatal("JSON error echoed private input", result.raw)
		}
	}
	points := []map[string]any{{"label": "PRIVATE MAP START", "lat": 43.24, "lon": 76.89}, {"label": "PRIVATE MAP FINISH", "lat": 43.25, "lon": 76.91}}
	badCSRF := *owner
	badCSRF.csrf = "incorrect"
	expect(t, f.request("POST", "/api/v1/geo/route", map[string]any{"points": points, "lang": "en"}, &badCSRF), 403)
	route := f.request("POST", "/api/v1/geo/route", map[string]any{"points": points, "lang": "en"}, owner)
	expect(t, route, 200)
	if route.body["plan"].(map[string]any)["distance_meters"] != float64(1500) {
		t.Fatal("route did not return meter summary", route.raw)
	}
	created := f.request("POST", "/api/v1/rides", map[string]any{"city": "Almaty", "description": "Public ride preview", "title": "Private map ride", "starts_at": "2026-11-07T10:00:00Z", "timezone": "Asia/Almaty", "capacity": 5, "map_points": points}, owner)
	expect(t, created, 201)
	ride := created.body["ride"].(map[string]any)
	id := ride["id"].(string)
	if ride["meeting_point"] != "PRIVATE MAP START" || ride["destination"] != "PRIVATE MAP FINISH" || ride["map_plan"] == nil {
		t.Fatal("map was not stored with synchronized labels", created.raw)
	}
	var original string
	if err := f.db.QueryRow(context.Background(), "SELECT map_plan::text FROM rides WHERE id=$1", id).Scan(&original); err != nil {
		t.Fatal(err)
	}
	for _, actor := range []*actor{nil, guest} {
		result := f.request("GET", "/api/v1/rides/"+id, nil, actor)
		expect(t, result, 200)
		if strings.Contains(result.raw, "map_plan") || strings.Contains(result.raw, "PRIVATE MAP") || strings.Contains(result.raw, "76.89") {
			t.Fatal("map leaked outside full ride view", result.raw)
		}
	}
	f.join(id, guest)
	pending := f.request("GET", "/api/v1/rides/"+id, nil, guest)
	if strings.Contains(pending.raw, "map_plan") {
		t.Fatal("pending participant received map", pending.raw)
	}
	expect(t, f.decision(id, owner, guest, "approve"), 200)
	approved := f.request("GET", "/api/v1/rides/"+id, nil, guest)
	expect(t, approved, 200)
	if approved.body["ride"].(map[string]any)["map_plan"] == nil {
		t.Fatal("approved participant missing map")
	}
	expect(t, f.request("PATCH", "/api/v1/rides/"+id, map[string]any{"plan_version": 1, "map_points": points}, guest), 403)
	if routeCalls.Load() != 2 {
		t.Fatal("unauthorized edit reached route provider")
	}
	expect(t, f.request("PATCH", "/api/v1/rides/"+id, map[string]any{"plan_version": 1, "title": "Edited title"}, owner), 200)
	if routeCalls.Load() != 2 {
		t.Fatal("unrelated edit recomputed route")
	}
	expect(t, f.request("PATCH", "/api/v1/rides/"+id, map[string]any{"plan_version": 1, "map_points": points}, owner), 409)
	if routeCalls.Load() != 2 {
		t.Fatal("stale edit reached route provider")
	}
	expect(t, f.request("POST", "/api/v1/geo/route", map[string]any{"points": points, "geometry": map[string]any{}}, owner), 400)
	unavailable.Store(true)
	for _, test := range []struct {
		path string
		body map[string]any
	}{
		{"/api/v1/geo/search", map[string]any{"query": "PRIVATE ADDRESS QUERY 76.891234"}},
		{"/api/v1/geo/reverse", map[string]any{"point": map[string]any{"lat": 43.241234, "lon": 76.891234}}},
	} {
		result := f.request("POST", test.path, test.body, owner)
		expect(t, result, 503)
		if strings.Contains(result.raw, "PRIVATE") || strings.Contains(result.raw, "43.241234") || strings.Contains(result.raw, "76.891234") {
			t.Fatal("upstream failure echoed private request or provider data", result.raw)
		}
	}
	expect(t, f.request("PATCH", "/api/v1/rides/"+id, map[string]any{"plan_version": 2, "map_points": points}, owner), 503)
	expect(t, f.request("GET", "/health/ready", nil, nil), 200)
	var preserved string
	var version int
	if err := f.db.QueryRow(context.Background(), "SELECT map_plan::text,plan_version FROM rides WHERE id=$1", id).Scan(&preserved, &version); err != nil || preserved != original || version != 2 {
		t.Fatal("unavailable router changed saved plan", err)
	}
	cleared := f.request("PATCH", "/api/v1/rides/"+id, map[string]any{"plan_version": 2, "map_points": []any{}}, owner)
	expect(t, cleared, 200)
	if _, exists := cleared.body["ride"].(map[string]any)["map_plan"]; exists {
		t.Fatal("empty points did not clear map")
	}
	pin := f.request("PATCH", "/api/v1/rides/"+id, map[string]any{"plan_version": 3, "map_points": points[:1]}, owner)
	expect(t, pin, 200)
	plan := pin.body["ride"].(map[string]any)["map_plan"].(map[string]any)
	if _, exists := plan["geometry"]; exists || len(plan["points"].([]any)) != 1 {
		t.Fatal("single meeting point required routing", pin.raw)
	}
}

func TestDisabledGeoConfigPreservesTextOnlyRides(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011234203", "Text-only rider")
	result := f.request("GET", "/api/v1/geo/config", nil, owner)
	expect(t, result, 200)
	if result.body["enabled"] != false || result.body["archive_url"] != "" {
		t.Fatal("unconfigured maps were enabled")
	}
	failed := f.request("POST", "/api/v1/geo/search", map[string]any{"query": "PRIVATE ADDRESS QUERY 76.891234"}, owner)
	expect(t, failed, 503)
	if strings.Contains(failed.raw, "PRIVATE ADDRESS") || strings.Contains(failed.raw, "76.891234") {
		t.Fatal("disabled-service error echoed private query", failed.raw)
	}
	f.ride(owner, 5)
	expect(t, f.request("GET", "/health/ready", nil, nil), 200)
}
