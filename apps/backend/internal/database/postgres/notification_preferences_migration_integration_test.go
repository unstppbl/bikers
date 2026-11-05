package postgres

import (
	"context"
	"crypto/sha256"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Reconstruct schema eight only in disposable test namespaces. Applied source
// migrations are immutable; no reverse migration is available in production.
func removeNotificationPreferencesAndReminders(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	removeRideMaps(t, pool)
	_, err := pool.Exec(context.Background(), `DROP TABLE ride_reminder_events,ride_reminders;
ALTER TABLE users DROP COLUMN push_ride_updates,DROP COLUMN push_participation,DROP COLUMN push_club_membership,DROP COLUMN push_ride_reminders;
ALTER TABLE notifications DROP CONSTRAINT notifications_reminder_scope;
ALTER TABLE notifications DROP COLUMN reminder_starts_at,DROP COLUMN reminder_lead_minutes;
ALTER TABLE notifications DROP CONSTRAINT notifications_kind_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_kind_check CHECK(kind IN ('ride.updated','ride.cancelled','ride.announcement','participation.requested','participation.approved','participation.rejected','participation.removed','club.membership.requested','club.membership.approved','club.membership.rejected','club.membership.removed'));
ALTER TABLE notifications DROP CONSTRAINT notifications_target_scope;
ALTER TABLE notifications ADD CONSTRAINT notifications_target_scope CHECK(
 (ride_id IS NOT NULL AND club_id IS NULL AND kind IN ('ride.updated','ride.cancelled','ride.announcement','participation.requested','participation.approved','participation.rejected','participation.removed'))
 OR (club_id IS NOT NULL AND ride_id IS NULL AND kind IN ('club.membership.requested','club.membership.approved','club.membership.rejected','club.membership.removed')));
DELETE FROM schema_migrations WHERE version=9`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestVersionEightPreferencesAndReminderUpgradeRetainsInboxAccessAndPush(t *testing.T) {
	store, pool := testStore(t)
	removeNotificationPreferencesAndReminders(t, pool)
	ctx := context.Background()
	owner, ride, club, notice, subscription, delivery := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	digest := sha256.Sum256([]byte("disposable existing schema-eight session"))
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,phone,display_name,locale,country_code,currency_code,created_at) VALUES($1,'+77011234567','Existing rider','kk','KG','EUR',now());`, owner); err != nil {
		t.Fatal(err)
	}
	for _, setup := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO sessions(token_digest,user_id,csrf_token,created_at,last_seen_at,expires_at) VALUES($1,$2,'existing-csrf',now(),now(),now()+interval '1 day')`, []any{digest[:], owner}},
		{`INSERT INTO trust_admins(user_id,granted_at) VALUES($1,now())`, []any{owner}},
		{`INSERT INTO rides(id,organizer_id,city,description,title,starts_at,timezone,meeting_point,capacity,created_at) VALUES($1,$2,'Almaty','Preview','Private plan',now()+interval '2 days','Asia/Almaty','Private meeting',5,now())`, []any{ride, owner}},
		{`INSERT INTO participation(ride_id,user_id,state,updated_at) VALUES($1,$2,'approved',now())`, []any{ride, owner}},
		{`INSERT INTO clubs(id,owner_id,name,city,description,created_at,updated_at) VALUES($1,$2,'Existing club','Almaty','Public description',now(),now())`, []any{club, owner}},
		{`INSERT INTO notifications(id,user_id,club_id,kind,event_id,created_at,read_at) VALUES($1,$2,$3,'club.membership.requested',$1,now(),now())`, []any{notice, owner, club}},
		{`INSERT INTO push_subscriptions(id,user_id,session_digest,endpoint,p256dh,auth,created_at) VALUES($1,$2,$3,'https://web.push.apple.com/existing','test-key','test-auth',now())`, []any{subscription, owner, digest[:]}},
		{`INSERT INTO push_deliveries(id,notification_id,subscription_id,state,attempts,next_attempt,lease_id,lease_until) VALUES($1,$2,$3,'sending',2,now(),$1,now()+interval '2 minutes')`, []any{delivery, notice, subscription}},
	} {
		if _, err := pool.Exec(ctx, setup.sql, setup.args...); err != nil {
			t.Fatal(err)
		}
	}
	const retained = `SELECT jsonb_build_object(
 'users',(SELECT jsonb_agg(to_jsonb(u)-'push_ride_updates'-'push_participation'-'push_club_membership'-'push_ride_reminders' ORDER BY id) FROM users u),
 'sessions',(SELECT jsonb_agg(to_jsonb(s) ORDER BY token_digest) FROM sessions s),
 'admins',(SELECT jsonb_agg(to_jsonb(a) ORDER BY user_id) FROM trust_admins a),
 'rides',(SELECT jsonb_agg(to_jsonb(r)-'map_plan' ORDER BY id) FROM rides r),
 'membership',(SELECT jsonb_agg(to_jsonb(p) ORDER BY ride_id,user_id) FROM participation p),
 'clubs',(SELECT jsonb_agg(to_jsonb(c) ORDER BY id) FROM clubs c),
 'notices',(SELECT jsonb_agg(to_jsonb(n)-'reminder_starts_at'-'reminder_lead_minutes' ORDER BY sequence) FROM notifications n),
 'subscriptions',(SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM push_subscriptions p),
 'deliveries',(SELECT jsonb_agg(to_jsonb(d) ORDER BY id) FROM push_deliveries d),
 'receipts',(SELECT jsonb_agg(to_jsonb(m) ORDER BY version) FROM schema_migrations m WHERE version<=8))::text`
	var before, after string
	if err := pool.QueryRow(ctx, retained).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if err := store.Ready(ctx); err == nil {
		t.Fatal("schema eight ready before required migration")
	}
	for range 2 {
		if err := store.Migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Ready(ctx); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, retained).Scan(&after); err != nil || before != after {
		t.Fatal("schema-nine upgrade changed prior inbox, access, lease or receipt state", err)
	}
	var enabled bool
	var settings, events int
	if err := pool.QueryRow(ctx, `SELECT push_ride_updates AND push_participation AND push_club_membership AND push_ride_reminders,(SELECT count(*) FROM ride_reminders),(SELECT count(*) FROM ride_reminder_events) FROM users WHERE id=$1`, owner).Scan(&enabled, &settings, &events); err != nil || !enabled || settings != 0 || events != 0 {
		t.Fatal("upgrade changed default push behavior or opted riders into reminders", err)
	}
}
