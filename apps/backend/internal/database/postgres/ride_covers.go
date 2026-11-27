package postgres

import (
	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"context"
	"github.com/jackc/pgx/v5"
	"time"
)

const rideCoverColumns = "id::text,ride_id::text,owner_id::text,state,reserved_bytes,width,height,size_bytes,cover_version,created_at,ready_at,expires_at"

type rideCoverTransaction struct {
	ctx                  context.Context
	tx                   pgx.Tx
	ride                 models.Ride
	actorID, actorStatus string
	coverState           models.RideCoverState
}

func (s *Store) WithRideCover(ctx context.Context, id, actor string, decide database.RideCoverDecision) error {
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
	ride, err := scanRide(tx.QueryRow(ctx, "SELECT "+rideColumns+" FROM rides r WHERE r.id=$1 FOR UPDATE OF r", id))
	if err != nil {
		return err
	}
	locked := &rideCoverTransaction{ctx: ctx, tx: tx, ride: ride, actorID: actor}
	// Ride operations lock the ride before accounts. Preserve that order; the
	// immutable uploader ID is quota attribution, never authorization.
	if actor != "" {
		if err = tx.QueryRow(ctx, "SELECT status FROM users WHERE id=$1 FOR SHARE", actor).Scan(&locked.actorStatus); err != nil {
			return repositoryError(err)
		}
	}
	if err = tx.QueryRow(ctx, "SELECT cover_version,COALESCE(current_cover_id::text,'') FROM rides WHERE id=$1", id).Scan(&locked.coverState.Version, &locked.coverState.CurrentCoverID); err != nil {
		return repositoryError(err)
	}
	if err = decide(locked); err != nil {
		return err
	}
	return repositoryError(tx.Commit(ctx))
}

func scanRideCover(row pgx.Row) (models.RideCoverRecord, error) {
	var cover models.RideCoverRecord
	var width, height *int
	var size *int64
	err := row.Scan(&cover.ID, &cover.RideID, &cover.OwnerID, &cover.State, &cover.ReservedBytes, &width, &height, &size, &cover.CoverVersion, &cover.CreatedAt, &cover.ReadyAt, &cover.ExpiresAt, &cover.Content)
	if width != nil {
		cover.Width = *width
	}
	if height != nil {
		cover.Height = *height
	}
	if size != nil {
		cover.SizeBytes = *size
	}
	return cover, repositoryError(err)
}

