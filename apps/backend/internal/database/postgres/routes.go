package postgres

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"github.com/jackc/pgx/v5"
)

const routesAdvisoryLock int64 = 773512006

func (s *Store) WithRoutes(ctx context.Context, decide database.RouteDecision) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return repositoryError(err)
	}
	defer tx.Rollback(ctx)
	// Trust mutations use this same first lock. Never reverse this order.
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", trustAdvisoryLock); err != nil {
		return repositoryError(err)
	}
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", routesAdvisoryLock); err != nil {
		return repositoryError(err)
	}
	if err = decide(&routeTransaction{ctx: ctx, tx: tx}); err != nil {
		return err
	}
	return repositoryError(tx.Commit(ctx))
}

type routeTransaction struct {
	ctx context.Context
	tx  pgx.Tx
}

func (t *routeTransaction) Account(id string) (database.RouteAccount, error) {
	var a database.RouteAccount
	err := t.tx.QueryRow(t.ctx, `SELECT `+userColumns+`,u.status,EXISTS(SELECT 1 FROM moderation_hidden h WHERE h.target_type='user' AND h.target_id=u.id) FROM users u WHERE u.id=$1`, id).Scan(&a.User.ID, &a.User.Profile.DisplayName, &a.User.Profile.City, &a.User.Profile.Languages, &a.User.Profile.Interests, &a.User.Profile.Locale, &a.User.OnboardingComplete, &a.Status, &a.Hidden)
	return a, repositoryError(err)
}
func (t *routeTransaction) Blocked(a, b string) (bool, error) {
	if a == "" || b == "" {
		return false, nil
	}
	var blocked bool
	err := t.tx.QueryRow(t.ctx, `SELECT EXISTS(SELECT 1 FROM user_blocks WHERE (blocker_id=$1 AND blocked_id=$2) OR (blocker_id=$2 AND blocked_id=$1))`, a, b).Scan(&blocked)
	return blocked, repositoryError(err)
}

const routeSummaryColumns = `r.id::text,r.owner_id::text,u.display_name,r.title,r.city,r.description,r.surface,r.kind,r.published,r.version,r.created_at,r.updated_at,r.distance_meters,r.duration_seconds`
const routePublicVisibility = `r.published AND NOT r.hidden AND u.status='active' AND NOT EXISTS(SELECT 1 FROM moderation_hidden h WHERE (h.target_type='route' AND h.target_id=r.id) OR (h.target_type='user' AND h.target_id=u.id)) AND NOT EXISTS(SELECT 1 FROM user_blocks b WHERE (b.blocker_id=NULLIF($1,'')::uuid AND b.blocked_id=r.owner_id) OR (b.blocker_id=r.owner_id AND b.blocked_id=NULLIF($1,'')::uuid))`

