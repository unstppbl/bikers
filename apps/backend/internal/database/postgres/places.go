package postgres

import (
	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
)

const placesAdvisoryLock int64 = 773512005
const placeColumns = "id::text,name,category,lat,lon,city,address,description,website,phone,opening_hours,source,coalesce(source_ref,''),source_url,coalesce(to_char(source_date,'YYYY-MM-DD'),''),status,version,updated_at"
const submissionColumns = "id::text,kind,coalesce(place_id::text,''),coalesce(base_version,0),proposal,reason,submitter_id::text,coalesce(reviewer_id::text,''),review_reason,status,version,created_at,updated_at"

func scanPlace(row pgx.Row) (models.Place, error) {
	var p models.Place
	err := row.Scan(&p.ID, &p.Name, &p.Category, &p.Lat, &p.Lon, &p.City, &p.Address, &p.Description, &p.Website, &p.Phone, &p.OpeningHours, &p.Source, &p.SourceRef, &p.SourceURL, &p.SourceDate, &p.Status, &p.Version, &p.UpdatedAt)
	return p, repositoryError(err)
}
func scanSubmission(row pgx.Row) (models.PlaceSubmission, error) {
	var s models.PlaceSubmission
	var proposal []byte
	err := row.Scan(&s.ID, &s.Kind, &s.PlaceID, &s.BaseVersion, &proposal, &s.Reason, &s.SubmitterID, &s.ReviewerID, &s.ReviewReason, &s.Status, &s.Version, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return s, repositoryError(err)
	}
	if len(proposal) > 0 {
		err = json.Unmarshal(proposal, &s.Place)
	}
	return s, err
}
func (s *Store) PublishedPlaces(ctx context.Context, q database.PlaceQuery) ([]models.Place, error) {
	aliases := make([]string, len(q.CityAliases))
	for i, city := range q.CityAliases {
		aliases[i] = strings.ToLower(city)
	}
	var west, south, east, north any
	if q.Bounds != nil {
		west, south, east, north = q.Bounds.West, q.Bounds.South, q.Bounds.East, q.Bounds.North
	}
	rows, err := s.pool.Query(ctx, `SELECT `+placeColumns+` FROM places WHERE status='published'
AND ($1='' OR strpos(lower(name||' '||city||' '||address||' '||description),lower($1))>0)
AND ($2='' OR category=$2) AND (cardinality($3::text[])=0 OR lower(city)=ANY($3::text[]))
AND ($4::float8 IS NULL OR (lon BETWEEN $4 AND $6 AND lat BETWEEN $5 AND $7))
AND ($8='' OR id>NULLIF($8,'')::uuid) ORDER BY id LIMIT $9`, q.Search, q.Category, aliases, west, south, east, north, q.AfterID, q.Limit)
	if err != nil {
		return nil, repositoryError(err)
	}
	defer rows.Close()
	out := []models.Place{}
	for rows.Next() {
		p, e := scanPlace(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	return out, repositoryError(rows.Err())
}
func (s *Store) PublishedPlace(ctx context.Context, id string) (models.Place, error) {
	return scanPlace(s.pool.QueryRow(ctx, "SELECT "+placeColumns+" FROM places WHERE id=$1 AND status='published'", id))
}
func (s *Store) WithPlaces(ctx context.Context, actor string, decide database.PlaceDecision) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return repositoryError(err)
	}
	defer tx.Rollback(ctx)
	// Match WithTrust's order so a restriction/role change cannot pass a stale
	// permission check during publication. Imports do not touch account rows.
	if actor != "" {
		if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", trustAdvisoryLock); err != nil {
			return repositoryError(err)
		}
	}
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", placesAdvisoryLock); err != nil {
		return repositoryError(err)
	}
	t := &placeTransaction{ctx: ctx, tx: tx}
	if err = decide(t); err != nil {
		return err
	}
	return repositoryError(tx.Commit(ctx))
}

type placeTransaction struct {
	ctx context.Context
	tx  pgx.Tx
}

