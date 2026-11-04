package rides

import (
	"context"

	"bikers.kz/backend/internal/models"
)

// Calendar takes the same private-plan snapshot available to the current
// organizer or approved participant. Pending applicants only have a preview.
func (m *Manager) Calendar(ctx context.Context, actor models.User, id string) (models.Ride, error) {
	view, err := m.View(ctx, id, &models.Session{User: actor})
	if err != nil {
		return models.Ride{}, err
	}
	ride, allowed := view.(models.Ride)
	if !allowed {
		return models.Ride{}, models.Fail("participant_required", "Approved participation is required")
	}
	return ride, nil
}
