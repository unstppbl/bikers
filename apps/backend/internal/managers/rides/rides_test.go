package rides

import (
	"context"
	"errors"
	"testing"
	"time"

	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
)

type rideRepository struct {
	ride                              models.Ride
	states                            map[string]string
	statuses                          map[string]string
	hidden                            map[string]bool
	list                              []models.RideListing
	listUser                          string
	listLimit                         int
	locked                            bool
	commitErr                         error
	setCalls, countCalls, updateCalls int
	createdState                      string
	createdDate                       string
	participants                      []models.Participant
	includePending                    bool
}

func (r *rideRepository) ListRides(_ context.Context, user string, _ time.Time, limit int) ([]models.RideListing, error) {
	r.listUser, r.listLimit = user, limit
	return r.list, nil
}
func (r *rideRepository) Ride(_ context.Context, id string) (models.Ride, error) {
	if id != r.ride.ID {
		return models.Ride{}, database.ErrNotFound
	}
	return r.ride, nil
}
func (r *rideRepository) Participation(_ context.Context, _, user string) (string, error) {
	return r.states[user], nil
}
func (*rideRepository) ContentHidden(context.Context, string, string) (bool, error) {
	return false, nil
}
func (r *rideRepository) Participants(_ context.Context, _ string, pending bool) ([]models.Participant, error) {
	r.includePending = pending
	return r.participants, nil
}
func (r *rideRepository) CreateRide(_ context.Context, ride models.Ride, ownerState string, _ time.Time, decide database.AccountDecision) (models.Ride, error) {
	if err := decide(models.GarageOwner{ID: ride.OrganizerID, Status: "active"}); err != nil {
		return models.Ride{}, err
	}
	ride.Date = r.createdDate
	r.ride = ride
	r.states = map[string]string{ride.OrganizerID: ownerState}
	r.createdState = ownerState
	return r.ride, nil
}
func (r *rideRepository) WithRide(_ context.Context, id string, decide database.RideDecision) error {
	if id != r.ride.ID {
		return database.ErrNotFound
	}
	r.locked = true
	defer func() { r.locked = false }()
	states := make(map[string]string, len(r.states))
	for id, state := range r.states {
		states[id] = state
	}
	tx := &rideTransaction{repo: r, ride: r.ride, states: states}
	if err := decide(tx); err != nil {
		return err
	}
	if r.commitErr != nil {
		return r.commitErr
	}
	r.ride, r.states = tx.ride, tx.states
	return nil
}

type rideTransaction struct {
	repo   *rideRepository
	ride   models.Ride
	states map[string]string
}

func (tx *rideTransaction) Ride() models.Ride                         { return tx.ride }
func (tx *rideTransaction) Participation(user string) (string, error) { return tx.states[user], nil }
func (tx *rideTransaction) ApprovedCount() (int, error) {
	tx.repo.countCalls++
	n := 0
	for _, state := range tx.states {
		if state == "approved" {
			n++
		}
	}
	return n, nil
}
func (tx *rideTransaction) SetParticipation(user, state string, _ time.Time) error {
	tx.repo.setCalls++
	tx.states[user] = state
	return nil
}
func (tx *rideTransaction) UpdateRide(ride models.Ride) error {
	tx.repo.updateCalls++
	ride.Date = "repository-mapped-date"
	tx.ride = ride
	return nil
}
func (*rideTransaction) RoomAccess(string) (models.RoomAccess, error) {
	panic("unexpected room access")
}
func (*rideTransaction) Message(string, string) (models.Message, error) {
	panic("unexpected message lookup")
}
func (*rideTransaction) AppendMessage(models.Message) (models.Message, error) {
	panic("unexpected message append")
}
func (*rideTransaction) Blocked(string, string) (bool, error) { return false, nil }
func (tx *rideTransaction) AccountStatus(id string) (string, error) {
	if status, ok := tx.repo.statuses[id]; ok {
		return status, nil
	}
	return "active", nil
}
func (tx *rideTransaction) ContentHidden(kind, id string) (bool, error) {
	return tx.repo.hidden[kind+":"+id], nil
}
func (tx *rideTransaction) ParticipationUsers() ([]models.Participant, error) {
	out := []models.Participant{}
	for id, state := range tx.states {
		out = append(out, models.Participant{UserID: id, State: state})
	}
	return out, nil
}
func (*rideTransaction) Notify(models.NotificationIntent) error { return nil }
func (*rideTransaction) Announcement(string) (models.Announcement, error) {
	return models.Announcement{}, database.ErrNotFound
}
func (*rideTransaction) Announcements(int64, int) ([]models.Announcement, error) {
	return []models.Announcement{}, nil
}
func (*rideTransaction) AppendAnnouncement(models.Announcement) (models.Announcement, error) {
	panic("unexpected announcement append")
}

