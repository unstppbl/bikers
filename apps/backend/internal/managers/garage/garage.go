// Package garage owns bike visibility, ownership and private maintenance rules.
package garage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/managers/photos"
	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
)

const pageSize = 50
const lookupLimit = 20
const cursorByteLimit = 256

type Manager struct {
	repo   database.GarageRepository
	now    func() time.Time
	photos *photos.Manager
}

func New(repo database.GarageRepository, now func() time.Time, processor *photos.Manager) *Manager {
	if now == nil {
		now = time.Now
	}
	return &Manager{repo: repo, now: now, photos: processor}
}
func notFound() error    { return models.Fail("not_found", "Bike or maintenance entry not found") }
func invalidBike() error { return models.Fail("invalid_bike", "Check motorcycle fields") }
func invalidMaintenance() error {
	return models.Fail("invalid_maintenance", "Check maintenance date, mileage and work performed")
}
func invalidCursor() error { return models.Fail("invalid_cursor", "Invalid page cursor") }
func staleVersion() error {
	return models.Fail("stale_version", "This record changed; refresh and retry")
}
func storageError(err error) error {
	if errors.Is(err, database.ErrNotFound) {
		return notFound()
	}
	if errors.Is(err, database.ErrConflict) {
		return staleVersion()
	}
	return err
}
func canonicalID(raw string) (string, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return "", notFound()
	}
	return id.String(), nil
}
func ownerView(bike models.Bike) models.Bike { bike.CanManage = true; return bike }
func approvedOwner(owner models.GarageOwner, actor models.User) bool {
	return owner.ID == actor.ID && owner.Status == "active"
}
func ownBike(tx database.BikeTransaction, actor models.User) error {
	if tx.Bike().OwnerID != actor.ID || !approvedOwner(tx.Owner(), actor) {
		return notFound()
	}
	return nil
}
func today(now time.Time) string {
	location, _ := time.LoadLocation("Asia/Almaty")
	return now.In(location).Format("2006-01-02")
}

func (m *Manager) SearchModels(query string) ([]models.BikeModel, error) {
	if len(query) > 200 {
		return nil, models.Fail("invalid_query", "Search query is too long")
	}
	matches := models.MatchBikeModels(query)
	if len(matches) > lookupLimit {
		matches = matches[:lookupLimit]
	}
	return matches, nil
}

