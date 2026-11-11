package routes

import (
	"context"
	"math"
	"strings"

	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
)

func boundedNumber(v, min, max float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= min && v <= max
}
func regional(lon, lat float64) bool {
	return models.ValidCoordinate(lat, lon) && lon >= 46 && lon <= 87.5 && lat >= 40.5 && lat <= 55.5
}
func validateTrack(in models.TrackInput) (models.TrackInput, float64, error) {
	in.Name = strings.TrimSpace(in.Name)
	bad := models.Fail("invalid_track", "Choose a recording name, valid Kazakhstan GPS points and a duration up to seven days")
	if in.Name == "" || !textOK(in.Name, 200) || !boundedNumber(in.DurationSeconds, 1, 604800) || len(in.Segments) == 0 || len(in.Segments) > 1000 {
		return in, 0, bad
	}
	total := 0
	for _, segment := range in.Segments {
		if len(segment) == 0 {
			return in, 0, bad
		}
		total += len(segment)
		if total > MaximumTrackPoints {
			return in, 0, bad
		}
		for _, p := range segment {
			if !regional(p[0], p[1]) {
				return in, 0, bad
			}
		}
	}
	meters := models.TrackDistanceMeters(in.Segments)
	if total < 2 || !boundedNumber(meters, 0, 10000000) {
		return in, 0, bad
	}
	return in, meters, nil
}
func (m *Manager) CreateTrack(ctx context.Context, actor models.User, input models.TrackInput) (models.Track, error) {
	var out models.Track
	in, meters, err := validateTrack(input)
	if err != nil {
		return out, err
	}
	at := m.now().UTC()
	out = models.Track{ID: uuid.NewString(), OwnerID: actor.ID, TrackInput: in, DistanceMeters: meters, Version: 1, CreatedAt: at}
	err = m.repo.WithRoutes(ctx, func(tx database.RouteTransaction) error {
		if _, err := active(tx, actor, true); err != nil {
			return err
		}
		n, err := tx.TrackCount(actor.ID)
		if err != nil {
			return err
		}
		if n >= MaximumTracks {
			return models.Fail("track_limited", "The GPS recording limit was reached")
		}
		return tx.SaveTrack(out)
	})
	return out, storageError(err)
}
func (m *Manager) Track(ctx context.Context, actor models.User, raw string) (models.Track, error) {
	var out models.Track
	id, err := identifier(raw)
	if err != nil {
		return out, err
	}
	err = m.repo.WithRoutes(ctx, func(tx database.RouteTransaction) error {
		if _, err := active(tx, actor, false); err != nil {
			return err
		}
		t, err := tx.Track(id)
		if err != nil {
			return err
		}
		if t.OwnerID != actor.ID {
			return missing()
		}
		out = t
		return nil
	})
	return out, storageError(err)
}
func (m *Manager) Tracks(ctx context.Context, actor models.User, after string, limit int) (models.TrackPage, error) {
	out := models.TrackPage{Tracks: []models.Track{}}
	limit, err := pageSize(limit)
	if err != nil {
		return out, err
	}
	b := binding(struct {
		Kind, Actor string
		Limit       int
	}{"tracks", actor.ID, limit})
	id, err := decode(after, b)
	if err != nil {
		return out, err
	}
	err = m.repo.WithRoutes(ctx, func(tx database.RouteTransaction) error {
		if _, err := active(tx, actor, false); err != nil {
			return err
		}
		var err error
		out.Tracks, err = tx.Tracks(database.RoutePrivateQuery{OwnerID: actor.ID, AfterID: id, Limit: limit + 1})
		return err
	})
	if err != nil {
		return out, storageError(err)
	}
	if len(out.Tracks) > limit {
		out.Tracks = out.Tracks[:limit]
		out.NextCursor = encode(out.Tracks[limit-1].ID, b)
	}
	return out, nil
}
func (m *Manager) DeleteTrack(ctx context.Context, actor models.User, raw string, version int64) error {
	id, err := identifier(raw)
	if err != nil {
		return err
	}
	err = m.repo.WithRoutes(ctx, func(tx database.RouteTransaction) error {
		if _, err := active(tx, actor, true); err != nil {
			return err
		}
		t, err := tx.Track(id)
		if err != nil {
			return err
		}
		if t.OwnerID != actor.ID {
			return missing()
		}
		if version < 1 || t.Version != version {
			return stale()
		}
		return tx.DeleteTrack(id)
	})
	return storageError(err)
}
