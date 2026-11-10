package notifications

import (
	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"context"
	"errors"
	"github.com/google/uuid"
	"strings"
	"testing"
	"time"
)

type diagnosticRepository struct {
	database.NotificationRepository
	tx        *diagnosticTransaction
	inside    bool
	calls     int
	err       error
	accountID string
}

func (r *diagnosticRepository) WithNotifications(ctx context.Context, id string, decide database.NotificationDecision) error {
	r.calls++
	r.accountID = id
	if err := ctx.Err(); err != nil {
		return err
	}
	r.inside = true
	defer func() { r.inside = false }()
	if r.err != nil {
		return r.err
	}
	return decide(r.tx)
}

type diagnosticTransaction struct {
	database.NotificationTransaction
	account                       models.PushAccount
	rows                          []models.PushSubscriptionRecord
	preferenceWrites, unreadCount int
	err                           error
	bindingTimes                  []time.Time
	writtenSubscriptions          []models.PushSubscriptionRecord
}

func (t *diagnosticTransaction) SetPreferences(p models.Preferences) error {
	t.preferenceWrites++
	t.account.Preferences = p
	return nil
}
func (t *diagnosticTransaction) UnreadCount() (int, error)   { return t.unreadCount, t.err }
func (t *diagnosticTransaction) Account() models.PushAccount { return t.account }
func (t *diagnosticTransaction) Subscriptions(at time.Time) ([]models.PushSubscriptionRecord, error) {
	t.bindingTimes = append(t.bindingTimes, at)
	return t.rows, t.err
}
func (t *diagnosticTransaction) SubscriptionByEndpoint(_ string, at time.Time) (models.PushSubscriptionRecord, error) {
	t.bindingTimes = append(t.bindingTimes, at)
	return models.PushSubscriptionRecord{}, database.ErrNotFound
}
func (t *diagnosticTransaction) PutSubscription(record models.PushSubscriptionRecord) (models.PushSubscriptionRecord, error) {
	t.writtenSubscriptions = append(t.writtenSubscriptions, record)
	return record, nil
}

type diagnosticGateway struct {
	repo          *diagnosticRepository
	t             *testing.T
	calls, status int
	err           error
	input         models.PushSubscriptionInput
	payload       models.PushPayload
}

func (g *diagnosticGateway) Send(ctx context.Context, input models.PushSubscriptionInput, payload models.PushPayload) (int, error) {
	g.t.Helper()
	if g.repo.inside {
		g.t.Fatal("diagnostic network request ran inside transaction callback")
	}
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) <= 0 || time.Until(deadline) > 10*time.Second {
		g.t.Fatal("diagnostic request must have a bounded ten-second context")
	}
	g.calls++
	g.input, g.payload = input, payload
	return g.status, g.err
}

func diagnosticFixture(t *testing.T) (*Manager, *diagnosticRepository, *diagnosticGateway, string, string) {
	t.Helper()
	account, subscription := uuid.NewString(), uuid.NewString()
	repo := &diagnosticRepository{tx: &diagnosticTransaction{
		account: models.PushAccount{ID: account, Status: "active", Preferences: models.Preferences{Locale: "kk"}},
		rows:    []models.PushSubscriptionRecord{{PushSubscription: models.PushSubscription{ID: subscription}, UserID: account, Endpoint: "https://web.push.apple.com/private-endpoint", P256dh: "private-browser-key", Auth: "private-browser-auth", SessionDigest: []byte("private-session"), BindingActive: true}},
	}}
	gateway := &diagnosticGateway{repo: repo, t: t, status: 201}
	return New(repo, gateway, "configured-public-key", nil), repo, gateway, account, subscription
}

func diagnosticFault(t *testing.T, err error, code string) {
	t.Helper()
	var fault *models.Fault
	if !errors.As(err, &fault) || fault.Code != code {
		t.Fatalf("expected %s, got %v", code, err)
	}
	for _, secret := range []string{"private-endpoint", "private-browser-key", "private-browser-auth", "private-session", "+77011239999"} {
		if strings.Contains(err.Error(), secret) {
			t.Fatal("diagnostic error exposed private adapter data")
		}
	}
}

