package main

import (
	"context"
	"testing"
	"time"

	"bikers.kz/backend/internal/database"
	"bikers.kz/backend/internal/managers/trust"
	"bikers.kz/backend/internal/models"
	"github.com/google/uuid"
)

type cliTrustRepo struct {
	database.TrustRepository
	tx    *cliTrustTransaction
	calls int
}

func (r *cliTrustRepo) WithTrust(_ context.Context, actor string, decide database.TrustDecision) error {
	r.calls++
	if actor != "" {
		panic("CLI unexpectedly impersonated an HTTP actor")
	}
	return decide(r.tx)
}

type cliTrustTransaction struct {
	database.TrustTransaction
	id      string
	granted bool
	audit   models.TrustAudit
}

func (t *cliTrustTransaction) Account(id string) (models.TrustAccount, error) {
	if id != t.id {
		return models.TrustAccount{}, database.ErrNotFound
	}
	return models.TrustAccount{ID: id, Status: "active"}, nil
}
func (t *cliTrustTransaction) SetAdmin(id string, on bool, _ time.Time) error {
	if id != t.id {
		return database.ErrNotFound
	}
	t.granted = on
	return nil
}
func (t *cliTrustTransaction) InsertAudit(a models.TrustAudit) error { t.audit = a; return nil }
func TestAdminCLIRequiresExplicitExistingAccountAndAuditedReason(t *testing.T) {
	id := uuid.NewString()
	tx := &cliTrustTransaction{id: id}
	repo := &cliTrustRepo{tx: tx}
	manager := trust.New(repo, nil, nil)
	if err := runAdmin(context.Background(), manager, []string{"grant", id, "--reason", "Reviewed local operator grant"}); err != nil {
		t.Fatal(err)
	}
	if !tx.granted || tx.audit.Source != "cli" || tx.audit.ActorID != "" || tx.audit.TargetID != id || tx.audit.Reason != "Reviewed local operator grant" {
		t.Fatal("CLI did not preserve explicit operator intent and audit")
	}
	if err := runAdmin(context.Background(), manager, []string{"revoke", id, "--reason", "Operator access removed"}); err != nil {
		t.Fatal(err)
	}
	if tx.granted || tx.audit.Action != "revoke_admin" {
		t.Fatal("CLI revoke not recorded")
	}
	before := repo.calls
	for _, args := range [][]string{{}, {"grant"}, {"grant", id}, {"grant", id, "--reason", " "}, {"grant", id, "--unknown", "value"}, {"grant", id, "--reason", "review", "extra"}, {"promote", id, "--reason", "review"}} {
		if err := runAdmin(context.Background(), manager, args); err == nil {
			t.Fatalf("ambiguous CLI args accepted: %v", args)
		}
	}
	if repo.calls != before {
		t.Fatal("invalid CLI arguments reached persistence")
	}
	if err := runAdmin(context.Background(), manager, []string{"grant", uuid.NewString(), "--reason", "Must not create accounts"}); err == nil {
		t.Fatal("CLI granted a missing account")
	}
}
