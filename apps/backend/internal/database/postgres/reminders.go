package postgres

import (
	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"context"
	"github.com/jackc/pgx/v5"
	"time"
)

type reminderTransaction struct {
	ctx   context.Context
	tx    pgx.Tx
	state models.ReminderContext
}

func loadReminderContext(ctx context.Context, tx pgx.Tx, rideID, actor string) (models.ReminderContext, error) {
	c := models.ReminderContext{UserID: actor}
	var err error
	c.Ride, err = scanRide(tx.QueryRow(ctx, "SELECT "+rideColumns+" FROM rides WHERE id=$1 FOR UPDATE", rideID))
	if err != nil {
		return c, err
	}
	err = tx.QueryRow(ctx, `SELECT u.status,COALESCE(p.state,''),COALESCE(p.media_version,0),o.status,
 COALESCE(rr.day_before,false),COALESCE(rr.hour_before,false),COALESCE(rr.participation_version,0),rr.day_before_enabled_at,rr.hour_before_enabled_at,
 EXISTS(SELECT 1 FROM moderation_hidden h WHERE h.target_type='ride' AND h.target_id=r.id),
 EXISTS(SELECT 1 FROM user_blocks b WHERE (b.blocker_id=u.id AND b.blocked_id=r.organizer_id) OR (b.blocked_id=u.id AND b.blocker_id=r.organizer_id))
 FROM rides r JOIN users o ON o.id=r.organizer_id JOIN users u ON u.id=$2
 LEFT JOIN participation p ON p.ride_id=r.id AND p.user_id=u.id LEFT JOIN ride_reminders rr ON rr.ride_id=r.id AND rr.user_id=u.id
 WHERE r.id=$1 FOR SHARE OF u,o`, rideID, actor).Scan(&c.AccountStatus, &c.ParticipationState, &c.ParticipationVersion, &c.OrganizerStatus,
		&c.Settings.DayBefore, &c.Settings.HourBefore, &c.SettingsParticipationVersion, &c.DayBeforeEnabledAt, &c.HourBeforeEnabledAt, &c.Hidden, &c.OrganizerBlocked)
	return c, repositoryError(err)
}
func (s *Store) WithRideReminders(ctx context.Context, rideID, actor string, decide database.ReminderDecision) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return repositoryError(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", trustAdvisoryLock); err != nil {
		return repositoryError(err)
	}
	state, err := loadReminderContext(ctx, tx, rideID, actor)
	if err != nil {
		return err
	}
	if err = decide(&reminderTransaction{ctx: ctx, tx: tx, state: state}); err != nil {
		return err
	}
	return repositoryError(tx.Commit(ctx))
}
func (t *reminderTransaction) Context() models.ReminderContext { return t.state }
func (t *reminderTransaction) SetReminders(settings models.RideReminders, at time.Time) error {
	_, err := t.tx.Exec(t.ctx, `INSERT INTO ride_reminders(ride_id,user_id,day_before,hour_before,participation_version,updated_at,day_before_enabled_at,hour_before_enabled_at)
 VALUES($1,$2,$3,$4,$5,$6,CASE WHEN $3 THEN $6::timestamptz END,CASE WHEN $4 THEN $6::timestamptz END)
 ON CONFLICT(ride_id,user_id) DO UPDATE SET
 day_before_enabled_at=CASE WHEN NOT EXCLUDED.day_before THEN NULL
  WHEN ride_reminders.day_before AND ride_reminders.participation_version=EXCLUDED.participation_version THEN ride_reminders.day_before_enabled_at
  ELSE EXCLUDED.day_before_enabled_at END,
 hour_before_enabled_at=CASE WHEN NOT EXCLUDED.hour_before THEN NULL
  WHEN ride_reminders.hour_before AND ride_reminders.participation_version=EXCLUDED.participation_version THEN ride_reminders.hour_before_enabled_at
  ELSE EXCLUDED.hour_before_enabled_at END,
 day_before=EXCLUDED.day_before,hour_before=EXCLUDED.hour_before,participation_version=EXCLUDED.participation_version,
 updated_at=CASE WHEN (ride_reminders.day_before,ride_reminders.hour_before,ride_reminders.participation_version)
 IS DISTINCT FROM (EXCLUDED.day_before,EXCLUDED.hour_before,EXCLUDED.participation_version) THEN EXCLUDED.updated_at ELSE ride_reminders.updated_at END`,
		t.state.Ride.ID, t.state.UserID, settings.DayBefore, settings.HourBefore, t.state.ParticipationVersion, at)
	return repositoryError(err)
}
func (s *Store) GenerateDueReminders(ctx context.Context, now, oldest time.Time, limit int, decide database.DueReminderDecision) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return repositoryError(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", trustAdvisoryLock); err != nil {
		return repositoryError(err)
	}
	// Excluding processed occurrences before LIMIT prevents already handled or
	// newly ineligible rows from starving the rest of a bounded worker batch.
	rows, err := tx.Query(ctx, `SELECT r.id::text,rr.user_id::text,leads.minutes FROM ride_reminders rr JOIN rides r ON r.id=rr.ride_id
 CROSS JOIN (VALUES(60),(1440)) leads(minutes)
 WHERE ((leads.minutes=60 AND rr.hour_before) OR (leads.minutes=1440 AND rr.day_before))
 AND r.starts_at-leads.minutes*interval '1 minute' BETWEEN $2 AND $1
 AND NOT EXISTS(SELECT 1 FROM ride_reminder_events e WHERE e.ride_id=r.id AND e.user_id=rr.user_id AND e.starts_at=r.starts_at AND e.lead_minutes=leads.minutes)
 ORDER BY r.starts_at,r.id,rr.user_id,leads.minutes LIMIT $3`, now, oldest, limit)
	if err != nil {
		return repositoryError(err)
	}
	type due struct {
		ride, user string
		minutes    int
	}
	pending := []due{}
	for rows.Next() {
		var d due
		if err = rows.Scan(&d.ride, &d.user, &d.minutes); err != nil {
			rows.Close()
			return repositoryError(err)
		}
		pending = append(pending, d)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return repositoryError(err)
	}
	for _, item := range pending {
		state, err := loadReminderContext(ctx, tx, item.ride, item.user)
		if err != nil {
			return err
		}
		intent, err := decide(models.ReminderCandidate{Context: state, LeadMinutes: item.minutes})
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO ride_reminder_events(ride_id,user_id,starts_at,lead_minutes,processed_at) VALUES($1,$2,$3,$4,$5)`, item.ride, item.user, state.Ride.StartsAt, item.minutes, now); err != nil {
			return repositoryError(err)
		}
		if intent != nil {
			intent.RideID, intent.ClubID = state.Ride.ID, ""
			intent.Recipients = []string{item.user}
			intent.ReminderStartsAt, intent.ReminderLeadMinutes = &state.Ride.StartsAt, item.minutes
			if err = persistNotifications(ctx, tx, *intent); err != nil {
				return err
			}
		}
	}
	return repositoryError(tx.Commit(ctx))
}

var _ database.ReminderRepository = (*Store)(nil)
