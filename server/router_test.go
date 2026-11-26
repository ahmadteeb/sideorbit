package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestRouter(t *testing.T) {
	up := func(context.Context) error { return nil }
	down := func(context.Context) error { return errors.New("secret-detail") }
	tests := []struct {
		name, method, path string
		db, anisette       check
		wantStatus         int
		wantBody           string
	}{
		{"healthz", http.MethodGet, "/healthz", down, down, http.StatusOK, "ok\n"},
		{"ready", http.MethodGet, "/readyz", up, up, http.StatusOK, `{"anisette":"ok","db":"ok"}` + "\n"},
		{"db down", http.MethodGet, "/readyz", down, up, http.StatusServiceUnavailable, `{"anisette":"ok","db":"error"}` + "\n"},
		{"anisette down", http.MethodGet, "/readyz", up, down, http.StatusServiceUnavailable, `{"anisette":"error","db":"ok"}` + "\n"},
		{"wrong method", http.MethodPost, "/healthz", up, up, http.StatusMethodNotAllowed, ""},
		{"unknown path", http.MethodGet, "/nope", up, up, http.StatusNotFound, "404 page not found\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newRouter(slog.New(slog.DiscardHandler), tt.db, tt.anisette)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))
			if rec.Code != tt.wantStatus || rec.Body.String() != tt.wantBody {
				t.Errorf("got %d %q, want %d %q", rec.Code, rec.Body, tt.wantStatus, tt.wantBody)
			}
		})
	}
}

func TestRequestLoggerOmitsQueryString(t *testing.T) {
	var out strings.Builder
	up := func(context.Context) error { return nil }
	h := newRouter(slog.New(slog.NewTextHandler(&out, nil)), up, up)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/healthz?token=hunter2", nil))
	if !strings.Contains(out.String(), "route=/healthz") || strings.Contains(out.String(), "hunter2") {
		t.Errorf("log = %q", out.String())
	}
}

func TestAnisetteCheck(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		wantErr bool
	}{
		{"ok", http.StatusOK, false},
		{"upstream error", http.StatusInternalServerError, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v3/client_info" {
					t.Errorf("path = %s", r.URL.Path)
				}
				w.WriteHeader(tt.status)
			}))
			defer srv.Close()
			u, err := url.Parse(srv.URL)
			if err != nil {
				t.Fatal(err)
			}
			if err := anisetteCheck(u)(context.Background()); (err != nil) != tt.wantErr {
				t.Errorf("err = %v", err)
			}
		})
	}
}
