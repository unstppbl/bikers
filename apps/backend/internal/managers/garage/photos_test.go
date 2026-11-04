package garage

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/managers/photos"
	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
)

type memoryPhotoAttempt struct {
	owner string
	at    time.Time
}
type memoryPhotoState struct {
	photos          map[string]models.PhotoRecord
	galleries       map[string]models.PhotoGalleryState
	attempts        []memoryPhotoAttempt
	lostReadyCommit bool
}

func newMemoryPhotoState() *memoryPhotoState {
	return &memoryPhotoState{photos: map[string]models.PhotoRecord{}, galleries: map[string]models.PhotoGalleryState{}}
}
func clonePhotoRecord(photo models.PhotoRecord) models.PhotoRecord {
	photo.Content = append([]byte(nil), photo.Content...)
	return photo
}
func (state *memoryPhotoState) clone() *memoryPhotoState {
	copy := &memoryPhotoState{photos: map[string]models.PhotoRecord{}, galleries: maps.Clone(state.galleries), attempts: append([]memoryPhotoAttempt{}, state.attempts...), lostReadyCommit: state.lostReadyCommit}
	for id, photo := range state.photos {
		copy.photos[id] = clonePhotoRecord(photo)
	}
	return copy
}

func (r *memoryGarage) WithPhotoBike(ctx context.Context, id string, decide database.PhotoDecision) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	bike, ok := r.bikes[id]
	if !ok {
		return database.ErrNotFound
	}
	if r.photoState == nil {
		r.photoState = newMemoryPhotoState()
	}
	beforeBikes, beforeEntries, beforePhotos := maps.Clone(r.bikes), maps.Clone(r.entries), r.photoState.clone()
	tx := &memoryPhotoTransaction{memoryBike: memoryBike{memoryOwner: memoryOwner{repo: r, owner: r.owners[bike.OwnerID]}, bike: bike}, state: r.photoState}
	if err := decide(tx); err != nil {
		r.bikes, r.entries, r.photoState = beforeBikes, beforeEntries, beforePhotos
		return err
	}
	if tx.finalized && r.photoState.lostReadyCommit {
		r.photoState.lostReadyCommit = false
		return errors.New("simulated successful commit with lost response")
	}
	return nil
}

type memoryPhotoTransaction struct {
	memoryBike
	state     *memoryPhotoState
	finalized bool
}

