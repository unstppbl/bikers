package clubs

import (
	"context"
	"errors"
	"testing"

	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
)

func TestMembershipNotificationsFollowOnlyCommittedTransitions(t *testing.T) {
	m, repo, owner, member := clubFixture()
	ctx := context.Background()
	assertNotice := func(index int, kind, recipient string) {
		t.Helper()
		if len(repo.notices) != index+1 {
			t.Fatalf("want %d notices, got %d", index+1, len(repo.notices))
		}
		n := repo.notices[index]
		if n.Kind != kind || n.ClubID != repo.club.ID || n.RideID != "" || len(n.Recipients) != 1 || n.Recipients[0] != recipient || !n.CreatedAt.Equal(m.now().UTC()) {
			t.Fatalf("incorrect club notice: %+v", n)
		}
		if _, err := uuid.Parse(n.EventID); err != nil {
			t.Fatal("notice event has no valid identity", err)
		}
	}
	for range 2 {
		if _, err := m.Join(ctx, member, repo.club.ID); err != nil {
			t.Fatal(err)
		}
	}
	assertNotice(0, "club.membership.requested", owner.ID)
	for range 2 {
		if _, err := m.Decide(ctx, owner, repo.club.ID, member.ID, "reject"); err != nil {
			t.Fatal(err)
		}
	}
	assertNotice(1, "club.membership.rejected", member.ID)
	if _, err := m.Join(ctx, member, repo.club.ID); err != nil {
		t.Fatal(err)
	}
	assertNotice(2, "club.membership.requested", owner.ID)
	for range 2 {
		if _, err := m.Decide(ctx, owner, repo.club.ID, member.ID, "approve"); err != nil {
			t.Fatal(err)
		}
	}
	assertNotice(3, "club.membership.approved", member.ID)
	if _, err := m.Join(ctx, member, repo.club.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Join(ctx, owner, repo.club.ID); err != nil {
		t.Fatal(err)
	}
	if len(repo.notices) != 4 {
		t.Fatal("an already-approved member or owner notified themselves")
	}
	for range 2 {
		if _, err := m.Decide(ctx, owner, repo.club.ID, member.ID, "remove"); err != nil {
			t.Fatal(err)
		}
	}
	assertNotice(4, "club.membership.removed", member.ID)
	seen := map[string]bool{}
	for _, n := range repo.notices {
		if seen[n.EventID] {
			t.Fatal("distinct transitions reused the same event identity")
		}
		seen[n.EventID] = true
	}
}

func TestFailedMembershipNotificationRollsBackMembership(t *testing.T) {
	m, repo, owner, member := clubFixture()
	ctx := context.Background()
	repo.notifyErr = errors.New("inbox write failed")
	if _, err := m.Join(ctx, member, repo.club.ID); err == nil || repo.members[member.ID].State != "" || len(repo.notices) != 0 {
		t.Fatal("failed request notice committed membership or inbox state")
	}
	repo.notifyErr = nil
	if _, err := m.Join(ctx, member, repo.club.ID); err != nil {
		t.Fatal(err)
	}
	repo.notifyErr = errors.New("decision notice write failed")
	if _, err := m.Decide(ctx, owner, repo.club.ID, member.ID, "approve"); err == nil || repo.members[member.ID].State != "requested" || len(repo.notices) != 1 {
		t.Fatal("failed decision notice committed membership or inbox state")
	}
	repo.notifyErr = nil
	repo.commitErr = errors.New("transaction commit failed")
	if _, err := m.Decide(ctx, owner, repo.club.ID, member.ID, "approve"); err == nil || repo.members[member.ID].State != "requested" || len(repo.notices) != 1 {
		t.Fatal("failed transaction committed a notification")
	}
}

func TestMembershipRequestsNotifyTheCurrentOwner(t *testing.T) {
	m, repo, owner, member := clubFixture()
	repo.members[member.ID] = models.ClubMembership{State: "approved", AccountStatus: "active"}
	if _, err := m.Transfer(context.Background(), owner, repo.club.ID, models.ClubTransfer{Version: 1, UserID: member.ID}); err != nil {
		t.Fatal(err)
	}
	applicant := models.User{ID: uuid.NewString(), OnboardingComplete: true}
	repo.members[applicant.ID] = models.ClubMembership{AccountStatus: "active"}
	if _, err := m.Join(context.Background(), applicant, repo.club.ID); err != nil {
		t.Fatal(err)
	}
	if len(repo.notices) != 1 || repo.notices[0].Recipients[0] != member.ID {
		t.Fatal("request notified the previous owner")
	}
}
