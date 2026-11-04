package postgres

import (
	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"context"
	"github.com/jackc/pgx/v5"
	"time"
)

const clubColumns = "id::text,owner_id::text,name,city,description,status,version"

func scanClub(row pgx.Row) (models.Club, error) {
	var club models.Club
	err := row.Scan(&club.ID, &club.OwnerID, &club.Name, &club.City, &club.Description, &club.Status, &club.Version)
	return club, repositoryError(err)
}
func countClubMembers(ctx context.Context, q querier, id string) (int, error) {
	var count int
	err := q.QueryRow(ctx, "SELECT count(*) FROM club_memberships WHERE club_id=$1 AND state='approved'", id).Scan(&count)
	return count, repositoryError(err)
}
func (s *Store) ListClubs(ctx context.Context, user string, mine bool, cursor string, limit int) ([]models.ClubListing, error) {
	rows, err := s.pool.Query(ctx, `SELECT c.id::text,c.owner_id::text,c.name,c.city,c.description,c.status,c.version,COALESCE(m.state,'') FROM clubs c LEFT JOIN club_memberships m ON m.club_id=c.id AND m.user_id::text=$1 WHERE (NOT EXISTS(SELECT 1 FROM moderation_hidden h WHERE h.target_type='club' AND h.target_id=c.id) OR c.owner_id::text=$1 OR m.state='approved') AND ($3='' OR c.id>NULLIF($3,'')::uuid) AND (($2 AND (c.owner_id::text=$1 OR m.state IN ('requested','approved'))) OR (NOT $2 AND c.status='active')) ORDER BY c.id LIMIT $4`, user, mine, cursor, limit)
	if err != nil {
		return nil, repositoryError(err)
	}
	defer rows.Close()
	out := []models.ClubListing{}
	for rows.Next() {
		var row models.ClubListing
		c := &row.Club
		if err := rows.Scan(&c.ID, &c.OwnerID, &c.Name, &c.City, &c.Description, &c.Status, &c.Version, &row.MembershipState); err != nil {
			return nil, repositoryError(err)
		}
		out = append(out, row)
	}
	return out, repositoryError(rows.Err())
}
func (s *Store) CreateClub(ctx context.Context, club models.Club, ownerState string, at time.Time, decide database.AccountDecision) (models.Club, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return models.Club{}, repositoryError(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", trustAdvisoryLock); err != nil {
		return models.Club{}, repositoryError(err)
	}
	var owner models.GarageOwner
	if err = tx.QueryRow(ctx, "SELECT id::text,status FROM users WHERE id=$1 FOR SHARE", club.OwnerID).Scan(&owner.ID, &owner.Status); err != nil {
		return models.Club{}, repositoryError(err)
	}
	if err = decide(owner); err != nil {
		return models.Club{}, err
	}
	stored, err := scanClub(tx.QueryRow(ctx, `INSERT INTO clubs(id,owner_id,name,city,description,status,version,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$8) RETURNING `+clubColumns, club.ID, club.OwnerID, club.Name, club.City, club.Description, club.Status, club.Version, at))
	if err != nil {
		return models.Club{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO club_memberships(club_id,user_id,state,created_at,updated_at) VALUES($1,$2,$3,$4,$4)`, club.ID, club.OwnerID, ownerState, at); err != nil {
		return models.Club{}, repositoryError(err)
	}
	stored.MemberCount, err = countClubMembers(ctx, tx, club.ID)
	if err != nil {
		return models.Club{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return models.Club{}, repositoryError(err)
	}
	return stored, nil
}
func (s *Store) WithClub(ctx context.Context, id string, decide database.ClubDecision) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return repositoryError(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", trustAdvisoryLock); err != nil {
		return repositoryError(err)
	}
	club, err := scanClub(tx.QueryRow(ctx, "SELECT "+clubColumns+" FROM clubs WHERE id=$1 FOR UPDATE", id))
	if err != nil {
		return err
	}
	// Count in a statement after the club lock is acquired, so a waiter cannot
	// reuse a pre-lock membership snapshot from the locking SELECT.
	club.MemberCount, err = countClubMembers(ctx, tx, id)
	if err != nil {
		return err
	}
	locked := &clubTransaction{ctx: ctx, tx: tx, club: club}
	if err = decide(locked); err != nil {
		return err
	}
	return repositoryError(tx.Commit(ctx))
}

type clubTransaction struct {
	ctx  context.Context
	tx   pgx.Tx
	club models.Club
}

func (t *clubTransaction) Club() models.Club { return t.club }
func (t *clubTransaction) Membership(user string) (models.ClubMembership, error) {
	var member models.ClubMembership
	// Club precedes account locks. SHARE protects account status during an
	// approval/ownership decision while allowing concurrent membership readers.
	err := t.tx.QueryRow(t.ctx, `SELECT COALESCE(m.state,''),u.status FROM users u LEFT JOIN club_memberships m ON m.user_id=u.id AND m.club_id=$1 WHERE u.id=$2 FOR SHARE OF u`, t.club.ID, user).Scan(&member.State, &member.AccountStatus)
	return member, repositoryError(err)
}
func (t *clubTransaction) SetMembership(user, state string, at time.Time) error {
	_, err := t.tx.Exec(t.ctx, `INSERT INTO club_memberships(club_id,user_id,state,created_at,updated_at) VALUES($1,$2,$3,$4,$4) ON CONFLICT(club_id,user_id) DO UPDATE SET state=$3,updated_at=$4`, t.club.ID, user, state, at)
	if err != nil {
		return repositoryError(err)
	}
	t.club.MemberCount, err = countClubMembers(t.ctx, t.tx, t.club.ID)
	return err
}
func (t *clubTransaction) UpdateClub(club models.Club, at time.Time) error {
	stored, err := scanClub(t.tx.QueryRow(t.ctx, `UPDATE clubs SET owner_id=$2,name=$3,city=$4,description=$5,status=$6,version=$7,updated_at=$8 WHERE id=$1 RETURNING `+clubColumns, t.club.ID, club.OwnerID, club.Name, club.City, club.Description, club.Status, club.Version, at))
	if err != nil {
		return err
	}
	stored.MemberCount = t.club.MemberCount
	t.club = stored
	return nil
}
func (t *clubTransaction) Members(includeRequested bool) ([]models.ClubMember, error) {
	rows, err := t.tx.Query(t.ctx, `SELECT u.id::text,u.display_name,m.state,CASE WHEN u.id=$2::uuid THEN 'owner' ELSE 'member' END FROM club_memberships m JOIN users u ON u.id=m.user_id WHERE m.club_id=$1 AND (m.state='approved' OR ($3 AND m.state='requested')) ORDER BY m.created_at,u.id`, t.club.ID, t.club.OwnerID, includeRequested)
	if err != nil {
		return nil, repositoryError(err)
	}
	defer rows.Close()
	out := []models.ClubMember{}
	for rows.Next() {
		var member models.ClubMember
		if err := rows.Scan(&member.UserID, &member.DisplayName, &member.State, &member.Role); err != nil {
			return nil, repositoryError(err)
		}
		out = append(out, member)
	}
	return out, repositoryError(rows.Err())
}
func (t *clubTransaction) LinkedRides(now time.Time, limit int) ([]models.RideSummary, error) {
	rows, err := t.tx.Query(t.ctx, `SELECT r.id::text,r.city,to_char(r.starts_at AT TIME ZONE r.timezone,'YYYY-MM-DD'),r.description,r.status FROM club_rides cr JOIN rides r ON r.id=cr.ride_id WHERE cr.club_id=$1 AND NOT EXISTS(SELECT 1 FROM moderation_hidden h WHERE h.target_type='ride' AND h.target_id=r.id) ORDER BY (r.status='published' AND r.starts_at >= $2) DESC,r.starts_at,r.id LIMIT $3`, t.club.ID, now, limit)
	if err != nil {
		return nil, repositoryError(err)
	}
	defer rows.Close()
	out := []models.RideSummary{}
	for rows.Next() {
		var ride models.RideSummary
		if err := rows.Scan(&ride.ID, &ride.City, &ride.Date, &ride.Description, &ride.Status); err != nil {
			return nil, repositoryError(err)
		}
		out = append(out, ride)
	}
	return out, repositoryError(rows.Err())
}
func (t *clubTransaction) RideForLink(id string) (models.ClubRideCandidate, error) {
	var ride models.ClubRideCandidate
	s := &ride.Summary
	err := t.tx.QueryRow(t.ctx, `SELECT id::text,city,to_char(starts_at AT TIME ZONE timezone,'YYYY-MM-DD'),description,status,organizer_id::text FROM rides WHERE id=$1 FOR SHARE`, id).Scan(&s.ID, &s.City, &s.Date, &s.Description, &s.Status, &ride.OrganizerID)
	return ride, repositoryError(err)
}
func (t *clubTransaction) LinkRide(ride, user string, at time.Time) error {
	_, err := t.tx.Exec(t.ctx, `INSERT INTO club_rides(club_id,ride_id,linked_by,linked_at) VALUES($1,$2,$3,$4) ON CONFLICT(club_id,ride_id) DO NOTHING`, t.club.ID, ride, user, at)
	return repositoryError(err)
}
func (t *clubTransaction) UnlinkRide(ride string) error {
	_, err := t.tx.Exec(t.ctx, "DELETE FROM club_rides WHERE club_id=$1 AND ride_id=$2", t.club.ID, ride)
	return repositoryError(err)
}

func (t *clubTransaction) Blocked(a, b string) (bool, error) { return trustBlocked(t.ctx, t.tx, a, b) }
func (t *clubTransaction) ContentHidden(kind, id string) (bool, error) {
	return trustContentHidden(t.ctx, t.tx, kind, id)
}
