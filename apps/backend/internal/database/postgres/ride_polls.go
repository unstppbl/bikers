package postgres

import (
	"context"

	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"github.com/jackc/pgx/v5"
)

func (s *Store) CreateRidePoll(ctx context.Context, poll models.RidePoll, decide database.RidePollCreateDecision) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return repositoryError(err)
	}
	defer tx.Rollback(ctx)
	owner, err := pollAccount(ctx, tx, poll.OrganizerID)
	if err != nil {
		return err
	}
	if err = decide(owner); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO ride_polls(id,organizer_id,city,description,created_at) VALUES($1,$2,$3,$4,$5)`, poll.ID, poll.OrganizerID, poll.City, poll.Description, poll.CreatedAt); err != nil {
		return repositoryError(err)
	}
	for _, option := range poll.Options {
		if _, err = tx.Exec(ctx, `INSERT INTO ride_poll_options(poll_id,id,starts_at) VALUES($1,$2,$3)`, poll.ID, option.ID, option.StartsAt); err != nil {
			return repositoryError(err)
		}
	}
	return repositoryError(tx.Commit(ctx))
}

func (s *Store) RidePoll(ctx context.Context, id string, voterHash []byte) (models.RidePoll, error) {
	return readRidePoll(ctx, s.pool, id, voterHash)
}

func (s *Store) WithRidePoll(ctx context.Context, id string, voterHash []byte, decide database.RidePollDecision) (models.RidePoll, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return models.RidePoll{}, repositoryError(err)
	}
	defer tx.Rollback(ctx)
	var locked string
	if err = tx.QueryRow(ctx, `SELECT id::text FROM ride_polls WHERE id=$1 FOR UPDATE`, id).Scan(&locked); err != nil {
		return models.RidePoll{}, repositoryError(err)
	}
	poll, err := readRidePoll(ctx, tx, id, voterHash)
	if err != nil {
		return models.RidePoll{}, err
	}
	mutation := &ridePollTransaction{ctx: ctx, tx: tx, poll: poll, voterHash: voterHash}
	if err = decide(mutation); err != nil {
		return models.RidePoll{}, err
	}
	poll, err = readRidePoll(ctx, tx, id, voterHash)
	if err != nil {
		return models.RidePoll{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return models.RidePoll{}, repositoryError(err)
	}
	return poll, nil
}

type pollReader interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func readRidePoll(ctx context.Context, query pollReader, id string, voterHash []byte) (models.RidePoll, error) {
	rows, err := query.Query(ctx, `
SELECT p.organizer_id::text,p.city,p.description,coalesce(p.selected_option_id::text,''),
    o.id::text,o.starts_at,count(v.option_id),coalesce(bool_or(v.voter_hash=$2::bytea),false)
FROM ride_polls p JOIN ride_poll_options o ON o.poll_id=p.id
LEFT JOIN ride_poll_votes v ON v.poll_id=o.poll_id AND v.option_id=o.id
WHERE p.id=$1
GROUP BY p.id,o.poll_id,o.id
ORDER BY o.starts_at,o.id`, id, voterHash)
	if err != nil {
		return models.RidePoll{}, repositoryError(err)
	}
	defer rows.Close()
	poll := models.RidePoll{ID: id, MyOptionIDs: []string{}}
	for rows.Next() {
		var option models.RidePollOption
		var mine bool
		if err = rows.Scan(&poll.OrganizerID, &poll.City, &poll.Description, &poll.SelectedOptionID, &option.ID, &option.StartsAt, &option.Votes, &mine); err != nil {
			return models.RidePoll{}, repositoryError(err)
		}
		option.StartsAt = option.StartsAt.UTC()
		poll.Options = append(poll.Options, option)
		if mine {
			poll.MyOptionIDs = append(poll.MyOptionIDs, option.ID)
		}
	}
	if err = rows.Err(); err != nil {
		return models.RidePoll{}, repositoryError(err)
	}
	if len(poll.Options) == 0 {
		return models.RidePoll{}, database.ErrNotFound
	}
	return poll, nil
}

func pollAccount(ctx context.Context, tx pgx.Tx, id string) (database.RidePollAccount, error) {
	var owner database.RidePollAccount
	err := tx.QueryRow(ctx, `SELECT status,onboarding_complete FROM users WHERE id=$1 FOR SHARE`, id).Scan(&owner.Status, &owner.OnboardingComplete)
	return owner, repositoryError(err)
}

type ridePollTransaction struct {
	ctx       context.Context
	tx        pgx.Tx
	poll      models.RidePoll
	voterHash []byte
}

func (t *ridePollTransaction) Poll() models.RidePoll { return t.poll }

func (t *ridePollTransaction) Account(id string) (database.RidePollAccount, error) {
	return pollAccount(t.ctx, t.tx, id)
}

func (t *ridePollTransaction) ReplaceVote(optionIDs []string) error {
	if _, err := t.tx.Exec(t.ctx, `DELETE FROM ride_poll_votes WHERE poll_id=$1 AND voter_hash=$2`, t.poll.ID, t.voterHash); err != nil {
		return repositoryError(err)
	}
	_, err := t.tx.Exec(t.ctx, `INSERT INTO ride_poll_votes(poll_id,voter_hash,option_id)
SELECT $1,$2,option_id::uuid FROM unnest($3::text[]) choices(option_id)`, t.poll.ID, t.voterHash, optionIDs)
	return repositoryError(err)
}

func (t *ridePollTransaction) SelectOption(optionID string) error {
	_, err := t.tx.Exec(t.ctx, `UPDATE ride_polls SET selected_option_id=$2 WHERE id=$1`, t.poll.ID, optionID)
	return repositoryError(err)
}
