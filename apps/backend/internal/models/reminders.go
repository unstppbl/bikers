package models

import "time"

// RideReminders is an explicit per-participant opt-in for departure reminders.
type RideReminders struct {
	DayBefore  bool `json:"day_before"`
	HourBefore bool `json:"hour_before"`
}

// ReminderContext is the current state read under the repository's ride/trust locks.
type ReminderContext struct {
	Ride                                                       Ride
	UserID, AccountStatus, ParticipationState, OrganizerStatus string
	ParticipationVersion, SettingsParticipationVersion         int64
	Settings                                                   RideReminders
	DayBeforeEnabledAt, HourBeforeEnabledAt                    *time.Time
	Hidden, OrganizerBlocked                                   bool
}

type ReminderCandidate struct {
	Context     ReminderContext
	LeadMinutes int
}
