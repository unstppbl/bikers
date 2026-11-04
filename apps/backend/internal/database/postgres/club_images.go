package postgres

import (
	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"context"
	"github.com/jackc/pgx/v5"
	"time"
)

const clubImageColumns = "id::text,club_id::text,owner_id::text,state,reserved_bytes,width,height,size_bytes,club_version,image_version,created_at,ready_at,expires_at"

type clubImageTransaction struct {
	ctx                  context.Context
	tx                   pgx.Tx
	club                 models.Club
	actorID, actorStatus string
	imageState           models.ClubImageState
}

func (s *Store) WithClubImage(ctx context.Context, id, actor string, decide database.ClubImageDecision) error {
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
	club, err := scanClub(tx.QueryRow(ctx, "SELECT "+clubColumns+" FROM clubs WHERE id=$1 FOR UPDATE", id))
	if err != nil {
		return err
	}
	locked := &clubImageTransaction{ctx: ctx, tx: tx, club: club, actorID: actor}
	// Existing club operations lock club before accounts. Preserve that order;
	// immutable uploader IDs are quota attribution, never authorization.
	if actor != "" {
		if err = tx.QueryRow(ctx, "SELECT status FROM users WHERE id=$1 FOR SHARE", actor).Scan(&locked.actorStatus); err != nil {
			return repositoryError(err)
		}
	}
	if err = tx.QueryRow(ctx, "SELECT image_version,COALESCE(current_image_id::text,'') FROM clubs WHERE id=$1", id).Scan(&locked.imageState.Version, &locked.imageState.CurrentImageID); err != nil {
		return repositoryError(err)
	}
	if err = decide(locked); err != nil {
		return err
	}
	return repositoryError(tx.Commit(ctx))
}

func scanClubImage(row pgx.Row) (models.ClubImageRecord, error) {
	var image models.ClubImageRecord
	var width, height *int
	var size *int64
	err := row.Scan(&image.ID, &image.ClubID, &image.OwnerID, &image.State, &image.ReservedBytes, &width, &height, &size, &image.ClubVersion, &image.ImageVersion, &image.CreatedAt, &image.ReadyAt, &image.ExpiresAt, &image.Content)
	if width != nil {
		image.Width = *width
	}
	if height != nil {
		image.Height = *height
	}
	if size != nil {
		image.SizeBytes = *size
	}
	return image, repositoryError(err)
}
func (t *clubImageTransaction) Club() models.Club                 { return t.club }
func (t *clubImageTransaction) ActorStatus() string               { return t.actorStatus }
func (t *clubImageTransaction) ImageState() models.ClubImageState { return t.imageState }
func (t *clubImageTransaction) Image(id string, content bool) (models.ClubImageRecord, error) {
	column := "NULL::bytea"
	if content {
		column = "content"
	}
	return scanClubImage(t.tx.QueryRow(t.ctx, "SELECT "+clubImageColumns+","+column+" FROM club_images WHERE club_id=$1 AND id=$2", t.club.ID, id))
}
func (t *clubImageTransaction) ExpirePending(now time.Time) error {
	return expirePendingPhotos(t.ctx, t.tx, now)
}
func (t *clubImageTransaction) PrunePhotoAttempts(before time.Time) error {
	return prunePhotoAttempts(t.ctx, t.tx, before)
}
func (t *clubImageTransaction) ImageQuota(since time.Time) (models.ClubImageQuota, error) {
	var quota models.ClubImageQuota
	err := t.tx.QueryRow(t.ctx, "SELECT COUNT(*) FROM club_images WHERE club_id=$1 AND state='pending'", t.club.ID).Scan(&quota.PendingCount)
	if err != nil {
		return quota, repositoryError(err)
	}
	quota.OwnerBytes, quota.GlobalBytes, err = photoStorageTotals(t.ctx, t.tx, t.actorID)
	if err != nil {
		return quota, err
	}
	err = t.tx.QueryRow(t.ctx, "SELECT COUNT(*) FROM photo_upload_attempts WHERE owner_id=$1 AND requested_at>=$2", t.actorID, since).Scan(&quota.RecentAttempts)
	return quota, repositoryError(err)
}
func (t *clubImageTransaction) RecordPhotoAttempt(now time.Time) error {
	_, err := t.tx.Exec(t.ctx, "INSERT INTO photo_upload_attempts(owner_id,requested_at) VALUES($1,$2)", t.actorID, now)
	return repositoryError(err)
}
func (t *clubImageTransaction) CreatePendingImage(image models.ClubImageRecord) error {
	_, err := t.tx.Exec(t.ctx, `INSERT INTO club_images(id,club_id,owner_id,state,reserved_bytes,club_version,image_version,created_at,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, image.ID, t.club.ID, t.actorID, image.State, image.ReservedBytes, image.ClubVersion, image.ImageVersion, image.CreatedAt, image.ExpiresAt)
	return repositoryError(err)
}
func (t *clubImageTransaction) FinalizeImage(image models.ClubImageRecord) (models.ClubImageRecord, error) {
	return scanClubImage(t.tx.QueryRow(t.ctx, `UPDATE club_images SET state=$3,reserved_bytes=$4,content=$5,width=$6,height=$7,size_bytes=$8,ready_at=$9,expires_at=NULL WHERE club_id=$1 AND id=$2 AND state='pending' RETURNING `+clubImageColumns+",NULL::bytea", t.club.ID, image.ID, image.State, image.ReservedBytes, image.Content, image.Width, image.Height, image.SizeBytes, image.ReadyAt))
}
func (t *clubImageTransaction) DeleteImage(id, state string) error {
	result, err := t.tx.Exec(t.ctx, "DELETE FROM club_images WHERE club_id=$1 AND id=$2 AND state=$3", t.club.ID, id, state)
	if err != nil {
		return repositoryError(err)
	}
	if result.RowsAffected() != 1 {
		return database.ErrNotFound
	}
	return nil
}
func (t *clubImageTransaction) SetImageState(state models.ClubImageState) error {
	_, err := t.tx.Exec(t.ctx, "UPDATE clubs SET image_version=$2,current_image_id=NULLIF($3,'')::uuid WHERE id=$1", t.club.ID, state.Version, state.CurrentImageID)
	if err == nil {
		t.imageState = state
	}
	return repositoryError(err)
}

var _ database.ClubImageRepository = (*Store)(nil)
var _ database.ClubImageTransaction = (*clubImageTransaction)(nil)

func (t *clubImageTransaction) ContentHidden(kind, id string) (bool, error) {
	return trustContentHidden(t.ctx, t.tx, kind, id)
}
func (t *clubImageTransaction) ActorMembership() (string, error) {
	if t.actorID == "" {
		return "", nil
	}
	var state string
	err := t.tx.QueryRow(t.ctx, "SELECT COALESCE((SELECT state FROM club_memberships WHERE club_id=$1 AND user_id=$2),'')", t.club.ID, t.actorID).Scan(&state)
	return state, repositoryError(err)
}
