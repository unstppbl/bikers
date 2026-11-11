package database

import (
	"context"
	"time"

	"bikers.kz/backend/internal/models"
)

type RouteQuery struct {
	ViewerID, Scope, Search, CollectionID, AfterID string
	CityAliases                                    []string
	Limit                                          int
}

type RoutePrivateQuery struct {
	OwnerID, AfterID string
	Limit            int
}

type RouteConditionQuery struct {
	RouteID, ViewerID, AfterID string
	Limit                      int
}

type RouteAccount struct {
	User   models.User
	Status string
	Hidden bool
}

type RouteDecision func(RouteTransaction) error
type RoutesRepository interface {
	WithRoutes(context.Context, RouteDecision) error
}

// The repository locks trust before routes, then owns transaction mechanics.
// Callbacks perform local decisions and persistence only, never provider calls.
type RouteTransaction interface {
	Account(string) (RouteAccount, error)
	Blocked(string, string) (bool, error)
	Routes(RouteQuery) ([]models.CommunityRouteSummary, error)
	Route(string) (models.CommunityRoute, error)
	RouteCount(string) (int, error)
	SaveRoute(models.CommunityRoute) error
	DeleteRoute(string) error
	Tracks(RoutePrivateQuery) ([]models.Track, error)
	Track(string) (models.Track, error)
	TrackCount(string) (int, error)
	SaveTrack(models.Track) error
	DeleteTrack(string) error
	Collections(RoutePrivateQuery) ([]models.RouteCollection, error)
	Collection(string) (models.RouteCollection, error)
	CollectionCount(string) (int, error)
	SaveCollection(models.RouteCollection) error
	DeleteCollection(string) error
	CollectionHas(string, string) (bool, error)
	CollectionSize(string) (int, error)
	SetCollectionRoute(string, string, bool, time.Time) error
	Conditions(RouteConditionQuery) ([]models.RouteCondition, error)
	Condition(string) (models.RouteCondition, error)
	ConditionCount(string, time.Time) (int, error)
	RecordConditionCreation(string, time.Time) error
	SaveCondition(models.RouteCondition) error
	DeleteCondition(string) error
}
