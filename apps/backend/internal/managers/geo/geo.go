// Package geo owns Kazakhstan map bounds, request budgets, and route planning.
package geo

import (
	"context"
	"math"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"bikers.kz/backend/internal/models"
)

const MaxGeometryCoordinates = 50000

type Geocoder interface {
	Search(context.Context, models.MapSearch) ([]models.MapPlace, error)
	Reverse(context.Context, models.MapPoint, string) ([]models.MapPlace, error)
}

type Router interface {
	Route(context.Context, []models.MapPoint, string) (models.MapPlan, error)
}

type budget struct {
	start            time.Time
	searches, routes int
}

type Manager struct {
	geocoder                Geocoder
	router                  Router
	archiveURL              string
	now                     func() time.Time
	searchSlots, routeSlots chan struct{}
	mu                      sync.Mutex
	budgets                 map[string]budget
}

func New(geocoder Geocoder, router Router, archiveURL string, now func() time.Time) *Manager {
	if now == nil {
		now = time.Now
	}
	return &Manager{geocoder: geocoder, router: router, archiveURL: archiveURL, now: now, searchSlots: make(chan struct{}, 4), routeSlots: make(chan struct{}, 1), budgets: make(map[string]budget)}
}

func (m *Manager) Config() models.MapConfig {
	enabled := m.geocoder != nil && m.router != nil && m.archiveURL != ""
	url := ""
	if enabled {
		url = m.archiveURL
	}
	return models.MapConfig{Enabled: enabled, ArchiveURL: url}
}

func (m *Manager) Search(ctx context.Context, actor models.User, input models.MapSearch) ([]models.MapPlace, error) {
	input.Query = strings.TrimSpace(input.Query)
	if utf8.RuneCountInString(input.Query) < 2 || len(input.Query) > 240 || !utf8.ValidString(input.Query) {
		return nil, invalid()
	}
	lang, err := language(input.Lang, actor.Profile.Locale)
	if err != nil {
		return nil, err
	}
	input.Lang = lang
	if input.Near != nil && !inKazakhstan(input.Near.Lat, input.Near.Lon) {
		return nil, invalid()
	}
	if m.geocoder == nil {
		return nil, unavailable()
	}
	if err := m.acquire(actor.ID, false); err != nil {
		return nil, err
	}
	defer func() { <-m.searchSlots }()
	places, err := m.geocoder.Search(ctx, input)
	return cleanPlaces(places), serviceError(err)
}

func (m *Manager) Reverse(ctx context.Context, actor models.User, point models.MapPoint, lang string) ([]models.MapPlace, error) {
	if !inKazakhstan(point.Lat, point.Lon) {
		return nil, invalid()
	}
	lang, err := language(lang, actor.Profile.Locale)
	if err != nil {
		return nil, err
	}
	if m.geocoder == nil {
		return nil, unavailable()
	}
	if err := m.acquire(actor.ID, false); err != nil {
		return nil, err
	}
	defer func() { <-m.searchSlots }()
	places, err := m.geocoder.Reverse(ctx, point, lang)
	return cleanPlaces(places), serviceError(err)
}

// Plan accepts zero points to clear a saved plan and one point for a meeting pin.
// Rides call this before acquiring persistence locks.
func (m *Manager) Plan(ctx context.Context, actor models.User, points []models.MapPoint) (*models.MapPlan, error) {
	return m.plan(ctx, actor, points, actor.Profile.Locale)
}

func (m *Manager) Route(ctx context.Context, actor models.User, input models.MapRouteInput) (models.MapPlan, error) {
	if len(input.Points) < 2 {
		return models.MapPlan{}, invalid()
	}
	plan, err := m.plan(ctx, actor, input.Points, input.Lang)
	if err != nil {
		return models.MapPlan{}, err
	}
	return *plan, nil
}

