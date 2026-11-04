package media

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

func avatarPhoto(image models.AvatarRecord, url string) *models.Photo {
	return &models.Photo{ID: image.ID, URL: url, Width: image.Width, Height: image.Height, CreatedAt: image.CreatedAt}
}
func ownAvatarView(tx database.AvatarTransaction) (models.Avatar, error) {
	if err := active(tx.OwnerStatus()); err != nil {
		return models.Avatar{}, err
	}
	state := tx.AvatarState()
	version := state.Version
	out := models.Avatar{Version: &version}
	if state.CurrentID != "" {
		image, err := tx.Avatar(state.CurrentID, false)
		if err != nil {
			return out, err
		}
		if image.State != "ready" {
			return out, missing()
		}
		out.Image = avatarPhoto(image, "/api/v1/me/avatar/"+image.ID+"/content")
	}
	return out, nil
}
func (m *Manager) Avatar(ctx context.Context, actor models.User) (models.Avatar, error) {
	var out models.Avatar
	err := m.repo.WithAvatar(ctx, actor.ID, func(tx database.AvatarTransaction) error { var e error; out, e = ownAvatarView(tx); return e })
	return out, storageError(err)
}
func (m *Manager) AvatarContent(ctx context.Context, actor models.User, imageID string) (models.PhotoContent, error) {
	imageID, err := identifier(imageID)
	if err != nil {
		return models.PhotoContent{}, err
	}
	var out models.PhotoContent
	err = m.repo.WithAvatar(ctx, actor.ID, func(tx database.AvatarTransaction) error {
		if err := active(tx.OwnerStatus()); err != nil {
			return err
		}
		if tx.AvatarState().CurrentID != imageID {
			return missing()
		}
		hidden, err := tx.ContentHidden("user", actor.ID)
		if err != nil {
			return err
		}
		if hidden {
			return missing()
		}
		image, err := tx.Avatar(imageID, true)
		if err != nil {
			return err
		}
		if image.State != "ready" {
			return missing()
		}
		shown, err := visibleAuthor(tx, actor.ID, actor.ID, "avatar", image.ID)
		if err != nil {
			return err
		}
		if !shown {
			return missing()
		}
		out.Bytes = image.Content
		return nil
	})
	return out, storageError(err)
}
func (m *Manager) UploadAvatar(ctx context.Context, actor models.User, version int64, input io.Reader) (models.Avatar, error) {
	if err := complete(actor); err != nil {
		return models.Avatar{}, err
	}
	if version < 1 {
		return models.Avatar{}, invalidVersion()
	}
	var out models.Avatar
	err := m.photos.WithSlot(ctx, func(process photos.Process) error {
		var e error
		out, e = m.uploadAvatar(ctx, actor, version, input, process)
		return e
	})
	return out, err
}
func (m *Manager) uploadAvatar(ctx context.Context, actor models.User, version int64, input io.Reader, process photos.Process) (models.Avatar, error) {
	now := m.now().UTC()
	expires := now.Add(photos.ReservationTTL)
	pending := models.AvatarRecord{ID: uuid.NewString(), OwnerID: actor.ID, State: "pending", ReservedBytes: photos.ReservedBytes, AvatarVersion: version, CreatedAt: now, ExpiresAt: &expires}
	err := m.repo.WithAvatar(ctx, actor.ID, func(tx database.AvatarTransaction) error {
		if err := active(tx.OwnerStatus()); err != nil {
			return err
		}
		if tx.AvatarState().Version != version {
			return stale()
		}
		if err := tx.ExpirePending(now); err != nil {
			return err
		}
		if err := tx.PrunePhotoAttempts(now.Add(-photos.AttemptRetention)); err != nil {
			return err
		}
		quota, err := tx.Quota(now.Add(-time.Hour))
		if err != nil {
			return err
		}
		if err = quotaAllowed(quota); err != nil {
			return err
		}
		if quota.PendingCount > 0 {
			return models.Fail("avatar_upload_busy", "Another avatar upload is in progress")
		}
		if err = tx.RecordPhotoAttempt(now); err != nil {
			return err
		}
		return tx.CreatePending(pending)
	})
	if err != nil {
		return models.Avatar{}, storageError(err)
	}
	completed := false
	defer func() {
		if !completed {
			m.cleanupAvatar(actor.ID, pending.ID)
		}
	}()
	image, err := process(ctx, input)
	if err != nil {
		return models.Avatar{}, err
	}
	if err = processed(image); err != nil {
		return models.Avatar{}, err
	}
	var out models.Avatar
	err = m.repo.WithAvatar(ctx, actor.ID, func(tx database.AvatarTransaction) error {
		if err := active(tx.OwnerStatus()); err != nil {
			return err
		}
		state := tx.AvatarState()
		if state.Version != version {
			return stale()
		}
		stored, err := tx.Avatar(pending.ID, false)
		if errors.Is(err, database.ErrNotFound) {
			return models.Fail("photo_upload_expired", "Photo upload expired; try again")
		}
		if err != nil {
			return err
		}
		now := m.now().UTC()
		if stored.State != "pending" || stored.OwnerID != actor.ID || stored.AvatarVersion != version || stored.ExpiresAt == nil || !now.Before(*stored.ExpiresAt) {
			return models.Fail("photo_upload_expired", "Photo upload expired; try again")
		}
		if state.CurrentID != "" {
			if err = tx.Delete(state.CurrentID, "ready"); err != nil {
				return err
			}
		}
		stored.State, stored.Content, stored.Width, stored.Height = "ready", image.Bytes, image.Width, image.Height
		stored.SizeBytes, stored.ReservedBytes, stored.ReadyAt, stored.ExpiresAt = int64(len(image.Bytes)), int64(len(image.Bytes)), &now, nil
		if _, err = tx.Finalize(stored); err != nil {
			return err
		}
		state.CurrentID = stored.ID
		state.Version++
		if err = tx.SetAvatarState(state); err != nil {
			return err
		}
		out, err = ownAvatarView(tx)
		return err
	})
	if err != nil {
		return models.Avatar{}, storageError(err)
	}
	completed = true
	return out, nil
}
func (m *Manager) cleanupAvatar(owner, id string) {
	ctx, cancel := context.WithTimeout(context.Background(), photos.CleanupTimeout)
	defer cancel()
	_ = m.repo.WithAvatar(ctx, owner, func(tx database.AvatarTransaction) error {
		record, err := tx.Avatar(id, false)
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if record.State != "pending" || record.OwnerID != owner {
			return nil
		}
		return tx.Delete(id, "pending")
	})
}
func (m *Manager) RemoveAvatar(ctx context.Context, actor models.User, version int64) (models.Avatar, error) {
	if version < 1 {
		return models.Avatar{}, invalidVersion()
	}
	var out models.Avatar
	err := m.repo.WithAvatar(ctx, actor.ID, func(tx database.AvatarTransaction) error {
		if err := active(tx.OwnerStatus()); err != nil {
			return err
		}
		state := tx.AvatarState()
		if state.Version != version {
			return stale()
		}
		previous := state.CurrentID
		state.CurrentID = ""
		state.Version++
		if err := tx.SetAvatarState(state); err != nil {
			return err
		}
		if previous != "" {
			if err := tx.Delete(previous, "ready"); err != nil {
				return err
			}
		}
		var err error
		out, err = ownAvatarView(tx)
		return err
	})
	return out, storageError(err)
}

