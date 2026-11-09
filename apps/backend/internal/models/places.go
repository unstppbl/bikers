package models

import (
	"net/url"
	"strings"
	"time"
)

// PlaceInput is the editable public information about a useful place. Provenance
// and publication state are assigned by the application, never by contributors.
type PlaceInput struct {
	Name         string  `json:"name"`
	Category     string  `json:"category"`
	Lat          float64 `json:"lat"`
	Lon          float64 `json:"lon"`
	City         string  `json:"city"`
	Address      string  `json:"address"`
	Description  string  `json:"description"`
	Website      string  `json:"website"`
	Phone        string  `json:"phone"`
	OpeningHours string  `json:"opening_hours"`
}

type Place struct {
	ID string `json:"id"`
	PlaceInput
	Source     string    `json:"source"`
	SourceRef  string    `json:"source_ref,omitempty"`
	SourceURL  string    `json:"source_url"`
	SourceDate string    `json:"source_date,omitempty"`
	UpdatedAt  time.Time `json:"updated_at"`
	Version    int64     `json:"version"`
	Status     string    `json:"status"`
}

type PlaceBounds struct{ West, South, East, North float64 }
type PlacePage struct {
	Places     []Place `json:"places"`
	NextCursor string  `json:"next_cursor"`
}
type PlaceSubmissionInput struct {
	Kind        string      `json:"kind"`
	PlaceID     string      `json:"place_id,omitempty"`
	BaseVersion int64       `json:"base_version,omitempty"`
	Place       *PlaceInput `json:"place,omitempty"`
	Reason      string      `json:"reason"`
}
type PlaceSubmission struct {
	ID string `json:"id"`
	PlaceSubmissionInput
	SubmitterID  string    `json:"submitter_id,omitempty"`
	ReviewerID   string    `json:"reviewer_id,omitempty"`
	ReviewReason string    `json:"review_reason,omitempty"`
	Status       string    `json:"status"`
	Version      int64     `json:"version"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
type PlaceSubmissionPage struct {
	Submissions []PlaceSubmission `json:"submissions"`
	NextCursor  string            `json:"next_cursor"`
}
type PlaceReviewInput struct {
	Version int64  `json:"version"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
}
type PlaceImportRecord struct {
	PlaceInput
	SourceRef  string `json:"source_ref"`
	SourceDate string `json:"source_date"`
}
type PlaceImportResult struct {
	Total     int `json:"total"`
	Inserted  int `json:"inserted"`
	Updated   int `json:"updated"`
	Unchanged int `json:"unchanged"`
	Preserved int `json:"preserved"`
}

func PlaceCategory(value string) bool {
	switch value {
	case "fuel", "workshop", "tires", "motorcycle_shop", "parking", "meeting_spot":
		return true
	}
	return false
}

// ValidatePlace normalizes only user-entered values. It does not geocode or
// infer a missing point, category, service, or opening time.
func ValidatePlace(in PlaceInput) (PlaceInput, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Category = strings.TrimSpace(in.Category)
	in.City = NormalizeCity(in.City)
	in.Address = strings.TrimSpace(in.Address)
	in.Description = strings.TrimSpace(in.Description)
	in.Website = strings.TrimSpace(in.Website)
	in.Phone = strings.TrimSpace(in.Phone)
	in.OpeningHours = strings.TrimSpace(in.OpeningHours)
	if in.Name == "" || len(in.Name) > 200 || !PlaceCategory(in.Category) || !ValidCoordinate(in.Lat, in.Lon) || in.Lon < 46 || in.Lon > 87.5 || in.Lat < 40.5 || in.Lat > 55.5 || len(in.City) > 160 || len(in.Address) > 500 || len(in.Description) > 2000 || len(in.Website) > 2000 || len(in.Phone) > 100 || len(in.OpeningHours) > 300 {
		return in, Fail("invalid_place", "Choose a name, category and Kazakhstan point within the field limits")
	}
	if in.Website != "" {
		u, e := url.Parse(in.Website)
		if e != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
			return in, Fail("invalid_place", "Website must be an HTTP or HTTPS URL")
		}
	}
	return in, nil
}
