package routes

import (
	"context"
	"strings"
	"time"

	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
)

func validateCondition(in models.RouteConditionInput, now time.Time) (models.RouteConditionInput, error) {
	in.Kind = strings.TrimSpace(in.Kind)
	in.Description = strings.TrimSpace(in.Description)
	in.ObservedAt = in.ObservedAt.UTC()
	bad := models.Fail("invalid_condition", "Choose an observation within the last year, a report kind and a description up to 2000 bytes")
	switch in.Kind {
	case "gravel", "closure", "roadworks", "damage", "other":
	default:
		return in, bad
	}
	if in.Description == "" || !textOK(in.Description, 2000) || in.ObservedAt.IsZero() || in.ObservedAt.After(now) || in.ObservedAt.Before(now.AddDate(-1, 0, 0)) {
		return in, bad
	}
	return in, nil
}
func conditionState(c models.RouteCondition, now time.Time) models.RouteCondition {
	c.ExpiresAt = c.ObservedAt.Add(30 * 24 * time.Hour)
	c.State = "recent"
	if !now.Before(c.ExpiresAt) {
		c.State = "expired"
	}
	if c.Resolved {
		c.State = "resolved"
	}
	return c
}
func (m *Manager) Conditions(ctx context.Context, actor models.User, raw, after string, limit int) (models.RouteConditionPage, error) {
	out := models.RouteConditionPage{Conditions: []models.RouteCondition{}}
	id, err := identifier(raw)
	if err != nil {
		return out, err
	}
	limit, err = pageSize(limit)
	if err != nil {
		return out, err
	}
	b := binding(struct {
		Kind, Actor, Route string
		Limit              int
	}{"conditions", actor.ID, id, limit})
	afterID, err := decode(after, b)
	if err != nil {
		return out, err
	}
	at := m.now().UTC()
	err = m.repo.WithRoutes(ctx, func(tx database.RouteTransaction) error {
		if err := optionalActor(tx, actor); err != nil {
			return err
		}
		r, err := tx.Route(id)
		if err != nil {
			return err
		}
		if err = visible(tx, actor, r); err != nil {
			return err
		}
		out.Conditions, err = tx.Conditions(database.RouteConditionQuery{RouteID: id, ViewerID: actor.ID, AfterID: afterID, Limit: limit + 1})
		return err
	})
	if err != nil {
		return out, storageError(err)
	}
	if len(out.Conditions) > limit {
		out.Conditions = out.Conditions[:limit]
		out.NextCursor = encode(out.Conditions[limit-1].ID, b)
	}
	for i, c := range out.Conditions {
		out.Conditions[i] = conditionState(c, at)
	}
	return out, nil
}
func (m *Manager) CreateCondition(ctx context.Context, actor models.User, raw string, input models.RouteConditionInput) (models.RouteCondition, error) {
	var out models.RouteCondition
	id, err := identifier(raw)
	if err != nil {
		return out, err
	}
	at := m.now().UTC()
	in, err := validateCondition(input, at)
	if err != nil {
		return out, err
	}
	if in.Version != 0 {
		return out, stale()
	}
	err = m.repo.WithRoutes(ctx, func(tx database.RouteTransaction) error {
		a, err := active(tx, actor, true)
		if err != nil {
			return err
		}
		r, err := tx.Route(id)
		if err != nil {
			return err
		}
		if err = visible(tx, actor, r); err != nil {
			return err
		}
		n, err := tx.ConditionCount(actor.ID, at.Truncate(24*time.Hour))
		if err != nil {
			return err
		}
		if n >= DailyConditionLimit {
			return models.Fail("condition_limited", "The daily road-condition report limit was reached")
		}
		out = models.RouteCondition{ID: uuid.NewString(), RouteID: id, RouteConditionInput: in, AuthorID: actor.ID, AuthorName: a.User.Profile.DisplayName, CreatedAt: at, UpdatedAt: at}
		out.Version = 1
		if err = tx.SaveCondition(out); err != nil {
			return err
		}
		return tx.RecordConditionCreation(actor.ID, at)
	})
	return conditionState(out, at), storageError(err)
}
func (m *Manager) UpdateCondition(ctx context.Context, actor models.User, routeRaw, raw string, input models.RouteConditionInput) (models.RouteCondition, error) {
	var out models.RouteCondition
	routeID, err := identifier(routeRaw)
	if err != nil {
		return out, err
	}
	id, err := identifier(raw)
	if err != nil {
		return out, err
	}
	at := m.now().UTC()
	in, err := validateCondition(input, at)
	if err != nil {
		return out, err
	}
	err = m.repo.WithRoutes(ctx, func(tx database.RouteTransaction) error {
		a, err := active(tx, actor, true)
		if err != nil {
			return err
		}
		r, err := tx.Route(routeID)
		if err != nil {
			return err
		}
		if err = visible(tx, actor, r); err != nil {
			return err
		}
		c, err := tx.Condition(id)
		if err != nil {
			return err
		}
		if c.RouteID != routeID || c.AuthorID != actor.ID {
			return missing()
		}
		if in.Version < 1 || c.Version != in.Version {
			return stale()
		}
		c.RouteConditionInput = in
		c.Version++
		c.AuthorName = a.User.Profile.DisplayName
		c.UpdatedAt = at
		if err = tx.SaveCondition(c); err != nil {
			return err
		}
		out = c
		return nil
	})
	return conditionState(out, at), storageError(err)
}
func (m *Manager) DeleteCondition(ctx context.Context, actor models.User, routeRaw, raw string, version int64) error {
	routeID, err := identifier(routeRaw)
	if err != nil {
		return err
	}
	id, err := identifier(raw)
	if err != nil {
		return err
	}
	err = m.repo.WithRoutes(ctx, func(tx database.RouteTransaction) error {
		if _, err := active(tx, actor, true); err != nil {
			return err
		}
		c, err := tx.Condition(id)
		if err != nil {
			return err
		}
		if c.RouteID != routeID || c.AuthorID != actor.ID {
			return missing()
		}
		if version < 1 || c.Version != version {
			return stale()
		}
		return tx.DeleteCondition(id)
	})
	return storageError(err)
}
