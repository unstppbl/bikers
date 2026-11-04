package clubs

import (
	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/managers/photos"
	"bikers.kz/backend/internal/models"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

type imageAttempt struct {
	owner string
	at    time.Time
}
type memoryClubImages struct {
	state                             models.ClubImageState
	records                           map[string]models.ClubImageRecord
	attempts                          []imageAttempt
	otherOwnerBytes, otherGlobalBytes int64
	lostReadyCommit                   bool
}

func newClubImages() *memoryClubImages {
	return &memoryClubImages{state: models.ClubImageState{Version: 1}, records: map[string]models.ClubImageRecord{}}
}
func (s *memoryClubImages) clone() *memoryClubImages {
	out := *s
	out.records = map[string]models.ClubImageRecord{}
	out.attempts = append([]imageAttempt{}, s.attempts...)
	for id, record := range s.records {
		record.Content = append([]byte{}, record.Content...)
		out.records[id] = record
	}
	return &out
}
func (r *clubRepo) WithClubImage(ctx context.Context, id, actor string, decide database.ClubImageDecision) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if id != r.club.ID {
		return database.ErrNotFound
	}
	if r.images == nil {
		r.images = newClubImages()
	}
	before := r.images
	tx := &memoryClubImageTx{repo: r, state: r.images.clone(), actor: actor}
	if err := decide(tx); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	r.images = tx.state
	if r.images.lostReadyCommit && before.state.CurrentImageID != r.images.state.CurrentImageID && r.images.state.CurrentImageID != "" {
		r.images.lostReadyCommit = false
		return errors.New("simulated successful commit with lost response")
	}
	return nil
}

type memoryClubImageTx struct {
	repo  *clubRepo
	state *memoryClubImages
	actor string
}