// rideCovers maps each ride ID to its current ready cover. Covers hidden by
// moderation are omitted; access to the ride itself is decided by callers.
func rideCovers(ctx context.Context, q querier, ids []string) (map[string]*models.Photo, error) {
	out := map[string]*models.Photo{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := q.Query(ctx, `SELECT c.id::text,c.ride_id::text,c.width,c.height,c.created_at
		FROM rides r JOIN ride_covers c ON c.ride_id=r.id AND c.id=r.current_cover_id
		WHERE r.id=ANY($1::uuid[]) AND c.state='ready'
		  AND NOT EXISTS(SELECT 1 FROM moderation_hidden h WHERE h.target_type='ride_cover' AND h.target_id=c.id)`, ids)
	if err != nil {
		return nil, repositoryError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var cover models.RideCoverRecord
		if err = rows.Scan(&cover.ID, &cover.RideID, &cover.Width, &cover.Height, &cover.CreatedAt); err != nil {
			return nil, repositoryError(err)
		}
		out[cover.RideID] = models.RideCoverPhoto(cover)
	}
	return out, repositoryError(rows.Err())
}

func attachRideCover(ctx context.Context, q querier, summary *models.RideSummary) error {
	covers, err := rideCovers(ctx, q, []string{summary.ID})
	summary.Cover = covers[summary.ID]
	return err
}

func (t *rideCoverTransaction) Ride() models.Ride                 { return t.ride }
func (t *rideCoverTransaction) ActorStatus() string               { return t.actorStatus }
func (t *rideCoverTransaction) CoverState() models.RideCoverState { return t.coverState }
func (t *rideCoverTransaction) ActorParticipation() (string, error) {
	if t.actorID == "" {
		return "", nil
	}
	return readParticipation(t.ctx, t.tx, t.ride.ID, t.actorID)
}
func (t *rideCoverTransaction) ContentHidden(kind, id string) (bool, error) {
	return trustContentHidden(t.ctx, t.tx, kind, id)
}
func (t *rideCoverTransaction) Cover(id string, content bool) (models.RideCoverRecord, error) {
	column := "NULL::bytea"
	if content {
		column = "content"
	}
	return scanRideCover(t.tx.QueryRow(t.ctx, "SELECT "+rideCoverColumns+","+column+" FROM ride_covers WHERE ride_id=$1 AND id=$2", t.ride.ID, id))
}
func (t *rideCoverTransaction) ExpirePending(now time.Time) error {
	return expirePendingPhotos(t.ctx, t.tx, now)
}
func (t *rideCoverTransaction) PrunePhotoAttempts(before time.Time) error {
	return prunePhotoAttempts(t.ctx, t.tx, before)
}
func (t *rideCoverTransaction) CoverQuota(since time.Time) (models.RideCoverQuota, error) {
	var quota models.RideCoverQuota
	err := t.tx.QueryRow(t.ctx, "SELECT COUNT(*) FROM ride_covers WHERE ride_id=$1 AND state='pending'", t.ride.ID).Scan(&quota.PendingCount)
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
func (t *rideCoverTransaction) RecordPhotoAttempt(now time.Time) error {
	_, err := t.tx.Exec(t.ctx, "INSERT INTO photo_upload_attempts(owner_id,requested_at) VALUES($1,$2)", t.actorID, now)
	return repositoryError(err)
}
func (t *rideCoverTransaction) CreatePendingCover(cover models.RideCoverRecord) error {
	_, err := t.tx.Exec(t.ctx, `INSERT INTO ride_covers(id,ride_id,owner_id,state,reserved_bytes,cover_version,created_at,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, cover.ID, t.ride.ID, t.actorID, cover.State, cover.ReservedBytes, cover.CoverVersion, cover.CreatedAt, cover.ExpiresAt)
	return repositoryError(err)
}
func (t *rideCoverTransaction) FinalizeCover(cover models.RideCoverRecord) (models.RideCoverRecord, error) {
	return scanRideCover(t.tx.QueryRow(t.ctx, `UPDATE ride_covers SET state=$3,reserved_bytes=$4,content=$5,width=$6,height=$7,size_bytes=$8,ready_at=$9,expires_at=NULL WHERE ride_id=$1 AND id=$2 AND state='pending' RETURNING `+rideCoverColumns+",NULL::bytea", t.ride.ID, cover.ID, cover.State, cover.ReservedBytes, cover.Content, cover.Width, cover.Height, cover.SizeBytes, cover.ReadyAt))
}
func (t *rideCoverTransaction) DeleteCover(id, state string) error {
	result, err := t.tx.Exec(t.ctx, "DELETE FROM ride_covers WHERE ride_id=$1 AND id=$2 AND state=$3", t.ride.ID, id, state)
	if err != nil {
		return repositoryError(err)
	}
	if result.RowsAffected() != 1 {
		return database.ErrNotFound
	}
	return nil
}
func (t *rideCoverTransaction) SetCoverState(state models.RideCoverState) error {
	_, err := t.tx.Exec(t.ctx, "UPDATE rides SET cover_version=$2,current_cover_id=NULLIF($3,'')::uuid WHERE id=$1", t.ride.ID, state.Version, state.CurrentCoverID)
	if err == nil {
		t.coverState = state
	}
	return repositoryError(err)
}

var _ database.RideCoverRepository = (*Store)(nil)
var _ database.RideCoverTransaction = (*rideCoverTransaction)(nil)
