// Package photon adapts the private Photon geocoding API to map places.
package photon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
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
		client = &http.Client{Timeout: 4 * time.Second, Transport: &http.Transport{Proxy: nil, MaxIdleConns: 4, MaxIdleConnsPerHost: 4, MaxConnsPerHost: 4, IdleConnTimeout: 60 * time.Second, ResponseHeaderTimeout: 3 * time.Second}, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("photon redirects disabled") }}
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), http: client}
}

func (c *Client) Search(ctx context.Context, input models.MapSearch) ([]models.MapPlace, error) {
	query := url.Values{"q": {input.Query}, "lang": {input.Lang}, "limit": {"8"}, "countrycode": {"KZ"}, "bbox": {"46,40.5,87.5,55.5"}}
	if input.Near != nil {
		query.Set("lat", number(input.Near.Lat))
		query.Set("lon", number(input.Near.Lon))
	}
	return c.places(ctx, "/api/", query)
}

func (c *Client) Reverse(ctx context.Context, point models.MapPoint, lang string) ([]models.MapPlace, error) {
	return c.places(ctx, "/reverse", url.Values{"lat": {number(point.Lat)}, "lon": {number(point.Lon)}, "lang": {lang}, "limit": {"5"}, "radius": {"2"}})
}

func number(value float64) string { return strconv.FormatFloat(value, 'f', -1, 64) }

type feature struct {
	Type     string `json:"type"`
	Geometry struct {
		Type        string    `json:"type"`
		Coordinates []float64 `json:"coordinates"`
	} `json:"geometry"`
	Properties struct {
		Name        string `json:"name"`
		Street      string `json:"street"`
		HouseNumber string `json:"housenumber"`
		City        string `json:"city"`
		District    string `json:"district"`
		State       string `json:"state"`
		CountryCode string `json:"countrycode"`
		OSMType     string `json:"osm_type"`
		OSMID       int64  `json:"osm_id"`
		OSMValue    string `json:"osm_value"`
		Type        string `json:"type"`
	} `json:"properties"`
}

func (c *Client) places(ctx context.Context, path string, query url.Values) ([]models.MapPlace, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path+"?"+query.Encode(), nil)
	if err != nil {
		return nil, errors.New("invalid photon endpoint")
	}
	request.Header.Set("Accept", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return nil, errors.New("photon unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, errors.New("photon request failed")
	}
	const maxBytes = 256 << 10
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil || len(body) > maxBytes {
		return nil, errors.New("photon response too large")
	}
	var collection struct {
		Type     string    `json:"type"`
		Features []feature `json:"features"`
	}
	if err := json.Unmarshal(body, &collection); err != nil || (collection.Type != "" && collection.Type != "FeatureCollection") || collection.Features == nil || len(collection.Features) > 100 {
		return nil, errors.New("invalid photon response")
	}
	places := make([]models.MapPlace, 0, 8)
	seen := make(map[string]bool)
	for _, f := range collection.Features {
		p := f.Properties
		if f.Type != "Feature" || f.Geometry.Type != "Point" || len(f.Geometry.Coordinates) != 2 || !strings.EqualFold(p.CountryCode, "KZ") || !models.ValidCoordinate(f.Geometry.Coordinates[1], f.Geometry.Coordinates[0]) || p.OSMID <= 0 || (p.OSMType != "N" && p.OSMType != "W" && p.OSMType != "R") {
			continue
		}
		address := strings.TrimSpace(p.Street + " " + p.HouseNumber)
		label := joined(p.Name, address, p.City, p.District, p.State)
		if label == "" || len(label) > 1000 {
			continue
		}
		kind := p.Type
		if kind == "" {
			kind = p.OSMValue
		}
		id := fmt.Sprintf("%s:%d", strings.ToLower(p.OSMType), p.OSMID)
		if seen[id] {
			continue
		}
		seen[id] = true
		places = append(places, models.MapPlace{ID: id, Label: label, Lat: f.Geometry.Coordinates[1], Lon: f.Geometry.Coordinates[0], Kind: kind})
		if len(places) == 8 {
			break
		}
	}
	return places, nil
}

func joined(values ...string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		duplicate := false
		for _, part := range parts {
			if strings.EqualFold(part, value) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, ", ")
}
