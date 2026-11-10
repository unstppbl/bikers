package postgres

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"bikers.kz/backend/internal/database"
	"github.com/google/uuid"
)

func TestPushSubscriptionBindingUsesOperationClockAndStrictSessionBoundaries(t *testing.T) {
	store, pool := testStore(t)
	ctx := context.Background()
	owner, subscription := uuid.NewString(), uuid.NewString()
	digest := sha256.Sum256([]byte("isolated binding operation clock"))
	old := time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC)
	future := time.Date(2100, 1, 1, 12, 0, 0, 0, time.UTC)
	endpoint := "https://web.push.apple.com/isolated-binding-clock"
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,phone,display_name,created_at) VALUES($1,'+77011234881','Clock fixture', $2)`, owner, old); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO sessions(token_digest,user_id,csrf_token,created_at,last_seen_at,expires_at) VALUES($1,$2,'clock-csrf',$3,$3,$4)`, digest[:], owner, old, old.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO push_subscriptions(id,user_id,session_digest,endpoint,p256dh,auth,created_at) VALUES($1,$2,$3,$4,'public-key','browser-auth',$5)`, subscription, owner, digest[:], endpoint, old); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name          string
		at            time.Time
		expires       time.Duration
		seen          time.Duration
		revoked       bool
		accountStatus string
		active        bool
	}{
		{name: "active under a historical application clock", at: old, expires: time.Hour, active: true},
		{name: "expiry just ahead", at: future, expires: time.Microsecond, active: true},
		{name: "expiry exactly now", at: future},
		{name: "expiry just passed", at: future, expires: -time.Microsecond},
		{name: "idle just inside seven days", at: future, expires: time.Hour, seen: -7*24*time.Hour + time.Microsecond, active: true},
		{name: "idle exactly seven days", at: future, expires: time.Hour, seen: -7 * 24 * time.Hour},
		{name: "idle just beyond seven days", at: future, expires: time.Hour, seen: -7*24*time.Hour - time.Microsecond},
		{name: "revoked binding", at: future, expires: time.Hour, revoked: true},
		{name: "inactive account", at: future, expires: time.Hour, accountStatus: "suspended"},
	} {
		t.Run(test.name, func(t *testing.T) {
			status := test.accountStatus
			if status == "" {
				status = "active"
			}
			var revoked *time.Time
			if test.revoked {
				value := test.at.Add(-time.Minute)
				revoked = &value
			}
			if _, err := pool.Exec(ctx, `UPDATE users SET status=$2 WHERE id=$1`, owner, status); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `UPDATE sessions SET expires_at=$2,last_seen_at=$3,revoked_at=$4 WHERE token_digest=$1`, digest[:], test.at.Add(test.expires), test.at.Add(test.seen), revoked); err != nil {
				t.Fatal(err)
			}
			if err := store.WithNotifications(ctx, owner, func(tx database.NotificationTransaction) error {
				byEndpoint, err := tx.SubscriptionByEndpoint(endpoint, test.at)
				if err != nil {
					return err
				}
				listed, err := tx.Subscriptions(test.at)
				if err != nil {
					return err
				}
				if byEndpoint.BindingActive != test.active || len(listed) != 1 || listed[0].BindingActive != test.active {
					t.Fatalf("binding at %s: endpoint=%v list=%+v want active=%v", test.at, byEndpoint.BindingActive, listed, test.active)
				}
				if test.at.Equal(old) {
					stored, err := tx.PutSubscription(byEndpoint)
					if err != nil {
						return err
					}
					if !stored.BindingActive {
						t.Fatal("subscription write/readback used a different operation clock")
					}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
