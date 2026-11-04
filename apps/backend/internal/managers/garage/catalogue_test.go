package garage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
)

func TestCataloguePagesPublicProjectionWithoutOwnerFields(t *testing.T) {
	m, repo, owner, _ := setupGarage(t)
	for i := range 21 {
		id := fmt.Sprintf("10000000-0000-0000-0000-%012x", i)
		repo.bikes[id] = models.Bike{ID: id, OwnerID: owner.ID, Make: "Honda", Model: "CB400", Year: 2005, Visibility: "public", Version: 7}
	}
	repo.bikes["00000000-0000-0000-0000-000000000001"] = models.Bike{ID: "00000000-0000-0000-0000-000000000001", OwnerID: owner.ID, Make: "PRIVATE", Visibility: "private"}
	page, err := m.Catalogue(context.Background(), "", "")
	if err != nil || len(page.Bikes) != 20 || page.NextCursor != page.Bikes[19].ID {
		t.Fatalf("page=%v err=%v", page, err)
	}
	if repo.publicQuery.Limit != 21 {
		t.Fatal("catalogue must request one bounded lookahead row")
	}
	for _, bike := range page.Bikes {
		encoded, _ := json.Marshal(bike)
		var fields map[string]any
		_ = json.Unmarshal(encoded, &fields)
		if len(fields) != 5 || fields["owner_id"] != nil || fields["visibility"] != nil || fields["can_manage"] != nil {
			t.Fatalf("catalogue leaked private fields: %s", encoded)
		}
	}
	tail, err := m.Catalogue(context.Background(), "", page.NextCursor)
	if err != nil || len(tail.Bikes) != 1 || tail.NextCursor != "" || tail.Bikes[0].ID <= page.NextCursor {
		t.Fatalf("tail=%v err=%v", tail, err)
	}
}

func TestCatalogueTrimsLiteralSearchAndReturnsEmptyArrays(t *testing.T) {
	m, repo, owner, _ := setupGarage(t)
	bike := models.Bike{ID: uuid.NewString(), OwnerID: owner.ID, Make: "Yamaha", Model: "MT-%_07", Year: 2020, Visibility: "public"}
	repo.bikes[bike.ID] = bike
	page, err := m.Catalogue(context.Background(), "  YAMAHA mt-%_  ", "")
	if err != nil || len(page.Bikes) != 1 || page.Bikes[0].ID != bike.ID || repo.publicQuery.Search != "YAMAHA mt-%_" {
		t.Fatalf("page=%v query=%v err=%v", page, repo.publicQuery, err)
	}
	page, err = m.Catalogue(context.Background(), "not a match", "")
	if err != nil || page.Bikes == nil || len(page.Bikes) != 0 || page.NextCursor != "" {
		t.Fatalf("empty page=%v err=%v", page, err)
	}
}

func TestCatalogueRejectsInvalidQueryAndNoncanonicalCursorsBeforePersistence(t *testing.T) {
	m, repo, _, _ := setupGarage(t)
	_, err := m.Catalogue(context.Background(), strings.Repeat("x", 201), "")
	faultCode(t, err, "invalid_query")
	for _, cursor := range []string{"not-a-cursor", strings.Repeat("x", 257), strings.ToUpper("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"), "urn:uuid:aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"} {
		_, err = m.Catalogue(context.Background(), "", cursor)
		faultCode(t, err, "invalid_cursor")
	}
	if repo.publicCalls != 0 {
		t.Fatal("invalid catalogue input reached persistence")
	}
}