func TestPushDiagnosticIsScopedGenericAndOutsideTransaction(t *testing.T) {
	m, repo, gateway, account, subscription := diagnosticFixture(t)
	if err := m.TestPush(context.Background(), account, subscription); err != nil {
		t.Fatal(err)
	}
	if repo.accountID != account || repo.calls != 1 || gateway.calls != 1 || gateway.input.Endpoint != repo.tx.rows[0].Endpoint || gateway.input.Keys.P256dh != repo.tx.rows[0].P256dh || gateway.input.Keys.Auth != repo.tx.rows[0].Auth {
		t.Fatal("diagnostic did not use exactly the selected account-owned subscription")
	}
	payload := gateway.payload
	if payload.Title != "BIKERS.KZ" || payload.Body != genericBody("kk") || payload.URL != "/inbox" {
		t.Fatal("diagnostic payload was not generic and localized")
	}
	if _, err := uuid.Parse(payload.NotificationID); err != nil {
		t.Fatal("diagnostic did not supply a valid notification tag UUID")
	}
	first := payload.NotificationID
	if err := m.TestPush(context.Background(), account, subscription); err != nil || gateway.payload.NotificationID == first || gateway.calls != 2 {
		t.Fatal("each explicitly invoked diagnostic must send once with a fresh tag")
	}
}

func TestPushDiagnosticRejectsWrongAccountSubscriptionAndInactiveBinding(t *testing.T) {
	for _, change := range []func(*diagnosticRepository, *string){
		func(r *diagnosticRepository, _ *string) { r.tx.account.Status = "suspended" },
		func(r *diagnosticRepository, _ *string) { r.tx.account.ID = uuid.NewString() },
		func(r *diagnosticRepository, _ *string) { r.tx.rows[0].UserID = uuid.NewString() },
		func(r *diagnosticRepository, _ *string) { r.tx.rows[0].BindingActive = false },
		func(r *diagnosticRepository, _ *string) { r.tx.rows = nil },
		func(_ *diagnosticRepository, id *string) { *id = uuid.NewString() },
	} {
		m, repo, gateway, account, subscription := diagnosticFixture(t)
		change(repo, &subscription)
		diagnosticFault(t, m.TestPush(context.Background(), account, subscription), "push_test_unavailable")
		if gateway.calls != 0 {
			t.Fatal("diagnostic contacted provider for inaccessible or inactive binding")
		}
	}
}

func TestPushDiagnosticRejectsDisabledAndInvalidInputsBeforeRepository(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		m, repo, gateway, account, subscription := diagnosticFixture(t)
		if enabled {
			m.publicKey = ""
		} else {
			m.gateway = nil
		}
		diagnosticFault(t, m.TestPush(context.Background(), account, subscription), "push_unavailable")
		if repo.calls != 0 || gateway.calls != 0 {
			t.Fatal("disabled push reached persistence or provider")
		}
	}
	for _, badAccount := range []bool{false, true} {
		m, repo, gateway, account, subscription := diagnosticFixture(t)
		if badAccount {
			account = "not-a-uuid"
		} else {
			subscription = "not-a-uuid"
		}
		diagnosticFault(t, m.TestPush(context.Background(), account, subscription), "invalid_push_subscription")
		if repo.calls != 0 || gateway.calls != 0 {
			t.Fatal("malformed diagnostic reached persistence or provider")
		}
	}
}

func TestPushDiagnosticFailuresAreSafeAndNeverRetried(t *testing.T) {
	for _, status := range []int{0, 199, 301, 400, 403, 410, 429, 500} {
		m, _, gateway, account, subscription := diagnosticFixture(t)
		gateway.status = status
		diagnosticFault(t, m.TestPush(context.Background(), account, subscription), "push_test_failed")
		if gateway.calls != 1 {
			t.Fatal("operator diagnostic unexpectedly retried")
		}
	}
	m, repo, gateway, account, subscription := diagnosticFixture(t)
	privateError := errors.New("private-endpoint private-browser-key private-browser-auth private-session +77011239999")
	gateway.err = privateError
	diagnosticFault(t, m.TestPush(context.Background(), account, subscription), "push_test_failed")
	repo.err = privateError
	diagnosticFault(t, m.TestPush(context.Background(), account, subscription), "push_test_unavailable")
	if gateway.calls != 1 {
		t.Fatal("repository failure still contacted provider")
	}
	repo.err = nil
	repo.tx.err = privateError
	diagnosticFault(t, m.TestPush(context.Background(), account, subscription), "push_test_unavailable")
	if gateway.calls != 1 {
		t.Fatal("subscription lookup failure still contacted provider")
	}
}

func TestPushDiagnosticCancelledContextDoesNotSend(t *testing.T) {
	m, _, gateway, account, subscription := diagnosticFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	diagnosticFault(t, m.TestPush(ctx, account, subscription), "push_test_unavailable")
	if gateway.calls != 0 {
		t.Fatal("cancelled diagnostic contacted provider")
	}
}
