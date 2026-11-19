package rides

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/managers/photos"
	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
)

type coverAttempt struct {
	owner string
	at    time.Time
}
type memoryRideCovers struct {
	state                             models.RideCoverState
	records                           map[string]models.RideCoverRecord
	attempts                          []coverAttempt
	otherOwnerBytes, otherGlobalBytes int64
	lostReadyCommit                   bool
}

func newRideCovers() *memoryRideCovers {
	return &memoryRideCovers{state: models.RideCoverState{Version: 1}, records: map[string]models.RideCoverRecord{}}
}
func (s *memoryRideCovers) clone() *memoryRideCovers {
	out := *s
	out.records = map[string]models.RideCoverRecord{}
	out.attempts = append([]coverAttempt{}, s.attempts...)
	for id, record := range s.records {
		record.Content = append([]byte{}, record.Content...)
		out.records[id] = record
	}
	return &out
}
func (r *rideRepository) WithRideCover(ctx context.Context, id, actor string, decide database.RideCoverDecision) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if id != r.ride.ID {
		return database.ErrNotFound
	}
	if r.covers == nil {
		r.covers = newRideCovers()
	}
	r.locked = true
	defer func() { r.locked = false }()
	before := r.covers
	tx := &memoryCoverTx{repo: r, state: r.covers.clone(), actor: actor}
	if err := decide(tx); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	r.covers = tx.state
	if r.covers.lostReadyCommit && before.state.CurrentCoverID != r.covers.state.CurrentCoverID && r.covers.state.CurrentCoverID != "" {
		r.covers.lostReadyCommit = false
		return errors.New("simulated successful commit with lost response")
	}
	return nil
}

type memoryCoverTx struct {
	repo  *rideRepository
	state *memoryRideCovers
	actor string
}

func (t *memoryCoverTx) Ride() models.Ride                 { return t.repo.ride }
func (t *memoryCoverTx) CoverState() models.RideCoverState { return t.state.state }
func (t *memoryCoverTx) ActorStatus() string {
	if t.actor == "" {
		return ""
	}
	if status, ok := t.repo.statuses[t.actor]; ok {
		return status
	}
	return "active"
}
func (t *memoryCoverTx) ActorParticipation() (string, error) { return t.repo.states[t.actor], nil }
func (t *memoryCoverTx) ContentHidden(kind, id string) (bool, error) {
	return t.repo.hidden[kind+":"+id], nil
}
func (t *memoryCoverTx) Cover(id string, content bool) (models.RideCoverRecord, error) {
	record, ok := t.state.records[id]
	if !ok || record.RideID != t.repo.ride.ID {
		return record, database.ErrNotFound
	}
	if content {
		record.Content = append([]byte{}, record.Content...)
	} else {
		record.Content = nil
	}
	return record, nil
}
func (t *memoryCoverTx) ExpirePending(now time.Time) error {
	for id, record := range t.state.records {
		if record.State == "pending" && record.ExpiresAt != nil && !now.Before(*record.ExpiresAt) {
			delete(t.state.records, id)
		}
	}
	return nil
}
func (t *memoryCoverTx) PrunePhotoAttempts(before time.Time) error {
	kept := []coverAttempt{}
	for _, attempt := range t.state.attempts {
		if !attempt.at.Before(before) {
			kept = append(kept, attempt)
		}
	}
	t.state.attempts = kept
	return nil
}
func (t *memoryCoverTx) CoverQuota(since time.Time) (models.RideCoverQuota, error) {
	quota := models.RideCoverQuota{OwnerBytes: t.state.otherOwnerBytes, GlobalBytes: t.state.otherGlobalBytes}
	for _, record := range t.state.records {
		quota.GlobalBytes += record.ReservedBytes
		if record.OwnerID == t.actor {
			quota.OwnerBytes += record.ReservedBytes
		}
		if record.RideID == t.repo.ride.ID && record.State == "pending" {
			quota.PendingCount++
		}
	}
	for _, attempt := range t.state.attempts {
		if attempt.owner == t.actor && !attempt.at.Before(since) {
			quota.RecentAttempts++
		}
	}
	return quota, nil
}
func (t *memoryCoverTx) RecordPhotoAttempt(now time.Time) error {
	t.state.attempts = append(t.state.attempts, coverAttempt{t.actor, now})
	return nil
}
func (t *memoryCoverTx) CreatePendingCover(record models.RideCoverRecord) error {
	t.state.records[record.ID] = record
	return nil
}
func (t *memoryCoverTx) FinalizeCover(record models.RideCoverRecord) (models.RideCoverRecord, error) {
	if old, ok := t.state.records[record.ID]; !ok || old.State != "pending" {
		return record, database.ErrNotFound
	}
	record.Content = append([]byte{}, record.Content...)
	t.state.records[record.ID] = record
	record.Content = nil
	return record, nil
}
func (t *memoryCoverTx) DeleteCover(id, state string) error {
	record, ok := t.state.records[id]
	if !ok || record.RideID != t.repo.ride.ID || record.State != state {
		return database.ErrNotFound
	}
	delete(t.state.records, id)
	return nil
}
func (t *memoryCoverTx) SetCoverState(state models.RideCoverState) error {
	t.state.state = state
	return nil
}

