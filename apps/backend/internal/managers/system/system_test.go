package system

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"bikers.kz/backend/internal/models"
)

type systemRepository struct {
	migrationErr, readyErr           error
	migrationDeadline, readyDeadline time.Time
}

func (r *systemRepository) Migrate(ctx context.Context) error {
	r.migrationDeadline, _ = ctx.Deadline()
	return r.migrationErr
}
func (r *systemRepository) Ready(ctx context.Context) error {
	r.readyDeadline, _ = ctx.Deadline()
	return r.readyErr
}
func requireSystemFault(t *testing.T, err error, code, message string) {
	t.Helper()
	var fault *models.Fault
	if !errors.As(err, &fault) || fault.Code != code || fault.Message != message {
		t.Fatalf("got %v; want %s %s", err, code, message)
	}
}

func TestMigrationsAreExplicitAndBounded(t *testing.T) {
	repo := &systemRepository{}
	manager := New(repo, t.TempDir())
	started := time.Now()
	if err := manager.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if repo.migrationDeadline.Before(started.Add(59*time.Second)) || repo.migrationDeadline.After(time.Now().Add(time.Minute)) {
		t.Fatal("migration timeout not owned by manager")
	}
	want := errors.New("checksum mismatch")
	repo.migrationErr = want
	if err := manager.Migrate(context.Background()); !errors.Is(err, want) {
		t.Fatal("migration cause was discarded")
	}
}

func TestReadinessChecksSchemaAndPersistentStorage(t *testing.T) {
	directory := t.TempDir()
	repo := &systemRepository{}
	manager := New(repo, directory)
	started := time.Now()
	if err := manager.Ready(context.Background()); err != nil {
		t.Fatal(err)
	}
	if repo.readyDeadline.Before(started.Add(time.Second)) || repo.readyDeadline.After(time.Now().Add(2*time.Second)) {
		t.Fatal("readiness timeout not owned by manager")
	}
	files, err := os.ReadDir(directory)
	if err != nil || len(files) != 0 {
		t.Fatal("readiness probe left temporary files")
	}
	repo.readyErr = errors.New("schema mismatch")
	requireSystemFault(t, manager.Ready(context.Background()), "not_ready", "Database migration required or database unavailable")
	repo.readyErr = nil
	manager = New(repo, filepath.Join(directory, "missing"))
	requireSystemFault(t, manager.Ready(context.Background()), "not_ready", "Persistent media storage unavailable")
}

func TestReadinessPreservesShorterCallerDeadline(t *testing.T) {
	repo := &systemRepository{}
	manager := New(repo, t.TempDir())
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	want, _ := ctx.Deadline()
	if err := manager.Ready(ctx); err != nil {
		t.Fatal(err)
	}
	if !repo.readyDeadline.Equal(want) {
		t.Fatal("readiness extended caller deadline")
	}
}
