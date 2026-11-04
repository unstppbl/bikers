package postgres

import (
	"context"
	"time"

	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"github.com/jackc/pgx/v5"
)

const photoQuotaAdvisoryLock int64 = 773512003
const photoColumns = "id::text,bike_id::text,owner_id::text,state,reserved_bytes,width,height,size_bytes,created_at,ready_at,expires_at"

type photoTransaction struct {
	bikeTransaction
	gallery models.PhotoGalleryState
}

func (s *Store) WithPhotoBike(ctx context.Context, id string, decide database.PhotoDecision) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return repositoryError(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", trustAdvisoryLock); err != nil {
		return repositoryError(err)
	}
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", photoQuotaAdvisoryLock); err != nil {
		return repositoryError(err)
	}
	var ownerID string
	if err = tx.QueryRow(ctx, "SELECT owner_id::text FROM bikes WHERE id=$1", id).Scan(&ownerID); err != nil {
		return repositoryError(err)
	}
	owner, err := lockedGarageOwner(ctx, tx, ownerID)
	if err != nil {
		return err
	}
	bike, err := scanBike(tx.QueryRow(ctx, "SELECT "+bikeColumns+" FROM bikes WHERE id=$1 FOR UPDATE", id))
	if err != nil {
		return err
	}
	if bike.OwnerID != owner.ID {
		return database.ErrConflict
	}
	locked := &photoTransaction{bikeTransaction: bikeTransaction{garageOwnerTransaction: garageOwnerTransaction{ctx: ctx, tx: tx, owner: owner}, bike: bike}}
	if err = tx.QueryRow(ctx, "SELECT photo_version,COALESCE(cover_photo_id::text,'') FROM bikes WHERE id=$1", id).Scan(&locked.gallery.Version, &locked.gallery.CoverPhotoID); err != nil {
		return repositoryError(err)
	}
	if err = decide(locked); err != nil {
		return err
	}
	return repositoryError(tx.Commit(ctx))
}

func scanPhoto(row pgx.Row) (models.PhotoRecord, error) {
	var photo models.PhotoRecord
	var width, height *int
	var size *int64
	err := row.Scan(&photo.ID, &photo.BikeID, &photo.OwnerID, &photo.State, &photo.ReservedBytes, &width, &height, &size, &photo.CreatedAt, &photo.ReadyAt, &photo.ExpiresAt, &photo.Content)
	if width != nil {
		photo.Width = *width
	}
	if height != nil {
		photo.Height = *height
	}
	if size != nil {
		photo.SizeBytes = *size
	}
	return photo, repositoryError(err)
}

func (t *photoTransaction) GalleryState() models.PhotoGalleryState { return t.gallery }

func (t *photoTransaction) ReadyPhotos() ([]models.PhotoRecord, error) {
	rows, err := t.tx.Query(t.ctx, "SELECT "+photoColumns+",NULL::bytea FROM bike_photos WHERE bike_id=$1 AND state='ready' ORDER BY created_at,id", t.bike.ID)
	if err != nil {
		return nil, repositoryError(err)
	}
	defer rows.Close()
	photos := []models.PhotoRecord{}
	for rows.Next() {
		photo, err := scanPhoto(rows)
		if err != nil {
			return nil, err
		}
		photos = append(photos, photo)
	}
	return photos, repositoryError(rows.Err())
}

func (t *photoTransaction) Photo(id string, content bool) (models.PhotoRecord, error) {
	column := "NULL::bytea"
	if content {
		column = "content"
	}
	return scanPhoto(t.tx.QueryRow(t.ctx, "SELECT "+photoColumns+","+column+" FROM bike_photos WHERE bike_id=$1 AND id=$2", t.bike.ID, id))
}

func (t *photoTransaction) ExpirePending(now time.Time) error {
	return expirePendingPhotos(t.ctx, t.tx, now)
}

func (t *photoTransaction) PrunePhotoAttempts(before time.Time) error {
	return prunePhotoAttempts(t.ctx, t.tx, before)
}

func (t *photoTransaction) PhotoQuota(since time.Time) (models.PhotoQuota, error) {
	var quota models.PhotoQuota
	err := t.tx.QueryRow(t.ctx, `SELECT COUNT(*) FROM bike_photos WHERE bike_id=$1`, t.bike.ID).Scan(&quota.BikeCount)
	if err != nil {
		return quota, repositoryError(err)
	}
	quota.OwnerBytes, quota.GlobalBytes, err = photoStorageTotals(t.ctx, t.tx, t.owner.ID)
	if err != nil {
		return quota, err
	}
	err = t.tx.QueryRow(t.ctx, "SELECT COUNT(*) FROM photo_upload_attempts WHERE owner_id=$1 AND requested_at>=$2", t.owner.ID, since).Scan(&quota.RecentAttempts)
	return quota, repositoryError(err)
}

