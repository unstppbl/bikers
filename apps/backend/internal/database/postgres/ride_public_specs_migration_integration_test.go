package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestVersionFifteenUpgradeKeepsExistingPrivateRideDetailsUnpublished(t *testing.T) {
	store, pool := testStore(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `ALTER TABLE rides DROP COLUMN style,DROP COLUMN surface,DROP COLUMN distance_km,DROP COLUMN public_details_published;
DELETE FROM schema_migrations WHERE version=16`); err != nil {
		t.Fatal(err)
	}
	owner, ride := uuid.NewString(), uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,phone,display_name,created_at) VALUES($1,'+77011239804','Private organizer',now())`, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO rides(id,organizer_id,city,description,title,starts_at,timezone,meeting_point,capacity,created_at)
VALUES($1,$2,'Almaty','Public description','Private title',now()+interval '1 day','Asia/Almaty','Private location',8,now())`, ride, owner); err != nil {
		t.Fatal(err)
	}
	var before string
	if err := pool.QueryRow(ctx, `SELECT to_jsonb(r)::text FROM rides r WHERE id=$1`, ride).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if store.Ready(ctx) == nil {
		t.Fatal("schema fifteen was incorrectly considered ready")
	}
	for range 2 {
		if err := store.Migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var after, style, surface string
	var distance int
	var published bool
	if err := pool.QueryRow(ctx, `SELECT (to_jsonb(r)-'style'-'surface'-'distance_km'-'public_details_published')::text,
style,surface,distance_km,public_details_published FROM rides r WHERE id=$1`, ride).Scan(&after, &style, &surface, &distance, &published); err != nil {
		t.Fatal(err)
	}
	if before != after || style != "" || surface != "" || distance != 0 || published {
		t.Fatal("upgrade/replay changed old values or published private details", before, after, style, surface, distance, published)
	}
	if err := store.Ready(ctx); err != nil {
		t.Fatal(err)
	}
}
