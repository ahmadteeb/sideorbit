package auth

import (
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"

	"github.com/ahmadteeb/sideorbit/server/internal/httpx"
	"github.com/ahmadteeb/sideorbit/server/internal/store/db"
)

const deleteTTL = 5 * time.Minute

func (s *Service) me(w http.ResponseWriter, r *http.Request) {
	user, err := s.q.GetActiveUser(r.Context(), UserID(r.Context()))
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	providers, err := s.q.ListProviders(r.Context(), user.ID)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"id":           user.ID,
		"email":        user.Email,
		"display_name": user.DisplayName,
		"identities":   providers,
		"limits":       map[string]int{"max_devices": s.cfg.MaxDevices},
	})
}

func (s *Service) deleteIntent(w http.ResponseWriter, r *http.Request) {
	token, err := s.sign(claims{RegisteredClaims: jwt.RegisteredClaims{Subject: UserID(r.Context()).String()}}, audDelete, deleteTTL)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"confirmation_token": token, "expires_in": int(deleteTTL / time.Second)})
}

// deleteMe removes every record belonging to the account. The user row stays,
// emptied and marked deleted.
func (s *Service) deleteMe(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ConfirmationToken string `json:"confirmation_token"`
	}
	if !httpx.Decode(w, r, &req) {
		return
	}
	id := UserID(r.Context())
	c, err := s.parse(req.ConfirmationToken, audDelete)
	if err != nil || c.Subject != id.String() {
		httpx.Error(w, http.StatusBadRequest, "confirmation_required", "Deleting your account needs a fresh confirmation.", "Start the deletion again.")
		return
	}
	ctx := r.Context()
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		user, err := q.GetActiveUser(ctx, id)
		if err != nil {
			return err
		}
		if err := q.DeleteUserLoginCodes(ctx, db.DeleteUserLoginCodesParams{UserID: &id, Email: user.Email}); err != nil {
			return err
		}
		for _, remove := range []func() error{
			func() error { return q.DeleteIdentities(ctx, id) },
			func() error { return q.DeleteUserRefreshTokens(ctx, id) },
			func() error { return q.DeleteUserDevices(ctx, id) }, // apps, events and alerts cascade
			func() error { return q.DeleteUserSources(ctx, id) },
			func() error { return q.DeleteUserPushSubscriptions(ctx, id) },
			func() error { return q.AnonymizeUser(ctx, id) },
		} {
			if err := remove(); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	s.log.Info("account deleted", "user_id", id)
	s.clearSessionCookies(w)
	w.WriteHeader(http.StatusNoContent)
}