func (m *Manager) List(ctx context.Context, actor models.User, cursor string) (models.BikePage, error) {
	after := ""
	if cursor != "" {
		if len(cursor) > cursorByteLimit {
			return models.BikePage{}, invalidCursor()
		}
		id, err := uuid.Parse(cursor)
		if err != nil {
			return models.BikePage{}, invalidCursor()
		}
		after = id.String()
	}
	bikes, err := m.repo.Bikes(ctx, actor.ID, after, pageSize+1)
	if err != nil {
		return models.BikePage{}, storageError(err)
	}
	page := models.BikePage{Bikes: []models.Bike{}}
	if len(bikes) > pageSize {
		bikes = bikes[:pageSize]
		page.NextCursor = bikes[len(bikes)-1].ID
	}
	for _, bike := range bikes {
		page.Bikes = append(page.Bikes, ownerView(bike))
	}
	return page, nil
}
func (m *Manager) View(ctx context.Context, id string, session *models.Session) (any, error) {
	id, err := canonicalID(id)
	if err != nil {
		return nil, err
	}
	var out any
	err = m.repo.WithBike(ctx, id, func(tx database.BikeTransaction) error {
		bike := tx.Bike()
		if session != nil && session.User.ID == bike.OwnerID {
			if err := ownBike(tx, session.User); err != nil {
				return err
			}
			out = ownerView(bike)
			return nil
		}
		hidden, err := tx.ContentHidden("bike", id)
		if err != nil {
			return err
		}
		if hidden || bike.Visibility != "public" {
			return notFound()
		}
		out = models.PublicBike{ID: bike.ID, Make: bike.Make, Model: bike.Model, ModelID: bike.ModelID, Year: bike.Year}
		return nil
	})
	return out, storageError(err)
}
func (m *Manager) validateBike(input models.BikeInput) (models.BikeInput, error) {
	input.Make = strings.TrimSpace(input.Make)
	input.Model = strings.TrimSpace(input.Model)
	input.ModelID = strings.TrimSpace(input.ModelID)
	if input.ModelID != "" {
		model, ok := models.FindBikeModel(input.ModelID)
		if !ok {
			return input, invalidBike()
		}
		input.Make = model.Make
		input.Model = model.Model
	}
	location, _ := time.LoadLocation("Asia/Almaty")
	if input.Make == "" || len(input.Make) > 100 || input.Model == "" || len(input.Model) > 100 || input.Year < 1885 || input.Year > m.now().In(location).Year()+1 || (input.Visibility != "private" && input.Visibility != "public") {
		return input, invalidBike()
	}
	return input, nil
}
func (m *Manager) Create(ctx context.Context, actor models.User, input models.BikeInput) (models.Bike, error) {
	if !actor.OnboardingComplete {
		return models.Bike{}, models.Fail("profile_required", "Complete your rider profile first")
	}
	if input.Visibility == "" {
		input.Visibility = "private"
	}
	input, err := m.validateBike(input)
	if err != nil {
		return models.Bike{}, err
	}
	now := m.now().UTC()
	bike := models.Bike{ID: uuid.NewString(), OwnerID: actor.ID, Make: input.Make, Model: input.Model, ModelID: input.ModelID, Year: input.Year, Visibility: input.Visibility, Version: 1, CreatedAt: now, UpdatedAt: now}
	var stored models.Bike
	err = m.repo.WithGarageOwner(ctx, actor.ID, func(tx database.GarageOwnerTransaction) error {
		if !approvedOwner(tx.Owner(), actor) {
			return models.Fail("account_unavailable", "Account unavailable")
		}
		var err error
		stored, err = tx.CreateBike(bike)
		return err
	})
	if err != nil {
		return models.Bike{}, storageError(err)
	}
	return ownerView(stored), nil
}
func (m *Manager) Update(ctx context.Context, actor models.User, id string, patch models.BikePatch) (models.Bike, error) {
	id, err := canonicalID(id)
	if err != nil {
		return models.Bike{}, err
	}
	var stored models.Bike
	err = m.repo.WithBike(ctx, id, func(tx database.BikeTransaction) error {
		if err := ownBike(tx, actor); err != nil {
			return err
		}
		bike := tx.Bike()
		if patch.Version != bike.Version {
			return staleVersion()
		}
		input := models.BikeInput{Make: bike.Make, Model: bike.Model, ModelID: bike.ModelID, Year: bike.Year, Visibility: bike.Visibility}
		if patch.Make != nil {
			input.Make = *patch.Make
		}
		if patch.Model != nil {
			input.Model = *patch.Model
		}
		if patch.ModelID != nil {
			input.ModelID = *patch.ModelID
		}
		if patch.Year != nil {
			input.Year = *patch.Year
		}
		if patch.Visibility != nil {
			input.Visibility = *patch.Visibility
		}
		input, err := m.validateBike(input)
		if err != nil {
			return err
		}
		bike.Make, bike.Model, bike.ModelID, bike.Year, bike.Visibility = input.Make, input.Model, input.ModelID, input.Year, input.Visibility
		bike.Version++
		bike.UpdatedAt = m.now().UTC()
		stored, err = tx.UpdateBike(bike)
		return err
	})
	if err != nil {
		return models.Bike{}, storageError(err)
	}
	return ownerView(stored), nil
}
func (m *Manager) Delete(ctx context.Context, actor models.User, id string, version int64) error {
	id, err := canonicalID(id)
	if err != nil {
		return err
	}
	return storageError(m.repo.WithPhotoBike(ctx, id, func(tx database.PhotoTransaction) error {
		if err := ownBike(tx, actor); err != nil {
			return err
		}
		if version != tx.Bike().Version {
			return staleVersion()
		}
		return tx.DeleteBike()
	}))
}

