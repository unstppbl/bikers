package postgres

import (
	"context"
	"errors"
	"time"

	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"github.com/jackc/pgx/v5"
)

const rideColumns = "id::text,city,description,status,title,starts_at,timezone,meeting_point,destination,route_url,expectations,capacity,plan_version,organizer_id::text,to_char(starts_at AT TIME ZONE timezone,'YYYY-MM-DD'),map_plan"

func scanRide(row pgx.Row) (models.Ride, error) {
	var ride models.Ride
	err := row.Scan(&ride.ID, &ride.City, &ride.Description, &ride.Status, &ride.Title, &ride.StartsAt, &ride.Timezone, &ride.MeetingPoint, &ride.Destination, &ride.RouteURL, &ride.Expectations, &ride.Capacity, &ride.PlanVersion, &ride.OrganizerID, &ride.Date, &ride.MapPlan)
	return ride, repositoryError(err)
}
func readParticipation(ctx context.Context, q querier, rideID, userID string) (string, error) {
	var state string
	err := q.QueryRow(ctx, "SELECT state FROM participation WHERE ride_id=$1 AND user_id=$2", rideID, userID).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return state, repositoryError(err)
}

func (s *Store) ListRides(ctx context.Context, query database.RideListQuery) ([]models.RideListing, error) {
	// Only the already-public local date and UUID enter the page cursor. Exact
	// departure times remain restricted to the full participant/organizer view.
	direction, comparison := "ASC", ">"
	if query.Past {
		direction, comparison = "DESC", "<"
	}
	rows, err := s.pool.Query(ctx, `
		SELECT r.id::text,r.city,to_char(r.starts_at AT TIME ZONE r.timezone,'YYYY-MM-DD'),
		       r.description,r.status,COALESCE(p.state,''),r.organizer_id::text
		FROM rides r
		LEFT JOIN participation p ON p.ride_id=r.id AND p.user_id::text=$1
		WHERE (NOT EXISTS(SELECT 1 FROM moderation_hidden h WHERE h.target_type='ride' AND h.target_id=r.id)
		       OR r.organizer_id::text=$1 OR p.state='approved')
		  AND (COALESCE(cardinality($2::text[]),0)=0 OR EXISTS(SELECT 1 FROM unnest($2::text[]) AS c(city) WHERE lower(trim(r.city))=lower(c.city)))
		  AND ($3='' OR to_char(r.starts_at AT TIME ZONE r.timezone,'YYYY-MM-DD') >= $3)
		  AND ($4='' OR to_char(r.starts_at AT TIME ZONE r.timezone,'YYYY-MM-DD') <= $4)
		  AND (NOT $5::boolean OR r.organizer_id::text=$1)
		  AND (NOT $6::boolean OR r.organizer_id::text<>$1)
		  AND ($7='' OR p.state=$7)
		  AND (NOT $8::boolean OR r.organizer_id::text=$1 OR p.state IN ('approved','requested'))
		  AND (($9::boolean AND (r.starts_at < $10 OR r.status='cancelled'))
		       OR (NOT $9::boolean AND r.status='published' AND r.starts_at >= $10))
		  AND ($11='' OR (to_char(r.starts_at AT TIME ZONE r.timezone,'YYYY-MM-DD'),r.id) `+comparison+` ($11,$12::uuid))
		  AND ($14='' OR strpos(lower(r.description),lower($14))>0)
		ORDER BY to_char(r.starts_at AT TIME ZONE r.timezone,'YYYY-MM-DD') `+direction+`,r.id `+direction+`
		LIMIT $13`, query.UserID, query.CityAliases, query.FromDate, query.ToDate,
		query.OrganizerOnly, query.ExcludeOrganizer, query.ParticipationState, query.RelatedOnly,
		query.Past, query.Now, query.AfterDate, listAfterID(query.AfterID), query.Limit, query.Search)
	if err != nil {
		return nil, repositoryError(err)
	}
	defer rows.Close()
	out := []models.RideListing{}
	for rows.Next() {
		var listing models.RideListing
		v := &listing.Summary
		if err = rows.Scan(&v.ID, &v.City, &v.Date, &v.Description, &v.Status, &v.ParticipationState, &listing.OrganizerID); err != nil {
			return nil, repositoryError(err)
		}
		out = append(out, listing)
	}
	if err = rows.Err(); err != nil {
		return nil, repositoryError(err)
	}
	return out, nil
}

