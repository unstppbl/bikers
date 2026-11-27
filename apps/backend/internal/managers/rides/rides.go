// Package rides owns ride plans, participation, and visibility rules.
package rides

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"reflect"
	"strings"
	"time"

	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/managers/photos"
	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
)

type Events interface{ Publish(string, string) }
type Planner interface {
	Plan(context.Context, models.User, []models.MapPoint) (*models.MapPlan, error)
}

type Manager struct {
	repo      database.RidesRepository
	events    Events
	now       func() time.Time
	cursorKey [32]byte
	planner   Planner
	photos    *photos.Manager
}

func New(repo database.RidesRepository, events interface{ Publish(string, string) }, now func() time.Time, planner Planner, processor *photos.Manager) *Manager {
	if now == nil {
		now = time.Now
	}
	manager := &Manager{repo: repo, events: events, now: now, planner: planner, photos: processor}
	if _, err := rand.Read(manager.cursorKey[:]); err != nil {
		panic("cannot initialize ride cursor signing key")
	}
	return manager
}

func (m *Manager) Preview(ctx context.Context, id string) (models.RideSummary, error) {
	id, err := canonicalID(id)
	if err != nil {
		return models.RideSummary{}, err
	}
	ride, err := m.repo.Ride(ctx, id)
	if err != nil {
		return models.RideSummary{}, rideError(err)
	}
	hidden, err := m.repo.ContentHidden(ctx, "ride", id)
	if err != nil {
		return models.RideSummary{}, err
	}
	if hidden {
		return models.RideSummary{}, models.Fail("not_found", "Ride not found")
	}
	if ride.DistanceKm == 0 && ride.MapPlan != nil {
		ride.DistanceKm = models.PublicRideDistanceKm(0, ride.MapPlan)
	}
	return publicSummary(ride.RideSummary), nil
}

func (m *Manager) View(ctx context.Context, id string, session *models.Session) (any, error) {
	id, err := canonicalID(id)
	if err != nil {
		return nil, err
	}
	var out any
	err = m.repo.WithRide(ctx, id, func(tx database.RideTransaction) error {
		ride := tx.Ride()
		hidden, err := tx.ContentHidden("ride", id)
		if err != nil {
			return err
		}
		if ride.DistanceKm == 0 && ride.MapPlan != nil {
			ride.DistanceKm = models.PublicRideDistanceKm(0, ride.MapPlan)
		}
		ride.RideSummary = publicSummary(ride.RideSummary)
		if session == nil {
			if hidden {
				return models.Fail("not_found", "Ride not found")
			}
			out = ride.RideSummary
			return nil
		}
		if err := activeRideActor(tx, session.User.ID); err != nil {
			return err
		}
		state, err := tx.Participation(session.User.ID)
		if err != nil {
			return err
		}
		canManage := session.User.ID == ride.OrganizerID
		ride.CanManage, ride.ParticipationState = &canManage, state
		if canManage || state == "approved" {
			if rideEditor(ride, session.User.ID) != nil {
				ride.CoverVersion = 0
			}
			out = ride
			return nil
		}
		if hidden {
			return models.Fail("not_found", "Ride not found")
		}
		out = ride.RideSummary
		return nil
	})
	return out, rideError(err)
}

func (m *Manager) Create(ctx context.Context, actor models.User, input models.RideInput) (models.Ride, error) {
	if !actor.OnboardingComplete {
		return models.Ride{}, models.Fail("profile_required", "Complete your rider profile first")
	}
	input = withMapLabels(input, input.MapPoints)
	input, err := validateInput(input)
	if err != nil {
		return models.Ride{}, err
	}
	if expected := input.ExpectedMapDigest; expected != "" {
		if len(expected) != sha256.Size*2 || len(input.MapPoints) < 2 {
			return models.Ride{}, invalidRide()
		}
		if _, err := hex.DecodeString(expected); err != nil || expected != strings.ToLower(expected) {
			return models.Ride{}, invalidRide()
		}
	}
	plan, err := m.plan(ctx, actor, input.MapPoints)
	if err != nil {
		return models.Ride{}, err
	}
	if input.ExpectedMapDigest != "" && plan.PreviewDigest() != input.ExpectedMapDigest {
		return models.Ride{}, models.Fail("route_preview_changed", "The calculated route changed. Calculate it again before publishing")
	}
	ride := models.Ride{
		RideSummary: models.RideSummary{
			ID:          uuid.NewString(),
			City:        input.City,
			Title:       input.Title,
			Description: input.Description,
			Status:      "published",
			TimeWindow:  models.RideTimeWindow(input.StartsAt, input.Timezone),
			Style:       input.Style,
			Surface:     input.Surface,
			DistanceKm:  input.DistanceKm,
			Capacity:    input.Capacity,
		},
		StartsAt: input.StartsAt, Timezone: input.Timezone, MeetingPoint: input.MeetingPoint,
		Destination: input.Destination, RouteURL: input.RouteURL, Expectations: input.Expectations,
		PlanVersion: 1, OrganizerID: actor.ID, MapPlan: plan,
	}
	stored, err := m.repo.CreateRide(ctx, ride, "approved", m.now().UTC(), func(owner models.GarageOwner) error {
		if owner.Status != "active" {
			return models.Fail("account_unavailable", "Account unavailable")
		}
		return nil
	})
	if err != nil {
		return models.Ride{}, err
	}
	return ownerView(stored), nil
}

