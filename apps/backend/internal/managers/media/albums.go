package media

import (
	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/managers/photos"
	"bikers.kz/backend/internal/models"
	"context"
	"errors"
	"github.com/google/uuid"
	"io"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func albumAccess(tx database.RideMediaTransaction, actor string) error {
	p := tx.Participation()
	if p.AccountStatus != "active" || (p.State != "approved" && tx.Ride().OrganizerID != actor) {
		return missing()
	}
	return nil
}
func albumWritable(tx database.RideMediaTransaction, actor string) error {
	if err := albumAccess(tx, actor); err != nil {
		return err
	}
	if tx.Ride().Status == "cancelled" {
		return models.Fail("ride_cancelled", "Ride is cancelled")
	}
	return nil
}
func albumPhoto(photo models.RidePhotoRecord, actor, organizer string) models.RidePhoto {
	return models.RidePhoto{Photo: models.Photo{ID: photo.ID, URL: "/api/v1/rides/" + photo.RideID + "/album/photos/" + photo.ID + "/content", Width: photo.Width, Height: photo.Height, CreatedAt: photo.CreatedAt}, UploaderID: photo.OwnerID, CanDelete: actor == photo.OwnerID || actor == organizer}
}
func albumView(tx database.RideMediaTransaction, actor string) (models.RideAlbum, error) {
	if err := albumAccess(tx, actor); err != nil {
		return models.RideAlbum{}, err
	}
	out := models.RideAlbum{Photos: []models.RidePhoto{}, Videos: []models.RideVideo{}, Version: tx.AlbumVersion()}
	images, err := tx.Photos(MaxRidePhotos)
	if err != nil {
		return out, err
	}
	for _, image := range images {
		if image.OwnerID != actor {
			blocked, err := tx.Blocked(actor, image.OwnerID)
			if err != nil {
				return out, err
			}
			if blocked {
				continue
			}
		}
		photo := albumPhoto(image, actor, tx.Ride().OrganizerID)
		hidden, err := tx.ContentHidden("ride_photo", image.ID)
		if err != nil {
			return out, err
		}
		if hidden {
			if !photo.CanDelete {
				continue
			}
			photo.Hidden = true
			photo.URL = ""
		}
		out.Photos = append(out.Photos, photo)
	}
	videos, err := tx.Videos(MaxRideVideos)
	if err != nil {
		return out, err
	}
	for _, video := range videos {
		if video.OwnerID != actor {
			blocked, err := tx.Blocked(actor, video.OwnerID)
			if err != nil {
				return out, err
			}
			if blocked {
				continue
			}
		}
		item := models.RideVideo{ID: video.ID, URL: video.URL, Title: video.Title, UploaderID: video.OwnerID, CreatedAt: video.CreatedAt, CanDelete: video.OwnerID == actor || tx.Ride().OrganizerID == actor}
		hidden, err := tx.ContentHidden("ride_video", video.ID)
		if err != nil {
			return out, err
		}
		if hidden {
			if !item.CanDelete {
				continue
			}
			item.Hidden = true
			item.URL = ""
			item.Title = ""
		}
		out.Videos = append(out.Videos, item)
	}
	return out, nil
}
func (m *Manager) Album(ctx context.Context, actor models.User, ride string) (models.RideAlbum, error) {
	ride, err := identifier(ride)
	if err != nil {
		return models.RideAlbum{}, err
	}
	var out models.RideAlbum
	err = m.repo.WithRideMedia(ctx, ride, actor.ID, func(tx database.RideMediaTransaction) error { var e error; out, e = albumView(tx, actor.ID); return e })
	return out, storageError(err)
}
func (m *Manager) RidePhotoContent(ctx context.Context, actor models.User, ride, id string) (models.PhotoContent, error) {
	ride, err := identifier(ride)
	if err != nil {
		return models.PhotoContent{}, err
	}
	id, err = identifier(id)
	if err != nil {
		return models.PhotoContent{}, err
	}
	var out models.PhotoContent
	err = m.repo.WithRideMedia(ctx, ride, actor.ID, func(tx database.RideMediaTransaction) error {
		if err := albumAccess(tx, actor.ID); err != nil {
			return err
		}
		photo, err := tx.Photo(id, true)
		if err != nil {
			return err
		}
		if photo.State != "ready" {
			return missing()
		}
		shown, err := visibleAuthor(tx, actor.ID, photo.OwnerID, "ride_photo", photo.ID)
		if err != nil {
			return err
		}
		if !shown {
			return missing()
		}
		out.Bytes = photo.Content
		return nil
	})
	return out, storageError(err)
}
func (m *Manager) UploadRidePhoto(ctx context.Context, actor models.User, ride string, input io.Reader) (models.RidePhotoUpload, error) {
	if err := complete(actor); err != nil {
		return models.RidePhotoUpload{}, err
	}
	ride, err := identifier(ride)
	if err != nil {
		return models.RidePhotoUpload{}, err
	}
	var out models.RidePhotoUpload
	err = m.photos.WithSlot(ctx, func(process photos.Process) error {
		var e error
		out, e = m.uploadRidePhoto(ctx, actor, ride, input, process)
		return e
	})
	return out, err
}
func (m *Manager) uploadRidePhoto(ctx context.Context, actor models.User, ride string, input io.Reader, process photos.Process) (models.RidePhotoUpload, error) {
	now := m.now().UTC()
	expires := now.Add(photos.ReservationTTL)
	pending := models.RidePhotoRecord{ID: uuid.NewString(), RideID: ride, OwnerID: actor.ID, State: "pending", ReservedBytes: photos.ReservedBytes, CreatedAt: now, ExpiresAt: &expires}
	err := m.repo.WithRideMedia(ctx, ride, actor.ID, func(tx database.RideMediaTransaction) error {
		if err := albumWritable(tx, actor.ID); err != nil {
			return err
		}
		if tx.Participation().Version < 1 {
			return missing()
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
		if quota.RideCount >= MaxRidePhotos || quota.UploaderCount >= MaxUploaderPhotos {
			return models.Fail("album_photo_limit", "Ride album or uploader photo limit reached")
		}
		pending.ParticipationVersion, pending.RidePlanVersion = tx.Participation().Version, tx.Ride().PlanVersion
		if err = tx.RecordPhotoAttempt(now); err != nil {
			return err
		}
		return tx.CreatePendingPhoto(pending)
	})
	if err != nil {
		return models.RidePhotoUpload{}, storageError(err)
	}
	completed := false
	defer func() {
		if !completed {
			m.cleanupRidePhoto(actor.ID, ride, pending.ID)
		}
	}()
	image, err := process(ctx, input)
	if err != nil {
		return models.RidePhotoUpload{}, err
	}
	if err = processed(image); err != nil {
		return models.RidePhotoUpload{}, err
	}
	var out models.RidePhotoUpload
	err = m.repo.WithRideMedia(ctx, ride, actor.ID, func(tx database.RideMediaTransaction) error {
		if err := albumWritable(tx, actor.ID); err != nil {
			return err
		}
		if tx.Participation().Version != pending.ParticipationVersion {
			return missing()
		}
		if tx.Ride().PlanVersion != pending.RidePlanVersion {
			return stale()
		}
		stored, err := tx.Photo(pending.ID, false)
		if errors.Is(err, database.ErrNotFound) {
			return models.Fail("photo_upload_expired", "Photo upload expired; try again")
		}
		if err != nil {
			return err
		}
		now := m.now().UTC()
		if stored.OwnerID != actor.ID || stored.State != "pending" || stored.ParticipationVersion != pending.ParticipationVersion || stored.RidePlanVersion != pending.RidePlanVersion || stored.ExpiresAt == nil || !now.Before(*stored.ExpiresAt) {
			return models.Fail("photo_upload_expired", "Photo upload expired; try again")
		}
		stored.State, stored.Content, stored.Width, stored.Height = "ready", image.Bytes, image.Width, image.Height
		stored.SizeBytes, stored.ReservedBytes, stored.ReadyAt, stored.ExpiresAt = int64(len(image.Bytes)), int64(len(image.Bytes)), &now, nil
		stored, err = tx.FinalizePhoto(stored)
		if err != nil {
			return err
		}
		if err = tx.SetAlbumVersion(tx.AlbumVersion() + 1); err != nil {
			return err
		}
		out.Photo = albumPhoto(stored, actor.ID, tx.Ride().OrganizerID)
		out.Album, err = albumView(tx, actor.ID)
		return err
	})
	if err != nil {
		return models.RidePhotoUpload{}, storageError(err)
	}
	completed = true
	return out, nil
}
func (m *Manager) cleanupRidePhoto(owner, ride, id string) {
	ctx, cancel := context.WithTimeout(context.Background(), photos.CleanupTimeout)
	defer cancel()
	_ = m.repo.WithRideMedia(ctx, ride, owner, func(tx database.RideMediaTransaction) error {
		record, err := tx.Photo(id, false)
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if record.OwnerID != owner || record.State != "pending" {
			return nil
		}
		return tx.DeletePhoto(id, "pending")
	})
}
func (m *Manager) DeleteRidePhoto(ctx context.Context, actor models.User, ride, id string, version int64) (models.RideAlbum, error) {
	ride, err := identifier(ride)
	if err != nil {
		return models.RideAlbum{}, err
	}
	id, err = identifier(id)
	if err != nil {
		return models.RideAlbum{}, err
	}
	if version < 1 {
		return models.RideAlbum{}, invalidVersion()
	}
	var out models.RideAlbum
	err = m.repo.WithRideMedia(ctx, ride, actor.ID, func(tx database.RideMediaTransaction) error {
		if err := albumAccess(tx, actor.ID); err != nil {
			return err
		}
		if tx.AlbumVersion() != version {
			return stale()
		}
		photo, err := tx.Photo(id, false)
		if err != nil {
			return err
		}
		if photo.State != "ready" {
			return missing()
		}
		if actor.ID != photo.OwnerID && actor.ID != tx.Ride().OrganizerID {
			return models.Fail("forbidden", "Delete only your own ride photos")
		}
		if err = tx.DeletePhoto(id, "ready"); err != nil {
			return err
		}
		if err = tx.SetAlbumVersion(version + 1); err != nil {
			return err
		}
		out, err = albumView(tx, actor.ID)
		return err
	})
	return out, storageError(err)
}
func validateVideo(input models.RideVideoInput) (models.RideVideoInput, error) {
	input.URL = strings.TrimSpace(input.URL)
	input.Title = strings.TrimSpace(input.Title)
	invalid := func() (models.RideVideoInput, error) {
		return input, models.Fail("invalid_video_link", "Use an HTTPS video link without credentials and a short title")
	}
	if len(input.URL) > 2048 || len(input.Title) > 160 {
		return invalid()
	}
	u, err := url.Parse(input.URL)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Opaque != "" {
		return invalid()
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if host == "" || !strings.Contains(host, ".") || strings.ContainsAny(host, "\\% \t\r\n") || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return invalid()
	}
	if _, err = netip.ParseAddr(host); err == nil {
		return invalid()
	}
	if port := u.Port(); port != "" {
		number, err := strconv.Atoi(port)
		if err != nil || number < 1 || number > 65535 {
			return invalid()
		}
	}
	if input.Title == "" {
		input.Title = host
	}
	input.URL = u.String()
	return input, nil
}
func (m *Manager) AddRideVideo(ctx context.Context, actor models.User, ride string, input models.RideVideoInput) (models.RideAlbum, error) {
	if err := complete(actor); err != nil {
		return models.RideAlbum{}, err
	}
	ride, err := identifier(ride)
	if err != nil {
		return models.RideAlbum{}, err
	}
	input, err = validateVideo(input)
	if err != nil {
		return models.RideAlbum{}, err
	}
	var out models.RideAlbum
	err = m.repo.WithRideMedia(ctx, ride, actor.ID, func(tx database.RideMediaTransaction) error {
		if err := albumWritable(tx, actor.ID); err != nil {
			return err
		}
		videos, err := tx.Videos(MaxRideVideos + 1)
		if err != nil {
			return err
		}
		own := 0
		for _, video := range videos {
			if video.OwnerID == actor.ID {
				own++
			}
		}
		if len(videos) >= MaxRideVideos || own >= MaxUploaderVideos {
			return models.Fail("album_video_limit", "Ride album or uploader video-link limit reached")
		}
		video := models.RideVideoRecord{ID: uuid.NewString(), RideID: ride, OwnerID: actor.ID, URL: input.URL, Title: input.Title, CreatedAt: m.now().UTC()}
		if err = tx.AddVideo(video); err != nil {
			return err
		}
		if err = tx.SetAlbumVersion(tx.AlbumVersion() + 1); err != nil {
			return err
		}
		out, err = albumView(tx, actor.ID)
		return err
	})
	return out, storageError(err)
}
func (m *Manager) DeleteRideVideo(ctx context.Context, actor models.User, ride, id string, version int64) (models.RideAlbum, error) {
	ride, err := identifier(ride)
	if err != nil {
		return models.RideAlbum{}, err
	}
	id, err = identifier(id)
	if err != nil {
		return models.RideAlbum{}, err
	}
	if version < 1 {
		return models.RideAlbum{}, invalidVersion()
	}
	var out models.RideAlbum
	err = m.repo.WithRideMedia(ctx, ride, actor.ID, func(tx database.RideMediaTransaction) error {
		if err := albumAccess(tx, actor.ID); err != nil {
			return err
		}
		if tx.AlbumVersion() != version {
			return stale()
		}
		video, err := tx.Video(id)
		if err != nil {
			return err
		}
		if actor.ID != video.OwnerID && actor.ID != tx.Ride().OrganizerID {
			return models.Fail("forbidden", "Delete only your own video links")
		}
		if err = tx.DeleteVideo(id); err != nil {
			return err
		}
		if err = tx.SetAlbumVersion(version + 1); err != nil {
			return err
		}
		out, err = albumView(tx, actor.ID)
		return err
	})
	return out, storageError(err)
}