func routeSummaryTargets(r *models.CommunityRouteSummary) []any {
	return []any{&r.ID, &r.OwnerID, &r.OwnerName, &r.Title, &r.City, &r.Description, &r.Surface, &r.Kind, &r.Published, &r.Version, &r.CreatedAt, &r.UpdatedAt, &r.DistanceMeters, &r.DurationSeconds}
}
func (t *routeTransaction) Routes(q database.RouteQuery) ([]models.CommunityRouteSummary, error) {
	aliases := make([]string, len(q.CityAliases))
	for i, v := range q.CityAliases {
		aliases[i] = strings.ToLower(v)
	}
	rows, err := t.tx.Query(t.ctx, `SELECT `+routeSummaryColumns+` FROM community_routes r JOIN users u ON u.id=r.owner_id WHERE (($2='mine' AND r.owner_id=NULLIF($1,'')::uuid) OR ($2='public' AND `+routePublicVisibility+`) OR ($2='collection' AND (r.owner_id=NULLIF($1,'')::uuid OR (`+routePublicVisibility+`)))) AND ($3='' OR strpos(lower(r.title||' '||r.description),lower($3))>0) AND (cardinality($4::text[])=0 OR lower(r.city)=ANY($4::text[])) AND ($5='' OR r.id>NULLIF($5,'')::uuid) AND ($6='' OR EXISTS(SELECT 1 FROM route_collection_items i WHERE i.collection_id=NULLIF($6,'')::uuid AND i.route_id=r.id)) ORDER BY r.id LIMIT $7`, q.ViewerID, q.Scope, q.Search, aliases, q.AfterID, q.CollectionID, q.Limit)
	if err != nil {
		return nil, repositoryError(err)
	}
	defer rows.Close()
	out := []models.CommunityRouteSummary{}
	for rows.Next() {
		var r models.CommunityRouteSummary
		if err = rows.Scan(routeSummaryTargets(&r)...); err != nil {
			return nil, repositoryError(err)
		}
		out = append(out, r)
	}
	return out, repositoryError(rows.Err())
}
func (t *routeTransaction) Route(id string) (models.CommunityRoute, error) {
	var r models.CommunityRoute
	var points, segments []byte
	targets := append(routeSummaryTargets(&r.CommunityRouteSummary), &points, &segments, &r.Hidden, &r.SourceTrackID)
	err := t.tx.QueryRow(t.ctx, `SELECT `+routeSummaryColumns+`,r.points,r.segments,(r.hidden OR EXISTS(SELECT 1 FROM moderation_hidden h WHERE h.target_type='route' AND h.target_id=r.id)),coalesce(r.source_track_id::text,'') FROM community_routes r JOIN users u ON u.id=r.owner_id WHERE r.id=$1`, id).Scan(targets...)
	if err != nil {
		return r, repositoryError(err)
	}
	if err = json.Unmarshal(points, &r.Points); err == nil {
		err = json.Unmarshal(segments, &r.Segments)
	}
	return r, err
}
func (t *routeTransaction) RouteCount(owner string) (int, error) {
	var n int
	err := t.tx.QueryRow(t.ctx, `SELECT count(*) FROM community_routes WHERE owner_id=$1`, owner).Scan(&n)
	return n, repositoryError(err)
}
func (t *routeTransaction) SaveRoute(r models.CommunityRoute) error {
	points, err := json.Marshal(r.Points)
	if err != nil {
		return err
	}
	segments, err := json.Marshal(r.Segments)
	if err != nil {
		return err
	}
	_, err = t.tx.Exec(t.ctx, `INSERT INTO community_routes(id,owner_id,title,city,description,surface,kind,published,hidden,version,points,segments,distance_meters,duration_seconds,source_track_id,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::jsonb,$12::jsonb,$13,$14,NULLIF($15,'')::uuid,$16,$17) ON CONFLICT(id) DO UPDATE SET title=excluded.title,city=excluded.city,description=excluded.description,surface=excluded.surface,kind=excluded.kind,published=excluded.published,version=excluded.version,points=excluded.points,segments=excluded.segments,distance_meters=excluded.distance_meters,duration_seconds=excluded.duration_seconds,source_track_id=excluded.source_track_id,updated_at=excluded.updated_at`, r.ID, r.OwnerID, r.Title, r.City, r.Description, r.Surface, r.Kind, r.Published, r.Hidden, r.Version, points, segments, r.DistanceMeters, r.DurationSeconds, r.SourceTrackID, r.CreatedAt, r.UpdatedAt)
	return repositoryError(err)
}
func (t *routeTransaction) DeleteRoute(id string) error {
	_, err := t.tx.Exec(t.ctx, `DELETE FROM community_routes WHERE id=$1`, id)
	return repositoryError(err)
}

func scanTrack(row pgx.Row) (models.Track, error) {
	var r models.Track
	var segments []byte
	err := row.Scan(&r.ID, &r.OwnerID, &r.Name, &segments, &r.DistanceMeters, &r.DurationSeconds, &r.Version, &r.CreatedAt)
	if err != nil {
		return r, repositoryError(err)
	}
	return r, json.Unmarshal(segments, &r.Segments)
}

const trackColumns = `id::text,owner_id::text,name,segments,distance_meters,duration_seconds,version,created_at`