func (t *memoryClubImageTx) Club() models.Club                 { return t.repo.club }
func (t *memoryClubImageTx) ActorStatus() string               { return t.repo.members[t.actor].AccountStatus }
func (t *memoryClubImageTx) ImageState() models.ClubImageState { return t.state.state }
func (t *memoryClubImageTx) Image(id string, content bool) (models.ClubImageRecord, error) {
	record, ok := t.state.records[id]
	if !ok || record.ClubID != t.repo.club.ID {
		return record, database.ErrNotFound
	}
	if content {
		record.Content = append([]byte{}, record.Content...)
	} else {
		record.Content = nil
	}
	return record, nil
}
func (t *memoryClubImageTx) ExpirePending(now time.Time) error {
	for id, record := range t.state.records {
		if record.State == "pending" && record.ExpiresAt != nil && !now.Before(*record.ExpiresAt) {
			delete(t.state.records, id)
		}
	}
	return nil
}
func (t *memoryClubImageTx) PrunePhotoAttempts(before time.Time) error {
	kept := []imageAttempt{}
	for _, attempt := range t.state.attempts {
		if !attempt.at.Before(before) {
			kept = append(kept, attempt)
		}
	}
	t.state.attempts = kept
	return nil
}
func (t *memoryClubImageTx) ImageQuota(since time.Time) (models.ClubImageQuota, error) {
	quota := models.ClubImageQuota{OwnerBytes: t.state.otherOwnerBytes, GlobalBytes: t.state.otherGlobalBytes}
	for _, record := range t.state.records {
		quota.GlobalBytes += record.ReservedBytes
		if record.OwnerID == t.actor {
			quota.OwnerBytes += record.ReservedBytes
		}
		if record.ClubID == t.repo.club.ID && record.State == "pending" {
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
func (t *memoryClubImageTx) RecordPhotoAttempt(now time.Time) error {
	t.state.attempts = append(t.state.attempts, imageAttempt{t.actor, now})
	return nil
}
func (t *memoryClubImageTx) CreatePendingImage(record models.ClubImageRecord) error {
	t.state.records[record.ID] = record
	return nil
}
func (t *memoryClubImageTx) FinalizeImage(record models.ClubImageRecord) (models.ClubImageRecord, error) {
	if old, ok := t.state.records[record.ID]; !ok || old.State != "pending" {
		return record, database.ErrNotFound
	}
	record.Content = append([]byte{}, record.Content...)
	t.state.records[record.ID] = record
	record.Content = nil
	return record, nil
}
func (t *memoryClubImageTx) DeleteImage(id, state string) error {
	record, ok := t.state.records[id]
	if !ok || record.ClubID != t.repo.club.ID || record.State != state {
		return database.ErrNotFound
	}
	delete(t.state.records, id)
	return nil
}
func (t *memoryClubImageTx) SetImageState(state models.ClubImageState) error {
	t.state.state = state
	return nil
}

func processedClubImage(context.Context, io.Reader) (models.ProcessedPhoto, error) {
	return models.ProcessedPhoto{Bytes: []byte("normalized JPEG"), Width: 640, Height: 480}, nil
}
func readyClubImage(t *testing.T) (*Manager, *clubRepo, models.User, models.User, models.ClubImage) {
	t.Helper()
	m, repo, owner, member := clubFixture()
	m.photos = photos.New(processedClubImage)
	image, err := m.UploadImage(context.Background(), owner, repo.club.ID, 1, strings.NewReader("source"))
	if err != nil {
		t.Fatal(err)
	}
	return m, repo, owner, member, image
}

func TestClubImageProjectionAuthorityReplacementAndRemoval(t *testing.T) {
	m, repo, owner, member, image := readyClubImage(t)
	ctx := context.Background()
	id := repo.club.ID
	if image.Image == nil || image.Version == nil || *image.Version != 2 {
		t.Fatalf("owner image=%+v", image)
	}
	public, err := m.Image(ctx, id, nil)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(public)
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(encoded, &fields)
	if len(fields) != 1 || public.Version != nil {
		t.Fatalf("public metadata leaks fields: %s", encoded)
	}
	encoded, _ = json.Marshal(public.Image)
	fields = nil
	_ = json.Unmarshal(encoded, &fields)
	if len(fields) != 5 {
		t.Fatalf("photo DTO changed: %s", encoded)
	}
	_, err = m.UploadImage(ctx, member, id, 2, strings.NewReader("source"))
	requireFault(t, err, "forbidden")
	_, err = m.RemoveImage(ctx, member, id, 2)
	requireFault(t, err, "forbidden")
	m.photos = photos.New(func(context.Context, io.Reader) (models.ProcessedPhoto, error) {
		return models.ProcessedPhoto{}, models.Fail("invalid_photo", "Invalid image")
	})
	_, err = m.UploadImage(ctx, owner, id, 2, strings.NewReader("invalid"))
	requireFault(t, err, "invalid_photo")
	content, err := m.ImageContent(ctx, id, image.Image.ID, nil)
	if err != nil || string(content.Bytes) != "normalized JPEG" || repo.images.state.Version != 2 || len(repo.images.records) != 1 {
		t.Fatalf("failed replacement destroyed old image: %v", err)
	}
	m.photos = photos.New(processedClubImage)
	replaced, err := m.UploadImage(ctx, owner, id, 2, strings.NewReader("source"))
	if err != nil {
		t.Fatal(err)
	}
	if replaced.Image.ID == image.Image.ID || *replaced.Version != 3 || len(repo.images.records) != 1 {
		t.Fatal("replacement was not atomic")
	}
	_, err = m.ImageContent(ctx, id, image.Image.ID, nil)
	requireFault(t, err, "not_found")
	_, err = m.RemoveImage(ctx, owner, id, 2)
	requireFault(t, err, "stale_version")
	removed, err := m.RemoveImage(ctx, owner, id, 3)
	if err != nil || removed.Image != nil || *removed.Version != 4 || len(repo.images.records) != 0 {
		t.Fatalf("remove=%+v %v", removed, err)
	}
	_, err = m.UploadImage(ctx, owner, id, 0, strings.NewReader("source"))
	requireFault(t, err, "invalid_photo_choice")
}

func TestClubImageTransferPreservesContentWithoutUploaderAuthority(t *testing.T) {
	m, repo, owner, member, image := readyClubImage(t)
	ctx := context.Background()
	id := repo.club.ID
	repo.members[member.ID] = models.ClubMembership{State: "approved", AccountStatus: "active"}
	if _, err := m.Transfer(ctx, owner, id, models.ClubTransfer{Version: repo.club.Version, UserID: member.ID}); err != nil {
		t.Fatal(err)
	}
	if repo.images.records[image.Image.ID].OwnerID != owner.ID {
		t.Fatal("transfer changed quota attribution")
	}
	_, err := m.RemoveImage(ctx, owner, id, 2)
	requireFault(t, err, "forbidden")
	if _, err = m.ImageContent(ctx, id, image.Image.ID, &models.Session{User: owner}); err != nil {
		t.Fatal(err)
	}
	current, err := m.Image(ctx, id, &models.Session{User: member})
	if err != nil || current.Version == nil {
		t.Fatalf("new owner cannot manage image: %v", err)
	}
	if _, err = m.Close(ctx, member, id, repo.club.Version); err != nil {
		t.Fatal(err)
	}
	closed, err := m.Image(ctx, id, nil)
	if err != nil || closed.Image == nil || closed.Version != nil {
		t.Fatalf("closed image no longer public: %v", err)
	}
	_, err = m.RemoveImage(ctx, member, id, 2)
	requireFault(t, err, "club_closed")
	_, err = m.UploadImage(ctx, member, id, 2, strings.NewReader("source"))
	requireFault(t, err, "club_closed")
}

func TestClubImageFinalizeRevalidatesLifecycleAndPreservesOldImage(t *testing.T) {
	for _, change := range []string{"expired", "suspended", "transfer", "ABA", "close", "remove"} {
		t.Run(change, func(t *testing.T) {
			m, repo, owner, member, old := readyClubImage(t)
			ctx := context.Background()
			id := repo.club.ID
			repo.members[member.ID] = models.ClubMembership{State: "approved", AccountStatus: "active"}
			m.photos = photos.New(func(context.Context, io.Reader) (models.ProcessedPhoto, error) {
				visible, err := m.Image(ctx, id, nil)
				if err != nil || visible.Image.ID != old.Image.ID {
					t.Fatal("pending replacement replaced old image or ran under a lock")
				}
				switch change {
				case "expired":
					later := m.now().Add(photos.ReservationTTL)
					m.now = func() time.Time { return later }
				case "suspended":
					repo.members[owner.ID] = models.ClubMembership{State: "approved", AccountStatus: "suspended"}
				case "transfer", "ABA":
					if _, err = m.Transfer(ctx, owner, id, models.ClubTransfer{Version: repo.club.Version, UserID: member.ID}); err != nil {
						t.Fatal(err)
					}
					if change == "ABA" {
						if _, err = m.Transfer(ctx, member, id, models.ClubTransfer{Version: repo.club.Version, UserID: owner.ID}); err != nil {
							t.Fatal(err)
						}
					}
				case "close":
					if _, err = m.Close(ctx, owner, id, repo.club.Version); err != nil {
						t.Fatal(err)
					}
				case "remove":
					if _, err = m.RemoveImage(ctx, owner, id, 2); err != nil {
						t.Fatal(err)
					}
				}
				return processedClubImage(ctx, nil)
			})
			_, err := m.UploadImage(ctx, owner, id, 2, strings.NewReader("source"))
			want := map[string]string{"expired": "photo_upload_expired", "suspended": "forbidden", "transfer": "forbidden", "ABA": "stale_version", "close": "club_closed", "remove": "stale_version"}[change]
			requireFault(t, err, want)
			if change == "remove" {
				if len(repo.images.records) != 0 {
					t.Fatal("stale replacement resurrected removed image")
				}
			} else if len(repo.images.records) != 1 || repo.images.state.CurrentImageID != old.Image.ID {
				t.Fatal("failed finalization destroyed old content or retained pending quota")
			}
		})
	}
}

func TestClubImageCancellationAndLostCommitNeverDeleteReadyImage(t *testing.T) {
	m, repo, owner, _, old := readyClubImage(t)
	ctx, cancel := context.WithCancel(context.Background())
	m.photos = photos.New(func(context.Context, io.Reader) (models.ProcessedPhoto, error) {
		cancel()
		return models.ProcessedPhoto{}, context.Canceled
	})
	_, err := m.UploadImage(ctx, owner, repo.club.ID, 2, strings.NewReader("source"))
	if !errors.Is(err, context.Canceled) || len(repo.images.records) != 1 || repo.images.state.CurrentImageID != old.Image.ID {
		t.Fatalf("canceled cleanup changed ready content: %v", err)
	}
	m.photos = photos.New(processedClubImage)
	repo.images.lostReadyCommit = true
	_, err = m.UploadImage(context.Background(), owner, repo.club.ID, 2, strings.NewReader("source"))
	if err == nil {
		t.Fatal("expected uncertain commit error")
	}
	if len(repo.images.records) != 1 || repo.images.state.CurrentImageID == old.Image.ID || repo.images.state.Version != 3 {
		t.Fatal("cleanup removed a committed replacement")
	}
}

func TestClubImageCombinedQuotaAndAttemptDecisions(t *testing.T) {
	for _, kind := range []string{"owner", "global"} {
		t.Run(kind, func(t *testing.T) {
			m, repo, owner, _, old := readyClubImage(t)
			if kind == "owner" {
				repo.images.otherOwnerBytes = photos.OwnerByteLimit - int64(len("normalized JPEG"))
			} else {
				repo.images.otherGlobalBytes = photos.GlobalByteLimit - int64(len("normalized JPEG"))
			}
			_, err := m.UploadImage(context.Background(), owner, repo.club.ID, 2, strings.NewReader("source"))
			requireFault(t, err, "photo_storage_limit")
			if len(repo.images.records) != 1 || repo.images.state.CurrentImageID != old.Image.ID {
				t.Fatal("quota failure changed current image")
			}
		})
	}
	m, repo, owner, _ := clubFixture()
	m.photos = photos.New(func(context.Context, io.Reader) (models.ProcessedPhoto, error) {
		return models.ProcessedPhoto{}, models.Fail("invalid_photo", "Invalid image")
	})
	for range photos.HourlyAttempts {
		_, err := m.UploadImage(context.Background(), owner, repo.club.ID, 1, strings.NewReader("invalid"))
		requireFault(t, err, "invalid_photo")
	}
	_, err := m.UploadImage(context.Background(), owner, repo.club.ID, 1, strings.NewReader("source"))
	requireFault(t, err, "photo_rate_limited")
	if len(repo.images.records) != 0 || len(repo.images.attempts) != photos.HourlyAttempts {
		t.Fatal("decode failures must consume attempts and release reservations")
	}
}

func (t *memoryClubImageTx) ContentHidden(string, string) (bool, error) { return false, nil }
func (t *memoryClubImageTx) ActorMembership() (string, error) {
	return t.repo.members[t.actor].State, nil
}
