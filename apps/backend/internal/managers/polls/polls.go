// Package polls owns unlisted scheduling proposals and anonymous availability.
package polls

import (
	"context"
	"crypto/sha256"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
)

const VoterTokenBytes = 32

type Manager struct {
	repo database.RidePollsRepository
	now  func() time.Time
}

func New(repo database.RidePollsRepository, now func() time.Time) *Manager {
	if now == nil {
		now = time.Now
	}
	return &Manager{repo: repo, now: now}
}

func invalid() error {
	return models.Fail("invalid_poll", "Check the proposal and its dates")
}

func closed() error {
	return models.Fail("poll_closed", "This poll is no longer open for voting")
}

func past() error {
	return models.Fail("poll_option_past", "Choose a date and time in the future")
}

func missing() error {
	return models.Fail("not_found", "Poll not found")
}

func canonicalID(raw string) (string, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return "", missing()
	}
	return id.String(), nil
}

func publicPoll(poll models.RidePoll, actor models.User, err error) (models.RidePoll, error) {
	if errors.Is(err, database.ErrNotFound) {
		return models.RidePoll{}, missing()
	}
	if err != nil {
		return models.RidePoll{}, err
	}
	poll.Timezone = models.RidePollTimezone
	poll.CanManage = actor.ID == poll.OrganizerID && actor.OnboardingComplete
	return poll, nil
}

func organizerAllowed(owner database.RidePollAccount) error {
	if owner.Status != "active" {
		return models.Fail("account_unavailable", "Account unavailable")
	}
	if !owner.OnboardingComplete {
		return models.Fail("profile_required", "Complete your rider profile first")
	}
	return nil
}

func (m *Manager) Create(ctx context.Context, actor models.User, in models.RidePollInput) (models.RidePoll, error) {
	if actor.ID == "" {
		return models.RidePoll{}, models.Fail("authentication_required", "Sign in to continue")
	}
	if !actor.OnboardingComplete {
		return models.RidePoll{}, models.Fail("profile_required", "Complete your rider profile first")
	}
	city, description := models.NormalizeCity(in.City), strings.TrimSpace(in.Description)
	if city == "" || len(city) > 160 || description == "" || len(description) > 1000 || !utf8.ValidString(description) || utf8.RuneCountInString(description) > 500 || strings.ContainsRune(description, 0) || strings.ContainsRune(city, 0) || len(in.Options) < 2 || len(in.Options) > 3 {
		return models.RidePoll{}, invalid()
	}
	poll := models.RidePoll{ID: uuid.NewString(), OrganizerID: actor.ID, City: city, Description: description, MyOptionIDs: []string{}}
	seen := make(map[time.Time]bool, len(in.Options))
	for _, raw := range in.Options {
		// Match PostgreSQL timestamp precision before comparing for duplicates.
		starts := raw.UTC().Truncate(time.Microsecond)
		if starts.IsZero() || starts.Year() > 2100 || seen[starts] {
			return models.RidePoll{}, invalid()
		}
		seen[starts] = true
		poll.Options = append(poll.Options, models.RidePollOption{ID: uuid.NewString(), StartsAt: starts})
	}
	sort.Slice(poll.Options, func(i, j int) bool { return poll.Options[i].StartsAt.Before(poll.Options[j].StartsAt) })
	poll.CreatedAt = m.now().UTC()
	err := m.repo.CreateRidePoll(ctx, poll, func(owner database.RidePollAccount) error {
		if err := organizerAllowed(owner); err != nil {
			return err
		}
		if !poll.Options[0].StartsAt.After(m.now()) {
			return past()
		}
		return nil
	})
	return publicPoll(poll, actor, err)
}

func voterHash(pollID string, token []byte) []byte {
	if len(token) != VoterTokenBytes {
		return nil
	}
	hash := sha256.New()
	_, _ = hash.Write([]byte(pollID))
	_, _ = hash.Write(token)
	return hash.Sum(nil)
}

func (m *Manager) View(ctx context.Context, actor models.User, raw string, token []byte) (models.RidePoll, error) {
	id, err := canonicalID(raw)
	if err != nil {
		return models.RidePoll{}, err
	}
	poll, err := m.repo.RidePoll(ctx, id, voterHash(id, token))
	return publicPoll(poll, actor, err)
}

func (m *Manager) Vote(ctx context.Context, actor models.User, raw string, token []byte, choices []string) (models.RidePoll, error) {
	id, err := canonicalID(raw)
	if err != nil {
		return models.RidePoll{}, err
	}
	hash := voterHash(id, token)
	if hash == nil || choices == nil || len(choices) > 3 {
		return models.RidePoll{}, invalid()
	}
	ids := make([]string, 0, len(choices))
	unique := make(map[string]bool, len(choices))
	for _, rawChoice := range choices {
		id, err := canonicalID(rawChoice)
		if err != nil || unique[id] {
			return models.RidePoll{}, invalid()
		}
		unique[id] = true
		ids = append(ids, id)
	}
	poll, err := m.repo.WithRidePoll(ctx, id, hash, func(tx database.RidePollTransaction) error {
		poll := tx.Poll()
		now := m.now()
		if poll.SelectedOptionID != "" || !poll.Options[len(poll.Options)-1].StartsAt.After(now) {
			return closed()
		}
		for _, id := range ids {
			option, err := findOption(poll, id)
			if err != nil {
				return err
			}
			if !option.StartsAt.After(now) {
				return past()
			}
		}
		return tx.ReplaceVote(ids)
	})
	return publicPoll(poll, actor, err)
}

func (m *Manager) Select(ctx context.Context, actor models.User, raw string, token []byte, choice string) (models.RidePoll, error) {
	if actor.ID == "" {
		return models.RidePoll{}, models.Fail("authentication_required", "Sign in to continue")
	}
	id, err := canonicalID(raw)
	if err != nil {
		return models.RidePoll{}, err
	}
	optionID, err := canonicalID(choice)
	if err != nil {
		return models.RidePoll{}, invalid()
	}
	poll, err := m.repo.WithRidePoll(ctx, id, voterHash(id, token), func(tx database.RidePollTransaction) error {
		poll := tx.Poll()
		if actor.ID != poll.OrganizerID {
			return models.Fail("forbidden", "Only the organizer can pick a time")
		}
		owner, err := tx.Account(actor.ID)
		if err != nil {
			return err
		}
		if err = organizerAllowed(owner); err != nil {
			return err
		}
		if poll.SelectedOptionID == optionID {
			return nil
		}
		if poll.SelectedOptionID != "" {
			return closed()
		}
		option, err := findOption(poll, optionID)
		if err != nil {
			return err
		}
		if !option.StartsAt.After(m.now()) {
			return past()
		}
		return tx.SelectOption(optionID)
	})
	return publicPoll(poll, actor, err)
}

func findOption(poll models.RidePoll, id string) (models.RidePollOption, error) {
	for _, option := range poll.Options {
		if option.ID == id {
			return option, nil
		}
	}
	return models.RidePollOption{}, invalid()
}
