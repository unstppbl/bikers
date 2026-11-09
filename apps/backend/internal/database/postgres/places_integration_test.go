package postgres

import (
	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"context"
	"github.com/google/uuid"
	"strings"
	"testing"
	"time"
)

func TestPlaceBatchFailureRollsBackEveryWrite(t *testing.T) {
	store, pool := testStore(t)
	ctx := context.Background()
	good := models.Place{ID: uuid.NewString(), PlaceInput: models.PlaceInput{Name: "First public place", Category: "fuel", Lat: 43.24, Lon: 76.89, City: "Almaty"}, Source: "osm", SourceRef: "node/3100", SourceURL: "https://www.openstreetmap.org/node/3100", SourceDate: "2026-11-02", Status: "published", Version: 1, UpdatedAt: time.Now().UTC()}
	bad := good
	bad.ID = uuid.NewString()
	bad.SourceRef = "node/3101"
	bad.Name = strings.Repeat("x", 201)
	if err := store.WithPlaces(ctx, "", func(tx database.PlaceTransaction) error { return tx.SavePlaces([]models.Place{good, bad}) }); err == nil {
		t.Fatal("database accepted invalid batch")
	}
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM places").Scan(&count); err != nil || count != 0 {
		t.Fatal("batch committed a partial import", count, err)
	}
	if err := store.WithPlaces(ctx, "", func(tx database.PlaceTransaction) error { return tx.SavePlaces([]models.Place{good}) }); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM places").Scan(&count); err != nil || count != 1 {
		t.Fatal("failed batch poisoned later transactions", count, err)
	}
}
