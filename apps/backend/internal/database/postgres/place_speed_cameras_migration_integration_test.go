package postgres

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func removeSpeedCameras(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	removeRideCovers(t, pool)
	if _, err := pool.Exec(context.Background(), `ALTER TABLE places DROP CONSTRAINT places_category_check;
ALTER TABLE places ADD CONSTRAINT places_category_check CHECK(category IN ('fuel','workshop','tires','motorcycle_shop','parking','meeting_spot'));
DELETE FROM schema_migrations WHERE version=13`); err != nil {
		t.Fatal(err)
	}
}

func TestVersionTwelveSpeedCameraUpgradePreservesPlacesAndPendingReview(t *testing.T) {
	store, pool := testStore(t)
	ctx := context.Background()
	removeSpeedCameras(t, pool)
	const insertPlace = `INSERT INTO places(id,name,category,lat,lon,city,address,description,website,phone,opening_hours,source,source_url,status,version,updated_at)
VALUES($1,'Existing community place',$2,51.1694,71.4491,'Astana','','','','','','community','','published',3,now())`
	for _, category := range []string{"fuel", "workshop", "tires", "motorcycle_shop", "parking", "meeting_spot"} {
		if _, err := pool.Exec(ctx, insertPlace, uuid.NewString(), category); err != nil {
			t.Fatal(err)
		}
	}
	owner, pending := uuid.NewString(), uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,phone,display_name,created_at) VALUES($1,'+77011234931','Camera contributor',now())`, owner); err != nil {
		t.Fatal(err)
	}
	proposal, err := json.Marshal(models.PlaceInput{Name: "Unreviewed camera", Category: "speed_camera", Lat: 51.1694, Lon: 71.4491, City: "Astana"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO place_submissions(id,kind,proposal,reason,submitter_id,status,version,created_at,updated_at)
VALUES($1,'new',$2::jsonb,'Contributor evidence',$3,'pending',1,now(),now())`, pending, proposal, owner); err != nil {
		t.Fatal(err)
	}
	snapshot := func() []string {
		t.Helper()
		var places, submissions, receipts string
		if err := pool.QueryRow(ctx, `SELECT coalesce(jsonb_agg(to_jsonb(p) ORDER BY id),'[]'::jsonb)::text FROM places p`).Scan(&places); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `SELECT coalesce(jsonb_agg(to_jsonb(s) ORDER BY id),'[]'::jsonb)::text FROM place_submissions s`).Scan(&submissions); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `SELECT jsonb_agg(to_jsonb(m) ORDER BY version)::text FROM schema_migrations m WHERE version<=12`).Scan(&receipts); err != nil {
			t.Fatal(err)
		}
		return []string{places, submissions, receipts}
	}
	before := snapshot()
	if store.Ready(ctx) == nil {
		t.Fatal("schema twelve accepted before the speed-camera migration")
	}
	for range 2 {
		if err := store.Migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Ready(ctx); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, snapshot()) {
		t.Fatal("constraint migration/replay changed places, pending review or earlier receipts")
	}
	if _, err := pool.Exec(ctx, insertPlace, uuid.NewString(), "speed_camera"); err != nil {
		t.Fatal("schema thirteen rejected the new category", err)
	}
	if _, err := pool.Exec(ctx, insertPlace, uuid.NewString(), "unknown"); err == nil {
		t.Fatal("expanded constraint accepted an unknown category")
	}
}
