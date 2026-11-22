package models

import (
	"math"
	"time"
)

// CommunityRouteInput creates a planned route or a snapshot of an owned track.
// Publication always needs explicit consent because every coordinate is public.
type CommunityRouteInput struct {
	Title          string     `json:"title"`
	City           string     `json:"city"`
	Description    string     `json:"description"`
	Surface        string     `json:"surface"`
	MapPoints      []MapPoint `json:"map_points"`
	TrackID        string     `json:"track_id"`
	Published      bool       `json:"published"`
	PublishConsent bool       `json:"publish_consent"`
	Version        int64      `json:"version"`
}

const MaxRouteDurationSeconds = 7 * 24 * 60 * 60

func IsRouteSurface(surface string) bool {
	switch surface {
	case "asphalt", "mixed", "unpaved", "unknown":
		return true
	default:
		return false
	}
}

// Summaries exclude exact points, geometry and the private track ID. A public
// result can include a small line drawing in thumbnail coordinates.
type CommunityRouteSummary struct {
	ID              string                 `json:"id"`
	OwnerID         string                 `json:"owner_id"`
	OwnerName       string                 `json:"owner_name"`
	Title           string                 `json:"title"`
	City            string                 `json:"city"`
	Description     string                 `json:"description"`
	Surface         string                 `json:"surface"`
	Kind            string                 `json:"kind"`
	Published       bool                   `json:"published"`
	Version         int64                  `json:"version"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
	DistanceMeters  float64                `json:"distance_meters"`
	DurationSeconds float64                `json:"duration_seconds"`
	PreviewSegments [][][2]float64         `json:"preview_segments,omitempty"`
	LatestCondition *RouteConditionSummary `json:"latest_condition,omitempty"`
}

type CommunityRoute struct {
	CommunityRouteSummary
	Points          []MapPoint     `json:"points"`
	Segments        [][][2]float64 `json:"segments"`
	Hidden          bool           `json:"-"`
	SourceTrackID   string         `json:"-"`
	PubliclyVisible bool           `json:"publicly_visible"`
}

type CommunityRoutePage struct {
	Routes     []CommunityRouteSummary `json:"routes"`
	NextCursor string                  `json:"next_cursor"`
}

// RoutePreview converts bounded WGS84 samples into a relative drawing on a
// square canvas. One scale preserves the line's aspect ratio, and neither
// latitude, longitude nor an absolute scale is carried in the result.
func RoutePreview(samples [][][2]float64) [][][2]float64 {
	minLon, maxLon := math.Inf(1), math.Inf(-1)
	minLat, maxLat := math.Inf(1), math.Inf(-1)
	for _, segment := range samples {
		for _, p := range segment {
			minLon, maxLon = math.Min(minLon, p[0]), math.Max(maxLon, p[0])
			minLat, maxLat = math.Min(minLat, p[1]), math.Max(maxLat, p[1])
		}
	}
	lonSpan, latSpan := maxLon-minLon, maxLat-minLat
	if lonSpan <= 0 && latSpan <= 0 {
		return nil
	}
	centerLon, centerLat := (minLon+maxLon)/2, (minLat+maxLat)/2
	lonScale := math.Cos(centerLat * math.Pi / 180)
	scale := math.Max(lonSpan*lonScale, latSpan)
	preview := make([][][2]float64, 0, len(samples))
	for _, segment := range samples {
		if len(segment) < 2 {
			continue
		}
		line := make([][2]float64, len(segment))
		for i, p := range segment {
			x := 0.5 + (p[0]-centerLon)*lonScale/scale
			y := 0.5 - (p[1]-centerLat)/scale
			line[i] = [2]float64{math.Round(x*1000) / 1000, math.Round(y*1000) / 1000}
		}
		preview = append(preview, line)
	}
	return preview
}

// TrackInput is recorded by the in-app GPS recorder. Segment boundaries retain
// pauses and lost-signal gaps; no line or distance connects different segments.
type TrackInput struct {
	Name            string         `json:"name"`
	Segments        [][][2]float64 `json:"segments"`
	DurationSeconds float64        `json:"duration_seconds"`
}

type Track struct {
	ID string `json:"id"`
	TrackInput
	OwnerID        string    `json:"-"`
	DistanceMeters float64   `json:"distance_meters"`
	Version        int64     `json:"version"`
	CreatedAt      time.Time `json:"created_at"`
}

type TrackPage struct {
	Tracks     []Track `json:"tracks"`
	NextCursor string  `json:"next_cursor"`
}

// TrackDistanceMeters uses WGS84 points in GeoJSON longitude/latitude order.
// Each segment is independent: pauses never invent a straight-line connection.
func TrackDistanceMeters(segments [][][2]float64) float64 {
	const radians = math.Pi / 180
	meters := 0.0
	for _, segment := range segments {
		for i := 1; i < len(segment); i++ {
			a, b := segment[i-1], segment[i]
			lat1, lat2 := a[1]*radians, b[1]*radians
			dlat, dlon := (b[1]-a[1])*radians, (b[0]-a[0])*radians
			h := math.Pow(math.Sin(dlat/2), 2) + math.Cos(lat1)*math.Cos(lat2)*math.Pow(math.Sin(dlon/2), 2)
			meters += 6371008.8 * 2 * math.Asin(math.Sqrt(math.Max(0, math.Min(1, h))))
		}
	}
	return meters
}

type RouteCollectionInput struct {
	Name    string `json:"name"`
	Version int64  `json:"version"`
}

type RouteCollection struct {
	ID        string    `json:"id"`
	OwnerID   string    `json:"-"`
	Name      string    `json:"name"`
	Version   int64     `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type RouteCollectionPage struct {
	Collections []RouteCollection `json:"collections"`
	NextCursor  string            `json:"next_cursor"`
}

type RouteConditionInput struct {
	Kind        string    `json:"kind"`
	Description string    `json:"description"`
	ObservedAt  time.Time `json:"observed_at"`
	Resolved    bool      `json:"resolved"`
	Version     int64     `json:"version"`
}

type RouteCondition struct {
	ID      string `json:"id"`
	RouteID string `json:"route_id"`
	RouteConditionInput
	AuthorID   string    `json:"author_id"`
	AuthorName string    `json:"author_name"`
	ExpiresAt  time.Time `json:"expires_at"`
	State      string    `json:"state"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	Hidden     bool      `json:"-"`
}
type RouteConditionSummary struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind"`
	ObservedAt  time.Time `json:"observed_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	AuthorName  string    `json:"author_name"`
	Description string    `json:"description,omitempty"`
}

type RouteConditionPage struct {
	Conditions []RouteCondition `json:"conditions"`
	NextCursor string           `json:"next_cursor"`
}