func (tx *memoryPhotoTransaction) GalleryState() models.PhotoGalleryState {
	state, ok := tx.state.galleries[tx.bike.ID]
	if !ok {
		return models.PhotoGalleryState{Version: 1}
	}
	return state
}
func (tx *memoryPhotoTransaction) ReadyPhotos() ([]models.PhotoRecord, error) {
	photos := []models.PhotoRecord{}
	for _, photo := range tx.state.photos {
		if photo.BikeID == tx.bike.ID && photo.State == "ready" {
			photo.Content = nil
			photos = append(photos, photo)
		}
	}
	sort.Slice(photos, func(i, j int) bool {
		if photos[i].CreatedAt.Equal(photos[j].CreatedAt) {
			return photos[i].ID < photos[j].ID
		}
		return photos[i].CreatedAt.Before(photos[j].CreatedAt)
	})
	return photos, nil
}
func (tx *memoryPhotoTransaction) Photo(id string, content bool) (models.PhotoRecord, error) {
	photo, ok := tx.state.photos[id]
	if !ok || photo.BikeID != tx.bike.ID {
		return models.PhotoRecord{}, database.ErrNotFound
	}
	photo = clonePhotoRecord(photo)
	if !content {
		photo.Content = nil
	}
	return photo, nil
}
func (tx *memoryPhotoTransaction) ExpirePending(now time.Time) error {
	for id, photo := range tx.state.photos {
		if photo.State == "pending" && photo.ExpiresAt != nil && !now.Before(*photo.ExpiresAt) {
			delete(tx.state.photos, id)
		}
	}
	return nil
}
func (tx *memoryPhotoTransaction) PrunePhotoAttempts(before time.Time) error {
	attempts := []memoryPhotoAttempt{}
	for _, attempt := range tx.state.attempts {
		if !attempt.at.Before(before) {
			attempts = append(attempts, attempt)
		}
	}
	tx.state.attempts = attempts
	return nil
}
func (tx *memoryPhotoTransaction) PhotoQuota(since time.Time) (models.PhotoQuota, error) {
	var quota models.PhotoQuota
	for _, photo := range tx.state.photos {
		quota.GlobalBytes += photo.ReservedBytes
		if photo.OwnerID == tx.owner.ID {
			quota.OwnerBytes += photo.ReservedBytes
		}
		if photo.BikeID == tx.bike.ID {
			quota.BikeCount++
		}
	}
	for _, attempt := range tx.state.attempts {
		if attempt.owner == tx.owner.ID && !attempt.at.Before(since) {
			quota.RecentAttempts++
		}
	}
	return quota, nil
}
func (tx *memoryPhotoTransaction) RecordPhotoAttempt(now time.Time) error {
	tx.state.attempts = append(tx.state.attempts, memoryPhotoAttempt{owner: tx.owner.ID, at: now})
	return nil
}
func (tx *memoryPhotoTransaction) CreatePendingPhoto(photo models.PhotoRecord) error {
	tx.state.photos[photo.ID] = clonePhotoRecord(photo)
	return nil
}
func (tx *memoryPhotoTransaction) FinalizePhoto(photo models.PhotoRecord) (models.PhotoRecord, error) {
	tx.state.photos[photo.ID] = clonePhotoRecord(photo)
	tx.finalized = true
	photo.Content = nil
	return photo, nil
}
func (tx *memoryPhotoTransaction) DeletePhoto(id, expectedState string) error {
	photo, ok := tx.state.photos[id]
	if !ok || photo.BikeID != tx.bike.ID || photo.State != expectedState {
		return database.ErrNotFound
	}
	delete(tx.state.photos, id)
	return nil
}
func (tx *memoryPhotoTransaction) SetPhotoGallery(state models.PhotoGalleryState) error {
	tx.state.galleries[tx.bike.ID] = state
	return nil
}
func (tx *memoryPhotoTransaction) DeleteBike() error {
	if err := tx.memoryBike.DeleteBike(); err != nil {
		return err
	}
	for id, photo := range tx.state.photos {
		if photo.BikeID == tx.bike.ID {
			delete(tx.state.photos, id)
		}
	}
	delete(tx.state.galleries, tx.bike.ID)
	return nil
}

func testProcessedPhoto(context.Context, io.Reader) (models.ProcessedPhoto, error) {
	return models.ProcessedPhoto{Bytes: []byte("processed jpeg bytes"), Width: 600, Height: 400}, nil
}

type countedPhotoReader struct{ reads atomic.Int32 }

func (r *countedPhotoReader) Read([]byte) (int, error) {
	r.reads.Add(1)
	return 0, io.EOF
}

