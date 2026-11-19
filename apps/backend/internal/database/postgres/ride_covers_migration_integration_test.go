package postgres

import (
	"context"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// removeRideCovers reconstructs schema thirteen only inside disposable test
// schemas. Applied migrations are immutable; production has no reverse path.
func removeRideCovers(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `ALTER TABLE rides DROP CONSTRAINT rides_current_cover_scope;
DROP TABLE ride_covers;
ALTER TABLE rides DROP COLUMN current_cover_id,DROP COLUMN cover_version;
ALTER TABLE moderation_hidden DROP CONSTRAINT moderation_hidden_target_type_check;
ALTER TABLE moderation_hidden ADD CONSTRAINT moderation_hidden_target_type_check CHECK(target_type IN ('user','ride','message','club','bike','bike_photo','club_image','avatar','ride_photo','ride_video','announcement','route','route_condition'));
ALTER TABLE trust_reports DROP CONSTRAINT trust_reports_target_type_check;
ALTER TABLE trust_reports ADD CONSTRAINT trust_reports_target_type_check CHECK(target_type IN ('user','ride','message','club','bike','bike_photo','club_image','avatar','ride_photo','ride_video','announcement','route','route_condition'));
DELETE FROM schema_migrations WHERE version=14`); err != nil {
		t.Fatal(err)
	}
}

// preRideCoverRows snapshots every schema-13 row. Migration 14 adds only the
// ride_covers table, two ride columns and receipt 14, which are excluded.
func preRideCoverRows(t *testing.T, pool *pgxpool.Pool) map[string]string {
	t.Helper()
	ctx := context.Background()
	rows, err := pool.Query(ctx, `SELECT tablename FROM pg_tables WHERE schemaname=current_schema() AND tablename<>'ride_covers' ORDER BY tablename`)
	if err != nil {
		t.Fatal(err)
	}
	names, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, name := range names {
		filter := ""
		if name == "schema_migrations" {
			filter = " WHERE version<=13"
		}
		var value string
		if err = pool.QueryRow(ctx, `SELECT coalesce(jsonb_agg(r ORDER BY r::text),'[]'::jsonb)::text FROM (SELECT to_jsonb(t)-'cover_version'-'current_cover_id' r FROM `+pgx.Identifier{name}.Sanitize()+` t`+filter+`) rows`).Scan(&value); err != nil {
			t.Fatal(err)
		}
		out[name] = value
	}
	return out
}

