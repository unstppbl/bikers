package main

import (
	"bikers.kz/backend/internal/managers/trust"
	"context"
	"errors"
	"flag"
	"io"
)

func runAdmin(ctx context.Context, manager *trust.Manager, args []string) error {
	if len(args) < 2 || (args[0] != "grant" && args[0] != "revoke") {
		return errors.New("usage: api admin grant|revoke USER_UUID --reason TEXT")
	}
	flags := flag.NewFlagSet("admin", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	why := flags.String("reason", "", "reason for the explicit administrator role change")
	if err := flags.Parse(args[2:]); err != nil || flags.NArg() != 0 {
		return errors.New("usage: api admin grant|revoke USER_UUID --reason TEXT")
	}
	return manager.Role(ctx, args[1], args[0] == "grant", *why)
}