type rideEvents struct {
	t      *testing.T
	repo   *rideRepository
	events []models.Event
}

func (e *rideEvents) Publish(ride, kind string) {
	e.t.Helper()
	if e.repo.locked || e.repo.commitErr != nil {
		e.t.Fatal("event published before successful commit")
	}
	e.events = append(e.events, models.Event{RideID: ride, Type: kind})
}
func rideFixture(t *testing.T) (*Manager, *rideRepository, *rideEvents, models.User) {
	t.Helper()
	owner := models.User{ID: uuid.NewString(), OnboardingComplete: true}
	repo := &rideRepository{ride: models.Ride{RideSummary: models.RideSummary{ID: uuid.NewString(), City: "Almaty", Date: "2026-11-05", Description: "Public preview", Status: "published"}, Title: "Private title", MeetingPoint: "Private place", StartsAt: time.Date(2026, 11, 5, 5, 0, 0, 0, time.UTC), Timezone: "Asia/Almaty", OrganizerID: owner.ID, Capacity: 2, PlanVersion: 1}, states: map[string]string{owner.ID: "approved"}}
	events := &rideEvents{t: t, repo: repo}
	return New(repo, events, func() time.Time { return time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC) }), repo, events, owner
}
func requireFault(t *testing.T, err error, code string) {
	t.Helper()
	var fault *models.Fault
	if !errors.As(err, &fault) || fault.Code != code {
		t.Fatalf("got %v; want %s", err, code)
	}
}

