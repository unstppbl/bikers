package app

import (
	"context"
	"strings"
	"testing"

	"bikers.kz/backend/internal/database/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestApplicationReplacementKeepsSessionsApprovalsAndOrderedChat(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011234567", "Organizer")
	rider := f.login("+77021234567", "Rider")
	ride := f.ride(owner, 2)
	f.join(ride, rider)
	expect(t, f.decision(ride, owner, rider, "approve"), 200)
	command := map[string]any{"client_message_id": uuid.NewString(), "body": "Persist across replacement"}
	expect(t, f.request("POST", "/api/v1/rides/"+ride+"/messages", command, rider), 201)
	challenge := f.challenge("+77051234567")

	// Replace both the HTTP application and its connection pool. Only the
	// external provider and PostgreSQL retain state across this boundary.
	config := f.db.Config()
	f.ts.Close()
	f.db.Close()
	db, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	if err := postgres.New(db).Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.db = db
	f.startServer(f.gateway)

	r := f.request("GET", "/api/v1/session", nil, rider)
	expect(t, r, 200)
	if r.body["user"].(map[string]any)["id"] != rider.id || r.body["csrf_token"] != rider.csrf {
		t.Fatal("replacement lost the persisted session")
	}
	r = f.request("GET", "/api/v1/rides/"+ride, nil, rider)
	expect(t, r, 200)
	if !strings.Contains(r.raw, "SECRET MEETING") {
		t.Fatal("replacement lost approved access")
	}
	r = f.request("GET", "/api/v1/rides/"+ride+"/messages", nil, rider)
	expect(t, r, 200)
	if len(r.body["messages"].([]any)) != 1 || r.body["next_sequence"] != float64(1) {
		t.Fatal("replacement lost ordered chat history")
	}
	expect(t, f.request("POST", "/api/v1/rides/"+ride+"/messages", command, rider), 200)
	r = f.request("POST", "/api/v1/rides/"+ride+"/messages", map[string]any{"client_message_id": uuid.NewString(), "body": "Next message"}, rider)
	expect(t, r, 201)
	if r.body["message"].(map[string]any)["sequence"] != float64(2) {
		t.Fatal("replacement reset or duplicated the room sequence")
	}
	expect(t, f.request("POST", "/api/v1/auth/challenges/verify", map[string]any{"challenge_id": challenge, "code": "123456"}, nil), 200)
}
