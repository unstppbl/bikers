// Package notifications owns preferences, durable inboxes, push consent and delivery decisions.
package notifications

import (
	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"context"
	"crypto/elliptic"
	"encoding/base64"
	"errors"
	"github.com/google/uuid"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Gateway interface {
	Send(context.Context, models.PushSubscriptionInput, models.PushPayload) (int, error)
}
type Manager struct {
	repo      database.NotificationRepository
	gateway   Gateway
	publicKey string
	now       func() time.Time
}

func New(repo database.NotificationRepository, gateway Gateway, publicKey string, now func() time.Time) *Manager {
	if now == nil {
		now = time.Now
	}
	return &Manager{repo: repo, gateway: gateway, publicKey: publicKey, now: now}
}
func (m *Manager) PushSettings() (bool, string) {
	return m.gateway != nil && m.publicKey != "", m.publicKey
}
func active(tx database.NotificationTransaction) error {
	if tx.Account().Status != "active" {
		return models.Fail("account_unavailable", "Account unavailable")
	}
	return nil
}
func outcome(err error) error {
	if errors.Is(err, database.ErrNotFound) {
		return models.Fail("not_found", "Record not found")
	}
	return err
}
func (m *Manager) Preferences(ctx context.Context, actor models.User) (models.Preferences, error) {
	var out models.Preferences
	err := m.repo.WithNotifications(ctx, actor.ID, func(tx database.NotificationTransaction) error {
		if err := active(tx); err != nil {
			return err
		}
		out = tx.Account().Preferences
		return nil
	})
	return out, outcome(err)
}
func (m *Manager) UpdatePreferences(ctx context.Context, actor models.User, patch models.PreferencesPatch) (models.Preferences, error) {
	var out models.Preferences
	err := m.repo.WithNotifications(ctx, actor.ID, func(tx database.NotificationTransaction) error {
		if err := active(tx); err != nil {
			return err
		}
		out = tx.Account().Preferences
		if patch.CountryCode != nil {
			out.CountryCode = *patch.CountryCode
		}
		if patch.CurrencyCode != nil {
			out.CurrencyCode = *patch.CurrencyCode
		}
		if patch.Locale != nil {
			out.Locale = *patch.Locale
		}
		if !oneOf(out.CountryCode, "KZ", "KG", "UZ", "RU", "ZZ") || !oneOf(out.CurrencyCode, "KZT", "KGS", "UZS", "RUB", "USD", "EUR") || !models.ValidLocale(out.Locale) {
			return models.Fail("invalid_preferences", "Choose supported independent country, currency and language settings")
		}
		return tx.SetPreferences(out)
	})
	return out, outcome(err)
}
func oneOf(v string, options ...string) bool {
	for _, option := range options {
		if option == v {
			return true
		}
	}
	return false
}
func (m *Manager) List(ctx context.Context, actor models.User, cursor string) (models.NotificationPage, error) {
	out := models.NotificationPage{Notifications: []models.Notification{}}
	var before int64
	if cursor != "" {
		var err error
		before, err = strconv.ParseInt(cursor, 10, 64)
		if err != nil || before < 1 {
			return out, models.Fail("invalid_cursor", "Invalid notification cursor")
		}
	}
	err := m.repo.WithNotifications(ctx, actor.ID, func(tx database.NotificationTransaction) error {
		if err := active(tx); err != nil {
			return err
		}
		rows, err := tx.Notifications(before, 51)
		if err != nil {
			return err
		}
		if len(rows) > 50 {
			rows = rows[:50]
			out.NextCursor = strconv.FormatInt(rows[49].Sequence, 10)
		}
		out.Notifications = rows
		out.UnreadCount, err = tx.UnreadCount()
		return err
	})
	return out, outcome(err)
}
func (m *Manager) Read(ctx context.Context, actor models.User, id string) (models.Notification, error) {
	var out models.Notification
	if _, err := uuid.Parse(id); err != nil {
		return out, models.Fail("not_found", "Notification not found")
	}
	err := m.repo.WithNotifications(ctx, actor.ID, func(tx database.NotificationTransaction) error {
		if err := active(tx); err != nil {
			return err
		}
		var e error
		out, e = tx.MarkRead(id, m.now().UTC())
		return e
	})
	return out, outcome(err)
}
func (m *Manager) ReadAll(ctx context.Context, actor models.User) error {
	return outcome(m.repo.WithNotifications(ctx, actor.ID, func(tx database.NotificationTransaction) error {
		if err := active(tx); err != nil {
			return err
		}
		return tx.MarkAllRead(m.now().UTC())
	}))
}
func (m *Manager) Subscriptions(ctx context.Context, actor models.User) ([]models.PushSubscription, error) {
	out := []models.PushSubscription{}
	err := m.repo.WithNotifications(ctx, actor.ID, func(tx database.NotificationTransaction) error {
		if err := active(tx); err != nil {
			return err
		}
		records, err := tx.Subscriptions()
		if err != nil {
			return err
		}
		for _, record := range records {
			out = append(out, record.PushSubscription)
		}
		return nil
	})
	return out, outcome(err)
}
func ValidPushEndpoint(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" || u.Port() != "" || u.Host != strings.ToLower(u.Host) || len(raw) > 4096 {
		return false
	}
	host := u.Hostname()
	return host == "fcm.googleapis.com" || host == "updates.push.services.mozilla.com" || host == "web.push.apple.com" || (strings.HasSuffix(host, ".notify.windows.com") && host != "notify.windows.com")
}
func validateSubscription(input models.PushSubscriptionInput) error {
	pub, e1 := base64.RawURLEncoding.DecodeString(input.Keys.P256dh)
	auth, e2 := base64.RawURLEncoding.DecodeString(input.Keys.Auth)
	x, y := elliptic.Unmarshal(elliptic.P256(), pub)
	if !ValidPushEndpoint(input.Endpoint) || e1 != nil || e2 != nil || len(auth) != 16 || x == nil || y == nil {
		return models.Fail("invalid_push_subscription", "Use a valid browser push subscription")
	}
	return nil
}
func (m *Manager) Subscribe(ctx context.Context, session models.Session, input models.PushSubscriptionInput) (models.PushSubscription, error) {
	if m.gateway == nil || m.publicKey == "" {
		return models.PushSubscription{}, models.Fail("push_unavailable", "Push is not configured")
	}
	if err := validateSubscription(input); err != nil {
		return models.PushSubscription{}, err
	}
	var out models.PushSubscription
	err := m.repo.WithNotifications(ctx, session.User.ID, func(tx database.NotificationTransaction) error {
		if err := active(tx); err != nil {
			return err
		}
		existing, err := tx.SubscriptionByEndpoint(input.Endpoint)
		if err != nil && !errors.Is(err, database.ErrNotFound) {
			return err
		}
		if err == nil && existing.UserID != session.User.ID && existing.BindingActive {
			return models.Fail("push_subscription_in_use", "This browser is still linked to another signed-in account")
		}
		all, err := tx.Subscriptions()
		if err != nil {
			return err
		}
		if len(all) >= 5 && existing.UserID != session.User.ID {
			return models.Fail("push_subscription_limit", "Remove a device before adding another")
		}
		record := models.PushSubscriptionRecord{PushSubscription: models.PushSubscription{ID: uuid.NewString(), CreatedAt: m.now().UTC()}, UserID: session.User.ID, Endpoint: input.Endpoint, P256dh: input.Keys.P256dh, Auth: input.Keys.Auth, SessionDigest: session.Digest}
		stored, err := tx.PutSubscription(record)
		out = stored.PushSubscription
		return err
	})
	return out, outcome(err)
}
func (m *Manager) Unsubscribe(ctx context.Context, actor models.User, id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return models.Fail("not_found", "Subscription not found")
	}
	return outcome(m.repo.WithNotifications(ctx, actor.ID, func(tx database.NotificationTransaction) error {
		if err := active(tx); err != nil {
			return err
		}
		return tx.DeleteSubscription(id)
	}))
}
func pushAllowed(d models.PushDelivery) bool {
	if !d.AccountActive || !d.SessionActive {
		return false
	}
	if strings.HasPrefix(d.Kind, "ride.") {
		return d.RideAllowed
	}
	return true
}
func genericBody(locale string) string {
	switch locale {
	case "ru":
		return "У вас новое обновление"
	case "kk":
		return "Сізде жаңа жаңарту бар"
	default:
		return "You have a new update"
	}
}