func processedCover(context.Context, io.Reader) (models.ProcessedPhoto, error) {
	return models.ProcessedPhoto{Bytes: []byte("normalized JPEG"), Width: 1280, Height: 720}, nil
}

// readyCover returns a ride fixture with one uploaded cover and an approved
// participant who is not the organizer.
func readyCover(t *testing.T) (*Manager, *rideRepository, *rideEvents, models.User, models.User, models.RideCover) {
	t.Helper()
	m, repo, events, owner := rideFixture(t)
	m.photos = photos.New(processedCover)
	rider := models.User{ID: uuid.NewString(), OnboardingComplete: true}
	repo.states[rider.ID] = "approved"
	cover, err := m.UploadCover(context.Background(), owner, repo.ride.ID, 1, strings.NewReader("source"))
	if err != nil {
		t.Fatal(err)
	}
	return m, repo, events, owner, rider, cover
}

func jsonFields(t *testing.T, value any) map[string]json.RawMessage {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	return fields
}

func TestRideCoverOrganizerUploadReplacementAndRemoval(t *testing.T) {
	m, repo, events, owner, rider, cover := readyCover(t)
	ctx := context.Background()
	id := repo.ride.ID
	if cover.Image == nil || cover.Version == nil || *cover.Version != 2 || cover.Image.URL != "/api/v1/rides/"+id+"/cover/"+cover.Image.ID+"/content" {
		t.Fatalf("organizer cover=%+v", cover)
	}
	if len(events.events) != 1 || events.events[0].Type != "ride.updated" {
		t.Fatalf("cover change did not refresh open ride views: %+v", events.events)
	}
	for _, session := range []*models.Session{nil, {User: rider}, {User: models.User{ID: uuid.NewString()}}} {
		public, err := m.Cover(ctx, id, session)
		if err != nil {
			t.Fatal(err)
		}
		if fields := jsonFields(t, public); len(fields) != 1 || public.Version != nil {
			t.Fatalf("public cover metadata leaks fields: %+v", public)
		}
		if fields := jsonFields(t, public.Image); len(fields) != 5 {
			t.Fatalf("photo DTO changed: %+v", public.Image)
		}
	}
	_, err := m.UploadCover(ctx, rider, id, 2, strings.NewReader("source"))
	requireFault(t, err, "forbidden")
	_, err = m.RemoveCover(ctx, rider, id, 2)
	requireFault(t, err, "forbidden")
	_, err = m.UploadCover(ctx, models.User{ID: owner.ID}, id, 2, strings.NewReader("source"))
	requireFault(t, err, "profile_required")
	_, err = m.UploadCover(ctx, owner, id, 1, strings.NewReader("source"))
	requireFault(t, err, "stale_version")
	m.photos = photos.New(func(context.Context, io.Reader) (models.ProcessedPhoto, error) {
		return models.ProcessedPhoto{}, models.Fail("invalid_photo", "Invalid image")
	})
	_, err = m.UploadCover(ctx, owner, id, 2, strings.NewReader("invalid"))
	requireFault(t, err, "invalid_photo")
	content, err := m.CoverContent(ctx, id, cover.Image.ID, nil)
	if err != nil || string(content.Bytes) != "normalized JPEG" || repo.covers.state.Version != 2 || len(repo.covers.records) != 1 {
		t.Fatalf("failed replacement destroyed the old cover: %v", err)
	}
	m.photos = photos.New(processedCover)
	replaced, err := m.UploadCover(ctx, owner, id, 2, strings.NewReader("source"))
	if err != nil {
		t.Fatal(err)
	}
	if replaced.Image.ID == cover.Image.ID || *replaced.Version != 3 || len(repo.covers.records) != 1 {
		t.Fatal("replacement was not atomic")
	}
	_, err = m.CoverContent(ctx, id, cover.Image.ID, nil)
	requireFault(t, err, "not_found")
	_, err = m.RemoveCover(ctx, owner, id, 2)
	requireFault(t, err, "stale_version")
	removed, err := m.RemoveCover(ctx, owner, id, 3)
	if err != nil || removed.Image != nil || *removed.Version != 4 || len(repo.covers.records) != 0 {
		t.Fatalf("remove=%+v %v", removed, err)
	}
	for _, version := range []int64{0, -1} {
		_, err = m.UploadCover(ctx, owner, id, version, strings.NewReader("source"))
		requireFault(t, err, "invalid_photo_choice")
		_, err = m.RemoveCover(ctx, owner, id, version)
		requireFault(t, err, "invalid_photo_choice")
	}
	_, err = m.Cover(ctx, uuid.NewString(), nil)
	requireFault(t, err, "not_found")
}

