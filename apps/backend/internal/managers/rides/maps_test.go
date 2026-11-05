package rides

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
)

type plannerFunc func(context.Context, models.User, []models.MapPoint) (*models.MapPlan, error)

func (p plannerFunc) Plan(ctx context.Context, user models.User, points []models.MapPoint) (*models.MapPlan, error) {
	return p(ctx, user, points)
}
func privatePoints() []models.MapPoint {
	return []models.MapPoint{{Label: "PRIVATE PIN A", Lat: 43.24, Lon: 76.89}, {Label: "PRIVATE PIN B", Lat: 43.25, Lon: 76.91}}
}

func TestMapPlanningRunsOutsideLocksAndRechecksPlanVersion(t *testing.T) {
	m, repo, events, owner := rideFixture(t)
	points := privatePoints()
	m.planner = plannerFunc(func(context.Context, models.User, []models.MapPoint) (*models.MapPlan, error) {
		if repo.locked {
			t.Fatal("external planning ran inside ride transaction")
		}
		repo.ride.PlanVersion++ // Another organizer edit commits during the request.
		return &models.MapPlan{Points: points}, nil
	})
	_, err := m.Update(context.Background(), owner, repo.ride.ID, models.RidePatch{PlanVersion: 1, MapPoints: &points})
	requireFault(t, err, "stale_plan")
	if repo.updateCalls != 0 || repo.ride.MapPlan != nil || len(events.events) != 0 {
		t.Fatal("stale planned route was committed")
	}
}

func TestPrivateMapPlanPersistsWithoutLeakingIntoSummaries(t *testing.T) {
	m, repo, _, owner := rideFixture(t)
	points := privatePoints()
	m.planner = plannerFunc(func(_ context.Context, _ models.User, p []models.MapPoint) (*models.MapPlan, error) {
		if repo.locked {
			t.Fatal("planning under lock")
		}
		return &models.MapPlan{Points: append([]models.MapPoint(nil), p...)}, nil
	})
	full, err := m.Update(context.Background(), owner, repo.ride.ID, models.RidePatch{PlanVersion: 1, MapPoints: &points})
	if err != nil || full.MapPlan == nil || full.MeetingPoint != points[0].Label || full.Destination != points[1].Label {
		t.Fatalf("map save failed: %+v %v", full, err)
	}
	for _, session := range []*models.Session{nil, {User: models.User{ID: uuid.NewString()}}} {
		view, err := m.View(context.Background(), repo.ride.ID, session)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := json.Marshal(view)
		if strings.Contains(string(body), "map_plan") || strings.Contains(string(body), "PRIVATE PIN") || strings.Contains(string(body), "76.89") {
			t.Fatalf("private map leaked: %s", body)
		}
	}
	member := models.User{ID: uuid.NewString()}
	repo.states[member.ID] = "approved"
	view, err := m.View(context.Background(), repo.ride.ID, &models.Session{User: member})
	if err != nil || view.(models.Ride).MapPlan == nil {
		t.Fatal("approved participant could not view map")
	}
	// A coordinate-only change must notify even when its labels are unchanged.
	notificationsBefore := len(repo.notifications)
	points[0].Lat += 0.01
	_, err = m.Update(context.Background(), owner, repo.ride.ID, models.RidePatch{PlanVersion: 2, MapPoints: &points})
	if err != nil || len(repo.notifications) != notificationsBefore+1 || len(repo.notifications[notificationsBefore].Recipients) != 1 || repo.notifications[notificationsBefore].Recipients[0] != member.ID {
		t.Fatalf("coordinate change did not notify participant: %+v %v", repo.notifications, err)
	}
	cleared := []models.MapPoint{}
	full, err = m.Update(context.Background(), owner, repo.ride.ID, models.RidePatch{PlanVersion: 3, MapPoints: &cleared})
	if err != nil || full.MapPlan != nil || full.MeetingPoint != "PRIVATE PIN A" {
		t.Fatal("clear lost text fallback or retained map")
	}
}

func TestManualLocationEditClearsMapAndUnauthorizedSaveDoesNotPlan(t *testing.T) {
	m, repo, _, owner := rideFixture(t)
	repo.ride.MapPlan = &models.MapPlan{Points: privatePoints()}
	points := privatePoints()
	m.planner = plannerFunc(func(context.Context, models.User, []models.MapPoint) (*models.MapPlan, error) {
		t.Fatal("unauthorized edit called router")
		return nil, nil
	})
	_, err := m.Update(context.Background(), models.User{ID: uuid.NewString()}, repo.ride.ID, models.RidePatch{PlanVersion: 1, MapPoints: &points})
	requireFault(t, err, "forbidden")
	label := "Another meeting place"
	updated, err := m.Update(context.Background(), owner, repo.ride.ID, models.RidePatch{PlanVersion: 1, MeetingPoint: &label})
	if err != nil || updated.MapPlan != nil {
		t.Fatal("manual location edit retained stale map")
	}
}

func TestDerivedMapLabelsAreValidatedBeforePlanningAndPersistence(t *testing.T) {
	for _, index := range []int{0, 1} {
		m, repo, _, owner := rideFixture(t)
		m.planner = plannerFunc(func(context.Context, models.User, []models.MapPoint) (*models.MapPlan, error) {
			t.Fatal("oversized derived ride label reached planner")
			return nil, nil
		})
		points := privatePoints()
		points[index].Label = strings.Repeat("я", 501) // 1002 UTF-8 bytes.
		input := models.RideInput{City: repo.ride.City, Description: repo.ride.Description, Title: repo.ride.Title, StartsAt: repo.ride.StartsAt, Timezone: repo.ride.Timezone, MeetingPoint: "Short text", Capacity: 2, MapPoints: points}
		_, err := m.Create(context.Background(), owner, input)
		requireFault(t, err, "invalid_ride")
		_, err = m.Update(context.Background(), owner, repo.ride.ID, models.RidePatch{PlanVersion: 1, MapPoints: &points})
		requireFault(t, err, "invalid_ride")
		if repo.updateCalls != 0 || repo.ride.MapPlan != nil {
			t.Fatal("invalid derived label was persisted")
		}
	}
}