func (t *routeTransaction) Track(id string) (models.Track, error) {
	return scanTrack(t.tx.QueryRow(t.ctx, `SELECT `+trackColumns+` FROM route_tracks WHERE id=$1`, id))
}
func (t *routeTransaction) Tracks(q database.RoutePrivateQuery) ([]models.Track, error) {
	// Geometry is fetched only by the private detail endpoint, keeping pages bounded.
	rows, err := t.tx.Query(t.ctx, `SELECT id::text,owner_id::text,name,'[]'::jsonb,distance_meters,duration_seconds,version,created_at FROM route_tracks WHERE owner_id=$1 AND ($2='' OR id>NULLIF($2,'')::uuid) ORDER BY id LIMIT $3`, q.OwnerID, q.AfterID, q.Limit)
	if err != nil {
		return nil, repositoryError(err)
	}
	defer rows.Close()
	out := []models.Track{}
	for rows.Next() {
		r, e := scanTrack(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, r)
	}
	return out, repositoryError(rows.Err())
}
func (t *routeTransaction) TrackCount(owner string) (int, error) {
	var n int
	err := t.tx.QueryRow(t.ctx, `SELECT count(*) FROM route_tracks WHERE owner_id=$1`, owner).Scan(&n)
	return n, repositoryError(err)
}
func (t *routeTransaction) SaveTrack(r models.Track) error {
	b, err := json.Marshal(r.Segments)
	if err != nil {
		return err
	}
	_, err = t.tx.Exec(t.ctx, `INSERT INTO route_tracks(id,owner_id,name,segments,distance_meters,duration_seconds,version,created_at) VALUES($1,$2,$3,$4::jsonb,$5,$6,$7,$8)`, r.ID, r.OwnerID, r.Name, b, r.DistanceMeters, r.DurationSeconds, r.Version, r.CreatedAt)
	return repositoryError(err)
}
func (t *routeTransaction) DeleteTrack(id string) error {
	_, err := t.tx.Exec(t.ctx, `DELETE FROM route_tracks WHERE id=$1`, id)
	return repositoryError(err)
}

const collectionColumns = `id::text,owner_id::text,name,version,created_at,updated_at`

