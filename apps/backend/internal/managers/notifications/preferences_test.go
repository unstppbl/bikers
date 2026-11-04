package notifications

import (
	"bikers.kz/backend/internal/models"
	"context"
	"testing"
	"time"
)

func TestSparsePushPreferencesPreserveLocaleAndOtherCategories(t *testing.T) {
	m, r, _, actor, _ := diagnosticFixture(t)
	r.tx.account.Preferences = models.Preferences{CountryCode: "KZ", CurrencyCode: "KZT", Locale: "kk", PushCategories: models.PushCategories{RideUpdates: true, Participation: true, ClubMembership: true, RideReminders: true}}
	disabled := false
	got, err := m.UpdatePreferences(context.Background(), models.User{ID: actor}, models.PreferencesPatch{PushCategories: &models.PushCategoriesPatch{ClubMembership: &disabled}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Locale != "kk" || got.CountryCode != "KZ" || !got.PushCategories.RideUpdates || !got.PushCategories.Participation || !got.PushCategories.RideReminders || got.PushCategories.ClubMembership {
		t.Fatalf("sparse preferences lost unrelated values: %+v", got)
	}
	if r.tx.preferenceWrites != 1 || r.tx.account.Preferences != got {
		t.Fatal("preferences were not persisted")
	}
	r.tx.unreadCount = 4
	if count, err := m.UnreadCount(context.Background(), models.User{ID: actor}); err != nil || count != 4 {
		t.Fatalf("unread count=%d %v", count, err)
	}
	r.tx.account.Status = "suspended"
	if _, err := m.UnreadCount(context.Background(), models.User{ID: actor}); err == nil {
		t.Fatal("restricted account read unread count")
	}
	if _, err := m.UpdatePreferences(context.Background(), models.User{ID: actor}, models.PreferencesPatch{}); err == nil || r.tx.preferenceWrites != 1 {
		t.Fatal("restricted account changed preferences")
	}
}
func TestPushCategoriesAreRecheckedForAllKinds(t *testing.T) {
	for _, kind := range []string{"ride.updated", "ride.cancelled", "ride.announcement", "participation.requested", "participation.approved", "participation.rejected", "participation.removed", "club.membership.requested", "club.membership.approved", "club.membership.rejected", "club.membership.removed"} {
		t.Run(kind, func(t *testing.T) {
			d := models.PushDelivery{Kind: kind, AccountActive: true, SessionActive: true, RideAllowed: true, ClubID: "club", ClubStatus: "active", ClubOwnerID: "actor", ClubMembershipState: "approved", Subscription: models.PushSubscriptionRecord{UserID: "actor"}}
			if pushAllowed(d, time.Now()) {
				t.Fatal("disabled category allowed delivery")
			}
			d.PushCategories = models.PushCategories{RideUpdates: true, Participation: true, ClubMembership: true, RideReminders: true}
			if !pushAllowed(d, time.Now()) {
				t.Fatal("enabled category rejected otherwise eligible delivery")
			}
		})
	}
}
func TestReminderPushRechecksDepartureChoiceAndGeneration(t *testing.T) {
	now := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	start := now.Add(time.Hour)
	enabledAt := now.Add(-time.Hour)
	d := models.PushDelivery{Kind: "ride.reminder", AccountActive: true, SessionActive: true, PushCategories: models.PushCategories{RideReminders: true}, ReminderStartsAt: &start, ReminderLeadMinutes: 60, ReminderContext: models.ReminderContext{Ride: models.Ride{RideSummary: models.RideSummary{Status: "published"}, StartsAt: start, OrganizerID: "owner"}, UserID: "member", AccountStatus: "active", OrganizerStatus: "active", ParticipationState: "approved", ParticipationVersion: 1, SettingsParticipationVersion: 1, Settings: models.RideReminders{HourBefore: true}, HourBeforeEnabledAt: &enabledAt}}
	if !pushAllowed(d, now) {
		t.Fatal("eligible reminder rejected")
	}
	for _, change := range []func(*models.PushDelivery){func(d *models.PushDelivery) { d.ReminderContext.Ride.StartsAt = start.Add(time.Hour) }, func(d *models.PushDelivery) { d.ReminderContext.ParticipationVersion++ }, func(d *models.PushDelivery) { d.ReminderContext.Settings.HourBefore = false }, func(d *models.PushDelivery) { d.PushCategories.RideReminders = false }, func(d *models.PushDelivery) { d.ReminderContext.Ride.Status = "cancelled" }} {
		changed := d
		change(&changed)
		if pushAllowed(changed, now) {
			t.Fatal("stale reminder delivery remained eligible")
		}
	}
	if pushAllowed(d, now.Add(11*time.Minute)) {
		t.Fatal("stale queued reminder survived grace window")
	}
}
