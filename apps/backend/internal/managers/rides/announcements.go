package rides

import (
	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"context"
	"errors"
	"github.com/google/uuid"
	"strconv"
	"strings"
)

func (m *Manager) Announcements(ctx context.Context, actor models.User, id, cursor string) (models.AnnouncementPage, error) {
	out := models.AnnouncementPage{Announcements: []models.Announcement{}}
	id, err := canonicalID(id)
	if err != nil {
		return out, err
	}
	var before int64
	if cursor != "" {
		before, err = strconv.ParseInt(cursor, 10, 64)
		if err != nil || before < 1 {
			return out, models.Fail("invalid_cursor", "Invalid announcement cursor")
		}
	}
	err = m.repo.WithRide(ctx, id, func(tx database.RideTransaction) error {
		access, err := tx.RoomAccess(actor.ID)
		if err != nil {
			return err
		}
		if access.UserStatus != "active" || access.ParticipationState != "approved" {
			return models.Fail("participant_required", "Approved participation required")
		}
		rows, err := tx.Announcements(before, 51)
		if err != nil {
			return err
		}
		if len(rows) > 50 {
			rows = rows[:50]
			out.NextCursor = strconv.FormatInt(rows[49].Sequence, 10)
		}
		out.Announcements = rows
		return nil
	})
	return out, rideError(err)
}
func (m *Manager) Announce(ctx context.Context, actor models.User, id string, input models.AnnouncementInput) (models.AnnouncementResult, error) {
	var out models.AnnouncementResult
	id, err := canonicalID(id)
	if err != nil {
		return out, err
	}
	client, err := uuid.Parse(input.ClientAnnouncementID)
	input.Body = strings.TrimSpace(input.Body)
	if err != nil || input.Body == "" || len(input.Body) > 4000 {
		return out, models.Fail("invalid_announcement", "Announcement needs a UUID and 1–4000 bytes of text")
	}
	input.ClientAnnouncementID = client.String()
	err = m.repo.WithRide(ctx, id, func(tx database.RideTransaction) error {
		if tx.Ride().OrganizerID != actor.ID {
			return models.Fail("forbidden", "Organizer access required")
		}
		access, err := tx.RoomAccess(actor.ID)
		if err != nil {
			return err
		}
		if access.UserStatus != "active" || access.ParticipationState != "approved" {
			return models.Fail("account_unavailable", "Account unavailable")
		}
		current, err := tx.Announcement(input.ClientAnnouncementID)
		if err == nil {
			hidden, e := tx.ContentHidden("announcement", current.ID)
			if e != nil {
				return e
			}
			if hidden {
				return models.Fail("not_found", "Announcement not found")
			}
			if current.Body != input.Body {
				return models.Fail("announcement_id_conflict", "Announcement ID already used for different text")
			}
			out = models.AnnouncementResult{Announcement: current}
			return nil
		}
		if !errors.Is(err, database.ErrNotFound) {
			return err
		}
		if tx.Ride().Status == "cancelled" {
			return models.Fail("ride_cancelled", "Ride is cancelled; announcements are read-only")
		}
		a, err := tx.AppendAnnouncement(models.Announcement{ID: uuid.NewString(), OwnerID: actor.ID, OwnerName: actor.Profile.DisplayName, Body: input.Body, CreatedAt: m.now().UTC(), ClientAnnouncementID: input.ClientAnnouncementID})
		if err != nil {
			return err
		}
		out = models.AnnouncementResult{Announcement: a, Created: true}
		return m.notifyApproved(tx, "ride.announcement", actor.ID, a.ID)
	})
	if err != nil {
		return out, rideError(err)
	}
	if out.Created {
		m.publish(id, "announcement.created")
	}
	return out, nil
}
func (m *Manager) notifyApproved(tx database.RideTransaction, kind, except, eventID string) error {
	people, err := tx.ParticipationUsers()
	if err != nil {
		return err
	}
	ids := []string{}
	for _, p := range people {
		if p.State == "approved" && p.UserID != except {
			ids = append(ids, p.UserID)
		}
	}
	return tx.Notify(models.NotificationIntent{EventID: eventID, Kind: kind, RideID: tx.Ride().ID, Recipients: ids, CreatedAt: m.now().UTC()})
}
