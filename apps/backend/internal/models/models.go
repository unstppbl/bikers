package models

import "time"

// Public domain types retain the original wire tags and field order.
type Profile struct {
	DisplayName string   `json:"display_name"`
	City        string   `json:"city"`
	Languages   []string `json:"languages"`
	Interests   string   `json:"interests"`
	Locale      string   `json:"locale"`
}
type User struct {
	ID                 string  `json:"id"`
	Profile            Profile `json:"profile"`
	OnboardingComplete bool    `json:"onboarding_complete"`
}
type Session struct {
	User   User
	CSRF   string
	Digest []byte
}
type Account struct {
	User      User
	Phone     string
	Status    string
	CreatedAt time.Time
}
type SessionRecord struct {
	Session
	CreatedAt  time.Time
	LastSeenAt time.Time
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	UserStatus string
}
type OTPChallenge struct {
	ID, Phone, SourceKey, ProviderID, State string
	Attempts                                int
	CreatedAt, ExpiresAt, ResendAfter       time.Time
	ConsumedAt                              *time.Time
}
type SendWindow struct {
	PhoneSends, SourceSends, Daily int
	Cooldown                       bool
}
type Verification struct {
	RequestID          string `json:"request_id"`
	Phone              string `json:"phone_number"`
	VerificationStatus *struct {
		Status string `json:"status"`
	} `json:"verification_status"`
}
type ChallengeResult struct {
	ID                     string
	ExpiresAt, ResendAfter time.Time
}
type VerificationInput struct {
	ChallengeID string `json:"challenge_id"`
	Code        string `json:"code"`
}
type LoginResult struct {
	User        User
	CSRF, Token string
	ExpiresAt   time.Time
	CreatedAt   time.Time
}
type ProfilePatch struct {
	DisplayName *string   `json:"display_name"`
	City        *string   `json:"city"`
	Languages   *[]string `json:"languages"`
	Interests   *string   `json:"interests"`
	Locale      *string   `json:"locale"`
}
type RideSummary struct {
	ID                 string `json:"id"`
	City               string `json:"city"`
	Date               string `json:"date"`
	Title              string `json:"title,omitempty"`
	Description        string `json:"description"`
	Status             string `json:"status"`
	TimeWindow         string `json:"time_window,omitempty"`
	Style              string `json:"style,omitempty"`
	Surface            string `json:"surface,omitempty"`
	DistanceKm         int    `json:"distance_km,omitempty"`
	Capacity           int    `json:"capacity,omitempty"`
	ApprovedCount      int    `json:"approved_count,omitempty"`
	OrganizerName      string `json:"organizer_name,omitempty"`
	ParticipationState string `json:"participation_state,omitempty"`
	CanManage          *bool  `json:"can_manage,omitempty"`
	// Cover is the current public cover photo; moderation-hidden covers are omitted.
	Cover *Photo `json:"cover,omitempty"`
	// DetailsPublished is persistence metadata used by managers for public projection.
	DetailsPublished bool `json:"-"`
}
type Ride struct {
	RideSummary
	StartsAt     time.Time `json:"starts_at"`
	Timezone     string    `json:"timezone"`
	MeetingPoint string    `json:"meeting_point"`
	Destination  string    `json:"destination"`
	RouteURL     string    `json:"route_url"`
	Expectations string    `json:"expectations"`
	PlanVersion  int64     `json:"plan_version"`
	OrganizerID  string    `json:"organizer_id"`
	MapPlan      *MapPlan  `json:"map_plan,omitempty"`
	// CoverVersion is sent only to the organizer while the cover may change.
	CoverVersion           int64 `json:"cover_version,omitempty"`
	PublicDetailsPublished bool  `json:"public_details_published"`
	// DistanceOverrideKm is the stored manual value; zero selects the current map estimate.
	DistanceOverrideKm int `json:"distance_override_km"`
}
type RideInput struct {
	City                 string     `json:"city"`
	Description          string     `json:"description"`
	Title                string     `json:"title"`
	Style                string     `json:"style,omitempty"`
	Surface              string     `json:"surface,omitempty"`
	DistanceKm           int        `json:"distance_km,omitempty"`
	StartsAt             time.Time  `json:"starts_at"`
	Timezone             string     `json:"timezone"`
	MeetingPoint         string     `json:"meeting_point"`
	Destination          string     `json:"destination"`
	RouteURL             string     `json:"route_url"`
	Expectations         string     `json:"expectations"`
	Capacity             int        `json:"capacity"`
	MapPoints            []MapPoint `json:"map_points,omitempty"`
	ExpectedMapDigest    string     `json:"expected_map_digest,omitempty"`
	PublishPublicDetails bool       `json:"publish_public_details"`
}
type RidePatch struct {
	City                 *string     `json:"city"`
	Description          *string     `json:"description"`
	Title                *string     `json:"title"`
	Style                *string     `json:"style"`
	Surface              *string     `json:"surface"`
	DistanceKm           *int        `json:"distance_km"`
	StartsAt             *time.Time  `json:"starts_at"`
	Timezone             *string     `json:"timezone"`
	MeetingPoint         *string     `json:"meeting_point"`
	Destination          *string     `json:"destination"`
	RouteURL             *string     `json:"route_url"`
	Expectations         *string     `json:"expectations"`
	Capacity             *int        `json:"capacity"`
	PlanVersion          int64       `json:"plan_version"`
	MapPoints            *[]MapPoint `json:"map_points"`
	PublishPublicDetails *bool       `json:"publish_public_details"`
}
type RideListing struct {
	Summary     RideSummary
	OrganizerID string
}
type Participant struct {
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name"`
	State       string `json:"state"`
}
type RoomAccess struct{ ParticipationState, UserStatus string }
type Message struct {
	ID              string    `json:"id"`
	Sequence        int64     `json:"sequence"`
	SenderID        string    `json:"sender_id"`
	SenderName      string    `json:"sender_name"`
	Body            string    `json:"body"`
	CreatedAt       time.Time `json:"created_at"`
	ClientMessageID string    `json:"client_message_id"`
}
type MessageInput struct {
	ClientMessageID string `json:"client_message_id"`
	Body            string `json:"body"`
}
type MessagePage struct {
	Messages     []Message
	NextSequence int64
}
type MessageResult struct {
	Message Message
	Created bool
}
type Event struct {
	Type   string `json:"type"`
	RideID string `json:"ride_id"`
}

// Faults describe application outcomes; HTTP status selection belongs to transport.
type Fault struct{ Code, Message string }

func (f *Fault) Error() string        { return f.Message }
func Fail(code, message string) error { return &Fault{Code: code, Message: message} }
