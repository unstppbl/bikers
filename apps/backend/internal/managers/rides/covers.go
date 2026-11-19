package rides

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

// coverEditor applies the ride edit rules to cover changes.
func coverEditor(tx database.RideCoverTransaction, actor models.User) error {
	if tx.ActorStatus() != "active" {
		return models.Fail("account_unavailable", "Account unavailable")
	}
	return rideEditor(tx.Ride(), actor.ID)
}

// coverVisible follows ride visibility: the cover is public wherever the
// ride's public summary is. A moderated ride stays visible only to its
// organizer and approved participants with active accounts.
func coverVisible(tx database.RideCoverTransaction, actor string) error {
	hidden, err := tx.ContentHidden("ride", tx.Ride().ID)
	if err != nil || !hidden {
		return err
	}
	if actor == "" || tx.ActorStatus() != "active" {
		return database.ErrNotFound
	}
	if actor == tx.Ride().OrganizerID {
		return nil
	}
	state, err := tx.ActorParticipation()
	if err != nil {
		return err
	}
	if state != "approved" {
		return database.ErrNotFound
	}
	return nil
}

func invalidCoverVersion() error {
	return models.Fail("invalid_photo_choice", "A positive cover version is required")
}

func coverView(tx database.RideCoverTransaction, actor string) (models.RideCover, error) {
	if err := coverVisible(tx, actor); err != nil {
		return models.RideCover{}, err
	}
	state := tx.CoverState()
	out := models.RideCover{}
	if actor != "" && tx.ActorStatus() == "active" && rideEditor(tx.Ride(), actor) == nil {
		version := state.Version
		out.Version = &version
	}
	if state.CurrentCoverID == "" {
		return out, nil
	}
	hidden, err := tx.ContentHidden("ride_cover", state.CurrentCoverID)
	if err != nil {
		return out, err
	}
	// The organizer still sees a moderated cover so it can be replaced or
	// removed; its content stays unavailable to everyone.
	if hidden && actor != tx.Ride().OrganizerID {
		return out, nil
	}
	cover, err := tx.Cover(state.CurrentCoverID, false)
	if err != nil {
		return out, err
	}
	if cover.State != "ready" {
		return out, database.ErrNotFound
	}
	out.Image = models.RideCoverPhoto(cover)
	return out, nil
}

func sessionActor(session *models.Session) string {
	if session == nil {
		return ""
	}
	return session.User.ID
}

func (m *Manager) Cover(ctx context.Context, rideID string, session *models.Session) (models.RideCover, error) {
	id, err := canonicalID(rideID)
	if err != nil {
		return models.RideCover{}, err
	}
	actor := sessionActor(session)
	var out models.RideCover
	err = m.repo.WithRideCover(ctx, id, actor, func(tx database.RideCoverTransaction) error {
		var e error
		out, e = coverView(tx, actor)
		return e
	})
	return out, rideError(err)
}

func (m *Manager) CoverContent(ctx context.Context, rideID, coverID string, session *models.Session) (models.PhotoContent, error) {
	id, err := canonicalID(rideID)
	if err != nil {
		return models.PhotoContent{}, err
	}
	coverID, err = canonicalID(coverID)
	if err != nil {
		return models.PhotoContent{}, err
	}
	actor := sessionActor(session)
	var content models.PhotoContent
	err = m.repo.WithRideCover(ctx, id, actor, func(tx database.RideCoverTransaction) error {
		if err := coverVisible(tx, actor); err != nil {
			return err
		}
		hidden, err := tx.ContentHidden("ride_cover", coverID)
		if err != nil {
			return err
		}
		if hidden || tx.CoverState().CurrentCoverID != coverID {
			return database.ErrNotFound
		}
		cover, err := tx.Cover(coverID, true)
		if err != nil {
			return err
		}
		if cover.State != "ready" {
			return database.ErrNotFound
		}
		content.Bytes = cover.Content
		return nil
	})
	return content, rideError(err)
}

func (m *Manager) UploadCover(ctx context.Context, actor models.User, rideID string, version int64, reader io.Reader) (models.RideCover, error) {
	if !actor.OnboardingComplete {
		return models.RideCover{}, models.Fail("profile_required", "Complete your rider profile first")
	}
	id, err := canonicalID(rideID)
	if err != nil {
		return models.RideCover{}, err
	}
	if version < 1 {
		return models.RideCover{}, invalidCoverVersion()
	}
	var out models.RideCover
	err = m.photos.WithSlot(ctx, func(process photos.Process) error {
		var e error
		out, e = m.uploadCover(ctx, actor, id, version, reader, process)
		return e
	})
	if err != nil {
		return models.RideCover{}, err
	}
	m.publish(id, "ride.updated")
	return out, nil
}

