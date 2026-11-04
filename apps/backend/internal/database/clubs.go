package database

import (
	"bikers.kz/backend/internal/models"
	"context"
	"time"
)

type ClubDecision func(ClubTransaction) error
type ClubsRepository interface {
	ClubImageRepository
	ListClubs(context.Context, string, bool, string, int) ([]models.ClubListing, error)
	CreateClub(context.Context, models.Club, string, time.Time, AccountDecision) (models.Club, error)
	WithClub(context.Context, string, ClubDecision) error
}

// The repository locks the club row before any membership/account rows. Callback
// work is local; driver handles and lock choices remain inside the repository.
type ClubTransaction interface {
	Club() models.Club
	Membership(string) (models.ClubMembership, error)
	SetMembership(string, string, time.Time) error
	Notify(models.NotificationIntent) error
	UpdateClub(models.Club, time.Time) error
	Members(bool) ([]models.ClubMember, error)
	LinkedRides(time.Time, int) ([]models.RideSummary, error)
	RideForLink(string) (models.ClubRideCandidate, error)
	LinkRide(string, string, time.Time) error
	UnlinkRide(string) error
	Blocked(string, string) (bool, error)
	ContentHidden(string, string) (bool, error)
}
