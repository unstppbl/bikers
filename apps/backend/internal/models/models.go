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
	Description        string `json:"description"`
	Status             string `json:"status"`
	ParticipationState string `json:"participation_state,omitempty"`
	CanManage          *bool  `json:"can_manage,omitempty"`
}
type Ride struct {
	RideSummary
	Title        string    `json:"title"`
	StartsAt     time.Time `json:"starts_at"`
	Timezone     string    `json:"timezone"`
	MeetingPoint string    `json:"meeting_point"`
	Destination  string    `json:"destination"`
	RouteURL     string    `json:"route_url"`
	Expectations string    `json:"expectations"`
	Capacity     int       `json:"capacity"`
	PlanVersion  int64     `json:"plan_version"`
	OrganizerID  string    `json:"organizer_id"`
	MapPlan      *MapPlan  `json:"map_plan,omitempty"`
}
type RideInput struct {
	City         string     `json:"city"`
	Description  string     `json:"description"`
	Title        string     `json:"title"`
	StartsAt     time.Time  `json:"starts_at"`
	Timezone     string     `json:"timezone"`
	MeetingPoint string     `json:"meeting_point"`
	Destination  string     `json:"destination"`
	RouteURL     string     `json:"route_url"`
	Expectations string     `json:"expectations"`
	Capacity     int        `json:"capacity"`
	MapPoints    []MapPoint `json:"map_points,omitempty"`
}
type RidePatch struct {
	City         *string     `json:"city"`
	Description  *string     `json:"description"`
	Title        *string     `json:"title"`
	StartsAt     *time.Time  `json:"starts_at"`
	Timezone     *string     `json:"timezone"`
	MeetingPoint *string     `json:"meeting_point"`
	Destination  *string     `json:"destination"`
	RouteURL     *string     `json:"route_url"`
	Expectations *string     `json:"expectations"`
	Capacity     *int        `json:"capacity"`
	PlanVersion  int64       `json:"plan_version"`
	MapPoints    *[]MapPoint `json:"map_points"`
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
