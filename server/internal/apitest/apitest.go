// Package apitest drives handlers in tests with a stand-in for authentication.
package apitest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ahmadteeb/sideorbit/server/internal/auth"
)

const userHeader = "X-Test-User"

// RequireUser trusts the test header in place of a real token.
func RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.Header.Get(userHeader))
		if err != nil {
			auth.Unauthorized(w)
			return
		}
		next.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), id)))
	})
}

// Router mounts the given route sets.
func Router(modules ...interface{ Routes(chi.Router) }) http.Handler {
	r := chi.NewRouter()
	for _, m := range modules {
		m.Routes(r)
	}
	return r
}

// NewUser inserts an account.
func NewUser(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := pool.Exec(context.Background(), "INSERT INTO users (id, email) VALUES ($1, $2)", id, id.String()+"@example.com"); err != nil {
		t.Fatal(err)
	}
	return id
}

// Do sends one request as user (uuid.Nil: signed out).
func Do(t *testing.T, h http.Handler, user uuid.UUID, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	if user != uuid.Nil {
		r.Header.Set(userHeader, user.String())
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

// JSON decodes a response body.
func JSON[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("%d %q: %v", rec.Code, rec.Body, err)
	}
	return v
}

// Count runs a query that returns one number.
func Count(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) (n int) {
	t.Helper()
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