// Dispatch processes a bounded batch. Database leases are committed before any network call.
func (m *Manager) Dispatch(ctx context.Context) error {
	if m.gateway == nil {
		return nil
	}
	for i := 0; i < 20; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		now := m.now().UTC()
		d, err := m.repo.ClaimPush(ctx, now, now.Add(2*time.Minute), pushAllowed)
		if err != nil {
			return err
		}
		if d == nil {
			return nil
		}
		sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		status, sendErr := m.gateway.Send(sendCtx, models.PushSubscriptionInput{Endpoint: d.Subscription.Endpoint, Keys: models.PushKeys{P256dh: d.Subscription.P256dh, Auth: d.Subscription.Auth}}, models.PushPayload{Title: "BIKERS.KZ", Body: genericBody(d.Locale), URL: "/inbox", NotificationID: d.NotificationID})
		cancel()
		result := models.PushDeliveryResult{State: "failed", NextAttempt: now}
		switch {
		case sendErr == nil && status >= 200 && status < 300:
			result.State = "delivered"
		case status == 404 || status == 410:
			result.RemoveSubscription = true
		case (sendErr != nil || status == 429 || status >= 500) && d.Attempts < 5:
			result.State = "pending"
			result.NextAttempt = m.now().UTC().Add(time.Duration(1<<d.Attempts) * 5 * time.Second)
		}
		finishCtx, finishCancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		err = m.repo.FinishPush(finishCtx, d.ID, d.LeaseID, result)
		finishCancel()
		if err != nil {
			return err
		}
	}
	return nil
}