func TestVersionThirteenRideCoverUpgradePreservesRidesAndModeration(t *testing.T) {
	store, pool := testStore(t)
	ctx := context.Background()
	removeRideCovers(t, pool)
	owner, ride, other := uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, setup := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO users(id,phone,display_name,created_at) VALUES($1,'+77011234941','Existing organizer',now())`, []any{owner}},
		{`INSERT INTO rides(id,organizer_id,city,description,title,starts_at,timezone,meeting_point,destination,capacity,plan_version,created_at,map_plan) VALUES($1,$2,'Almaty','Old public preview','Old private title',now()+interval '1 day','Asia/Almaty','Private A','Private B',5,7,now(),'{"points":[{"label":"Private A","lat":43.24,"lon":76.89}],"distance_meters":0,"duration_seconds":0}'::jsonb)`, []any{ride, owner}},
		{`INSERT INTO rides(id,organizer_id,city,description,title,starts_at,timezone,meeting_point,capacity,created_at) VALUES($1,$2,'Astana','Second ride','Plan',now()+interval '2 days','Asia/Almaty','Meeting',3,now())`, []any{other, owner}},
		{`INSERT INTO participation(ride_id,user_id,state,updated_at) VALUES($1,$2,'approved',now())`, []any{ride, owner}},
		{`INSERT INTO moderation_hidden(target_type,target_id,hidden_at) VALUES('ride',$1,now())`, []any{other}},
		{`INSERT INTO trust_reports(id,reporter_id,target_type,target_id,reason,details,status,version,snapshot,created_at,updated_at) VALUES($1,$2,'ride',$3,'spam','',$4,1,'{}'::jsonb,now(),now())`, []any{uuid.NewString(), owner, other, "open"}},
	} {
		if _, err := pool.Exec(ctx, setup.sql, setup.args...); err != nil {
			t.Fatal(err)
		}
	}
	before := preRideCoverRows(t, pool)
	if store.Ready(ctx) == nil {
		t.Fatal("schema thirteen accepted before the ride cover migration")
	}
	for range 2 {
		if err := store.Migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Ready(ctx); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, preRideCoverRows(t, pool)) {
		t.Fatal("ride cover migration/replay changed existing rows, moderation or receipts")
	}
	var covers, changed int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM ride_covers),(SELECT count(*) FROM rides WHERE cover_version<>1 OR current_cover_id IS NOT NULL)`).Scan(&covers, &changed); err != nil || covers != 0 || changed != 0 {
		t.Fatal("migration assigned covers to existing rides", covers, changed, err)
	}
	stored, err := store.Ride(ctx, ride)
	if err != nil || stored.Cover != nil || stored.CoverVersion != 1 || stored.MapPlan == nil || stored.PlanVersion != 7 {
		t.Fatalf("existing ride did not round trip: %+v %v", stored, err)
	}
	cover := uuid.NewString()
	if _, err = pool.Exec(ctx, `INSERT INTO moderation_hidden(target_type,target_id,hidden_at) VALUES('ride_cover',$1,now())`, cover); err != nil {
		t.Fatal("schema fourteen rejected a moderated ride cover", err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO trust_reports(id,reporter_id,target_type,target_id,reason,details,status,version,snapshot,created_at,updated_at) VALUES($1,$2,'ride_cover',$3,'spam','','open',1,'{}'::jsonb,now(),now())`, uuid.NewString(), owner, cover); err != nil {
		t.Fatal("schema fourteen rejected a ride cover report", err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO moderation_hidden(target_type,target_id,hidden_at) VALUES('unknown',$1,now())`, cover); err == nil {
		t.Fatal("expanded constraint accepted an unknown target type")
	}
	const pending = `INSERT INTO ride_covers(id,ride_id,owner_id,state,reserved_bytes,cover_version,created_at,expires_at) VALUES($1,$2,$3,'pending',$4,1,now(),now()+interval '10 minutes')`
	if _, err = pool.Exec(ctx, pending, uuid.NewString(), ride, owner, 1024); err == nil {
		t.Fatal("pending cover accepted a reservation other than 4 MiB")
	}
	if _, err = pool.Exec(ctx, pending, uuid.NewString(), ride, owner, 4194304); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, pending, uuid.NewString(), ride, owner, 4194304); err == nil {
		t.Fatal("second pending cover accepted for one ride")
	}
	const ready = `INSERT INTO ride_covers(id,ride_id,owner_id,state,reserved_bytes,content,width,height,size_bytes,cover_version,created_at,ready_at) VALUES($1,$2,$3,'ready',3,'\x010203'::bytea,$4,$5,3,1,now(),now())`
	for _, size := range [][2]int{{1921, 10}, {10, 0}} {
		if _, err = pool.Exec(ctx, ready, uuid.NewString(), ride, owner, size[0], size[1]); err == nil {
			t.Fatalf("ready cover accepted dimensions %v", size)
		}
	}
	first := uuid.NewString()
	if _, err = pool.Exec(ctx, ready, first, ride, owner, 1920, 1080); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, ready, uuid.NewString(), ride, owner, 640, 480); err == nil {
		t.Fatal("second ready cover accepted for one ride")
	}
	if _, err = pool.Exec(ctx, `UPDATE rides SET current_cover_id=$1 WHERE id=$2`, first, other); err == nil {
		t.Fatal("ride referenced another ride's cover")
	}
	if _, err = pool.Exec(ctx, `UPDATE rides SET current_cover_id=$1,cover_version=2 WHERE id=$2`, first, ride); err != nil {
		t.Fatal(err)
	}
	stored, err = store.Ride(ctx, ride)
	if err != nil || stored.Cover == nil || stored.Cover.ID != first || stored.Cover.URL != "/api/v1/rides/"+ride+"/cover/"+first+"/content" || stored.CoverVersion != 2 {
		t.Fatalf("current cover did not map into the ride summary: %+v %v", stored.Cover, err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO moderation_hidden(target_type,target_id,hidden_at) VALUES('ride_cover',$1,now())`, first); err != nil {
		t.Fatal(err)
	}
	if stored, err = store.Ride(ctx, ride); err != nil || stored.Cover != nil {
		t.Fatal("moderated cover remained in the ride summary", err)
	}
	if _, err = pool.Exec(ctx, `UPDATE rides SET cover_version=0 WHERE id=$1`, ride); err == nil {
		t.Fatal("ride accepted a non-positive cover version")
	}
}
