package main

import (
	"context"
	"errors"
	"flag"
	"github.com/google/uuid"
	"io"
	"log"
)

type pushTester interface {
	TestPush(context.Context, string, string) error
}

func runPushTest(ctx context.Context, manager pushTester, args []string) error {
	usage := errors.New("usage: api push-test USER_UUID --subscription SUBSCRIPTION_UUID")
	if len(args) < 1 {
		return usage
	}
	flags := flag.NewFlagSet("push-test", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	subscription := flags.String("subscription", "", "explicit account-owned browser subscription UUID")
	if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 {
		return usage
	}
	account, err := uuid.Parse(args[0])
	if err != nil {
		return usage
	}
	sub, err := uuid.Parse(*subscription)
	if err != nil {
		return usage
	}
	if err = manager.TestPush(ctx, account.String(), sub.String()); err != nil {
		return err
	}
	log.Print("Web Push provider accepted the test notification; device display is not yet confirmed")
	return nil
}
