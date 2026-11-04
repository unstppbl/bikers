package database

import (
	"bikers.kz/backend/internal/models"
	"context"
	"time"
)

type TrustDecision func(TrustTransaction) error
type TrustRepository interface {
	WithTrust(context.Context, string, TrustDecision) error
	PairBlocked(context.Context, string, string) (bool, error)
	ContentHidden(context.Context, string, string) (bool, error)
}

// WithTrust owns the first/global trust lock and all following account locks.
// Callbacks decide permission/state; these methods only map persisted records.
type TrustTransaction interface {
	Account(string) (models.TrustAccount, error)
	SetAdmin(string, bool, time.Time) error
	SetAccountStatus(string, string) error
	RevokeUserSessions(string, time.Time) error
	Blocked(string, string) (bool, error)
	SetBlock(string, string, bool, time.Time) error
	BlockCount(string) (int, error)
	Blocks(string, models.TrustCursor, int) ([]models.Block, error)
	Target(string, string, string, bool) (models.TrustTargetRecord, error)
	Hidden(string, string) (bool, error)
	SetHidden(string, string, bool, time.Time) error
	Badge(string) (*time.Time, error)
	SetBadge(string, bool, time.Time) error
	OpenReport(string, string, string) (models.Report, error)
	ReportCount(string, time.Time) (int, error)
	InsertReport(models.Report) error
	Report(string) (models.Report, error)
	Reports(string, models.TrustCursor, int) ([]models.Report, error)
	UpdateReport(models.Report) error
	AuditAction(string, string) (models.TrustAudit, error)
	InsertAudit(models.TrustAudit) error
	Audit(models.TrustCursor, int) ([]models.TrustAudit, error)
}
