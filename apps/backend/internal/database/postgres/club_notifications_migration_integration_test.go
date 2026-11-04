package postgres

import (
	"context"
	"crypto/sha256"
	"testing"

	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
)

func TestVersionSevenClubNotificationsUpgradePreservesInboxAndPushState(t *testing.T) {
	store, pool := testStore(t)
	ctx := context.Background()
	// Reconstruct schema seven only in this isolated test namespace. The old SQL
	// and receipts remain immutable; the actual upgrade is migration eight.
	if _, err := pool.Exec(ctx, `ALTER TABLE notifications DROP CONSTRAINT notifications_target_scope;
ALTER TABLE notifications DROP COLUMN club_id;
ALTER TABLE notifications ALTER COLUMN ride_id SET NOT NULL;
ALTER TABLE notifications DROP CONSTRAINT notifications_kind_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_kind_check CHECK(kind IN ('ride.updated','ride.cancelled','ride.announcement','participation.requested','participation.approved','participation.rejected','participation.removed'));
DELETE FROM schema_migrations WHERE version=8`); err != nil {
		t.Fatal(err)
	}
	owner, ride, club := uuid.NewString(), uuid.NewString(), uuid.NewString()
	readNotice, unreadNotice, subscription, delivery, lease := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	digest := sha256.Sum256([]byte("isolated existing club notifications session"))
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,phone,display_name,created_at) VALUES($1,'+77011234567','Existing rider',now())`, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO sessions(token_digest,user_id,csrf_token,created_at,last_seen_at,expires_at) VALUES($1,$2,'existing-csrf',now(),now(),now()+interval '1 day')`, digest[:], owner); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO rides(id,organizer_id,city,description,title,starts_at,timezone,meeting_point,capacity,created_at) VALUES($1,$2,'Almaty','Existing preview','Existing private plan',now()+interval '1 day','Asia/Almaty','Existing private meeting',5,now())`, ride, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO clubs(id,owner_id,name,city,description,status,version,created_at,updated_at) VALUES($1,$2,'Existing club','Almaty','Club description','active',4,now(),now())`, club, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO notifications(id,user_id,ride_id,kind,event_id,created_at,read_at) VALUES($1,$2,$3,'ride.updated',$4,now()-interval '1 hour',now()-interval '30 minutes'),($5,$2,$3,'ride.announcement',$6,now(),NULL)`, readNotice, owner, ride, uuid.NewString(), unreadNotice, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO push_subscriptions(id,user_id,session_digest,endpoint,p256dh,auth,created_at) VALUES($1,$2,$3,'https://web.push.apple.com/existing','existing-p256dh','existing-auth',now())`, subscription, owner, digest[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO push_deliveries(id,notification_id,subscription_id,state,attempts,next_attempt,lease_id,lease_until) VALUES($1,$2,$3,'sending',2,now(),$4,now()+interval '2 minutes')`, delivery, unreadNotice, subscription, lease); err != nil {
		t.Fatal(err)
	}
	const originalState = `SELECT jsonb_build_object(
 'notices',(SELECT jsonb_agg(to_jsonb(n)-'club_id' ORDER BY sequence) FROM notifications n),
 'subscriptions',(SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM push_subscriptions p),
 'deliveries',(SELECT jsonb_agg(to_jsonb(d) ORDER BY id) FROM push_deliveries d),
 'sessions',(SELECT jsonb_agg(to_jsonb(s) ORDER BY created_at) FROM sessions s),
 'clubs',(SELECT jsonb_agg(to_jsonb(c) ORDER BY id) FROM clubs c),
 'receipts',(SELECT jsonb_agg(to_jsonb(m) ORDER BY version) FROM schema_migrations m WHERE version<=7))::text`
	var before, after string
	if err := pool.QueryRow(ctx, originalState).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if err := store.Ready(ctx); err == nil {
		t.Fatal("schema seven incorrectly ready before club notifications upgrade")
	}
	for range 2 {
		if err := store.Migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Ready(ctx); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, originalState).Scan(&after); err != nil || before != after {
		t.Fatal("upgrade/replay changed existing inbox, read state, sessions, push work, club or migration receipts", err)
	}
	if err := store.WithNotifications(ctx, owner, func(tx database.NotificationTransaction) error {
		notices, err := tx.Notifications(0, 50)
		if err != nil || len(notices) != 2 {
			t.Fatal("existing ride inbox is unreadable after upgrade", err)
		}
		for _, n := range notices {
			if n.RideID != ride || n.ClubID != "" {
				t.Fatal("upgrade changed ride notification target")
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"club.membership.requested", "club.membership.approved", "club.membership.rejected", "club.membership.removed"} {
		if _, err := pool.Exec(ctx, `INSERT INTO notifications(id,user_id,club_id,kind,event_id,created_at) VALUES($1,$2,$3,$4,$5,now())`, uuid.NewString(), owner, club, kind, uuid.NewString()); err != nil {
			t.Fatal("valid club notification rejected", kind, err)
		}
	}
	for _, target := range []struct{ rideID, clubID, kind string }{
		{"", "", "club.membership.requested"},
		{ride, club, "club.membership.approved"},
		{ride, "", "club.membership.rejected"},
		{"", club, "ride.updated"},
		{"", club, "club.unknown"},
		{"", uuid.NewString(), "club.membership.removed"},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO notifications(id,user_id,ride_id,club_id,kind,event_id,created_at) VALUES($1,$2,NULLIF($3,'')::uuid,NULLIF($4,'')::uuid,$5,$6,now())`, uuid.NewString(), owner, target.rideID, target.clubID, target.kind, uuid.NewString()); err == nil {
			t.Fatal("invalid notification target accepted", target)
		}
	}
	// A caller cannot redirect a club transaction's notification to another club.
	intent := models.NotificationIntent{EventID: uuid.NewString(), Kind: "club.membership.requested", ClubID: uuid.NewString(), RideID: ride, Recipients: []string{owner}}
	if err := store.WithClub(ctx, club, func(tx database.ClubTransaction) error { return tx.Notify(intent) }); err != nil {
		t.Fatal(err)
	}
	var targetClub string
	var targetRide *string
	if err := pool.QueryRow(ctx, `SELECT club_id::text,ride_id::text FROM notifications WHERE event_id=$1`, intent.EventID).Scan(&targetClub, &targetRide); err != nil || targetClub != club || targetRide != nil {
		t.Fatal("notification escaped its locked club target", err)
	}
}
