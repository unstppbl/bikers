package rides

import (
	"context"
	"testing"

	"bikers.kz/backend/internal/models"
)

func TestRouteChangesRecalculateAutomaticDistanceWithoutPersistingEstimate(t *testing.T) {
	m, repo, _, owner := rideFixture(t)
	points := privatePoints()
	repo.ride.MapPlan = &models.MapPlan{Points: points, DistanceMeters: 100000}
	repo.ride.PublicDetailsPublished = true
	view, err := m.View(context.Background(), repo.ride.ID, &models.Session{User: owner})
	if err != nil || view.(models.Ride).DistanceKm != 100 || view.(models.Ride).DistanceOverrideKm != 0 {
		t.Fatal("automatic distance missing", view, err)
	}
	m.planner = plannerFunc(func(context.Context, models.User, []models.MapPoint) (*models.MapPlan, error) {
		return &models.MapPlan{Points: points, DistanceMeters: 200000}, nil
	})
	updated, err := m.Update(context.Background(), owner, repo.ride.ID, models.RidePatch{PlanVersion: 1, MapPoints: &points})
	if err != nil || updated.DistanceKm != 200 || updated.DistanceOverrideKm != 0 || repo.ride.DistanceOverrideKm != 0 {
		t.Fatal("route edit froze an automatic distance", updated, err)
	}
	explicit := 75
	updated, err = m.Update(context.Background(), owner, repo.ride.ID, models.RidePatch{PlanVersion: 2, DistanceKm: &explicit})
	if err != nil || updated.DistanceKm != 75 || updated.DistanceOverrideKm != 75 {
		t.Fatal("manual distance override lost", updated, err)
	}
	points[0].Lat += .01
	updated, err = m.Update(context.Background(), owner, repo.ride.ID, models.RidePatch{PlanVersion: 3, MapPoints: &points})
	if err != nil || updated.DistanceKm != 75 || updated.DistanceOverrideKm != 75 {
		t.Fatal("route edit discarded manual override", updated, err)
	}
	auto := 0
	updated, err = m.Update(context.Background(), owner, repo.ride.ID, models.RidePatch{PlanVersion: 4, DistanceKm: &auto})
	if err != nil || updated.DistanceKm != 200 || updated.DistanceOverrideKm != 0 {
		t.Fatal("clearing manual override did not restore current route estimate", updated, err)
	}
}
