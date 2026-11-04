package accounts

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"testing"
	"time"

	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
)

var testNow = time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)

type accountRepository struct {
	challenge             models.OTPChallenge
	window                models.SendWindow
	account               models.Account
	locked                bool
	reserved              bool
	windowStart, dayStart time.Time
	committed             *models.SessionRecord
	oldDigest             []byte
	beforeCommit          func(*models.OTPChallenge, *models.Account)
	session               models.SessionRecord
	sessionErr            error
	touches               int
	revokes               int
	profile               models.User
}

func (r *accountRepository) ReserveChallenge(_ context.Context, challenge models.OTPChallenge, windowStart, dayStart time.Time, decide database.SendDecision) error {
	r.locked = true
	defer func() { r.locked = false }()
	if err := decide(r.window); err != nil {
		return err
	}
	r.challenge, r.windowStart, r.dayStart = challenge, windowStart, dayStart
	r.reserved = true
	return nil
}
func (r *accountRepository) UpdateChallenge(_ context.Context, id, expected, state, provider string) error {
	if r.challenge.ID != id || r.challenge.State != expected {
		return database.ErrConflict
	}
	r.challenge.State = state
	if provider != "" {
		r.challenge.ProviderID = provider
	}
	return nil
}
func (r *accountRepository) AttemptVerification(_ context.Context, id string, decide database.AttemptDecision) (models.OTPChallenge, error) {
	r.locked = true
	defer func() { r.locked = false }()
	if r.challenge.ID != id {
		return models.OTPChallenge{}, database.ErrNotFound
	}
	if err := decide(r.challenge); err != nil {
		return models.OTPChallenge{}, err
	}
	r.challenge.Attempts++
	return r.challenge, nil
}
func (r *accountRepository) CommitLogin(_ context.Context, _ string, proposed models.Account, record models.SessionRecord, oldDigest []byte, decide database.LoginDecision) (models.User, error) {
	r.locked = true
	defer func() { r.locked = false }()
	actual := r.account
	if actual.User.ID == "" {
		actual = proposed
	}
	if r.beforeCommit != nil {
		r.beforeCommit(&r.challenge, &actual)
	}
	if err := decide(r.challenge, actual); err != nil {
		return models.User{}, err
	}
	r.challenge.State = "consumed"
	r.challenge.ConsumedAt = &record.CreatedAt
	record.User = actual.User
	r.committed, r.oldDigest = &record, oldDigest
	return actual.User, nil
}
func (r *accountRepository) Session(context.Context, []byte) (models.SessionRecord, error) {
	return r.session, r.sessionErr
}
func (r *accountRepository) TouchSession(context.Context, []byte, time.Time) error {
	r.touches++
	return nil
}
func (r *accountRepository) RevokeSession(context.Context, []byte, time.Time) error {
	r.revokes++
	return nil
}
func (r *accountRepository) UpdateProfile(_ context.Context, _ string, decide database.ProfileDecision) (models.User, error) {
	user, err := decide(r.account)
	if err == nil {
		r.profile = user
		r.account.User = user
	}
	return user, err
}

type gateway struct {
	repo                   *accountRepository
	t                      *testing.T
	sendCalls, verifyCalls int
	sendResult             models.Verification
	verifyResult           models.Verification
	sendErr, verifyErr     error
	afterVerify            func()
}

func (g *gateway) Send(context.Context, string) (models.Verification, error) {
	g.t.Helper()
	if g.repo.locked || !g.repo.reserved {
		g.t.Fatal("provider send must follow committed reservation and run outside locks")
	}
	g.sendCalls++
	return g.sendResult, g.sendErr
}
func (g *gateway) Verify(_ context.Context, requestID, code string) (models.Verification, error) {
	g.t.Helper()
	if g.repo.locked || g.repo.challenge.Attempts == 0 {
		g.t.Fatal("provider verification must follow committed attempt and run outside locks")
	}
	if requestID != g.repo.challenge.ProviderID || code != "123456" {
		g.t.Fatal("provider verification is not bound to stored request")
	}
	g.verifyCalls++
	if g.afterVerify != nil {
		g.afterVerify()
	}
	return g.verifyResult, g.verifyErr
}

type wakeEvents struct{ calls int }

func (e *wakeEvents) WakeAll() { e.calls++ }