func (t *placeTransaction) Account(id string) (models.TrustAccount, error) {
	return (&trustTransaction{ctx: t.ctx, tx: t.tx}).Account(id)
}
func (t *placeTransaction) Place(id string) (models.Place, error) {
	return scanPlace(t.tx.QueryRow(t.ctx, "SELECT "+placeColumns+" FROM places WHERE id=$1 FOR UPDATE", id))
}
func (t *placeTransaction) PlacesBySources(refs []string) (map[string]models.Place, error) {
	rows, err := t.tx.Query(t.ctx, "SELECT "+placeColumns+" FROM places WHERE source_ref=ANY($1::text[]) ORDER BY id FOR UPDATE", refs)
	if err != nil {
		return nil, repositoryError(err)
	}
	defer rows.Close()
	out := map[string]models.Place{}
	for rows.Next() {
		p, e := scanPlace(rows)
		if e != nil {
			return nil, e
		}
		out[p.SourceRef] = p
	}
	return out, repositoryError(rows.Err())
}
func (t *placeTransaction) SavePlaces(places []models.Place) error {
	batch := &pgx.Batch{}
	for _, p := range places {
		batch.Queue(`INSERT INTO places(id,name,category,lat,lon,city,address,description,website,phone,opening_hours,source,source_ref,source_url,source_date,status,version,updated_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NULLIF($13,''),$14,NULLIF($15,'')::date,$16,$17,$18)
ON CONFLICT(id) DO UPDATE SET name=excluded.name,category=excluded.category,lat=excluded.lat,lon=excluded.lon,city=excluded.city,address=excluded.address,description=excluded.description,website=excluded.website,phone=excluded.phone,opening_hours=excluded.opening_hours,source=excluded.source,source_ref=excluded.source_ref,source_url=excluded.source_url,source_date=excluded.source_date,status=excluded.status,version=excluded.version,updated_at=excluded.updated_at`, p.ID, p.Name, p.Category, p.Lat, p.Lon, p.City, p.Address, p.Description, p.Website, p.Phone, p.OpeningHours, p.Source, p.SourceRef, p.SourceURL, p.SourceDate, p.Status, p.Version, p.UpdatedAt)
	}
	result := t.tx.SendBatch(t.ctx, batch)
	return repositoryError(result.Close())
}
func (t *placeTransaction) Submission(id string) (models.PlaceSubmission, error) {
	return scanSubmission(t.tx.QueryRow(t.ctx, "SELECT "+submissionColumns+" FROM place_submissions WHERE id=$1 FOR UPDATE", id))
}
func (t *placeTransaction) Submissions(q database.PlaceSubmissionQuery) ([]models.PlaceSubmission, error) {
	rows, err := t.tx.Query(t.ctx, "SELECT "+submissionColumns+` FROM place_submissions WHERE ($1='' OR submitter_id=NULLIF($1,'')::uuid) AND ($2='' OR status=$2) AND ($3='' OR id>NULLIF($3,'')::uuid) ORDER BY id LIMIT $4`, q.SubmitterID, q.Status, q.AfterID, q.Limit)
	if err != nil {
		return nil, repositoryError(err)
	}
	defer rows.Close()
	out := []models.PlaceSubmission{}
	for rows.Next() {
		s, e := scanSubmission(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, s)
	}
	return out, repositoryError(rows.Err())
}
func (t *placeTransaction) SubmissionCount(actor string, since time.Time) (int, error) {
	var count int
	err := t.tx.QueryRow(t.ctx, "SELECT count(*) FROM place_submissions WHERE submitter_id=$1 AND created_at>=$2", actor, since).Scan(&count)
	return count, repositoryError(err)
}
func (t *placeTransaction) InsertSubmission(s models.PlaceSubmission) error {
	var proposal any
	if s.Place != nil {
		b, err := json.Marshal(s.Place)
		if err != nil {
			return err
		}
		proposal = b
	}
	_, err := t.tx.Exec(t.ctx, `INSERT INTO place_submissions(id,kind,place_id,base_version,proposal,reason,submitter_id,status,version,created_at,updated_at) VALUES($1,$2,NULLIF($3,'')::uuid,NULLIF($4,0),$5,$6,$7,$8,$9,$10,$11)`, s.ID, s.Kind, s.PlaceID, s.BaseVersion, proposal, s.Reason, s.SubmitterID, s.Status, s.Version, s.CreatedAt, s.UpdatedAt)
	return repositoryError(err)
}
func (t *placeTransaction) UpdateSubmission(s models.PlaceSubmission) error {
	_, err := t.tx.Exec(t.ctx, `UPDATE place_submissions SET place_id=NULLIF($2,'')::uuid,status=$3,version=$4,reviewer_id=$5,review_reason=$6,updated_at=$7 WHERE id=$1`, s.ID, s.PlaceID, s.Status, s.Version, s.ReviewerID, s.ReviewReason, s.UpdatedAt)
	return repositoryError(err)
}
