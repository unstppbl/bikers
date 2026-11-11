// Package routes owns community route publication, private GPS recordings and
// saved collections, and dated community road-condition reports.
package routes

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"

	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
)

const DefaultLimit = 20
const MaximumLimit = 100
const MaximumRoutes = 100
const MaximumTracks = 100
const MaximumTrackPoints = 20000
const MaximumCollections = 50
const MaximumCollectionRoutes = 200
const DailyConditionLimit = 20

type RoutePlanner interface {
	Plan(context.Context, models.User, []models.MapPoint) (*models.MapPlan, error)
}
type Manager struct {
	repo    database.RoutesRepository
	planner RoutePlanner
	now     func() time.Time
}

func New(repo database.RoutesRepository, planner RoutePlanner, now func() time.Time) *Manager {
	if now == nil {
		now = time.Now
	}
	return &Manager{repo: repo, planner: planner, now: now}
}

func storageError(err error) error {
	if errors.Is(err, database.ErrNotFound) {
		return missing()
	}
	if errors.Is(err, database.ErrConflict) {
		return stale()
	}
	return err
}
func missing() error { return models.Fail("not_found", "Route, recording or collection not found") }
func stale() error {
	return models.Fail("stale_version", "This item changed; reload before trying again")
}
func identifier(value string) (string, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return "", missing()
	}
	return id.String(), nil
}
func active(tx database.RouteTransaction, actor models.User, onboard bool) (database.RouteAccount, error) {
	if actor.ID == "" {
		return database.RouteAccount{}, models.Fail("authentication_required", "Sign in to continue")
	}
	a, err := tx.Account(actor.ID)
	if err != nil {
		return a, err
	}
	if a.Status != "active" {
		return a, models.Fail("account_restricted", "Account is restricted")
	}
	if onboard && !a.User.OnboardingComplete {
		return a, models.Fail("profile_required", "Complete your rider profile first")
	}
	return a, nil
}
func optionalActor(tx database.RouteTransaction, actor models.User) error {
	if actor.ID == "" {
		return nil
	}
	_, err := active(tx, actor, false)
	return err
}
func visible(tx database.RouteTransaction, actor models.User, r models.CommunityRoute) error {
	if actor.ID == r.OwnerID {
		return nil
	}
	if !r.Published || r.Hidden {
		return missing()
	}
	a, err := tx.Account(r.OwnerID)
	if err != nil {
		return err
	}
	if a.Status != "active" || a.Hidden {
		return missing()
	}
	blocked, err := tx.Blocked(actor.ID, r.OwnerID)
	if err != nil {
		return err
	}
	if blocked {
		return missing()
	}
	return nil
}
func textOK(s string, max int) bool { return utf8.ValidString(s) && len(s) <= max }
func normalize(in models.CommunityRouteInput) (models.CommunityRouteInput, error) {
	in.Title = strings.TrimSpace(in.Title)
	in.City = models.NormalizeCity(in.City)
	in.Description = strings.TrimSpace(in.Description)
	in.Surface = strings.TrimSpace(in.Surface)
	if in.Surface == "" {
		in.Surface = "unknown"
	}
	if in.Title == "" || !textOK(in.Title, 200) || !textOK(in.City, 160) || !textOK(in.Description, 4000) {
		return in, models.Fail("invalid_route", "Choose a route title and keep text within the field limits")
	}
	switch in.Surface {
	case "asphalt", "mixed", "unpaved", "unknown":
	default:
		return in, models.Fail("invalid_route", "Choose a valid route surface")
	}
	if in.TrackID != "" && len(in.MapPoints) > 0 {
		return in, models.Fail("invalid_route", "Choose either a GPS recording or planned map points")
	}
	if in.TrackID != "" {
		var err error
		in.TrackID, err = identifier(in.TrackID)
		if err != nil {
			return in, models.Fail("invalid_route", "Choose one of your GPS recordings")
		}
	}
	if len(in.MapPoints) > 0 {
		if len(in.MapPoints) < 2 || len(in.MapPoints) > 10 {
			return in, models.Fail("invalid_route", "Choose between two and ten route points")
		}
		for i, p := range in.MapPoints {
			p.Label = strings.TrimSpace(p.Label)
			if p.Label == "" || !textOK(p.Label, 1000) || !regional(p.Lon, p.Lat) {
				return in, models.Fail("invalid_route", "Choose labelled Kazakhstan route points")
			}
			in.MapPoints[i] = p
		}
	}
	return in, nil
}

type ListInput struct {
	Scope, Search, City, After, CollectionID string
	Limit                                    int
}
type cursor struct{ ID, Binding string }

