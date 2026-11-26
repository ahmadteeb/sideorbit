// Package devices serves the device registry and the status report endpoint.
package devices

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ahmadteeb/sideorbit/server/internal/auth"
	"github.com/ahmadteeb/sideorbit/server/internal/httpx"
	"github.com/ahmadteeb/sideorbit/server/internal/ratelimit"
	"github.com/ahmadteeb/sideorbit/server/internal/store/db"
)

// Service serves /api/devices.
type Service struct {
	pool        *pgxpool.Pool
	q           *db.Queries
	log         *slog.Logger
	requireUser func(http.Handler) http.Handler
	maxDevices  int
	reports     *ratelimit.Limiter
}

// New builds the service. maxDevices is the per-account cap (D24).
func New(pool *pgxpool.Pool, log *slog.Logger, requireUser func(http.Handler) http.Handler, maxDevices int) *Service {
	return &Service{pool: pool, q: db.New(pool), log: log, requireUser: requireUser, maxDevices: maxDevices,
		reports: ratelimit.New(60, time.Minute)}
}

// Routes registers the endpoints on r.
func (s *Service) Routes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(s.requireUser)
		r.Get("/api/devices", s.list)
		r.Post("/api/devices", s.create)
		r.Patch("/api/devices/{id}", s.rename)
		r.Delete("/api/devices/{id}", s.remove)
		r.Post("/api/devices/{id}/report", s.report)
	})
}

type view struct {
	ID           uuid.UUID  `json:"id"`
	Name         string     `json:"name"`
	Model        *string    `json:"model"`
	IOSVersion   *string    `json:"ios_version"`
	ClientID     *string    `json:"client_id"`
	AppleTeamID  *string    `json:"apple_team_id"`
	AppleLabel   *string    `json:"apple_label"`
	AppIDsUsed7d *int16     `json:"app_ids_used_7d"`
	VPNOk        *bool      `json:"vpn_ok"`
	PairingOk    *bool      `json:"pairing_ok"`
	LastSeenAt   *time.Time `json:"last_seen_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

func toView(d db.Device) view {
	return view{d.ID, d.Name, d.Model, d.IosVersion, d.AnisetteClientID, d.AppleTeamID, d.AppleLabel,
		d.AppIdsUsed7d, d.VpnOk, d.PairingOk, d.LastSeenAt, d.CreatedAt}
}

// text reports whether s is printable text of at most max characters.
func text(s string, max int) bool {
	return utf8.ValidString(s) && utf8.RuneCountInString(s) <= max && strings.IndexFunc(s, unicode.IsControl) < 0
}

// clientIDPattern covers the anisette v3 identifier (base64 or hex).
var clientIDPattern = regexp.MustCompile(`^[A-Za-z0-9+/=_-]{8,128}$`)

func (s *Service) list(w http.ResponseWriter, r *http.Request) {
	rows, err := s.q.ListDevices(r.Context(), auth.UserID(r.Context()))
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	out := make([]view, 0, len(rows))
	for _, d := range rows {
		out = append(out, toView(d))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"devices": out})
}

var (
	errLimit = errors.New("device limit")
	errTaken = errors.New("client id in use")
)

// create registers a device. Registering the same client_id again returns the
// existing record, so an app can call it on every first launch after login.
func (s *Service) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name       string `json:"name"`
		Model      string `json:"model"`
		IOSVersion string `json:"ios_version"`
		ClientID   string `json:"client_id"`
	}
	if !httpx.Decode(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || !text(req.Name, 100) || !text(req.Model, 100) || !text(req.IOSVersion, 32) || !clientIDPattern.MatchString(req.ClientID) {
		httpx.Error(w, http.StatusBadRequest, "invalid_device", "A name (up to 100 characters) and a client_id are required.", "")
		return
	}
	optional := func(v string) *string {
		if v == "" {
			return nil
		}
		return &v
	}
	userID := auth.UserID(r.Context())
	var device db.Device
	status := http.StatusCreated
	err := pgx.BeginFunc(r.Context(), s.pool, func(tx pgx.Tx) error {
		q, ctx := s.q.WithTx(tx), r.Context()
		// Serialise registrations per account so the cap can't be raced.
		if _, err := q.LockUser(ctx, userID); err != nil {
			return err
		}
		existing, err := q.GetDeviceByClientID(ctx, req.ClientID)
		switch {
		case err == nil && existing.UserID != userID:
			return errTaken
		case err == nil:
			status = http.StatusOK
			device, err = q.TouchDevice(ctx, db.TouchDeviceParams{ID: existing.ID, Model: optional(req.Model), IosVersion: optional(req.IOSVersion)})
			return err
		case !errors.Is(err, pgx.ErrNoRows):
			return err
		}
		n, err := q.CountDevices(ctx, userID)
		if err != nil {
			return err
		}
		if n >= int64(s.maxDevices) {
			return errLimit
		}
		device, err = q.CreateDevice(ctx, db.CreateDeviceParams{
			ID: uuid.Must(uuid.NewV7()), UserID: userID, Name: req.Name,
			Model: optional(req.Model), IosVersion: optional(req.IOSVersion), AnisetteClientID: &req.ClientID,
		})
		return err
	})
	switch {
	case errors.Is(err, errLimit):
		s.log.Warn("device limit hit", "user_id", userID)
		httpx.Error(w, http.StatusForbidden, "device_limit", "This account has reached its device limit.", "Remove a device you no longer use, then try again.")
	case errors.Is(err, errTaken):
		httpx.Error(w, http.StatusConflict, "client_id_in_use", "This device is registered to another account.", "Remove it from that account first.")
	case err != nil:
		httpx.Internal(w, r, err)
	default:
		httpx.JSON(w, status, toView(device))
	}
}

func (s *Service) rename(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.ID(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if !httpx.Decode(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || !text(req.Name, 100) {
		httpx.Error(w, http.StatusBadRequest, "invalid_device", "A name of up to 100 characters is required.", "")
		return
	}
	device, err := s.q.RenameDevice(r.Context(), db.RenameDeviceParams{ID: id, UserID: auth.UserID(r.Context()), Name: req.Name})
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.NotFound(w)
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toView(device))
}

func (s *Service) remove(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.ID(w, r, "id")
	if !ok {
		return
	}
	n, err := s.q.DeleteDevice(r.Context(), db.DeleteDeviceParams{ID: id, UserID: auth.UserID(r.Context())})
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

// Owns reports whether the device belongs to the account.
func (s *Service) Owns(ctx context.Context, userID, deviceID uuid.UUID) (bool, error) {
	_, err := s.q.GetDevice(ctx, db.GetDeviceParams{ID: deviceID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}
