// Package system owns migration execution and application readiness decisions.
package system

import (
	"context"
	"os"
	"time"

	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
)

const migrationTimeout = time.Minute
const readinessTimeout = 2 * time.Second

type Manager struct {
	repo     database.SystemRepository
	mediaDir string
}

func New(repo database.SystemRepository, mediaDir string) *Manager {
	return &Manager{repo: repo, mediaDir: mediaDir}
}

func (m *Manager) Migrate(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, migrationTimeout)
	defer cancel()
	return m.repo.Migrate(ctx)
}

func (m *Manager) Ready(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, readinessTimeout)
	defer cancel()
	if err := m.repo.Ready(ctx); err != nil {
		return models.Fail("not_ready", "Database migration required or database unavailable")
	}
	if err := ctx.Err(); err != nil {
		return models.Fail("not_ready", "Database migration required or database unavailable")
	}
	file, err := os.CreateTemp(m.mediaDir, ".health-*")
	if err != nil {
		return models.Fail("not_ready", "Persistent media storage unavailable")
	}
	name := file.Name()
	err = file.Close()
	_ = os.Remove(name)
	if err != nil {
		return models.Fail("service_unavailable", "Service temporarily unavailable")
	}
	return nil
}
