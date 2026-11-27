// Command server is the Orbit API server.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"

	"github.com/ahmadteeb/sideorbit/server/internal/anisette"
	"github.com/ahmadteeb/sideorbit/server/internal/apps"
	"github.com/ahmadteeb/sideorbit/server/internal/auth"
	"github.com/ahmadteeb/sideorbit/server/internal/devices"
	"github.com/ahmadteeb/sideorbit/server/internal/events"
	"github.com/ahmadteeb/sideorbit/server/internal/metrics"
	"github.com/ahmadteeb/sideorbit/server/internal/push"
	"github.com/ahmadteeb/sideorbit/server/internal/safehttp"
	"github.com/ahmadteeb/sideorbit/server/internal/scheduler"
	"github.com/ahmadteeb/sideorbit/server/internal/store"
	"github.com/ahmadteeb/sideorbit/server/internal/web"
)

func main() {
	command := run
	switch {
	case len(os.Args) == 2 && os.Args[1] == "vapid-keys":
		command = func() error { return vapidKeys(os.Stdout) }
	case len(os.Args) > 1:
		command = func() error { return errors.New("usage: orbit-server [vapid-keys]") }
	}
	if err := command(); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

// vapidKeys prints a new web push key pair as .env lines.
func vapidKeys(w io.Writer) error {
	private, public, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		return fmt.Errorf("generate keys: %w", err)
	}
	_, err = fmt.Fprintf(w, "VAPID_PUBLIC_KEY=%s\nVAPID_PRIVATE_KEY=%s\n", public, private)
	return err
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
	modules := []module{
		accounts,
		registry,
		anisette.New(cfg.gateway, accounts, registry, log),
		apps.New(db, log, accounts.RequireUser, safehttp.New(5*time.Second)),
		events.New(db, accounts.RequireUser),
	}
	var sender scheduler.Sender
	if cfg.push.Public != "" {
		alerts := push.New(cfg.push, db, log, accounts.RequireUser, accounts.SessionID, safehttp.New(10*time.Second))
		modules, sender = append(modules, alerts), alerts
	} else {
		log.Warn("web push is off: VAPID keys are not set")
	}
	var jobs sync.WaitGroup
	defer jobs.Wait()
	jobsCtx, stopJobs := context.WithCancel(ctx)
	defer stopJobs() // runs before Wait
	jobs.Go(func() { scheduler.New(db, sender, log, cfg.retention).Run(jobsCtx) })

	handler := newRouter(log, db.Ping, anisetteCheck(cfg.anisetteURL), web.Handler(), modules...)
	srv := &http.Server{
		Addr:              cfg.httpAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	failed := make(chan error, 2)
	go func() { failed <- srv.ListenAndServe() }()
	// A separate listener, so metrics are never reachable through the public one.
	if cfg.metricsAddr != "" {
		metrics.WatchPool(db)
		internal := &http.Server{Addr: cfg.metricsAddr, Handler: metrics.Handler(), ReadHeaderTimeout: 10 * time.Second}
		defer internal.Close()
		go func() { failed <- internal.ListenAndServe() }()
		log.Info("metrics listening", "addr", cfg.metricsAddr)
	}
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
