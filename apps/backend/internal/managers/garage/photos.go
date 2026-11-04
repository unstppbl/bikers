package garage

import (
	"context"
	"errors"
	"io"
	"time"

	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/managers/photos"
	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
)

const maxBikePhotos = 6

func safePhoto(photo models.PhotoRecord) models.Photo {
	return models.Photo{ID: photo.ID, URL: "/api/v1/bikes/" + photo.BikeID + "/photos/" + photo.ID + "/content", Width: photo.Width, Height: photo.Height, CreatedAt: photo.CreatedAt}
}

func photoReadAccess(tx database.PhotoTransaction, session *models.Session) (bool, error) {
	if session != nil && session.User.ID == tx.Bike().OwnerID {
		if !approvedOwner(tx.Owner(), session.User) {
			return false, notFound()
		}
		return true, nil
	}
	hidden, err := tx.ContentHidden("bike", tx.Bike().ID)
	if err != nil {
		return false, err
	}
	if hidden {
		return false, notFound()
	}
	if tx.Bike().Visibility != "public" {
		return false, notFound()
	}
	return false, nil
}

func photoGallery(tx database.PhotoTransaction, owner bool) (models.PhotoGallery, error) {
	state := tx.GalleryState()
	gallery := models.PhotoGallery{Photos: []models.Photo{}, CoverPhotoID: state.CoverPhotoID}
	if owner {
		version := state.Version
		gallery.Version = &version
	}
	photos, err := tx.ReadyPhotos()
	if err != nil {
		return gallery, err
	}
	for _, photo := range photos {
		hidden, err := tx.ContentHidden("bike_photo", photo.ID)
		if err != nil {
			return gallery, err
		}
		if hidden && !owner {
			continue
		}
		gallery.Photos = append(gallery.Photos, safePhoto(photo))
	}
	if !owner {
		found := gallery.CoverPhotoID == ""
		for _, p := range gallery.Photos {
			if p.ID == gallery.CoverPhotoID {
				found = true
			}
		}
		if !found {
			gallery.CoverPhotoID = ""
		}
	}
	return gallery, nil
}

func (m *Manager) Photos(ctx context.Context, bikeID string, session *models.Session) (models.PhotoGallery, error) {
	bikeID, err := canonicalID(bikeID)
	if err != nil {
		return models.PhotoGallery{}, err
	}
	var gallery models.PhotoGallery
	err = m.repo.WithPhotoBike(ctx, bikeID, func(tx database.PhotoTransaction) error {
		owner, err := photoReadAccess(tx, session)
		if err != nil {
			return err
		}
		gallery, err = photoGallery(tx, owner)
		return err
	})
	return gallery, storageError(err)
}

func (m *Manager) PhotoContent(ctx context.Context, bikeID, photoID string, session *models.Session) (models.PhotoContent, error) {
	bikeID, err := canonicalID(bikeID)
	if err != nil {
		return models.PhotoContent{}, err
	}
	photoID, err = canonicalID(photoID)
	if err != nil {
		return models.PhotoContent{}, err
	}
	var content models.PhotoContent
	err = m.repo.WithPhotoBike(ctx, bikeID, func(tx database.PhotoTransaction) error {
		if _, err := photoReadAccess(tx, session); err != nil {
			return err
		}
		hidden, err := tx.ContentHidden("bike_photo", photoID)
		if err != nil {
			return err
		}
		if hidden {
			return notFound()
		}
		photo, err := tx.Photo(photoID, true)
		if err != nil {
			return err
		}
		if photo.State != "ready" {
			return notFound()
		}
		content.Bytes = photo.Content
		return nil
	})
	if err != nil {
		return models.PhotoContent{}, storageError(err)
	}
	return content, nil
}

