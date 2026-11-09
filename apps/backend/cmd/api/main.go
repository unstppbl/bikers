package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"bikers.kz/backend/internal/app"
	"bikers.kz/backend/internal/config"
)

func main() {
	if e := run(); e != nil {
		log.Print(e)
		os.Exit(1)
	}
}
func run() error {
	command := "serve"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	if command == "push-keys" {
		return runPushKeys(os.Args[2:])
	}
	if command == "healthcheck" {
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		client := http.Client{Timeout: 3 * time.Second}
		r, e := client.Get("http://127.0.0.1:" + port + "/health/ready")
		if e != nil {
			return errors.New("healthcheck request failed")
		}
		defer r.Body.Close()
		if r.StatusCode != 200 {
			return errors.New("application is not ready")
		}
		return nil
	}
	if command != "serve" && command != "migrate" && command != "admin" && command != "push-test" && command != "places" {
		return errors.New("usage: api serve|migrate|healthcheck|admin|push-keys|push-test|places")
	}
	cfg, e := config.Load()
	if e != nil {
		return e
	}
	if command == "serve" && cfg.TelegramToken == "" {
		return errors.New("TELEGRAM_GATEWAY_TOKEN is required; production does not support fake login")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	runtime, e := app.Open(ctx, cfg)
	if e != nil {
		return e
	}
	defer runtime.Close()
	if command == "admin" {
		return runAdmin(ctx, runtime.Trust, os.Args[2:])
	}
	if command == "push-test" {
		return runPushTest(ctx, runtime.Notifications, os.Args[2:])
	}
	if command == "places" {
		return runPlaces(ctx, runtime.Places, os.Args[2:])
	}
	if command == "migrate" {
		if e = runtime.System.Migrate(ctx); e != nil {
			return fmt.Errorf("migration failed: %w", e)
		}
		log.Print("database migration complete")
		return nil
	}
	return runtime.Run(ctx)
}
