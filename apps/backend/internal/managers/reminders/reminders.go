// Package reminders owns explicit ride-reminder consent and departure-time decisions.
package reminders

import (
	"context"
	"errors"
	"strconv"
	"time"

	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
)

const GracePeriod = 10 * time.Minute

type Manager struct {
	repo database.ReminderRepository
	now  func() time.Time
}

func New(repo database.ReminderRepository, now func() time.Time) *Manager {
	if now == nil {
		now = time.Now
	}
	return &Manager{repo: repo, now: now}
}
func Eligible(c models.ReminderContext) bool {
	return c.AccountStatus == "active" && c.OrganizerStatus == "active" && c.Ride.Status == "published" &&
		(c.UserID == c.Ride.OrganizerID || c.ParticipationState == "approved") &&
		c.ParticipationVersion == c.SettingsParticipationVersion && !c.Hidden && !c.OrganizerBlocked
}
func Selected(settings models.RideReminders, minutes int) bool {
	return (minutes == 1440 && settings.DayBefore) || (minutes == 60 && settings.HourBefore)
}

// Due is shared with push dispatch, so queued work loses eligibility after a
// changed departure, revoked participation, opt-out or the bounded grace window.
func Due(c models.ReminderContext, minutes int, now time.Time) bool {
	due := c.Ride.StartsAt.Add(-time.Duration(minutes) * time.Minute)
	enabledAt := c.HourBeforeEnabledAt
	if minutes == 1440 {
		enabledAt = c.DayBeforeEnabledAt
	}
	return Eligible(c) && Selected(c.Settings, minutes) && c.Ride.StartsAt.After(now) &&
		!due.After(now) && !due.Before(now.Add(-GracePeriod)) && enabledAt != nil && !enabledAt.After(due)
}
func access(c models.ReminderContext) error {
	if c.AccountStatus != "active" {
		return models.Fail("account_unavailable", "Account unavailable")
	}
	if c.UserID != c.Ride.OrganizerID && c.ParticipationState != "approved" {
		return models.Fail("participant_required", "Approved participation required")
	}
	return nil
}
func result(err error) error {
	if errors.Is(err, database.ErrNotFound) {
		return models.Fail("not_found", "Ride not found")
	}
	return err
}
func (m *Manager) Get(ctx context.Context, actor models.User, rideID string) (models.RideReminders, error) {
	var out models.RideReminders
	id, err := uuid.Parse(rideID)
	if err != nil {
		return out, models.Fail("not_found", "Ride not found")
	}
	err = m.repo.WithRideReminders(ctx, id.String(), actor.ID, func(tx database.ReminderTransaction) error {
		c := tx.Context()
		if err := access(c); err != nil {
			return err
		}
		if c.ParticipationVersion == c.SettingsParticipationVersion {
			out = c.Settings
		}
		return nil
	})
	return out, result(err)
}
func (m *Manager) Set(ctx context.Context, actor models.User, rideID string, settings models.RideReminders) (models.RideReminders, error) {
	id, err := uuid.Parse(rideID)
	if err != nil {
		return models.RideReminders{}, models.Fail("not_found", "Ride not found")
	}
	now := m.now().UTC()
	err = m.repo.WithRideReminders(ctx, id.String(), actor.ID, func(tx database.ReminderTransaction) error {
		c := tx.Context()
		if err := access(c); err != nil {
			return err
		}
		// New consent binds to the current approved participation generation.
		c.SettingsParticipationVersion = c.ParticipationVersion
		if (settings.DayBefore || settings.HourBefore) && (!Eligible(c) || !c.Ride.StartsAt.After(now)) {
			return models.Fail("ride_unavailable", "Choose reminders for an upcoming active ride")
		}
		return tx.SetReminders(settings, now)
	})
	return settings, result(err)
}
func (m *Manager) Dispatch(ctx context.Context) error {
	now := m.now().UTC()
	return m.repo.GenerateDueReminders(ctx, now, now.Add(-GracePeriod), 100, func(candidate models.ReminderCandidate) (*models.NotificationIntent, error) {
		c := candidate.Context
		if !Due(c, candidate.LeadMinutes, now) {
			return nil, nil
		}
		starts := c.Ride.StartsAt
		event := uuid.NewSHA1(uuid.NameSpaceOID, []byte(c.Ride.ID+":"+c.UserID+":"+starts.UTC().Format(time.RFC3339Nano)+":"+strconv.Itoa(candidate.LeadMinutes))).String()
		return &models.NotificationIntent{EventID: event, Kind: "ride.reminder", RideID: c.Ride.ID, Recipients: []string{c.UserID}, CreatedAt: now, ReminderStartsAt: &starts, ReminderLeadMinutes: candidate.LeadMinutes}, nil
	})
}