func binding(value any) string {
	b, _ := json.Marshal(value)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:16])
}
func encode(id, scope string) string {
	b, _ := json.Marshal(cursor{id, scope})
	return base64.RawURLEncoding.EncodeToString(b)
}
func decode(raw, scope string) (string, error) {
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
	var c cursor
	if json.Unmarshal(b, &c) != nil || c.Binding != scope {
		return "", bad
	}
	id, err := identifier(c.ID)
	if err != nil || id != c.ID || encode(id, scope) != raw {
		return "", bad
	}
	return id, nil
}
func pageSize(limit int) (int, error) {
	if limit == 0 {
		return DefaultLimit, nil
	}
	if limit < 1 || limit > MaximumLimit {
		return 0, models.Fail("invalid_query", "Choose a page size from 1 to 100")
	}
	return limit, nil
}

func (m *Manager) List(ctx context.Context, actor models.User, in ListInput) (models.CommunityRoutePage, error) {
	out := models.CommunityRoutePage{Routes: []models.CommunityRouteSummary{}}
	in.Search = strings.TrimSpace(in.Search)
	in.City = models.NormalizeCity(in.City)
	if in.Scope == "" {
		in.Scope = "public"
	}
	if in.Scope != "public" && in.Scope != "mine" || !textOK(in.Search, 160) || !textOK(in.City, 160) {
		return out, models.Fail("invalid_query", "Choose valid route filters")
	}
	limit, err := pageSize(in.Limit)
	if err != nil {
		return out, err
	}
	if in.CollectionID != "" {
		in.CollectionID, err = identifier(in.CollectionID)
		if err != nil {
			return out, err
		}
	}
	q := database.RouteQuery{ViewerID: actor.ID, Scope: in.Scope, Search: in.Search, CityAliases: models.CityAliases(in.City), CollectionID: in.CollectionID, Limit: limit + 1}
	if in.CollectionID != "" {
		q.Scope = "collection"
	}
	b := binding(q)
	q.AfterID, err = decode(in.After, b)
	if err != nil {
		return out, err
	}
	err = m.repo.WithRoutes(ctx, func(tx database.RouteTransaction) error {
		if err := optionalActor(tx, actor); err != nil {
			return err
		}
		if q.Scope == "mine" || q.Scope == "collection" {
			if _, err := active(tx, actor, false); err != nil {
				return err
			}
		}
		if q.CollectionID != "" {
			c, err := tx.Collection(q.CollectionID)
			if err != nil {
				return err
			}
			if c.OwnerID != actor.ID {
				return missing()
			}
		}
		var err error
		out.Routes, err = tx.Routes(q)
		return err
	})
	if err != nil {
		return out, storageError(err)
	}
	if len(out.Routes) > limit {
		out.Routes = out.Routes[:limit]
		out.NextCursor = encode(out.Routes[limit-1].ID, b)
	}
	return out, nil
}
func (m *Manager) View(ctx context.Context, actor models.User, raw string) (models.CommunityRoute, error) {
	var out models.CommunityRoute
	id, err := identifier(raw)
	if err != nil {
		return out, err
	}
	err = m.repo.WithRoutes(ctx, func(tx database.RouteTransaction) error {
		if err := optionalActor(tx, actor); err != nil {
			return err
		}
		r, err := tx.Route(id)
		if err != nil {
			return err
		}
		if err = visible(tx, actor, r); err != nil {
			return err
		}
		out = r
		return nil
	})
	return out, storageError(err)
}

