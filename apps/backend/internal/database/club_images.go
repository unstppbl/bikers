package database

import (
	"bikers.kz/backend/internal/models"
	"context"
	"time"
)

type ClubImageDecision func(ClubImageTransaction) error

// WithClubImage locks global photo quota, then club, then the actor account
// (SHARE). The callback owns authorization, quotas, time bounds and transitions.
type ClubImageRepository interface {
	WithClubImage(context.Context, string, string, ClubImageDecision) error
}

type ClubImageTransaction interface {
	Club() models.Club
	ActorStatus() string
	ActorMembership() (string, error)
	ContentHidden(string, string) (bool, error)
	ImageState() models.ClubImageState
	Image(string, bool) (models.ClubImageRecord, error)
	ExpirePending(time.Time) error
	PrunePhotoAttempts(time.Time) error
	ImageQuota(time.Time) (models.ClubImageQuota, error)
	RecordPhotoAttempt(time.Time) error
	CreatePendingImage(models.ClubImageRecord) error
	FinalizeImage(models.ClubImageRecord) (models.ClubImageRecord, error)
	DeleteImage(string, string) error
	SetImageState(models.ClubImageState) error
}
