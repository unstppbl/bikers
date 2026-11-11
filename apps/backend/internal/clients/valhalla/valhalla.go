// Package valhalla adapts fixed road-motorcycle routing to a bounded map plan.
package valhalla

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"bikers.kz/backend/internal/models"
)

type Client struct {
	baseURL string
	http    *http.Client
}
type Options struct{ HTTPClient *http.Client }

func New(baseURL string, options Options) *Client {
	client := options.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second, Transport: &http.Transport{Proxy: nil, MaxIdleConns: 1, MaxIdleConnsPerHost: 1, MaxConnsPerHost: 1, IdleConnTimeout: 60 * time.Second, ResponseHeaderTimeout: 7 * time.Second}, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("valhalla redirects disabled") }}
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), http: client}
}

func (c *Client) Route(ctx context.Context, points []models.MapPoint, _ string) (models.MapPlan, error) {
	locations := make([]map[string]any, len(points))
	for i, point := range points {
		locations[i] = map[string]any{"lat": point.Lat, "lon": point.Lon, "type": "break", "search_cutoff": 1000}
	}
	// GeoJSON is supported by the OSRM response format. Directions are omitted:
	// this is a planning estimate, and does not implement navigation. Omit the
	// narration language because no directions are requested and Kazakh is not a
	// supported Valhalla narration locale.
	payload := map[string]any{"locations": locations, "costing": "motorcycle", "costing_options": map[string]any{"motorcycle": map[string]any{"use_trails": 0, "use_tracks": 0}}, "format": "osrm", "shape_format": "geojson", "directions_type": "none", "alternates": 0, "units": "kilometers"}
	body, err := json.Marshal(payload)
	if err != nil {
		return models.MapPlan{}, errors.New("invalid valhalla request")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/route", bytes.NewReader(body))
	if err != nil {
		return models.MapPlan{}, errors.New("invalid valhalla endpoint")
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return models.MapPlan{}, errors.New("valhalla unavailable")
	}
	defer response.Body.Close()
	const maxBytes = 8 << 20
	body, err = io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil || len(body) > maxBytes {
		return models.MapPlan{}, errors.New("valhalla response too large")
	}
	var result struct {
		Code      string `json:"code"`
		ErrorCode int    `json:"error_code"`
		Routes    []struct {
			Geometry struct {
				Type        string      `json:"type"`
				Coordinates [][]float64 `json:"coordinates"`
			} `json:"geometry"`
			Distance float64 `json:"distance"`
			Duration float64 `json:"duration"`
		} `json:"routes"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return models.MapPlan{}, errors.New("invalid valhalla response")
	}
	if result.Code == "NoRoute" || (response.StatusCode == http.StatusBadRequest && (result.ErrorCode == 170 || result.ErrorCode == 171 || result.ErrorCode == 442 || result.ErrorCode == 443)) {
		return models.MapPlan{}, models.Fail("route_not_found", "No motorcycle route found between these points")
	}
	if response.StatusCode != http.StatusOK || result.Code != "Ok" || len(result.Routes) != 1 {
		return models.MapPlan{}, errors.New("valhalla request failed")
	}
	route := result.Routes[0]
	if route.Geometry.Type != "LineString" || len(route.Geometry.Coordinates) < 2 || len(route.Geometry.Coordinates) > 50000 {
		return models.MapPlan{}, errors.New("invalid valhalla geometry")
	}
	coordinates := make([][2]float64, len(route.Geometry.Coordinates))
	for i, coordinate := range route.Geometry.Coordinates {
		if len(coordinate) != 2 || !models.ValidCoordinate(coordinate[1], coordinate[0]) {
			return models.MapPlan{}, errors.New("invalid valhalla coordinate")
		}
		coordinates[i] = [2]float64{coordinate[0], coordinate[1]}
	}
	return models.MapPlan{Points: points, Geometry: &models.LineString{Type: "LineString", Coordinates: coordinates}, DistanceMeters: route.Distance, DurationSeconds: route.Duration}, nil
}
