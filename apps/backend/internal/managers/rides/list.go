package rides

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
)

const ridePageSize = 20

// This includes worst-case JSON escaping of the bounded city/search strings,
// base64 expansion, and the SHA-256 signature.
const rideCursorLimit = 4096

type listCursor struct {
	View, City, From, To, Search, User, Date, ID string
	Now                                          time.Time
}

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
	invalid := models.Fail("invalid_cursor", "Invalid ride page cursor; restart this view")
	if len(token) > rideCursorLimit {
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
	if err := json.Unmarshal(data, &cursor); err != nil {
		return cursor, invalid
	}
	return cursor, nil
}

func validListDate(value string) bool {
	parsed, err := time.Parse("2006-01-02", value)
	return err == nil && parsed.Format("2006-01-02") == value
}

func (m *Manager) List(ctx context.Context, session *models.Session, input models.RideListInput) (models.RidePage, error) {
	page := models.RidePage{Rides: []models.RideSummary{}}
	if input.View == "" {
		input.View = "discover"
	}
	switch input.View {
	case "discover", "organizing", "joined", "pending", "past":
	default:
		return page, models.Fail("invalid_query", "Choose a valid ride view")
	}
	if input.View != "discover" && session == nil {
		return page, models.Fail("authentication_required", "Sign in to view your rides")
	}
	input.City = models.NormalizeCity(input.City)
	input.Search = strings.TrimSpace(input.Search)
	if len(input.City) > 160 || len(input.Search) > 200 || (input.From != "" && !validListDate(input.From)) ||
		(input.To != "" && !validListDate(input.To)) || (input.From != "" && input.To != "" && input.From > input.To) {
		return page, models.Fail("invalid_query", "Check city and ride date filters")
	}
	query := database.RideListQuery{
		CityAliases: models.CityAliases(input.City), FromDate: input.From, ToDate: input.To, Search: input.Search,
		Now: m.now().UTC(), Limit: ridePageSize + 1,
	}
	if session != nil {
		query.UserID = session.User.ID
	}
	switch input.View {
	case "organizing":
		query.OrganizerOnly = true
	case "joined", "pending":
		query.ExcludeOrganizer = true
		query.ParticipationState = "approved"
		if input.View == "pending" {
			query.ParticipationState = "requested"
		}
	case "past":
		query.RelatedOnly, query.Past = true, true
	}
	bound := listCursor{View: input.View, City: input.City, From: input.From, To: input.To, Search: input.Search, User: query.UserID}
	if input.Cursor != "" {
		invalid := models.Fail("invalid_cursor", "Invalid ride page cursor; restart this view")
		cursor, err := m.decodeListCursor(input.Cursor)
		if err != nil {
			return page, invalid
		}
		id, err := uuid.Parse(cursor.ID)
		if err != nil || cursor.ID != id.String() || !validListDate(cursor.Date) || cursor.Now.IsZero() || cursor.Now.After(query.Now) ||
			cursor.View != bound.View || cursor.City != bound.City || cursor.From != bound.From || cursor.To != bound.To || cursor.Search != bound.Search || cursor.User != bound.User {
			return page, invalid
		}
		query.AfterDate, query.AfterID, query.Now = cursor.Date, cursor.ID, cursor.Now
	}
	rows, err := m.repo.ListRides(ctx, query)
	if err != nil {
		return page, err
	}
	if len(rows) > ridePageSize {
		rows = rows[:ridePageSize]
		last := rows[len(rows)-1].Summary
		bound.Date, bound.ID, bound.Now = last.Date, last.ID, query.Now
		page.NextCursor, err = m.encodeListCursor(bound)
		if err != nil {
			return page, err
		}
	}
	for _, row := range rows {
		summary := publicSummary(row.Summary)
		if session != nil {
			canManage := row.OrganizerID == query.UserID
			summary.CanManage = &canManage
			summary.ParticipationState = row.Summary.ParticipationState
		}
		page.Rides = append(page.Rides, summary)
	}
	return page, nil
}
