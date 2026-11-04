package clubs

import (
	"context"
	"strings"
	"testing"

	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
)

func TestClubListViewsAndFilterValidation(t *testing.T) {
	manager, repo, owner, _ := clubFixture()
	session := &models.Session{User: owner}
	for _, view := range []string{"organizing", "joined", "pending"} {
		_, err := manager.List(context.Background(), nil, models.ClubListInput{View: view})
		requireFault(t, err, "authentication_required")
	}
	for _, input := range []models.ClubListInput{{View: "unknown"}, {City: strings.Repeat("x", 161)}, {Search: strings.Repeat("x", 201)}} {
		_, err := manager.List(context.Background(), session, input)
		requireFault(t, err, "invalid_query")
	}
	for _, view := range []string{"discover", "organizing", "joined", "pending"} {
		_, err := manager.List(context.Background(), session, models.ClubListInput{View: view, City: " Алматы ", Search: " Touring "})
		if err != nil {
			t.Fatal(err)
		}
		query := repo.listQuery
		if query.UserID != owner.ID || query.Search != "Touring" || len(query.CityAliases) < 2 || query.Limit != 21 {
			t.Fatalf("filters were not normalized: %+v", query)
		}
		if query.OwnerOnly != (view == "organizing") || query.ExcludeOwner != (view == "joined" || view == "pending") || query.ActiveOnly != (view == "discover" || view == "pending") {
			t.Fatalf("incorrect view policy %s: %+v", view, query)
		}
		expected := ""
		if view == "joined" {
			expected = "approved"
		}
		if view == "pending" {
			expected = "requested"
		}
		if query.MembershipState != expected {
			t.Fatalf("incorrect state policy %s: %+v", view, query)
		}
	}
}

func TestClubListCursorsBindFiltersActorAndManagerInstance(t *testing.T) {
	manager, repo, owner, other := clubFixture()
	for i := 0; i < 21; i++ {
		club := repo.club
		club.ID = uuid.NewString()
		repo.list = append(repo.list, models.ClubListing{Club: club, MembershipState: "requested"})
	}
	input := models.ClubListInput{View: "pending", City: "Almaty", Search: "Touring"}
	session := &models.Session{User: owner}
	page, err := manager.List(context.Background(), session, input)
	if err != nil || len(page.Clubs) != 20 || page.NextCursor == "" {
		t.Fatalf("invalid first page %+v %v", page, err)
	}
	input.Cursor = page.NextCursor
	repo.list = nil
	_, err = manager.List(context.Background(), session, input)
	if err != nil || repo.listQuery.AfterID != page.Clubs[19].ID {
		t.Fatalf("cursor did not continue at last record: %v", err)
	}
	for _, changed := range []models.ClubListInput{{View: "joined", City: input.City, Search: input.Search, Cursor: input.Cursor}, {View: input.View, City: "Astana", Search: input.Search, Cursor: input.Cursor}, {View: input.View, City: input.City, Search: "other", Cursor: input.Cursor}} {
		_, err = manager.List(context.Background(), session, changed)
		requireFault(t, err, "invalid_cursor")
	}
	_, err = manager.List(context.Background(), &models.Session{User: other}, input)
	requireFault(t, err, "invalid_cursor")
	another, _, _, _ := clubFixture()
	_, err = another.List(context.Background(), session, input)
	requireFault(t, err, "invalid_cursor")
	for _, token := range []string{"broken", input.Cursor + "x", strings.Repeat("x", clubCursorLimit+1)} {
		input.Cursor = token
		_, err = manager.List(context.Background(), session, input)
		requireFault(t, err, "invalid_cursor")
	}
}

func TestClubGuestListsNeverExposeMembershipOrOwnerFields(t *testing.T) {
	manager, repo, _, _ := clubFixture()
	repo.list = []models.ClubListing{{Club: repo.club, MembershipState: "approved"}}
	page, err := manager.List(context.Background(), nil, models.ClubListInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Clubs) != 1 || page.Clubs[0].CanManage != nil || page.Clubs[0].MembershipState != "" {
		t.Fatalf("private fields leaked: %+v", page)
	}
}