func (m *Manager) Update(ctx context.Context, actor models.User, id string, patch models.RidePatch) (models.Ride, error) {
	id, err := canonicalID(id)
	if err != nil {
		return models.Ride{}, err
	}
	var preparedPlan *models.MapPlan
	if patch.MapPoints != nil {
		// A cheap read avoids planning for unauthorized or already-stale edits.
		// Every condition is checked again under the write lock after the call.
		current, err := m.repo.Ride(ctx, id)
		if err != nil {
			return models.Ride{}, rideError(err)
		}
		if err := editableRide(current, actor.ID, patch.PlanVersion); err != nil {
			return models.Ride{}, err
		}
		if _, err := validateInput(withMapLabels(patchedInput(current, patch), *patch.MapPoints)); err != nil {
			return models.Ride{}, err
		}
		preparedPlan, err = m.plan(ctx, actor, *patch.MapPoints)
		if err != nil {
			return models.Ride{}, err
		}
	}
	var result models.Ride
	err = m.repo.WithRide(ctx, id, func(tx database.RideTransaction) error {
		if err := activeRideActor(tx, actor.ID); err != nil {
			return err
		}
		ride := tx.Ride()
		if err := editableRide(ride, actor.ID, patch.PlanVersion); err != nil {
			return err
		}
		input := patchedInput(ride, patch)
		if patch.MapPoints != nil {
			input = withMapLabels(input, *patch.MapPoints)
		}
		input, err := validateInput(input)
		if err != nil {
			return err
		}
		count, err := tx.ApprovedCount()
		if err != nil {
			return err
		}
		if input.Capacity < count {
			return models.Fail("capacity_too_small", "Capacity is below the approved participant count")
		}
		plan := ride.MapPlan
		if patch.MapPoints != nil {
			plan = preparedPlan
		} else if ride.MeetingPoint != input.MeetingPoint || ride.Destination != input.Destination {
			plan = nil
		}
		significant := !ride.StartsAt.Equal(input.StartsAt) || ride.MeetingPoint != input.MeetingPoint || ride.Destination != input.Destination || ride.RouteURL != input.RouteURL || ride.Expectations != input.Expectations || !reflect.DeepEqual(ride.MapPlan, plan)
		ride.City, ride.Description, ride.Title = input.City, input.Description, input.Title
		ride.Style, ride.Surface, ride.DistanceKm = input.Style, input.Surface, input.DistanceKm
		ride.StartsAt, ride.Timezone, ride.MeetingPoint = input.StartsAt, input.Timezone, input.MeetingPoint
		ride.Destination, ride.RouteURL, ride.Expectations, ride.Capacity = input.Destination, input.RouteURL, input.Expectations, input.Capacity
		ride.MapPlan = plan
		ride.PlanVersion++
		if err := tx.UpdateRide(ride); err != nil {
			return err
		}
		result = ownerView(tx.Ride())
		if significant {
			return m.notifyApproved(tx, "ride.updated", actor.ID, uuid.NewString())
		}
		return nil
	})
	if err != nil {
		return models.Ride{}, rideError(err)
	}
	m.publish(id, "ride.updated")
	return result, nil
}

// rideEditor holds the rules shared by every organizer change to a ride,
// including its cover.
func rideEditor(ride models.Ride, actor string) error {
	if ride.OrganizerID != actor {
		return models.Fail("forbidden", "Organizer access required")
	}
	if ride.Status == "cancelled" {
		return models.Fail("ride_cancelled", "Ride is cancelled")
	}
	return nil
}

