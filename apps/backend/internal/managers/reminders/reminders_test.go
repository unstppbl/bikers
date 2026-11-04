package reminders

import (
	"context"
	"errors"
	"testing"
	"time"

	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
)

type reminderFake struct {
	state       models.ReminderContext
	writes      int
	candidates  []models.ReminderCandidate
	intents     []*models.NotificationIntent
	now, oldest time.Time
	limit       int
	err         error
}

func (r *reminderFake) WithRideReminders(_ context.Context, _, _ string, decide database.ReminderDecision) error {
	if r.err != nil {
		return r.err
	}
	return decide(r)
}
func (r *reminderFake) Context() models.ReminderContext { return r.state }
func (r *reminderFake) SetReminders(value models.RideReminders, at time.Time) error {
	r.writes++
	if !value.DayBefore {
		r.state.DayBeforeEnabledAt = nil
	} else if !r.state.Settings.DayBefore || r.state.ParticipationVersion != r.state.SettingsParticipationVersion {
		r.state.DayBeforeEnabledAt = consentAt(at)
	}
	if !value.HourBefore {
		r.state.HourBeforeEnabledAt = nil
	} else if !r.state.Settings.HourBefore || r.state.ParticipationVersion != r.state.SettingsParticipationVersion {
		r.state.HourBeforeEnabledAt = consentAt(at)
	}
	r.state.Settings = value
	r.state.SettingsParticipationVersion = r.state.ParticipationVersion
	return nil
}
func (r *reminderFake) GenerateDueReminders(_ context.Context, now, oldest time.Time, limit int, decide database.DueReminderDecision) error {
	r.now, r.oldest, r.limit = now, oldest, limit
	for _, c := range r.candidates {
		intent, err := decide(c)
		if err != nil {
			return err
		}
		r.intents = append(r.intents, intent)
	}
	return r.err
}
func consentAt(at time.Time) *time.Time { return &at }
func fixture() (time.Time, models.ReminderContext) {
	now := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	return now, models.ReminderContext{Ride: models.Ride{RideSummary: models.RideSummary{ID: uuid.NewString(), Status: "published"}, OrganizerID: uuid.NewString(), StartsAt: now.Add(time.Hour)}, UserID: uuid.NewString(), AccountStatus: "active", OrganizerStatus: "active", ParticipationState: "approved", ParticipationVersion: 3, SettingsParticipationVersion: 3, Settings: models.RideReminders{DayBefore: true, HourBefore: true}, DayBeforeEnabledAt: consentAt(now.Add(-48 * time.Hour)), HourBeforeEnabledAt: consentAt(now.Add(-48 * time.Hour))}
}
func TestDueRevalidatesConsentAccessAndBoundedDepartureWindow(t *testing.T) {
	now, base := fixture()
	for _, tc := range []struct {
		name   string
		change func(*models.ReminderContext)
		shift  time.Duration
		lead   int
		want   bool
	}{
		{name: "hour due", lead: 60, want: true},
		{name: "day due", lead: 1440, change: func(c *models.ReminderContext) { c.Ride.StartsAt = now.Add(24 * time.Hour) }, want: true},
		{name: "not due yet", lead: 60, shift: -time.Second},
		{name: "grace boundary", lead: 60, shift: GracePeriod, want: true},
		{name: "missed window", lead: 60, shift: GracePeriod + time.Nanosecond},
		{name: "departed", lead: 60, shift: time.Hour},
		{name: "cancelled", lead: 60, change: func(c *models.ReminderContext) { c.Ride.Status = "cancelled" }},
		{name: "left", lead: 60, change: func(c *models.ReminderContext) { c.ParticipationState = "left" }},
		{name: "rejoined generation", lead: 60, change: func(c *models.ReminderContext) { c.ParticipationVersion++ }},
		{name: "owner", lead: 60, change: func(c *models.ReminderContext) { c.Ride.OrganizerID = c.UserID; c.ParticipationState = "" }, want: true},
		{name: "account restricted", lead: 60, change: func(c *models.ReminderContext) { c.AccountStatus = "suspended" }},
		{name: "organizer restricted", lead: 60, change: func(c *models.ReminderContext) { c.OrganizerStatus = "suspended" }},
		{name: "hidden", lead: 60, change: func(c *models.ReminderContext) { c.Hidden = true }},
		{name: "blocked", lead: 60, change: func(c *models.ReminderContext) { c.OrganizerBlocked = true }},
		{name: "opted out", lead: 60, change: func(c *models.ReminderContext) { c.Settings.HourBefore = false }},
		{name: "late opt in", lead: 60, change: func(c *models.ReminderContext) { c.HourBeforeEnabledAt = consentAt(now.Add(time.Second)) }, shift: time.Second},
		{name: "unsupported lead", lead: 30},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := base
			if tc.change != nil {
				tc.change(&c)
			}
			if got := Due(c, tc.lead, now.Add(tc.shift)); got != tc.want {
				t.Fatalf("Due=%v want %v", got, tc.want)
			}
		})
	}
}
func TestSettingsRequireApprovedAccessAndRebindExplicitConsent(t *testing.T) {
	now, state := fixture()
	r := &reminderFake{state: state}
	m := New(r, func() time.Time { return now })
	actor := models.User{ID: state.UserID}
	r.state.SettingsParticipationVersion = 2
	got, err := m.Get(context.Background(), actor, state.Ride.ID)
	if err != nil || got != (models.RideReminders{}) {
		t.Fatalf("rejoined rider inherited old consent: %+v %v", got, err)
	}
	choice := models.RideReminders{HourBefore: true}
	got, err = m.Set(context.Background(), actor, state.Ride.ID, choice)
	if err != nil || got != choice || r.writes != 1 || r.state.SettingsParticipationVersion != 3 {
		t.Fatalf("explicit consent was not rebound: %+v %v", got, err)
	}
	r.state.ParticipationState = "requested"
	if _, err = m.Get(context.Background(), actor, state.Ride.ID); err == nil {
		t.Fatal("pending rider read reminder settings")
	}
	if _, err = m.Set(context.Background(), actor, state.Ride.ID, choice); err == nil || r.writes != 1 {
		t.Fatal("pending rider changed reminder settings")
	}
	r.state = state
	r.state.Ride.Status = "cancelled"
	if _, err = m.Set(context.Background(), actor, state.Ride.ID, choice); err == nil {
		t.Fatal("cancelled ride enabled reminders")
	}
	if _, err = m.Set(context.Background(), actor, state.Ride.ID, models.RideReminders{}); err != nil {
		t.Fatalf("cancelled ride could not opt out: %v", err)
	}
	r.err = database.ErrNotFound
	if _, err = m.Get(context.Background(), actor, state.Ride.ID); err == nil {
		t.Fatal("missing ride accepted")
	}
	var fault *models.Fault
	_, err = m.Set(context.Background(), actor, "bad-id", choice)
	if !errors.As(err, &fault) || fault.Code != "not_found" {
		t.Fatalf("bad ID=%v", err)
	}
}
func TestDispatchBindsStableOccurrenceIntentAndSkipsInvalidCandidates(t *testing.T) {
	now, state := fixture()
	invalid := state
	invalid.ParticipationState = "removed"
	r := &reminderFake{candidates: []models.ReminderCandidate{{Context: state, LeadMinutes: 60}, {Context: invalid, LeadMinutes: 60}}}
	m := New(r, func() time.Time { return now })
	if err := m.Dispatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if r.limit != 100 || !r.now.Equal(now) || !r.oldest.Equal(now.Add(-GracePeriod)) || len(r.intents) != 2 || r.intents[1] != nil {
		t.Fatal("dispatch did not bound/skip candidates")
	}
	intent := r.intents[0]
	if intent == nil || intent.Kind != "ride.reminder" || intent.RideID != state.Ride.ID || len(intent.Recipients) != 1 || intent.Recipients[0] != state.UserID || intent.ReminderStartsAt == nil || !intent.ReminderStartsAt.Equal(state.Ride.StartsAt) || intent.ReminderLeadMinutes != 60 {
		t.Fatalf("wrong reminder intent: %+v", intent)
	}
	first := intent.EventID
	if err := m.Dispatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if r.intents[2].EventID != first {
		t.Fatal("poll retries changed occurrence event identity")
	}
	r.candidates[0].Context.Ride.StartsAt = now.Add(24 * time.Hour)
	r.candidates[0].LeadMinutes = 1440
	r.candidates[0].Context.DayBeforeEnabledAt = consentAt(now.Add(-time.Hour))
	if err := m.Dispatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if r.intents[4] == nil || r.intents[4].EventID == first {
		t.Fatal("different departure occurrence reused event")
	}
}
