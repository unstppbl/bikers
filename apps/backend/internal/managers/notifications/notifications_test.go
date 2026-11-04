package notifications

import (
	"context"
	"testing"
	"time"

	"bikers.kz/backend/internal/models"
)

func TestPushEndpointBoundaryRejectsArbitraryNetworkTargets(t *testing.T) {
	for _, raw := range []string{
		"https://fcm.googleapis.com/fcm/send/browser-token",
		"https://updates.push.services.mozilla.com/wpush/v2/browser-token",
		"https://web.push.apple.com/browser-token",
		"https://wns2-par02p.notify.windows.com/w/?token=browser-token",
	} {
		if !ValidPushEndpoint(raw) {
			t.Errorf("supported browser endpoint rejected: %s", raw)
		}
	}
	for _, raw := range []string{
		"http://fcm.googleapis.com/browser-token", "https://127.0.0.1/private", "https://[::1]/private",
		"https://localhost/private", "https://192.168.1.1/private", "https://169.254.169.254/latest/meta-data",
		"https://example.com/push", "https://fcm.googleapis.com.example.com/push", "https://evil-fcm.googleapis.com/push",
		"https://fcm.googleapis.com:443/push", "https://user:password@fcm.googleapis.com/push",
		"https://fcm.googleapis.com/push#fragment", "https://FCM.GOOGLEAPIS.COM/push",
		"https://notify.windows.com/push", "https://example.notify.windows.com.evil.com/push",
		"javascript:alert(1)", "/relative/push", "",
	} {
		if ValidPushEndpoint(raw) {
			t.Errorf("untrusted endpoint accepted: %s", raw)
		}
	}
}

func TestMalformedInboxCursorsAndDisabledPushFailBeforeRepositoryAccess(t *testing.T) {
	m := New(nil, nil, "", nil)
	for _, cursor := range []string{"0", "-1", "abc", "1.5", "9223372036854775808"} {
		if _, err := m.List(context.Background(), models.User{}, cursor); err == nil {
			t.Errorf("invalid cursor accepted: %s", cursor)
		}
	}
	if enabled, key := m.PushSettings(); enabled || key != "" {
		t.Fatal("unconfigured push advertised as enabled")
	}
	if _, err := m.Subscribe(context.Background(), models.Session{}, models.PushSubscriptionInput{}); err == nil {
		t.Fatal("unconfigured push accepted a subscription")
	}
	if err := m.Dispatch(context.Background()); err != nil {
		t.Fatal("unconfigured push worker did not idle", err)
	}
}

func TestClubPushEligibilityRechecksOwnershipMembershipAndSession(t *testing.T) {
	base := models.PushDelivery{
		PushCategories: models.PushCategories{RideUpdates: true, Participation: true, ClubMembership: true, RideReminders: true},
		Kind:           "club.membership.requested", ClubID: "club", ClubStatus: "active", ClubOwnerID: "owner",
		AccountActive: true, SessionActive: true,
		Subscription: models.PushSubscriptionRecord{UserID: "owner"},
	}
	for _, test := range []struct {
		name    string
		change  func(*models.PushDelivery)
		allowed bool
	}{
		{name: "current owner request", allowed: true},
		{name: "transferred owner", change: func(d *models.PushDelivery) { d.ClubOwnerID = "new-owner" }},
		{name: "closed club request", change: func(d *models.PushDelivery) { d.ClubStatus = "closed" }},
		{name: "approved member", change: func(d *models.PushDelivery) {
			d.Kind = "club.membership.approved"
			d.Subscription.UserID = "member"
			d.ClubMembershipState = "approved"
		}, allowed: true},
		{name: "approved became owner", change: func(d *models.PushDelivery) { d.Kind = "club.membership.approved" }, allowed: true},
		{name: "approved member left", change: func(d *models.PushDelivery) {
			d.Kind = "club.membership.approved"
			d.Subscription.UserID = "member"
			d.ClubMembershipState = "left"
		}},
		{name: "approved club closed", change: func(d *models.PushDelivery) { d.Kind = "club.membership.approved"; d.ClubStatus = "closed" }},
		{name: "rejected applicant", change: func(d *models.PushDelivery) {
			d.Kind = "club.membership.rejected"
			d.Subscription.UserID = "applicant"
			d.ClubMembershipState = "rejected"
		}, allowed: true},
		{name: "removed member", change: func(d *models.PushDelivery) {
			d.Kind = "club.membership.removed"
			d.Subscription.UserID = "member"
			d.ClubMembershipState = "removed"
		}, allowed: true},
		{name: "outcome after closure", change: func(d *models.PushDelivery) { d.Kind = "club.membership.rejected"; d.ClubStatus = "closed" }, allowed: true},
		{name: "revoked session", change: func(d *models.PushDelivery) { d.SessionActive = false }},
		{name: "restricted account", change: func(d *models.PushDelivery) { d.AccountActive = false }},
		{name: "unknown kind", change: func(d *models.PushDelivery) { d.Kind = "club.membership.future" }},
		{name: "ambiguous target", change: func(d *models.PushDelivery) { d.RideID = "ride" }},
		{name: "missing club", change: func(d *models.PushDelivery) { d.ClubID = "" }},
		{name: "missing recipient", change: func(d *models.PushDelivery) { d.Subscription.UserID = "" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			delivery := base
			if test.change != nil {
				test.change(&delivery)
			}
			if allowed := pushAllowed(delivery, time.Now()); allowed != test.allowed {
				t.Fatalf("push allowed=%v, want %v", allowed, test.allowed)
			}
		})
	}
}
