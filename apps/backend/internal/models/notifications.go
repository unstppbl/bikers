package models

import "time"

type Preferences struct {
	CountryCode  string `json:"country_code"`
	CurrencyCode string `json:"currency_code"`
	Locale       string `json:"locale"`
}
type PreferencesPatch struct {
	CountryCode  *string `json:"country_code"`
	CurrencyCode *string `json:"currency_code"`
	Locale       *string `json:"locale"`
}
type Notification struct {
	ID        string     `json:"id"`
	Kind      string     `json:"kind"`
	RideID    string     `json:"ride_id,omitempty"`
	ClubID    string     `json:"club_id,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	ReadAt    *time.Time `json:"read_at"`
	Sequence  int64      `json:"-"`
}
type NotificationPage struct {
	Notifications []Notification `json:"notifications"`
	NextCursor    string         `json:"next_cursor"`
	UnreadCount   int            `json:"unread_count"`
}
type NotificationIntent struct {
	EventID, Kind, RideID, ClubID string
	Recipients                    []string
	CreatedAt                     time.Time
}
type Announcement struct {
	ID                   string    `json:"id"`
	Sequence             int64     `json:"sequence"`
	OwnerID              string    `json:"owner_id"`
	OwnerName            string    `json:"owner_name"`
	Body                 string    `json:"body"`
	CreatedAt            time.Time `json:"created_at"`
	ClientAnnouncementID string    `json:"client_announcement_id"`
}
type AnnouncementInput struct {
	ClientAnnouncementID string `json:"client_announcement_id"`
	Body                 string `json:"body"`
}
type AnnouncementPage struct {
	Announcements []Announcement `json:"announcements"`
	NextCursor    string         `json:"next_cursor"`
}
type AnnouncementResult struct {
	Announcement Announcement `json:"announcement"`
	Created      bool         `json:"created"`
}
type PushKeys struct {
	P256dh string `json:"p256dh"`
	Auth   string `json:"auth"`
}
type PushSubscriptionInput struct {
	Endpoint       string   `json:"endpoint"`
	Keys           PushKeys `json:"keys"`
	ExpirationTime *float64 `json:"expirationTime,omitempty"`
}
type PushSubscription struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}
type PushSubscriptionRecord struct {
	PushSubscription
	UserID, Endpoint, P256dh, Auth string `json:"-"`
	SessionDigest                  []byte `json:"-"`
	BindingActive                  bool   `json:"-"`
}
type PushAccount struct {
	ID, Status  string
	Preferences Preferences
}
type PushDelivery struct {
	ID, LeaseID, Kind, Locale                    string
	NotificationID, RideID, ClubID               string
	ClubStatus, ClubOwnerID, ClubMembershipState string
	Subscription                                 PushSubscriptionRecord
	Attempts                                     int
	AccountActive, SessionActive, RideAllowed    bool
}
type PushDeliveryResult struct {
	State              string
	NextAttempt        time.Time
	RemoveSubscription bool
}
type PushPayload struct {
	Title          string `json:"title"`
	Body           string `json:"body"`
	URL            string `json:"url"`
	NotificationID string `json:"notification_id"`
}