func TestPreviewAndDiscoveryNeverReturnParticipantDetails(t *testing.T) {
	manager, repo, _, owner := rideFixture(t)
	yes := true
	repo.ride.CanManage = &yes
	repo.ride.ParticipationState = "approved"
	preview, err := manager.Preview(context.Background(), repo.ride.ID)
	if err != nil || preview.CanManage != nil || preview.ParticipationState != "" || preview.Description != "Public preview" {
		t.Fatal("preview leaked caller-specific data")
	}
	guest, err := manager.View(context.Background(), repo.ride.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := guest.(models.RideSummary); !ok {
		t.Fatal("guest received full ride")
	}
	rider := models.User{ID: uuid.NewString()}
	repo.states[rider.ID] = "requested"
	view, err := manager.View(context.Background(), repo.ride.ID, &models.Session{User: rider})
	if err != nil {
		t.Fatal(err)
	}
	summary, ok := view.(models.RideSummary)
	if !ok || summary.ParticipationState != "requested" || summary.CanManage == nil || *summary.CanManage {
		t.Fatal("pending view was not a strict summary")
	}
	repo.states[rider.ID] = "approved"
	view, err = manager.View(context.Background(), repo.ride.ID, &models.Session{User: rider})
	if err != nil {
		t.Fatal(err)
	}
	if full, ok := view.(models.Ride); !ok || full.MeetingPoint != "Private place" {
		t.Fatal("approved participant did not receive plan")
	}
	repo.list = []models.RideListing{{Summary: repo.ride.RideSummary, OrganizerID: owner.ID}}
	listed, err := manager.List(context.Background(), nil)
	if err != nil || repo.listLimit != 100 || repo.listUser != "" || listed[0].CanManage != nil || listed[0].ParticipationState != "" {
		t.Fatal("anonymous discovery boundary changed")
	}
	listed, err = manager.List(context.Background(), &models.Session{User: owner})
	if err != nil || listed[0].CanManage == nil || !*listed[0].CanManage {
		t.Fatal("owner discovery flag missing")
	}
}

func TestCreateOwnsOrganizerApprovalAndInitialPlanState(t *testing.T) {
	manager, repo, _, owner := rideFixture(t)
	// Stored-record projection is returned by the repository, not recomputed by the manager.
	repo.createdDate = "2026-11-06"
	input := models.RideInput{City: " Almaty ", Description: " Preview ", Title: " Plan ", StartsAt: time.Date(2026, 11, 5, 23, 30, 0, 0, time.UTC), MeetingPoint: " Meet here ", Capacity: 1}
	ride, err := manager.Create(context.Background(), owner, input)
	if err != nil {
		t.Fatal(err)
	}
	if ride.Status != "published" || ride.PlanVersion != 1 || repo.createdState != "approved" || repo.states[owner.ID] != "approved" || ride.ParticipationState != "approved" || ride.CanManage == nil || !*ride.CanManage || ride.Date != "2026-11-06" || ride.Timezone != "Asia/Almaty" {
		t.Fatal("initial owner/plan state or mapped creation response changed")
	}
	owner.OnboardingComplete = false
	_, err = manager.Create(context.Background(), owner, input)
	requireFault(t, err, "profile_required")
}

func TestJoinDoesNotReserveCapacityAndApprovalChecksOwnerCount(t *testing.T) {
	manager, repo, events, owner := rideFixture(t)
	repo.ride.Capacity = 1
	rider := models.User{ID: uuid.NewString(), OnboardingComplete: true}
	state, err := manager.Join(context.Background(), rider, repo.ride.ID)
	if err != nil || state != "requested" || repo.countCalls != 0 || repo.states[rider.ID] != "requested" {
		t.Fatal("request unexpectedly reserved or checked capacity")
	}
	_, err = manager.Decide(context.Background(), owner, repo.ride.ID, rider.ID, "approve")
	requireFault(t, err, "ride_full")
	if repo.states[rider.ID] != "requested" || len(events.events) != 1 {
		t.Fatal("failed approval changed participation or published")
	}
	repo.ride.Capacity = 2
	state, err = manager.Decide(context.Background(), owner, repo.ride.ID, rider.ID, "approve")
	if err != nil || state != "approved" || repo.states[rider.ID] != "approved" {
		t.Fatal("available place was not approved")
	}
	before := repo.setCalls
	_, err = manager.Decide(context.Background(), owner, repo.ride.ID, rider.ID, "approve")
	if err != nil || repo.setCalls != before {
		t.Fatal("repeated approval was not idempotent")
	}
}

func TestPlanVersionsCapacityAndCancellationAreCoordinated(t *testing.T) {
	manager, repo, events, owner := rideFixture(t)
	_, err := manager.Update(context.Background(), owner, repo.ride.ID, models.RidePatch{PlanVersion: 0})
	requireFault(t, err, "stale_plan")
	if repo.updateCalls != 0 || len(events.events) != 0 {
		t.Fatal("stale edit wrote or published")
	}
	participant := uuid.NewString()
	repo.states[participant] = "approved"
	capacity := 1
	_, err = manager.Update(context.Background(), owner, repo.ride.ID, models.RidePatch{PlanVersion: 1, Capacity: &capacity})
	requireFault(t, err, "capacity_too_small")
	title := "Updated plan"
	updated, err := manager.Update(context.Background(), owner, repo.ride.ID, models.RidePatch{PlanVersion: 1, Title: &title})
	if err != nil || updated.PlanVersion != 2 || updated.Title != title || updated.Date != "repository-mapped-date" || len(events.events) != 1 {
		t.Fatal("edit did not return committed mapped plan")
	}
	_, err = manager.Cancel(context.Background(), owner, repo.ride.ID, 1)
	requireFault(t, err, "stale_plan")
	cancelled, err := manager.Cancel(context.Background(), owner, repo.ride.ID, 2)
	if err != nil || cancelled.Status != "cancelled" || cancelled.PlanVersion != 3 {
		t.Fatal("cancellation did not version the plan")
	}
	again, err := manager.Cancel(context.Background(), owner, repo.ride.ID, 1)
	if err != nil || again.PlanVersion != 3 {
		t.Fatal("repeated cancellation must ignore stale version without incrementing again")
	}
	_, err = manager.Join(context.Background(), models.User{ID: uuid.NewString(), OnboardingComplete: true}, repo.ride.ID)
	requireFault(t, err, "ride_cancelled")
}

func TestFailedCommitDoesNotPublishOrApplyDecisions(t *testing.T) {
	manager, repo, events, _ := rideFixture(t)
	repo.commitErr = errors.New("commit failed")
	rider := models.User{ID: uuid.NewString(), OnboardingComplete: true}
	_, err := manager.Join(context.Background(), rider, repo.ride.ID)
	if err == nil || repo.states[rider.ID] != "" || len(events.events) != 0 {
		t.Fatal("failed transaction escaped as applied decision")
	}
}

func TestLeaveAndParticipantVisibility(t *testing.T) {
	manager, repo, _, owner := rideFixture(t)
	_, err := manager.Leave(context.Background(), owner, repo.ride.ID)
	requireFault(t, err, "organizer_cannot_leave")
	rider := models.User{ID: uuid.NewString()}
	state, err := manager.Leave(context.Background(), rider, repo.ride.ID)
	if err != nil || state != "left" || repo.setCalls != 0 {
		t.Fatal("leaving absent participation inserted history")
	}
	_, err = manager.Participants(context.Background(), rider, repo.ride.ID)
	requireFault(t, err, "participant_required")
	repo.states[rider.ID] = "approved"
	participants, err := manager.Participants(context.Background(), rider, repo.ride.ID)
	if err != nil || len(participants) != 2 {
		t.Fatal("participant roster visibility changed")
	}
	pending := uuid.NewString()
	repo.states[pending] = "requested"
	participants, err = manager.Participants(context.Background(), rider, repo.ride.ID)
	if err != nil || len(participants) != 2 {
		t.Fatal("participant received pending requests")
	}
	participants, err = manager.Participants(context.Background(), owner, repo.ride.ID)
	if err != nil || len(participants) != 3 {
		t.Fatal("organizer cannot review requests")
	}
}

func TestRideValidationRejectsUnsafeLinksAndBounds(t *testing.T) {
	base := models.RideInput{City: "Almaty", Description: "Preview", Title: "Plan", StartsAt: time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC), MeetingPoint: "Place", Capacity: 1}
	for name, change := range map[string]func(*models.RideInput){
		"javascript link": func(input *models.RideInput) { input.RouteURL = "javascript:alert(1)" },
		"credential link": func(input *models.RideInput) { input.RouteURL = "https://user:pass@example.com" },
		"invalid zone":    func(input *models.RideInput) { input.Timezone = "invalid/timezone" },
		"zero capacity":   func(input *models.RideInput) { input.Capacity = 0 },
		"large capacity":  func(input *models.RideInput) { input.Capacity = 1001 },
	} {
		t.Run(name, func(t *testing.T) {
			input := base
			change(&input)
			_, err := validateInput(input)
			requireFault(t, err, "invalid_ride")
		})
	}
	manager, repo, _, _ := rideFixture(t)
	_, err := manager.Preview(context.Background(), uuid.NewString())
	requireFault(t, err, "not_found")
	if repo.updateCalls != 0 {
		t.Fatal("missing read changed plan")
	}
}