func expirePendingPhotos(ctx context.Context, tx pgx.Tx, now time.Time) error {
	if _, err := tx.Exec(ctx, "DELETE FROM bike_photos WHERE state='pending' AND expires_at<=$1", now); err != nil {
		return repositoryError(err)
	}
	if _, err := tx.Exec(ctx, "DELETE FROM club_images WHERE state='pending' AND expires_at<=$1", now); err != nil {
		return repositoryError(err)
	}
	if _, err := tx.Exec(ctx, "DELETE FROM avatars WHERE state='pending' AND expires_at<=$1", now); err != nil {
		return repositoryError(err)
	}
	_, err := tx.Exec(ctx, "DELETE FROM ride_photos WHERE state='pending' AND expires_at<=$1", now)
	return repositoryError(err)
}
func prunePhotoAttempts(ctx context.Context, tx pgx.Tx, before time.Time) error {
	_, err := tx.Exec(ctx, "DELETE FROM photo_upload_attempts WHERE requested_at<$1", before)
	return repositoryError(err)
}
func photoStorageTotals(ctx context.Context, tx pgx.Tx, owner string) (int64, int64, error) {
	var ownerBytes, globalBytes int64
	err := tx.QueryRow(ctx, `SELECT COALESCE(SUM(reserved_bytes) FILTER(WHERE owner_id=$1),0),COALESCE(SUM(reserved_bytes),0) FROM (SELECT owner_id,reserved_bytes FROM bike_photos UNION ALL SELECT owner_id,reserved_bytes FROM club_images UNION ALL SELECT owner_id,reserved_bytes FROM avatars UNION ALL SELECT owner_id,reserved_bytes FROM ride_photos) AS photos`, owner).Scan(&ownerBytes, &globalBytes)
	return ownerBytes, globalBytes, repositoryError(err)
}

func (t *photoTransaction) RecordPhotoAttempt(now time.Time) error {
	_, err := t.tx.Exec(t.ctx, "INSERT INTO photo_upload_attempts(owner_id,requested_at) VALUES($1,$2)", t.owner.ID, now)
	return repositoryError(err)
}

func (t *photoTransaction) CreatePendingPhoto(photo models.PhotoRecord) error {
	_, err := t.tx.Exec(t.ctx, `INSERT INTO bike_photos(id,bike_id,owner_id,state,reserved_bytes,created_at,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, photo.ID, t.bike.ID, t.owner.ID, photo.State, photo.ReservedBytes, photo.CreatedAt, photo.ExpiresAt)
	return repositoryError(err)
}

func (t *photoTransaction) FinalizePhoto(photo models.PhotoRecord) (models.PhotoRecord, error) {
	return scanPhoto(t.tx.QueryRow(t.ctx, `UPDATE bike_photos SET state=$3,reserved_bytes=$4,content=$5,width=$6,height=$7,size_bytes=$8,ready_at=$9,expires_at=NULL WHERE bike_id=$1 AND id=$2 AND state='pending' RETURNING `+photoColumns+",NULL::bytea", t.bike.ID, photo.ID, photo.State, photo.ReservedBytes, photo.Content, photo.Width, photo.Height, photo.SizeBytes, photo.ReadyAt))
}

func (t *photoTransaction) DeletePhoto(id, expectedState string) error {
	result, err := t.tx.Exec(t.ctx, "DELETE FROM bike_photos WHERE bike_id=$1 AND id=$2 AND state=$3", t.bike.ID, id, expectedState)
	if err != nil {
		return repositoryError(err)
	}
	if result.RowsAffected() != 1 {
		return database.ErrNotFound
	}
	return nil
}

func (t *photoTransaction) SetPhotoGallery(gallery models.PhotoGalleryState) error {
	_, err := t.tx.Exec(t.ctx, "UPDATE bikes SET photo_version=$2,cover_photo_id=NULLIF($3,'')::uuid WHERE id=$1", t.bike.ID, gallery.Version, gallery.CoverPhotoID)
	if err == nil {
		t.gallery = gallery
	}
	return repositoryError(err)
}

var _ database.PhotoRepository = (*Store)(nil)
var _ database.PhotoTransaction = (*photoTransaction)(nil)
