package database

import (
	"bikers.kz/backend/internal/models"
	"context"
	"time"
)

type RideCoverDecision func(RideCoverTransaction) error

// WithRideCover locks the trust and global photo quota keys, then the ride,
// then the actor account (SHARE). The callback owns authorization, quotas,
// time bounds and transitions.
type RideCoverRepository interface {
	WithRideCover(context.Context, string, string, RideCoverDecision) error
}

type RideCoverTransaction interface {
	Ride() models.Ride
	ActorStatus() string
	ActorParticipation() (string, error)
	ContentHidden(string, string) (bool, error)
	CoverState() models.RideCoverState
	Cover(string, bool) (models.RideCoverRecord, error)
	ExpirePending(time.Time) error
	PrunePhotoAttempts(time.Time) error
	CoverQuota(time.Time) (models.RideCoverQuota, error)
	RecordPhotoAttempt(time.Time) error
	CreatePendingCover(models.RideCoverRecord) error
	FinalizeCover(models.RideCoverRecord) (models.RideCoverRecord, error)
	DeleteCover(string, string) error
	SetCoverState(models.RideCoverState) error
}
