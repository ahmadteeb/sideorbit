// Package apps serves each device's app inventory and the user's saved app
// sources (AltStore-format JSON).
package apps

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ahmadteeb/sideorbit/server/internal/auth"
	"github.com/ahmadteeb/sideorbit/server/internal/httpx"
	"github.com/ahmadteeb/sideorbit/server/internal/ratelimit"
	"github.com/ahmadteeb/sideorbit/server/internal/store/db"
)

const (
	maxSourceBytes = 2 << 20
	maxSources     = 50
)

// Service serves the inventory and source endpoints.
type Service struct {
	q           *db.Queries
	log         *slog.Logger
	requireUser func(http.Handler) http.Handler
	fetch       *http.Client // must refuse non-public addresses
	fetches     *ratelimit.Limiter
}

// New builds the service. fetch downloads user-supplied URLs, so it must be a
// safehttp client.
func New(pool *pgxpool.Pool, log *slog.Logger, requireUser func(http.Handler) http.Handler, fetch *http.Client) *Service {
	return &Service{q: db.New(pool), log: log, requireUser: requireUser, fetch: fetch,
		fetches: ratelimit.New(10, time.Minute)}
}

// Routes registers the endpoints on r.
func (s *Service) Routes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(s.requireUser)
		r.Get("/api/devices/{id}/apps", s.deviceApps)
		r.Get("/api/sources", s.listSources)
		r.Post("/api/sources", s.addSource)
		r.Delete("/api/sources/{id}", s.deleteSource)
	})
}

type appView struct {
	BundleID  string     `json:"bundle_id"`
	Name      string     `json:"name"`
	Version   *string    `json:"version"`
	ExpiresAt *time.Time `json:"expires_at"`
	SourceURL *string    `json:"source_url"`
	UpdatedAt time.Time  `json:"updated_at"`
}

func (s *Service) deviceApps(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.ID(w, r, "id")
	if !ok {
		return
	}
	if _, err := s.q.GetDevice(r.Context(), db.GetDeviceParams{ID: id, UserID: auth.UserID(r.Context())}); errors.Is(err, pgx.ErrNoRows) {
		httpx.NotFound(w)
		return
	} else if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	rows, err := s.q.ListDeviceApps(r.Context(), id)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	out := make([]appView, 0, len(rows))
	for _, a := range rows {
		out = append(out, appView{a.BundleID, a.Name, a.Version, a.ExpiresAt, a.SourceUrl, a.UpdatedAt})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"apps": out})
}

type sourceView struct {
	ID        uuid.UUID `json:"id"`
	URL       string    `json:"url"`
	Name      *string   `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

func toSourceView(s db.Source) sourceView { return sourceView{s.ID, s.Url, s.Name, s.CreatedAt} }

func (s *Service) listSources(w http.ResponseWriter, r *http.Request) {
	rows, err := s.q.ListSources(r.Context(), auth.UserID(r.Context()))
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	out := make([]sourceView, 0, len(rows))
	for _, src := range rows {
		out = append(out, toSourceView(src))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"sources": out})
}

var errNotASource = errors.New("not an AltStore source")

// sourceName downloads an AltStore source and returns its name.
func (s *Service) sourceName(ctx context.Context, target string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Orbit")
	res, err := s.fetch.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", errNotASource
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxSourceBytes+1))
	if err != nil {
		return "", err
	}
	var src struct {
		Name string            `json:"name"`
		Apps []json.RawMessage `json:"apps"`
	}
	if len(body) > maxSourceBytes || json.Unmarshal(body, &src) != nil || src.Apps == nil {
		return "", errNotASource
	}
	name := strings.TrimSpace(src.Name)
	if name == "" {
		return "", errNotASource
	}
	if r := []rune(name); len(r) > 200 {
		name = string(r[:200])
	}
	return strings.ReplaceAll(strings.ToValidUTF8(name, ""), "\x00", ""), nil
}

func (s *Service) addSource(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if !httpx.Decode(w, r, &req) {
		return
	}
	u, err := url.Parse(strings.TrimSpace(req.URL))
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || len(req.URL) > 2048 {
		httpx.Error(w, http.StatusBadRequest, "invalid_url", "A source must be an https URL.", "Check the address.")
		return
	}
	u.Fragment = ""
	target, userID := u.String(), auth.UserID(r.Context())
	if !s.fetches.Allow(userID.String()) {
		s.log.Warn("rate limit hit", "endpoint", "sources", "user_id", userID)
		w.Header().Set("Retry-After", "60")
		httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "Too many sources added at once.", "Wait a minute.")
		return
	}

	_, err = s.q.GetSourceByURL(r.Context(), db.GetSourceByURLParams{UserID: userID, Url: target})
	status := http.StatusOK // already saved: refresh its name
	if errors.Is(err, pgx.ErrNoRows) {
		status = http.StatusCreated
		var n int64
		if n, err = s.q.CountSources(r.Context(), userID); err == nil && n >= maxSources {
			httpx.Error(w, http.StatusForbidden, "source_limit", "This account has reached its source limit.", "Remove a source, then try again.")
			return
		}
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}

	name, err := s.sourceName(r.Context(), target)
	if err != nil {
		// One message for every cause, so the endpoint can't be used to probe
		// which hosts or ports answer.
		s.log.Info("source rejected", "user_id", userID, "err", err)
		httpx.Error(w, http.StatusUnprocessableEntity, "invalid_source", "Orbit couldn't read an app source at that address.",
			"Check that it is a public https link to AltStore source JSON under 2 MB.")
		return
	}
	src, err := s.q.CreateSource(r.Context(), db.CreateSourceParams{ID: uuid.Must(uuid.NewV7()), UserID: userID, Url: target, Name: &name})
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, status, toSourceView(src))
}

func (s *Service) deleteSource(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.ID(w, r, "id")
	if !ok {
		return
	}
	n, err := s.q.DeleteSource(r.Context(), db.DeleteSourceParams{ID: id, UserID: auth.UserID(r.Context())})
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if n == 0 {
		httpx.NotFound(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
