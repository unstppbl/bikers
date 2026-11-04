// Package trust owns reports, personal blocks, moderation, and manual badges.
package trust

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
)

const pageLimit = 50
const BadgeLabel = "Community verified"
const BadgeDisclaimer = "Administrator-assigned community badge; not identity, licence, or safety verification."

type Events interface {
	WakeAll()
	PublishAll(string)
}
type Manager struct {
	repo   database.TrustRepository
	events Events
	now    func() time.Time
}

func New(repo database.TrustRepository, events Events, now func() time.Time) *Manager {
	if now == nil {
		now = time.Now
	}
	return &Manager{repo: repo, events: events, now: now}
}
func (m *Manager) changed() {
	if m.events != nil {
		m.events.WakeAll()
		m.events.PublishAll("trust.changed")
	}
}
func identifier(id string) (string, error) {
	v, err := uuid.Parse(id)
	if err != nil {
		return "", models.Fail("not_found", "Content not found")
	}
	return v.String(), nil
}
func storageError(err error) error {
	if errors.Is(err, database.ErrNotFound) {
		return models.Fail("not_found", "Content not found")
	}
	return err
}
func active(tx database.TrustTransaction, id string) (models.TrustAccount, error) {
	a, err := tx.Account(id)
	if err != nil {
		return a, err
	}
	if a.Status != "active" {
		return a, models.Fail("account_restricted", "Account is restricted")
	}
	return a, nil
}
func admin(tx database.TrustTransaction, id string) error {
	a, err := active(tx, id)
	if err != nil {
		return err
	}
	if !a.Admin {
		return models.Fail("admin_required", "Administrator access required")
	}
	return nil
}
func validKind(kind string) bool {
	switch kind {
	case "user", "ride", "message", "club", "bike", "bike_photo", "club_image", "avatar", "ride_photo", "ride_video", "announcement":
		return true
	}
	return false
}
func imageKind(kind string) bool {
	return kind == "bike_photo" || kind == "club_image" || kind == "avatar" || kind == "ride_photo"
}
func validStatus(status string) bool {
	return status == "open" || status == "reviewing" || status == "resolved" || status == "dismissed"
}
func reason(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 2000 {
		return "", models.Fail("invalid_admin_action", "A reason of 1–2000 bytes is required")
	}
	return value, nil
}
func parseCursor(value string) (models.TrustCursor, error) {
	if value == "" {
		return models.TrustCursor{}, nil
	}
	bad := models.Fail("invalid_cursor", "Invalid cursor")
	if len(value) > 256 {
		return models.TrustCursor{}, bad
	}
	b, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return models.TrustCursor{}, bad
	}
	parts := strings.Split(string(b), "|")
	if len(parts) != 2 {
		return models.TrustCursor{}, bad
	}
	at, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return models.TrustCursor{}, bad
	}
	id, err := uuid.Parse(parts[1])
	if err != nil || id.String() != parts[1] {
		return models.TrustCursor{}, bad
	}
	c := models.TrustCursor{CreatedAt: at, ID: id.String()}
	if encodeCursor(c) != value {
		return models.TrustCursor{}, bad
	}
	return c, nil
}
func encodeCursor(c models.TrustCursor) string {
	return base64.RawURLEncoding.EncodeToString([]byte(c.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + c.ID))
}
func (m *Manager) IsAdmin(ctx context.Context, id string) (bool, error) {
	var out bool
	err := m.repo.WithTrust(ctx, id, func(tx database.TrustTransaction) error { a, e := active(tx, id); out = a.Admin; return e })
	return out, storageError(err)
}
func (m *Manager) PairBlocked(ctx context.Context, a, b string) (bool, error) {
	return m.repo.PairBlocked(ctx, a, b)
}
func (m *Manager) ContentHidden(ctx context.Context, kind, id string) (bool, error) {
	return m.repo.ContentHidden(ctx, kind, id)
}
func (m *Manager) Blocks(ctx context.Context, actor models.User, after string) (models.BlockPage, error) {
	c, err := parseCursor(after)
	if err != nil {
		return models.BlockPage{}, err
	}
	out := models.BlockPage{Blocks: []models.Block{}}
	err = m.repo.WithTrust(ctx, actor.ID, func(tx database.TrustTransaction) error {
		if _, e := active(tx, actor.ID); e != nil {
			return e
		}
		rows, e := tx.Blocks(actor.ID, c, pageLimit+1)
		if e != nil {
			return e
		}
		if len(rows) > pageLimit {
			rows = rows[:pageLimit]
			last := rows[len(rows)-1]
			out.NextCursor = encodeCursor(models.TrustCursor{CreatedAt: last.CreatedAt, ID: last.UserID})
		}
		out.Blocks = rows
		return nil
	})
	return out, storageError(err)
}
func (m *Manager) Block(ctx context.Context, actor models.User, target string, on bool) (bool, error) {
	id, err := identifier(target)
	if err != nil {
		return false, err
	}
	if id == actor.ID {
		return false, models.Fail("invalid_block", "You cannot block yourself")
	}
	err = m.repo.WithTrust(ctx, actor.ID, func(tx database.TrustTransaction) error {
		if _, e := active(tx, actor.ID); e != nil {
			return e
		}
		if _, e := tx.Account(id); e != nil {
			return e
		}
		if on {
			count, e := tx.BlockCount(actor.ID)
			if e != nil {
				return e
			}
			if count >= 500 {
				rows, e := tx.Blocks(actor.ID, models.TrustCursor{}, 501)
				if e != nil {
					return e
				}
				exists := false
				for _, b := range rows {
					if b.UserID == id {
						exists = true
						break
					}
				}
				if !exists {
					return models.Fail("block_limit", "Your block list is full")
				}
			}
		}
		return tx.SetBlock(actor.ID, id, on, m.now().UTC())
	})
	if err == nil {
		m.changed()
	}
	return on, storageError(err)
}
func (m *Manager) Badge(ctx context.Context, userID string) (*models.TrustBadge, error) {
	id, err := identifier(userID)
	if err != nil {
		return nil, err
	}
	var out *models.TrustBadge
	err = m.repo.WithTrust(ctx, "", func(tx database.TrustTransaction) error {
		a, e := tx.Account(id)
		if e != nil {
			return e
		}
		if a.Status != "active" {
			return nil
		}
		hidden, e := tx.Hidden("user", id)
		if e != nil {
			return e
		}
		if hidden {
			return nil
		}
		at, e := tx.Badge(id)
		if e != nil {
			return e
		}
		if at != nil {
			out = &models.TrustBadge{Label: BadgeLabel, AssignedAt: *at, Disclaimer: BadgeDisclaimer}
		}
		return nil
	})
	return out, storageError(err)
}
func reportAllowed(r models.TrustTargetRecord, actor string) bool {
	switch r.Target.Type {
	case "user":
		return r.State != "deleted"
	case "ride", "club":
		return true
	case "bike":
		return r.OwnerID == actor || r.Visibility == "public"
	case "bike_photo":
		return r.State == "ready" && (r.OwnerID == actor || r.Visibility == "public")
	case "club_image":
		return r.State == "ready" && r.Current
	case "avatar":
		return r.State == "ready" && r.Current && r.AuthorStatus == "active" && (r.OwnerID == actor || r.SharedRides > 0 || r.SharedClubs > 0)
	case "message", "ride_video", "announcement":
		return r.OwnerID == actor || r.Participation == "approved"
	case "ride_photo":
		return r.State == "ready" && (r.OwnerID == actor || r.Participation == "approved")
	}
	return false
}