// uploadCover reserves quota under locks, processes the image without locks,
// then revalidates the organizer, the ride and the cover version before
// replacing the current cover.
func (m *Manager) uploadCover(ctx context.Context, actor models.User, id string, version int64, reader io.Reader, process photos.Process) (models.RideCover, error) {
	now := m.now().UTC()
	expires := now.Add(photos.ReservationTTL)
	pending := models.RideCoverRecord{ID: uuid.NewString(), RideID: id, OwnerID: actor.ID, State: "pending", ReservedBytes: photos.ReservedBytes, CreatedAt: now, ExpiresAt: &expires}
	err := m.repo.WithRideCover(ctx, id, actor.ID, func(tx database.RideCoverTransaction) error {
		if err := coverEditor(tx, actor); err != nil {
			return err
		}
		state := tx.CoverState()
		if version != state.Version {
			return models.Fail("stale_version", "Ride cover changed; refresh and retry")
		}
		if err := tx.ExpirePending(now); err != nil {
			return err
		}
		if err := tx.PrunePhotoAttempts(now.Add(-photos.AttemptRetention)); err != nil {
			return err
		}
		quota, err := tx.CoverQuota(now.Add(-time.Hour))
		if err != nil {
			return err
		}
		if quota.RecentAttempts >= photos.HourlyAttempts {
			return models.Fail("photo_rate_limited", "Too many photo upload attempts; try again later")
		}
		if quota.PendingCount > 0 {
			return models.Fail("ride_cover_busy", "Another ride cover upload is in progress")
		}
		if quota.OwnerBytes > photos.OwnerByteLimit-photos.ReservedBytes || quota.GlobalBytes > photos.GlobalByteLimit-photos.ReservedBytes {
			return models.Fail("photo_storage_limit", "Photo storage limit reached")
		}
		pending.CoverVersion = state.Version
		if err := tx.RecordPhotoAttempt(now); err != nil {
			return err
		}
		return tx.CreatePendingCover(pending)
	})
	if err != nil {
		return models.RideCover{}, rideError(err)
	}
	completed := false
	defer func() {
		if !completed {
			m.cleanupPendingCover(id, pending.ID, actor.ID)
		}
	}()
	image, err := process(ctx, reader)
	if err != nil {
		return models.RideCover{}, err
	}
	if len(image.Bytes) == 0 || image.Width < 1 || image.Width > 1920 || image.Height < 1 || image.Height > 1920 {
		return models.RideCover{}, models.Fail("invalid_photo", "Photo could not be processed")
	}
	if int64(len(image.Bytes)) > photos.ReservedBytes {
		return models.RideCover{}, models.Fail("photo_too_large", "Processed photo exceeds the size limit")
	}
	var out models.RideCover
	err = m.repo.WithRideCover(ctx, id, actor.ID, func(tx database.RideCoverTransaction) error {
		if err := coverEditor(tx, actor); err != nil {
			return err
		}
		state := tx.CoverState()
		if state.Version != pending.CoverVersion {
			return models.Fail("stale_version", "Ride cover changed; refresh and retry")
		}
		stored, err := tx.Cover(pending.ID, false)
		if errors.Is(err, database.ErrNotFound) {
			return models.Fail("photo_upload_expired", "Photo upload expired; try again")
		}
		if err != nil {
			return err
		}
		now := m.now().UTC()
		if stored.OwnerID != actor.ID || stored.State != "pending" || stored.ExpiresAt == nil || !now.Before(*stored.ExpiresAt) || stored.CoverVersion != pending.CoverVersion {
			return models.Fail("photo_upload_expired", "Photo upload expired; try again")
		}
		if state.CurrentCoverID != "" {
			if err := tx.DeleteCover(state.CurrentCoverID, "ready"); err != nil {
				return err
			}
		}
		stored.State, stored.Content, stored.Width, stored.Height = "ready", image.Bytes, image.Width, image.Height
		stored.SizeBytes, stored.ReservedBytes, stored.ReadyAt, stored.ExpiresAt = int64(len(image.Bytes)), int64(len(image.Bytes)), &now, nil
		if _, err = tx.FinalizeCover(stored); err != nil {
			return err
		}
		state.CurrentCoverID, state.Version = stored.ID, state.Version+1
		if err = tx.SetCoverState(state); err != nil {
			return err
		}
		out, err = coverView(tx, actor.ID)
		return err
	})
	if err != nil {
		return models.RideCover{}, rideError(err)
	}
	completed = true
	return out, nil
}

// cleanupPendingCover deletes only this upload's reservation. It must keep
// ready content after an uncertain successful commit.
func (m *Manager) cleanupPendingCover(rideID, coverID, owner string) {
	ctx, cancel := context.WithTimeout(context.Background(), photos.CleanupTimeout)
	defer cancel()
	_ = m.repo.WithRideCover(ctx, rideID, "", func(tx database.RideCoverTransaction) error {
		cover, err := tx.Cover(coverID, false)
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if cover.State != "pending" || cover.OwnerID != owner {
			return nil
		}
		return tx.DeleteCover(coverID, "pending")
	})
}

func (m *Manager) RemoveCover(ctx context.Context, actor models.User, rideID string, version int64) (models.RideCover, error) {
	id, err := canonicalID(rideID)
	if err != nil {
		return models.RideCover{}, err
	}
	if version < 1 {
		return models.RideCover{}, invalidCoverVersion()
	}
	var out models.RideCover
	err = m.repo.WithRideCover(ctx, id, actor.ID, func(tx database.RideCoverTransaction) error {
		if err := coverEditor(tx, actor); err != nil {
			return err
		}
		state := tx.CoverState()
		if state.Version != version {
			return models.Fail("stale_version", "Ride cover changed; refresh and retry")
		}
		previous := state.CurrentCoverID
		state.CurrentCoverID = ""
		state.Version++
		if err := tx.SetCoverState(state); err != nil {
			return err
		}
		if previous != "" {
			if err := tx.DeleteCover(previous, "ready"); err != nil {
				return err
			}
		}
		var err error
		out, err = coverView(tx, actor.ID)
		return err
	})
	if err != nil {
		return models.RideCover{}, rideError(err)
	}
	m.publish(id, "ride.updated")
	return out, nil
}
