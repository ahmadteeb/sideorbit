// Package auth implements sign-in (GitHub, Google, email magic link), tokens,
// sessions and the account endpoints.
package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ahmadteeb/sideorbit/server/internal/httpx"
	"github.com/ahmadteeb/sideorbit/server/internal/ratelimit"
	"github.com/ahmadteeb/sideorbit/server/internal/store/db"
)

// Client kinds.
const (
	ClientWeb     = "web"
	ClientIOS     = "ios"
	ClientDesktop = "desktop"
)

// Mailer sends one plain-text email.
type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

// Config holds the auth settings.
type Config struct {
	PublicURL       *url.URL
	JWTSecret       []byte
	AccessTTL       time.Duration
	RefreshTTL      time.Duration
	NativeRedirects []string
	Providers       []*Provider // only those with credentials
	Mailer          Mailer      // nil disables magic links
	MaxDevices      int
}

// Service serves the auth endpoints.
type Service struct {
	cfg       Config
	pool      *pgxpool.Pool
	q         *db.Queries
	log       *slog.Logger
	providers map[string]*Provider
	secure    bool // set the Secure cookie flag

	mailByAddress *ratelimit.Limiter
	mailByIP      *ratelimit.Limiter
	mailTotal     *ratelimit.Limiter // protects the SMTP account's quota and reputation
}

// New builds the service.
func New(cfg Config, pool *pgxpool.Pool, log *slog.Logger) *Service {
	s := &Service{
		cfg:           cfg,
		pool:          pool,
		q:             db.New(pool),
		log:           log,
		providers:     map[string]*Provider{},
		secure:        cfg.PublicURL.Scheme == "https",
		mailByAddress: ratelimit.New(3, 15*time.Minute),
		mailByIP:      ratelimit.New(10, 15*time.Minute),
		mailTotal:     ratelimit.New(100, time.Hour),
	}
	for _, p := range cfg.Providers {
		p.oauth.RedirectURL = cfg.PublicURL.JoinPath("api", "auth", p.name, "callback").String()
		s.providers[p.name] = p
	}
	return s
}

// Routes registers the endpoints on r.
func (s *Service) Routes(r chi.Router) {
	r.Get("/api/auth/methods", s.methods)
	r.Get("/api/auth/{provider}/start", s.oauthStart)
	r.Get("/api/auth/{provider}/callback", s.oauthCallback)
	if s.cfg.Mailer != nil {
		r.Post("/api/auth/email/start", s.emailStart)
		r.Get("/api/auth/email/verify", s.emailConfirm)
		// Refuse cross-site form posts, which would sign a victim into the
		// sender's account.
		r.Method(http.MethodPost, "/api/auth/email/verify",
			http.NewCrossOriginProtection().Handler(http.HandlerFunc(s.emailVerify)))
	}
	r.Post("/api/auth/token", s.token)
	r.Post("/api/auth/logout", s.logout)
	r.Group(func(r chi.Router) {
		r.Use(s.RequireUser)
		r.Get("/api/me", s.me)
		r.Post("/api/me/delete-intent", s.deleteIntent)
		r.Delete("/api/me", s.deleteMe)
		r.Get("/api/me/sessions", s.listSessions)
		r.Delete("/api/me/sessions/{id}", s.revokeSession)
	})
}

// target is where a finished sign-in should land.
type target struct {
	client    string
	challenge string // PKCE S256 challenge; native clients only
	redirect  string // native clients only
}

// parseTarget validates the client, code_challenge and redirect parameters.
func (s *Service) parseTarget(client, challenge, redirect string) (target, error) {
	switch client {
	case "", ClientWeb:
		// The web client always lands on "/", so there is no open redirect.
		return target{client: ClientWeb}, nil
	case ClientIOS, ClientDesktop:
	default:
		return target{}, errors.New("client must be web, ios or desktop")
	}
	if b, err := base64.RawURLEncoding.DecodeString(challenge); err != nil || len(b) != sha256.Size {
		return target{}, errors.New("code_challenge must be a base64url SHA-256 (PKCE S256)")
	}
	if redirect == "" && client == ClientIOS {
		if i := slices.IndexFunc(s.cfg.NativeRedirects, func(a string) bool { return !strings.HasPrefix(a, "http") }); i >= 0 {
			redirect = s.cfg.NativeRedirects[i]
		}
	}
	if !s.allowedRedirect(client, redirect) {
		return target{}, errors.New("redirect is not allowed")
	}
	return target{client: client, challenge: challenge, redirect: redirect}, nil
}