// reportContextVisible keeps guessed child IDs from revealing objects whose
// parent is unavailable in the reporter's actual viewing context.
func reportContextVisible(tx database.TrustTransaction, target models.TrustTargetRecord, actor string) error {
	kind := target.Target.Type
	if kind == "avatar" {
		hidden, err := tx.Hidden("user", target.Target.AuthorID)
		if err != nil {
			return err
		}
		if hidden {
			return database.ErrNotFound
		}
	}
	parent := ""
	switch kind {
	case "bike_photo":
		parent = "bike"
	case "club_image":
		parent = "club"
	}
	if parent == "" {
		return nil
	}
	hidden, err := tx.Hidden(parent, target.Target.ScopeID)
	if err != nil {
		return err
	}
	if !hidden || target.OwnerID == actor {
		return nil
	}
	if parent == "club" && target.Membership == "approved" {
		return nil
	}
	return database.ErrNotFound
}

func (m *Manager) Report(ctx context.Context, actor models.User, input models.ReportInput) (models.ReportReceipt, error) {
	if !validKind(input.TargetType) {
		return models.ReportReceipt{}, models.Fail("invalid_report", "Choose a supported report target")
	}
	id, err := identifier(input.TargetID)
	if err != nil {
		return models.ReportReceipt{}, err
	}
	input.TargetID = id
	if input.Reason != "spam" && input.Reason != "harassment" && input.Reason != "dangerous_content" && input.Reason != "other" {
		return models.ReportReceipt{}, models.Fail("invalid_report", "Choose a report reason")
	}
	input.Details = strings.TrimSpace(input.Details)
	if len(input.Details) > 4000 {
		return models.ReportReceipt{}, models.Fail("invalid_report", "Report details exceed 4000 bytes")
	}
	var out models.ReportReceipt
	err = m.repo.WithTrust(ctx, actor.ID, func(tx database.TrustTransaction) error {
		if _, e := active(tx, actor.ID); e != nil {
			return e
		}
		target, e := tx.Target(input.TargetType, id, actor.ID, false)
		if e != nil {
			return e
		}
		if !reportAllowed(target, actor.ID) {
			return database.ErrNotFound
		}
		if e = reportContextVisible(tx, target, actor.ID); e != nil {
			return e
		}
		hidden, e := tx.Hidden(input.TargetType, id)
		if e != nil {
			return e
		}
		if hidden {
			return database.ErrNotFound
		}
		publicContent := input.TargetType == "club" || input.TargetType == "club_image" || ((input.TargetType == "bike" || input.TargetType == "bike_photo") && target.Visibility == "public")
		if !publicContent && input.TargetType != "user" && !(input.TargetType == "announcement" && target.Target.AuthorID == target.OwnerID) {
			blocked, e := tx.Blocked(actor.ID, target.Target.AuthorID)
			if e != nil {
				return e
			}
			if blocked {
				return database.ErrNotFound
			}
		}
		existing, e := tx.OpenReport(actor.ID, input.TargetType, id)
		if e == nil {
			out = models.ReportReceipt{ID: existing.ID, Status: existing.Status, CreatedAt: existing.CreatedAt}
			return nil
		}
		if !errors.Is(e, database.ErrNotFound) {
			return e
		}
		now := m.now().UTC()
		count, e := tx.ReportCount(actor.ID, now.Add(-time.Hour))
		if e != nil {
			return e
		}
		if count >= 10 {
			return models.Fail("report_rate_limited", "Too many reports; try again later")
		}
		r := models.Report{ID: uuid.NewString(), ReporterID: actor.ID, TargetType: input.TargetType, TargetID: id, Reason: input.Reason, Details: input.Details, Status: "open", Version: 1, Snapshot: target.Target, CreatedAt: now, UpdatedAt: now}
		if e = tx.InsertReport(r); e != nil {
			return e
		}
		out = models.ReportReceipt{ID: r.ID, Status: r.Status, CreatedAt: r.CreatedAt}
		return nil
	})
	return out, storageError(err)
}
func (m *Manager) Reports(ctx context.Context, actor models.User, status, after string) (models.ReportPage, error) {
	if status != "" && !validStatus(status) {
		return models.ReportPage{}, models.Fail("invalid_report", "Choose a valid report status")
	}
	c, err := parseCursor(after)
	if err != nil {
		return models.ReportPage{}, err
	}
	out := models.ReportPage{Reports: []models.Report{}}
	err = m.repo.WithTrust(ctx, actor.ID, func(tx database.TrustTransaction) error {
		if e := admin(tx, actor.ID); e != nil {
			return e
		}
		rows, e := tx.Reports(status, c, pageLimit+1)
		if e != nil {
			return e
		}
		if len(rows) > pageLimit {
			rows = rows[:pageLimit]
			last := rows[len(rows)-1]
			out.NextCursor = encodeCursor(models.TrustCursor{CreatedAt: last.CreatedAt, ID: last.ID})
		}
		out.Reports = rows
		return nil
	})
	return out, storageError(err)
}
func (m *Manager) Review(ctx context.Context, actor models.User, reportID string) (models.Report, models.TrustTarget, error) {
	id, err := identifier(reportID)
	if err != nil {
		return models.Report{}, models.TrustTarget{}, err
	}
	var r models.Report
	err = m.repo.WithTrust(ctx, actor.ID, func(tx database.TrustTransaction) error {
		if e := admin(tx, actor.ID); e != nil {
			return e
		}
		var e error
		r, e = tx.Report(id)
		return e
	})
	return r, r.Snapshot, storageError(err)
}
func (m *Manager) ReviewContent(ctx context.Context, actor models.User, reportID string) (models.PhotoContent, error) {
	id, err := identifier(reportID)
	if err != nil {
		return models.PhotoContent{}, err
	}
	var content models.PhotoContent
	err = m.repo.WithTrust(ctx, actor.ID, func(tx database.TrustTransaction) error {
		if e := admin(tx, actor.ID); e != nil {
			return e
		}
		r, e := tx.Report(id)
		if e != nil {
			return e
		}
		if !imageKind(r.TargetType) {
			return models.Fail("report_content_unavailable", "Reported image is unavailable")
		}
		target, e := tx.Target(r.TargetType, r.TargetID, actor.ID, true)
		if errors.Is(e, database.ErrNotFound) {
			return models.Fail("report_content_unavailable", "Reported image is no longer available")
		}
		if e != nil {
			return e
		}
		if target.State != "ready" || len(target.Content) == 0 {
			return models.Fail("report_content_unavailable", "Reported image is unavailable")
		}
		content.Bytes = target.Content
		return tx.InsertAudit(models.TrustAudit{ID: uuid.NewString(), ActorID: actor.ID, Source: "admin", Action: "review_content", TargetType: r.TargetType, TargetID: r.TargetID, Reason: "Reviewed reported image", CreatedAt: m.now().UTC()})
	})
	return content, storageError(err)
}
func (m *Manager) UpdateReport(ctx context.Context, actor models.User, reportID string, patch models.ReportPatch) (models.Report, error) {
	id, err := identifier(reportID)
	if err != nil {
		return models.Report{}, err
	}
	if !validStatus(patch.Status) || patch.Version < 1 {
		return models.Report{}, models.Fail("invalid_report", "Choose a current report version and valid status")
	}
	why, err := reason(patch.Reason)
	if err != nil {
		return models.Report{}, err
	}
	var out models.Report
	err = m.repo.WithTrust(ctx, actor.ID, func(tx database.TrustTransaction) error {
		if e := admin(tx, actor.ID); e != nil {
			return e
		}
		r, e := tx.Report(id)
		if e != nil {
			return e
		}
		if r.Version != patch.Version {
			return models.Fail("stale_version", "Report changed; refresh and retry")
		}
		r.Status = patch.Status
		r.Version++
		r.UpdatedAt = m.now().UTC()
		if e = tx.UpdateReport(r); e != nil {
			return e
		}
		if e = tx.InsertAudit(models.TrustAudit{ID: uuid.NewString(), ActorID: actor.ID, Source: "admin", Action: "report_" + patch.Status, TargetType: "report", TargetID: id, Reason: why, CreatedAt: r.UpdatedAt}); e != nil {
			return e
		}
		out = r
		return nil
	})
	if err == nil {
		m.changed()
	}
	return out, storageError(err)
}
func (m *Manager) Action(ctx context.Context, actor models.User, input models.TrustActionInput) (models.TrustAudit, error) {
	client, err := uuid.Parse(input.ClientActionID)
	if err != nil {
		return models.TrustAudit{}, models.Fail("invalid_admin_action", "A client action UUID is required")
	}
	input.ClientActionID = client.String()
	id, err := identifier(input.TargetID)
	if err != nil {
		return models.TrustAudit{}, err
	}
	input.TargetID = id
	input.Reason, err = reason(input.Reason)
	if err != nil {
		return models.TrustAudit{}, err
	}
	switch input.Action {
	case "hide_content", "restore_content":
		if !validKind(input.TargetType) {
			return models.TrustAudit{}, models.Fail("invalid_admin_action", "Choose a supported content target")
		}
	case "restrict_account", "restore_account", "assign_badge", "remove_badge":
		if input.TargetType != "user" {
			return models.TrustAudit{}, models.Fail("invalid_admin_action", "This action requires a user target")
		}
	default:
		return models.TrustAudit{}, models.Fail("invalid_admin_action", "Choose a supported moderation action")
	}
	payload, _ := json.Marshal(input)
	hash := sha256.Sum256(payload)
	digest := hex.EncodeToString(hash[:])
	var out models.TrustAudit
	err = m.repo.WithTrust(ctx, actor.ID, func(tx database.TrustTransaction) error {
		if e := admin(tx, actor.ID); e != nil {
			return e
		}
		old, e := tx.AuditAction(actor.ID, input.ClientActionID)
		if e == nil {
			if old.PayloadHash != digest {
				return models.Fail("action_id_conflict", "Action ID already used for another action")
			}
			out = old
			return nil
		}
		if !errors.Is(e, database.ErrNotFound) {
			return e
		}
		now := m.now().UTC()
		switch input.Action {
		case "hide_content", "restore_content":
			if _, e = tx.Target(input.TargetType, id, actor.ID, false); e != nil {
				return e
			}
			e = tx.SetHidden(input.TargetType, id, input.Action == "hide_content", now)
		case "restrict_account", "restore_account":
			a, readErr := tx.Account(id)
			if readErr != nil {
				return readErr
			}
			if a.Status == "deleted" {
				return models.Fail("invalid_admin_action", "Deleted accounts cannot be restored or restricted")
			}
			if input.Action == "restrict_account" {
				if id == actor.ID {
					return models.Fail("invalid_admin_action", "You cannot restrict your own administrator account")
				}
				if e = tx.SetAccountStatus(id, "suspended"); e != nil {
					return e
				}
				e = tx.RevokeUserSessions(id, now)
			} else {
				e = tx.SetAccountStatus(id, "active")
			}
		case "assign_badge", "remove_badge":
			a, readErr := tx.Account(id)
			if readErr != nil {
				return readErr
			}
			if input.Action == "assign_badge" && a.Status != "active" {
				return models.Fail("invalid_admin_action", "A badge requires an active account")
			}
			e = tx.SetBadge(id, input.Action == "assign_badge", now)
		}
		if e != nil {
			return e
		}
		out = models.TrustAudit{ID: uuid.NewString(), ActorID: actor.ID, Source: "admin", Action: input.Action, TargetType: input.TargetType, TargetID: id, Reason: input.Reason, ClientActionID: input.ClientActionID, PayloadHash: digest, CreatedAt: now}
		return tx.InsertAudit(out)
	})
	if err == nil {
		m.changed()
	}
	return out, storageError(err)
}
func (m *Manager) Audit(ctx context.Context, actor models.User, after string) (models.TrustAuditPage, error) {
	c, err := parseCursor(after)
	if err != nil {
		return models.TrustAuditPage{}, err
	}
	out := models.TrustAuditPage{Actions: []models.TrustAudit{}}
	err = m.repo.WithTrust(ctx, actor.ID, func(tx database.TrustTransaction) error {
		if e := admin(tx, actor.ID); e != nil {
			return e
		}
		rows, e := tx.Audit(c, pageLimit+1)
		if e != nil {
			return e
		}
		if len(rows) > pageLimit {
			rows = rows[:pageLimit]
			last := rows[len(rows)-1]
			out.NextCursor = encodeCursor(models.TrustCursor{CreatedAt: last.CreatedAt, ID: last.ID})
		}
		out.Actions = rows
		return nil
	})
	return out, storageError(err)
}

// Role is the explicit operator CLI operation. It never creates an account.
func (m *Manager) Role(ctx context.Context, userID string, grant bool, why string) error {
	id, err := identifier(userID)
	if err != nil {
		return err
	}
	why, err = reason(why)
	if err != nil {
		return err
	}
	err = m.repo.WithTrust(ctx, "", func(tx database.TrustTransaction) error {
		a, e := tx.Account(id)
		if e != nil {
			return e
		}
		if grant && a.Status != "active" {
			return models.Fail("invalid_admin_action", "Administrator access requires an active account")
		}
		now := m.now().UTC()
		if e = tx.SetAdmin(id, grant, now); e != nil {
			return e
		}
		action := "revoke_admin"
		if grant {
			action = "grant_admin"
		}
		return tx.InsertAudit(models.TrustAudit{ID: uuid.NewString(), Source: "cli", Action: action, TargetType: "user", TargetID: id, Reason: why, CreatedAt: now})
	})
	if err == nil {
		m.changed()
	}
	return storageError(err)
}
