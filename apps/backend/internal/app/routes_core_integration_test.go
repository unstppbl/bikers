package app

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/database/postgres"
	routemanager "bikers.kz/backend/internal/managers/routes"
	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type communityPlanner struct{ before func(context.Context) error }

func (p communityPlanner) Plan(ctx context.Context, _ models.User, points []models.MapPoint) (*models.MapPlan, error) {
	if p.before != nil {
		if err := p.before(ctx); err != nil {
			return nil, err
		}
	}
	coordinates := make([][2]float64, len(points))
	for i, v := range points {
		coordinates[i] = [2]float64{v.Lon, v.Lat}
	}
	return &models.MapPlan{Points: points, Geometry: &models.LineString{Type: "LineString", Coordinates: coordinates}, DistanceMeters: 1200, DurationSeconds: 180}, nil
}
func routeActor(t *testing.T, pool *pgxpool.Pool, name string) models.User {
	t.Helper()
	u := models.User{ID: uuid.NewString(), Profile: models.Profile{DisplayName: name, City: "Almaty", Languages: []string{"en"}, Locale: "en"}, OnboardingComplete: true}
	if _, err := pool.Exec(context.Background(), `INSERT INTO users(id,phone,display_name,city,languages,locale,onboarding_complete,created_at) VALUES($1,$2,$3,'Almaty',ARRAY['en'],'en',true,now())`, u.ID, "+7"+strings.ReplaceAll(u.ID, "-", ""), name); err != nil {
		t.Fatal(err)
	}
	return u
}
func routeFault(t *testing.T, err error, code string) {
	t.Helper()
	var f *models.Fault
	if !errors.As(err, &f) || f.Code != code {
		t.Fatalf("wanted %s, got %v", code, err)
	}
}
func routeInput() models.CommunityRouteInput {
	return models.CommunityRouteInput{Title: "Mountain road", City: "Almaty", Description: "Community route", Surface: "asphalt", MapPoints: []models.MapPoint{{Label: "Start", Lon: 76.89, Lat: 43.24}, {Label: "End", Lon: 76.91, Lat: 43.25}}}
}

func TestCommunityRouteRecordedSnapshotAndPrivateCollections(t *testing.T) {
	store, pool := routeCoreStore(t)
	ctx := context.Background()
	owner := routeActor(t, pool, "Route owner")
	other := routeActor(t, pool, "Other rider")
	m := routemanager.New(store, communityPlanner{}, time.Now)
	track, err := m.CreateTrack(ctx, owner, models.TrackInput{Name: "Private recording name", Segments: [][][2]float64{{{76.89, 43.24}, {76.90, 43.24}}, {{80, 50}, {80.01, 50}}}, DurationSeconds: 120})
	if err != nil {
		t.Fatal(err)
	}
	if track.DistanceMeters < 1500 || track.DistanceMeters > 1700 {
		t.Fatal("GPS pause gap included in distance", track.DistanceMeters)
	}
	_, err = m.Track(ctx, other, track.ID)
	routeFault(t, err, "not_found")
	in := routeInput()
	in.MapPoints = nil
	in.TrackID = track.ID
	r, err := m.Create(ctx, owner, in)
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.View(ctx, other, r.ID)
	routeFault(t, err, "not_found")
	in.Version = r.Version
	in.Published = true
	_, err = m.Update(ctx, owner, r.ID, in)
	routeFault(t, err, "publication_consent_required")
	in.PublishConsent = true
	r, err = m.Update(ctx, owner, r.ID, in)
	if err != nil {
		t.Fatal(err)
	}
	public, err := m.View(ctx, models.User{}, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(public.Segments) != 2 || public.Kind != "recorded" {
		t.Fatal("recorded route lost its pause gaps")
	}
	b, err := json.Marshal(public)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), track.ID) || strings.Contains(string(b), "Private recording name") || strings.Contains(string(b), "track_id") {
		t.Fatal("public route exposed private recording linkage", string(b))
	}
	page, err := m.List(ctx, models.User{}, routemanager.ListInput{})
	if err != nil || len(page.Routes) != 1 {
		t.Fatal(page, err)
	}
	b, _ = json.Marshal(page)
	for _, field := range []string{"segments", "points", "track_id"} {
		if strings.Contains(string(b), field) {
			t.Fatal("public summary exposed geometry", string(b))
		}
	}
	collection, err := m.CreateCollection(ctx, other, models.RouteCollectionInput{Name: "Weekend"})
	if err != nil {
		t.Fatal(err)
	}
	if err = m.SetCollectionRoute(ctx, other, collection.ID, r.ID, true); err != nil {
		t.Fatal(err)
	}
	_, err = m.List(ctx, owner, routemanager.ListInput{CollectionID: collection.ID})
	routeFault(t, err, "not_found")
	saved, err := m.List(ctx, other, routemanager.ListInput{CollectionID: collection.ID})
	if err != nil || len(saved.Routes) != 1 {
		t.Fatal(saved, err)
	}
	if err = m.DeleteTrack(ctx, owner, track.ID, track.Version); err != nil {
		t.Fatal(err)
	}
	public, err = m.View(ctx, models.User{}, r.ID)
	if err != nil || !reflect.DeepEqual(public.Segments, track.Segments) || public.SourceTrackID != "" {
		t.Fatal("deleting private source changed published snapshot", public, err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO user_blocks(blocker_id,blocked_id,created_at) VALUES($1,$2,now())`, owner.ID, other.ID); err != nil {
		t.Fatal(err)
	}
	_, err = m.View(ctx, other, r.ID)
	routeFault(t, err, "not_found")
	saved, err = m.List(ctx, other, routemanager.ListInput{CollectionID: collection.ID})
	if err != nil || len(saved.Routes) != 0 {
		t.Fatal("saved collection bypassed bilateral block", saved, err)
	}
	if _, err = m.View(ctx, models.User{}, r.ID); err != nil {
		t.Fatal("anonymous public access unexpectedly lost", err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO moderation_hidden(target_type,target_id,hidden_at) VALUES('route',$1,now())`, r.ID); err != nil {
		t.Fatal(err)
	}
	_, err = m.View(ctx, models.User{}, r.ID)
	routeFault(t, err, "not_found")
	page, err = m.List(ctx, models.User{}, routemanager.ListInput{})
	if err != nil || len(page.Routes) != 0 {
		t.Fatal("hidden route remained discoverable", page, err)
	}
	if _, err = m.View(ctx, owner, r.ID); err != nil {
		t.Fatal("owner cannot manage hidden route", err)
	}
}

