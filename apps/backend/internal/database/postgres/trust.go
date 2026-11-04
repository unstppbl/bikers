package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"github.com/jackc/pgx/v5"
)

const trustAdvisoryLock int64 = 773512004

func trustBlocked(ctx context.Context, q querier, viewer, target string) (bool, error) {
	if viewer == "" || target == "" || viewer == target {
		return false, nil
	}
	var blocked bool
	err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM user_blocks WHERE (blocker_id=$1 AND blocked_id=$2) OR (blocker_id=$2 AND blocked_id=$1))`, viewer, target).Scan(&blocked)
	return blocked, repositoryError(err)
}
func trustContentHidden(ctx context.Context, q querier, kind, id string) (bool, error) {
	var hidden bool
	err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM moderation_hidden WHERE target_type=$1 AND target_id=$2)`, kind, id).Scan(&hidden)
	return hidden, repositoryError(err)
}
func (s *Store) PairBlocked(ctx context.Context, a, b string) (bool, error) {
	return trustBlocked(ctx, s.pool, a, b)
}
func (s *Store) ContentHidden(ctx context.Context, kind, id string) (bool, error) {
	return trustContentHidden(ctx, s.pool, kind, id)
}
func (s *Store) WithTrust(ctx context.Context, actor string, decide database.TrustDecision) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return repositoryError(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, trustAdvisoryLock); err != nil {
		return repositoryError(err)
	}
	t := &trustTransaction{ctx: ctx, tx: tx}
	if actor != "" {
		if _, err = t.Account(actor); err != nil {
			return err
		}
	}
	if err = decide(t); err != nil {
		return err
	}
	return repositoryError(tx.Commit(ctx))
}

type trustTransaction struct {
	ctx context.Context
	tx  pgx.Tx
}

