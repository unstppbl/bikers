package database

import (
	"bikers.kz/backend/internal/models"
	"context"
	"time"
)

type AvatarDecision func(AvatarTransaction) error
type RideMediaDecision func(RideMediaTransaction) error
type AvatarContextDecision func(AvatarContextTransaction) error
type MediaRepository interface {
	WithAvatar(context.Context, string, AvatarDecision) error
	WithRideMedia(context.Context, string, string, RideMediaDecision) error
	WithRideAvatar(context.Context, string, string, string, AvatarContextDecision) error
	WithClubAvatar(context.Context, string, string, string, AvatarContextDecision) error
}
type MediaTrust interface {
	Blocked(string, string) (bool, error)
	ContentHidden(string, string) (bool, error)
}
type AvatarTransaction interface {
	MediaTrust
	OwnerStatus() string
	AvatarState() models.AvatarState
	Avatar(string, bool) (models.AvatarRecord, error)
	ExpirePending(time.Time) error
	PrunePhotoAttempts(time.Time) error
	Quota(time.Time) (models.MediaQuota, error)
	RecordPhotoAttempt(time.Time) error
	CreatePending(models.AvatarRecord) error
	Finalize(models.AvatarRecord) (models.AvatarRecord, error)
	Delete(string, string) error
	SetAvatarState(models.AvatarState) error
}
type AvatarContextTransaction interface {
	MediaTrust
	Context() models.AvatarContext
	AvatarState() models.AvatarState
	Avatar(string, bool) (models.AvatarRecord, error)
}
type RideMediaTransaction interface {
	MediaTrust
	Ride() models.Ride
	Participation() models.MediaParticipation
	AlbumVersion() int64
	SetAlbumVersion(int64) error
	Photos(int) ([]models.RidePhotoRecord, error)
	Photo(string, bool) (models.RidePhotoRecord, error)
	Videos(int) ([]models.RideVideoRecord, error)
	Video(string) (models.RideVideoRecord, error)
	ExpirePending(time.Time) error
	PrunePhotoAttempts(time.Time) error
	Quota(time.Time) (models.MediaQuota, error)
	RecordPhotoAttempt(time.Time) error
	CreatePendingPhoto(models.RidePhotoRecord) error
	FinalizePhoto(models.RidePhotoRecord) (models.RidePhotoRecord, error)
	DeletePhoto(string, string) error
	AddVideo(models.RideVideoRecord) error
	DeleteVideo(string) error
}
