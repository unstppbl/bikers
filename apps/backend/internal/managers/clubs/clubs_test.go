package clubs

import (
	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/managers/photos"
	"bikers.kz/backend/internal/models"
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

type clubRepo struct {
	mu               sync.Mutex
	club             models.Club
	members          map[string]models.ClubMembership
	links            map[string]models.RideSummary
	candidates       map[string]models.ClubRideCandidate
	list             []models.ClubListing
	listQuery        database.ClubListQuery
	rideLimit        int
	rideNow          time.Time
	includeRequested bool
	commitErr        error
	notifyErr        error
	notices          []models.NotificationIntent
	images           *memoryClubImages
	hidden           map[string]bool
}

func (r *clubRepo) ListClubs(_ context.Context, query database.ClubListQuery) ([]models.ClubListing, error) {
	r.listQuery = query
	return r.list, nil
}
func (r *clubRepo) CreateClub(_ context.Context, club models.Club, state string, _ time.Time, decide database.AccountDecision) (models.Club, error) {
	if err := decide(models.GarageOwner{ID: club.OwnerID, Status: "active"}); err != nil {
		return models.Club{}, err
	}
	club.MemberCount = 1
	r.club = club
	r.members = map[string]models.ClubMembership{club.OwnerID: {State: state, AccountStatus: "active"}}
	return club, nil
}
func (r *clubRepo) WithClub(_ context.Context, id string, decide database.ClubDecision) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if id != r.club.ID {
		return database.ErrNotFound
	}
	members := map[string]models.ClubMembership{}
	for id, m := range r.members {
		members[id] = m
	}
	links := map[string]models.RideSummary{}
	for id, ride := range r.links {
		links[id] = ride
	}
	tx := &clubTx{repo: r, club: r.club, members: members, links: links, notices: append([]models.NotificationIntent(nil), r.notices...)}
	if err := decide(tx); err != nil {
		return err
	}
	if r.commitErr != nil {
		return r.commitErr
	}
	r.club, r.members, r.links, r.notices = tx.club, tx.members, tx.links, tx.notices
	return nil
}

type clubTx struct {
	repo    *clubRepo
	club    models.Club
	members map[string]models.ClubMembership
	links   map[string]models.RideSummary
	notices []models.NotificationIntent
}