func editableRide(ride models.Ride, actor string, version int64) error {
	if err := rideEditor(ride, actor); err != nil {
		return err
	}
	if version != ride.PlanVersion {
		return models.Fail("stale_plan", "Ride plan changed; refresh and retry")
	}
	return nil
}

func (m *Manager) plan(ctx context.Context, actor models.User, points []models.MapPoint) (*models.MapPlan, error) {
	if len(points) == 0 {
		return nil, nil
	}
	if m.planner == nil {
		return nil, models.Fail("maps_unavailable", "Maps are temporarily unavailable")
	}
	return m.planner.Plan(ctx, actor, points)
}

func withMapLabels(input models.RideInput, points []models.MapPoint) models.RideInput {
	if len(points) == 0 {
		return input
	}
	input.MeetingPoint = points[0].Label
	input.Destination = ""
	if len(points) > 1 {
		input.Destination = points[len(points)-1].Label
	}
	return input
}

func (m *Manager) Cancel(ctx context.Context, actor models.User, id string, planVersion int64) (models.Ride, error) {
	id, err := canonicalID(id)
	if err != nil {
		return models.Ride{}, err
	}
	var result models.Ride
	err = m.repo.WithRide(ctx, id, func(tx database.RideTransaction) error {
		if err := activeRideActor(tx, actor.ID); err != nil {
			return err
		}
		ride := tx.Ride()
		if ride.OrganizerID != actor.ID {
			return models.Fail("forbidden", "Organizer access required")
		}
		if ride.Status != "cancelled" {
			if ride.PlanVersion != planVersion {
				return models.Fail("stale_plan", "Ride plan changed; refresh and retry")
			}
			ride.Status = "cancelled"
			ride.PlanVersion++
			if err := tx.UpdateRide(ride); err != nil {
				return err
			}
			if err := m.notifyApproved(tx, "ride.cancelled", actor.ID, uuid.NewString()); err != nil {
				return err
			}
		}
		result = ownerView(tx.Ride())
		return nil
	})
	if err != nil {
		return models.Ride{}, rideError(err)
	}
	m.publish(id, "ride.updated")
	return result, nil
}

func (m *Manager) Join(ctx context.Context, actor models.User, id string) (string, error) {
	if !actor.OnboardingComplete {
		return "", models.Fail("profile_required", "Complete your rider profile first")
	}
	id, err := canonicalID(id)
	if err != nil {
		return "", err
	}
	var result string
	err = m.repo.WithRide(ctx, id, func(tx database.RideTransaction) error {
		if err := activeRideActor(tx, actor.ID); err != nil {
			return err
		}
		if tx.Ride().Status == "cancelled" {
			return models.Fail("ride_cancelled", "Ride is cancelled")
		}
		state, err := tx.Participation(actor.ID)
		if err != nil {
			return err
		}
		if state != "approved" {
			hidden, err := tx.ContentHidden("ride", id)
			if err != nil {
				return err
			}
			if hidden {
				return models.Fail("not_found", "Ride not found")
			}
			blocked, err := tx.Blocked(actor.ID, tx.Ride().OrganizerID)
			if err != nil {
				return err
			}
			if blocked {
				return models.Fail("user_blocked", "Unblock the organizer before requesting to join")
			}
		}
		if state != "approved" && state != "requested" {
			state = "requested"
			if err := tx.SetParticipation(actor.ID, state, m.now().UTC()); err != nil {
				return err
			}
			if err := tx.Notify(models.NotificationIntent{EventID: uuid.NewString(), Kind: "participation.requested", RideID: id, Recipients: []string{tx.Ride().OrganizerID}, CreatedAt: m.now().UTC()}); err != nil {
				return err
			}
		}
		result = state
		return nil
	})
	if err != nil {
		return "", rideError(err)
	}
	m.publish(id, "participation.changed")
	return result, nil
}

func (m *Manager) Leave(ctx context.Context, actor models.User, id string) (string, error) {
	id, err := canonicalID(id)
	if err != nil {
		return "", err
	}
	err = m.repo.WithRide(ctx, id, func(tx database.RideTransaction) error {
		if err := activeRideActor(tx, actor.ID); err != nil {
			return err
		}
		if tx.Ride().OrganizerID == actor.ID {
			return models.Fail("organizer_cannot_leave", "The organizer must cancel the ride instead")
		}
		state, err := tx.Participation(actor.ID)
		if err != nil {
			return err
		}
		if state != "" {
			return tx.SetParticipation(actor.ID, "left", m.now().UTC())
		}
		return nil
	})
	if err != nil {
		return "", rideError(err)
	}
	m.publish(id, "participation.changed")
	return "left", nil
}