func (m *Manager) plan(ctx context.Context, actor models.User, points []models.MapPoint, lang string) (*models.MapPlan, error) {
	if len(points) > 10 {
		return nil, invalid()
	}
	lang, err := language(lang, actor.Profile.Locale)
	if err != nil {
		return nil, err
	}
	if len(points) == 0 {
		return nil, nil
	}
	normalized := make([]models.MapPoint, len(points))
	for i, point := range points {
		point.Label = strings.TrimSpace(point.Label)
		if point.Label == "" || len(point.Label) > 1000 || !utf8.ValidString(point.Label) || !inKazakhstan(point.Lat, point.Lon) {
			return nil, invalid()
		}
		normalized[i] = point
	}
	if len(points) == 1 {
		return &models.MapPlan{Points: normalized}, nil
	}
	if m.router == nil {
		return nil, unavailable()
	}
	if err := m.acquire(actor.ID, true); err != nil {
		return nil, err
	}
	defer func() { <-m.routeSlots }()
	plan, err := m.router.Route(ctx, normalized, lang)
	if err != nil {
		return nil, serviceError(err)
	}
	if !validRoute(plan) {
		return nil, unavailable()
	}
	plan.Points = normalized
	return &plan, nil
}

func validRoute(plan models.MapPlan) bool {
	if plan.Geometry == nil || plan.Geometry.Type != "LineString" || len(plan.Geometry.Coordinates) < 2 || len(plan.Geometry.Coordinates) > MaxGeometryCoordinates || math.IsNaN(plan.DistanceMeters) || math.IsInf(plan.DistanceMeters, 0) || plan.DistanceMeters < 0 || plan.DistanceMeters > 10000000 || math.IsNaN(plan.DurationSeconds) || math.IsInf(plan.DurationSeconds, 0) || plan.DurationSeconds < 0 || plan.DurationSeconds > 7*24*60*60 {
		return false
	}
	for _, coordinate := range plan.Geometry.Coordinates {
		if !inKazakhstan(coordinate[1], coordinate[0]) {
			return false
		}
	}
	return true
}

// This regional envelope bounds workload; it is not a political-border test.
func inKazakhstan(lat, lon float64) bool {
	return models.ValidCoordinate(lat, lon) && lat >= 40.5 && lat <= 55.5 && lon >= 46 && lon <= 87.5
}

func language(value, fallback string) (string, error) {
	if value == "" {
		value = fallback
	}
	if value == "" {
		value = "ru"
	}
	switch value {
	case "kk", "ru", "en":
		return value, nil
	default:
		return "", invalid()
	}
}

func cleanPlaces(input []models.MapPlace) []models.MapPlace {
	out := make([]models.MapPlace, 0, 8)
	for _, place := range input {
		if len(out) == 8 {
			break
		}
		if place.Label == "" || len(place.Label) > 1000 || len(place.ID) > 160 || len(place.Kind) > 80 || !inKazakhstan(place.Lat, place.Lon) {
			continue
		}
		out = append(out, place)
	}
	return out
}

func (m *Manager) acquire(actor string, route bool) error {
	if actor == "" {
		return models.Fail("authentication_required", "Sign in to continue")
	}
	m.mu.Lock()
	now := m.now()
	entry := m.budgets[actor]
	if !now.Before(entry.start.Add(time.Minute)) {
		entry = budget{start: now}
	}
	if len(m.budgets) >= 10000 {
		for id, old := range m.budgets {
			if !now.Before(old.start.Add(time.Minute)) {
				delete(m.budgets, id)
			}
		}
		if _, exists := m.budgets[actor]; !exists && len(m.budgets) >= 10000 {
			m.mu.Unlock()
			return busy()
		}
	}
	if (route && entry.routes >= 12) || (!route && entry.searches >= 60) {
		m.mu.Unlock()
		return busy()
	}
	if route {
		entry.routes++
	} else {
		entry.searches++
	}
	m.budgets[actor] = entry
	m.mu.Unlock()
	slots := m.searchSlots
	if route {
		slots = m.routeSlots
	}
	select {
	case slots <- struct{}{}:
		return nil
	default:
		return busy()
	}
}

func invalid() error {
	return models.Fail("invalid_map", "Check Kazakhstan coordinates, points, query and language")
}
func unavailable() error { return models.Fail("maps_unavailable", "Maps are temporarily unavailable") }
func busy() error        { return models.Fail("maps_busy", "Map request limit reached; retry shortly") }
func serviceError(err error) error {
	if err != nil {
		if fault, ok := err.(*models.Fault); ok && fault.Code == "route_not_found" {
			return models.Fail("route_not_found", "No motorcycle route found between these points")
		}
		return unavailable()
	}
	return nil
}
