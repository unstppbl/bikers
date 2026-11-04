package models

import "time"

type ClubImage struct {
	Image   *Photo `json:"image"`
	Version *int64 `json:"version,omitempty"`
}

// ClubImageRecord is private persistence state. OwnerID attributes storage to
// the original uploader and confers no image permissions after club transfer.
type ClubImageRecord struct {
	ID, ClubID, OwnerID, State string     `json:"-"`
	ReservedBytes, SizeBytes   int64      `json:"-"`
	Width, Height              int        `json:"-"`
	ClubVersion, ImageVersion  int64      `json:"-"`
	CreatedAt                  time.Time  `json:"-"`
	ReadyAt, ExpiresAt         *time.Time `json:"-"`
	Content                    []byte     `json:"-"`
}

type ClubImageState struct {
	Version        int64  `json:"-"`
	CurrentImageID string `json:"-"`
}

type ClubImageQuota struct {
	PendingCount            int   `json:"-"`
	OwnerBytes, GlobalBytes int64 `json:"-"`
	RecentAttempts          int   `json:"-"`
}