func TestRestrictedActorCannotReadPlanRosterOrRequestHiddenRide(t *testing.T) {
	manager, repo, _, owner := rideFixture(t)
	ctx := context.Background()
	rider := models.User{ID: uuid.NewString(), OnboardingComplete: true}
	repo.states[rider.ID] = "approved"
	repo.statuses = map[string]string{rider.ID: "suspended"}
	_, err := manager.View(ctx, repo.ride.ID, &models.Session{User: rider})
	requireFault(t, err, "account_unavailable")
	_, err = manager.Participants(ctx, rider, repo.ride.ID)
	requireFault(t, err, "account_unavailable")
	delete(repo.statuses, rider.ID)
	repo.hidden = map[string]bool{"ride:" + repo.ride.ID: true}
	if _, err = manager.View(ctx, repo.ride.ID, &models.Session{User: rider}); err != nil {
		t.Fatal("approved coordination lost", err)
	}
	_, err = manager.View(ctx, repo.ride.ID, nil)
	requireFault(t, err, "not_found")
	repo.states[rider.ID] = "removed"
	_, err = manager.Join(ctx, rider, repo.ride.ID)
	requireFault(t, err, "not_found")
	if _, err = manager.View(ctx, repo.ride.ID, &models.Session{User: owner}); err != nil {
		t.Fatal("owner lost hidden ride", err)
	}
}

func TestCannotApproveRestrictedRequestedRider(t *testing.T) {
	m, repo, _, owner := rideFixture(t)
	target := uuid.NewString()
	repo.states[target] = "requested"
	repo.statuses = map[string]string{target: "suspended"}
	_, err := m.Decide(context.Background(), owner, repo.ride.ID, target, "approve")
	requireFault(t, err, "account_unavailable")
	if repo.states[target] != "requested" {
		t.Fatal("restricted rider approved")
	}
}
