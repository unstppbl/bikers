package models

type ClubSummary struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	City            string `json:"city"`
	Description     string `json:"description"`
	Status          string `json:"status"`
	MembershipState string `json:"membership_state,omitempty"`
	CanManage       *bool  `json:"can_manage,omitempty"`
}
type Club struct {
	ClubSummary
	OwnerID     string `json:"owner_id"`
	Version     int64  `json:"version"`
	MemberCount int    `json:"member_count"`
}
type ClubInput struct {
	Name        string `json:"name"`
	City        string `json:"city"`
	Description string `json:"description"`
}
type ClubPatch struct {
	Version     int64   `json:"version"`
	Name        *string `json:"name"`
	City        *string `json:"city"`
	Description *string `json:"description"`
}
type ClubTransfer struct {
	Version int64  `json:"version"`
	UserID  string `json:"user_id"`
}
type ClubMember struct {
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name"`
	State       string `json:"state"`
	Role        string `json:"role"`
}
type ClubPage struct {
	Clubs      []ClubSummary `json:"clubs"`
	NextCursor string        `json:"next_cursor"`
}

// Repository records carry authorization state without serializing it.
type ClubListing struct {
	Club            Club
	MembershipState string
}
type ClubMembership struct{ State, AccountStatus string }
type ClubRideCandidate struct {
	Summary     RideSummary
	OrganizerID string
}
