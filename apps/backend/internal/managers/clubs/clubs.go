// Package clubs owns club visibility, membership and ownership decisions.
package clubs

import (
	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/managers/photos"
	"bikers.kz/backend/internal/models"
	"context"
	"errors"
	"github.com/google/uuid"
	"strings"
	"time"
)

const pageSize = 50
const rideLimit = 100

type Manager struct {
	repo   database.ClubsRepository
	now    func() time.Time
	photos *photos.Manager
}

func New(repo database.ClubsRepository, now func() time.Time, processor *photos.Manager) *Manager {
	if now == nil {
		now = time.Now
	}
	return &Manager{repo: repo, now: now, photos: processor}
}
func clubError(err error) error {
	if errors.Is(err, database.ErrNotFound) {
		return models.Fail("not_found", "Club or member not found")
	}
	return err
}
func identifier(raw string) (string, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return "", models.Fail("not_found", "Record not found")
	}
	return id.String(), nil
}
func validate(input models.ClubInput) (models.ClubInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.City = models.NormalizeCity(input.City)
	input.Description = strings.TrimSpace(input.Description)
	if input.Name == "" || len(input.Name) > 160 || input.City == "" || len(input.City) > 160 || input.Description == "" || len(input.Description) > 2000 {
		return input, models.Fail("invalid_club", "Name, city, and description are required within their limits")
	}
	return input, nil
}
func summary(club models.Club, user string, state string) models.ClubSummary {
	out := models.ClubSummary{ID: club.ID, Name: club.Name, City: club.City, Description: club.Description, Status: club.Status}
	if user != "" {
		manage := club.Status == "active" && club.OwnerID == user
		out.CanManage = &manage
		out.MembershipState = state
	}
	return out
}
func full(club models.Club, user, state string) models.Club {
	club.ClubSummary = summary(club, user, state)
	return club
}
func view(tx database.ClubTransaction, user string) (any, error) {
	club := tx.Club()
	hidden, err := tx.ContentHidden("club", club.ID)
	if err != nil {
		return nil, err
	}
	if user == "" {
		if hidden {
			return nil, models.Fail("not_found", "Club not found")
		}
		return summary(club, "", ""), nil
	}
	member, err := tx.Membership(user)
	if err != nil {
		return nil, err
	}
	if member.AccountStatus != "active" {
		return nil, models.Fail("account_unavailable", "Account unavailable")
	}
	if member.State == "approved" || (club.Status == "active" && club.OwnerID == user) {
		return full(club, user, member.State), nil
	}
	if hidden {
		return nil, models.Fail("not_found", "Club not found")
	}
	return summary(club, user, member.State), nil
}
func owner(tx database.ClubTransaction, actor string, active bool) error {
	club := tx.Club()
	member, err := tx.Membership(actor)
	if err != nil {
		return err
	}
	if member.AccountStatus != "active" {
		return models.Fail("account_unavailable", "Account unavailable")
	}
	if club.OwnerID != actor {
		return models.Fail("forbidden", "Club owner access required")
	}
	if active && club.Status != "active" {
		return models.Fail("club_closed", "Club is closed")
	}
	return nil
}
func version(club models.Club, want int64) error {
	if want != club.Version {
		return models.Fail("stale_version", "Club changed; refresh and retry")
	}
	return nil
}

