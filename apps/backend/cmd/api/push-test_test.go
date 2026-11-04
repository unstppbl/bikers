package main

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"testing"
)

type cliPushTester struct {
	calls                 int
	account, subscription string
	err                   error
}

func (t *cliPushTester) TestPush(_ context.Context, account, subscription string) error {
	t.calls++
	t.account, t.subscription = account, subscription
	return t.err
}

func TestPushDiagnosticCLIRequiresExplicitAccountAndSubscription(t *testing.T) {
	account, subscription := uuid.NewString(), uuid.NewString()
	manager := &cliPushTester{}
	if err := runPushTest(context.Background(), manager, []string{account, "--subscription", subscription}); err != nil {
		t.Fatal(err)
	}
	if manager.calls != 1 || manager.account != account || manager.subscription != subscription {
		t.Fatal("operator selection was not preserved")
	}
	for _, args := range [][]string{nil, {account}, {account, "--subscription", "invalid"}, {"invalid", "--subscription", subscription}, {account, "--subscription", subscription, "extra"}, {account, "--unknown", subscription}} {
		if err := runPushTest(context.Background(), manager, args); err == nil {
			t.Fatal("ambiguous diagnostic selection accepted")
		}
	}
	if manager.calls != 1 {
		t.Fatal("invalid command reached application behavior")
	}
	manager.err = errors.New("safe diagnostic failure")
	if err := runPushTest(context.Background(), manager, []string{account, "--subscription", subscription}); !errors.Is(err, manager.err) || manager.calls != 2 {
		t.Fatal("provider rejection did not propagate as command failure")
	}
}
