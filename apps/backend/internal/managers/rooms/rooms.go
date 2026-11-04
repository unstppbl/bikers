// Package rooms owns ride-room access, durable messaging, and foreground events.
package rooms

import (
	"context"
	"errors"
	"strings"
	"time"

	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
)

const messagePageLimit = 100
const messageByteLimit = 8000

type Events interface{ Publish(string, string) }

type Manager struct {
	repo   database.RoomsRepository
	events Events
	now    func() time.Time
}

func New(repo database.RoomsRepository, events interface{ Publish(string, string) }, now func() time.Time) *Manager {
	if now == nil {
		now = time.Now
	}
	return &Manager{repo: repo, events: events, now: now}
}

func (m *Manager) Access(ctx context.Context, ride, userID string) (bool, error) {
	access, err := m.repo.RoomAccess(ctx, ride, userID)
	if errors.Is(err, database.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return roomAllowed(access), nil
}

func (m *Manager) Messages(ctx context.Context, ride, userID string, after int64) (models.MessagePage, error) {
	allowed, err := m.Access(ctx, ride, userID)
	if err != nil {
		return models.MessagePage{}, err
	}
	if !allowed {
		return models.MessagePage{}, participantRequired()
	}
	if after < 0 {
		return models.MessagePage{}, models.Fail("invalid_cursor", "Invalid message cursor")
	}
	page, err := m.repo.Messages(ctx, ride, userID, after, messagePageLimit)
	if err != nil {
		return models.MessagePage{}, err
	}
	if page.Messages == nil {
		page.Messages = []models.Message{}
	}
	return page, nil
}

func (m *Manager) Send(ctx context.Context, ride string, actor models.User, input models.MessageInput) (models.MessageResult, error) {
	_, err := uuid.Parse(input.ClientMessageID)
	if err != nil {
		return models.MessageResult{}, models.Fail("invalid_message", "A UUID client message ID is required")
	}
	input.Body = strings.TrimSpace(input.Body)
	if input.Body == "" || len(input.Body) > messageByteLimit {
		return models.MessageResult{}, models.Fail("invalid_message", "Message must contain 1–8000 bytes")
	}
	var result models.MessageResult
	err = m.repo.WithRide(ctx, ride, func(tx database.RideTransaction) error {
		access, err := tx.RoomAccess(actor.ID)
		if err != nil {
			return err
		}
		if !roomAllowed(access) {
			return participantRequired()
		}
		message, err := tx.Message(actor.ID, input.ClientMessageID)
		if err == nil {
			hidden, e := tx.ContentHidden("message", message.ID)
			if e != nil {
				return e
			}
			if hidden {
				return models.Fail("not_found", "Message not found")
			}
			if message.Body != input.Body {
				return models.Fail("message_id_conflict", "Message ID already used for different text")
			}
			message.SenderName = actor.Profile.DisplayName
			result = models.MessageResult{Message: message}
			return nil
		}
		if !errors.Is(err, database.ErrNotFound) {
			return err
		}
		if tx.Ride().Status == "cancelled" {
			return models.Fail("ride_cancelled", "Ride is cancelled; its room is read-only")
		}
		message = models.Message{ID: uuid.NewString(), SenderID: actor.ID, SenderName: actor.Profile.DisplayName, Body: input.Body, CreatedAt: m.now().UTC(), ClientMessageID: input.ClientMessageID}
		message, err = tx.AppendMessage(message)
		if err != nil {
			return err
		}
		result = models.MessageResult{Message: message, Created: true}
		return nil
	})
	if errors.Is(err, database.ErrNotFound) {
		return models.MessageResult{}, models.Fail("not_found", "Ride not found")
	}
	if err != nil {
		return models.MessageResult{}, err
	}
	if result.Created && m.events != nil {
		m.events.Publish(ride, "message.created")
	}
	return result, nil
}

func roomAllowed(access models.RoomAccess) bool {
	return access.ParticipationState == "approved" && access.UserStatus == "active"
}

func participantRequired() error {
	return models.Fail("participant_required", "Approved participation required")
}
