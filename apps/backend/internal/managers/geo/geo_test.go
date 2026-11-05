package geo

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"bikers.kz/backend/internal/models"
)

type geocoderFake struct {
	calls int
	input models.MapSearch
}

func (g *geocoderFake) Search(_ context.Context, input models.MapSearch) ([]models.MapPlace, error) {
	g.calls++
	g.input = input
	return []models.MapPlace{{ID: "n:1", Label: "Almaty", Lat: 43.24, Lon: 76.89, Kind: "city"}, {ID: "n:2", Label: "Outside", Lat: 10, Lon: 20}}, nil
}
func (g *geocoderFake) Reverse(ctx context.Context, point models.MapPoint, lang string) ([]models.MapPlace, error) {
	return g.Search(ctx, models.MapSearch{Lang: lang, Near: &point})
}

type routerFake struct {
	calls int
	run   func(context.Context)
}

func (r *routerFake) Route(ctx context.Context, points []models.MapPoint, _ string) (models.MapPlan, error) {
	r.calls++
	if r.run != nil {
		r.run(ctx)
	}
	return models.MapPlan{Geometry: &models.LineString{Type: "LineString", Coordinates: [][2]float64{{points[0].Lon, points[0].Lat}, {points[len(points)-1].Lon, points[len(points)-1].Lat}}}, DistanceMeters: 1200, DurationSeconds: 180}, nil
}

func user() models.User { return models.User{ID: "rider", Profile: models.Profile{Locale: "kk"}} }
func points() []models.MapPoint {
	return []models.MapPoint{{Label: " Start ", Lat: 43.24, Lon: 76.89}, {Label: "Finish", Lat: 43.25, Lon: 76.91}}
}
func fault(t *testing.T, err error, code string) {
	t.Helper()
	var f *models.Fault
	if !errors.As(err, &f) || f.Code != code {
		t.Fatalf("got %v, want %s", err, code)
	}
}

func TestRegionalSearchValidationLanguageAndOutput(t *testing.T) {
	g := &geocoderFake{}
	m := New(g, &routerFake{}, "/map-data/abcdef123456/kazakhstan.pmtiles", nil)
	result, err := m.Search(context.Background(), user(), models.MapSearch{Query: "  Алматы "})
	if err != nil || len(result) != 1 || g.input.Query != "Алматы" || g.input.Lang != "kk" || !m.Config().Enabled {
		t.Fatalf("search normalization failed: %+v %v", result, err)
	}
	for _, input := range []models.MapSearch{{Query: "a"}, {Query: "Almaty", Lang: "de"}, {Query: "Almaty", Near: &models.MapPoint{Lat: math.NaN(), Lon: 76}}, {Query: "Almaty", Near: &models.MapPoint{Lat: 1, Lon: 1}}} {
		_, err := m.Search(context.Background(), user(), input)
		fault(t, err, "invalid_map")
	}
	if g.calls != 1 {
		t.Fatal("invalid query reached geocoder")
	}
	_, err = m.Reverse(context.Background(), user(), models.MapPoint{Lat: 43.24, Lon: 76.89}, "en")
	if err != nil || g.input.Lang != "en" {
		t.Fatal("reverse language was not passed")
	}
}

func TestPlanClearPinAndBoundedRouting(t *testing.T) {
	r := &routerFake{}
	m := New(nil, r, "", nil)
	plan, err := m.Plan(context.Background(), user(), nil)
	if err != nil || plan != nil {
		t.Fatal("clear requires map service")
	}
	plan, err = m.Plan(context.Background(), user(), points()[:1])
	if err != nil || plan.Geometry != nil || plan.Points[0].Label != "Start" || r.calls != 0 {
		t.Fatal("meeting pin unexpectedly routed")
	}
	plan, err = m.Plan(context.Background(), user(), points())
	if err != nil || plan.Geometry == nil || plan.DistanceMeters != 1200 || plan.Points[0].Label != "Start" || r.calls != 1 {
		t.Fatalf("plan failed: %+v %v", plan, err)
	}
	bad := append(make([]models.MapPoint, 10), points()...)
	_, err = m.Plan(context.Background(), user(), bad)
	fault(t, err, "invalid_map")
	bad = points()
	bad[0].Lon = math.Inf(1)
	_, err = m.Plan(context.Background(), user(), bad)
	fault(t, err, "invalid_map")
	_, err = m.Route(context.Background(), user(), models.MapRouteInput{Points: points()[:1]})
	fault(t, err, "invalid_map")
	if r.calls != 1 {
		t.Fatal("invalid input reached router")
	}
	for _, plan := range []models.MapPlan{{}, {Geometry: &models.LineString{Type: "LineString", Coordinates: [][2]float64{{76, 43}, {76, 43}}}, DistanceMeters: math.NaN()}, {Geometry: &models.LineString{Type: "LineString", Coordinates: [][2]float64{{76, 43}, {1, 1}}}}} {
		if validRoute(plan) {
			t.Fatal("malformed regional route accepted")
		}
	}
}

func TestRouteConcurrencyAndBudgetRecover(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	r := &routerFake{run: func(context.Context) { close(entered); <-release }}
	now := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	m := New(nil, r, "", func() time.Time { return now })
	done := make(chan error, 1)
	go func() { _, err := m.Plan(context.Background(), user(), points()); done <- err }()
	<-entered
	_, err := m.Plan(context.Background(), user(), points())
	fault(t, err, "maps_busy")
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	r.run = nil
	for i := 0; i < 10; i++ {
		if _, err := m.Plan(context.Background(), user(), points()); err != nil {
			t.Fatal(err)
		}
	}
	_, err = m.Plan(context.Background(), user(), points())
	fault(t, err, "maps_busy")
	now = now.Add(time.Minute)
	if _, err := m.Plan(context.Background(), user(), points()); err != nil {
		t.Fatal("budget did not recover", err)
	}
}

func TestDisabledServicesKeepMapConfigAndTextPlansAvailable(t *testing.T) {
	m := New(nil, nil, "", nil)
	if m.Config().Enabled || m.Config().ArchiveURL != "" {
		t.Fatal("disabled map config exposed archive")
	}
	if plan, err := m.Plan(context.Background(), user(), nil); err != nil || plan != nil {
		t.Fatal("clear failed while maps disabled")
	}
	_, err := m.Search(context.Background(), user(), models.MapSearch{Query: "Almaty"})
	fault(t, err, "maps_unavailable")
}

func TestProviderErrorsNeverEchoPrivateRequestData(t *testing.T) {
	const private = "PRIVATE ADDRESS QUERY 43.241234 76.891234"
	for _, upstream := range []error{errors.New(private), models.Fail("route_not_found", private)} {
		err := serviceError(upstream)
		if err == nil || strings.Contains(err.Error(), private) || strings.Contains(err.Error(), "76.891234") {
			t.Fatalf("provider error leaked private input: %v", err)
		}
	}
}
