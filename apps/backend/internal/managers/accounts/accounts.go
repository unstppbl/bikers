// Package accounts owns account, authentication, session, and profile rules.
package accounts

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"log"
	"strings"
	"time"

	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
)

const (
	challengeTTL             = 5 * time.Minute
	resendCooldown           = time.Minute
	sendWindow               = 15 * time.Minute
	phoneSendLimit           = 3
	sourceSendLimit          = 30
	verificationAttemptLimit = 5
	sessionLifetime          = 30 * 24 * time.Hour
	sessionIdleLifetime      = 7 * 24 * time.Hour
)

// Gateway describes the external verification behavior needed by accounts.
type Gateway interface {
	Send(context.Context, string) (models.Verification, error)
	Verify(context.Context, string, string) (models.Verification, error)
}

type Events interface{ WakeAll() }

type Options struct {
	Now         func() time.Time
	DailyBudget int
}

type Manager struct {
	repo        database.AccountsRepository
	gateway     Gateway
	events      Events
	now         func() time.Time
	dailyBudget int
}

func New(repo database.AccountsRepository, gateway Gateway, events Events, opts Options) *Manager {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.DailyBudget <= 0 {
		opts.DailyBudget = 1000
	}
	return &Manager{repo: repo, gateway: gateway, events: events, now: opts.Now, dailyBudget: opts.DailyBudget}
}

func (m *Manager) Challenge(ctx context.Context, rawPhone, sourceKey string) (models.ChallengeResult, error) {
	phone, err := models.NormalizePhone(rawPhone, "KZ")
	if err != nil {
		return models.ChallengeResult{}, models.Fail("invalid_phone", "Enter a valid phone number")
	}
	now := m.now().UTC()
	challenge := models.OTPChallenge{
		ID: uuid.NewString(), Phone: phone, SourceKey: sourceKey, State: "sending",
		CreatedAt: now, ExpiresAt: now.Add(challengeTTL), ResendAfter: now.Add(resendCooldown),
	}
	err = m.repo.ReserveChallenge(ctx, challenge, now.Add(-sendWindow), now.Truncate(24*time.Hour), func(window models.SendWindow) error {
		if window.Cooldown || window.PhoneSends >= phoneSendLimit || window.SourceSends >= sourceSendLimit || window.Daily >= m.dailyBudget {
			return models.Fail("send_limited", "Please wait before requesting another code")
		}
		return nil
	})
	if err != nil {
		return models.ChallengeResult{}, err
	}

	// Reservation is committed before any billable network request.
	verification, err := m.gateway.Send(ctx, phone)
	if err != nil {
		_ = m.repo.UpdateChallenge(ctx, challenge.ID, "sending", "uncertain", "")
		return models.ChallengeResult{}, models.Fail("delivery_unavailable", "Code delivery unavailable; wait before retrying")
	}
	if reason := providerBindingFailure(verification, phone, ""); reason != "" {
		log.Printf("telegram_response_validation_failed method=send reason=%s", reason)
		_ = m.repo.UpdateChallenge(ctx, challenge.ID, "sending", "failed", "")
		return models.ChallengeResult{}, models.Fail("delivery_unavailable", "Code delivery unavailable; wait before retrying")
	}
	if err := m.repo.UpdateChallenge(ctx, challenge.ID, "sending", "pending", verification.RequestID); err != nil {
		return models.ChallengeResult{}, err
	}
	return models.ChallengeResult{ID: challenge.ID, ExpiresAt: challenge.ExpiresAt, ResendAfter: challenge.ResendAfter}, nil
}

func (m *Manager) Verify(ctx context.Context, input models.VerificationInput, oldToken string) (models.LoginResult, error) {
	if _, err := uuid.Parse(input.ChallengeID); err != nil || !validCode(input.Code) {
		return models.LoginResult{}, invalidCode()
	}
	challenge, err := m.repo.AttemptVerification(ctx, input.ChallengeID, func(current models.OTPChallenge) error {
		if !pendingChallenge(current, m.now().UTC()) || current.Attempts >= verificationAttemptLimit {
			return invalidCode()
		}
		return nil
	})
	if errors.Is(err, database.ErrNotFound) {
		return models.LoginResult{}, invalidCode()
	}
	if err != nil {
		return models.LoginResult{}, err
	}

	// The committed attempt reservation releases its lock before verification.
	verification, err := m.gateway.Verify(ctx, challenge.ProviderID, input.Code)
	if err != nil {
		return models.LoginResult{}, models.Fail("verification_unavailable", "Verification unavailable; try again")
	}
	bindingFailure := providerBindingFailure(verification, challenge.Phone, challenge.ProviderID)
	if bindingFailure != "" {
		log.Printf("telegram_response_validation_failed method=verify reason=%s", bindingFailure)
	}
	if bindingFailure != "" || verification.VerificationStatus == nil || verification.VerificationStatus.Status != "code_valid" {
		if verification.VerificationStatus != nil && (verification.VerificationStatus.Status == "expired" || verification.VerificationStatus.Status == "code_max_attempts_exceeded") {
			_ = m.repo.UpdateChallenge(ctx, challenge.ID, "pending", "failed", "")
		}
		return models.LoginResult{}, invalidCode()
	}
	token, err := randomToken()
	if err != nil {
		return models.LoginResult{}, err
	}
	csrf, err := randomToken()
	if err != nil {
		return models.LoginResult{}, err
	}
	now := m.now().UTC()
	digest := sha256.Sum256([]byte(token))
	proposed := models.Account{
		User:  models.User{ID: uuid.NewString(), Profile: models.Profile{Languages: []string{}, Locale: "en"}},
		Phone: challenge.Phone, Status: "active", CreatedAt: now,
	}
	record := models.SessionRecord{Session: models.Session{User: proposed.User, CSRF: csrf, Digest: digest[:]}, CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(sessionLifetime)}
	var oldDigest []byte
	if oldToken != "" {
		value := sha256.Sum256([]byte(oldToken))
		oldDigest = value[:]
	}
	user, err := m.repo.CommitLogin(ctx, challenge.ID, proposed, record, oldDigest, func(current models.OTPChallenge, account models.Account) error {
		// The repository locks the current challenge and actual account. Never
		// rely only on the snapshot obtained before the external call.
		if !pendingChallenge(current, m.now().UTC()) || current.Attempts > verificationAttemptLimit || current.Phone != challenge.Phone || current.ProviderID != challenge.ProviderID || account.Phone != challenge.Phone {
			return invalidCode()
		}
		if providerBindingFailure(verification, current.Phone, current.ProviderID) != "" || verification.VerificationStatus == nil || verification.VerificationStatus.Status != "code_valid" {
			return invalidCode()
		}
		if account.Status != "active" {
			return models.Fail("account_unavailable", "Account unavailable")
		}
		return nil
	})
	if errors.Is(err, database.ErrNotFound) || errors.Is(err, database.ErrConflict) {
		return models.LoginResult{}, invalidCode()
	}
	if err != nil {
		return models.LoginResult{}, err
	}
	if user.Profile.Languages == nil {
		user.Profile.Languages = []string{}
	}
	return models.LoginResult{User: user, CSRF: csrf, Token: token, CreatedAt: now, ExpiresAt: record.ExpiresAt}, nil
}

