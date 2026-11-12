package postgres

import (
	"context"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func removeCommunityRoutes(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `DROP TABLE route_daily_limits,route_conditions,route_collection_items,route_collections,community_routes,route_tracks;
ALTER TABLE moderation_hidden DROP CONSTRAINT moderation_hidden_target_type_check;
ALTER TABLE moderation_hidden ADD CONSTRAINT moderation_hidden_target_type_check CHECK(target_type IN ('user','ride','message','club','bike','bike_photo','club_image','avatar','ride_photo','ride_video','announcement'));
ALTER TABLE trust_reports DROP CONSTRAINT trust_reports_target_type_check;
ALTER TABLE trust_reports ADD CONSTRAINT trust_reports_target_type_check CHECK(target_type IN ('user','ride','message','club','bike','bike_photo','club_image','avatar','ride_photo','ride_video','announcement'));
DELETE FROM schema_migrations WHERE version=12`)
	if err != nil {
		t.Fatal(err)
	}
}

func oldRouteRows(t *testing.T, pool *pgxpool.Pool) map[string]string {
	t.Helper()
	ctx := context.Background()
	rows, err := pool.Query(ctx, `SELECT tablename FROM pg_tables WHERE schemaname=current_schema() AND tablename NOT IN ('community_routes','route_tracks','route_collections','route_collection_items','route_conditions','route_daily_limits') ORDER BY tablename`)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for rows.Next() {
		var n string
		if err = rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		names = append(names, n)
	}
	rows.Close()
	if rows.Err() != nil {
		t.Fatal(rows.Err())
	}
	out := map[string]string{}
	for _, name := range names {
		filter := ""
		if name == "schema_migrations" {
			filter = " WHERE version<=11"
		}
		var value string
		if err = pool.QueryRow(ctx, `SELECT coalesce(jsonb_agg(r ORDER BY r::text),'[]'::jsonb)::text FROM (SELECT to_jsonb(t) r FROM `+pgx.Identifier{name}.Sanitize()+` t`+filter+`) rows`).Scan(&value); err != nil {
			t.Fatal(err)
		}
		out[name] = value
	}
	return out
}

func TestVersionElevenRoutesUpgradePreservesEveryExistingRow(t *testing.T) {
	store, pool := testStore(t)
	ctx := context.Background()
	removeCommunityRoutes(t, pool)
	owner, place := uuid.NewString(), uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,phone,display_name,created_at) VALUES($1,'+77011234921','Existing rider',now())`, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO places(id,name,category,lat,lon,city,address,description,website,phone,opening_hours,source,source_url,status,version,updated_at) VALUES($1,'Existing community place','meeting_spot',43.24,76.89,'Almaty','','','','','','community','','published',3,now())`, place); err != nil {
		t.Fatal(err)
	}
	before := oldRouteRows(t, pool)
	if store.Ready(ctx) == nil {
		t.Fatal("schema eleven accepted by current executable")
	}
	for range 2 {
		if err := store.Migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Ready(ctx); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, oldRouteRows(t, pool)) {
		t.Fatal("route migration/replay changed old rows or receipts")
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM community_routes)+(SELECT count(*) FROM route_tracks)+(SELECT count(*) FROM route_collections)+(SELECT count(*) FROM route_collection_items)+(SELECT count(*) FROM route_conditions)+(SELECT count(*) FROM route_daily_limits)`).Scan(&n); err != nil || n != 0 {
		t.Fatal("migration populated route data", n, err)
	}
}
