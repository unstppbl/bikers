package rides

import (
	"context"
	"testing"

	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
)

func TestCalendarRequiresCurrentApprovedPrivatePlanAccess(t *testing.T) {
	manager, repo, _, owner := rideFixture(t)
	ctx := context.Background()
	applicant := models.User{ID: uuid.NewString()}
	for _, state := range []string{"", "requested", "rejected", "removed", "left"} {
		repo.states[applicant.ID] = state
		_, err := manager.Calendar(ctx, applicant, repo.ride.ID)
		requireFault(t, err, "participant_required")
	}
	repo.states[applicant.ID] = "approved"
	for _, actor := range []models.User{owner, applicant} {
		ride, err := manager.Calendar(ctx, actor, repo.ride.ID)
		if err != nil || ride.Title != repo.ride.Title || ride.MeetingPoint != repo.ride.MeetingPoint {
			t.Fatalf("approved calendar snapshot unavailable: %v", err)
		}
	}
	repo.statuses = map[string]string{applicant.ID: "restricted"}
	_, err := manager.Calendar(ctx, applicant, repo.ride.ID)
	requireFault(t, err, "account_unavailable")
	repo.ride.Status = "cancelled"
	ride, err := manager.Calendar(ctx, owner, repo.ride.ID)
	if err != nil || ride.Status != "cancelled" {
		t.Fatal("organizer cannot export cancellation", err)
	}
}