// allowedRedirect matches exactly, except that an allowlisted
// http://127.0.0.1/path accepts any port (RFC 8252 loopback). Desktop uses the
// loopback form only, and iOS never does.
func (s *Service) allowedRedirect(client, raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || raw == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || strings.Contains(raw, "#") {
		return false
	}
	loopback := u.Scheme == "http" && u.Hostname() == "127.0.0.1"
	if loopback != (client == ClientDesktop) {
		return false
	}
	for _, allowed := range s.cfg.NativeRedirects {
		a, err := url.Parse(allowed)
		if err != nil {
			continue
		}
		if loopback && a.Scheme == "http" && a.Host == "127.0.0.1" {
			if u.EscapedPath() == a.EscapedPath() && u.Port() != "" {
				return true
			}
			continue
		}
		if raw == allowed {
			return true
		}
	}
	return false
}

var (
	errAccountExists = errors.New("an account with this email already exists")
	errBadEmail      = errors.New("unsupported email address")
)

// ascii rejects look-alike addresses: lowercasing maps some non-ASCII letters
// (such as U+0130) onto ASCII ones.
func ascii(s string) bool {
	return !strings.ContainsFunc(s, func(r rune) bool { return r > unicode.MaxASCII })
}

// findOrCreateUser returns the account for a verified identity. A new identity
// joins an existing account with the same email only if it shows control of the
// mailbox now (D25).
func (s *Service) findOrCreateUser(ctx context.Context, provider string, id identity) (db.User, error) {
	if !ascii(id.email) {
		return db.User{}, errBadEmail
	}
	email := strings.ToLower(id.email)
	var user db.User
	attempt := func() error {
		return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
			q := s.q.WithTx(tx)
			var err error
			user, err = q.GetUserByIdentity(ctx, db.GetUserByIdentityParams{Provider: provider, Subject: id.subject})
			if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			user, err = q.GetUserByEmail(ctx, email)
			if errors.Is(err, pgx.ErrNoRows) {
				user, err = q.CreateUser(ctx, db.CreateUserParams{ID: newID(), Email: &email, DisplayName: optional(id.name)})
			} else if err == nil && !id.ownsMailbox {
				return errAccountExists
			}
			if err != nil {
				return err
			}
			return q.CreateIdentity(ctx, db.CreateIdentityParams{ID: newID(), UserID: user.ID, Provider: provider, Subject: id.subject})
		})
	}
	err := attempt()
	if isUniqueViolation(err) { // two first sign-ins at once
		err = attempt()
	}
	return user, err
}

// finish completes a verified sign-in: web gets session cookies, native clients
// get a one-time code on their redirect URI.
func (s *Service) finish(w http.ResponseWriter, r *http.Request, user db.User, t target, status int) {
	if t.client == ClientWeb {
		pair, err := s.issue(r.Context(), s.q, user.ID, newID(), ClientWeb, time.Now().Add(s.cfg.RefreshTTL))
		if err != nil {
			httpx.Internal(w, r, err)
			return
		}
		s.setSessionCookies(w, pair)
		http.Redirect(w, r, "/", status)
		return
	}
	code := randomToken()
	err := s.q.CreateLoginCode(r.Context(), db.CreateLoginCodeParams{
		CodeHash: hashToken(code), UserID: &user.ID, Purpose: purposeExchange,
		CodeChallenge: &t.challenge, Client: &t.client, TtlSeconds: int32(exchangeTTL / time.Second),
	})
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	redirectWith(w, r, t.redirect, "code", code, status)
}

func redirectWith(w http.ResponseWriter, r *http.Request, to, key, value string, status int) {
	http.Redirect(w, r, to+"?"+url.Values{key: {value}}.Encode(), status)
}

func newID() uuid.UUID { return uuid.Must(uuid.NewV7()) }

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