// prepare captures current authorization and geometry, releases the transaction,
// then calls the routing provider. Mutations recheck the captured version.
func (m *Manager) prepare(ctx context.Context, actor models.User, id string, in models.CommunityRouteInput) (models.CommunityRoute, *models.MapPlan, error) {
	var old models.CommunityRoute
	err := m.repo.WithRoutes(ctx, func(tx database.RouteTransaction) error {
		if _, err := active(tx, actor, true); err != nil {
			return err
		}
		if id != "" {
			var err error
			old, err = tx.Route(id)
			if err != nil {
				return err
			}
			if old.OwnerID != actor.ID {
				return missing()
			}
			if old.Version != in.Version {
				return stale()
			}
		}
		return nil
	})
	if err != nil {
		return old, nil, storageError(err)
	}
	if len(in.MapPoints) == 0 || (old.Kind == "planned" && reflect.DeepEqual(old.Points, in.MapPoints)) {
		return old, nil, nil
	}
	if m.planner == nil {
		return old, nil, models.Fail("maps_unavailable", "Maps are temporarily unavailable")
	}
	plan, err := m.planner.Plan(ctx, actor, in.MapPoints)
	if err != nil {
		return old, nil, err
	}
	if plan == nil || plan.Geometry == nil || plan.Geometry.Type != "LineString" || len(plan.Geometry.Coordinates) < 2 || len(plan.Geometry.Coordinates) > 50000 || !boundedNumber(plan.DistanceMeters, 0, 10000000) || !boundedNumber(plan.DurationSeconds, 0, 604800) {
		return old, nil, models.Fail("invalid_route", "The route planner returned invalid geometry")
	}
	for _, p := range plan.Geometry.Coordinates {
		if !regional(p[0], p[1]) {
			return old, nil, models.Fail("invalid_route", "The route planner returned points outside Kazakhstan")
		}
	}
	plan.Points = append([]models.MapPoint{}, in.MapPoints...)
	return old, plan, nil
}
func routeGeometry(tx database.RouteTransaction, actor models.User, in models.CommunityRouteInput, old models.CommunityRoute, plan *models.MapPlan) (models.CommunityRoute, error) {
	r := old
	if in.TrackID != "" {
		track, err := tx.Track(in.TrackID)
		if err != nil {
			return r, err
		}
		if track.OwnerID != actor.ID {
			return r, missing()
		}
		r.Kind = "recorded"
		r.SourceTrackID = track.ID
		r.Points = []models.MapPoint{}
		r.Segments = track.Segments
		r.DistanceMeters = track.DistanceMeters
		r.DurationSeconds = track.DurationSeconds
	} else if plan != nil {
		r.Kind = "planned"
		r.SourceTrackID = ""
		r.Points = plan.Points
		r.Segments = [][][2]float64{plan.Geometry.Coordinates}
		r.DistanceMeters = plan.DistanceMeters
		r.DurationSeconds = plan.DurationSeconds
	}
	if r.Kind == "" {
		return r, models.Fail("invalid_route", "Choose planned map points or a GPS recording")
	}
	if in.Published && (!old.Published || !reflect.DeepEqual(old.Points, r.Points) || !reflect.DeepEqual(old.Segments, r.Segments)) && !in.PublishConsent {
		return r, models.Fail("publication_consent_required", "Confirm that every route coordinate may be published")
	}
	return r, nil
}
func (m *Manager) Create(ctx context.Context, actor models.User, input models.CommunityRouteInput) (models.CommunityRoute, error) {
	return m.save(ctx, actor, "", input)
}
func (m *Manager) Update(ctx context.Context, actor models.User, raw string, input models.CommunityRouteInput) (models.CommunityRoute, error) {
	id, err := identifier(raw)
	if err != nil {
		return models.CommunityRoute{}, err
	}
	return m.save(ctx, actor, id, input)
}
func (m *Manager) save(ctx context.Context, actor models.User, id string, input models.CommunityRouteInput) (models.CommunityRoute, error) {
	var out models.CommunityRoute
	in, err := normalize(input)
	if err != nil {
		return out, err
	}
	if (id == "" && in.Version != 0) || (id != "" && in.Version < 1) {
		return out, stale()
	}
	old, plan, err := m.prepare(ctx, actor, id, in)
	if err != nil {
		return out, err
	}
	at := m.now().UTC()
	err = m.repo.WithRoutes(ctx, func(tx database.RouteTransaction) error {
		a, err := active(tx, actor, true)
		if err != nil {
			return err
		}
		current := old
		if id != "" {
			current, err = tx.Route(id)
			if err != nil {
				return err
			}
			if current.OwnerID != actor.ID {
				return missing()
			}
			if current.Version != in.Version {
				return stale()
			}
		} else {
			n, err := tx.RouteCount(actor.ID)
			if err != nil {
				return err
			}
			if n >= MaximumRoutes {
				return models.Fail("route_limited", "The route limit was reached")
			}
		}
		r, err := routeGeometry(tx, actor, in, current, plan)
		if err != nil {
			return err
		}
		if id == "" {
			r.ID = uuid.NewString()
			r.OwnerID = actor.ID
			r.CreatedAt = at
			r.Version = 0
		}
		r.Title = in.Title
		r.City = in.City
		r.Description = in.Description
		r.Surface = in.Surface
		r.Published = in.Published
		r.OwnerName = a.User.Profile.DisplayName
		r.Version++
		r.UpdatedAt = at
		if err = tx.SaveRoute(r); err != nil {
			return err
		}
		out = r
		return nil
	})
	return out, storageError(err)
}
func (m *Manager) Delete(ctx context.Context, actor models.User, raw string, version int64) error {
	id, err := identifier(raw)
	if err != nil {
		return err
	}
	err = m.repo.WithRoutes(ctx, func(tx database.RouteTransaction) error {
		if _, err := active(tx, actor, true); err != nil {
			return err
		}
		r, err := tx.Route(id)
		if err != nil {
			return err
		}
		if r.OwnerID != actor.ID {
			return missing()
		}
		if version < 1 || r.Version != version {
			return stale()
		}
		return tx.DeleteRoute(id)
	})
	return storageError(err)
}