func verified(status string) models.Verification {
	return models.Verification{RequestID: "bound-request", Phone: "77011234567", VerificationStatus: &struct {
		Status string `json:"status"`
	}{Status: status}}
}
func fixture(t *testing.T) (*Manager, *accountRepository, *gateway) {
	t.Helper()
	repo := &accountRepository{challenge: models.OTPChallenge{ID: uuid.NewString(), Phone: "+77011234567", ProviderID: "bound-request", State: "pending", CreatedAt: testNow, ExpiresAt: testNow.Add(5 * time.Minute)}}
	client := &gateway{repo: repo, t: t, sendResult: verified(""), verifyResult: verified("code_valid")}
	return New(repo, client, nil, Options{Now: func() time.Time { return testNow }, DailyBudget: 1000}), repo, client
}
func faultCode(t *testing.T, err error, code string) {
	t.Helper()
	var fault *models.Fault
	if !errors.As(err, &fault) || fault.Code != code {
		t.Fatalf("got %v; want fault %s", err, code)
	}
}

func TestChallengeLimitsPrecedeProviderCalls(t *testing.T) {
	for name, window := range map[string]models.SendWindow{
		"cooldown": {Cooldown: true}, "phone": {PhoneSends: 3}, "source": {SourceSends: 30}, "daily": {Daily: 1000},
	} {
		t.Run(name, func(t *testing.T) {
			manager, repo, client := fixture(t)
			repo.window = window
			_, err := manager.Challenge(context.Background(), "+77011234567", "source")
			faultCode(t, err, "send_limited")
			if client.sendCalls != 0 || repo.reserved {
				t.Fatal("limited send reached provider or reserved a challenge")
			}
		})
	}
	manager, repo, client := fixture(t)
	result, err := manager.Challenge(context.Background(), "+7 701 1234567", "source")
	if err != nil {
		t.Fatal(err)
	}
	if client.sendCalls != 1 || repo.challenge.Phone != "+77011234567" || repo.challenge.State != "pending" || repo.challenge.ProviderID != "bound-request" {
		t.Fatal("challenge did not bind provider result")
	}
	if !result.ExpiresAt.Equal(testNow.Add(5*time.Minute)) || !result.ResendAfter.Equal(testNow.Add(time.Minute)) || !repo.windowStart.Equal(testNow.Add(-15*time.Minute)) || !repo.dayStart.Equal(testNow.Truncate(24*time.Hour)) {
		t.Fatal("challenge limits changed")
	}
}

func TestUncertainSendIsNotRetried(t *testing.T) {
	manager, repo, client := fixture(t)
	client.sendErr = errors.New("response lost")
	_, err := manager.Challenge(context.Background(), "+77011234567", "source")
	faultCode(t, err, "delivery_unavailable")
	if client.sendCalls != 1 || repo.challenge.State != "uncertain" {
		t.Fatal("billable send was retried or uncertainty was discarded")
	}
}

func TestProviderPhoneUsesInternationalCanonicalBinding(t *testing.T) {
	for _, raw := range []string{"+77011234567", "77011234567"} {
		got, err := models.NormalizeInternationalPhone(raw)
		if err != nil || got != "+77011234567" {
			t.Fatalf("international normalization failed: %v", err)
		}
	}
	for _, raw := range []string{"", "7011234567", "87011234567", "++77011234567", "+7 701 1234567", "not-a-phone"} {
		if _, err := models.NormalizeInternationalPhone(raw); err == nil {
			t.Errorf("malformed/national provider number accepted: %q", raw)
		}
	}
	for _, test := range []struct {
		value         models.Verification
		request, want string
	}{
		{models.Verification{Phone: "77011234567"}, "", "missing_request_id"},
		{models.Verification{RequestID: strings.Repeat("x", 513), Phone: "77011234567"}, "", "invalid_request_id"},
		{models.Verification{RequestID: "other", Phone: "77011234567"}, "bound", "request_id_mismatch"},
		{models.Verification{RequestID: "bound", Phone: "not-a-phone"}, "bound", "invalid_phone"},
		{models.Verification{RequestID: "bound", Phone: "77021234567"}, "bound", "phone_mismatch"},
		{models.Verification{RequestID: "bound", Phone: "77011234567"}, "bound", ""},
	} {
		if got := providerBindingFailure(test.value, "+77011234567", test.request); got != test.want {
			t.Errorf("diagnostic=%s want=%s", got, test.want)
		}
	}
}

