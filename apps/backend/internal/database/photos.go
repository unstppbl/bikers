package database

import (
	"context"
	"time"

	"bikers.kz/backend/internal/models"
)

type PhotoDecision func(PhotoTransaction) error

// WithPhotoBike takes the global photo lock before the owner and bike locks.
// The callback chooses all quotas, time bounds, access and state transitions.
type PhotoRepository interface {
	WithPhotoBike(context.Context, string, PhotoDecision) error
}

type PhotoTransaction interface {
	BikeTransaction
	GalleryState() models.PhotoGalleryState
	ReadyPhotos() ([]models.PhotoRecord, error)
	Photo(string, bool) (models.PhotoRecord, error)
	ExpirePending(time.Time) error
	PrunePhotoAttempts(time.Time) error
	PhotoQuota(time.Time) (models.PhotoQuota, error)
	RecordPhotoAttempt(time.Time) error
	CreatePendingPhoto(models.PhotoRecord) error
	FinalizePhoto(models.PhotoRecord) (models.PhotoRecord, error)
	DeletePhoto(string, string) error
	SetPhotoGallery(models.PhotoGalleryState) error
}
