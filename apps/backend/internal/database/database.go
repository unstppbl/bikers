package database

import (
	"bikers.kz/backend/internal/models"
	"context"
	"errors"
	"time"
)

var ErrNotFound = errors.New("record not found")
var ErrConflict = errors.New("record changed")

// Decision callbacks execute under repository-owned locks and must remain local.
type SendDecision func(models.SendWindow) error
type AttemptDecision func(models.OTPChallenge) error
type LoginDecision func(models.OTPChallenge, models.Account) error
type RideDecision func(RideTransaction) error
type AccountDecision func(models.GarageOwner) error
type ProfileDecision func(models.Account) (models.User, error)

type AccountsRepository interface {
	ReserveChallenge(context.Context, models.OTPChallenge, time.Time, time.Time, SendDecision) error
	UpdateChallenge(context.Context, string, string, string, string) error
	AttemptVerification(context.Context, string, AttemptDecision) (models.OTPChallenge, error)
	CommitLogin(context.Context, string, models.Account, models.SessionRecord, []byte, LoginDecision) (models.User, error)
	Session(context.Context, []byte) (models.SessionRecord, error)
	TouchSession(context.Context, []byte, time.Time) error
	RevokeSession(context.Context, []byte, time.Time) error
	UpdateProfile(context.Context, string, ProfileDecision) (models.User, error)
}
type RidesRepository interface {
	ContentHidden(context.Context, string, string) (bool, error)
	ListRides(context.Context, RideListQuery) ([]models.RideListing, error)
	Ride(context.Context, string) (models.Ride, error)
	Participation(context.Context, string, string) (string, error)
	Participants(context.Context, string, bool) ([]models.Participant, error)
	CreateRide(context.Context, models.Ride, string, time.Time, AccountDecision) (models.Ride, error)
	WithRide(context.Context, string, RideDecision) error
}
type RoomsRepository interface {
	RoomAccess(context.Context, string, string) (models.RoomAccess, error)
	Messages(context.Context, string, string, int64, int) (models.MessagePage, error)
	WithRide(context.Context, string, RideDecision) error
}
type SystemRepository interface {
	Migrate(context.Context) error
	Ready(context.Context) error
}
type Repository interface {
	AccountsRepository
	RidesRepository
	RoomsRepository
	SystemRepository
	GarageRepository
	ClubsRepository
	PhotoRepository
	MediaRepository
	TrustRepository
	NotificationRepository
}

// The implementation locks the ride first, then runs the decision and commits.
// Callers never choose lock strength/order or receive driver/transaction handles.
type RideTransaction interface {
	Ride() models.Ride
	AccountStatus(string) (string, error)
	Participation(string) (string, error)
	ApprovedCount() (int, error)
	SetParticipation(string, string, time.Time) error
	UpdateRide(models.Ride) error
	RoomAccess(string) (models.RoomAccess, error)
	Message(string, string) (models.Message, error)
	AppendMessage(models.Message) (models.Message, error)
	Blocked(string, string) (bool, error)
	ContentHidden(string, string) (bool, error)
	ParticipationUsers() ([]models.Participant, error)
	Notify(models.NotificationIntent) error
	Announcement(string) (models.Announcement, error)
	Announcements(int64, int) ([]models.Announcement, error)
	AppendAnnouncement(models.Announcement) (models.Announcement, error)
}
