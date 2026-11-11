package models

import "math"

// MapPoint is a user-confirmed label and WGS84 coordinate.
type MapPoint struct {
	Label string  `json:"label"`
	Lat   float64 `json:"lat"`
	Lon   float64 `json:"lon"`
}

// LineString follows GeoJSON's longitude, latitude coordinate order.
type LineString struct {
	Type        string       `json:"type"`
	Coordinates [][2]float64 `json:"coordinates"`
}

type MapPlan struct {
	Points          []MapPoint  `json:"points"`
	Geometry        *LineString `json:"geometry,omitempty"`
	DistanceMeters  float64     `json:"distance_meters"`
	DurationSeconds float64     `json:"duration_seconds"`
}

type MapPlace struct {
	ID    string  `json:"id"`
	Label string  `json:"label"`
	Lat   float64 `json:"lat"`
	Lon   float64 `json:"lon"`
	Kind  string  `json:"kind"`
}

type MapSearch struct {
	Query string    `json:"query"`
	Lang  string    `json:"lang"`
	Near  *MapPoint `json:"near,omitempty"`
}

type MapRouteInput struct {
	Points []MapPoint `json:"points"`
	Lang   string     `json:"lang"`
}

type MapConfig struct {
	Enabled    bool   `json:"enabled"`
	ArchiveURL string `json:"archive_url"`
}

func ValidCoordinate(lat, lon float64) bool {
	return !math.IsNaN(lat) && !math.IsInf(lat, 0) && !math.IsNaN(lon) && !math.IsInf(lon, 0) && lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180
}
