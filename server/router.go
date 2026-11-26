package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// check reports whether a dependency is reachable.
type check func(context.Context) error

func newRouter(log *slog.Logger, db, anisette check) http.Handler {
	r := chi.NewRouter()
	r.Use(requestLogger(log), middleware.Recoverer)
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ok\n")
	})
	r.Get("/readyz", readyz(log, db, anisette))
	return r
}

// requestLogger records metadata only. It logs the route pattern rather than the
// URL because query strings carry one-time tokens, and it never logs headers,
// bodies or client IPs.
func requestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			log.Info("request",
				"method", r.Method,
				"route", chi.RouteContext(r.Context()).RoutePattern(),
				"status", ww.Status(),
				"duration_ms", time.Since(start).Milliseconds())
		})
	}
}

// readyz answers 503 if any dependency is down. Causes are logged, not returned.
func readyz(log *slog.Logger, db, anisette check) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := http.StatusOK
		run := func(name string, c check) string {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if err := c(ctx); err != nil {
				log.Warn("not ready", "dependency", name, "err", err)
				status = http.StatusServiceUnavailable
				return "error"
			}
			return "ok"
		}
		body := map[string]string{"db": run("db", db), "anisette": run("anisette", anisette)}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}
}

func anisetteCheck(upstream *url.URL) check {
	target := upstream.JoinPath("v3", "client_info").String()
	return func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return err
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			return fmt.Errorf("anisette returned %s", res.Status)
		}
		return nil
	}
}