func TestCommunityRoutePlanningRechecksPermissionsAndVersions(t *testing.T) {
	store, pool := routeCoreStore(t)
	ctx := context.Background()
	owner := routeActor(t, pool, "Planner owner")
	planner := communityPlanner{before: func(ctx context.Context) error {
		probe, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if err := store.WithRoutes(probe, func(database.RouteTransaction) error { return nil }); err != nil {
			return err
		}
		_, err := pool.Exec(ctx, `UPDATE users SET status='suspended' WHERE id=$1`, owner.ID)
		return err
	}}
	m := routemanager.New(store, planner, time.Now)
	_, err := m.Create(ctx, owner, routeInput())
	routeFault(t, err, "account_restricted")
	var n int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM community_routes`).Scan(&n); err != nil || n != 0 {
		t.Fatal("restricted actor committed route", n, err)
	}
	if _, err = pool.Exec(ctx, `UPDATE users SET status='active' WHERE id=$1`, owner.ID); err != nil {
		t.Fatal(err)
	}
	m = routemanager.New(store, communityPlanner{}, time.Now)
	r, err := m.Create(ctx, owner, routeInput())
	if err != nil {
		t.Fatal(err)
	}
	in := routeInput()
	in.Version = r.Version
	in.MapPoints[1].Lon += 0.01
	planner.before = func(ctx context.Context) error {
		_, err := pool.Exec(ctx, `UPDATE community_routes SET version=version+1 WHERE id=$1`, r.ID)
		return err
	}
	m = routemanager.New(store, planner, time.Now)
	_, err = m.Update(ctx, owner, r.ID, in)
	routeFault(t, err, "stale_version")
}

func TestCommunityRoutePublicGeometryEditsRequireFreshConsent(t *testing.T) {
	store, pool := routeCoreStore(t)
	ctx := context.Background()
	owner := routeActor(t, pool, "Public owner")
	m := routemanager.New(store, communityPlanner{}, time.Now)
	in := routeInput()
	in.Published = true
	_, err := m.Create(ctx, owner, in)
	routeFault(t, err, "publication_consent_required")
	in.PublishConsent = true
	r, err := m.Create(ctx, owner, in)
	if err != nil {
		t.Fatal(err)
	}
	in.Version = r.Version
	in.PublishConsent = false
	in.Title = "Changed title"
	r, err = m.Update(ctx, owner, r.ID, in)
	if err != nil {
		t.Fatal("metadata edit required new geometry consent", err)
	}
	in.Version = r.Version
	in.MapPoints[1].Lon += 0.01
	_, err = m.Update(ctx, owner, r.ID, in)
	routeFault(t, err, "publication_consent_required")
	in.Published = false
	r, err = m.Update(ctx, owner, r.ID, in)
	if err != nil {
		t.Fatal("unpublishing required consent", err)
	}
	_, err = m.View(ctx, models.User{}, r.ID)
	routeFault(t, err, "not_found")
}

func TestCommunityRouteConditionsAttributionExpiryAndDeletionSafeQuota(t *testing.T) {
	store, pool := routeCoreStore(t)
	ctx := context.Background()
	owner := routeActor(t, pool, "Route organizer")
	author := routeActor(t, pool, "Reporter")
	at := time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)
	m := routemanager.New(store, communityPlanner{}, func() time.Time { return at })
	in := routeInput()
	in.Published = true
	in.PublishConsent = true
	r, err := m.Create(ctx, owner, in)
	if err != nil {
		t.Fatal(err)
	}
	input := models.RouteConditionInput{Kind: "roadworks", Description: "Work at the bend", ObservedAt: at.Add(-30 * 24 * time.Hour)}
	c, err := m.CreateCondition(ctx, author, r.ID, input)
	if err != nil || c.State != "expired" || !c.ExpiresAt.Equal(at) || c.AuthorID != author.ID {
		t.Fatal(c, err)
	}
	input.Version = c.Version
	input.Resolved = true
	_, err = m.UpdateCondition(ctx, owner, r.ID, c.ID, input)
	routeFault(t, err, "not_found")
	c, err = m.UpdateCondition(ctx, author, r.ID, c.ID, input)
	if err != nil || c.State != "resolved" {
		t.Fatal(c, err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO user_blocks(blocker_id,blocked_id,created_at) VALUES($1,$2,now())`, owner.ID, author.ID); err != nil {
		t.Fatal(err)
	}
	page, err := m.Conditions(ctx, owner, r.ID, "", 20)
	if err != nil || len(page.Conditions) != 0 {
		t.Fatal("condition bypassed bilateral block", page, err)
	}
	if err = m.DeleteCondition(ctx, author, r.ID, c.ID, c.Version); err != nil {
		t.Fatal("author could not delete own blocked report", err)
	}
	if _, err = pool.Exec(ctx, `DELETE FROM user_blocks`); err != nil {
		t.Fatal(err)
	}
	input.Version = 0
	for range 19 {
		c, err = m.CreateCondition(ctx, author, r.ID, input)
		if err != nil {
			t.Fatal(err)
		}
		if err = m.DeleteCondition(ctx, author, r.ID, c.ID, c.Version); err != nil {
			t.Fatal(err)
		}
	}
	_, err = m.CreateCondition(ctx, author, r.ID, input)
	routeFault(t, err, "condition_limited")
	if err = m.Delete(ctx, owner, r.ID, r.Version); err != nil {
		t.Fatal(err)
	}
	r, err = m.Create(ctx, owner, in)
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.CreateCondition(ctx, author, r.ID, input)
	routeFault(t, err, "condition_limited")
}

