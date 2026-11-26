// Command server is the Orbit API server.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ahmadteeb/sideorbit/server/internal/anisette"
	"github.com/ahmadteeb/sideorbit/server/internal/apps"
	"github.com/ahmadteeb/sideorbit/server/internal/auth"
	"github.com/ahmadteeb/sideorbit/server/internal/devices"
	"github.com/ahmadteeb/sideorbit/server/internal/events"
	"github.com/ahmadteeb/sideorbit/server/internal/safehttp"
	"github.com/ahmadteeb/sideorbit/server/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.logLevel}))
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := store.Open(ctx, cfg.databaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	accounts := auth.New(cfg.auth, db, log)
	registry := devices.New(db, log, accounts.RequireUser, cfg.auth.MaxDevices)
	handler := newRouter(log, db.Ping, anisetteCheck(cfg.anisetteURL),
		accounts,
		registry,
		anisette.New(cfg.gateway, accounts, registry, log),
		apps.New(db, log, accounts.RequireUser, safehttp.New(5*time.Second)),
		events.New(db, accounts.RequireUser),
	)
	srv := &http.Server{
		Addr:              cfg.httpAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	failed := make(chan error, 1)
	go func() { failed <- srv.ListenAndServe() }()
	log.Info("listening", "addr", srv.Addr)

	select {
	case err := <-failed:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdown); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}