func TestBikePhotosArePrivateUntilBikeSharedAndCoverIsExplicit(t *testing.T) {
	m, repo, owner, other := setupGarage(t)
	m.photos = photos.New(testProcessedPhoto)
	ctx := context.Background()
	bike := createTestBike(t, m, owner)
	result, err := m.UploadPhoto(ctx, owner, bike.ID, strings.NewReader("source"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Gallery.Version == nil || *result.Gallery.Version != 2 || result.Gallery.CoverPhotoID != "" || len(result.Gallery.Photos) != 1 {
		t.Fatalf("upload changed default cover or gallery version: %+v", result)
	}
	for _, session := range []*models.Session{nil, {User: other}} {
		_, err = m.Photos(ctx, bike.ID, session)
		faultCode(t, err, "not_found")
		_, err = m.PhotoContent(ctx, bike.ID, result.Photo.ID, session)
		faultCode(t, err, "not_found")
	}
	reader := &countedPhotoReader{}
	_, err = m.UploadPhoto(ctx, other, bike.ID, reader)
	faultCode(t, err, "not_found")
	if reader.reads.Load() != 0 {
		t.Fatal("unauthorized upload read the body")
	}
	public := "public"
	if _, err = m.Update(ctx, owner, bike.ID, models.BikePatch{Version: 1, Visibility: &public}); err != nil {
		t.Fatal(err)
	}
	gallery, err := m.Photos(ctx, bike.ID, nil)
	if err != nil || gallery.Version != nil || len(gallery.Photos) != 1 {
		t.Fatalf("public gallery=%+v err=%v", gallery, err)
	}
	encoded, _ := json.Marshal(gallery)
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(encoded, &fields)
	if len(fields) != 2 {
		t.Fatalf("public gallery has private fields: %s", encoded)
	}
	encoded, _ = json.Marshal(gallery.Photos[0])
	fields = nil
	_ = json.Unmarshal(encoded, &fields)
	if len(fields) != 5 {
		t.Fatalf("photo projection changed: %s", encoded)
	}
	selected, err := m.SetPhotoCover(ctx, owner, bike.ID, models.PhotoCoverInput{Version: 2, PhotoID: result.Photo.ID})
	if err != nil || selected.CoverPhotoID != result.Photo.ID || *selected.Version != 3 {
		t.Fatalf("select cover=%+v err=%v", selected, err)
	}
	_, err = m.SetPhotoCover(ctx, owner, bike.ID, models.PhotoCoverInput{Version: 2})
	faultCode(t, err, "stale_version")
	_, err = m.SetPhotoCover(ctx, owner, bike.ID, models.PhotoCoverInput{Version: 3, PhotoID: uuid.NewString()})
	faultCode(t, err, "invalid_photo_choice")
	deleted, err := m.DeletePhoto(ctx, owner, bike.ID, result.Photo.ID, 3)
	if err != nil || deleted.CoverPhotoID != "" || len(deleted.Photos) != 0 || *deleted.Version != 4 {
		t.Fatalf("delete selected cover=%+v err=%v", deleted, err)
	}
	if len(repo.photoState.photos) != 0 {
		t.Fatal("deleted photo content retained")
	}
}

func TestPhotoDecodeFailuresConsumeAttemptsAndCleanupSurvivesCancellation(t *testing.T) {
	m, repo, owner, _ := setupGarage(t)
	ctx := context.Background()
	bike := createTestBike(t, m, owner)
	m.photos = photos.New(func(context.Context, io.Reader) (models.ProcessedPhoto, error) {
		return models.ProcessedPhoto{}, models.Fail("invalid_photo", "Invalid image")
	})
	for range photos.HourlyAttempts {
		_, err := m.UploadPhoto(ctx, owner, bike.ID, strings.NewReader("invalid"))
		faultCode(t, err, "invalid_photo")
	}
	if len(repo.photoState.photos) != 0 || len(repo.photoState.attempts) != photos.HourlyAttempts {
		t.Fatal("invalid images must release reservations and retain attempt history")
	}
	reader := &countedPhotoReader{}
	_, err := m.UploadPhoto(ctx, owner, bike.ID, reader)
	faultCode(t, err, "photo_rate_limited")
	if reader.reads.Load() != 0 {
		t.Fatal("rate-limited upload read the body")
	}
	m.now = func() time.Time { return time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC) }
	cancelled, cancel := context.WithCancel(ctx)
	m.photos = photos.New(func(context.Context, io.Reader) (models.ProcessedPhoto, error) {
		cancel()
		return models.ProcessedPhoto{}, context.Canceled
	})
	_, err = m.UploadPhoto(cancelled, owner, bike.ID, strings.NewReader("source"))
	if !errors.Is(err, context.Canceled) || len(repo.photoState.photos) != 0 || len(repo.photoState.attempts) != 1 {
		t.Fatalf("independent cleanup/pruning failed: photos=%d attempts=%d err=%v", len(repo.photoState.photos), len(repo.photoState.attempts), err)
	}
}

func TestPhotoFinalizeRevalidatesExpiryAccountAndBikeDeletion(t *testing.T) {
	for _, change := range []string{"expired", "suspended", "deleted"} {
		t.Run(change, func(t *testing.T) {
			m, repo, owner, _ := setupGarage(t)
			ctx := context.Background()
			bike := createTestBike(t, m, owner)
			m.photos = photos.New(func(context.Context, io.Reader) (models.ProcessedPhoto, error) {
				gallery, err := m.Photos(ctx, bike.ID, &models.Session{User: owner})
				if err != nil || len(gallery.Photos) != 0 || *gallery.Version != 1 {
					t.Fatalf("pending upload became visible or processor ran under DB lock: %+v %v", gallery, err)
				}
				switch change {
				case "expired":
					later := m.now().Add(photos.ReservationTTL)
					m.now = func() time.Time { return later }
				case "suspended":
					repo.owners[owner.ID] = models.GarageOwner{ID: owner.ID, Status: "suspended"}
				case "deleted":
					if err := m.Delete(ctx, owner, bike.ID, bike.Version); err != nil {
						t.Fatal(err)
					}
				}
				return testProcessedPhoto(ctx, nil)
			})
			_, err := m.UploadPhoto(ctx, owner, bike.ID, strings.NewReader("source"))
			want := "not_found"
			if change == "expired" {
				want = "photo_upload_expired"
			}
			faultCode(t, err, want)
			if len(repo.photoState.photos) != 0 {
				t.Fatal("failed finalize retained content or pending quota")
			}
		})
	}
}

func TestPhotoProcessingSemaphoreRejectsBeforeReadingAndLostCommitKeepsReadyContent(t *testing.T) {
	m, repo, owner, _ := setupGarage(t)
	ctx := context.Background()
	bike := createTestBike(t, m, owner)
	entered, release := make(chan struct{}, 2), make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	m.photos = photos.New(func(context.Context, io.Reader) (models.ProcessedPhoto, error) {
		entered <- struct{}{}
		<-release
		return testProcessedPhoto(ctx, nil)
	})
	finished := make(chan error, 2)
	for range 2 {
		go func() { _, err := m.UploadPhoto(ctx, owner, bike.ID, strings.NewReader("source")); finished <- err }()
	}
	for range 2 {
		select {
		case <-entered:
		case <-time.After(3 * time.Second):
			t.Fatal("uploads failed to leave database transaction before processing")
		}
	}
	reader := &countedPhotoReader{}
	_, err := m.UploadPhoto(ctx, owner, bike.ID, reader)
	faultCode(t, err, "photo_upload_busy")
	if reader.reads.Load() != 0 {
		t.Fatal("busy upload read body")
	}
	close(release)
	for range 2 {
		if err := <-finished; err != nil {
			t.Fatal(err)
		}
	}
	if len(repo.photoState.photos) != 2 || len(repo.photoState.attempts) != 2 {
		t.Fatal("processing semaphore changed reserved upload counts")
	}
	m.photos = photos.New(testProcessedPhoto)
	repo.photoState.lostReadyCommit = true
	_, err = m.UploadPhoto(ctx, owner, bike.ID, strings.NewReader("source"))
	if err == nil {
		t.Fatal("expected lost commit response")
	}
	gallery, err := m.Photos(ctx, bike.ID, &models.Session{User: owner})
	if err != nil || len(gallery.Photos) != 3 || *gallery.Version != 4 || gallery.CoverPhotoID != "" {
		t.Fatalf("cleanup destroyed committed upload: %+v err=%v", gallery, err)
	}
}

func TestPhotoPendingReservationsEnforceBikeAndByteQuotas(t *testing.T) {
	for _, scenario := range []string{"bike", "owner", "global", "expired"} {
		t.Run(scenario, func(t *testing.T) {
			m, repo, owner, other := setupGarage(t)
			ctx := context.Background()
			bike := createTestBike(t, m, owner)
			repo.photoState = newMemoryPhotoState()
			expires := m.now().Add(time.Minute)
			count := maxBikePhotos
			if scenario == "owner" {
				count = int(photos.OwnerByteLimit / photos.ReservedBytes)
			}
			if scenario == "global" {
				count = int(photos.GlobalByteLimit / photos.ReservedBytes)
			}
			if scenario == "expired" {
				expires = m.now()
			}
			for range count {
				id, bikeID, ownerID := uuid.NewString(), bike.ID, owner.ID
				if scenario == "owner" || scenario == "global" {
					bikeID = uuid.NewString()
				}
				if scenario == "global" {
					ownerID = other.ID
				}
				repo.photoState.photos[id] = models.PhotoRecord{ID: id, BikeID: bikeID, OwnerID: ownerID, State: "pending", ReservedBytes: photos.ReservedBytes, ExpiresAt: &expires}
			}
			m.photos = photos.New(testProcessedPhoto)
			_, err := m.UploadPhoto(ctx, owner, bike.ID, strings.NewReader("source"))
			if scenario == "expired" {
				if err != nil || len(repo.photoState.photos) != 1 {
					t.Fatalf("expired reservations still occupy quota: %v", err)
				}
			} else {
				want := "photo_storage_limit"
				if scenario == "bike" {
					want = "photo_limit"
				}
				faultCode(t, err, want)
			}
		})
	}
}