func TestRideCoverFollowsRideVisibilityAndModeration(t *testing.T) {
	m, repo, _, owner, rider, cover := readyCover(t)
	ctx := context.Background()
	id := repo.ride.ID
	outsider := models.User{ID: uuid.NewString(), OnboardingComplete: true}
	others := []*models.Session{nil, {User: outsider}}
	for _, state := range []string{"requested", "rejected", "removed", "left"} {
		user := models.User{ID: uuid.NewString(), OnboardingComplete: true}
		repo.states[user.ID] = state
		others = append(others, &models.Session{User: user})
	}
	for _, session := range others {
		public, err := m.Cover(ctx, id, session)
		if err != nil || public.Image == nil || public.Image.ID != cover.Image.ID || public.Version != nil {
			t.Fatalf("visible ride cover=%+v %v", public, err)
		}
		if _, err = m.CoverContent(ctx, id, cover.Image.ID, session); err != nil {
			t.Fatal(err)
		}
	}
	repo.hidden = map[string]bool{"ride:" + id: true}
	for _, session := range others {
		_, err := m.Cover(ctx, id, session)
		requireFault(t, err, "not_found")
		_, err = m.CoverContent(ctx, id, cover.Image.ID, session)
		requireFault(t, err, "not_found")
		if session == nil {
			continue
		}
		_, err = m.UploadCover(ctx, session.User, id, 2, strings.NewReader("source"))
		requireFault(t, err, "forbidden")
		_, err = m.RemoveCover(ctx, session.User, id, 2)
		requireFault(t, err, "forbidden")
	}
	for _, session := range []*models.Session{{User: rider}, {User: owner}} {
		if visible, err := m.Cover(ctx, id, session); err != nil || visible.Image == nil {
			t.Fatalf("hidden ride cover unavailable to its riders: %v", err)
		}
		if _, err := m.CoverContent(ctx, id, cover.Image.ID, session); err != nil {
			t.Fatal(err)
		}
	}
	repo.statuses = map[string]string{rider.ID: "suspended"}
	_, err := m.Cover(ctx, id, &models.Session{User: rider})
	requireFault(t, err, "not_found")
	repo.statuses = nil
	repo.hidden = map[string]bool{"ride_cover:" + cover.Image.ID: true}
	for _, session := range []*models.Session{nil, {User: rider}} {
		public, err := m.Cover(ctx, id, session)
		if err != nil || public.Image != nil {
			t.Fatalf("moderated cover served to others: %+v %v", public, err)
		}
	}
	organizer, err := m.Cover(ctx, id, &models.Session{User: owner})
	if err != nil || organizer.Image == nil || organizer.Version == nil {
		t.Fatalf("organizer cannot replace a moderated cover: %+v %v", organizer, err)
	}
	for _, session := range []*models.Session{nil, {User: owner}} {
		_, err = m.CoverContent(ctx, id, cover.Image.ID, session)
		requireFault(t, err, "not_found")
	}
}