func TestVerificationRequiresBoundValidStatus(t *testing.T) {
	for name, mutate := range map[string]func(*models.Verification){
		"missing status":  func(v *models.Verification) { v.VerificationStatus = nil },
		"invalid status":  func(v *models.Verification) { v.VerificationStatus.Status = "code_invalid" },
		"unknown status":  func(v *models.Verification) { v.VerificationStatus.Status = "delivered" },
		"missing request": func(v *models.Verification) { v.RequestID = "" },
		"wrong request":   func(v *models.Verification) { v.RequestID = "other" },
		"wrong phone":     func(v *models.Verification) { v.Phone = "77021234567" },
	} {
		t.Run(name, func(t *testing.T) {
			manager, repo, client := fixture(t)
			mutate(&client.verifyResult)
			_, err := manager.Verify(context.Background(), models.VerificationInput{ChallengeID: repo.challenge.ID, Code: "123456"}, "")
			faultCode(t, err, "invalid_code")
			if repo.committed != nil {
				t.Fatal("unverified response created session")
			}
		})
	}
	for _, terminal := range []string{"expired", "code_max_attempts_exceeded"} {
		manager, repo, client := fixture(t)
		client.verifyResult = verified(terminal)
		_, err := manager.Verify(context.Background(), models.VerificationInput{ChallengeID: repo.challenge.ID, Code: "123456"}, "")
		faultCode(t, err, "invalid_code")
		if repo.challenge.State != "failed" {
			t.Fatal("terminal provider state remains retryable")
		}
	}
}

func TestFifthAttemptAndAtomicSessionRotation(t *testing.T) {
	manager, repo, client := fixture(t)
	repo.challenge.Attempts = 4
	repo.account = models.Account{User: models.User{ID: uuid.NewString(), Profile: models.Profile{Locale: "ru"}}, Phone: repo.challenge.Phone, Status: "active"}
	result, err := manager.Verify(context.Background(), models.VerificationInput{ChallengeID: repo.challenge.ID, Code: "123456"}, "old-session")
	if err != nil {
		t.Fatal(err)
	}
	if client.verifyCalls != 1 || repo.challenge.Attempts != 5 || result.User.ID != repo.account.User.ID || repo.committed == nil || repo.challenge.State != "consumed" {
		t.Fatal("fifth attempt or existing account identity changed")
	}
	want := sha256.Sum256([]byte("old-session"))
	digest := sha256.Sum256([]byte(result.Token))
	if string(repo.oldDigest) != string(want[:]) || string(repo.committed.Digest) != string(digest[:]) || len(result.Token) != 43 || len(result.CSRF) != 43 || !result.CreatedAt.Equal(testNow) || !result.ExpiresAt.Equal(testNow.Add(30*24*time.Hour)) {
		t.Fatal("session rotation contract changed")
	}
	_, err = manager.Verify(context.Background(), models.VerificationInput{ChallengeID: repo.challenge.ID, Code: "123456"}, "")
	faultCode(t, err, "invalid_code")
	if client.verifyCalls != 1 {
		t.Fatal("consumed challenge reached provider")
	}
}

func TestCommitRevalidatesStateAfterExternalVerification(t *testing.T) {
	for name, change := range map[string]func(*models.OTPChallenge, *models.Account){
		"expired":           func(c *models.OTPChallenge, _ *models.Account) { c.ExpiresAt = testNow },
		"consumed":          func(c *models.OTPChallenge, _ *models.Account) { c.State = "consumed" },
		"phone rebound":     func(c *models.OTPChallenge, _ *models.Account) { c.Phone = "+77021234567" },
		"provider rebound":  func(c *models.OTPChallenge, _ *models.Account) { c.ProviderID = "other" },
		"suspended account": func(_ *models.OTPChallenge, a *models.Account) { a.Status = "suspended" },
	} {
		t.Run(name, func(t *testing.T) {
			manager, repo, _ := fixture(t)
			repo.beforeCommit = change
			_, err := manager.Verify(context.Background(), models.VerificationInput{ChallengeID: repo.challenge.ID, Code: "123456"}, "")
			want := "invalid_code"
			if name == "suspended account" {
				want = "account_unavailable"
			}
			faultCode(t, err, want)
			if repo.committed != nil {
				t.Fatal("stale validation created session")
			}
		})
	}
}