func (m *Manager) List(ctx context.Context, session *models.Session, cursor string, mine bool) (models.ClubPage, error) {
	out := models.ClubPage{Clubs: []models.ClubSummary{}}
	user := ""
	if session != nil {
		user = session.User.ID
	}
	if mine && user == "" {
		return out, models.Fail("authentication_required", "Sign in to continue")
	}
	if cursor != "" {
		id, err := uuid.Parse(cursor)
		if err != nil {
			return out, models.Fail("invalid_cursor", "Invalid club cursor")
		}
		cursor = id.String()
	}
	rows, err := m.repo.ListClubs(ctx, user, mine, cursor, pageSize+1)
	if err != nil {
		return out, err
	}
	if len(rows) > pageSize {
		rows = rows[:pageSize]
		out.NextCursor = rows[len(rows)-1].Club.ID
	}
	for _, row := range rows {
		out.Clubs = append(out.Clubs, summary(row.Club, user, row.MembershipState))
	}
	return out, nil
}
func (m *Manager) View(ctx context.Context, id string, session *models.Session) (any, error) {
	id, err := identifier(id)
	if err != nil {
		return nil, err
	}
	user := ""
	if session != nil {
		user = session.User.ID
	}
	var out any
	err = m.repo.WithClub(ctx, id, func(tx database.ClubTransaction) error { var e error; out, e = view(tx, user); return e })
	return out, clubError(err)
}
func (m *Manager) Create(ctx context.Context, actor models.User, input models.ClubInput) (models.Club, error) {
	if !actor.OnboardingComplete {
		return models.Club{}, models.Fail("profile_required", "Complete your rider profile first")
	}
	input, err := validate(input)
	if err != nil {
		return models.Club{}, err
	}
	club := models.Club{ClubSummary: models.ClubSummary{ID: uuid.NewString(), Name: input.Name, City: input.City, Description: input.Description, Status: "active"}, OwnerID: actor.ID, Version: 1}
	stored, err := m.repo.CreateClub(ctx, club, "approved", m.now().UTC(), func(owner models.GarageOwner) error {
		if owner.Status != "active" {
			return models.Fail("account_unavailable", "Account unavailable")
		}
		return nil
	})
	if err != nil {
		return models.Club{}, clubError(err)
	}
	return full(stored, actor.ID, "approved"), nil
}
func (m *Manager) Update(ctx context.Context, actor models.User, id string, patch models.ClubPatch) (models.Club, error) {
	id, err := identifier(id)
	if err != nil {
		return models.Club{}, err
	}
	var out models.Club
	err = m.repo.WithClub(ctx, id, func(tx database.ClubTransaction) error {
		if err := owner(tx, actor.ID, true); err != nil {
			return err
		}
		club := tx.Club()
		if err := version(club, patch.Version); err != nil {
			return err
		}
		input := models.ClubInput{Name: club.Name, City: club.City, Description: club.Description}
		if patch.Name != nil {
			input.Name = *patch.Name
		}
		if patch.City != nil {
			input.City = *patch.City
		}
		if patch.Description != nil {
			input.Description = *patch.Description
		}
		input, err := validate(input)
		if err != nil {
			return err
		}
		club.Name, club.City, club.Description = input.Name, input.City, input.Description
		club.Version++
		if err := tx.UpdateClub(club, m.now().UTC()); err != nil {
			return err
		}
		out = full(tx.Club(), actor.ID, "approved")
		return nil
	})
	return out, clubError(err)
}
func (m *Manager) Close(ctx context.Context, actor models.User, id string, want int64) (any, error) {
	id, err := identifier(id)
	if err != nil {
		return nil, err
	}
	var out any
	err = m.repo.WithClub(ctx, id, func(tx database.ClubTransaction) error {
		if err := owner(tx, actor.ID, false); err != nil {
			return err
		}
		club := tx.Club()
		if club.Status != "closed" {
			if err := version(club, want); err != nil {
				return err
			}
			club.Status = "closed"
			club.Version++
			if err := tx.UpdateClub(club, m.now().UTC()); err != nil {
				return err
			}
		}
		var e error
		out, e = view(tx, actor.ID)
		return e
	})
	return out, clubError(err)
}
func (m *Manager) Transfer(ctx context.Context, actor models.User, id string, input models.ClubTransfer) (models.Club, error) {
	id, err := identifier(id)
	if err != nil {
		return models.Club{}, err
	}
	target, err := identifier(input.UserID)
	if err != nil {
		return models.Club{}, err
	}
	var out models.Club
	err = m.repo.WithClub(ctx, id, func(tx database.ClubTransaction) error {
		if err := owner(tx, actor.ID, true); err != nil {
			return err
		}
		club := tx.Club()
		if err := version(club, input.Version); err != nil {
			return err
		}
		member, err := tx.Membership(target)
		if err != nil {
			return err
		}
		if target == actor.ID || member.State != "approved" || member.AccountStatus != "active" {
			return models.Fail("invalid_owner", "Choose another active approved member")
		}
		club.OwnerID = target
		club.Version++
		if err := tx.UpdateClub(club, m.now().UTC()); err != nil {
			return err
		}
		if err := tx.SetMembership(actor.ID, "approved", m.now().UTC()); err != nil {
			return err
		}
		out = full(tx.Club(), actor.ID, "approved")
		return nil
	})
	return out, clubError(err)
}
func (m *Manager) Join(ctx context.Context, actor models.User, id string) (string, error) {
	if !actor.OnboardingComplete {
		return "", models.Fail("profile_required", "Complete your rider profile first")
	}
	id, err := identifier(id)
	if err != nil {
		return "", err
	}
	state := ""
	err = m.repo.WithClub(ctx, id, func(tx database.ClubTransaction) error {
		if tx.Club().Status != "active" {
			return models.Fail("club_closed", "Club is closed")
		}
		member, err := tx.Membership(actor.ID)
		if err != nil {
			return err
		}
		state = member.State
		if member.AccountStatus != "active" {
			return models.Fail("account_unavailable", "Account unavailable")
		}
		if state != "approved" {
			hidden, err := tx.ContentHidden("club", id)
			if err != nil {
				return err
			}
			if hidden {
				return models.Fail("not_found", "Club not found")
			}
			blocked, err := tx.Blocked(actor.ID, tx.Club().OwnerID)
			if err != nil {
				return err
			}
			if blocked {
				return models.Fail("user_blocked", "Unblock the club owner before requesting membership")
			}
		}
		if state != "requested" && state != "approved" {
			state = "requested"
			at := m.now().UTC()
			if err := tx.SetMembership(actor.ID, state, at); err != nil {
				return err
			}
			return tx.Notify(models.NotificationIntent{EventID: uuid.NewString(), Kind: "club.membership.requested", ClubID: id, Recipients: []string{tx.Club().OwnerID}, CreatedAt: at})
		}
		return nil
	})
	return state, clubError(err)
}
func (m *Manager) Leave(ctx context.Context, actor models.User, id string) (string, error) {
	id, err := identifier(id)
	if err != nil {
		return "", err
	}
	err = m.repo.WithClub(ctx, id, func(tx database.ClubTransaction) error {
		club := tx.Club()
		if club.Status == "active" && club.OwnerID == actor.ID {
			return models.Fail("owner_transfer_required", "Transfer ownership or close the club before leaving")
		}
		member, err := tx.Membership(actor.ID)
		if err != nil {
			return err
		}
		if member.AccountStatus != "active" {
			return models.Fail("account_unavailable", "Account unavailable")
		}
		if member.State != "" && member.State != "left" {
			return tx.SetMembership(actor.ID, "left", m.now().UTC())
		}
		return nil
	})
	return "left", clubError(err)
}
func (m *Manager) Decide(ctx context.Context, actor models.User, id, target, decision string) (string, error) {
	want := map[string]string{"approve": "approved", "reject": "rejected", "remove": "removed"}[decision]
	if want == "" {
		return "", models.Fail("invalid_decision", "Choose approve, reject, or remove")
	}
	id, err := identifier(id)
	if err != nil {
		return "", err
	}
	target, err = identifier(target)
	if err != nil {
		return "", err
	}
	err = m.repo.WithClub(ctx, id, func(tx database.ClubTransaction) error {
		if err := owner(tx, actor.ID, true); err != nil {
			return err
		}
		if target == tx.Club().OwnerID {
			return models.Fail("owner_transfer_required", "The owner's membership cannot be changed")
		}
		member, err := tx.Membership(target)
		if err != nil {
			return err
		}
		if member.State == want {
			return nil
		}
		if decision == "approve" {
			blocked, err := tx.Blocked(actor.ID, target)
			if err != nil {
				return err
			}
			if blocked {
				return models.Fail("user_blocked", "Unblock this rider before approving")
			}
		}
		if (decision == "approve" && (member.State != "requested" || member.AccountStatus != "active")) || (decision == "reject" && member.State != "requested") || (decision == "remove" && member.State != "approved") {
			return models.Fail("membership_changed", "A current matching membership state is required")
		}
		at := m.now().UTC()
		if err := tx.SetMembership(target, want, at); err != nil {
			return err
		}
		return tx.Notify(models.NotificationIntent{EventID: uuid.NewString(), Kind: "club.membership." + want, ClubID: id, Recipients: []string{target}, CreatedAt: at})
	})
	return want, clubError(err)
}
func (m *Manager) Members(ctx context.Context, actor models.User, id string) ([]models.ClubMember, error) {
	id, err := identifier(id)
	if err != nil {
		return nil, err
	}
	var out []models.ClubMember
	err = m.repo.WithClub(ctx, id, func(tx database.ClubTransaction) error {
		club := tx.Club()
		member, err := tx.Membership(actor.ID)
		if err != nil {
			return err
		}
		if member.AccountStatus != "active" {
			return models.Fail("account_unavailable", "Account unavailable")
		}
		owner := club.OwnerID == actor.ID
		if member.State != "approved" && !(owner && club.Status == "active") {
			return models.Fail("club_member_required", "Approved club membership required")
		}
		out, err = tx.Members(owner)
		return err
	})
	return out, clubError(err)
}
func (m *Manager) Rides(ctx context.Context, id string, session *models.Session) ([]models.RideSummary, error) {
	id, err := identifier(id)
	if err != nil {
		return nil, err
	}
	out := []models.RideSummary{}
	err = m.repo.WithClub(ctx, id, func(tx database.ClubTransaction) error {
		hidden, err := tx.ContentHidden("club", id)
		if err != nil {
			return err
		}
		if hidden {
			if session == nil {
				return models.Fail("not_found", "Club not found")
			}
			member, err := tx.Membership(session.User.ID)
			if err != nil {
				return err
			}
			if member.AccountStatus != "active" || (member.State != "approved" && tx.Club().OwnerID != session.User.ID) {
				return models.Fail("not_found", "Club not found")
			}
		}
		rows, err := tx.LinkedRides(m.now().UTC(), rideLimit)
		if err != nil {
			return err
		}
		for _, r := range rows {
			out = append(out, models.RideSummary{ID: r.ID, City: r.City, Date: r.Date, Description: r.Description, Status: r.Status})
		}
		return nil
	})
	return out, clubError(err)
}
func (m *Manager) Link(ctx context.Context, actor models.User, id, rideID string) error {
	id, err := identifier(id)
	if err != nil {
		return err
	}
	rideID, err = identifier(rideID)
	if err != nil {
		return err
	}
	err = m.repo.WithClub(ctx, id, func(tx database.ClubTransaction) error {
		if err := owner(tx, actor.ID, true); err != nil {
			return err
		}
		ride, err := tx.RideForLink(rideID)
		if err != nil {
			return err
		}
		if ride.OrganizerID != actor.ID {
			return models.Fail("forbidden", "Link only rides you organize")
		}
		if ride.Summary.Status != "published" {
			return models.Fail("ride_cancelled", "Only published rides can be linked")
		}
		return tx.LinkRide(rideID, actor.ID, m.now().UTC())
	})
	return clubError(err)
}
func (m *Manager) Unlink(ctx context.Context, actor models.User, id, rideID string) error {
	id, err := identifier(id)
	if err != nil {
		return err
	}
	rideID, err = identifier(rideID)
	if err != nil {
		return err
	}
	err = m.repo.WithClub(ctx, id, func(tx database.ClubTransaction) error {
		if err := owner(tx, actor.ID, true); err != nil {
			return err
		}
		return tx.UnlinkRide(rideID)
	})
	return clubError(err)
}