func TestRideCoverStaysPublicOnCancelledRideButRejectsChanges(t *testing.T) {
	m, repo, _, owner, _, cover := readyCover(t)
	ctx := context.Background()
	id := repo.ride.ID
	if _, err := m.Cancel(ctx, owner, id, repo.ride.PlanVersion); err != nil {
		t.Fatal(err)
	}
	for _, session := range []*models.Session{nil, {User: owner}} {
		current, err := m.Cover(ctx, id, session)
		if err != nil || current.Image == nil || current.Image.ID != cover.Image.ID || current.Version != nil {
			t.Fatalf("cancelled ride cover=%+v %v", current, err)
		}
	}
	_, err := m.UploadCover(ctx, owner, id, 2, strings.NewReader("source"))
	requireFault(t, err, "ride_cancelled")
	_, err = m.RemoveCover(ctx, owner, id, 2)
	requireFault(t, err, "ride_cancelled")
	if len(repo.covers.records) != 1 || repo.covers.state.CurrentCoverID != cover.Image.ID {
		t.Fatal("rejected change altered the cancelled ride cover")
	}
}

func TestRideCoverStaysEditableAfterThePublishedRideStarts(t *testing.T) {
	m, repo, _, owner, rider, cover := readyCover(t)
	ctx := context.Background()
	id := repo.ride.ID
	departed := repo.ride.StartsAt.Add(time.Hour)
	m.now = func() time.Time { return departed }
	current, err := m.Cover(ctx, id, &models.Session{User: owner})
	if err != nil || current.Version == nil || *current.Version != 2 || current.Image == nil || current.Image.ID != cover.Image.ID {
		t.Fatalf("departed ride stopped offering cover changes: %+v %v", current, err)
	}
	_, err = m.UploadCover(ctx, rider, id, 2, strings.NewReader("source"))
	requireFault(t, err, "forbidden")
	_, err = m.RemoveCover(ctx, rider, id, 2)
	requireFault(t, err, "forbidden")
	replaced, err := m.UploadCover(ctx, owner, id, 2, strings.NewReader("source"))
	if err != nil || replaced.Image == nil || replaced.Image.ID == cover.Image.ID || *replaced.Version != 3 {
		t.Fatalf("departed ride replacement=%+v %v", replaced, err)
	}
	if public, err := m.Cover(ctx, id, nil); err != nil || public.Image == nil || public.Image.ID != replaced.Image.ID || public.Version != nil {
		t.Fatalf("departed ride public cover=%+v %v", public, err)
	}
	if _, err = m.CoverContent(ctx, id, replaced.Image.ID, nil); err != nil {
		t.Fatal(err)
	}
	removed, err := m.RemoveCover(ctx, owner, id, 3)
	if err != nil || removed.Image != nil || *removed.Version != 4 || len(repo.covers.records) != 0 {
		t.Fatalf("departed ride removal=%+v %v", removed, err)
	}
}

