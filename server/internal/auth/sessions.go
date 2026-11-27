package auth

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ahmadteeb/sideorbit/server/internal/httpx"
	"github.com/ahmadteeb/sideorbit/server/internal/store/db"
)

// methods tells the sign-in page which buttons to show.
func (s *Service) methods(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]bool{
		"github": s.providers["github"] != nil,
		"google": s.providers["google"] != nil,
		"email":  s.cfg.Mailer != nil,
	})
}

// SessionID returns the signed-in account's live web session named by the
// request's cookie, or the zero UUID.
func (s *Service) SessionID(r *http.Request) uuid.UUID {
	c, err := r.Cookie(s.cookieName(cookieRefresh))
	if err != nil {
		return uuid.Nil
	}
	// A stale cookie just means no known session.
	id, _ := s.q.LiveSession(r.Context(), db.LiveSessionParams{TokenHash: hashToken(c.Value), UserID: UserID(r.Context())})
	return id
}

type sessionView struct {
	ID         uuid.UUID `json:"id"`
	Client     string    `json:"client"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at"`
	Current    bool      `json:"current"`
}

func (s *Service) listSessions(w http.ResponseWriter, r *http.Request) {
	rows, err := s.q.ListSessions(r.Context(), UserID(r.Context()))
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	current := s.SessionID(r)
	out := make([]sessionView, 0, len(rows))
	for _, row := range rows {
		out = append(out, sessionView{row.FamilyID, row.Client, row.CreatedAt, row.LastUsedAt, row.FamilyID == current})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"sessions": out})
}

// revokeSession signs one session out. Its access token lasts up to 15 minutes
// more.
func (s *Service) revokeSession(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.ID(w, r, "id")
	if !ok {
		return
	}
	userID := UserID(r.Context())
	var n int64
	// The same account lock as rotation, so a refresh can't slip past.
	err := pgx.BeginFunc(r.Context(), s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		if _, err := q.LockUser(r.Context(), userID); err != nil {
			return err
		}
		var err error
		n, err = q.RevokeUserFamily(r.Context(), db.RevokeUserFamilyParams{UserID: userID, FamilyID: id})
		if err != nil || n == 0 {
			return err // not this account's session: leave its alerts alone
		}
		return q.DeleteSessionPushSubscriptions(r.Context(), id)
	})
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if n == 0 {
		httpx.NotFound(w)
		return
	}
	s.log.Info("session revoked", "user_id", userID)
	w.WriteHeader(http.StatusNoContent)
}
