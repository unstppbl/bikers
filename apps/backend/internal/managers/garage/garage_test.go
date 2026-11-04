package garage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/managers/photos"
	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
)

type memoryGarage struct {
	mu          sync.Mutex
	owners      map[string]models.GarageOwner
	bikes       map[string]models.Bike
	entries     map[string]models.MaintenanceEntry
	writes      int
	photoState  *memoryPhotoState
	publicCalls int
	publicQuery database.PublicBikeQuery
}

func (r *memoryGarage) PublicBikes(_ context.Context, query database.PublicBikeQuery) ([]models.Bike, error) {
	r.publicCalls++
	r.publicQuery = query
	out := []models.Bike{}
	for _, bike := range r.bikes {
		if bike.Visibility == "public" && bike.ID > query.AfterID && strings.Contains(strings.ToLower(bike.Make+" "+bike.Model), strings.ToLower(query.Search)) {
			out = append(out, bike)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if len(out) > query.Limit {
		out = out[:query.Limit]
	}
	return out, nil
}

func newMemoryGarage() *memoryGarage {
	return &memoryGarage{owners: map[string]models.GarageOwner{}, bikes: map[string]models.Bike{}, entries: map[string]models.MaintenanceEntry{}}
}
func (r *memoryGarage) Bikes(_ context.Context, owner, after string, limit int) ([]models.Bike, error) {
	out := []models.Bike{}
	for _, bike := range r.bikes {
		if bike.OwnerID == owner && bike.ID > after {
			out = append(out, bike)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
func (r *memoryGarage) Bike(_ context.Context, id string) (models.Bike, error) {
	bike, ok := r.bikes[id]
	if !ok {
		return models.Bike{}, database.ErrNotFound
	}
	return bike, nil
}
func (r *memoryGarage) WithGarageOwner(_ context.Context, id string, decide database.GarageOwnerDecision) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	owner, ok := r.owners[id]
	if !ok {
		return database.ErrNotFound
	}
	before := maps.Clone(r.bikes)
	err := decide(&memoryOwner{repo: r, owner: owner})
	if err != nil {
		r.bikes = before
	}
	return err
}
func (r *memoryGarage) WithBike(_ context.Context, id string, decide database.BikeDecision) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	bike, ok := r.bikes[id]
	if !ok {
		return database.ErrNotFound
	}
	beforeBikes, beforeEntries := maps.Clone(r.bikes), maps.Clone(r.entries)
	err := decide(&memoryBike{memoryOwner: memoryOwner{repo: r, owner: r.owners[bike.OwnerID]}, bike: bike})
	if err != nil {
		r.bikes, r.entries = beforeBikes, beforeEntries
	}
	return err
}

type memoryOwner struct {
	repo  *memoryGarage
	owner models.GarageOwner
}

func (t *memoryOwner) Owner() models.GarageOwner { return t.owner }
func (t *memoryOwner) CreateBike(bike models.Bike) (models.Bike, error) {
	t.repo.writes++
	t.repo.bikes[bike.ID] = bike
	return bike, nil
}

type memoryBike struct {
	memoryOwner
	bike models.Bike
}

func (t *memoryBike) Bike() models.Bike { return t.bike }
func (t *memoryBike) UpdateBike(bike models.Bike) (models.Bike, error) {
	t.repo.writes++
	t.bike = bike
	t.repo.bikes[bike.ID] = bike
	return bike, nil
}
func (t *memoryBike) DeleteBike() error {
	t.repo.writes++
	delete(t.repo.bikes, t.bike.ID)
	for id, e := range t.repo.entries {
		if e.BikeID == t.bike.ID {
			delete(t.repo.entries, id)
		}
	}
	return nil
}
func (t *memoryBike) Maintenance(after models.MaintenanceCursor, limit int) ([]models.MaintenanceEntry, error) {
	out := []models.MaintenanceEntry{}
	for _, e := range t.repo.entries {
		if e.BikeID != t.bike.ID {
			continue
		}
		if after.ID != "" && (e.ServiceDate > after.Date || (e.ServiceDate == after.Date && e.ID >= after.ID)) {
			continue
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ServiceDate == out[j].ServiceDate {
			return out[i].ID > out[j].ID
		}
		return out[i].ServiceDate > out[j].ServiceDate
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
func (t *memoryBike) Entry(id string) (models.MaintenanceEntry, error) {
	e, ok := t.repo.entries[id]
	if !ok || e.BikeID != t.bike.ID {
		return models.MaintenanceEntry{}, database.ErrNotFound
	}
	return e, nil
}
func (t *memoryBike) CreateEntry(e models.MaintenanceEntry) (models.MaintenanceEntry, error) {
	t.repo.writes++
	t.repo.entries[e.ID] = e
	return e, nil
}
func (t *memoryBike) UpdateEntry(e models.MaintenanceEntry) (models.MaintenanceEntry, error) {
	t.repo.writes++
	t.repo.entries[e.ID] = e
	return e, nil
}
func (t *memoryBike) DeleteEntry(id string) error {
	t.repo.writes++
	delete(t.repo.entries, id)
	return nil
}

func setupGarage(t *testing.T) (*Manager, *memoryGarage, models.User, models.User) {
	t.Helper()
	r := newMemoryGarage()
	owner := models.User{ID: uuid.NewString(), OnboardingComplete: true}
	other := models.User{ID: uuid.NewString(), OnboardingComplete: true}
	for _, u := range []models.User{owner, other} {
		r.owners[u.ID] = models.GarageOwner{ID: u.ID, Status: "active"}
	}
	now := time.Date(2026, 11, 3, 20, 0, 0, 0, time.UTC)
	return New(r, func() time.Time { return now }, photos.New(nil)), r, owner, other
}
func faultCode(t *testing.T, err error, want string) {
	t.Helper()
	var fault *models.Fault
	if !errors.As(err, &fault) || fault.Code != want {
		t.Fatalf("fault=%v want=%s", err, want)
	}
}
func createTestBike(t *testing.T, m *Manager, owner models.User) models.Bike {
	t.Helper()
	bike, err := m.Create(context.Background(), owner, models.BikeInput{Make: "My make", Model: "Unlisted bike", Year: 2020})
	if err != nil {
		t.Fatal(err)
	}
	return bike
}

func TestPrivateBikeRequiresExplicitSharingAndNeverExposesLogs(t *testing.T) {
	m, repo, owner, other := setupGarage(t)
	ctx := context.Background()
	bike, err := m.Create(ctx, owner, models.BikeInput{ModelID: "yamaha-mt-07", Year: 2026})
	if err != nil {
		t.Fatal(err)
	}
	if bike.Visibility != "private" || !bike.CanManage || bike.Version != 1 || bike.Make != "Yamaha" || bike.Model != "MT-07" {
		t.Fatalf("created bike=%v", bike)
	}
	_, err = m.View(ctx, bike.ID, nil)
	faultCode(t, err, "not_found")
	_, err = m.View(ctx, bike.ID, &models.Session{User: other})
	faultCode(t, err, "not_found")
	public := "public"
	shared, err := m.Update(ctx, owner, bike.ID, models.BikePatch{Version: 1, Visibility: &public})
	if err != nil {
		t.Fatal(err)
	}
	if shared.Version != 2 {
		t.Fatal("visibility did not advance version")
	}
	projection, err := m.View(ctx, bike.ID, &models.Session{User: other})
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(projection)
	var fields map[string]any
	_ = json.Unmarshal(encoded, &fields)
	if len(fields) != 5 || fields["owner_id"] != nil || fields["version"] != nil {
		t.Fatalf("unsafe public projection %s", encoded)
	}
	writes := repo.writes
	_, err = m.Update(ctx, other, bike.ID, models.BikePatch{Version: 2})
	faultCode(t, err, "not_found")
	_, err = m.Update(ctx, owner, bike.ID, models.BikePatch{Version: 1})
	faultCode(t, err, "stale_version")
	if repo.writes != writes {
		t.Fatal("unauthorized/stale mutation reached persistence")
	}
	repo.owners[owner.ID] = models.GarageOwner{ID: owner.ID, Status: "suspended"}
	_, err = m.Update(ctx, owner, bike.ID, models.BikePatch{Version: 2})
	faultCode(t, err, "not_found")
}

func TestPrivateMaintenanceAllowsBackfillAndUsesAlmatyCalendar(t *testing.T) {
	m, repo, owner, other := setupGarage(t)
	ctx := context.Background()
	bike := createTestBike(t, m, owner)
	zero := int64(0)
	entry, err := m.CreateMaintenance(ctx, owner, bike.ID, models.MaintenanceInput{ServiceDate: "2026-11-04", MileageKM: &zero, WorkPerformed: "  Oil changed  "})
	if err != nil {
		t.Fatal(err)
	}
	if entry.WorkPerformed != "Oil changed" || entry.Version != 1 {
		t.Fatalf("entry=%v", entry)
	}
	for _, date := range []string{"2026-11-05", "2026-02-30", "0000-01-01", "2026-1-01"} {
		_, err = m.CreateMaintenance(ctx, owner, bike.ID, models.MaintenanceInput{ServiceDate: date, MileageKM: &zero, WorkPerformed: "Check"})
		faultCode(t, err, "invalid_maintenance")
	}
	_, err = m.CreateMaintenance(ctx, owner, bike.ID, models.MaintenanceInput{ServiceDate: "2026-11-03", WorkPerformed: "Missing mileage"})
	faultCode(t, err, "invalid_maintenance")
	high := int64(123456)
	_, err = m.CreateMaintenance(ctx, owner, bike.ID, models.MaintenanceInput{ServiceDate: "2020-01-01", MileageKM: &high, WorkPerformed: "Backfilled older record"})
	if err != nil {
		t.Fatal("backfilled mileage must not obey monotonic rule:", err)
	}
	public := "public"
	_, err = m.Update(ctx, owner, bike.ID, models.BikePatch{Version: 1, Visibility: &public})
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.Maintenance(ctx, other, bike.ID, "")
	faultCode(t, err, "not_found")
	otherBike := createTestBike(t, m, owner)
	_, err = m.UpdateMaintenance(ctx, owner, otherBike.ID, entry.ID, models.MaintenancePatch{Version: 1})
	faultCode(t, err, "not_found")
	work := "New work"
	updated, err := m.UpdateMaintenance(ctx, owner, bike.ID, entry.ID, models.MaintenancePatch{Version: 1, WorkPerformed: &work})
	if err != nil || updated.Version != 2 {
		t.Fatalf("updated=%v err=%v", updated, err)
	}
	_, err = m.UpdateMaintenance(ctx, owner, bike.ID, entry.ID, models.MaintenancePatch{Version: 1})
	faultCode(t, err, "stale_version")
	err = m.Delete(ctx, owner, bike.ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range repo.entries {
		if entry.BikeID == bike.ID {
			t.Fatal("deleting bike retained maintenance")
		}
	}
}

func TestGaragePagesAndCuratedAliasLookup(t *testing.T) {
	m, repo, owner, other := setupGarage(t)
	ctx := context.Background()
	for i := 1; i <= 51; i++ {
		id := fmt.Sprintf("00000000-0000-0000-0000-%012x", i)
		repo.bikes[id] = models.Bike{ID: id, OwnerID: owner.ID, Make: "Make", Model: "Model", Year: 2020, Visibility: "private", Version: 1}
	}
	page, err := m.List(ctx, owner, "")
	if err != nil || len(page.Bikes) != 50 || page.NextCursor == "" {
		t.Fatalf("page=%v err=%v", page, err)
	}
	next, err := m.List(ctx, owner, page.NextCursor)
	if err != nil || len(next.Bikes) != 1 || next.NextCursor != "" {
		t.Fatalf("next=%v err=%v", next, err)
	}
	private, err := m.List(ctx, other, "")
	if err != nil || len(private.Bikes) != 0 {
		t.Fatal("another rider's bikes leaked")
	}
	_, err = m.List(ctx, owner, "not-a-uuid")
	faultCode(t, err, "invalid_cursor")
	for _, q := range []string{"ЯМАХА МТ-07", "yamaha mt07"} {
		matches, err := m.SearchModels(q)
		if err != nil || len(matches) != 1 || matches[0].ID != "yamaha-mt-07" {
			t.Fatalf("lookup %q=%v err=%v", q, matches, err)
		}
	}
	_, err = m.SearchModels(strings.Repeat("x", 201))
	faultCode(t, err, "invalid_query")
	bike := page.Bikes[0]
	for i := 1; i <= 51; i++ {
		id := fmt.Sprintf("10000000-0000-0000-0000-%012x", i)
		repo.entries[id] = models.MaintenanceEntry{ID: id, BikeID: bike.ID, ServiceDate: "2026-11-03", MileageKM: int64(i), WorkPerformed: "Private", Version: 1}
	}
	entries, err := m.Maintenance(ctx, owner, bike.ID, "")
	if err != nil || len(entries.Entries) != 50 || entries.NextCursor == "" {
		t.Fatalf("entries=%v err=%v", entries, err)
	}
	tail, err := m.Maintenance(ctx, owner, bike.ID, entries.NextCursor)
	if err != nil || len(tail.Entries) != 1 || tail.NextCursor != "" {
		t.Fatalf("tail=%v err=%v", tail, err)
	}
	_, err = m.Maintenance(ctx, owner, bike.ID, strings.Repeat("x", 257))
	faultCode(t, err, "invalid_cursor")
	_, err = m.Maintenance(ctx, other, bike.ID, "not-a-cursor")
	faultCode(t, err, "not_found")
}

func (r *memoryGarage) ContentHidden(context.Context, string, string) (bool, error) {
	return false, nil
}
func (t *memoryBike) ContentHidden(string, string) (bool, error) { return false, nil }

func TestRestrictedStaleOwnerCannotReadPrivateBike(t *testing.T) {
	m, repo, owner, _ := setupGarage(t)
	bike, err := m.Create(context.Background(), owner, models.BikeInput{Make: "Honda", Model: "CB500F", Year: 2020, Visibility: "private"})
	if err != nil {
		t.Fatal(err)
	}
	repo.owners[owner.ID] = models.GarageOwner{ID: owner.ID, Status: "suspended"}
	_, err = m.View(context.Background(), bike.ID, &models.Session{User: owner})
	faultCode(t, err, "not_found")
}
