package postgres

import (
	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"context"
	"github.com/jackc/pgx/v5"
	"time"
)

const avatarColumns = "id::text,owner_id::text,state,reserved_bytes,width,height,size_bytes,avatar_version,created_at,ready_at,expires_at"

type avatarTransaction struct {
	mediaTransaction
	status string
	state  models.AvatarState
}

func (s *Store) WithAvatar(ctx context.Context, owner string, decide database.AvatarDecision) error {
	tx, err := s.beginMedia(ctx, true)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	locked := &avatarTransaction{mediaTransaction: mediaTransaction{ctx: ctx, tx: tx, actorID: owner}}
	if err = tx.QueryRow(ctx, "SELECT status,avatar_version,COALESCE(current_avatar_id::text,'') FROM users WHERE id=$1 FOR UPDATE", owner).Scan(&locked.status, &locked.state.Version, &locked.state.CurrentID); err != nil {
		return repositoryError(err)
	}
	if err = decide(locked); err != nil {
		return err
	}
	return repositoryError(tx.Commit(ctx))
}
func scanAvatar(row pgx.Row) (models.AvatarRecord, error) {
	var out models.AvatarRecord
	var width, height *int
	var size *int64
	err := row.Scan(&out.ID, &out.OwnerID, &out.State, &out.ReservedBytes, &width, &height, &size, &out.AvatarVersion, &out.CreatedAt, &out.ReadyAt, &out.ExpiresAt, &out.Content)
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
func (t *avatarTransaction) OwnerStatus() string             { return t.status }
func (t *avatarTransaction) AvatarState() models.AvatarState { return t.state }
func readAvatar(ctx context.Context, tx pgx.Tx, owner, id string, content bool) (models.AvatarRecord, error) {
	column := "NULL::bytea"
	if content {
		column = "content"
	}
	return scanAvatar(tx.QueryRow(ctx, "SELECT "+avatarColumns+","+column+" FROM avatars WHERE owner_id=$1 AND id=$2", owner, id))
}
func (t *avatarTransaction) Avatar(id string, content bool) (models.AvatarRecord, error) {
	return readAvatar(t.ctx, t.tx, t.actorID, id, content)
}
func (t *avatarTransaction) Quota(since time.Time) (models.MediaQuota, error) {
	quota, err := t.quota(since)
	if err != nil {
		return quota, err
	}
	err = t.tx.QueryRow(t.ctx, "SELECT COUNT(*) FROM avatars WHERE owner_id=$1 AND state='pending'", t.actorID).Scan(&quota.PendingCount)
	return quota, repositoryError(err)
}
func (t *avatarTransaction) CreatePending(image models.AvatarRecord) error {
	_, err := t.tx.Exec(t.ctx, `INSERT INTO avatars(id,owner_id,state,reserved_bytes,avatar_version,created_at,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, image.ID, t.actorID, image.State, image.ReservedBytes, image.AvatarVersion, image.CreatedAt, image.ExpiresAt)
	return repositoryError(err)
}
func (t *avatarTransaction) Finalize(image models.AvatarRecord) (models.AvatarRecord, error) {
	return scanAvatar(t.tx.QueryRow(t.ctx, `UPDATE avatars SET state=$3,reserved_bytes=$4,content=$5,width=$6,height=$7,size_bytes=$8,ready_at=$9,expires_at=NULL WHERE owner_id=$1 AND id=$2 AND state='pending' RETURNING `+avatarColumns+",NULL::bytea", t.actorID, image.ID, image.State, image.ReservedBytes, image.Content, image.Width, image.Height, image.SizeBytes, image.ReadyAt))
}
func (t *avatarTransaction) Delete(id, state string) error {
	result, err := t.tx.Exec(t.ctx, "DELETE FROM avatars WHERE owner_id=$1 AND id=$2 AND state=$3", t.actorID, id, state)
	if err != nil {
		return repositoryError(err)
	}
	if result.RowsAffected() != 1 {
		return database.ErrNotFound
	}
	return nil
}
func (t *avatarTransaction) SetAvatarState(state models.AvatarState) error {
	_, err := t.tx.Exec(t.ctx, "UPDATE users SET avatar_version=$2,current_avatar_id=NULLIF($3,'')::uuid WHERE id=$1", t.actorID, state.Version, state.CurrentID)
	if err == nil {
		t.state = state
	}
	return repositoryError(err)
}

type avatarContextTransaction struct {
	mediaTransaction
	context models.AvatarContext
	state   models.AvatarState
}

func (t *avatarContextTransaction) Context() models.AvatarContext   { return t.context }
func (t *avatarContextTransaction) AvatarState() models.AvatarState { return t.state }
func (t *avatarContextTransaction) Avatar(id string, content bool) (models.AvatarRecord, error) {
	return readAvatar(t.ctx, t.tx, t.context.TargetID, id, content)
}
func (s *Store) WithRideAvatar(ctx context.Context, parent, viewer, target string, decide database.AvatarContextDecision) error {
	return s.withContextAvatar(ctx, "ride", parent, viewer, target, decide)
}
func (s *Store) WithClubAvatar(ctx context.Context, parent, viewer, target string, decide database.AvatarContextDecision) error {
	return s.withContextAvatar(ctx, "club", parent, viewer, target, decide)
}
func (s *Store) withContextAvatar(ctx context.Context, kind, parent, viewer, target string, decide database.AvatarContextDecision) error {
	tx, err := s.beginMedia(ctx, false)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	locked := &avatarContextTransaction{mediaTransaction: mediaTransaction{ctx: ctx, tx: tx, actorID: viewer}, context: models.AvatarContext{Kind: kind, ParentID: parent, ViewerID: viewer, TargetID: target}}
	ownerQuery := "SELECT organizer_id::text FROM rides WHERE id=$1 FOR SHARE"
	if kind == "club" {
		ownerQuery = "SELECT owner_id::text FROM clubs WHERE id=$1 FOR SHARE"
	}
	if err = tx.QueryRow(ctx, ownerQuery, parent).Scan(&locked.context.OwnerID); err != nil {
		return repositoryError(err)
	}
	// Context precedes accounts; sort account locks so viewer/target order does
	// not introduce a cross-request inversion.
	rows, err := tx.Query(ctx, "SELECT id::text,status,avatar_version,COALESCE(current_avatar_id::text,'') FROM users WHERE id::text=ANY($1::text[]) ORDER BY id FOR SHARE", []string{viewer, target})
	if err != nil {
		return repositoryError(err)
	}
	for rows.Next() {
		var id, status, current string
		var version int64
		if err = rows.Scan(&id, &status, &version, &current); err != nil {
			rows.Close()
			return repositoryError(err)
		}
		if id == viewer {
			locked.context.ViewerStatus = status
		}
		if id == target {
			locked.context.TargetStatus = status
			locked.state = models.AvatarState{Version: version, CurrentID: current}
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return repositoryError(err)
	}
	if locked.context.ViewerStatus == "" || locked.context.TargetStatus == "" {
		return database.ErrNotFound
	}
	stateQuery := "SELECT COALESCE((SELECT state FROM participation WHERE ride_id=$1 AND user_id=$2),''),COALESCE((SELECT state FROM participation WHERE ride_id=$1 AND user_id=$3),'')"
	if kind == "club" {
		stateQuery = "SELECT COALESCE((SELECT state FROM club_memberships WHERE club_id=$1 AND user_id=$2),''),COALESCE((SELECT state FROM club_memberships WHERE club_id=$1 AND user_id=$3),'')"
	}
	if err = tx.QueryRow(ctx, stateQuery, parent, viewer, target).Scan(&locked.context.ViewerState, &locked.context.TargetState); err != nil {
		return repositoryError(err)
	}
	if err = decide(locked); err != nil {
		return err
	}
	return repositoryError(tx.Commit(ctx))
}

var _ database.AvatarTransaction = (*avatarTransaction)(nil)
var _ database.AvatarContextTransaction = (*avatarContextTransaction)(nil)