func (t *trustTransaction) Account(id string) (models.TrustAccount, error) {
	var out models.TrustAccount
	err := t.tx.QueryRow(t.ctx, `SELECT u.id::text,u.display_name,u.status,EXISTS(SELECT 1 FROM trust_admins a WHERE a.user_id=u.id) FROM users u WHERE u.id=$1 FOR UPDATE OF u`, id).Scan(&out.ID, &out.DisplayName, &out.Status, &out.Admin)
	return out, repositoryError(err)
}
func (t *trustTransaction) SetAdmin(id string, on bool, at time.Time) error {
	if on {
		_, err := t.tx.Exec(t.ctx, `INSERT INTO trust_admins(user_id,granted_at) VALUES($1,$2) ON CONFLICT(user_id) DO NOTHING`, id, at)
		return repositoryError(err)
	}
	_, err := t.tx.Exec(t.ctx, `DELETE FROM trust_admins WHERE user_id=$1`, id)
	return repositoryError(err)
}
func (t *trustTransaction) SetAccountStatus(id, status string) error {
	_, err := t.tx.Exec(t.ctx, `UPDATE users SET status=$2 WHERE id=$1`, id, status)
	return repositoryError(err)
}
func (t *trustTransaction) RevokeUserSessions(id string, at time.Time) error {
	_, err := t.tx.Exec(t.ctx, `UPDATE sessions SET revoked_at=$2 WHERE user_id=$1 AND revoked_at IS NULL`, id, at)
	return repositoryError(err)
}
func (t *trustTransaction) Blocked(a, b string) (bool, error) { return trustBlocked(t.ctx, t.tx, a, b) }
func (t *trustTransaction) Hidden(kind, id string) (bool, error) {
	return trustContentHidden(t.ctx, t.tx, kind, id)
}
func (t *trustTransaction) SetHidden(kind, id string, on bool, at time.Time) error {
	if on {
		_, err := t.tx.Exec(t.ctx, `INSERT INTO moderation_hidden(target_type,target_id,hidden_at) VALUES($1,$2,$3) ON CONFLICT(target_type,target_id) DO NOTHING`, kind, id, at)
		return repositoryError(err)
	}
	_, err := t.tx.Exec(t.ctx, `DELETE FROM moderation_hidden WHERE target_type=$1 AND target_id=$2`, kind, id)
	return repositoryError(err)
}
func (t *trustTransaction) SetBlock(a, b string, on bool, at time.Time) error {
	if on {
		_, err := t.tx.Exec(t.ctx, `INSERT INTO user_blocks(blocker_id,blocked_id,created_at) VALUES($1,$2,$3) ON CONFLICT(blocker_id,blocked_id) DO NOTHING`, a, b, at)
		return repositoryError(err)
	}
	_, err := t.tx.Exec(t.ctx, `DELETE FROM user_blocks WHERE blocker_id=$1 AND blocked_id=$2`, a, b)
	return repositoryError(err)
}
func (t *trustTransaction) BlockCount(id string) (int, error) {
	var count int
	err := t.tx.QueryRow(t.ctx, `SELECT count(*) FROM user_blocks WHERE blocker_id=$1`, id).Scan(&count)
	return count, repositoryError(err)
}
func (t *trustTransaction) Blocks(id string, c models.TrustCursor, limit int) ([]models.Block, error) {
	rows, err := t.tx.Query(t.ctx, `SELECT u.id::text,u.display_name,b.created_at FROM user_blocks b JOIN users u ON u.id=b.blocked_id WHERE b.blocker_id=$1 AND ($2::timestamptz IS NULL OR (b.created_at,b.blocked_id)<($2,$3::uuid)) ORDER BY b.created_at DESC,b.blocked_id DESC LIMIT $4`, id, cursorTime(c), cursorID(c), limit)
	if err != nil {
		return nil, repositoryError(err)
	}
	defer rows.Close()
	out := []models.Block{}
	for rows.Next() {
		var b models.Block
		if err = rows.Scan(&b.UserID, &b.DisplayName, &b.CreatedAt); err != nil {
			return nil, repositoryError(err)
		}
		out = append(out, b)
	}
	return out, repositoryError(rows.Err())
}
func (t *trustTransaction) Badge(id string) (*time.Time, error) {
	var at time.Time
	err := t.tx.QueryRow(t.ctx, `SELECT assigned_at FROM community_badges WHERE user_id=$1`, id).Scan(&at)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &at, repositoryError(err)
}
func (t *trustTransaction) SetBadge(id string, on bool, at time.Time) error {
	if on {
		_, err := t.tx.Exec(t.ctx, `INSERT INTO community_badges(user_id,assigned_at) VALUES($1,$2) ON CONFLICT(user_id) DO NOTHING`, id, at)
		return repositoryError(err)
	}
	_, err := t.tx.Exec(t.ctx, `DELETE FROM community_badges WHERE user_id=$1`, id)
	return repositoryError(err)
}

const trustReportColumns = `id::text,reporter_id::text,target_type,target_id::text,reason,details,status,version,snapshot,created_at,updated_at`

