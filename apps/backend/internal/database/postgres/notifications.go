package postgres

import (
	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"time"
)

const notificationColumns = "id::text,sequence,kind,COALESCE(ride_id::text,''),COALESCE(club_id::text,''),created_at,read_at"

type notificationTransaction struct {
	ctx     context.Context
	tx      pgx.Tx
	account models.PushAccount
}

func (s *Store) WithNotifications(ctx context.Context, actor string, decide database.NotificationDecision) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return repositoryError(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", trustAdvisoryLock); err != nil {
		return repositoryError(err)
	}
	t := &notificationTransaction{ctx: ctx, tx: tx}
	if err = tx.QueryRow(ctx, "SELECT id::text,status,country_code,currency_code,locale,push_ride_updates,push_participation,push_club_membership,push_ride_reminders FROM users WHERE id=$1 FOR SHARE", actor).Scan(&t.account.ID, &t.account.Status, &t.account.Preferences.CountryCode, &t.account.Preferences.CurrencyCode, &t.account.Preferences.Locale, &t.account.Preferences.PushCategories.RideUpdates, &t.account.Preferences.PushCategories.Participation, &t.account.Preferences.PushCategories.ClubMembership, &t.account.Preferences.PushCategories.RideReminders); err != nil {
		return repositoryError(err)
	}
	if err = decide(t); err != nil {
		return err
	}
	return repositoryError(tx.Commit(ctx))
}
func (t *notificationTransaction) Account() models.PushAccount { return t.account }
func (t *notificationTransaction) SetPreferences(p models.Preferences) error {
	_, err := t.tx.Exec(t.ctx, "UPDATE users SET country_code=$2,currency_code=$3,locale=$4,push_ride_updates=$5,push_participation=$6,push_club_membership=$7,push_ride_reminders=$8 WHERE id=$1", t.account.ID, p.CountryCode, p.CurrencyCode, p.Locale, p.PushCategories.RideUpdates, p.PushCategories.Participation, p.PushCategories.ClubMembership, p.PushCategories.RideReminders)
	if err == nil {
		t.account.Preferences = p
	}
	return repositoryError(err)
}
func scanNotification(row pgx.Row) (models.Notification, error) {
	var n models.Notification
	err := row.Scan(&n.ID, &n.Sequence, &n.Kind, &n.RideID, &n.ClubID, &n.CreatedAt, &n.ReadAt)
	return n, repositoryError(err)
}
func (t *notificationTransaction) Notifications(before int64, limit int) ([]models.Notification, error) {
	rows, err := t.tx.Query(t.ctx, "SELECT "+notificationColumns+" FROM notifications WHERE user_id=$1 AND ($2::bigint=0 OR sequence<$2) ORDER BY sequence DESC LIMIT $3", t.account.ID, before, limit)
	if err != nil {
		return nil, repositoryError(err)
	}
	defer rows.Close()
	out := []models.Notification{}
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, repositoryError(rows.Err())
}
func (t *notificationTransaction) UnreadCount() (int, error) {
	var n int
	err := t.tx.QueryRow(t.ctx, "SELECT count(*) FROM notifications WHERE user_id=$1 AND read_at IS NULL", t.account.ID).Scan(&n)
	return n, repositoryError(err)
}
func (t *notificationTransaction) MarkRead(id string, at time.Time) (models.Notification, error) {
	return scanNotification(t.tx.QueryRow(t.ctx, "UPDATE notifications SET read_at=COALESCE(read_at,$3) WHERE user_id=$1 AND id=$2 RETURNING "+notificationColumns, t.account.ID, id, at))
}
func (t *notificationTransaction) MarkAllRead(at time.Time) error {
	_, err := t.tx.Exec(t.ctx, "UPDATE notifications SET read_at=$2 WHERE user_id=$1 AND read_at IS NULL", t.account.ID, at)
	return repositoryError(err)
}

const subscriptionColumns = "p.id::text,p.user_id::text,p.endpoint,p.p256dh,p.auth,p.session_digest,p.created_at,(s.revoked_at IS NULL AND s.expires_at>now() AND s.last_seen_at>now()-interval '7 days' AND u.status='active')"

