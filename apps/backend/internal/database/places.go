package database

import (
	"bikers.kz/backend/internal/models"
	"context"
	"time"
)

type PlaceQuery struct {
	Search, Category, AfterID string
	CityAliases               []string
	Bounds                    *models.PlaceBounds
	Limit                     int
}
type PlaceSubmissionQuery struct {
	SubmitterID, Status, AfterID string
	Limit                        int
}
type PlaceDecision func(PlaceTransaction) error
type PlacesRepository interface {
	PublishedPlaces(context.Context, PlaceQuery) ([]models.Place, error)
	PublishedPlace(context.Context, string) (models.Place, error)
	WithPlaces(context.Context, string, PlaceDecision) error
}

// The repository acquires the global trust lock before the places lock, then
// account and record locks. Managers decide permissions, quotas and changes
// locally in this callback, including import preservation and stale approvals.
type PlaceTransaction interface {
	Account(string) (models.TrustAccount, error)
	Place(string) (models.Place, error)
	PlacesBySources([]string) (map[string]models.Place, error)
	SavePlaces([]models.Place) error
	Submission(string) (models.PlaceSubmission, error)
	Submissions(PlaceSubmissionQuery) ([]models.PlaceSubmission, error)
	SubmissionCount(string, time.Time) (int, error)
	InsertSubmission(models.PlaceSubmission) error
	UpdateSubmission(models.PlaceSubmission) error
}
