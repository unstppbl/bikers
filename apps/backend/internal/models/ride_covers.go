package models

import "time"

// RideCover is the public cover metadata. Version is present only for the
// organizer while the ride still accepts cover changes.
type RideCover struct {
	Image   *Photo `json:"image"`
	Version *int64 `json:"version,omitempty"`
}

// RideCoverRecord is private persistence state. OwnerID attributes storage to
// the uploader and confers no cover permissions.
type RideCoverRecord struct {
	ID, RideID, OwnerID, State string     `json:"-"`
	ReservedBytes, SizeBytes   int64      `json:"-"`
	Width, Height              int        `json:"-"`
	CoverVersion               int64      `json:"-"`
	CreatedAt                  time.Time  `json:"-"`
	ReadyAt, ExpiresAt         *time.Time `json:"-"`
	Content                    []byte     `json:"-"`
}

type RideCoverState struct {
	Version        int64  `json:"-"`
	CurrentCoverID string `json:"-"`
}

type RideCoverQuota struct {
	PendingCount            int   `json:"-"`
	OwnerBytes, GlobalBytes int64 `json:"-"`
	RecentAttempts          int   `json:"-"`
}

// RideCoverPhoto is the safe public projection of a ready ride cover.
func RideCoverPhoto(cover RideCoverRecord) *Photo {
	return &Photo{ID: cover.ID, URL: "/api/v1/rides/" + cover.RideID + "/cover/" + cover.ID + "/content", Width: cover.Width, Height: cover.Height, CreatedAt: cover.CreatedAt}
}
