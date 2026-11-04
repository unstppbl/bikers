package database

import "time"

// RideListQuery contains the manager's selection policy. The repository owns
// applying these selections, moderation visibility and keyset pagination.
type RideListQuery struct {
	UserID, FromDate, ToDate, Search                   string
	CityAliases                                        []string
	OrganizerOnly, ExcludeOrganizer, RelatedOnly, Past bool
	ParticipationState                                 string
	Now                                                time.Time
	AfterDate, AfterID                                 string
	Limit                                              int
}