func (t *clubTx) Club() models.Club                           { return t.club }
func (t *clubTx) Blocked(_, _ string) (bool, error)           { return false, nil }
func (t *clubTx) ContentHidden(kind, id string) (bool, error) { return t.repo.hidden[kind+":"+id], nil }
func (t *clubTx) Membership(user string) (models.ClubMembership, error) {
	m, ok := t.members[user]
	if !ok {
		return models.ClubMembership{}, database.ErrNotFound
	}
	return m, nil
}
func (t *clubTx) SetMembership(user, state string, _ time.Time) error {
	m := t.members[user]
	m.State = state
	t.members[user] = m
	t.club.MemberCount = 0
	for _, v := range t.members {
		if v.State == "approved" {
			t.club.MemberCount++
		}
	}
	return nil
}
func (t *clubTx) Notify(intent models.NotificationIntent) error {
	if t.repo.notifyErr != nil {
		return t.repo.notifyErr
	}
	t.notices = append(t.notices, intent)
	return nil
}
func (t *clubTx) UpdateClub(club models.Club, _ time.Time) error { t.club = club; return nil }
func (t *clubTx) Members(include bool) ([]models.ClubMember, error) {
	t.repo.includeRequested = include
	out := []models.ClubMember{}
	for id, m := range t.members {
		if m.State == "approved" || (include && m.State == "requested") {
			role := "member"
			if id == t.club.OwnerID {
				role = "owner"
			}
			out = append(out, models.ClubMember{UserID: id, DisplayName: "Rider", State: m.State, Role: role})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UserID < out[j].UserID })
	return out, nil
}
func (t *clubTx) LinkedRides(now time.Time, limit int) ([]models.RideSummary, error) {
	t.repo.rideLimit = limit
	t.repo.rideNow = now
	out := []models.RideSummary{}
	for _, ride := range t.links {
		out = append(out, ride)
	}
	return out, nil
}
func (t *clubTx) RideForLink(id string) (models.ClubRideCandidate, error) {
	r, ok := t.repo.candidates[id]
	if !ok {
		return r, database.ErrNotFound
	}
	return r, nil
}
func (t *clubTx) LinkRide(id, _ string, _ time.Time) error {
	t.links[id] = t.repo.candidates[id].Summary
	return nil
}
func (t *clubTx) UnlinkRide(id string) error { delete(t.links, id); return nil }
func clubFixture() (*Manager, *clubRepo, models.User, models.User) {
	owner := models.User{ID: uuid.NewString(), OnboardingComplete: true}
	member := models.User{ID: uuid.NewString(), OnboardingComplete: true}
	r := &clubRepo{club: models.Club{ClubSummary: models.ClubSummary{ID: uuid.NewString(), Name: "Almaty Riders", City: "Almaty", Description: "Weekend rides", Status: "active"}, OwnerID: owner.ID, Version: 1, MemberCount: 1}, members: map[string]models.ClubMembership{owner.ID: {State: "approved", AccountStatus: "active"}, member.ID: {State: "", AccountStatus: "active"}}, links: map[string]models.RideSummary{}, candidates: map[string]models.ClubRideCandidate{}}
	return New(r, func() time.Time { return time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC) }, photos.New(nil)), r, owner, member
}
func requireFault(t *testing.T, err error, code string) {
	t.Helper()
	var f *models.Fault
	if !errors.As(err, &f) || f.Code != code {
		t.Fatalf("want fault %s, got %v", code, err)
	}
}
func TestCreateValidationAndOwnerApproval(t *testing.T) {
	m, r, owner, _ := clubFixture()
	created, err := m.Create(context.Background(), owner, models.ClubInput{Name: " Club ", City: " Алматы ", Description: " Rides "})
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "Club" || created.City != "Almaty" || created.Version != 1 || created.MemberCount != 1 || created.CanManage == nil || !*created.CanManage || r.members[owner.ID].State != "approved" {
		t.Fatalf("bad created club: %+v", created)
	}
	_, err = m.Create(context.Background(), models.User{}, models.ClubInput{})
	requireFault(t, err, "profile_required")
	_, err = m.Create(context.Background(), owner, models.ClubInput{Name: strings.Repeat("a", 161), City: "Almaty", Description: "Rides"})
	requireFault(t, err, "invalid_club")
}
func TestPendingPrivacyAndApprovedRoster(t *testing.T) {
	m, r, owner, member := clubFixture()
	ctx := context.Background()
	state, err := m.Join(ctx, member, r.club.ID)
	if err != nil || state != "requested" {
		t.Fatalf("join: %s %v", state, err)
	}
	for _, session := range []*models.Session{nil, {User: member}} {
		v, err := m.View(ctx, r.club.ID, session)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(v)
		for _, private := range []string{"owner_id", "member_count", "version"} {
			if strings.Contains(string(data), private) {
				t.Fatalf("leaked %s: %s", private, data)
			}
		}
	}
	_, err = m.Members(ctx, member, r.club.ID)
	requireFault(t, err, "club_member_required")
	_, err = m.Members(ctx, owner, r.club.ID)
	if err != nil || !r.includeRequested {
		t.Fatalf("owner roster: %v", err)
	}
	state, err = m.Decide(ctx, owner, r.club.ID, member.ID, "approve")
	if err != nil || state != "approved" {
		t.Fatalf("approval: %s %v", state, err)
	}
	v, err := m.View(ctx, r.club.ID, &models.Session{User: member})
	if err != nil {
		t.Fatal(err)
	}
	full := v.(models.Club)
	if full.MemberCount != 2 || full.CanManage == nil || *full.CanManage {
		t.Fatalf("member view: %+v", full)
	}
	_, err = m.Members(ctx, member, r.club.ID)
	if err != nil || r.includeRequested {
		t.Fatalf("member roster: %v", err)
	}
	_, err = m.Decide(ctx, owner, r.club.ID, member.ID, "remove")
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.Members(ctx, member, r.club.ID)
	requireFault(t, err, "club_member_required")
	state, err = m.Join(ctx, member, r.club.ID)
	if err != nil || state != "requested" {
		t.Fatalf("re-request: %s %v", state, err)
	}
}
func TestTransferAndClosedHistoricalOwnerPrivacy(t *testing.T) {
	m, r, owner, member := clubFixture()
	ctx := context.Background()
	_, err := m.Leave(ctx, owner, r.club.ID)
	requireFault(t, err, "owner_transfer_required")
	_, err = m.Transfer(ctx, owner, r.club.ID, models.ClubTransfer{Version: 1, UserID: member.ID})
	requireFault(t, err, "invalid_owner")
	r.members[member.ID] = models.ClubMembership{State: "approved", AccountStatus: "suspended"}
	_, err = m.Transfer(ctx, owner, r.club.ID, models.ClubTransfer{Version: 1, UserID: member.ID})
	requireFault(t, err, "invalid_owner")
	r.members[member.ID] = models.ClubMembership{State: "approved", AccountStatus: "active"}
	_, err = m.Transfer(ctx, owner, r.club.ID, models.ClubTransfer{Version: 9, UserID: member.ID})
	requireFault(t, err, "stale_version")
	club, err := m.Transfer(ctx, owner, r.club.ID, models.ClubTransfer{Version: 1, UserID: member.ID})
	if err != nil {
		t.Fatal(err)
	}
	if club.OwnerID != member.ID || club.Version != 2 || *club.CanManage || r.members[owner.ID].State != "approved" {
		t.Fatalf("bad transfer: %+v", club)
	}
	_, err = m.Close(ctx, owner, r.club.ID, 2)
	requireFault(t, err, "forbidden")
	_, err = m.Close(ctx, member, r.club.ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if r.club.Version != 3 || r.club.Status != "closed" {
		t.Fatal("closure not persisted")
	}
	_, err = m.Leave(ctx, member, r.club.ID)
	if err != nil {
		t.Fatal(err)
	}
	if r.club.OwnerID != member.ID {
		t.Fatal("historical owner changed")
	}
	v, err := m.View(ctx, r.club.ID, &models.Session{User: member})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := v.(models.ClubSummary); !ok {
		t.Fatalf("left historical owner got private club: %#v", v)
	}
	_, err = m.Members(ctx, member, r.club.ID)
	requireFault(t, err, "club_member_required")
	v, err = m.Close(ctx, member, r.club.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := v.(models.ClubSummary); !ok {
		t.Fatal("idempotent close leaked historical private data")
	}
	_, err = m.Join(ctx, member, r.club.ID)
	requireFault(t, err, "club_closed")
	_, err = m.Update(ctx, member, r.club.ID, models.ClubPatch{Version: 3})
	requireFault(t, err, "club_closed")
	_, err = m.Members(ctx, owner, r.club.ID)
	if err != nil {
		t.Fatalf("approved old owner lost closed roster: %v", err)
	}
}
func TestMembershipTransitionsAndRollback(t *testing.T) {
	m, r, owner, member := clubFixture()
	ctx := context.Background()
	_, err := m.Decide(ctx, owner, r.club.ID, owner.ID, "remove")
	requireFault(t, err, "owner_transfer_required")
	_, err = m.Decide(ctx, owner, r.club.ID, member.ID, "approve")
	requireFault(t, err, "membership_changed")
	_, _ = m.Join(ctx, member, r.club.ID)
	_, err = m.Decide(ctx, member, r.club.ID, owner.ID, "approve")
	requireFault(t, err, "forbidden")
	_, err = m.Decide(ctx, owner, r.club.ID, member.ID, "reject")
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.Decide(ctx, owner, r.club.ID, member.ID, "reject")
	if err != nil {
		t.Fatal("same decision not idempotent")
	}
	_, _ = m.Join(ctx, member, r.club.ID)
	r.commitErr = errors.New("commit failed")
	_, err = m.Decide(ctx, owner, r.club.ID, member.ID, "approve")
	if err == nil || r.members[member.ID].State != "requested" {
		t.Fatal("failed commit changed membership")
	}
}
func TestLinksNeverGrantRideAccessAndAlwaysProjectPublicFields(t *testing.T) {
	m, r, owner, member := clubFixture()
	m.now = func() time.Time { return time.Date(2026, 11, 6, 15, 0, 0, 0, time.FixedZone("test-clock", 5*60*60)) }
	ctx := context.Background()
	id := uuid.NewString()
	yes := true
	r.candidates[id] = models.ClubRideCandidate{OrganizerID: owner.ID, Summary: models.RideSummary{ID: id, City: "Almaty", Date: "2026-11-06", Description: "Ride preview", Status: "published", CanManage: &yes, ParticipationState: "approved"}}
	if err := m.Link(ctx, member, r.club.ID, id); err == nil {
		t.Fatal("member linked ride")
	}
	if err := m.Link(ctx, owner, r.club.ID, id); err != nil {
		t.Fatal(err)
	}
	if err := m.Link(ctx, owner, r.club.ID, id); err != nil || len(r.links) != 1 {
		t.Fatal("link retry not idempotent")
	}
	rides, err := m.Rides(ctx, r.club.ID, nil)
	if err != nil || len(rides) != 1 || r.rideLimit != 100 {
		t.Fatalf("rides: %+v %v", rides, err)
	}
	if !r.rideNow.Equal(m.now().UTC()) || r.rideNow.Location() != time.UTC {
		t.Fatal("linked ride selection did not receive the manager's UTC clock")
	}
	data, _ := json.Marshal(rides)
	if strings.Contains(string(data), "can_manage") || strings.Contains(string(data), "participation_state") {
		t.Fatalf("private flags leaked: %s", data)
	}
	foreign := uuid.NewString()
	r.candidates[foreign] = models.ClubRideCandidate{OrganizerID: member.ID, Summary: models.RideSummary{ID: foreign, Status: "published"}}
	requireFault(t, m.Link(ctx, owner, r.club.ID, foreign), "forbidden")
	cancelled := uuid.NewString()
	r.candidates[cancelled] = models.ClubRideCandidate{OrganizerID: owner.ID, Summary: models.RideSummary{ID: cancelled, Status: "cancelled"}}
	requireFault(t, m.Link(ctx, owner, r.club.ID, cancelled), "ride_cancelled")
	r.links[foreign] = r.candidates[foreign].Summary
	if err := m.Unlink(ctx, owner, r.club.ID, foreign); err != nil {
		t.Fatal("current owner cannot remove previous owner's association")
	}
	if err := m.Unlink(ctx, owner, r.club.ID, foreign); err != nil {
		t.Fatal("unlink retry failed")
	}
	if len(r.members) != 2 || r.members[member.ID].State != "" {
		t.Fatal("links changed club membership")
	}
}

func TestHiddenClubStopsNewJoinsAndPublicAssociationsButRetainsApprovedCoordination(t *testing.T) {
	m, r, owner, member := clubFixture()
	r.hidden = map[string]bool{"club:" + r.club.ID: true}
	ctx := context.Background()
	_, err := m.Join(ctx, member, r.club.ID)
	requireFault(t, err, "not_found")
	_, err = m.Rides(ctx, r.club.ID, nil)
	requireFault(t, err, "not_found")
	r.members[member.ID] = models.ClubMembership{State: "approved", AccountStatus: "active"}
	if _, err = m.View(ctx, r.club.ID, &models.Session{User: member}); err != nil {
		t.Fatal(err)
	}
	if _, err = m.Rides(ctx, r.club.ID, &models.Session{User: member}); err != nil {
		t.Fatal("approved club lost associated rides", err)
	}

	if _, err = m.Members(ctx, owner, r.club.ID); err != nil {
		t.Fatal(err)
	}
	r.members[member.ID] = models.ClubMembership{State: "approved", AccountStatus: "suspended"}
	_, err = m.View(ctx, r.club.ID, &models.Session{User: member})
	requireFault(t, err, "account_unavailable")
	_, err = m.Members(ctx, member, r.club.ID)
	requireFault(t, err, "account_unavailable")
	_, err = m.Leave(ctx, member, r.club.ID)
	requireFault(t, err, "account_unavailable")
	if r.members[member.ID].State != "approved" {
		t.Fatal("restricted stale actor mutated membership")
	}
}
