package notifications

import (
	"context"
	"testing"

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