func scanTrustReport(row pgx.Row) (models.Report, error) {
	var out models.Report
	var snapshot []byte
	err := row.Scan(&out.ID, &out.ReporterID, &out.TargetType, &out.TargetID, &out.Reason, &out.Details, &out.Status, &out.Version, &snapshot, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return out, repositoryError(err)
	}
	err = json.Unmarshal(snapshot, &out.Snapshot)
	return out, err
}
func (t *trustTransaction) OpenReport(reporter, kind, id string) (models.Report, error) {
	return scanTrustReport(t.tx.QueryRow(t.ctx, `SELECT `+trustReportColumns+` FROM trust_reports WHERE reporter_id=$1 AND target_type=$2 AND target_id=$3 AND status IN ('open','reviewing')`, reporter, kind, id))
}
func (t *trustTransaction) ReportCount(id string, since time.Time) (int, error) {
	var count int
	err := t.tx.QueryRow(t.ctx, `SELECT count(*) FROM trust_reports WHERE reporter_id=$1 AND created_at>=$2`, id, since).Scan(&count)
	return count, repositoryError(err)
}
func (t *trustTransaction) InsertReport(r models.Report) error {
	snapshot, err := json.Marshal(r.Snapshot)
	if err != nil {
		return err
	}
	_, err = t.tx.Exec(t.ctx, `INSERT INTO trust_reports(id,reporter_id,target_type,target_id,reason,details,status,version,snapshot,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, r.ID, r.ReporterID, r.TargetType, r.TargetID, r.Reason, r.Details, r.Status, r.Version, snapshot, r.CreatedAt, r.UpdatedAt)
	return repositoryError(err)
}
func (t *trustTransaction) Report(id string) (models.Report, error) {
	return scanTrustReport(t.tx.QueryRow(t.ctx, `SELECT `+trustReportColumns+` FROM trust_reports WHERE id=$1 FOR UPDATE`, id))
}
func (t *trustTransaction) Reports(status string, c models.TrustCursor, limit int) ([]models.Report, error) {
	rows, err := t.tx.Query(t.ctx, `SELECT `+trustReportColumns+` FROM trust_reports WHERE ($1='' OR status=$1) AND ($2::timestamptz IS NULL OR (created_at,id)<($2,$3::uuid)) ORDER BY created_at DESC,id DESC LIMIT $4`, status, cursorTime(c), cursorID(c), limit)
	if err != nil {
		return nil, repositoryError(err)
	}
	defer rows.Close()
	out := []models.Report{}
	for rows.Next() {
		r, e := scanTrustReport(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, r)
	}
	return out, repositoryError(rows.Err())
}
func (t *trustTransaction) UpdateReport(r models.Report) error {
	_, err := t.tx.Exec(t.ctx, `UPDATE trust_reports SET status=$2,version=$3,updated_at=$4 WHERE id=$1`, r.ID, r.Status, r.Version, r.UpdatedAt)
	return repositoryError(err)
}

const trustAuditColumns = `id::text,COALESCE(actor_id::text,''),source,action,target_type,target_id::text,reason,COALESCE(client_action_id::text,''),payload_hash,created_at`

func scanTrustAudit(row pgx.Row) (models.TrustAudit, error) {
	var out models.TrustAudit
	err := row.Scan(&out.ID, &out.ActorID, &out.Source, &out.Action, &out.TargetType, &out.TargetID, &out.Reason, &out.ClientActionID, &out.PayloadHash, &out.CreatedAt)
	return out, repositoryError(err)
}
func (t *trustTransaction) AuditAction(actor, id string) (models.TrustAudit, error) {
	return scanTrustAudit(t.tx.QueryRow(t.ctx, `SELECT `+trustAuditColumns+` FROM trust_audit WHERE actor_id=$1 AND client_action_id=$2`, actor, id))
}
func (t *trustTransaction) InsertAudit(a models.TrustAudit) error {
	_, err := t.tx.Exec(t.ctx, `INSERT INTO trust_audit(id,actor_id,source,action,target_type,target_id,reason,client_action_id,payload_hash,created_at) VALUES($1,NULLIF($2,'')::uuid,$3,$4,$5,$6,$7,NULLIF($8,'')::uuid,$9,$10)`, a.ID, a.ActorID, a.Source, a.Action, a.TargetType, a.TargetID, a.Reason, a.ClientActionID, a.PayloadHash, a.CreatedAt)
	return repositoryError(err)
}
func (t *trustTransaction) Audit(c models.TrustCursor, limit int) ([]models.TrustAudit, error) {
	rows, err := t.tx.Query(t.ctx, `SELECT `+trustAuditColumns+` FROM trust_audit WHERE ($1::timestamptz IS NULL OR (created_at,id)<($1,$2::uuid)) ORDER BY created_at DESC,id DESC LIMIT $3`, cursorTime(c), cursorID(c), limit)
	if err != nil {
		return nil, repositoryError(err)
	}
	defer rows.Close()
	out := []models.TrustAudit{}
	for rows.Next() {
		a, e := scanTrustAudit(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, a)
	}
	return out, repositoryError(rows.Err())
}
func cursorTime(c models.TrustCursor) any {
	if c.ID == "" {
		return nil
	}
	return c.CreatedAt
}
func cursorID(c models.TrustCursor) any {
	if c.ID == "" {
		return nil
	}
	return c.ID
}
