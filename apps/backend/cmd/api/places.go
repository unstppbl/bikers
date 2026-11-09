package main

import (
	"bikers.kz/backend/internal/managers/places"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
)

func runPlaces(ctx context.Context, manager *places.Manager, args []string) error {
	const usage = "usage: api places import --file PATH --sha256 HASH"
	if len(args) < 1 || args[0] != "import" {
		return errors.New(usage)
	}
	flags := flag.NewFlagSet("places import", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	path := flags.String("file", "", "local pinned OSM NDJSON file")
	digest := flags.String("sha256", "", "expected file SHA-256")
	if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 || *path == "" || *digest == "" {
		return errors.New(usage)
	}
	file, err := os.Open(*path)
	if err != nil {
		return err
	}
	defer file.Close()
	out, err := manager.Import(ctx, file, *digest)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(out)
}
