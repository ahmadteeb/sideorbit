package main

import (
	"cmp"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
)

type config struct {
	httpAddr    string
	databaseURL string
	anisetteURL *url.URL
	logLevel    slog.Level
}

// loadConfig reads the environment and reports every problem at once.
// Error messages never include values, which may be secrets.
func loadConfig(getenv func(string) string) (config, error) {
	cfg := config{
		httpAddr:    cmp.Or(getenv("HTTP_ADDR"), ":8080"),
		databaseURL: getenv("DATABASE_URL"),
	}
	var errs []error
	if cfg.databaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}
	u, err := url.Parse(getenv("ANISETTE_UPSTREAM"))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		errs = append(errs, errors.New("ANISETTE_UPSTREAM must be an http(s) URL"))
	}
	cfg.anisetteURL = u
	if err := cfg.logLevel.UnmarshalText([]byte(cmp.Or(getenv("LOG_LEVEL"), "info"))); err != nil {
		errs = append(errs, fmt.Errorf("LOG_LEVEL: %w", err))
	}
	return cfg, errors.Join(errs...)
}
