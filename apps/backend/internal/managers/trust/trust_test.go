package trust

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
)

func TestReportedTargetAccessKeepsPrivateMediaAndAvatarsScoped(t *testing.T) {
	for _, tc := range []struct {
		name   string
		record models.TrustTargetRecord
		want   bool
	}{
		{"private bike owner", models.TrustTargetRecord{Target: models.TrustTarget{Type: "bike"}, OwnerID: "viewer", Visibility: "private"}, true},
		{"private bike outsider", models.TrustTargetRecord{Target: models.TrustTarget{Type: "bike"}, OwnerID: "other", Visibility: "private"}, false},
		{"public bike", models.TrustTargetRecord{Target: models.TrustTarget{Type: "bike"}, OwnerID: "other", Visibility: "public"}, true},
		{"private route outsider", models.TrustTargetRecord{Target: models.TrustTarget{Type: "route"}, OwnerID: "other", Visibility: "private", AuthorStatus: "active", Current: true}, false},
		{"public route", models.TrustTargetRecord{Target: models.TrustTarget{Type: "route"}, OwnerID: "other", Visibility: "public", AuthorStatus: "active", Current: true}, true},
		{"restricted route author", models.TrustTargetRecord{Target: models.TrustTarget{Type: "route"}, OwnerID: "other", Visibility: "public", AuthorStatus: "suspended", Current: true}, false},
		{"unavailable condition context", models.TrustTargetRecord{Target: models.TrustTarget{Type: "route_condition"}, OwnerID: "other", Visibility: "public", AuthorStatus: "active", Current: false}, false},
		{"public condition", models.TrustTargetRecord{Target: models.TrustTarget{Type: "route_condition"}, OwnerID: "other", Visibility: "public", AuthorStatus: "active", Current: true}, true},
		{"pending bike image", models.TrustTargetRecord{Target: models.TrustTarget{Type: "bike_photo"}, OwnerID: "viewer", State: "pending"}, false},
		{"private ready image outsider", models.TrustTargetRecord{Target: models.TrustTarget{Type: "bike_photo"}, OwnerID: "other", State: "ready", Visibility: "private"}, false},
		{"old club image", models.TrustTargetRecord{Target: models.TrustTarget{Type: "club_image"}, State: "ready", Current: false}, false},
		{"current club image", models.TrustTargetRecord{Target: models.TrustTarget{Type: "club_image"}, State: "ready", Current: true}, true},
		{"pending ride cover", models.TrustTargetRecord{Target: models.TrustTarget{Type: "ride_cover"}, OwnerID: "viewer", State: "pending", Current: false}, false},
		{"old ride cover", models.TrustTargetRecord{Target: models.TrustTarget{Type: "ride_cover"}, OwnerID: "other", State: "ready", Current: false}, false},
		{"current ride cover outsider", models.TrustTargetRecord{Target: models.TrustTarget{Type: "ride_cover"}, OwnerID: "other", State: "ready", Current: true}, true},
		{"avatar outsider", models.TrustTargetRecord{Target: models.TrustTarget{Type: "avatar"}, OwnerID: "other", State: "ready", Current: true, AuthorStatus: "active"}, false},
		{"avatar own", models.TrustTargetRecord{Target: models.TrustTarget{Type: "avatar"}, OwnerID: "viewer", State: "ready", Current: true, AuthorStatus: "active"}, true},
		{"avatar shared ride", models.TrustTargetRecord{Target: models.TrustTarget{Type: "avatar"}, OwnerID: "other", State: "ready", Current: true, AuthorStatus: "active", SharedRides: 1}, true},
		{"avatar shared club", models.TrustTargetRecord{Target: models.TrustTarget{Type: "avatar"}, OwnerID: "other", State: "ready", Current: true, AuthorStatus: "active", SharedClubs: 1}, true},
		{"suspended avatar", models.TrustTargetRecord{Target: models.TrustTarget{Type: "avatar"}, OwnerID: "other", State: "ready", Current: true, AuthorStatus: "suspended", SharedClubs: 1}, false},
		{"pending ride video", models.TrustTargetRecord{Target: models.TrustTarget{Type: "ride_video"}, OwnerID: "other", Participation: "requested"}, false},
		{"approved ride photo", models.TrustTargetRecord{Target: models.TrustTarget{Type: "ride_photo"}, OwnerID: "other", Participation: "approved", State: "ready"}, true},
		{"organizer message", models.TrustTargetRecord{Target: models.TrustTarget{Type: "message"}, OwnerID: "viewer"}, true},
		{"approved announcement", models.TrustTargetRecord{Target: models.TrustTarget{Type: "announcement"}, OwnerID: "other", Participation: "approved"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := reportAllowed(tc.record, "viewer"); got != tc.want {
				t.Fatalf("report access=%v want%v", got, tc.want)
			}
		})
	}
}
func TestTrustCursorHasCanonicalBoundedEncoding(t *testing.T) {
	c := models.TrustCursor{ID: uuid.NewString(), CreatedAt: time.Date(2026, 11, 3, 9, 0, 0, 123000, time.UTC)}
	encoded := encodeCursor(c)
	decoded, err := parseCursor(encoded)
	if err != nil || decoded.ID != c.ID || !decoded.CreatedAt.Equal(c.CreatedAt) {
		t.Fatalf("cursor roundtrip: %+v %v", decoded, err)
	}
	for _, bad := range []string{"not-a-cursor", encoded + "=", strings.Repeat("A", 257)} {
		if _, err := parseCursor(bad); err == nil {
			t.Fatal("ambiguous/unbounded cursor accepted")
		}
	}
}

