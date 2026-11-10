package notifications

import (
	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"context"
	"github.com/google/uuid"
	"time"
)

// TestPush sends one generic operator diagnostic to an explicitly selected
// account-owned subscription. It creates no ride, inbox item or delivery job.
func (m *Manager) TestPush(ctx context.Context, accountID, subscriptionID string) error {
	if m.gateway == nil || m.publicKey == "" {
		return models.Fail("push_unavailable", "Push is not configured")
	}
	account, accountErr := uuid.Parse(accountID)
	subscription, subscriptionErr := uuid.Parse(subscriptionID)
	if accountErr != nil || subscriptionErr != nil {
		return models.Fail("invalid_push_subscription", "Use valid account and subscription UUIDs")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var selected models.PushSubscriptionRecord
	var locale string
	at := m.now().UTC()
	err := m.repo.WithNotifications(ctx, account.String(), func(tx database.NotificationTransaction) error {
		if tx.Account().ID != account.String() || tx.Account().Status != "active" {
			return database.ErrNotFound
		}
		rows, err := tx.Subscriptions(at)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if row.ID == subscription.String() && row.UserID == account.String() && row.BindingActive {
				selected = row
				locale = tx.Account().Preferences.Locale
				return nil
			}
		}
		return database.ErrNotFound
	})
	if err != nil {
		// Repository/transport diagnostics can contain credentials or endpoint
		// data. Operator output intentionally contains no wrapped adapter errors.
		return models.Fail("push_test_unavailable", "An active push subscription for this account is unavailable")
	}
	if ctx.Err() != nil {
		return models.Fail("push_test_failed", "Push provider did not accept the test notification")
	}
	status, err := m.gateway.Send(ctx, models.PushSubscriptionInput{
		Endpoint: selected.Endpoint,
		Keys:     models.PushKeys{P256dh: selected.P256dh, Auth: selected.Auth},
	}, models.PushPayload{Title: "BIKERS.KZ", Body: genericBody(locale), URL: "/inbox", NotificationID: uuid.NewString()})
	if err != nil || status < 200 || status >= 300 {
		return models.Fail("push_test_failed", "Push provider did not accept the test notification")
	}
	return nil
}