func (m *Manager) Decide(ctx context.Context, actor models.User, id, target, decision string) (string, error) {
	if decision != "approve" && decision != "reject" && decision != "remove" {
		return "", models.Fail("invalid_decision", "Choose approve, reject, or remove")
	}
	id, err := canonicalID(id)
	if err != nil {
		return "", err
	}
	target, err = canonicalID(target)
	if err != nil {
		return "", err
	}
	want := map[string]string{"approve": "approved", "reject": "rejected", "remove": "removed"}[decision]
	err = m.repo.WithRide(ctx, id, func(tx database.RideTransaction) error {
		if err := activeRideActor(tx, actor.ID); err != nil {
			return err
		}
		ride := tx.Ride()
		if ride.OrganizerID != actor.ID {
			return models.Fail("forbidden", "Organizer access required")
		}
		if target == actor.ID {
			return models.Fail("organizer_cannot_leave", "Organizer participation cannot be removed")
		}
		state, err := tx.Participation(target)
		if err != nil {
			return err
		}
		if state == "" {
			return models.Fail("not_found", "Join request not found")
		}
		if state == want {
			return nil
		}
		switch decision {
		case "approve":
			if err := activeRideActor(tx, target); err != nil {
				return err
			}
			blocked, err := tx.Blocked(actor.ID, target)
			if err != nil {
				return err
			}
			if blocked {
				return models.Fail("user_blocked", "Unblock this rider before approving")
			}
			if ride.Status == "cancelled" {
				return models.Fail("ride_cancelled", "Ride is cancelled")
			}
			if state != "requested" {
				return models.Fail("participation_changed", "A current join request is required")
			}
			count, err := tx.ApprovedCount()
			if err != nil {
				return err
			}
			if count >= ride.Capacity {
				return models.Fail("ride_full", "Ride capacity reached")
			}
		case "reject":
			if state != "requested" {
				return models.Fail("participation_changed", "A current join request is required")
			}
		case "remove":
			if state != "approved" {
				return models.Fail("participation_changed", "Participant is not approved")
			}
		}
		if err := tx.SetParticipation(target, want, m.now().UTC()); err != nil {
			return err
		}
		return tx.Notify(models.NotificationIntent{EventID: uuid.NewString(), Kind: "participation." + want, RideID: id, Recipients: []string{target}, CreatedAt: m.now().UTC()})
	})
	if err != nil {
		return "", rideError(err)
	}
	m.publish(id, "participation.changed")
	return want, nil
}

func (m *Manager) Participants(ctx context.Context, actor models.User, id string) ([]models.Participant, error) {
	id, err := canonicalID(id)
	if err != nil {
		return nil, err
	}
	out := []models.Participant{}
	err = m.repo.WithRide(ctx, id, func(tx database.RideTransaction) error {
		if err := activeRideActor(tx, actor.ID); err != nil {
			return err
		}
		state, err := tx.Participation(actor.ID)
		if err != nil {
			return err
		}
		canManage := tx.Ride().OrganizerID == actor.ID
		if !canManage && state != "approved" {
			return models.Fail("participant_required", "Approved participation required")
		}
		people, err := tx.ParticipationUsers()
		if err != nil {
			return err
		}
		for _, person := range people {
			if canManage || person.State == "approved" {
				out = append(out, person)
			}
		}
		return nil
	})
	return out, rideError(err)
}

func (m *Manager) publish(ride, kind string) {
	if m.events != nil {
		m.events.Publish(ride, kind)
	}
}

// publicSummary keeps only fields that anyone who can see the ride may read.
// Exact meeting coordinates, departure minute, roster and chat remain private.
func publicSummary(summary models.RideSummary) models.RideSummary {
	return models.RideSummary{
		ID:            summary.ID,
		City:          summary.City,
		Date:          summary.Date,
		Title:         summary.Title,
		Description:   summary.Description,
		Status:        summary.Status,
		TimeWindow:    summary.TimeWindow,
		Style:         summary.Style,
		Surface:       summary.Surface,
		DistanceKm:    summary.DistanceKm,
		Capacity:      summary.Capacity,
		ApprovedCount: summary.ApprovedCount,
		OrganizerName: summary.OrganizerName,
		Cover:         summary.Cover,
	}
}