func TestRideCoverFinalizeRevalidatesOrganizerRideAndVersion(t *testing.T) {
	for _, change := range []string{"expired", "suspended", "cancel", "remove"} {
		t.Run(change, func(t *testing.T) {
			m, repo, _, owner, _, old := readyCover(t)
			ctx := context.Background()
			id := repo.ride.ID
			m.photos = photos.New(func(context.Context, io.Reader) (models.ProcessedPhoto, error) {
				if repo.locked {
					t.Fatal("cover processing ran under the ride lock")
				}
				visible, err := m.Cover(ctx, id, nil)
				if err != nil || visible.Image.ID != old.Image.ID {
					t.Fatal("pending replacement replaced the old cover early")
				}
				switch change {
				case "expired":
					later := m.now().Add(photos.ReservationTTL)
					m.now = func() time.Time { return later }
				case "suspended":
					repo.statuses = map[string]string{owner.ID: "suspended"}
				case "cancel":
					if _, err = m.Cancel(ctx, owner, id, repo.ride.PlanVersion); err != nil {
						t.Fatal(err)
					}
				case "remove":
					if _, err = m.RemoveCover(ctx, owner, id, 2); err != nil {
						t.Fatal(err)
					}
				}
				return processedCover(ctx, nil)
			})
			_, err := m.UploadCover(ctx, owner, id, 2, strings.NewReader("source"))
			want := map[string]string{"expired": "photo_upload_expired", "suspended": "account_unavailable", "cancel": "ride_cancelled", "remove": "stale_version"}[change]
			requireFault(t, err, want)
			if change == "remove" {
				if len(repo.covers.records) != 0 {
					t.Fatal("stale replacement resurrected the removed cover")
				}
			} else if len(repo.covers.records) != 1 || repo.covers.state.CurrentCoverID != old.Image.ID {
				t.Fatal("failed finalization destroyed old content or kept the pending reservation")
			}
		})
	}
}

func TestRideCoverCancellationAndLostCommitNeverDeleteReadyCover(t *testing.T) {
	m, repo, _, owner, _, old := readyCover(t)
	ctx, cancel := context.WithCancel(context.Background())
	m.photos = photos.New(func(context.Context, io.Reader) (models.ProcessedPhoto, error) {
		cancel()
		return models.ProcessedPhoto{}, context.Canceled
	})
	_, err := m.UploadCover(ctx, owner, repo.ride.ID, 2, strings.NewReader("source"))
	if !errors.Is(err, context.Canceled) || len(repo.covers.records) != 1 || repo.covers.state.CurrentCoverID != old.Image.ID {
		t.Fatalf("cancelled cleanup changed ready content: %v", err)
	}
	m.photos = photos.New(processedCover)
	repo.covers.lostReadyCommit = true
	if _, err = m.UploadCover(context.Background(), owner, repo.ride.ID, 2, strings.NewReader("source")); err == nil {
		t.Fatal("expected uncertain commit error")
	}
	if len(repo.covers.records) != 1 || repo.covers.state.CurrentCoverID == old.Image.ID || repo.covers.state.Version != 3 {
		t.Fatal("cleanup removed a committed replacement")
	}
}

