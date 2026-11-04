package clubs

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"

	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
)

const clubPageSize = 20
const clubCursorLimit = 4096

type listCursor struct{ View, City, Search, User, ID string }

func (m *Manager) encodeListCursor(cursor listCursor) (string, error) {
	data, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, m.cursorKey[:])
	_, _ = mac.Write(data)
	return base64.RawURLEncoding.EncodeToString(data) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
func (m *Manager) decodeListCursor(token string) (listCursor, error) {
	var cursor listCursor
	invalid := models.Fail("invalid_cursor", "Invalid club page cursor; restart this view")
	if len(token) > clubCursorLimit {
		return cursor, invalid
	}
	payload, signature, ok := strings.Cut(token, ".")
	if !ok || payload == "" || signature == "" {
		return cursor, invalid
	}
	data, err := base64.RawURLEncoding.Strict().DecodeString(payload)
	if err != nil {
		return cursor, invalid
	}
	provided, err := base64.RawURLEncoding.Strict().DecodeString(signature)
	if err != nil || len(provided) != sha256.Size {
		return cursor, invalid
	}
	mac := hmac.New(sha256.New, m.cursorKey[:])
	_, _ = mac.Write(data)
	if !hmac.Equal(provided, mac.Sum(nil)) {
		return cursor, invalid
	}
	if err = json.Unmarshal(data, &cursor); err != nil {
		return cursor, invalid
	}
	return cursor, nil
}

func (m *Manager) List(ctx context.Context, session *models.Session, input models.ClubListInput) (models.ClubPage, error) {
	page := models.ClubPage{Clubs: []models.ClubSummary{}}
	if input.View == "" {
		input.View = "discover"
	}
	switch input.View {
	case "discover", "organizing", "joined", "pending":
	default:
		return page, models.Fail("invalid_query", "Choose a valid club view")
	}
	if input.View != "discover" && session == nil {
		return page, models.Fail("authentication_required", "Sign in to view your clubs")
	}
	input.City = models.NormalizeCity(input.City)
	input.Search = strings.TrimSpace(input.Search)
	if len(input.City) > 160 || len(input.Search) > 200 {
		return page, models.Fail("invalid_query", "Check club city and name filters")
	}
	query := database.ClubListQuery{CityAliases: models.CityAliases(input.City), Search: input.Search, Limit: clubPageSize + 1}
	if session != nil {
		query.UserID = session.User.ID
	}
	switch input.View {
	case "discover":
		query.ActiveOnly = true
	case "organizing":
		query.OwnerOnly = true
	case "joined":
		query.ExcludeOwner = true
		query.MembershipState = "approved"
	case "pending":
		query.ExcludeOwner = true
		query.MembershipState = "requested"
		query.ActiveOnly = true
	}
	bound := listCursor{View: input.View, City: input.City, Search: input.Search, User: query.UserID}
	if input.Cursor != "" {
		cursor, err := m.decodeListCursor(input.Cursor)
		if err != nil {
			return page, err
		}
		id, err := uuid.Parse(cursor.ID)
		if err != nil || cursor.ID != id.String() || cursor.View != bound.View || cursor.City != bound.City || cursor.Search != bound.Search || cursor.User != bound.User {
			return page, models.Fail("invalid_cursor", "Invalid club page cursor; restart this view")
		}
		query.AfterID = cursor.ID
	}
	rows, err := m.repo.ListClubs(ctx, query)
	if err != nil {
		return page, err
	}
	if len(rows) > clubPageSize {
		rows = rows[:clubPageSize]
		bound.ID = rows[len(rows)-1].Club.ID
		page.NextCursor, err = m.encodeListCursor(bound)
		if err != nil {
			return page, err
		}
	}
	for _, row := range rows {
		page.Clubs = append(page.Clubs, summary(row.Club, query.UserID, row.MembershipState))
	}
	return page, nil
}
