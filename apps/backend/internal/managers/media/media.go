// Package media owns profile avatars, contextual avatar access and ride albums.
package media

import (
	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/managers/photos"
	"bikers.kz/backend/internal/models"
	"errors"
	"github.com/google/uuid"
	"time"
)

const MaxRidePhotos = 24
const MaxUploaderPhotos = 6
const MaxRideVideos = 12
const MaxUploaderVideos = 3

type Manager struct {
	repo   database.MediaRepository
	now    func() time.Time
	photos *photos.Manager
}

func New(repo database.MediaRepository, now func() time.Time, processor *photos.Manager) *Manager {
	if now == nil {
		now = time.Now
	}
	return &Manager{repo: repo, now: now, photos: processor}
}
func missing() error { return models.Fail("not_found", "Media not found") }
func stale() error   { return models.Fail("stale_version", "Media changed; refresh and retry") }
func invalidVersion() error {
	return models.Fail("invalid_photo_choice", "A positive media version is required")
}
func identifier(value string) (string, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return "", missing()
	}
	return id.String(), nil
}
func storageError(err error) error {
	if errors.Is(err, database.ErrNotFound) {
		return missing()
	}
	if errors.Is(err, database.ErrConflict) {
		return stale()
	}
	return err
}
func active(status string) error {
	if status != "active" {
		return missing()
	}
	return nil
}
func complete(actor models.User) error {
	if !actor.OnboardingComplete {
		return models.Fail("profile_required", "Complete your rider profile first")
	}
	return nil
}
func quotaAllowed(quota models.MediaQuota) error {
	if quota.RecentAttempts >= photos.HourlyAttempts {
		return models.Fail("photo_rate_limited", "Too many photo upload attempts; try again later")
	}
	if quota.OwnerBytes > photos.OwnerByteLimit-photos.ReservedBytes || quota.GlobalBytes > photos.GlobalByteLimit-photos.ReservedBytes {
		return models.Fail("photo_storage_limit", "Photo storage limit reached")
	}
	return nil
}
func processed(image models.ProcessedPhoto) error {
	if len(image.Bytes) == 0 || image.Width < 1 || image.Width > 1920 || image.Height < 1 || image.Height > 1920 {
		return models.Fail("invalid_photo", "Photo could not be processed")
	}
	if int64(len(image.Bytes)) > photos.ReservedBytes {
		return models.Fail("photo_too_large", "Processed photo exceeds the size limit")
	}
	return nil
}
func visibleAuthor(tx database.MediaTrust, actor, owner, kind, id string) (bool, error) {
	if actor != owner {
		blocked, err := tx.Blocked(actor, owner)
		if err != nil {
			return false, err
		}
		if blocked {
			return false, nil
		}
	}
	hidden, err := tx.ContentHidden(kind, id)
	return !hidden, err
}
