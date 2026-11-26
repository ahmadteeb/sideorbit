package auth

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/endpoints"

	"github.com/ahmadteeb/sideorbit/server/internal/httpx"
)

const (
	cookieState = "orbit_oauth"
	stateTTL    = 10 * time.Minute
)

// identity is a verified account at a provider.
type identity struct {
	subject, email, name string
}

// Provider is one OAuth login option.
type Provider struct {
	name     string
	oauth    oauth2.Config
	api      string // base URL of the profile API
	identify func(ctx context.Context, c *http.Client, api string) (identity, error)
}

var errNoVerifiedEmail = errors.New("no verified email")

// GitHub returns the GitHub provider.
func GitHub(clientID, secret string) *Provider {
	return &Provider{
		name: "github",
		oauth: oauth2.Config{ClientID: clientID, ClientSecret: secret, Endpoint: endpoints.GitHub,
			Scopes: []string{"read:user", "user:email"}},
		api:      "https://api.github.com",
		identify: githubIdentity,
	}
}

// Google returns the Google provider.
func Google(clientID, secret string) *Provider {
	return &Provider{
		name: "google",
		oauth: oauth2.Config{ClientID: clientID, ClientSecret: secret, Endpoint: endpoints.Google,
			Scopes: []string{"openid", "email", "profile"}},
		api:      "https://openidconnect.googleapis.com",
		identify: googleIdentity,
	}
}

func getJSON(ctx context.Context, c *http.Client, url string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	res, err := c.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("profile request returned %s", res.Status)
	}
	return json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(v)
}

func githubIdentity(ctx context.Context, c *http.Client, api string) (identity, error) {
	var user struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
	}
	if err := getJSON(ctx, c, api+"/user", &user); err != nil {
		return identity{}, err
	}
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := getJSON(ctx, c, api+"/user/emails", &emails); err != nil {
		return identity{}, err
	}
	if user.ID == 0 {
		return identity{}, errors.New("profile has no id")
	}
	name := user.Name
	if name == "" {
		name = user.Login
	}
	for _, e := range emails {
		if e.Primary && e.Verified && e.Email != "" {
			return identity{strconv.FormatInt(user.ID, 10), e.Email, name}, nil
		}
	}
	return identity{}, errNoVerifiedEmail
}

func googleIdentity(ctx context.Context, c *http.Client, api string) (identity, error) {
	var user struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
	}
	if err := getJSON(ctx, c, api+"/v1/userinfo", &user); err != nil {
		return identity{}, err
	}
	if user.Sub == "" {
		return identity{}, errors.New("profile has no id")
	}
	if !user.EmailVerified || user.Email == "" {
		return identity{}, errNoVerifiedEmail
	}
	return identity{user.Sub, user.Email, user.Name}, nil
}

func (s *Service) oauthStart(w http.ResponseWriter, r *http.Request) {
	p := s.providers[chi.URLParam(r, "provider")]
	if p == nil {
		httpx.Error(w, http.StatusNotFound, "unknown_provider", "That sign-in method isn't available.", "")
		return
	}
	q := r.URL.Query()
	t, err := s.parseTarget(q.Get("client"), q.Get("code_challenge"), q.Get("redirect"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
		return
	}
	state, verifier := randomToken(), oauth2.GenerateVerifier()
	signed, err := s.sign(claims{
		State: state, Verifier: verifier, Provider: p.name,
		Client: t.client, Challenge: t.challenge, Redirect: t.redirect,
	}, audState, stateTTL)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	http.SetCookie(w, s.cookie(cookieState, signed, "/api/auth", time.Now().Add(stateTTL)))
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, p.oauth.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier)), http.StatusFound)
}

func (s *Service) oauthCallback(w http.ResponseWriter, r *http.Request) {
	p := s.providers[chi.URLParam(r, "provider")]
	if p == nil {
		httpx.Error(w, http.StatusNotFound, "unknown_provider", "That sign-in method isn't available.", "")
		return
	}
	badState := func() {
		s.log.Warn("sign-in refused", "provider", p.name, "reason", "state")
		httpx.Error(w, http.StatusBadRequest, "invalid_state", "This sign-in attempt expired or didn't start here.", "Start signing in again.")
	}
	cookie, err := r.Cookie(cookieState)
	if err != nil {
		badState()
		return
	}
	st, err := s.parse(cookie.Value, audState)
	if err != nil || st.Provider != p.name || subtle.ConstantTimeCompare([]byte(st.State), []byte(r.URL.Query().Get("state"))) != 1 {
		badState()
		return
	}
	clear := s.cookie(cookieState, "", "/api/auth", time.Unix(0, 0))
	clear.MaxAge = -1
	http.SetCookie(w, clear)
	t := target{client: st.Client, challenge: st.Challenge, redirect: st.Redirect}

	fail := func(code string, cause error) {
		s.log.Warn("sign-in failed", "provider", p.name, "reason", code, "err", cause)
		if t.client == ClientWeb {
			redirectWith(w, r, "/login", "error", code, http.StatusFound)
			return
		}
		redirectWith(w, r, t.redirect, "error", code, http.StatusFound)
	}
	if r.URL.Query().Get("error") != "" {
		fail("access_denied", nil)
		return
	}

	ctx := context.WithValue(r.Context(), oauth2.HTTPClient, &http.Client{Timeout: 10 * time.Second})
	token, err := p.oauth.Exchange(ctx, r.URL.Query().Get("code"), oauth2.VerifierOption(st.Verifier))
	if err != nil {
		// The library's error includes the provider's response body.
		var re *oauth2.RetrieveError
		if errors.As(err, &re) {
			err = fmt.Errorf("token endpoint: %s", re.ErrorCode)
		}
		fail("provider_error", err)
		return
	}
	client := p.oauth.Client(ctx, token)
	client.Timeout = 10 * time.Second
	id, err := p.identify(ctx, client, p.api)
	if errors.Is(err, errNoVerifiedEmail) {
		fail("email_unverified", err)
		return
	}
	if err != nil {
		fail("provider_error", err)
		return
	}
	user, err := s.findOrCreateUser(r.Context(), p.name, id.subject, id.email, id.name)
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	s.log.Info("identity verified", "provider", p.name, "user_id", user.ID, "client", t.client)
	s.finish(w, r, user, t, http.StatusFound)
}