func (m *Manager) Session(ctx context.Context, token string, touch bool) (*models.Session, error) {
	if len(token) != 43 {
		return nil, nil
	}
	digest := sha256.Sum256([]byte(token))
	record, err := m.repo.Session(ctx, digest[:])
	if errors.Is(err, database.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	now := m.now().UTC()
	if record.RevokedAt != nil || record.UserStatus != "active" || !record.ExpiresAt.After(now) || !record.LastSeenAt.After(now.Add(-sessionIdleLifetime)) {
		return nil, nil
	}
	if touch {
		if err := m.repo.TouchSession(ctx, digest[:], now); err != nil {
			return nil, err
		}
	}
	session := record.Session
	session.Digest = digest[:]
	if session.User.Profile.Languages == nil {
		session.User.Profile.Languages = []string{}
	}
	return &session, nil
}

func (m *Manager) Logout(ctx context.Context, session models.Session) error {
	if err := m.repo.RevokeSession(ctx, session.Digest, m.now().UTC()); err != nil {
		return err
	}
	if m.events != nil {
		m.events.WakeAll()
	}
	return nil
}

func (m *Manager) UpdateProfile(ctx context.Context, user models.User, patch models.ProfilePatch) (models.User, error) {
	updated, err := m.repo.UpdateProfile(ctx, user.ID, func(account models.Account) (models.User, error) {
		if account.Status != "active" {
			return models.User{}, models.Fail("account_unavailable", "Account unavailable")
		}
		return updateProfile(account.User, patch)
	})
	if errors.Is(err, database.ErrNotFound) {
		return models.User{}, models.Fail("not_found", "Record not found")
	}
	return updated, err
}

func updateProfile(user models.User, patch models.ProfilePatch) (models.User, error) {
	if patch.DisplayName != nil {
		user.Profile.DisplayName = strings.TrimSpace(*patch.DisplayName)
	}
	if patch.City != nil {
		user.Profile.City = strings.TrimSpace(*patch.City)
	}
	if patch.Languages != nil {
		user.Profile.Languages = append([]string{}, (*patch.Languages)...)
	} else {
		user.Profile.Languages = append([]string{}, user.Profile.Languages...)
	}
	if patch.Interests != nil {
		user.Profile.Interests = strings.TrimSpace(*patch.Interests)
	}
	if patch.Locale != nil {
		user.Profile.Locale = *patch.Locale
	}
	if len(user.Profile.DisplayName) > 160 || len(user.Profile.City) > 160 || len(user.Profile.Interests) > 4000 || !models.ValidLocale(user.Profile.Locale) || len(user.Profile.Languages) > 10 {
		return models.User{}, models.Fail("invalid_profile", "Check profile fields")
	}
	for index, language := range user.Profile.Languages {
		language = strings.TrimSpace(language)
		if language == "" || len(language) > 64 {
			return models.User{}, models.Fail("invalid_profile", "Check profile languages")
		}
		user.Profile.Languages[index] = language
	}
	user.OnboardingComplete = user.Profile.DisplayName != "" && user.Profile.City != "" && len(user.Profile.Languages) > 0
	return user, nil
}

func pendingChallenge(challenge models.OTPChallenge, now time.Time) bool {
	return challenge.State == "pending" && challenge.ConsumedAt == nil && challenge.ExpiresAt.After(now) && challenge.ProviderID != ""
}

func invalidCode() error { return models.Fail("invalid_code", "Invalid or expired code") }

func validCode(code string) bool {
	if len(code) != 6 {
		return false
	}
	for _, digit := range code {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

// Fixed diagnostic values reveal neither a provider payload nor identity data.
func providerBindingFailure(value models.Verification, phone, requestID string) string {
	if value.RequestID == "" {
		return "missing_request_id"
	}
	if len(value.RequestID) > 512 {
		return "invalid_request_id"
	}
	if requestID != "" && value.RequestID != requestID {
		return "request_id_mismatch"
	}
	canonical, err := models.NormalizeInternationalPhone(value.Phone)
	if err != nil {
		return "invalid_phone"
	}
	if canonical != phone {
		return "phone_mismatch"
	}
	return ""
}

func randomToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