func decodeMaintenanceCursor(raw string) (models.MaintenanceCursor, error) {
	if raw == "" {
		return models.MaintenanceCursor{}, nil
	}
	if len(raw) > cursorByteLimit {
		return models.MaintenanceCursor{}, invalidCursor()
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return models.MaintenanceCursor{}, invalidCursor()
	}
	var cursor models.MaintenanceCursor
	if err = json.Unmarshal(decoded, &cursor); err != nil {
		return cursor, invalidCursor()
	}
	if _, err = models.ParseMaintenanceDate(cursor.Date); err != nil {
		return cursor, invalidCursor()
	}
	id, err := uuid.Parse(cursor.ID)
	if err != nil {
		return cursor, invalidCursor()
	}
	cursor.ID = id.String()
	return cursor, nil
}
func encodeMaintenanceCursor(entry models.MaintenanceEntry) string {
	encoded, _ := json.Marshal(models.MaintenanceCursor{Date: entry.ServiceDate, ID: entry.ID})
	return base64.RawURLEncoding.EncodeToString(encoded)
}
func (m *Manager) Maintenance(ctx context.Context, actor models.User, bikeID, cursor string) (models.MaintenancePage, error) {
	bikeID, err := canonicalID(bikeID)
	if err != nil {
		return models.MaintenancePage{}, err
	}
	var page models.MaintenancePage
	page.Entries = []models.MaintenanceEntry{}
	err = m.repo.WithBike(ctx, bikeID, func(tx database.BikeTransaction) error {
		if err := ownBike(tx, actor); err != nil {
			return err
		}
		after, err := decodeMaintenanceCursor(cursor)
		if err != nil {
			return err
		}
		entries, err := tx.Maintenance(after, pageSize+1)
		if err != nil {
			return err
		}
		if len(entries) > pageSize {
			entries = entries[:pageSize]
			page.NextCursor = encodeMaintenanceCursor(entries[len(entries)-1])
		}
		page.Entries = append(page.Entries, entries...)
		return nil
	})
	if err != nil {
		return models.MaintenancePage{}, storageError(err)
	}
	return page, nil
}
func (m *Manager) validateMaintenance(date string, mileage *int64, work string) (models.MaintenanceInput, error) {
	work = strings.TrimSpace(work)
	_, err := models.ParseMaintenanceDate(date)
	if err != nil || date > today(m.now()) || mileage == nil || *mileage < 0 || *mileage > 10000000 || work == "" || len(work) > 4000 {
		return models.MaintenanceInput{}, invalidMaintenance()
	}
	return models.MaintenanceInput{ServiceDate: date, MileageKM: mileage, WorkPerformed: work}, nil
}
func (m *Manager) CreateMaintenance(ctx context.Context, actor models.User, bikeID string, input models.MaintenanceInput) (models.MaintenanceEntry, error) {
	bikeID, err := canonicalID(bikeID)
	if err != nil {
		return models.MaintenanceEntry{}, err
	}
	var stored models.MaintenanceEntry
	err = m.repo.WithBike(ctx, bikeID, func(tx database.BikeTransaction) error {
		if err := ownBike(tx, actor); err != nil {
			return err
		}
		input, err := m.validateMaintenance(input.ServiceDate, input.MileageKM, input.WorkPerformed)
		if err != nil {
			return err
		}
		now := m.now().UTC()
		stored, err = tx.CreateEntry(models.MaintenanceEntry{ID: uuid.NewString(), BikeID: bikeID, ServiceDate: input.ServiceDate, MileageKM: *input.MileageKM, WorkPerformed: input.WorkPerformed, Version: 1, CreatedAt: now, UpdatedAt: now})
		return err
	})
	if err != nil {
		return models.MaintenanceEntry{}, storageError(err)
	}
	return stored, nil
}
func (m *Manager) UpdateMaintenance(ctx context.Context, actor models.User, bikeID, entryID string, patch models.MaintenancePatch) (models.MaintenanceEntry, error) {
	bikeID, err := canonicalID(bikeID)
	if err != nil {
		return models.MaintenanceEntry{}, err
	}
	entryID, err = canonicalID(entryID)
	if err != nil {
		return models.MaintenanceEntry{}, err
	}
	var stored models.MaintenanceEntry
	err = m.repo.WithBike(ctx, bikeID, func(tx database.BikeTransaction) error {
		if err := ownBike(tx, actor); err != nil {
			return err
		}
		entry, err := tx.Entry(entryID)
		if err != nil {
			return err
		}
		if entry.Version != patch.Version {
			return staleVersion()
		}
		date, mileage, work := entry.ServiceDate, entry.MileageKM, entry.WorkPerformed
		if patch.ServiceDate != nil {
			date = *patch.ServiceDate
		}
		if patch.MileageKM != nil {
			mileage = *patch.MileageKM
		}
		if patch.WorkPerformed != nil {
			work = *patch.WorkPerformed
		}
		validated, err := m.validateMaintenance(date, &mileage, work)
		if err != nil {
			return err
		}
		entry.ServiceDate = validated.ServiceDate
		entry.MileageKM = *validated.MileageKM
		entry.WorkPerformed = validated.WorkPerformed
		entry.Version++
		entry.UpdatedAt = m.now().UTC()
		stored, err = tx.UpdateEntry(entry)
		return err
	})
	if err != nil {
		return models.MaintenanceEntry{}, storageError(err)
	}
	return stored, nil
}
func (m *Manager) DeleteMaintenance(ctx context.Context, actor models.User, bikeID, entryID string, version int64) error {
	bikeID, err := canonicalID(bikeID)
	if err != nil {
		return err
	}
	entryID, err = canonicalID(entryID)
	if err != nil {
		return err
	}
	return storageError(m.repo.WithBike(ctx, bikeID, func(tx database.BikeTransaction) error {
		if err := ownBike(tx, actor); err != nil {
			return err
		}
		entry, err := tx.Entry(entryID)
		if err != nil {
			return err
		}
		if entry.Version != version {
			return staleVersion()
		}
		return tx.DeleteEntry(entryID)
	}))
}