func (m *Manager) UploadPhoto(ctx context.Context, actor models.User, bikeID string, reader io.Reader) (models.PhotoUploadResult, error) {
	if !actor.OnboardingComplete {
		return models.PhotoUploadResult{}, models.Fail("profile_required", "Complete your rider profile first")
	}
	bikeID, err := canonicalID(bikeID)
	if err != nil {
		return models.PhotoUploadResult{}, err
	}
	var result models.PhotoUploadResult
	err = m.photos.WithSlot(ctx, func(process photos.Process) error {
		var uploadErr error
		result, uploadErr = m.uploadPhoto(ctx, actor, bikeID, reader, process)
		return uploadErr
	})
	return result, err
}
func (m *Manager) uploadPhoto(ctx context.Context, actor models.User, bikeID string, reader io.Reader, process photos.Process) (models.PhotoUploadResult, error) {
	now := m.now().UTC()
	expires := now.Add(photos.ReservationTTL)
	pending := models.PhotoRecord{ID: uuid.NewString(), BikeID: bikeID, OwnerID: actor.ID, State: "pending", ReservedBytes: photos.ReservedBytes, CreatedAt: now, ExpiresAt: &expires}
	err := m.repo.WithPhotoBike(ctx, bikeID, func(tx database.PhotoTransaction) error {
		if err := ownBike(tx, actor); err != nil {
			return err
		}
		if err := tx.ExpirePending(now); err != nil {
			return err
		}
		if err := tx.PrunePhotoAttempts(now.Add(-photos.AttemptRetention)); err != nil {
			return err
		}
		quota, err := tx.PhotoQuota(now.Add(-time.Hour))
		if err != nil {
			return err
		}
		if quota.RecentAttempts >= photos.HourlyAttempts {
			return models.Fail("photo_rate_limited", "Too many photo upload attempts; try again later")
		}
		if quota.BikeCount >= maxBikePhotos {
			return models.Fail("photo_limit", "This motorcycle already has the maximum number of photos")
		}
		if quota.OwnerBytes > photos.OwnerByteLimit-photos.ReservedBytes || quota.GlobalBytes > photos.GlobalByteLimit-photos.ReservedBytes {
			return models.Fail("photo_storage_limit", "Photo storage limit reached")
		}
		if err := tx.RecordPhotoAttempt(now); err != nil {
			return err
		}
		return tx.CreatePendingPhoto(pending)
	})
	if err != nil {
		return models.PhotoUploadResult{}, storageError(err)
	}
	// A commit error below is ambiguous. Cleanup only ever removes pending rows.
	// A ready row must survive a lost response or an uncertain successful commit.
	completed := false
	defer func() {
		if !completed {
			m.cleanupPendingPhoto(bikeID, pending.ID, actor.ID)
		}
	}()
	image, err := process(ctx, reader)
	if err != nil {
		return models.PhotoUploadResult{}, err
	}
	if len(image.Bytes) == 0 || image.Width < 1 || image.Width > 1920 || image.Height < 1 || image.Height > 1920 {
		return models.PhotoUploadResult{}, models.Fail("invalid_photo", "Photo could not be processed")
	}
	if int64(len(image.Bytes)) > photos.ReservedBytes {
		return models.PhotoUploadResult{}, models.Fail("photo_too_large", "Processed photo exceeds the size limit")
	}
	var result models.PhotoUploadResult
	err = m.repo.WithPhotoBike(ctx, bikeID, func(tx database.PhotoTransaction) error {
		if err := ownBike(tx, actor); err != nil {
			return err
		}
		photo, err := tx.Photo(pending.ID, false)
		if errors.Is(err, database.ErrNotFound) {
			return models.Fail("photo_upload_expired", "Photo upload expired; try again")
		}
		if err != nil {
			return err
		}
		now := m.now().UTC()
		if photo.OwnerID != actor.ID || photo.State != "pending" || photo.ExpiresAt == nil || !now.Before(*photo.ExpiresAt) {
			return models.Fail("photo_upload_expired", "Photo upload expired; try again")
		}
		photo.State, photo.Content, photo.Width, photo.Height = "ready", image.Bytes, image.Width, image.Height
		photo.SizeBytes, photo.ReservedBytes, photo.ReadyAt, photo.ExpiresAt = int64(len(image.Bytes)), int64(len(image.Bytes)), &now, nil
		stored, err := tx.FinalizePhoto(photo)
		if err != nil {
			return err
		}
		gallery := tx.GalleryState()
		gallery.Version++
		if err = tx.SetPhotoGallery(gallery); err != nil {
			return err
		}
		result.Photo = safePhoto(stored)
		result.Gallery, err = photoGallery(tx, true)
		return err
	})
	if err != nil {
		return models.PhotoUploadResult{}, storageError(err)
	}
	completed = true
	return result, nil
}