func (m *Manager) RideAvatar(ctx context.Context, actor models.User, ride, target string) (models.Avatar, error) {
	out, _, err := m.contextAvatar(ctx, actor, "ride", ride, target, "", false)
	return out, err
}
func (m *Manager) RideAvatarContent(ctx context.Context, actor models.User, ride, target, image string) (models.PhotoContent, error) {
	_, out, err := m.contextAvatar(ctx, actor, "ride", ride, target, image, true)
	return out, err
}
func (m *Manager) ClubAvatar(ctx context.Context, actor models.User, club, target string) (models.Avatar, error) {
	out, _, err := m.contextAvatar(ctx, actor, "club", club, target, "", false)
	return out, err
}
func (m *Manager) ClubAvatarContent(ctx context.Context, actor models.User, club, target, image string) (models.PhotoContent, error) {
	_, out, err := m.contextAvatar(ctx, actor, "club", club, target, image, true)
	return out, err
}
func (m *Manager) contextAvatar(ctx context.Context, actor models.User, kind, parent, target, imageID string, content bool) (models.Avatar, models.PhotoContent, error) {
	parent, err := identifier(parent)
	if err != nil {
		return models.Avatar{}, models.PhotoContent{}, err
	}
	target, err = identifier(target)
	if err != nil {
		return models.Avatar{}, models.PhotoContent{}, err
	}
	if content {
		imageID, err = identifier(imageID)
		if err != nil {
			return models.Avatar{}, models.PhotoContent{}, err
		}
	}
	var out models.Avatar
	var bytes models.PhotoContent
	load := m.repo.WithRideAvatar
	if kind == "club" {
		load = m.repo.WithClubAvatar
	}
	err = load(ctx, parent, actor.ID, target, func(tx database.AvatarContextTransaction) error {
		access := tx.Context()
		viewerApproved := access.ViewerState == "approved"
		targetApproved := access.TargetState == "approved"
		if kind == "ride" {
			viewerApproved = viewerApproved || access.ViewerID == access.OwnerID
			targetApproved = targetApproved || access.TargetID == access.OwnerID
		}
		if access.ViewerStatus != "active" || access.TargetStatus != "active" || !viewerApproved || !targetApproved {
			return missing()
		}
		blocked, err := tx.Blocked(actor.ID, target)
		if err != nil {
			return err
		}
		if blocked {
			return missing()
		}
		hiddenUser, err := tx.ContentHidden("user", target)
		if err != nil {
			return err
		}
		if hiddenUser {
			if content {
				return missing()
			}
			return nil
		}
		state := tx.AvatarState()
		if state.CurrentID == "" {
			if content {
				return missing()
			}
			return nil
		}
		if content && state.CurrentID != imageID {
			return missing()
		}
		image, err := tx.Avatar(state.CurrentID, content)
		if err != nil {
			return err
		}
		if image.State != "ready" {
			return missing()
		}
		hidden, err := tx.ContentHidden("avatar", image.ID)
		if err != nil {
			return err
		}
		if hidden {
			if content {
				return missing()
			}
			return nil
		}
		if content {
			bytes.Bytes = image.Content
		} else {
			out.Image = avatarPhoto(image, "/api/v1/"+map[string]string{"ride": "rides", "club": "clubs"}[kind]+"/"+parent+"/avatars/"+target+"/"+image.ID+"/content")
		}
		return nil
	})
	return out, bytes, storageError(err)
}