func TestCommunityRouteConcurrentQuotaIsAtomic(t *testing.T) {
	store, pool := routeCoreStore(t)
	ctx := context.Background()
	owner := routeActor(t, pool, "Route author")
	reporter := routeActor(t, pool, "Concurrent reporter")
	at := time.Now().UTC()
	m := routemanager.New(store, communityPlanner{}, func() time.Time { return at })
	in := routeInput()
	in.Published = true
	in.PublishConsent = true
	r, err := m.Create(ctx, owner, in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO route_daily_limits(owner_id,day,conditions_created) VALUES($1,($2::timestamptz AT TIME ZONE 'UTC')::date,19)`, reporter.ID, at); err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := m.CreateCondition(ctx, reporter, r.ID, models.RouteConditionInput{Kind: "damage", Description: "Road surface damage", ObservedAt: at})
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	passed, limited := 0, 0
	for err := range results {
		if err == nil {
			passed++
		} else {
			routeFault(t, err, "condition_limited")
			limited++
		}
	}
	if passed != 1 || limited != 1 {
		t.Fatal("concurrent requests bypassed quota", passed, limited)
	}
}

func routeCoreStore(t *testing.T) (*postgres.Store, *pgxpool.Pool) {
	t.Helper()
	f := newFixture(t)
	return postgres.New(f.db), f.db
}
