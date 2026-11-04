package clubs

import (
	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/managers/photos"
	"bikers.kz/backend/internal/models"
	"context"
	"errors"
	"github.com/google/uuid"
	"io"
	"time"
)

func imageOwner(tx database.ClubImageTransaction, actor models.User) error {
	if tx.Club().OwnerID != actor.ID || tx.ActorStatus() != "active" {
		return models.Fail("forbidden", "Club owner access required")
	}
	if tx.Club().Status != "active" {
		return models.Fail("club_closed", "Club is closed")
	}
	return nil
}
func invalidImageVersion() error {
	return models.Fail("invalid_photo_choice", "A positive image version is required")
}
func imageView(tx database.ClubImageTransaction, actor string) (models.ClubImage, error) {
	hidden, err := tx.ContentHidden("club", tx.Club().ID)
	if err != nil {
		return models.ClubImage{}, err
	}
	if hidden {
		member, err := tx.ActorMembership()
		if err != nil {
			return models.ClubImage{}, err
		}
		if tx.ActorStatus() != "active" || (actor != tx.Club().OwnerID && member != "approved") {
			return models.ClubImage{}, database.ErrNotFound
		}
	}
	state := tx.ImageState()
	out := models.ClubImage{}
	if actor != "" && tx.Club().OwnerID == actor && tx.Club().Status == "active" && tx.ActorStatus() == "active" {
		version := state.Version
		out.Version = &version
	}
	if state.CurrentImageID != "" {
		hidden, err := tx.ContentHidden("club_image", state.CurrentImageID)
		if err != nil {
			return out, err
		}
		if hidden && actor != tx.Club().OwnerID {
			return out, nil
		}
		image, err := tx.Image(state.CurrentImageID, false)
		if err != nil {
			return out, err
		}
		if image.State != "ready" {
			return out, database.ErrNotFound
		}
		out.Image = &models.Photo{ID: image.ID, URL: "/api/v1/clubs/" + image.ClubID + "/image/" + image.ID + "/content", Width: image.Width, Height: image.Height, CreatedAt: image.CreatedAt}
	}
	return out, nil
}
func (m *Manager) Image(ctx context.Context, clubID string, session *models.Session) (models.ClubImage, error) {
	id, err := identifier(clubID)
	if err != nil {
		return models.ClubImage{}, err
	}
	actor := ""
	if session != nil {
		actor = session.User.ID
	}
	var out models.ClubImage
	err = m.repo.WithClubImage(ctx, id, actor, func(tx database.ClubImageTransaction) error { var e error; out, e = imageView(tx, actor); return e })
	return out, clubError(err)
}
func (m *Manager) ImageContent(ctx context.Context, clubID, imageID string, session *models.Session) (models.PhotoContent, error) {
	id, err := identifier(clubID)
	if err != nil {
		return models.PhotoContent{}, err
	}
	imageID, err = identifier(imageID)
	if err != nil {
		return models.PhotoContent{}, err
	}
	actor := ""
	if session != nil {
		actor = session.User.ID
	}
	var content models.PhotoContent
	err = m.repo.WithClubImage(ctx, id, actor, func(tx database.ClubImageTransaction) error {
		if _, err := imageView(tx, actor); err != nil {
			return err
		}
		hidden, err := tx.ContentHidden("club_image", imageID)
		if err != nil {
			return err
		}
		if hidden {
			return database.ErrNotFound
		}
		if tx.ImageState().CurrentImageID != imageID {
			return database.ErrNotFound
		}
		image, err := tx.Image(imageID, true)
		if err != nil {
			return err
		}
		if image.State != "ready" {
			return database.ErrNotFound
		}
		content.Bytes = image.Content
		return nil
	})
	return content, clubError(err)
}
func (m *Manager) UploadImage(ctx context.Context, actor models.User, clubID string, version int64, reader io.Reader) (models.ClubImage, error) {
	if !actor.OnboardingComplete {
		return models.ClubImage{}, models.Fail("profile_required", "Complete your rider profile first")
	}
	id, err := identifier(clubID)
	if err != nil {
		return models.ClubImage{}, err
	}
	if version < 1 {
		return models.ClubImage{}, invalidImageVersion()
	}
	var out models.ClubImage
	err = m.photos.WithSlot(ctx, func(process photos.Process) error {
		var e error
		out, e = m.uploadImage(ctx, actor, id, version, reader, process)
		return e
	})
	return out, err
}
func (m *Manager) uploadImage(ctx context.Context, actor models.User, id string, version int64, reader io.Reader, process photos.Process) (models.ClubImage, error) {
	now := m.now().UTC()
	expires := now.Add(photos.ReservationTTL)
	pending := models.ClubImageRecord{ID: uuid.NewString(), ClubID: id, OwnerID: actor.ID, State: "pending", ReservedBytes: photos.ReservedBytes, CreatedAt: now, ExpiresAt: &expires}
	err := m.repo.WithClubImage(ctx, id, actor.ID, func(tx database.ClubImageTransaction) error {
		if err := imageOwner(tx, actor); err != nil {
			return err
		}
		state := tx.ImageState()
		if version != state.Version {
			return models.Fail("stale_version", "Club image changed; refresh and retry")
		}
		if err := tx.ExpirePending(now); err != nil {
			return err
		}
		if err := tx.PrunePhotoAttempts(now.Add(-photos.AttemptRetention)); err != nil {
			return err
		}
		quota, err := tx.ImageQuota(now.Add(-time.Hour))
		if err != nil {
			return err
		}
		if quota.RecentAttempts >= photos.HourlyAttempts {
			return models.Fail("photo_rate_limited", "Too many photo upload attempts; try again later")
		}
		if quota.PendingCount > 0 {
			return models.Fail("club_image_busy", "Another club image upload is in progress")
		}
		if quota.OwnerBytes > photos.OwnerByteLimit-photos.ReservedBytes || quota.GlobalBytes > photos.GlobalByteLimit-photos.ReservedBytes {
			return models.Fail("photo_storage_limit", "Photo storage limit reached")
		}
		pending.ClubVersion, pending.ImageVersion = tx.Club().Version, state.Version
		if err := tx.RecordPhotoAttempt(now); err != nil {
			return err
		}
		return tx.CreatePendingImage(pending)
	})
	if err != nil {
		return models.ClubImage{}, clubError(err)
	}
	completed := false
	defer func() {
		if !completed {
			m.cleanupPendingImage(id, pending.ID, actor.ID)
		}
	}()
	image, err := process(ctx, reader)
	if err != nil {
		return models.ClubImage{}, err
	}
	if len(image.Bytes) == 0 || image.Width < 1 || image.Width > 1920 || image.Height < 1 || image.Height > 1920 {
		return models.ClubImage{}, models.Fail("invalid_photo", "Photo could not be processed")
	}
	if int64(len(image.Bytes)) > photos.ReservedBytes {
		return models.ClubImage{}, models.Fail("photo_too_large", "Processed photo exceeds the size limit")
	}
	var out models.ClubImage
	err = m.repo.WithClubImage(ctx, id, actor.ID, func(tx database.ClubImageTransaction) error {
		if err := imageOwner(tx, actor); err != nil {
			return err
		}
		state := tx.ImageState()
		if state.Version != pending.ImageVersion || tx.Club().Version != pending.ClubVersion {
			return models.Fail("stale_version", "Club or image changed; refresh and retry")
		}
		stored, err := tx.Image(pending.ID, false)
		if errors.Is(err, database.ErrNotFound) {
			return models.Fail("photo_upload_expired", "Photo upload expired; try again")
		}
		if err != nil {
			return err
		}
		now := m.now().UTC()
		if stored.OwnerID != actor.ID || stored.State != "pending" || stored.ExpiresAt == nil || !now.Before(*stored.ExpiresAt) || stored.ClubVersion != pending.ClubVersion || stored.ImageVersion != pending.ImageVersion {
			return models.Fail("photo_upload_expired", "Photo upload expired; try again")
		}
		if state.CurrentImageID != "" {
			if err := tx.DeleteImage(state.CurrentImageID, "ready"); err != nil {
				return err
			}
		}
		stored.State, stored.Content, stored.Width, stored.Height = "ready", image.Bytes, image.Width, image.Height
		stored.SizeBytes, stored.ReservedBytes, stored.ReadyAt, stored.ExpiresAt = int64(len(image.Bytes)), int64(len(image.Bytes)), &now, nil
		if _, err = tx.FinalizeImage(stored); err != nil {
			return err
		}
		state.CurrentImageID, state.Version = stored.ID, state.Version+1
		if err = tx.SetImageState(state); err != nil {
			return err
		}
		out, err = imageView(tx, actor.ID)
		return err
	})
	if err != nil {
		return models.ClubImage{}, clubError(err)
	}
	completed = true
	return out, nil
}
func (m *Manager) cleanupPendingImage(clubID, imageID, owner string) {
	ctx, cancel := context.WithTimeout(context.Background(), photos.CleanupTimeout)
	defer cancel()
	_ = m.repo.WithClubImage(ctx, clubID, "", func(tx database.ClubImageTransaction) error {
		image, err := tx.Image(imageID, false)
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if image.State != "pending" || image.OwnerID != owner {
			return nil
		}
		return tx.DeleteImage(imageID, "pending")
	})
}
func (m *Manager) RemoveImage(ctx context.Context, actor models.User, clubID string, version int64) (models.ClubImage, error) {
	id, err := identifier(clubID)
	if err != nil {
		return models.ClubImage{}, err
	}
	if version < 1 {
		return models.ClubImage{}, invalidImageVersion()
	}
	var out models.ClubImage
	err = m.repo.WithClubImage(ctx, id, actor.ID, func(tx database.ClubImageTransaction) error {
		if err := imageOwner(tx, actor); err != nil {
			return err
		}
		state := tx.ImageState()
		if state.Version != version {
			return models.Fail("stale_version", "Club image changed; refresh and retry")
		}
		previous := state.CurrentImageID
		state.CurrentImageID = ""
		state.Version++
		if err := tx.SetImageState(state); err != nil {
			return err
		}
		if previous != "" {
			if err := tx.DeleteImage(previous, "ready"); err != nil {
				return err
			}
		}
		var err error
		out, err = imageView(tx, actor.ID)
		return err
	})
	return out, clubError(err)
}
