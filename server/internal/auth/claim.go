package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ahmadteeb/sideorbit/server/internal/httpx"
	"github.com/ahmadteeb/sideorbit/server/internal/store/db"
)

var providerNames = map[string]string{"github": "GitHub", "google": "Google", "email": "an email link"}

// askAboutAccount shows "was that you?" to someone who has just proven they
// control the address of an account that never did (D38). Whoever made it may
// have lost the mailbox long ago.
func (s *Service) askAboutAccount(w http.ResponseWriter, r *http.Request, user db.User, provider string, id identity, t target) {
	providers, err := s.q.ListProviders(r.Context(), user.ID)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	names := make([]string, len(providers))
	for i, p := range providers {
		names[i] = providerNames[p]
	}
	token, err := s.sign(claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: user.ID.String()},
		Provider:         provider, State: id.subject,
		Client: t.client, Challenge: t.challenge, Redirect: t.redirect,
	}, audClaim, confirmTTL)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	s.log.Info("asking about an unproven account", "user_id", user.ID, "provider", provider)
	writeClaimPage(w, claimPage{
		Token: token, Email: strings.ToLower(id.email), App: appNames[t.client],
		How: strings.Join(names, " and "), Since: user.CreatedAt.UTC().Format("2 January 2006"),
	})
}

var errAlreadyAnswered = errors.New("already answered")

// claim applies the answer. "keep" links the new sign-in; "remove" empties the
// account first.
func (s *Service) claim(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	answer := r.PostFormValue("answer")
	c, err := s.parse(r.PostFormValue("token"), audClaim)
	var id uuid.UUID
	if err == nil {
		id, err = uuid.Parse(c.Subject)
	}
	if err != nil || (answer != "keep" && answer != "remove") {
		writePage(w, http.StatusBadRequest, "", "", "", "")
		return
	}
	user, err := s.settleAccount(r.Context(), id, c.Provider, c.State, answer == "remove")
	if errors.Is(err, errAlreadyAnswered) || errors.Is(err, pgx.ErrNoRows) {
		writePage(w, http.StatusBadRequest, "", "", "", "")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	s.log.Info("unproven account claimed", "user_id", id, "removed_previous", answer == "remove")
	s.finish(w, r, user, target{client: c.Client, challenge: c.Challenge, redirect: c.Redirect}, http.StatusSeeOther)
}

// settleAccount answers once: it only acts while the address is still unproven,
// so a resubmitted page can't remove data added since.
func (s *Service) settleAccount(ctx context.Context, id uuid.UUID, provider, subject string, remove bool) (db.User, error) {
	var user db.User
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		if _, err := q.LockUser(ctx, id); err != nil {
			return err
		}
		var err error
		if user, err = q.GetActiveUser(ctx, id); err != nil {
			return err
		}
		if user.EmailProvenAt != nil {
			return errAlreadyAnswered
		}
		if remove {
			if err := wipe(ctx, q, user); err != nil {
				return err
			}
			if err := q.ClearDisplayName(ctx, id); err != nil {
				return err
			}
		}
		if err := q.CreateIdentity(ctx, db.CreateIdentityParams{ID: newID(), UserID: id, Provider: provider, Subject: subject}); err != nil {
			return err
		}
		return q.MarkEmailProven(ctx, id)
	})
	return user, err
}
