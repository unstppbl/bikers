package postgres

import (
	"context"
	"reflect"
	"testing"
	"time"

	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func removeRideMaps(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	removePlaces(t, pool)
	if _, err := pool.Exec(context.Background(), `ALTER TABLE rides DROP COLUMN map_plan; DELETE FROM schema_migrations WHERE version=10`); err != nil {
		t.Fatal(err)
	}
}

func TestVersionNineMapUpgradePreservesRideAndOldReceipts(t *testing.T) {
	store, pool := testStore(t)
	removeRideMaps(t, pool)
	ctx := context.Background()
	owner, id := uuid.NewString(), uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,phone,display_name,created_at) VALUES($1,'+77011234678','Existing owner',now())`, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO rides(id,organizer_id,city,description,title,starts_at,timezone,meeting_point,destination,route_url,capacity,plan_version,created_at) VALUES($1,$2,'Almaty','Public preview','Private plan',now()+interval '1 day','Asia/Almaty','Meeting text','Destination text','https://example.com/ride',5,7,now())`, id, owner); err != nil {
		t.Fatal(err)
	}
	const retained = `SELECT jsonb_build_object('ride',(SELECT to_jsonb(r)-'map_plan' FROM rides r WHERE id=$1),'receipts',(SELECT jsonb_agg(to_jsonb(m) ORDER BY version) FROM schema_migrations m WHERE version<=9))::text`
	var before, after string
	if err := pool.QueryRow(ctx, retained, id).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if err := store.Ready(ctx); err == nil {
		t.Fatal("schema nine accepted before migration")
	}
	for range 2 {
		if err := store.Migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Ready(ctx); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, retained, id).Scan(&after); err != nil || before != after {
		t.Fatal("map migration changed preexisting data or receipts", err)
	}
	var unset bool
	if err := pool.QueryRow(ctx, `SELECT map_plan IS NULL FROM rides WHERE id=$1`, id).Scan(&unset); err != nil || !unset {
		t.Fatal("existing ride was assigned a map", err)
	}
	ride, err := store.Ride(ctx, id)
	if err != nil || ride.MapPlan != nil || ride.PlanVersion != 7 {
		t.Fatal("text-only ride did not round trip", err)
	}
	plan := &models.MapPlan{Points: []models.MapPoint{{Label: "A", Lat: 43.24, Lon: 76.89}, {Label: "B", Lat: 43.25, Lon: 76.91}}, Geometry: &models.LineString{Type: "LineString", Coordinates: [][2]float64{{76.89, 43.24}, {76.91, 43.25}}}, DistanceMeters: 1200, DurationSeconds: 180}
	if err := store.WithRide(ctx, id, func(tx database.RideTransaction) error { r := tx.Ride(); r.MapPlan = plan; return tx.UpdateRide(r) }); err != nil {
		t.Fatal(err)
	}
	ride, err = store.Ride(ctx, id)
	if err != nil || !reflect.DeepEqual(ride.MapPlan, plan) {
		t.Fatalf("map plan did not round trip: %+v %v", ride.MapPlan, err)
	}
	if err := store.WithRide(ctx, id, func(tx database.RideTransaction) error { r := tx.Ride(); r.MapPlan = nil; return tx.UpdateRide(r) }); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT map_plan IS NULL FROM rides WHERE id=$1`, id).Scan(&unset); err != nil || !unset {
		t.Fatal("clear did not restore SQL NULL", err)
	}
	_, err = store.CreateRide(ctx, models.Ride{RideSummary: models.RideSummary{ID: uuid.NewString(), City: "Almaty", Description: "Public preview", Status: "published"}, Title: "Text plan", StartsAt: time.Now().Add(time.Hour), Timezone: "Asia/Almaty", MeetingPoint: "Meeting", Capacity: 2, PlanVersion: 1, OrganizerID: owner}, "approved", time.Now(), func(models.GarageOwner) error { return nil })
	if err != nil {
		t.Fatal("text-only creation failed after map migration", err)
	}
	for _, invalid := range []string{`{}`, `null`, `{"points":[],"distance_meters":0,"duration_seconds":0}`} {
		if _, err := pool.Exec(ctx, `UPDATE rides SET map_plan=$2::jsonb WHERE id=$1`, id, invalid); err == nil {
			t.Fatalf("invalid persisted map accepted: %s", invalid)
		}
	}
}