func scanSubscription(row pgx.Row) (models.PushSubscriptionRecord, error) {
	var p models.PushSubscriptionRecord
	err := row.Scan(&p.ID, &p.UserID, &p.Endpoint, &p.P256dh, &p.Auth, &p.SessionDigest, &p.CreatedAt, &p.BindingActive)
	return p, repositoryError(err)
}
func (t *notificationTransaction) Subscriptions() ([]models.PushSubscriptionRecord, error) {
	rows, err := t.tx.Query(t.ctx, "SELECT "+subscriptionColumns+" FROM push_subscriptions p JOIN sessions s ON s.token_digest=p.session_digest JOIN users u ON u.id=p.user_id WHERE p.user_id=$1 ORDER BY p.created_at,p.id", t.account.ID)
	if err != nil {
		return nil, repositoryError(err)
	}
	defer rows.Close()
	out := []models.PushSubscriptionRecord{}
	for rows.Next() {
		p, err := scanSubscription(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, repositoryError(rows.Err())
}
func (t *notificationTransaction) SubscriptionByEndpoint(endpoint string) (models.PushSubscriptionRecord, error) {
	return scanSubscription(t.tx.QueryRow(t.ctx, "SELECT "+subscriptionColumns+" FROM push_subscriptions p JOIN sessions s ON s.token_digest=p.session_digest JOIN users u ON u.id=p.user_id WHERE p.endpoint=$1", endpoint))
}
func (t *notificationTransaction) PutSubscription(p models.PushSubscriptionRecord) (models.PushSubscriptionRecord, error) {
	if _, err := t.tx.Exec(t.ctx, "DELETE FROM push_deliveries d USING push_subscriptions p WHERE d.subscription_id=p.id AND p.endpoint=$1 AND p.session_digest<>$2", p.Endpoint, p.SessionDigest); err != nil {
		return p, repositoryError(err)
	}
	result, err := t.tx.Exec(t.ctx, `INSERT INTO push_subscriptions(id,user_id,session_digest,endpoint,p256dh,auth,created_at)
 SELECT $1,$2,$3,$4,$5,$6,$7 FROM sessions WHERE token_digest=$3 AND user_id=$2 AND revoked_at IS NULL AND expires_at>$7 AND last_seen_at>$7-interval '7 days'
 ON CONFLICT(endpoint) DO UPDATE SET user_id=EXCLUDED.user_id,session_digest=EXCLUDED.session_digest,p256dh=EXCLUDED.p256dh,auth=EXCLUDED.auth,created_at=EXCLUDED.created_at`, p.ID, t.account.ID, p.SessionDigest, p.Endpoint, p.P256dh, p.Auth, p.CreatedAt)
	if err != nil {
		return p, repositoryError(err)
	}
	if result.RowsAffected() == 0 {
		return p, database.ErrNotFound
	}
	return t.SubscriptionByEndpoint(p.Endpoint)
}
func (t *notificationTransaction) DeleteSubscription(id string) error {
	_, err := t.tx.Exec(t.ctx, "DELETE FROM push_subscriptions WHERE user_id=$1 AND id=$2", t.account.ID, id)
	return repositoryError(err)
}
func (t *rideTransaction) Notify(intent models.NotificationIntent) error {
	intent.RideID, intent.ClubID = t.ride.ID, ""
	return persistNotifications(t.ctx, t.tx, intent)
}
func (t *clubTransaction) Notify(intent models.NotificationIntent) error {
	intent.ClubID, intent.RideID = t.club.ID, ""
	return persistNotifications(t.ctx, t.tx, intent)
}

// Both membership state and its durable inbox/push work commit in the owning
// transaction. The transaction adapter binds the intent to its locked target.
func persistNotifications(ctx context.Context, tx pgx.Tx, intent models.NotificationIntent) error {
	for _, recipient := range intent.Recipients {
		id := uuid.NewString()
		var stored string
		err := tx.QueryRow(ctx, `INSERT INTO notifications(id,user_id,ride_id,club_id,kind,event_id,created_at,reminder_starts_at,reminder_lead_minutes) VALUES($1,$2,NULLIF($3,'')::uuid,NULLIF($4,'')::uuid,$5,$6,$7,$8,NULLIF($9,0)) ON CONFLICT(user_id,event_id) DO NOTHING RETURNING id::text`, id, recipient, intent.RideID, intent.ClubID, intent.Kind, intent.EventID, intent.CreatedAt, intent.ReminderStartsAt, intent.ReminderLeadMinutes).Scan(&stored)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return repositoryError(err)
		}
		rows, err := tx.Query(ctx, "SELECT id::text FROM push_subscriptions WHERE user_id=$1", recipient)
		if err != nil {
			return repositoryError(err)
		}
		subs := []string{}
		for rows.Next() {
			var sub string
			if err = rows.Scan(&sub); err != nil {
				rows.Close()
				return repositoryError(err)
			}
			subs = append(subs, sub)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return repositoryError(err)
		}
		for _, sub := range subs {
			if _, err = tx.Exec(ctx, "INSERT INTO push_deliveries(id,notification_id,subscription_id,next_attempt) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING", uuid.NewString(), stored, sub, intent.CreatedAt); err != nil {
				return repositoryError(err)
			}
		}
	}
	return nil
}
func (s *Store) ClaimPush(ctx context.Context, now, until time.Time, decide database.PushClaimDecision) (*models.PushDelivery, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, repositoryError(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", trustAdvisoryLock); err != nil {
		return nil, repositoryError(err)
	}
	for i := 0; i < 25; i++ {
		var d models.PushDelivery
		err = tx.QueryRow(ctx, `SELECT d.id::text,d.notification_id::text,n.kind,COALESCE(n.ride_id::text,''),COALESCE(n.club_id::text,''),COALESCE(c.status,''),COALESCE(c.owner_id::text,''),COALESCE(cm.state,''),u.locale,d.attempts,p.id::text,p.user_id::text,p.endpoint,p.p256dh,p.auth,p.session_digest,p.created_at,(u.status='active' AND n.user_id=p.user_id AND s.user_id=p.user_id),(s.revoked_at IS NULL AND s.expires_at>$1 AND s.last_seen_at>$1-interval '7 days'),COALESCE(pa.state='approved',false),u.push_ride_updates,u.push_participation,u.push_club_membership,u.push_ride_reminders,n.reminder_starts_at,COALESCE(n.reminder_lead_minutes,0)
 FROM push_deliveries d JOIN notifications n ON n.id=d.notification_id JOIN push_subscriptions p ON p.id=d.subscription_id JOIN users u ON u.id=p.user_id JOIN sessions s ON s.token_digest=p.session_digest LEFT JOIN participation pa ON pa.ride_id=n.ride_id AND pa.user_id=n.user_id LEFT JOIN clubs c ON c.id=n.club_id LEFT JOIN club_memberships cm ON cm.club_id=n.club_id AND cm.user_id=n.user_id
 WHERE ((d.state='pending' AND d.next_attempt<=$1) OR (d.state='sending' AND d.lease_until<=$1)) ORDER BY d.next_attempt,d.id FOR UPDATE OF d SKIP LOCKED LIMIT 1`, now).Scan(&d.ID, &d.NotificationID, &d.Kind, &d.RideID, &d.ClubID, &d.ClubStatus, &d.ClubOwnerID, &d.ClubMembershipState, &d.Locale, &d.Attempts, &d.Subscription.ID, &d.Subscription.UserID, &d.Subscription.Endpoint, &d.Subscription.P256dh, &d.Subscription.Auth, &d.Subscription.SessionDigest, &d.Subscription.CreatedAt, &d.AccountActive, &d.SessionActive, &d.RideAllowed, &d.PushCategories.RideUpdates, &d.PushCategories.Participation, &d.PushCategories.ClubMembership, &d.PushCategories.RideReminders, &d.ReminderStartsAt, &d.ReminderLeadMinutes)
		if errors.Is(err, pgx.ErrNoRows) {
			if err = tx.Commit(ctx); err != nil {
				return nil, repositoryError(err)
			}
			return nil, nil
		}
		if err != nil {
			return nil, repositoryError(err)
		}
		if d.Kind == "ride.reminder" {
			d.ReminderContext, err = loadReminderContext(ctx, tx, d.RideID, d.Subscription.UserID)
			if err != nil {
				return nil, err
			}
		}
		if d.Attempts >= 5 || !decide(d) {
			if _, err = tx.Exec(ctx, "UPDATE push_deliveries SET state='skipped',lease_id=NULL,lease_until=NULL WHERE id=$1", d.ID); err != nil {
				return nil, repositoryError(err)
			}
			continue
		}
		d.LeaseID = uuid.NewString()
		d.Attempts++
		if _, err = tx.Exec(ctx, "UPDATE push_deliveries SET state='sending',attempts=$2,lease_id=$3,lease_until=$4 WHERE id=$1", d.ID, d.Attempts, d.LeaseID, until); err != nil {
			return nil, repositoryError(err)
		}
		if err = tx.Commit(ctx); err != nil {
			return nil, repositoryError(err)
		}
		return &d, nil
	}
	return nil, repositoryError(tx.Commit(ctx))
}
func (s *Store) FinishPush(ctx context.Context, id, lease string, result models.PushDeliveryResult) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return repositoryError(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", trustAdvisoryLock); err != nil {
		return repositoryError(err)
	}
	var sub string
	err = tx.QueryRow(ctx, "UPDATE push_deliveries SET state=$3,next_attempt=$4,lease_id=NULL,lease_until=NULL WHERE id=$1 AND lease_id=$2 AND state='sending' RETURNING subscription_id::text", id, lease, result.State, result.NextAttempt).Scan(&sub)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return repositoryError(err)
	}
	if result.RemoveSubscription {
		if _, err = tx.Exec(ctx, "DELETE FROM push_subscriptions WHERE id=$1", sub); err != nil {
			return repositoryError(err)
		}
	}
	return repositoryError(tx.Commit(ctx))
}

var _ database.NotificationRepository = (*Store)(nil)
