package models

// RideListInput contains the public discovery and personal ride-list filters.
type RideListInput struct {
	View, City, From, To, Search, Cursor string
}

type RidePage struct {
	Rides      []RideSummary `json:"rides"`
	NextCursor string        `json:"next_cursor"`
}
