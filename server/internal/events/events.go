// Package events serves the refresh history.
package events

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ahmadteeb/sideorbit/server/internal/auth"
	"github.com/ahmadteeb/sideorbit/server/internal/httpx"
	"github.com/ahmadteeb/sideorbit/server/internal/store/db"
)

const pageSize = 50

// Service serves /api/events.
type Service struct {
	q           *db.Queries
	requireUser func(http.Handler) http.Handler
}

// New builds the service.
func New(pool *pgxpool.Pool, requireUser func(http.Handler) http.Handler) *Service {
	return &Service{q: db.New(pool), requireUser: requireUser}
}

// Routes registers the endpoints on r.
func (s *Service) Routes(r chi.Router) {
	r.With(s.requireUser).Get("/api/events", s.list)
}

type view struct {
	ID         uuid.UUID `json:"id"`
	DeviceID   uuid.UUID `json:"device_id"`
	DeviceName string    `json:"device_name"`
	Kind       string    `json:"kind"`
	Trigger    *string   `json:"trigger"`
	Result     string    `json:"result"`
	ErrorCode  *string   `json:"error_code"`
	DurationMs *int32    `json:"duration_ms"`
	OccurredAt time.Time `json:"occurred_at"`
}

// The cursor is the (time, id) of the last row returned.
func encodeCursor(at time.Time, id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(at.UnixMicro(), 10) + "." + id.String()))
}

func decodeCursor(c string) (time.Time, uuid.UUID, error) {
	b, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	micros, idText, ok := strings.Cut(string(b), ".")
	if !ok {
		return time.Time{}, uuid.Nil, errors.New("malformed cursor")
	}
	n, err := strconv.ParseInt(micros, 10, 64)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	id, err := uuid.Parse(idText)
	return time.UnixMicro(n), id, err
}

func (s *Service) list(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	params := db.ListEventsParams{UserID: auth.UserID(r.Context()), PageSize: pageSize + 1}
	bad := func(what string) {
		httpx.Error(w, http.StatusBadRequest, "invalid_query", what+" is not valid.", "")
	}
	if v := query.Get("device_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			bad("device_id")
			return
		}
		params.DeviceID = &id
	}
	if v := query.Get("result"); v != "" {
		if v != "success" && v != "failure" {
			bad("result")
			return
		}
		params.Result = &v
	}
	if v := query.Get("cursor"); v != "" {
		at, id, err := decodeCursor(v)
		if err != nil {
			bad("cursor")
			return
		}
		params.BeforeTime, params.BeforeID = &at, &id
	}
	rows, err := s.q.ListEvents(r.Context(), params)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	var next *string
	if len(rows) > pageSize {
		rows = rows[:pageSize]
		c := encodeCursor(rows[pageSize-1].OccurredAt, rows[pageSize-1].ID)
		next = &c
	}
	out := make([]view, 0, len(rows))
	for _, e := range rows {
		out = append(out, view{e.ID, e.DeviceID, e.DeviceName, e.Kind, e.Trigger, e.Result, e.ErrorCode, e.DurationMs, e.OccurredAt})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"events": out, "next_cursor": next})
}
