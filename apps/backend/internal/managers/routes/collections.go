package routes

import (
	"context"
	"strings"

	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
)

func collectionName(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" || !textOK(s, 100) {
		return "", models.Fail("invalid_collection", "Choose a collection name of 1–100 bytes")
	}
	return s, nil
}
func ownCollection(tx database.RouteTransaction, actor models.User, id string) (models.RouteCollection, error) {
	c, err := tx.Collection(id)
	if err != nil {
		return c, err
	}
	if c.OwnerID != actor.ID {
		return c, missing()
	}
	return c, nil
}
func (m *Manager) Collections(ctx context.Context, actor models.User, after string, limit int) (models.RouteCollectionPage, error) {
	out := models.RouteCollectionPage{Collections: []models.RouteCollection{}}
	limit, err := pageSize(limit)
	if err != nil {
		return out, err
	}
	b := binding(struct {
		Kind, Actor string
		Limit       int
	}{"collections", actor.ID, limit})
	id, err := decode(after, b)
	if err != nil {
		return out, err
	}
	err = m.repo.WithRoutes(ctx, func(tx database.RouteTransaction) error {
		if _, err := active(tx, actor, false); err != nil {
			return err
		}
		var err error
		out.Collections, err = tx.Collections(database.RoutePrivateQuery{OwnerID: actor.ID, AfterID: id, Limit: limit + 1})
		return err
	})
	if err != nil {
		return out, storageError(err)
	}
	if len(out.Collections) > limit {
		out.Collections = out.Collections[:limit]
		out.NextCursor = encode(out.Collections[limit-1].ID, b)
	}
	return out, nil
}
func (m *Manager) CreateCollection(ctx context.Context, actor models.User, in models.RouteCollectionInput) (models.RouteCollection, error) {
	var out models.RouteCollection
	name, err := collectionName(in.Name)
	if err != nil {
		return out, err
	}
	if in.Version != 0 {
		return out, stale()
	}
	at := m.now().UTC()
	out = models.RouteCollection{ID: uuid.NewString(), OwnerID: actor.ID, Name: name, Version: 1, CreatedAt: at, UpdatedAt: at}
	err = m.repo.WithRoutes(ctx, func(tx database.RouteTransaction) error {
		if _, err := active(tx, actor, true); err != nil {
			return err
		}
		n, err := tx.CollectionCount(actor.ID)
		if err != nil {
			return err
		}
		if n >= MaximumCollections {
			return models.Fail("collection_limited", "The collection limit was reached")
		}
		return tx.SaveCollection(out)
	})
	return out, storageError(err)
}
func (m *Manager) UpdateCollection(ctx context.Context, actor models.User, raw string, in models.RouteCollectionInput) (models.RouteCollection, error) {
	var out models.RouteCollection
	id, err := identifier(raw)
	if err != nil {
		return out, err
	}
	name, err := collectionName(in.Name)
	if err != nil {
		return out, err
	}
	err = m.repo.WithRoutes(ctx, func(tx database.RouteTransaction) error {
		if _, err := active(tx, actor, true); err != nil {
			return err
		}
		c, err := ownCollection(tx, actor, id)
		if err != nil {
			return err
		}
		if in.Version < 1 || c.Version != in.Version {
			return stale()
		}
		c.Name = name
		c.Version++
		c.UpdatedAt = m.now().UTC()
		if err = tx.SaveCollection(c); err != nil {
			return err
		}
		out = c
		return nil
	})
	return out, storageError(err)
}
func (m *Manager) DeleteCollection(ctx context.Context, actor models.User, raw string, version int64) error {
	id, err := identifier(raw)
	if err != nil {
		return err
	}
	err = m.repo.WithRoutes(ctx, func(tx database.RouteTransaction) error {
		if _, err := active(tx, actor, true); err != nil {
			return err
		}
		c, err := ownCollection(tx, actor, id)
		if err != nil {
			return err
		}
		if version < 1 || c.Version != version {
			return stale()
		}
		return tx.DeleteCollection(id)
	})
	return storageError(err)
}
func (m *Manager) SetCollectionRoute(ctx context.Context, actor models.User, raw, routeRaw string, add bool) error {
	id, err := identifier(raw)
	if err != nil {
		return err
	}
	routeID, err := identifier(routeRaw)
	if err != nil {
		return err
	}
	err = m.repo.WithRoutes(ctx, func(tx database.RouteTransaction) error {
		if _, err := active(tx, actor, true); err != nil {
			return err
		}
		c, err := ownCollection(tx, actor, id)
		if err != nil {
			return err
		}
		found, err := tx.CollectionHas(id, routeID)
		if err != nil {
			return err
		}
		if found == add {
			return nil
		}
		if add {
			r, err := tx.Route(routeID)
			if err != nil {
				return err
			}
			if err = visible(tx, actor, r); err != nil {
				return err
			}
			n, err := tx.CollectionSize(id)
			if err != nil {
				return err
			}
			if n >= MaximumCollectionRoutes {
				return models.Fail("collection_limited", "The collection route limit was reached")
			}
		}
		at := m.now().UTC()
		if err = tx.SetCollectionRoute(id, routeID, add, at); err != nil {
			return err
		}
		c.Version++
		c.UpdatedAt = at
		return tx.SaveCollection(c)
	})
	return storageError(err)
}
