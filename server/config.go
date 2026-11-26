package main

import (
	"cmp"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ahmadteeb/sideorbit/server/internal/anisette"
	"github.com/ahmadteeb/sideorbit/server/internal/auth"
	"github.com/ahmadteeb/sideorbit/server/internal/mail"
)

type config struct {
	httpAddr    string
	databaseURL string
	anisetteURL *url.URL
	logLevel    slog.Level
	auth        auth.Config
	gateway     anisette.Config
}

// loadConfig reads the environment and reports every problem at once.
// Error messages never include values, which may be secrets.
func loadConfig(getenv func(string) string) (config, error) {
	var errs []error
	fail := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }

	httpURL := func(name string) *url.URL {
		u, err := url.Parse(getenv(name))
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			fail("%s must be an http(s) URL", name)
			return &url.URL{}
		}
		return u
	}
	duration := func(name string, def time.Duration) time.Duration {
		v := getenv(name)
		if v == "" {
			return def
		}
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			fail("%s must be a positive duration such as 15m", name)
		}
		return d
	}
	count := func(name string, def int) int {
		v := getenv(name)
		if v == "" {
			return def
		}
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			fail("%s must be a positive number", name)
		}
		return n
	}

	cfg := config{
		httpAddr:    cmp.Or(getenv("HTTP_ADDR"), ":8080"),
		databaseURL: getenv("DATABASE_URL"),
		anisetteURL: httpURL("ANISETTE_UPSTREAM"),
	}
	if cfg.databaseURL == "" {
		fail("DATABASE_URL is required")
	}
	if err := cfg.logLevel.UnmarshalText([]byte(cmp.Or(getenv("LOG_LEVEL"), "info"))); err != nil {
		fail("LOG_LEVEL: %w", err)
	}

	a := &cfg.auth
	a.PublicURL = httpURL("PUBLIC_URL")
	// Cookies and sign-in links must not travel in the clear.
	if h := a.PublicURL.Hostname(); a.PublicURL.Scheme == "http" && h != "localhost" && h != "127.0.0.1" {
		fail("PUBLIC_URL must use https (http is allowed for localhost only)")
	}
	a.JWTSecret = []byte(getenv("JWT_SECRET"))
	if len(a.JWTSecret) < 32 || strings.HasPrefix(getenv("JWT_SECRET"), "change-me") {
		fail("JWT_SECRET must be a random string of at least 32 characters")
	}
	a.AccessTTL = duration("ACCESS_TOKEN_TTL", 15*time.Minute)
	a.RefreshTTL = duration("REFRESH_TOKEN_TTL", 30*24*time.Hour)
	a.MaxDevices = count("MAX_DEVICES_PER_USER", 5)
	for r := range strings.SplitSeq(cmp.Or(getenv("NATIVE_REDIRECTS"), "orbit://auth/callback,http://127.0.0.1/callback"), ",") {
		if r = strings.TrimSpace(r); r != "" {
			a.NativeRedirects = append(a.NativeRedirects, r)
		}
	}

	cfg.gateway = anisette.Config{
		Upstream:               cfg.anisetteURL,
		HeadersPerMinUser:      count("ANISETTE_HEADERS_PER_MIN_PER_USER", 60),
		ProvisionPerHourUser:   count("ANISETTE_PROVISION_PER_HOUR_PER_USER", 5),
		ProvisionPerHourDevice: count("ANISETTE_PROVISION_PER_HOUR_PER_DEVICE", 3),
		RequestsPerMinIP:       count("ANISETTE_REQUESTS_PER_MIN_PER_IP", 120),
		SessionTimeout:         2 * time.Minute,
	}

	for _, p := range []struct {
		prefix string
		build  func(id, secret string) *auth.Provider
	}{{"GITHUB", auth.GitHub}, {"GOOGLE", auth.Google}} {
		id, secret := getenv(p.prefix+"_CLIENT_ID"), getenv(p.prefix+"_CLIENT_SECRET")
		if (id == "") != (secret == "") {
			fail("set both %s_CLIENT_ID and %s_CLIENT_SECRET, or neither", p.prefix, p.prefix)
		} else if id != "" {
			a.Providers = append(a.Providers, p.build(id, secret))
		}
	}
	if host := getenv("SMTP_HOST"); host != "" {
		from := getenv("SMTP_FROM")
		if from == "" {
			fail("SMTP_FROM is required with SMTP_HOST")
		}
		a.Mailer = mail.SMTP{Host: host, Port: cmp.Or(getenv("SMTP_PORT"), "587"),
			Username: getenv("SMTP_USERNAME"), Password: getenv("SMTP_PASSWORD"), From: from}
	}
	if len(a.Providers) == 0 && a.Mailer == nil {
		fail("configure at least one sign-in method (GitHub, Google or SMTP)")
	}
	return cfg, errors.Join(errs...)
}
