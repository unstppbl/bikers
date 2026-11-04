package models

import "time"

// Photo is the complete safe projection of an owner's processed bike photo.
type Photo struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	CreatedAt time.Time `json:"created_at"`
}

type PhotoGallery struct {
	Photos       []Photo `json:"photos"`
	CoverPhotoID string  `json:"cover_photo_id"`
	Version      *int64  `json:"version,omitempty"`
}

type PhotoUploadResult struct {
	Photo   Photo        `json:"photo"`
	Gallery PhotoGallery `json:"gallery"`
}

type PhotoCoverInput struct {
	Version int64  `json:"version"`
	PhotoID string `json:"photo_id"`
}

// PhotoRecord and the following state types are persistence values, never wire DTOs.
type PhotoRecord struct {
	ID, BikeID, OwnerID, State string     `json:"-"`
	ReservedBytes, SizeBytes   int64      `json:"-"`
	Width, Height              int        `json:"-"`
	CreatedAt                  time.Time  `json:"-"`
	ReadyAt, ExpiresAt         *time.Time `json:"-"`
	Content                    []byte     `json:"-"`
}

type PhotoGalleryState struct {
	Version      int64  `json:"-"`
	CoverPhotoID string `json:"-"`
}

type PhotoQuota struct {
	BikeCount               int   `json:"-"`
	OwnerBytes, GlobalBytes int64 `json:"-"`
	RecentAttempts          int   `json:"-"`
}

type PhotoContent struct {
	Bytes []byte `json:"-"`
}

// ProcessedPhoto contains normalized pixels encoded as a fresh JPEG. It is never
// a JSON projection; transports serve the bytes only through content endpoints.
type ProcessedPhoto struct {
	Bytes  []byte `json:"-"`
	Width  int    `json:"-"`
	Height int    `json:"-"`
}