func TestRideCoverSharedQuotaAttemptsAndBusyReservation(t *testing.T) {
	for _, kind := range []string{"owner", "global"} {
		t.Run(kind, func(t *testing.T) {
			m, repo, _, owner, _, old := readyCover(t)
			if kind == "owner" {
				repo.covers.otherOwnerBytes = photos.OwnerByteLimit - int64(len("normalized JPEG"))
			} else {
				repo.covers.otherGlobalBytes = photos.GlobalByteLimit - int64(len("normalized JPEG"))
			}
			_, err := m.UploadCover(context.Background(), owner, repo.ride.ID, 2, strings.NewReader("source"))
			requireFault(t, err, "photo_storage_limit")
			if len(repo.covers.records) != 1 || repo.covers.state.CurrentCoverID != old.Image.ID {
				t.Fatal("quota failure changed the current cover")
			}
		})
	}
	m, repo, _, owner := rideFixture(t)
	m.photos = photos.New(func(context.Context, io.Reader) (models.ProcessedPhoto, error) {
		return models.ProcessedPhoto{}, models.Fail("invalid_photo", "Invalid image")
	})
	for range photos.HourlyAttempts {
		_, err := m.UploadCover(context.Background(), owner, repo.ride.ID, 1, strings.NewReader("invalid"))
		requireFault(t, err, "invalid_photo")
	}
	_, err := m.UploadCover(context.Background(), owner, repo.ride.ID, 1, strings.NewReader("source"))
	requireFault(t, err, "photo_rate_limited")
	if len(repo.covers.records) != 0 || len(repo.covers.attempts) != photos.HourlyAttempts {
		t.Fatal("decode failures must consume attempts and release reservations")
	}
	m, repo, _, owner = rideFixture(t)
	m.photos = photos.New(func(context.Context, io.Reader) (models.ProcessedPhoto, error) {
		_, err := m.UploadCover(context.Background(), owner, repo.ride.ID, 1, strings.NewReader("source"))
		requireFault(t, err, "ride_cover_busy")
		return processedCover(context.Background(), nil)
	})
	if _, err := m.UploadCover(context.Background(), owner, repo.ride.ID, 1, strings.NewReader("source")); err != nil {
		t.Fatal(err)
	}
}

func TestRideShapesCarryPublicCoverAndOrganizerVersion(t *testing.T) {
	m, repo, _, owner := rideFixture(t)
	ctx := context.Background()
	photo := &models.Photo{ID: uuid.NewString(), URL: "/api/v1/rides/" + repo.ride.ID + "/cover/x/content", Width: 1280, Height: 720}
	repo.ride.Cover, repo.ride.CoverVersion = photo, 3
	rider := models.User{ID: uuid.NewString()}
	repo.states[rider.ID] = "approved"
	preview, err := m.Preview(ctx, repo.ride.ID)
	if err != nil || preview.Cover != photo {
		t.Fatalf("preview lost the public cover: %v", err)
	}
	guest, err := m.View(ctx, repo.ride.ID, nil)
	if err != nil || guest.(models.RideSummary).Cover != photo {
		t.Fatalf("guest summary lost the public cover: %v", err)
	}
	full, err := m.View(ctx, repo.ride.ID, &models.Session{User: owner})
	if err != nil || full.(models.Ride).Cover != photo || full.(models.Ride).CoverVersion != 3 {
		t.Fatalf("organizer ride=%+v %v", full, err)
	}
	participant, err := m.View(ctx, repo.ride.ID, &models.Session{User: rider})
	if err != nil || participant.(models.Ride).Cover != photo || participant.(models.Ride).CoverVersion != 0 {
		t.Fatalf("participant received the cover version: %+v %v", participant, err)
	}
	if _, leaked := jsonFields(t, participant)["cover_version"]; leaked {
		t.Fatal("participant JSON contains cover_version")
	}
	repo.list = []models.RideListing{{Summary: repo.ride.RideSummary, OrganizerID: owner.ID}}
	page, err := m.List(ctx, nil, models.RideListInput{})
	if err != nil || len(page.Rides) != 1 || page.Rides[0].Cover != photo {
		t.Fatalf("feed card lost the public cover: %v", err)
	}
	cancelled, err := m.Cancel(ctx, owner, repo.ride.ID, repo.ride.PlanVersion)
	if err != nil || cancelled.CoverVersion != 0 || cancelled.Cover != photo {
		t.Fatalf("cancelled organizer ride=%+v %v", cancelled, err)
	}
}
