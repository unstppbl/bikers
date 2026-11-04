// Package photos owns shared photo processing limits and concurrent CPU work.
package photos

import (
	"bikers.kz/backend/internal/models"
	"context"
	"io"
	"time"
)

const ReservedBytes int64 = 4 << 20
const OwnerByteLimit int64 = 100 << 20
const GlobalByteLimit int64 = 1 << 30
const HourlyAttempts = 12
const ReservationTTL = 10 * time.Minute
const AttemptRetention = 24 * time.Hour
const CleanupTimeout = 3 * time.Second

type Process func(context.Context, io.Reader) (models.ProcessedPhoto, error)

type Manager struct {
	slots   chan struct{}
	process Process
}

func New(process Process) *Manager {
	if process == nil {
		process = processPhoto
	}
	return &Manager{slots: make(chan struct{}, 2), process: process}
}

// WithSlot acquires a shared slot before the caller reserves or reads a body.
// It retains the slot until the callback and synchronous decoder have stopped.
func (m *Manager) WithSlot(ctx context.Context, run func(Process) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case m.slots <- struct{}{}:
		defer func() { <-m.slots }()
	default:
		return models.Fail("photo_upload_busy", "Photo processing is busy; try again shortly")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return run(m.process)
}
