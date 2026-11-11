package postgres

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"reflect"
	"testing"
)

func removePlaces(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	removeCommunityRoutes(t, pool)
	if _, err := pool.Exec(context.Background(), `DROP TABLE place_submissions,places; DELETE FROM schema_migrations WHERE version=11`); err != nil {
		t.Fatal(err)
	}
}

func oldPlaceMigrationRows(t *testing.T, pool *pgxpool.Pool) map[string]string {
	t.Helper()
	ctx := context.Background()
	rows, err := pool.Query(ctx, `SELECT tablename FROM pg_tables WHERE schemaname=current_schema() AND tablename NOT IN ('places','place_submissions','community_routes','route_tracks','route_collections','route_collection_items','route_conditions','route_daily_limits') ORDER BY tablename`)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	rows.Close()
	if rows.Err() != nil {
		t.Fatal(rows.Err())
	}
	out := map[string]string{}
	for _, name := range names {
		table := pgx.Identifier{name}.Sanitize()
		filter := ""
		if name == "schema_migrations" {
			filter = " WHERE version<=10"
		}
		var snapshot string
		if err = pool.QueryRow(ctx, `SELECT coalesce(jsonb_agg(r ORDER BY r::text),'[]'::jsonb)::text FROM (SELECT to_jsonb(t) r FROM `+table+` t`+filter+`) rows`).Scan(&snapshot); err != nil {
			t.Fatal(err)
		}
		out[name] = snapshot
	}
	return out
}

func TestVersionTenPlacesUpgradePreservesAllOldRowsAndSavedMap(t *testing.T) {
	store, pool := testStore(t)
	removePlaces(t, pool)
	ctx := context.Background()
	owner, ride := uuid.NewString(), uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,phone,display_name,created_at) VALUES($1,'+77011234901','Preserved rider',now())`, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO trust_admins(user_id,granted_at) VALUES($1,now())`, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO sessions(token_digest,user_id,csrf_token,created_at,last_seen_at,expires_at) VALUES(decode('aa','hex'),$1,'preserved-csrf',now(),now(),now()+interval '1 day')`, owner); err != nil {
		t.Fatal(err)
	}
	const plan = `{"points":[{"label":"Private A","lat":43.24,"lon":76.89},{"label":"Private B","lat":43.25,"lon":76.91}],"geometry":{"type":"LineString","coordinates":[[76.89,43.24],[76.91,43.25]]},"distance_meters":1200,"duration_seconds":180}`
	if _, err := pool.Exec(ctx, `INSERT INTO rides(id,organizer_id,city,description,title,starts_at,timezone,meeting_point,destination,capacity,plan_version,created_at,map_plan) VALUES($1,$2,'Almaty','Old public preview','Old private title',now()+interval '1 day','Asia/Almaty','Private A','Private B',5,7,now(),$3::jsonb)`, ride, owner, plan); err != nil {
		t.Fatal(err)
	}
	before := oldPlaceMigrationRows(t, pool)
	if store.Ready(ctx) == nil {
		t.Fatal("schema ten accepted before places migration")
	}
	for range 2 {
		if err := store.Migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Ready(ctx); err != nil {
		t.Fatal(err)
	}
	after := oldPlaceMigrationRows(t, pool)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("places upgrade/replay changed old rows, map plan, session or receipts")
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM places)+(SELECT count(*) FROM place_submissions)`).Scan(&count); err != nil || count != 0 {
		t.Fatal("additive migration populated places", count, err)
	}
	r, err := store.Ride(ctx, ride)
	if err != nil || r.MapPlan == nil || r.MapPlan.Points[0].Label != "Private A" || r.PlanVersion != 7 {
		t.Fatal("saved private map no longer readable", err)
	}
}
