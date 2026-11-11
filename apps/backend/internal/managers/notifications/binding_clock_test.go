package notifications

import (
	"context"
	"crypto/elliptic"
	"encoding/base64"
	"testing"
	"time"

	"bikers.kz/backend/internal/models"
)

func TestBindingOperationsCaptureOneUTCApplicationTime(t *testing.T) {
	at := time.Date(2000, 1, 1, 17, 0, 0, 0, time.FixedZone("test UTC+5", 5*60*60))
	for _, operation := range []string{"subscribe", "list subscriptions", "diagnostic selection"} {
		t.Run(operation, func(t *testing.T) {
			manager, repo, _, account, subscription := diagnosticFixture(t)
			calls := 0
			manager.now = func() time.Time {
				value := at.Add(time.Duration(calls) * time.Hour)
				calls++
				return value
			}
			var err error
			wantReads := 1
			switch operation {
			case "subscribe":
				x, y := elliptic.P256().ScalarBaseMult([]byte{1})
				input := models.PushSubscriptionInput{Endpoint: "https://web.push.apple.com/isolated-clock-subscription", Keys: models.PushKeys{P256dh: base64.RawURLEncoding.EncodeToString(elliptic.Marshal(elliptic.P256(), x, y)), Auth: base64.RawURLEncoding.EncodeToString(make([]byte, 16))}}
				_, err = manager.Subscribe(context.Background(), models.Session{User: models.User{ID: account}, Digest: []byte("clock-session")}, input)
				wantReads = 2
				if len(repo.tx.writtenSubscriptions) != 1 || !repo.tx.writtenSubscriptions[0].CreatedAt.Equal(at) {
					t.Fatal("subscription timestamp did not reuse its binding-check time")
				}
			case "list subscriptions":
				_, err = manager.Subscriptions(context.Background(), models.User{ID: account})
			case "diagnostic selection":
				err = manager.TestPush(context.Background(), account, subscription)
			}
			if err != nil {
				t.Fatal(err)
			}
			if calls != 1 || len(repo.tx.bindingTimes) != wantReads {
				t.Fatalf("operation used %d clock reads and %d binding queries", calls, len(repo.tx.bindingTimes))
			}
			for _, got := range repo.tx.bindingTimes {
				if !got.Equal(at) || got.Location() != time.UTC {
					t.Fatalf("binding query did not receive the captured UTC operation time: %s", got)
				}
			}
		})
	}
}