func (m *Manager) cleanupPendingPhoto(bikeID, photoID, ownerID string) {
	ctx, cancel := context.WithTimeout(context.Background(), photos.CleanupTimeout)
	defer cancel()
	_ = m.repo.WithPhotoBike(ctx, bikeID, func(tx database.PhotoTransaction) error {
		photo, err := tx.Photo(photoID, false)
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if photo.State != "pending" || photo.OwnerID != ownerID {
			return nil
		}
		return tx.DeletePhoto(photoID, "pending")
	})
}

func (m *Manager) SetPhotoCover(ctx context.Context, actor models.User, bikeID string, input models.PhotoCoverInput) (models.PhotoGallery, error) {
	bikeID, err := canonicalID(bikeID)
	if err != nil {
		return models.PhotoGallery{}, err
	}
	if input.PhotoID != "" {
		id, err := uuid.Parse(input.PhotoID)
		if err != nil {
			return models.PhotoGallery{}, models.Fail("invalid_photo_choice", "Choose a ready photo belonging to this motorcycle")
		}
		input.PhotoID = id.String()
	}
	var gallery models.PhotoGallery
	err = m.repo.WithPhotoBike(ctx, bikeID, func(tx database.PhotoTransaction) error {
		if err := ownBike(tx, actor); err != nil {
			return err
		}
		state := tx.GalleryState()
		if input.Version != state.Version {
			return staleVersion()
		}
		if input.PhotoID != "" {
			hidden, err := tx.ContentHidden("bike_photo", input.PhotoID)
			if err != nil {
				return err
			}
			if hidden {
				return models.Fail("invalid_photo_choice", "Choose an available photo")
			}

			photo, err := tx.Photo(input.PhotoID, false)
			if errors.Is(err, database.ErrNotFound) || (err == nil && photo.State != "ready") {
				return models.Fail("invalid_photo_choice", "Choose a ready photo belonging to this motorcycle")
			}
			if err != nil {
				return err
			}
		}
		state.CoverPhotoID, state.Version = input.PhotoID, state.Version+1
		if err := tx.SetPhotoGallery(state); err != nil {
			return err
		}
		gallery, err = photoGallery(tx, true)
		return err
	})
	return gallery, storageError(err)
}

func (m *Manager) DeletePhoto(ctx context.Context, actor models.User, bikeID, photoID string, version int64) (models.PhotoGallery, error) {
	bikeID, err := canonicalID(bikeID)
	if err != nil {
		return models.PhotoGallery{}, err
	}
	photoID, err = canonicalID(photoID)
	if err != nil {
		return models.PhotoGallery{}, err
	}
	var gallery models.PhotoGallery
	err = m.repo.WithPhotoBike(ctx, bikeID, func(tx database.PhotoTransaction) error {
		if err := ownBike(tx, actor); err != nil {
			return err
		}
		state := tx.GalleryState()
		if version != state.Version {
			return staleVersion()
		}
		photo, err := tx.Photo(photoID, false)
		if err != nil {
			return err
		}
		if photo.State != "ready" {
			return notFound()
		}
		if state.CoverPhotoID == photoID {
			state.CoverPhotoID = ""
		}
		state.Version++
		if err := tx.SetPhotoGallery(state); err != nil {
			return err
		}
		if err := tx.DeletePhoto(photoID, "ready"); err != nil {
			return err
		}
		gallery, err = photoGallery(tx, true)
		return err
	})
	return gallery, storageError(err)
}
