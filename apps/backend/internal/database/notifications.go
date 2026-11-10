package database

import (
	"bikers.kz/backend/internal/models"
	"context"
	"time"
)

type NotificationDecision func(NotificationTransaction) error
type PushClaimDecision func(models.PushDelivery) bool
type NotificationRepository interface {
	WithNotifications(context.Context, string, NotificationDecision) error
	ClaimPush(context.Context, time.Time, time.Time, PushClaimDecision) (*models.PushDelivery, error)
	FinishPush(context.Context, string, string, models.PushDeliveryResult) error
}
type NotificationTransaction interface {
	Account() models.PushAccount
	SetPreferences(models.Preferences) error
	Notifications(int64, int) ([]models.Notification, error)
	UnreadCount() (int, error)
	MarkRead(string, time.Time) (models.Notification, error)
	MarkAllRead(time.Time) error
	// Binding reads use the manager's captured operation time. PutSubscription
	// uses its record's CreatedAt for both persistence and binding readback.
	Subscriptions(time.Time) ([]models.PushSubscriptionRecord, error)
	SubscriptionByEndpoint(string, time.Time) (models.PushSubscriptionRecord, error)
	PutSubscription(models.PushSubscriptionRecord) (models.PushSubscriptionRecord, error)
	DeleteSubscription(string) error
}