type decisionRepo struct {
	database.TrustRepository
	tx          database.TrustTransaction
	commitError error
	calls       int
}

func (r *decisionRepo) WithTrust(_ context.Context, _ string, fn database.TrustDecision) error {
	r.calls++
	if err := fn(r.tx); err != nil {
		return err
	}
	return r.commitError
}

type blockTx struct{ database.TrustTransaction }

func (t blockTx) Account(id string) (models.TrustAccount, error) {
	return models.TrustAccount{ID: id, Status: "active"}, nil
}
func (t blockTx) SetBlock(_, _ string, _ bool, _ time.Time) error { return nil }

type eventRecorder struct {
	wakes int
	types []string
}

func (e *eventRecorder) WakeAll()               { e.wakes++ }
func (e *eventRecorder) PublishAll(kind string) { e.types = append(e.types, kind) }
func TestTrustHintsPublishOnlyAfterCommittedMutation(t *testing.T) {
	actor := models.User{ID: uuid.NewString()}
	target := uuid.NewString()
	events := &eventRecorder{}
	repo := &decisionRepo{tx: blockTx{}, commitError: errors.New("commit uncertain")}
	m := New(repo, events, nil)
	if _, err := m.Block(context.Background(), actor, target, false); err == nil {
		t.Fatal("commit failure hidden")
	}
	if events.wakes != 0 || len(events.types) != 0 {
		t.Fatal("uncommitted trust state was published")
	}
	repo.commitError = nil
	if _, err := m.Block(context.Background(), actor, target, false); err != nil {
		t.Fatal(err)
	}
	if events.wakes != 1 || len(events.types) != 1 || events.types[0] != "trust.changed" {
		t.Fatal("committed block did not invalidate foreground state")
	}
}
func TestInvalidAdminPayloadNeverEntersTransaction(t *testing.T) {
	repo := &decisionRepo{}
	m := New(repo, nil, nil)
	actor := models.User{ID: uuid.NewString()}
	for _, input := range []models.TrustActionInput{
		{Action: "assign_badge", TargetType: "user", TargetID: uuid.NewString(), Reason: "review"},
		{ClientActionID: uuid.NewString(), Action: "assign_badge", TargetType: "message", TargetID: uuid.NewString(), Reason: "review"},
		{ClientActionID: uuid.NewString(), Action: "restrict_account", TargetType: "user", TargetID: uuid.NewString(), Reason: " "},
		{ClientActionID: uuid.NewString(), Action: "grant_admin", TargetType: "user", TargetID: uuid.NewString(), Reason: "review"},
	} {
		if _, err := m.Action(context.Background(), actor, input); err == nil {
			t.Fatal("invalid admin input accepted")
		}
	}
	if repo.calls != 0 {
		t.Fatal("invalid payload entered persistence")
	}
}

type hiddenContextTx struct {
	database.TrustTransaction
	hidden map[string]bool
}

