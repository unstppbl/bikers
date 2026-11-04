package models

import (
	"errors"
	"time"
)

type BikeModel struct {
	ID      string           `json:"id"`
	Make    string           `json:"make"`
	Model   string           `json:"model"`
	Aliases []string         `json:"aliases"`
	Images  []BikeModelImage `json:"images"`
}

// BikeModelImage describes a curated public reference, never an owner's upload.
type BikeModelImage struct {
	URL        string `json:"url"`
	YearFrom   int    `json:"year_from"`
	YearTo     int    `json:"year_to"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	SourceURL  string `json:"source_url"`
	Author     string `json:"author"`
	License    string `json:"license"`
	LicenseURL string `json:"license_url"`
}

type Bike struct {
	ID         string    `json:"id"`
	OwnerID    string    `json:"owner_id"`
	Make       string    `json:"make"`
	Model      string    `json:"model"`
	ModelID    string    `json:"model_id"`
	Year       int       `json:"year"`
	Visibility string    `json:"visibility"`
	Version    int64     `json:"version"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	CanManage  bool      `json:"can_manage"`
}

type PublicBike struct {
	ID      string `json:"id"`
	Make    string `json:"make"`
	Model   string `json:"model"`
	ModelID string `json:"model_id"`
	Year    int    `json:"year"`
}

type BikeInput struct {
	Make       string `json:"make"`
	Model      string `json:"model"`
	ModelID    string `json:"model_id"`
	Year       int    `json:"year"`
	Visibility string `json:"visibility"`
}

type BikePatch struct {
	Version    int64   `json:"version"`
	Make       *string `json:"make"`
	Model      *string `json:"model"`
	ModelID    *string `json:"model_id"`
	Year       *int    `json:"year"`
	Visibility *string `json:"visibility"`
}

type BikePage struct {
	Bikes      []Bike `json:"bikes"`
	NextCursor string `json:"next_cursor"`
}

type MaintenanceEntry struct {
	ID            string    `json:"id"`
	BikeID        string    `json:"bike_id"`
	ServiceDate   string    `json:"service_date"`
	MileageKM     int64     `json:"mileage_km"`
	WorkPerformed string    `json:"work_performed"`
	Version       int64     `json:"version"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type MaintenanceInput struct {
	ServiceDate   string `json:"service_date"`
	MileageKM     *int64 `json:"mileage_km"`
	WorkPerformed string `json:"work_performed"`
}

type MaintenancePatch struct {
	Version       int64   `json:"version"`
	ServiceDate   *string `json:"service_date"`
	MileageKM     *int64  `json:"mileage_km"`
	WorkPerformed *string `json:"work_performed"`
}

type MaintenancePage struct {
	Entries    []MaintenanceEntry `json:"entries"`
	NextCursor string             `json:"next_cursor"`
}

// GarageOwner is locked account state for manager decisions, not a wire DTO.
type GarageOwner struct{ ID, Status string }
type MaintenanceCursor struct {
	Date string `json:"date"`
	ID   string `json:"id"`
}

func ParseMaintenanceDate(value string) (time.Time, error) {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil || len(value) != 10 || parsed.Year() < 1 {
		return time.Time{}, errors.New("invalid calendar date")
	}
	return parsed, nil
}
