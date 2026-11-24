package database

import (
	"context"

	"bikers.kz/backend/internal/models"
)

type RidePollAccount struct {
	Status             string
	OnboardingComplete bool
}

type RidePollCreateDecision func(RidePollAccount) error
type RidePollDecision func(RidePollTransaction) error

type RidePollsRepository interface {
	CreateRidePoll(context.Context, models.RidePoll, RidePollCreateDecision) error
	RidePoll(context.Context, string, []byte) (models.RidePoll, error)
	WithRidePoll(context.Context, string, []byte, RidePollDecision) (models.RidePoll, error)
}

// A poll row is locked before the decision. Selection and vote replacement
// serialize against that row and return the state committed by the operation.
type RidePollTransaction interface {
	Poll() models.RidePoll
	Account(string) (RidePollAccount, error)
	ReplaceVote([]string) error
	SelectOption(string) error
}
