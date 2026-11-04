package postgres

import (
	"context"
	"time"

	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"github.com/jackc/pgx/v5"
)

const challengeColumns = "id::text,phone,source_key,provider_id,state,attempts,created_at,expires_at,resend_after,consumed_at"
const userColumns = "u.id::text,u.display_name,u.city,u.languages,u.interests,u.locale,u.onboarding_complete"

func scanChallenge(row pgx.Row) (models.OTPChallenge, error) {
	var c models.OTPChallenge
	err := row.Scan(&c.ID, &c.Phone, &c.SourceKey, &c.ProviderID, &c.State, &c.Attempts, &c.CreatedAt, &c.ExpiresAt, &c.ResendAfter, &c.ConsumedAt)
	return c, repositoryError(err)
}
func emptyLanguages(languages []string) []string {
	if languages == nil {
		return []string{}
	}
	return languages
}

func (s *Store) ReserveChallenge(ctx context.Context, c models.OTPChallenge, windowStart, dayStart time.Time, decide database.SendDecision) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return repositoryError(err)
	}
	defer tx.Rollback(ctx)
	// All send reservations share the original advisory key, including during a
	// rolling upgrade. Provider calls happen only after this operation commits.
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(773512002)"); err != nil {
		return repositoryError(err)
	}
	var window models.SendWindow
	err = tx.QueryRow(ctx, `SELECT count(*) FILTER (WHERE phone=$1 AND created_at>$3),count(*) FILTER (WHERE source_key=$2 AND created_at>$3),count(*) FILTER (WHERE created_at>=$5),COALESCE(bool_or(phone=$1 AND resend_after>$4),false) FROM otp_challenges WHERE created_at>=LEAST($3,$5)`, c.Phone, c.SourceKey, windowStart, c.CreatedAt, dayStart).Scan(&window.PhoneSends, &window.SourceSends, &window.Daily, &window.Cooldown)
	if err != nil {
		return repositoryError(err)
	}
	if err = decide(window); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO otp_challenges(id,phone,source_key,provider_id,state,attempts,created_at,expires_at,resend_after,consumed_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, c.ID, c.Phone, c.SourceKey, c.ProviderID, c.State, c.Attempts, c.CreatedAt, c.ExpiresAt, c.ResendAfter, c.ConsumedAt)
	if err != nil {
		return repositoryError(err)
	}
	return repositoryError(tx.Commit(ctx))
}

func (s *Store) UpdateChallenge(ctx context.Context, id, expectedState, state, providerID string) error {
	result, err := s.pool.Exec(ctx, `UPDATE otp_challenges SET state=$3,provider_id=CASE WHEN $4='' THEN provider_id ELSE $4 END WHERE id=$1 AND state=$2`, id, expectedState, state, providerID)
	if err != nil {
		return repositoryError(err)
	}
	if result.RowsAffected() == 0 {
		return database.ErrConflict
	}
	return nil
}

func (s *Store) AttemptVerification(ctx context.Context, id string, decide database.AttemptDecision) (models.OTPChallenge, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return models.OTPChallenge{}, repositoryError(err)
	}
	defer tx.Rollback(ctx)
	c, err := scanChallenge(tx.QueryRow(ctx, "SELECT "+challengeColumns+" FROM otp_challenges WHERE id=$1 FOR UPDATE", id))
	if err != nil {
		return models.OTPChallenge{}, err
	}
	if err = decide(c); err != nil {
		return models.OTPChallenge{}, err
	}
	if _, err = tx.Exec(ctx, "UPDATE otp_challenges SET attempts=attempts+1 WHERE id=$1", id); err != nil {
		return models.OTPChallenge{}, repositoryError(err)
	}
	c.Attempts++
	if err = tx.Commit(ctx); err != nil {
		return models.OTPChallenge{}, repositoryError(err)
	}
	return c, nil
}

