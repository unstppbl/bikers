package postgres

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testStore(t *testing.T) (*Store, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("PostgreSQL repository integration requires TEST_DATABASE_URL")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := "postgres_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(ctx, "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
		admin.Close()
	})
	store := New(pool)
	if err = store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	return store, pool
}
func challengeFixture(now time.Time) models.OTPChallenge {
	return models.OTPChallenge{ID: uuid.NewString(), Phone: "+77011234567", SourceKey: "test-source", ProviderID: "test-provider", State: "pending", CreatedAt: now, ExpiresAt: now.Add(5 * time.Minute), ResendAfter: now.Add(time.Minute)}
}
func accountFixture(phone string, now time.Time) models.Account {
	return models.Account{User: models.User{ID: uuid.NewString(), Profile: models.Profile{Locale: "en"}}, Phone: phone, Status: "active", CreatedAt: now}
}
func sessionFixture(now time.Time) models.SessionRecord {
	digest := sha256.Sum256([]byte(uuid.NewString()))
	return models.SessionRecord{Session: models.Session{Digest: digest[:], CSRF: "test-csrf"}, CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(30 * 24 * time.Hour)}
}
func reserveFixture(t *testing.T, store *Store, c models.OTPChallenge) {
	t.Helper()
	if err := store.ReserveChallenge(context.Background(), c, c.CreatedAt.Add(-15*time.Minute), c.CreatedAt.Truncate(24*time.Hour), func(models.SendWindow) error { return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestMigrationMatchesExistingAppliedReceipt(t *testing.T) {
	const originalChecksum = "efae824b77f9fb710faacbfab6963f643b59020d06355882ba7287aeeb70bcbd"
	if got := schemaChecksum(); got != originalChecksum {
		t.Fatalf("existing migration changed: %s", got)
	}
	store, pool := testStore(t)
	ctx := context.Background()
	var checksum string
	if err := pool.QueryRow(ctx, "SELECT checksum FROM schema_migrations WHERE version=1").Scan(&checksum); err != nil {
		t.Fatal(err)
	}
	if checksum != originalChecksum {
		t.Fatal("applied receipt differs")
	}
	if err := store.Migrate(ctx); err != nil {
		t.Fatal("existing compatible receipt not accepted:", err)
	}
	if err := store.Ready(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestAccountCallbacksRejectWithoutPartialWrites(t *testing.T) {
	store, pool := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	c := challengeFixture(now)
	reject := errors.New("manager rejected")
	err := store.ReserveChallenge(ctx, c, now.Add(-15*time.Minute), now.Truncate(24*time.Hour), func(models.SendWindow) error { return reject })
	if !errors.Is(err, reject) {
		t.Fatal(err)
	}
	var count int
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM otp_challenges").Scan(&count)
	if count != 0 {
		t.Fatal("rejected send reservation persisted")
	}
	reserveFixture(t, store, c)
	_, err = store.AttemptVerification(ctx, c.ID, func(current models.OTPChallenge) error {
		if current.Attempts != 0 {
			t.Error("callback did not see pre-increment snapshot")
		}
		return reject
	})
	if !errors.Is(err, reject) {
		t.Fatal(err)
	}
	var attempts int
	_ = pool.QueryRow(ctx, "SELECT attempts FROM otp_challenges WHERE id=$1", c.ID).Scan(&attempts)
	if attempts != 0 {
		t.Fatal("rejected attempt was incremented")
	}
	current, err := store.AttemptVerification(ctx, c.ID, func(models.OTPChallenge) error { return nil })
	if err != nil || current.Attempts != 1 {
		t.Fatalf("attempt reservation=%v err=%v", current, err)
	}
	account := accountFixture(c.Phone, now)
	session := sessionFixture(now)
	_, err = store.CommitLogin(ctx, c.ID, account, session, nil, func(current models.OTPChallenge, actual models.Account) error {
		if current.Attempts != 1 || actual.User.ID != account.User.ID || actual.Status != "active" {
			t.Error("manager received wrong locked state")
		}
		return reject
	})
	if !errors.Is(err, reject) {
		t.Fatal(err)
	}
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&count)
	if count != 0 {
		t.Fatal("rejected login left new account")
	}
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM sessions").Scan(&count)
	if count != 0 {
		t.Fatal("rejected login left session")
	}
	var state string
	_ = pool.QueryRow(ctx, "SELECT state FROM otp_challenges WHERE id=$1", c.ID).Scan(&state)
	if state != "pending" {
		t.Fatal("rejected login consumed challenge")
	}
	user, err := store.CommitLogin(ctx, c.ID, account, session, nil, func(current models.OTPChallenge, actual models.Account) error {
		if current.State != "pending" || actual.Phone != c.Phone {
			return reject
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if user.Profile.Languages == nil {
		t.Fatal("empty languages serialized as null")
	}
	record, err := store.Session(ctx, session.Digest)
	if err != nil || record.User.ID != user.ID || record.UserStatus != "active" {
		t.Fatalf("session=%v err=%v", record, err)
	}
	_, err = store.CommitLogin(ctx, c.ID, account, sessionFixture(now), nil, func(current models.OTPChallenge, _ models.Account) error {
		if current.State != "pending" {
			return reject
		}
		return nil
	})
	if !errors.Is(err, reject) {
		t.Fatal("manager could not reject consumed snapshot:", err)
	}
}

func TestLoginCallbackSeesExistingLockedAccountStatus(t *testing.T) {
	store, pool := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	c := challengeFixture(now)
	reserveFixture(t, store, c)
	existing := accountFixture(c.Phone, now)
	_, err := pool.Exec(ctx, "INSERT INTO users(id,phone,status,display_name,created_at) VALUES($1,$2,'suspended','Existing profile',$3)", existing.User.ID, existing.Phone, now)
	if err != nil {
		t.Fatal(err)
	}
	reject := errors.New("inactive account")
	_, err = store.CommitLogin(ctx, c.ID, accountFixture(c.Phone, now), sessionFixture(now), nil, func(_ models.OTPChallenge, actual models.Account) error {
		if actual.User.ID != existing.User.ID || actual.Status != "suspended" || actual.User.Profile.DisplayName != "Existing profile" {
			t.Error("proposed account overwrote existing state")
		}
		return reject
	})
	if !errors.Is(err, reject) {
		t.Fatal(err)
	}
	var status string
	if err = pool.QueryRow(ctx, "SELECT status FROM users WHERE id=$1", existing.User.ID).Scan(&status); err != nil || status != "suspended" {
		t.Fatal("account state changed despite rollback")
	}
}

func TestRideCallbackRollbackAndCommittedMessageSequence(t *testing.T) {
	store, pool := testStore(t)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	now := time.Now().UTC()
	owner := uuid.NewString()
	other := uuid.NewString()
	for i, id := range []string{owner, other} {
		if _, err := pool.Exec(ctx, "INSERT INTO users(id,phone,created_at) VALUES($1,$2,$3)", id, "fixture-"+id, now.Add(time.Duration(i)*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	ride := models.Ride{RideSummary: models.RideSummary{ID: uuid.NewString(), City: "Almaty", Description: "Public description", Status: "published"}, Title: "Private title", StartsAt: now.Add(time.Hour), Timezone: "Asia/Almaty", MeetingPoint: "Private meeting", Capacity: 2, PlanVersion: 1, OrganizerID: owner}
	if _, err := store.CreateRide(ctx, ride, "approved", now, func(models.GarageOwner) error { return nil }); err != nil {
		t.Fatal(err)
	}
	reject := errors.New("manager rejected")
	err := store.WithRide(ctx, ride.ID, func(tx database.RideTransaction) error {
		if err := tx.SetParticipation(other, "approved", now); err != nil {
			return err
		}
		updated := tx.Ride()
		updated.PlanVersion++
		updated.City = "Changed"
		if err := tx.UpdateRide(updated); err != nil {
			return err
		}
		return reject
	})
	if !errors.Is(err, reject) {
		t.Fatal(err)
	}
	state, err := store.Participation(ctx, ride.ID, other)
	if err != nil || state != "" {
		t.Fatal("rejected participation persisted")
	}
	unchanged, err := store.Ride(ctx, ride.ID)
	if err != nil || unchanged.PlanVersion != 1 || unchanged.City != "Almaty" {
		t.Fatal("rejected ride update persisted")
	}
	first := models.Message{ID: uuid.NewString(), SenderID: owner, SenderName: "Owner", ClientMessageID: uuid.NewString(), Body: "first", CreatedAt: now}
	second := models.Message{ID: uuid.NewString(), SenderID: owner, SenderName: "Owner", ClientMessageID: uuid.NewString(), Body: "second", CreatedAt: now}
	assigned := make(chan struct{})
	allowCommit := make(chan struct{})
	var release sync.Once
	allowFirstCommit := func() { release.Do(func() { close(allowCommit) }) }
	t.Cleanup(allowFirstCommit)
	firstDone := make(chan error, 1)
	secondStarted := make(chan struct{})
	secondEntered := make(chan struct{})
	secondDone := make(chan error, 1)
	go func() {
		firstDone <- store.WithRide(ctx, ride.ID, func(tx database.RideTransaction) error {
			if _, err := tx.AppendMessage(first); err != nil {
				return err
			}
			close(assigned)
			<-allowCommit
			return nil
		})
	}()
	select {
	case <-assigned:
	case err := <-firstDone:
		t.Fatal("first transaction failed before assignment:", err)
	case <-ctx.Done():
		t.Fatal("first transaction did not assign a message")
	}
	go func() {
		close(secondStarted)
		secondDone <- store.WithRide(ctx, ride.ID, func(tx database.RideTransaction) error {
			close(secondEntered)
			visible, err := tx.Message(owner, first.ClientMessageID)
			if err != nil {
				return err
			}
			if visible.Sequence != 1 {
				return errors.New("earlier message not committed before next room transaction")
			}
			_, err = tx.AppendMessage(second)
			return err
		})
	}()
	<-secondStarted
	page, err := store.Messages(ctx, ride.ID, owner, 0, 10)
	if err != nil || len(page.Messages) != 0 {
		t.Fatal("uncommitted message became visible")
	}
	select {
	case <-secondEntered:
		t.Fatal("room lock released before commit")
	case <-time.After(30 * time.Millisecond):
	}
	allowFirstCommit()
	if err = <-firstDone; err != nil {
		t.Fatal(err)
	}
	if err = <-secondDone; err != nil {
		t.Fatal(err)
	}
	page, err = store.Messages(ctx, ride.ID, owner, 0, 10)
	if err != nil || len(page.Messages) != 2 || page.Messages[0].Sequence != 1 || page.Messages[1].Sequence != 2 || page.NextSequence != 2 {
		t.Fatalf("cursor page=%v err=%v", page, err)
	}
	missing, err := store.RoomAccess(ctx, ride.ID, other)
	if err != nil || missing != (models.RoomAccess{}) {
		t.Fatal("missing access should be empty state")
	}
	if _, err = store.Ride(ctx, uuid.NewString()); !errors.Is(err, database.ErrNotFound) {
		t.Fatal("driver not-found escaped repository")
	}
}
