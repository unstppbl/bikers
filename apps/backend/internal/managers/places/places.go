// Package places owns public discovery, moderated contributions and imported
// useful-place publication. Private proposals are never public Place values.
package places

import (
	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"strings"
	"time"
)

const DefaultLimit = 50
const MaximumLimit = 100
const DailySubmissionLimit = 20

type Manager struct {
	repo       database.PlacesRepository
	now        func() time.Time
	archiveURL string
}

func New(repo database.PlacesRepository, archiveURL string, now func() time.Time) *Manager {
	if now == nil {
		now = time.Now
	}
	return &Manager{repo: repo, now: now, archiveURL: archiveURL}
}
func (m *Manager) Config() models.MapConfig {
	return models.MapConfig{Enabled: m.archiveURL != "", ArchiveURL: m.archiveURL}
}
func storageError(err error) error {
	if errors.Is(err, database.ErrNotFound) {
		return models.Fail("not_found", "Place or submission not found")
	}
	return err
}
func identifier(raw string) (string, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return "", models.Fail("not_found", "Place or submission not found")
	}
	return id.String(), nil
}
func active(tx database.PlaceTransaction, id string, admin bool) error {
	a, err := tx.Account(id)
	if err != nil {
		return err
	}
	if a.Status != "active" {
		return models.Fail("account_restricted", "Account is restricted")
	}
	if admin && !a.Admin {
		return models.Fail("admin_required", "Administrator access required")
	}
	return nil
}
func reason(raw string) (string, error) {
	v := strings.TrimSpace(raw)
	if v == "" || len(v) > 2000 {
		return "", models.Fail("invalid_submission", "A reason of 1–2000 bytes is required")
	}
	return v, nil
}

type ListInput struct {
	Search, Category, City, After string
	Bounds                        *models.PlaceBounds
	Limit                         int
}
type pageCursor struct {
	ID    string `json:"id"`
	Scope string `json:"scope"`
}

func scope(value any) string {
	b, _ := json.Marshal(value)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:16])
}
func encodeCursor(id, binding string) string {
	b, _ := json.Marshal(pageCursor{ID: id, Scope: binding})
	return base64.RawURLEncoding.EncodeToString(b)
}
func parseCursor(raw, binding string) (string, error) {
	if raw == "" {
		return "", nil
	}
	bad := models.Fail("invalid_cursor", "Refresh this list to restart paging")
	if len(raw) > 256 {
		return "", bad
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return "", bad
	}
	var c pageCursor
	if json.Unmarshal(b, &c) != nil || c.Scope != binding {
		return "", bad
	}
	id, e := uuid.Parse(c.ID)
	if e != nil || id.String() != c.ID || encodeCursor(c.ID, binding) != raw {
		return "", bad
	}
	return c.ID, nil
}
func (m *Manager) List(ctx context.Context, in ListInput) (models.PlacePage, error) {
	out := models.PlacePage{Places: []models.Place{}}
	in.Search = strings.TrimSpace(in.Search)
	in.City = models.NormalizeCity(in.City)
	in.Category = strings.TrimSpace(in.Category)
	if in.Limit == 0 {
		in.Limit = DefaultLimit
	}
	if len(in.Search) > 160 || len(in.City) > 160 || (in.Category != "" && !models.PlaceCategory(in.Category)) || in.Limit < 1 || in.Limit > MaximumLimit {
		return out, models.Fail("invalid_query", "Choose valid place filters and a page size from 1 to 100")
	}
	if b := in.Bounds; b != nil {
		if !models.ValidCoordinate(b.South, b.West) || !models.ValidCoordinate(b.North, b.East) || b.West < 46 || b.East > 87.5 || b.South < 40.5 || b.North > 55.5 || b.West >= b.East || b.South >= b.North {
			return out, models.Fail("invalid_query", "Choose an ordered Kazakhstan map area")
		}
	}
	q := database.PlaceQuery{Search: in.Search, Category: in.Category, CityAliases: models.CityAliases(in.City), Bounds: in.Bounds, Limit: in.Limit + 1}
	binding := scope(q)
	after, err := parseCursor(in.After, binding)
	if err != nil {
		return out, err
	}
	q.AfterID = after
	rows, err := m.repo.PublishedPlaces(ctx, q)
	if err != nil {
		return out, storageError(err)
	}
	if len(rows) > in.Limit {
		rows = rows[:in.Limit]
		out.NextCursor = encodeCursor(rows[len(rows)-1].ID, binding)
	}
	out.Places = rows
	return out, nil
}
func (m *Manager) View(ctx context.Context, raw string) (models.Place, error) {
	id, err := identifier(raw)
	if err != nil {
		return models.Place{}, err
	}
	p, err := m.repo.PublishedPlace(ctx, id)
	return p, storageError(err)
}

