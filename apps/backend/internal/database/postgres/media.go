package postgres

import (
	"bikers.kz/backend/internal/models"
	"context"
	"github.com/jackc/pgx/v5"
	"time"
)

type mediaTransaction struct {
	ctx     context.Context
	tx      pgx.Tx
	actorID string
}

func (s *Store) beginMedia(ctx context.Context, photoQuota bool) (pgx.Tx, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, repositoryError(err)
	}
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", trustAdvisoryLock); err != nil {
		_ = tx.Rollback(ctx)
		return nil, repositoryError(err)
	}
	if photoQuota {
		if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", photoQuotaAdvisoryLock); err != nil {
			_ = tx.Rollback(ctx)
			return nil, repositoryError(err)
		}
	}
	return tx, nil
}
func (t *mediaTransaction) Blocked(viewer, target string) (bool, error) {
	return trustBlocked(t.ctx, t.tx, viewer, target)
}
func (t *mediaTransaction) ContentHidden(kind, id string) (bool, error) {
	return trustContentHidden(t.ctx, t.tx, kind, id)
}
func (t *mediaTransaction) ExpirePending(now time.Time) error {
	return expirePendingPhotos(t.ctx, t.tx, now)
}
func (t *mediaTransaction) PrunePhotoAttempts(before time.Time) error {
	return prunePhotoAttempts(t.ctx, t.tx, before)
}
func (t *mediaTransaction) RecordPhotoAttempt(now time.Time) error {
	_, err := t.tx.Exec(t.ctx, "INSERT INTO photo_upload_attempts(owner_id,requested_at) VALUES($1,$2)", t.actorID, now)
	return repositoryError(err)
}
func (t *mediaTransaction) quota(since time.Time) (models.MediaQuota, error) {
	var quota models.MediaQuota
	var err error
	quota.OwnerBytes, quota.GlobalBytes, err = photoStorageTotals(t.ctx, t.tx, t.actorID)
	if err != nil {
		return quota, err
	}
	err = t.tx.QueryRow(t.ctx, "SELECT COUNT(*) FROM photo_upload_attempts WHERE owner_id=$1 AND requested_at>=$2", t.actorID, since).Scan(&quota.RecentAttempts)
	return quota, repositoryError(err)
}