func scanCollection(row pgx.Row) (models.RouteCollection, error) {
	var c models.RouteCollection
	err := row.Scan(&c.ID, &c.OwnerID, &c.Name, &c.Version, &c.CreatedAt, &c.UpdatedAt)
	return c, repositoryError(err)
}
func (t *routeTransaction) Collection(id string) (models.RouteCollection, error) {
	return scanCollection(t.tx.QueryRow(t.ctx, `SELECT `+collectionColumns+` FROM route_collections WHERE id=$1`, id))
}
func (t *routeTransaction) Collections(q database.RoutePrivateQuery) ([]models.RouteCollection, error) {
	rows, err := t.tx.Query(t.ctx, `SELECT `+collectionColumns+` FROM route_collections WHERE owner_id=$1 AND ($2='' OR id>NULLIF($2,'')::uuid) ORDER BY id LIMIT $3`, q.OwnerID, q.AfterID, q.Limit)
	if err != nil {
		return nil, repositoryError(err)
	}
	defer rows.Close()
	out := []models.RouteCollection{}
	for rows.Next() {
		c, e := scanCollection(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, c)
	}
	return out, repositoryError(rows.Err())
}
func (t *routeTransaction) CollectionCount(owner string) (int, error) {
	var n int
	err := t.tx.QueryRow(t.ctx, `SELECT count(*) FROM route_collections WHERE owner_id=$1`, owner).Scan(&n)
	return n, repositoryError(err)
}
func (t *routeTransaction) SaveCollection(c models.RouteCollection) error {
	_, err := t.tx.Exec(t.ctx, `INSERT INTO route_collections(id,owner_id,name,version,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(id) DO UPDATE SET name=excluded.name,version=excluded.version,updated_at=excluded.updated_at`, c.ID, c.OwnerID, c.Name, c.Version, c.CreatedAt, c.UpdatedAt)
	return repositoryError(err)
}
func (t *routeTransaction) DeleteCollection(id string) error {
	_, err := t.tx.Exec(t.ctx, `DELETE FROM route_collections WHERE id=$1`, id)
	return repositoryError(err)
}
func (t *routeTransaction) CollectionHas(collection, route string) (bool, error) {
	var found bool
	err := t.tx.QueryRow(t.ctx, `SELECT EXISTS(SELECT 1 FROM route_collection_items WHERE collection_id=$1 AND route_id=$2)`, collection, route).Scan(&found)
	return found, repositoryError(err)
}
func (t *routeTransaction) CollectionSize(id string) (int, error) {
	var n int
	err := t.tx.QueryRow(t.ctx, `SELECT count(*) FROM route_collection_items WHERE collection_id=$1`, id).Scan(&n)
	return n, repositoryError(err)
}
func (t *routeTransaction) SetCollectionRoute(collection, route string, add bool, at time.Time) error {
	var err error
	if add {
		_, err = t.tx.Exec(t.ctx, `INSERT INTO route_collection_items(collection_id,route_id,created_at) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, collection, route, at)
	} else {
		_, err = t.tx.Exec(t.ctx, `DELETE FROM route_collection_items WHERE collection_id=$1 AND route_id=$2`, collection, route)
	}
	return repositoryError(err)
}

const conditionColumns = `c.id::text,c.route_id::text,c.author_id::text,u.display_name,c.kind,c.description,c.observed_at,c.resolved,c.version,c.created_at,c.updated_at,(c.hidden OR EXISTS(SELECT 1 FROM moderation_hidden h WHERE h.target_type='route_condition' AND h.target_id=c.id))`

func scanCondition(row pgx.Row) (models.RouteCondition, error) {
	var c models.RouteCondition
	err := row.Scan(&c.ID, &c.RouteID, &c.AuthorID, &c.AuthorName, &c.Kind, &c.Description, &c.ObservedAt, &c.Resolved, &c.Version, &c.CreatedAt, &c.UpdatedAt, &c.Hidden)
	return c, repositoryError(err)
}
func (t *routeTransaction) Condition(id string) (models.RouteCondition, error) {
	return scanCondition(t.tx.QueryRow(t.ctx, `SELECT `+conditionColumns+` FROM route_conditions c JOIN users u ON u.id=c.author_id WHERE c.id=$1`, id))
}
func (t *routeTransaction) Conditions(q database.RouteConditionQuery) ([]models.RouteCondition, error) {
	rows, err := t.tx.Query(t.ctx, `SELECT `+conditionColumns+` FROM route_conditions c JOIN users u ON u.id=c.author_id WHERE c.route_id=$2 AND ($3='' OR c.id>NULLIF($3,'')::uuid) AND NOT c.hidden AND u.status='active' AND NOT EXISTS(SELECT 1 FROM moderation_hidden h WHERE (h.target_type='route_condition' AND h.target_id=c.id) OR (h.target_type='user' AND h.target_id=c.author_id)) AND NOT EXISTS(SELECT 1 FROM user_blocks b WHERE (b.blocker_id=NULLIF($1,'')::uuid AND b.blocked_id=c.author_id) OR (b.blocker_id=c.author_id AND b.blocked_id=NULLIF($1,'')::uuid)) ORDER BY c.id LIMIT $4`, q.ViewerID, q.RouteID, q.AfterID, q.Limit)
	if err != nil {
		return nil, repositoryError(err)
	}
	defer rows.Close()
	out := []models.RouteCondition{}
	for rows.Next() {
		c, e := scanCondition(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, c)
	}
	return out, repositoryError(rows.Err())
}
func (t *routeTransaction) ConditionCount(author string, since time.Time) (int, error) {
	var n int
	err := t.tx.QueryRow(t.ctx, `SELECT coalesce((SELECT conditions_created FROM route_daily_limits WHERE owner_id=$1 AND day=($2::timestamptz AT TIME ZONE 'UTC')::date),0)`, author, since).Scan(&n)
	return n, repositoryError(err)
}
func (t *routeTransaction) SaveCondition(c models.RouteCondition) error {
	_, err := t.tx.Exec(t.ctx, `INSERT INTO route_conditions(id,route_id,author_id,kind,description,observed_at,resolved,hidden,version,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) ON CONFLICT(id) DO UPDATE SET kind=excluded.kind,description=excluded.description,observed_at=excluded.observed_at,resolved=excluded.resolved,version=excluded.version,updated_at=excluded.updated_at`, c.ID, c.RouteID, c.AuthorID, c.Kind, c.Description, c.ObservedAt, c.Resolved, c.Hidden, c.Version, c.CreatedAt, c.UpdatedAt)
	return repositoryError(err)
}
func (t *routeTransaction) DeleteCondition(id string) error {
	_, err := t.tx.Exec(t.ctx, `DELETE FROM route_conditions WHERE id=$1`, id)
	return repositoryError(err)
}

func (t *routeTransaction) RecordConditionCreation(owner string, at time.Time) error {
	if _, err := t.tx.Exec(t.ctx, `DELETE FROM route_daily_limits WHERE owner_id=$1 AND day<($2::timestamptz AT TIME ZONE 'UTC')::date-32`, owner, at); err != nil {
		return repositoryError(err)
	}
	_, err := t.tx.Exec(t.ctx, `INSERT INTO route_daily_limits(owner_id,day,conditions_created) VALUES($1,($2::timestamptz AT TIME ZONE 'UTC')::date,1) ON CONFLICT(owner_id,day) DO UPDATE SET conditions_created=route_daily_limits.conditions_created+1`, owner, at)
	return repositoryError(err)
}
