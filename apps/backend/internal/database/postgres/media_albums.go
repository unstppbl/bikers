package postgres

import (
	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"context"
	"github.com/jackc/pgx/v5"
	"time"
)

const ridePhotoColumns = "id::text,ride_id::text,owner_id::text,state,reserved_bytes,width,height,size_bytes,participation_version,ride_plan_version,created_at,ready_at,expires_at"

type rideMediaTransaction struct {
	mediaTransaction
	ride          models.Ride
	participation models.MediaParticipation
	albumVersion  int64
}

func (s *Store) WithRideMedia(ctx context.Context, rideID, actor string, decide database.RideMediaDecision) error {
	tx, err := s.beginMedia(ctx, true)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	ride, err := scanRide(tx.QueryRow(ctx, "SELECT "+rideColumns+" FROM rides WHERE id=$1 FOR UPDATE", rideID))
	if err != nil {
		return err
	}
	locked := &rideMediaTransaction{mediaTransaction: mediaTransaction{ctx: ctx, tx: tx, actorID: actor}, ride: ride}
	if err = tx.QueryRow(ctx, `SELECT u.status,COALESCE(p.state,''),COALESCE(p.media_version,0) FROM users u LEFT JOIN participation p ON p.user_id=u.id AND p.ride_id=$1 WHERE u.id=$2 FOR SHARE OF u`, rideID, actor).Scan(&locked.participation.AccountStatus, &locked.participation.State, &locked.participation.Version); err != nil {
		return repositoryError(err)
	}
	if err = tx.QueryRow(ctx, "SELECT album_version FROM rides WHERE id=$1", rideID).Scan(&locked.albumVersion); err != nil {
		return repositoryError(err)
	}
	if err = decide(locked); err != nil {
		return err
	}
	return repositoryError(tx.Commit(ctx))
}
func (t *rideMediaTransaction) Ride() models.Ride                        { return t.ride }
func (t *rideMediaTransaction) Participation() models.MediaParticipation { return t.participation }
func (t *rideMediaTransaction) AlbumVersion() int64                      { return t.albumVersion }
func (t *rideMediaTransaction) SetAlbumVersion(version int64) error {
	_, err := t.tx.Exec(t.ctx, "UPDATE rides SET album_version=$2 WHERE id=$1", t.ride.ID, version)
	if err == nil {
		t.albumVersion = version
	}
	return repositoryError(err)
}
func scanRidePhoto(row pgx.Row) (models.RidePhotoRecord, error) {
	var out models.RidePhotoRecord
	var width, height *int
	var size *int64
	err := row.Scan(&out.ID, &out.RideID, &out.OwnerID, &out.State, &out.ReservedBytes, &width, &height, &size, &out.ParticipationVersion, &out.RidePlanVersion, &out.CreatedAt, &out.ReadyAt, &out.ExpiresAt, &out.Content)
	if width != nil {
		out.Width = *width
	}
	if height != nil {
		out.Height = *height
	}
	if size != nil {
		out.SizeBytes = *size
	}
	return out, repositoryError(err)
}
func (t *rideMediaTransaction) Photos(limit int) ([]models.RidePhotoRecord, error) {
	rows, err := t.tx.Query(t.ctx, "SELECT "+ridePhotoColumns+",NULL::bytea FROM ride_photos WHERE ride_id=$1 AND state='ready' ORDER BY created_at,id LIMIT $2", t.ride.ID, limit)
	if err != nil {
		return nil, repositoryError(err)
	}
	defer rows.Close()
	out := []models.RidePhotoRecord{}
	for rows.Next() {
		photo, err := scanRidePhoto(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, photo)
	}
	return out, repositoryError(rows.Err())
}
func (t *rideMediaTransaction) Photo(id string, content bool) (models.RidePhotoRecord, error) {
	column := "NULL::bytea"
	if content {
		column = "content"
	}
	return scanRidePhoto(t.tx.QueryRow(t.ctx, "SELECT "+ridePhotoColumns+","+column+" FROM ride_photos WHERE ride_id=$1 AND id=$2", t.ride.ID, id))
}
func scanRideVideo(row pgx.Row) (models.RideVideoRecord, error) {
	var out models.RideVideoRecord
	err := row.Scan(&out.ID, &out.RideID, &out.OwnerID, &out.URL, &out.Title, &out.CreatedAt)
	return out, repositoryError(err)
}
func (t *rideMediaTransaction) Videos(limit int) ([]models.RideVideoRecord, error) {
	rows, err := t.tx.Query(t.ctx, "SELECT id::text,ride_id::text,owner_id::text,url,title,created_at FROM ride_video_links WHERE ride_id=$1 ORDER BY created_at,id LIMIT $2", t.ride.ID, limit)
	if err != nil {
		return nil, repositoryError(err)
	}
	defer rows.Close()
	out := []models.RideVideoRecord{}
	for rows.Next() {
		video, err := scanRideVideo(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, video)
	}
	return out, repositoryError(rows.Err())
}
func (t *rideMediaTransaction) Video(id string) (models.RideVideoRecord, error) {
	return scanRideVideo(t.tx.QueryRow(t.ctx, "SELECT id::text,ride_id::text,owner_id::text,url,title,created_at FROM ride_video_links WHERE ride_id=$1 AND id=$2", t.ride.ID, id))
}
func (t *rideMediaTransaction) Quota(since time.Time) (models.MediaQuota, error) {
	quota, err := t.quota(since)
	if err != nil {
		return quota, err
	}
	err = t.tx.QueryRow(t.ctx, "SELECT COUNT(*),COUNT(*) FILTER(WHERE owner_id=$2) FROM ride_photos WHERE ride_id=$1", t.ride.ID, t.actorID).Scan(&quota.RideCount, &quota.UploaderCount)
	return quota, repositoryError(err)
}
func (t *rideMediaTransaction) CreatePendingPhoto(photo models.RidePhotoRecord) error {
	_, err := t.tx.Exec(t.ctx, `INSERT INTO ride_photos(id,ride_id,owner_id,state,reserved_bytes,participation_version,ride_plan_version,created_at,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, photo.ID, t.ride.ID, t.actorID, photo.State, photo.ReservedBytes, photo.ParticipationVersion, photo.RidePlanVersion, photo.CreatedAt, photo.ExpiresAt)
	return repositoryError(err)
}
func (t *rideMediaTransaction) FinalizePhoto(photo models.RidePhotoRecord) (models.RidePhotoRecord, error) {
	return scanRidePhoto(t.tx.QueryRow(t.ctx, `UPDATE ride_photos SET state=$3,reserved_bytes=$4,content=$5,width=$6,height=$7,size_bytes=$8,ready_at=$9,expires_at=NULL WHERE ride_id=$1 AND id=$2 AND state='pending' RETURNING `+ridePhotoColumns+",NULL::bytea", t.ride.ID, photo.ID, photo.State, photo.ReservedBytes, photo.Content, photo.Width, photo.Height, photo.SizeBytes, photo.ReadyAt))
}
func (t *rideMediaTransaction) DeletePhoto(id, state string) error {
	result, err := t.tx.Exec(t.ctx, "DELETE FROM ride_photos WHERE ride_id=$1 AND id=$2 AND state=$3", t.ride.ID, id, state)
	if err != nil {
		return repositoryError(err)
	}
	if result.RowsAffected() != 1 {
		return database.ErrNotFound
	}
	return nil
}
func (t *rideMediaTransaction) AddVideo(video models.RideVideoRecord) error {
	_, err := t.tx.Exec(t.ctx, "INSERT INTO ride_video_links(id,ride_id,owner_id,url,title,created_at) VALUES($1,$2,$3,$4,$5,$6)", video.ID, t.ride.ID, t.actorID, video.URL, video.Title, video.CreatedAt)
	return repositoryError(err)
}
func (t *rideMediaTransaction) DeleteVideo(id string) error {
	result, err := t.tx.Exec(t.ctx, "DELETE FROM ride_video_links WHERE ride_id=$1 AND id=$2", t.ride.ID, id)
	if err != nil {
		return repositoryError(err)
	}
	if result.RowsAffected() != 1 {
		return database.ErrNotFound
	}
	return nil
}

var _ database.MediaRepository = (*Store)(nil)
var _ database.RideMediaTransaction = (*rideMediaTransaction)(nil)
