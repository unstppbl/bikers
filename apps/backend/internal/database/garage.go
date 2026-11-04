package database

import (
	"context"

	"bikers.kz/backend/internal/models"
)

type GarageOwnerDecision func(GarageOwnerTransaction) error
type BikeDecision func(BikeTransaction) error

type GarageRepository interface {
	ContentHidden(context.Context, string, string) (bool, error)
	PhotoRepository
	Bikes(context.Context, string, string, int) ([]models.Bike, error)
	Bike(context.Context, string) (models.Bike, error)
	WithGarageOwner(context.Context, string, GarageOwnerDecision) error
	WithBike(context.Context, string, BikeDecision) error
}

type GarageOwnerTransaction interface {
	Owner() models.GarageOwner
	CreateBike(models.Bike) (models.Bike, error)
}

type BikeTransaction interface {
	ContentHidden(string, string) (bool, error)
	Owner() models.GarageOwner
	Bike() models.Bike
	UpdateBike(models.Bike) (models.Bike, error)
	DeleteBike() error
	Maintenance(models.MaintenanceCursor, int) ([]models.MaintenanceEntry, error)
	Entry(string) (models.MaintenanceEntry, error)
	CreateEntry(models.MaintenanceEntry) (models.MaintenanceEntry, error)
	UpdateEntry(models.MaintenanceEntry) (models.MaintenanceEntry, error)
	DeleteEntry(string) error
}
