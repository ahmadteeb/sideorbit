package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/ahmadteeb/sideorbit/server/internal/httpx"
)

// check reports whether a dependency is reachable.
type check func(context.Context) error

// module is a group of endpoints.
type module interface{ Routes(chi.Router) }

// newRouter wires the endpoints. site answers every path that isn't an API,
// gateway or health route; nil means there is no dashboard.
func newRouter(log *slog.Logger, db, anisette check, site http.Handler, modules ...module) http.Handler {
	r := chi.NewRouter()
	r.Use(requestLogger(log), middleware.Recoverer)
	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		api := strings.HasPrefix(req.URL.Path, "/api/") || strings.HasPrefix(req.URL.Path, "/anisette/")
		if site != nil && !api {
			site.ServeHTTP(w, req)
			return
		}
		httpx.Error(w, http.StatusNotFound, "not_found", "There's nothing at this address.", "")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		httpx.Error(w, http.StatusMethodNotAllowed, "method_not_allowed", "That method isn't supported here.", "")
	})
	for _, m := range modules {
		m.Routes(r)
	}
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

// readyzTTL is how long a readiness result is reused. The endpoint is public, so
// a flood must not turn into the same flood against the database and anisette.
const readyzTTL = 5 * time.Second

// readyz answers 503 if any dependency is down. Causes are logged, not returned.
func readyz(log *slog.Logger, db, anisette check) http.HandlerFunc {
	var (
		mu      sync.Mutex
		checked time.Time
		status  int
		body    map[string]string
	)
	return func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if time.Since(checked) >= readyzTTL {
			status = http.StatusOK
			run := func(name string, c check) string {
				// Not the request's context: the result is shared.
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				if err := c(ctx); err != nil {
					log.Warn("not ready", "dependency", name, "err", err)
					status = http.StatusServiceUnavailable
					return "error"
				}
				return "ok"
			}
			body = map[string]string{"db": run("db", db), "anisette": run("anisette", anisette)}
			checked = time.Now()
		}
		httpx.JSON(w, status, body)
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