func (s *Store) CommitLogin(ctx context.Context, challengeID string, proposed models.Account, session models.SessionRecord, oldDigest []byte, decide database.LoginDecision) (models.User, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return models.User{}, repositoryError(err)
	}
	defer tx.Rollback(ctx)
	c, err := scanChallenge(tx.QueryRow(ctx, "SELECT "+challengeColumns+" FROM otp_challenges WHERE id=$1 FOR UPDATE", challengeID))
	if err != nil {
		return models.User{}, err
	}
	var actual models.Account
	// The unique-phone upsert obtains and retains the account row lock. Existing
	// status/profile data is returned unchanged for the manager to revalidate.
	err = tx.QueryRow(ctx, `INSERT INTO users AS u(id,phone,status,display_name,city,languages,interests,locale,onboarding_complete,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT(phone) DO UPDATE SET phone=u.phone RETURNING `+userColumns+`,u.phone,u.status,u.created_at`, proposed.User.ID, proposed.Phone, proposed.Status, proposed.User.Profile.DisplayName, proposed.User.Profile.City, emptyLanguages(proposed.User.Profile.Languages), proposed.User.Profile.Interests, proposed.User.Profile.Locale, proposed.User.OnboardingComplete, proposed.CreatedAt).Scan(&actual.User.ID, &actual.User.Profile.DisplayName, &actual.User.Profile.City, &actual.User.Profile.Languages, &actual.User.Profile.Interests, &actual.User.Profile.Locale, &actual.User.OnboardingComplete, &actual.Phone, &actual.Status, &actual.CreatedAt)
	if err != nil {
		return models.User{}, repositoryError(err)
	}
	actual.User.Profile.Languages = emptyLanguages(actual.User.Profile.Languages)
	if err = decide(c, actual); err != nil {
		return models.User{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE otp_challenges SET state='consumed',consumed_at=$2 WHERE id=$1`, challengeID, session.CreatedAt); err != nil {
		return models.User{}, repositoryError(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO sessions(token_digest,user_id,csrf_token,created_at,last_seen_at,expires_at,revoked_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, session.Digest, actual.User.ID, session.CSRF, session.CreatedAt, session.LastSeenAt, session.ExpiresAt, session.RevokedAt); err != nil {
		return models.User{}, repositoryError(err)
	}
	if len(oldDigest) > 0 {
		if _, err = tx.Exec(ctx, "UPDATE sessions SET revoked_at=$2 WHERE token_digest=$1", oldDigest, session.CreatedAt); err != nil {
			return models.User{}, repositoryError(err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return models.User{}, repositoryError(err)
	}
	return actual.User, nil
}

func (s *Store) Session(ctx context.Context, digest []byte) (models.SessionRecord, error) {
	var record models.SessionRecord
	err := s.pool.QueryRow(ctx, `SELECT `+userColumns+`,s.csrf_token,s.created_at,s.last_seen_at,s.expires_at,s.revoked_at,u.status FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_digest=$1`, digest).Scan(&record.User.ID, &record.User.Profile.DisplayName, &record.User.Profile.City, &record.User.Profile.Languages, &record.User.Profile.Interests, &record.User.Profile.Locale, &record.User.OnboardingComplete, &record.CSRF, &record.CreatedAt, &record.LastSeenAt, &record.ExpiresAt, &record.RevokedAt, &record.UserStatus)
	if err != nil {
		return models.SessionRecord{}, repositoryError(err)
	}
	record.Digest = append([]byte(nil), digest...)
	record.User.Profile.Languages = emptyLanguages(record.User.Profile.Languages)
	return record, nil
}
func (s *Store) TouchSession(ctx context.Context, digest []byte, at time.Time) error {
	_, err := s.pool.Exec(ctx, "UPDATE sessions SET last_seen_at=$2 WHERE token_digest=$1 AND revoked_at IS NULL", digest, at)
	return repositoryError(err)
}
func (s *Store) RevokeSession(ctx context.Context, digest []byte, at time.Time) error {
	_, err := s.pool.Exec(ctx, "UPDATE sessions SET revoked_at=$2 WHERE token_digest=$1", digest, at)
	return repositoryError(err)
}
func (s *Store) UpdateProfile(ctx context.Context, id string, decide database.ProfileDecision) (models.User, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return models.User{}, repositoryError(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", trustAdvisoryLock); err != nil {
		return models.User{}, repositoryError(err)
	}
	var account models.Account
	err = tx.QueryRow(ctx, "SELECT "+userColumns+",u.status FROM users u WHERE u.id=$1 FOR UPDATE", id).Scan(&account.User.ID, &account.User.Profile.DisplayName, &account.User.Profile.City, &account.User.Profile.Languages, &account.User.Profile.Interests, &account.User.Profile.Locale, &account.User.OnboardingComplete, &account.Status)
	if err != nil {
		return models.User{}, repositoryError(err)
	}
	user, err := decide(account)
	if err != nil {
		return models.User{}, err
	}
	_, err = tx.Exec(ctx, `UPDATE users SET display_name=$2,city=$3,languages=$4,interests=$5,locale=$6,onboarding_complete=$7 WHERE id=$1`, id, user.Profile.DisplayName, user.Profile.City, emptyLanguages(user.Profile.Languages), user.Profile.Interests, user.Profile.Locale, user.OnboardingComplete)
	if err != nil {
		return models.User{}, repositoryError(err)
	}
	return user, repositoryError(tx.Commit(ctx))
}