func (m *Manager) Submit(ctx context.Context, actor models.User, in models.PlaceSubmissionInput) (models.PlaceSubmission, error) {
	var out models.PlaceSubmission
	if !actor.OnboardingComplete {
		return out, models.Fail("profile_required", "Complete your rider profile first")
	}
	var err error
	in.Reason, err = reason(in.Reason)
	if err != nil {
		return out, err
	}
	if in.Kind != "new" && in.Kind != "correction" && in.Kind != "closed" {
		return out, models.Fail("invalid_submission", "Choose a new place, correction or closure")
	}
	if in.Kind == "new" {
		if in.PlaceID != "" || in.BaseVersion != 0 || in.Place == nil {
			return out, models.Fail("invalid_submission", "A new place needs full place details")
		}
	} else {
		in.PlaceID, err = identifier(in.PlaceID)
		if err != nil {
			return out, err
		}
		if in.BaseVersion < 1 || (in.Kind == "correction" && in.Place == nil) || (in.Kind == "closed" && in.Place != nil) {
			return out, models.Fail("invalid_submission", "Choose the current place version and matching proposal")
		}
	}
	if in.Place != nil {
		p, e := models.ValidatePlace(*in.Place)
		if e != nil {
			return out, e
		}
		in.Place = &p
	}
	at := m.now().UTC()
	out = models.PlaceSubmission{ID: uuid.NewString(), PlaceSubmissionInput: in, SubmitterID: actor.ID, Status: "pending", Version: 1, CreatedAt: at, UpdatedAt: at}
	err = m.repo.WithPlaces(ctx, actor.ID, func(tx database.PlaceTransaction) error {
		if err := active(tx, actor.ID, false); err != nil {
			return err
		}
		count, err := tx.SubmissionCount(actor.ID, at.Truncate(24*time.Hour))
		if err != nil {
			return err
		}
		if count >= DailySubmissionLimit {
			return models.Fail("submission_limited", "The daily place submission limit was reached")
		}
		if in.Kind != "new" {
			p, e := tx.Place(in.PlaceID)
			if e != nil {
				return e
			}
			if p.Status != "published" {
				return models.Fail("not_found", "Place not found")
			}
			if p.Version != in.BaseVersion {
				return models.Fail("stale_version", "Place changed; reload its current details before submitting")
			}
		}
		return tx.InsertSubmission(out)
	})
	out.SubmitterID = ""
	return out, storageError(err)
}
func (m *Manager) Submissions(ctx context.Context, actor models.User, admin bool, status, after string) (models.PlaceSubmissionPage, error) {
	out := models.PlaceSubmissionPage{Submissions: []models.PlaceSubmission{}}
	q := database.PlaceSubmissionQuery{Limit: DefaultLimit + 1}
	if admin {
		if status == "" {
			status = "pending"
		}
		if status != "pending" && status != "approved" && status != "rejected" {
			return out, models.Fail("invalid_query", "Choose pending, approved or rejected submissions")
		}
		q.Status = status
	} else {
		q.SubmitterID = actor.ID
	}
	binding := scope(struct {
		Actor string
		Admin bool
		Query database.PlaceSubmissionQuery
	}{actor.ID, admin, q})
	id, err := parseCursor(after, binding)
	if err != nil {
		return out, err
	}
	q.AfterID = id
	err = m.repo.WithPlaces(ctx, actor.ID, func(tx database.PlaceTransaction) error {
		if e := active(tx, actor.ID, admin); e != nil {
			return e
		}
		var e error
		out.Submissions, e = tx.Submissions(q)
		return e
	})
	if err != nil {
		return out, storageError(err)
	}
	if len(out.Submissions) > DefaultLimit {
		out.Submissions = out.Submissions[:DefaultLimit]
		out.NextCursor = encodeCursor(out.Submissions[DefaultLimit-1].ID, binding)
	}
	if !admin {
		for i := range out.Submissions {
			out.Submissions[i].SubmitterID = ""
			out.Submissions[i].ReviewerID = ""
		}
	}
	return out, nil
}
func (m *Manager) Review(ctx context.Context, actor models.User, raw string, in models.PlaceReviewInput) (models.PlaceSubmission, error) {
	var out models.PlaceSubmission
	id, err := identifier(raw)
	if err != nil {
		return out, err
	}
	in.Reason, err = reason(in.Reason)
	if err != nil {
		return out, err
	}
	if in.Version < 1 || (in.Status != "approved" && in.Status != "rejected") {
		return out, models.Fail("invalid_submission", "Choose approval or rejection and the current submission version")
	}
	err = m.repo.WithPlaces(ctx, actor.ID, func(tx database.PlaceTransaction) error {
		if e := active(tx, actor.ID, true); e != nil {
			return e
		}
		s, e := tx.Submission(id)
		if e != nil {
			return e
		}
		if s.Version != in.Version || s.Status != "pending" {
			return models.Fail("stale_version", "Submission changed; refresh the review queue")
		}
		if in.Status == "approved" {
			if e := active(tx, s.SubmitterID, false); e != nil {
				return e
			}
			p := models.Place{ID: uuid.NewString(), Source: "community", Status: "published", Version: 1, SourceDate: m.now().UTC().Format("2006-01-02")}
			if s.Kind != "new" {
				p, e = tx.Place(s.PlaceID)
				if e != nil {
					return e
				}
				if p.Version != s.BaseVersion || p.Status != "published" {
					return models.Fail("stale_version", "Place changed; reject this stale suggestion and request a fresh correction")
				}
				p.Version++
				p.Source = "community"
			}
			if s.Kind == "closed" {
				p.Status = "closed"
			} else {
				p.PlaceInput = *s.Place
			}
			p.UpdatedAt = m.now().UTC()
			if e = tx.SavePlaces([]models.Place{p}); e != nil {
				return e
			}
			s.PlaceID = p.ID
		}
		s.Status = in.Status
		s.Version++
		s.ReviewerID = actor.ID
		s.ReviewReason = in.Reason
		s.UpdatedAt = m.now().UTC()
		if e = tx.UpdateSubmission(s); e != nil {
			return e
		}
		out = s
		return nil
	})
	return out, storageError(err)
}