func listAfterID(id string) any {
	if id == "" {
		return nil
	}
	return id
}
func (s *Store) Ride(ctx context.Context, id string) (models.Ride, error) {
	return scanRide(s.pool.QueryRow(ctx, "SELECT "+rideColumns+" FROM rides WHERE id=$1", id))
}
func (s *Store) Participation(ctx context.Context, rideID, userID string) (string, error) {
	return readParticipation(ctx, s.pool, rideID, userID)
}
func (s *Store) Participants(ctx context.Context, rideID string, includeAll bool) ([]models.Participant, error) {
	rows, err := s.pool.Query(ctx, `SELECT u.id::text,u.display_name,p.state FROM participation p JOIN users u ON u.id=p.user_id WHERE p.ride_id=$1 AND ($2 OR p.state='approved') ORDER BY p.updated_at,u.id`, rideID, includeAll)
	if err != nil {
		return nil, repositoryError(err)
	}
	defer rows.Close()
	out := []models.Participant{}
	for rows.Next() {
		var p models.Participant
		if err = rows.Scan(&p.UserID, &p.DisplayName, &p.State); err != nil {
			return nil, repositoryError(err)
		}
		out = append(out, p)
	}
	if err = rows.Err(); err != nil {
		return nil, repositoryError(err)
	}
	return out, nil
}