func (t hiddenContextTx) Hidden(kind, id string) (bool, error) { return t.hidden[kind+":"+id], nil }
func TestReportContextRejectsGuessedChildrenOfUnavailableParents(t *testing.T) {
	tx := hiddenContextTx{hidden: map[string]bool{"bike:parent": true, "club:parent": true, "ride:parent": true, "user:author": true}}
	for _, tc := range []struct {
		name    string
		record  models.TrustTargetRecord
		allowed bool
	}{
		{"hidden bike outsider", models.TrustTargetRecord{Target: models.TrustTarget{Type: "bike_photo", ScopeID: "parent"}, OwnerID: "other"}, false},
		{"hidden bike owner", models.TrustTargetRecord{Target: models.TrustTarget{Type: "bike_photo", ScopeID: "parent"}, OwnerID: "viewer"}, true},
		{"hidden club outsider", models.TrustTargetRecord{Target: models.TrustTarget{Type: "club_image", ScopeID: "parent"}, OwnerID: "other"}, false},
		{"hidden club approved member", models.TrustTargetRecord{Target: models.TrustTarget{Type: "club_image", ScopeID: "parent"}, OwnerID: "other", Membership: "approved"}, true},
		{"hidden club owner", models.TrustTargetRecord{Target: models.TrustTarget{Type: "club_image", ScopeID: "parent"}, OwnerID: "viewer"}, true},
		{"hidden user avatar", models.TrustTargetRecord{Target: models.TrustTarget{Type: "avatar", AuthorID: "author"}, OwnerID: "viewer"}, false},
		{"approved ride coordination", models.TrustTargetRecord{Target: models.TrustTarget{Type: "ride_photo", ScopeID: "parent"}, Participation: "approved"}, true},
		{"hidden ride cover outsider", models.TrustTargetRecord{Target: models.TrustTarget{Type: "ride_cover", ScopeID: "parent"}, OwnerID: "other", Participation: "requested"}, false},
		{"hidden ride cover approved rider", models.TrustTargetRecord{Target: models.TrustTarget{Type: "ride_cover", ScopeID: "parent"}, OwnerID: "other", Participation: "approved"}, true},
		{"hidden ride cover organizer", models.TrustTargetRecord{Target: models.TrustTarget{Type: "ride_cover", ScopeID: "parent"}, OwnerID: "viewer"}, true},
		{"visible ride cover outsider", models.TrustTargetRecord{Target: models.TrustTarget{Type: "ride_cover", ScopeID: "visible"}, OwnerID: "other"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := reportContextVisible(tx, tc.record, "viewer")
			if (err == nil) != tc.allowed {
				t.Fatalf("allowed=%v err=%v", tc.allowed, err)
			}
		})
	}
}

// reportTx answers one ride-cover report. The reporter and the organizer
// block each other, which must not stop reports of public content.
type reportTx struct {
	database.TrustTransaction
	target   models.TrustTargetRecord
	hidden   map[string]bool
	inserted []models.Report
}

func (t *reportTx) Account(id string) (models.TrustAccount, error) {
	return models.TrustAccount{ID: id, Status: "active"}, nil
}
func (t *reportTx) Target(kind, id, _ string, _ bool) (models.TrustTargetRecord, error) {
	if kind != t.target.Target.Type || id != t.target.Target.ID {
		return models.TrustTargetRecord{}, database.ErrNotFound
	}
	return t.target, nil
}
func (t *reportTx) Hidden(kind, id string) (bool, error) { return t.hidden[kind+":"+id], nil }
func (t *reportTx) Blocked(string, string) (bool, error) { return true, nil }
func (t *reportTx) OpenReport(string, string, string) (models.Report, error) {
	return models.Report{}, database.ErrNotFound
}
func (t *reportTx) ReportCount(string, time.Time) (int, error) { return 0, nil }
func (t *reportTx) InsertReport(r models.Report) error {
	t.inserted = append(t.inserted, r)
	return nil
}

func TestRideCoverIsReportableHideableAndReviewablePublicContent(t *testing.T) {
	if !validKind("ride_cover") || !imageKind("ride_cover") {
		t.Fatal("ride covers must be reportable, hideable and reviewable images")
	}
	cover, ride := uuid.NewString(), uuid.NewString()
	record := models.TrustTargetRecord{Target: models.TrustTarget{Type: "ride_cover", ID: cover, AuthorID: "organizer", ScopeID: ride, MediaURL: "/api/v1/rides/" + ride + "/cover/" + cover + "/content"}, OwnerID: "organizer", State: "ready", Current: true}
	actor := models.User{ID: uuid.NewString()}
	input := models.ReportInput{TargetType: "ride_cover", TargetID: cover, Reason: "spam"}
	tx := &reportTx{target: record, hidden: map[string]bool{}}
	m := New(&decisionRepo{tx: tx}, nil, nil)
	if _, err := m.Report(context.Background(), actor, input); err != nil || len(tx.inserted) != 1 || tx.inserted[0].Snapshot.MediaURL != record.Target.MediaURL {
		t.Fatalf("blocked rider could not report a public ride cover: %v", err)
	}
	for _, hidden := range []string{"ride_cover:" + cover, "ride:" + ride} {
		tx = &reportTx{target: record, hidden: map[string]bool{hidden: true}}
		m = New(&decisionRepo{tx: tx}, nil, nil)
		if _, err := m.Report(context.Background(), actor, input); err == nil || len(tx.inserted) != 0 {
			t.Fatalf("report revealed a cover behind %s", hidden)
		}
	}
}
