package models

import "time"

type Avatar struct {
	Image   *Photo `json:"image"`
	Version *int64 `json:"version,omitempty"`
}
type RidePhoto struct {
	Photo
	UploaderID string `json:"uploader_id"`
	CanDelete  bool   `json:"can_delete"`
	Hidden     bool   `json:"hidden,omitempty"`
}
type RideVideo struct {
	ID         string    `json:"id"`
	URL        string    `json:"url"`
	Title      string    `json:"title"`
	UploaderID string    `json:"uploader_id"`
	CreatedAt  time.Time `json:"created_at"`
	CanDelete  bool      `json:"can_delete"`
	Hidden     bool      `json:"hidden,omitempty"`
}
type RideAlbum struct {
	Photos  []RidePhoto `json:"photos"`
	Videos  []RideVideo `json:"videos"`
	Version int64       `json:"version"`
}
type RidePhotoUpload struct {
	Photo RidePhoto `json:"photo"`
	Album RideAlbum `json:"album"`
}
type RideVideoInput struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

// The remaining types are internal snapshots; none serialize persistence data.
type AvatarState struct {
	Version   int64  `json:"-"`
	CurrentID string `json:"-"`
}
type AvatarRecord struct {
	ID, OwnerID, State                      string     `json:"-"`
	ReservedBytes, SizeBytes, AvatarVersion int64      `json:"-"`
	Width, Height                           int        `json:"-"`
	CreatedAt                               time.Time  `json:"-"`
	ReadyAt, ExpiresAt                      *time.Time `json:"-"`
	Content                                 []byte     `json:"-"`
}
type RidePhotoRecord struct {
	ID, RideID, OwnerID, State            string     `json:"-"`
	ReservedBytes, SizeBytes              int64      `json:"-"`
	Width, Height                         int        `json:"-"`
	ParticipationVersion, RidePlanVersion int64      `json:"-"`
	CreatedAt                             time.Time  `json:"-"`
	ReadyAt, ExpiresAt                    *time.Time `json:"-"`
	Content                               []byte     `json:"-"`
}
type RideVideoRecord struct {
	ID, RideID, OwnerID, URL, Title string    `json:"-"`
	CreatedAt                       time.Time `json:"-"`
}
type MediaQuota struct {
	PendingCount, RideCount, UploaderCount, RecentAttempts int   `json:"-"`
	OwnerBytes, GlobalBytes                                int64 `json:"-"`
}
type MediaParticipation struct {
	State, AccountStatus string `json:"-"`
	Version              int64  `json:"-"`
}
type AvatarContext struct {
	Kind, ParentID, OwnerID, ViewerID, TargetID          string `json:"-"`
	ViewerState, TargetState, ViewerStatus, TargetStatus string `json:"-"`
}