func (s *Store) CreateRide(ctx context.Context, ride models.Ride, organizerState string, at time.Time, decide database.AccountDecision) (models.Ride, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return models.Ride{}, repositoryError(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", trustAdvisoryLock); err != nil {
		return models.Ride{}, repositoryError(err)
	}
	var owner models.GarageOwner
	if err = tx.QueryRow(ctx, "SELECT id::text,status FROM users WHERE id=$1 FOR SHARE", ride.OrganizerID).Scan(&owner.ID, &owner.Status); err != nil {
		return models.Ride{}, repositoryError(err)
	}
	if err = decide(owner); err != nil {
		return models.Ride{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO rides(id,organizer_id,city,description,title,starts_at,timezone,meeting_point,destination,route_url,expectations,capacity,status,plan_version,created_at,map_plan) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`, ride.ID, ride.OrganizerID, ride.City, ride.Description, ride.Title, ride.StartsAt, ride.Timezone, ride.MeetingPoint, ride.Destination, ride.RouteURL, ride.Expectations, ride.Capacity, ride.Status, ride.PlanVersion, at, ride.MapPlan)
	if err != nil {
		return models.Ride{}, repositoryError(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO participation(ride_id,user_id,state,updated_at) VALUES($1,$2,$3,$4)`, ride.ID, ride.OrganizerID, organizerState, at); err != nil {
		return models.Ride{}, repositoryError(err)
	}
	stored, err := scanRide(tx.QueryRow(ctx, "SELECT "+rideColumns+" FROM rides WHERE id=$1", ride.ID))
	if err != nil {
		return models.Ride{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return models.Ride{}, repositoryError(err)
	}
	return stored, nil
}

func (s *Store) WithRide(ctx context.Context, id string, decide database.RideDecision) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return repositoryError(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", trustAdvisoryLock); err != nil {
		return repositoryError(err)
	}
	ride, err := scanRide(tx.QueryRow(ctx, "SELECT "+rideColumns+" FROM rides WHERE id=$1 FOR UPDATE", id))
	if err != nil {
		return err
	}
	locked := &rideTransaction{ctx: ctx, tx: tx, ride: ride}
	if err = decide(locked); err != nil {
		return err
	}
	return repositoryError(tx.Commit(ctx))
}

type rideTransaction struct {
	ctx  context.Context
	tx   pgx.Tx
	ride models.Ride
}

var _ database.RideTransaction = (*rideTransaction)(nil)

func (t *rideTransaction) Ride() models.Ride { return t.ride }
func (t *rideTransaction) Participation(userID string) (string, error) {
	return readParticipation(t.ctx, t.tx, t.ride.ID, userID)
}
func (t *rideTransaction) ApprovedCount() (int, error) {
	var count int
	err := t.tx.QueryRow(t.ctx, "SELECT count(*) FROM participation WHERE ride_id=$1 AND state='approved'", t.ride.ID).Scan(&count)
	return count, repositoryError(err)
}
func (t *rideTransaction) SetParticipation(userID, state string, at time.Time) error {
	_, err := t.tx.Exec(t.ctx, `INSERT INTO participation(ride_id,user_id,state,updated_at) VALUES($1,$2,$3,$4) ON CONFLICT(ride_id,user_id) DO UPDATE SET media_version=CASE WHEN participation.state IS DISTINCT FROM EXCLUDED.state THEN participation.media_version+1 ELSE participation.media_version END,state=$3,updated_at=$4`, t.ride.ID, userID, state, at)
	return repositoryError(err)
}
func (t *rideTransaction) UpdateRide(ride models.Ride) error {
	// Identity/ownership and message sequence are not changed by a plan update.
	updated, err := scanRide(t.tx.QueryRow(t.ctx, `UPDATE rides SET city=$2,description=$3,title=$4,starts_at=$5,timezone=$6,meeting_point=$7,destination=$8,route_url=$9,expectations=$10,capacity=$11,status=$12,plan_version=$13,map_plan=$14 WHERE id=$1 RETURNING `+rideColumns, t.ride.ID, ride.City, ride.Description, ride.Title, ride.StartsAt, ride.Timezone, ride.MeetingPoint, ride.Destination, ride.RouteURL, ride.Expectations, ride.Capacity, ride.Status, ride.PlanVersion, ride.MapPlan))
	if err != nil {
		return err
	}
	t.ride = updated
	return nil
}

func (t *rideTransaction) Blocked(a, b string) (bool, error) { return trustBlocked(t.ctx, t.tx, a, b) }
func (t *rideTransaction) ContentHidden(kind, id string) (bool, error) {
	return trustContentHidden(t.ctx, t.tx, kind, id)
}
func (t *rideTransaction) ParticipationUsers() ([]models.Participant, error) {
	rows, err := t.tx.Query(t.ctx, `SELECT p.user_id::text,u.display_name,p.state FROM participation p JOIN users u ON u.id=p.user_id WHERE p.ride_id=$1 ORDER BY p.updated_at,u.id`, t.ride.ID)
	if err != nil {
		return nil, repositoryError(err)
	}
	defer rows.Close()
	out := []models.Participant{}
	for rows.Next() {
		var p models.Participant
		if err = rows.Scan(&p.UserID, &p.DisplayName, &p.State); err != nil {
			return nil, repositoryError(err)
		}
		out = append(out, p)
	}
	return out, repositoryError(rows.Err())
}

const announcementColumns = "a.id::text,a.sequence,a.owner_id::text,u.display_name,a.body,a.created_at,a.client_announcement_id::text"

func scanAnnouncement(row pgx.Row) (models.Announcement, error) {
	var a models.Announcement
	err := row.Scan(&a.ID, &a.Sequence, &a.OwnerID, &a.OwnerName, &a.Body, &a.CreatedAt, &a.ClientAnnouncementID)
	return a, repositoryError(err)
}
func (t *rideTransaction) Announcement(clientID string) (models.Announcement, error) {
	return scanAnnouncement(t.tx.QueryRow(t.ctx, "SELECT "+announcementColumns+" FROM announcements a JOIN users u ON u.id=a.owner_id WHERE a.ride_id=$1 AND a.client_announcement_id=$2", t.ride.ID, clientID))
}
func (t *rideTransaction) Announcements(before int64, limit int) ([]models.Announcement, error) {
	rows, err := t.tx.Query(t.ctx, "SELECT "+announcementColumns+" FROM announcements a JOIN users u ON u.id=a.owner_id WHERE a.ride_id=$1 AND ($2::bigint=0 OR a.sequence<$2) AND NOT EXISTS(SELECT 1 FROM moderation_hidden h WHERE h.target_type='announcement' AND h.target_id=a.id) ORDER BY a.sequence DESC LIMIT $3", t.ride.ID, before, limit)
	if err != nil {
		return nil, repositoryError(err)
	}
	defer rows.Close()
	out := []models.Announcement{}
	for rows.Next() {
		a, err := scanAnnouncement(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, repositoryError(rows.Err())
}
func (t *rideTransaction) AppendAnnouncement(a models.Announcement) (models.Announcement, error) {
	_, err := t.tx.Exec(t.ctx, `INSERT INTO announcements(id,ride_id,owner_id,client_announcement_id,body,created_at) VALUES($1,$2,$3,$4,$5,$6)`, a.ID, t.ride.ID, a.OwnerID, a.ClientAnnouncementID, a.Body, a.CreatedAt)
	if err != nil {
		return a, repositoryError(err)
	}
	return t.Announcement(a.ClientAnnouncementID)
}

func (t *rideTransaction) AccountStatus(user string) (string, error) {
	var status string
	err := t.tx.QueryRow(t.ctx, "SELECT status FROM users WHERE id=$1 FOR SHARE", user).Scan(&status)
	return status, repositoryError(err)
}