func ownerView(ride models.Ride) models.Ride {
	yes := true
	ride.CanManage = &yes
	ride.ParticipationState = "approved"
	if rideEditor(ride, ride.OrganizerID) != nil {
		ride.CoverVersion = 0
	}
	return ride
}

func canonicalID(raw string) (string, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return "", models.Fail("not_found", "Record not found")
	}
	return id.String(), nil
}

func rideError(err error) error {
	if errors.Is(err, database.ErrNotFound) {
		return models.Fail("not_found", "Ride not found")
	}
	return err
}

func validateInput(input models.RideInput) (models.RideInput, error) {
	input.City = models.NormalizeCity(input.City)
	input.Description = strings.TrimSpace(input.Description)
	input.Title = strings.TrimSpace(input.Title)
	input.Style = strings.TrimSpace(input.Style)
	input.Surface = strings.TrimSpace(input.Surface)
	input.MeetingPoint = strings.TrimSpace(input.MeetingPoint)
	input.Destination = strings.TrimSpace(input.Destination)
	input.RouteURL = strings.TrimSpace(input.RouteURL)
	input.Expectations = strings.TrimSpace(input.Expectations)
	if input.Timezone == "" {
		input.Timezone = "Asia/Almaty"
	}
	if _, err := time.LoadLocation(input.Timezone); err != nil {
		return models.RideInput{}, invalidRide()
	}
	if !models.ValidRideStyle(input.Style) || !models.ValidRideSurface(input.Surface) || input.DistanceKm < 0 || input.DistanceKm > 50000 {
		return models.RideInput{}, invalidRide()
	}
	if input.City == "" || len(input.City) > 160 || input.Description == "" || len(input.Description) > 1000 || input.Title == "" || len(input.Title) > 240 || input.MeetingPoint == "" || len(input.MeetingPoint) > 1000 || len(input.Destination) > 1000 || len(input.Expectations) > 4000 || input.StartsAt.IsZero() || input.StartsAt.Year() < 2020 || input.StartsAt.Year() > 2100 || input.Capacity < 1 || input.Capacity > 1000 {
		return models.RideInput{}, invalidRide()
	}
	if input.RouteURL != "" {
		link, err := url.Parse(input.RouteURL)
		if err != nil || len(input.RouteURL) > 2048 || link.Host == "" || link.User != nil || (link.Scheme != "https" && link.Scheme != "http") {
			return models.RideInput{}, invalidRide()
		}
	}
	return input, nil
}

func invalidRide() error { return models.Fail("invalid_ride", "Check ride fields") }

func patchedInput(ride models.Ride, patch models.RidePatch) models.RideInput {
	input := models.RideInput{City: ride.City, Description: ride.Description, Title: ride.Title, Style: ride.Style, Surface: ride.Surface, DistanceKm: ride.DistanceKm, StartsAt: ride.StartsAt, Timezone: ride.Timezone, MeetingPoint: ride.MeetingPoint, Destination: ride.Destination, RouteURL: ride.RouteURL, Expectations: ride.Expectations, Capacity: ride.Capacity}
	if patch.City != nil {
		input.City = *patch.City
	}
	if patch.Description != nil {
		input.Description = *patch.Description
	}
	if patch.Title != nil {
		input.Title = *patch.Title
	}
	if patch.Style != nil {
		input.Style = *patch.Style
	}
	if patch.Surface != nil {
		input.Surface = *patch.Surface
	}
	if patch.DistanceKm != nil {
		input.DistanceKm = *patch.DistanceKm
	}
	if patch.StartsAt != nil {
		input.StartsAt = *patch.StartsAt
	}
	if patch.Timezone != nil {
		input.Timezone = *patch.Timezone
	}
	if patch.MeetingPoint != nil {
		input.MeetingPoint = *patch.MeetingPoint
	}
	if patch.Destination != nil {
		input.Destination = *patch.Destination
	}
	if patch.RouteURL != nil {
		input.RouteURL = *patch.RouteURL
	}
	if patch.Expectations != nil {
		input.Expectations = *patch.Expectations
	}
	if patch.Capacity != nil {
		input.Capacity = *patch.Capacity
	}
	return input
}

func activeRideActor(tx database.RideTransaction, user string) error {
	status, err := tx.AccountStatus(user)
	if err != nil {
		return err
	}
	if status != "active" {
		return models.Fail("account_unavailable", "Account unavailable")
	}
	return nil
}
