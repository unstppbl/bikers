package database

import (
	"bikers.kz/backend/internal/models"
	"context"
	"time"
)

type ReminderDecision func(ReminderTransaction) error
type DueReminderDecision func(models.ReminderCandidate) (*models.NotificationIntent, error)

type ReminderRepository interface {
	WithRideReminders(context.Context, string, string, ReminderDecision) error
	// GenerateDueReminders locks and rereads each due occurrence before calling
	// decide. Its occurrence receipt and any returned inbox/push intent commit together.
	GenerateDueReminders(context.Context, time.Time, time.Time, int, DueReminderDecision) error
}
type ReminderTransaction interface {
	Context() models.ReminderContext
	SetReminders(models.RideReminders, time.Time) error
}
