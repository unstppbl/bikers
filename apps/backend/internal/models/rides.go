package models

import (
	"math"
	"time"
)

// RideListInput contains the public discovery and personal ride-list filters.
type RideListInput struct {
	View, City, From, To, Search, Style, Surface, Cursor string
}

type RidePage struct {
	Rides      []RideSummary `json:"rides"`
	NextCursor string        `json:"next_cursor"`
}

func ValidRideStyle(value string) bool {
	switch value {
	case "", "chill", "touring", "sport", "enduro", "city":
		return true
	}
	return false
}

func ValidRideSurface(value string) bool {
	switch value {
	case "", "asphalt", "mixed", "unpaved":
		return true
	}
	return false
}

// RideTimeWindow maps a departure instant in its IANA timezone to a coarse public window
// so public previews never expose the exact departure minute.
func RideTimeWindow(startsAt time.Time, timezone string) string {
	if startsAt.IsZero() {
		return ""
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}
	hour := startsAt.In(loc).Hour()
	switch {
	case hour >= 5 && hour < 12:
		return "morning"
	case hour >= 12 && hour < 17:
		return "afternoon"
	case hour >= 17 && hour < 22:
		return "evening"
	default:
		return "night"
	}
}

// PublicRideDistanceKm returns explicit km or a coarse 5 km-rounded estimate from the map plan.
func PublicRideDistanceKm(explicitKm int, plan *MapPlan) int {
	if explicitKm > 0 {
		return explicitKm
	}
	if plan == nil || plan.DistanceMeters <= 0 {
		return 0
	}
	rounded := int(math.Round((float64(plan.DistanceMeters)/1000.0)/5.0) * 5.0)
	if rounded < 5 {
		return 5
	}
	return rounded
}