func TestAttemptBoundsPreventProviderCalls(t *testing.T) {
	for name, change := range map[string]func(*models.OTPChallenge){
		"expired":     func(c *models.OTPChallenge) { c.ExpiresAt = testNow },
		"exhausted":   func(c *models.OTPChallenge) { c.Attempts = 5 },
		"consumed":    func(c *models.OTPChallenge) { c.ConsumedAt = &testNow },
		"not pending": func(c *models.OTPChallenge) { c.State = "sending" },
	} {
		t.Run(name, func(t *testing.T) {
			manager, repo, client := fixture(t)
			change(&repo.challenge)
			_, err := manager.Verify(context.Background(), models.VerificationInput{ChallengeID: repo.challenge.ID, Code: "123456"}, "")
			faultCode(t, err, "invalid_code")
			if client.verifyCalls != 0 {
				t.Fatal("invalid challenge reached provider")
			}
		})
	}
}

func TestSessionPoliciesAndLogoutWake(t *testing.T) {
	for name, change := range map[string]func(*models.SessionRecord){
		"revoked":         func(s *models.SessionRecord) { s.RevokedAt = &testNow },
		"suspended":       func(s *models.SessionRecord) { s.UserStatus = "suspended" },
		"absolute expiry": func(s *models.SessionRecord) { s.ExpiresAt = testNow },
		"idle expiry":     func(s *models.SessionRecord) { s.LastSeenAt = testNow.Add(-7 * 24 * time.Hour) },
	} {
		t.Run(name, func(t *testing.T) {
			manager, repo, _ := fixture(t)
			repo.session = models.SessionRecord{UserStatus: "active", ExpiresAt: testNow.Add(time.Hour), LastSeenAt: testNow}
			change(&repo.session)
			session, err := manager.Session(context.Background(), strings.Repeat("a", 43), true)
			if err != nil || session != nil || repo.touches != 0 {
				t.Fatal("invalid session was accepted or touched")
			}
		})
	}
	manager, repo, _ := fixture(t)
	repo.session = models.SessionRecord{UserStatus: "active", ExpiresAt: testNow.Add(time.Hour), LastSeenAt: testNow}
	session, err := manager.Session(context.Background(), strings.Repeat("a", 43), true)
	if err != nil || session == nil || repo.touches != 1 || len(session.Digest) != 32 {
		t.Fatal("valid session missing")
	}
	events := &wakeEvents{}
	manager.events = events
	if err := manager.Logout(context.Background(), *session); err != nil || repo.revokes != 1 || events.calls != 1 {
		t.Fatal("logout did not revoke and wake connections")
	}
}

func TestProfileValidationDoesNotMutateCallerSlices(t *testing.T) {
	manager, repo, _ := fixture(t)
	languages := []string{" en "}
	user := models.User{ID: uuid.NewString(), Profile: models.Profile{DisplayName: "Name", City: "Almaty", Languages: languages, Locale: "en"}}
	repo.account = models.Account{User: user, Status: "active"}
	updated, err := manager.UpdateProfile(context.Background(), user, models.ProfilePatch{})
	if err != nil || !updated.OnboardingComplete || repo.profile.Profile.Languages[0] != "en" || languages[0] != " en " {
		t.Fatal("profile rules or input isolation changed")
	}
	bad := strings.Repeat("x", 161)
	_, err = manager.UpdateProfile(context.Background(), user, models.ProfilePatch{DisplayName: &bad})
	faultCode(t, err, "invalid_profile")
}

func TestProfileEditUsesCurrentStateAndRejectsRestriction(t *testing.T) {
	manager, repo, _ := fixture(t)
	stale := models.User{ID: uuid.NewString(), Profile: models.Profile{DisplayName: "Earlier", City: "Almaty", Languages: []string{"en"}, Locale: "en"}}
	current := stale
	current.Profile.Locale = "kk"
	repo.account = models.Account{User: current, Status: "active"}
	name := "Updated"
	updated, err := manager.UpdateProfile(context.Background(), stale, models.ProfilePatch{DisplayName: &name})
	if err != nil || updated.Profile.Locale != "kk" || updated.Profile.DisplayName != name {
		t.Fatal("profile edit overwrote current independent language setting")
	}
	repo.account.Status = "restricted"
	_, err = manager.UpdateProfile(context.Background(), updated, models.ProfilePatch{DisplayName: &name})
	faultCode(t, err, "account_unavailable")
}
