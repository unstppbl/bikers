package models

import "time"

type Block struct {
	UserID      string    `json:"user_id"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
}
type BlockPage struct {
	Blocks     []Block `json:"blocks"`
	NextCursor string  `json:"next_cursor"`
}
type TrustAccount struct {
	ID, DisplayName, Status string
	Admin                   bool
}
type TrustBadge struct {
	Label      string    `json:"label"`
	AssignedAt time.Time `json:"assigned_at"`
	Disclaimer string    `json:"disclaimer"`
}

// TrustTarget is the limited reported-content snapshot, never an account dump.
type TrustTarget struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	AuthorID    string `json:"author_id,omitempty"`
	ScopeID     string `json:"scope_id,omitempty"`
	Text        string `json:"text,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	City        string `json:"city,omitempty"`
	Date        string `json:"date,omitempty"`
	Status      string `json:"status,omitempty"`
	MediaURL    string `json:"media_url,omitempty"`
}
type TrustTargetRecord struct {
	Target                                                TrustTarget
	OwnerID, Visibility, Participation, Membership, State string
	AuthorStatus                                          string
	SharedRides, SharedClubs                              int
	Current                                               bool
	Content                                               []byte
}
type ReportInput struct {
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Reason     string `json:"reason"`
	Details    string `json:"details"`
}
type Report struct {
	ID         string      `json:"id"`
	ReporterID string      `json:"reporter_id"`
	TargetType string      `json:"target_type"`
	TargetID   string      `json:"target_id"`
	Reason     string      `json:"reason"`
	Details    string      `json:"details"`
	Status     string      `json:"status"`
	Version    int64       `json:"version"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
	Snapshot   TrustTarget `json:"-"`
}
type ReportReceipt struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}
type ReportPage struct {
	Reports    []Report `json:"reports"`
	NextCursor string   `json:"next_cursor"`
}
type ReportPatch struct {
	Status  string `json:"status"`
	Version int64  `json:"version"`
	Reason  string `json:"reason"`
}
type TrustActionInput struct {
	ClientActionID string `json:"client_action_id"`
	Action         string `json:"action"`
	TargetType     string `json:"target_type"`
	TargetID       string `json:"target_id"`
	Reason         string `json:"reason"`
}
type TrustAudit struct {
	ID             string    `json:"id"`
	ActorID        string    `json:"actor_id,omitempty"`
	Source         string    `json:"source"`
	Action         string    `json:"action"`
	TargetType     string    `json:"target_type"`
	TargetID       string    `json:"target_id"`
	Reason         string    `json:"reason"`
	ClientActionID string    `json:"client_action_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	PayloadHash    string    `json:"-"`
}
type TrustAuditPage struct {
	Actions    []TrustAudit `json:"actions"`
	NextCursor string       `json:"next_cursor"`
}
type TrustCursor struct {
	CreatedAt time.Time
	ID        string
}
