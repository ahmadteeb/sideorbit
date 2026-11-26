package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ahmadteeb/sideorbit/server/internal/httpx"
	"github.com/ahmadteeb/sideorbit/server/internal/store/db"
)

const (
	purposeExchange = "app_exchange"
	purposeMagic    = "magic_link"

	exchangeTTL = time.Minute
	magicTTL    = 15 * time.Minute

	// JWT audiences keep one kind of signed token from being used as another.
	audAccess = "orbit-access"
	audState  = "orbit-oauth-state"
	audDelete = "orbit-delete-account"

	cookieAccess  = "orbit_access"
	cookieRefresh = "orbit_refresh"

	// csrfHeader must be present on cookie-authenticated mutations. Another
	// origin cannot send a custom header without a CORS preflight, which this
	// server never approves.
	csrfHeader = "X-Orbit-CSRF"
)

type claims struct {
	jwt.RegisteredClaims
	// OAuth state only.
	State     string `json:"st,omitempty"`
	Verifier  string `json:"pv,omitempty"`
	Provider  string `json:"pr,omitempty"`
	Client    string `json:"cl,omitempty"`
	Challenge string `json:"cc,omitempty"`
	Redirect  string `json:"ru,omitempty"`
}

func (s *Service) sign(c claims, audience string, ttl time.Duration) (string, error) {
	now := time.Now()
	c.Audience = jwt.ClaimStrings{audience}
	c.IssuedAt = jwt.NewNumericDate(now)
	c.ExpiresAt = jwt.NewNumericDate(now.Add(ttl))
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(s.cfg.JWTSecret)
}

func (s *Service) parse(token, audience string) (claims, error) {
	var c claims
	_, err := jwt.ParseWithClaims(token, &c, func(*jwt.Token) (any, error) { return s.cfg.JWTSecret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithAudience(audience),
		jwt.WithExpirationRequired())
	return c, err
}

func randomToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b) // never fails
	return base64.RawURLEncoding.EncodeToString(b)
}

func hashToken(t string) []byte {
	sum := sha256.Sum256([]byte(t))
	return sum[:]
}

// verifyPKCE reports whether verifier matches the S256 challenge.
func verifyPKCE(verifier, challenge string) bool {
	if len(verifier) < 43 || len(verifier) > 128 {
		return false
	}
	got := base64.RawURLEncoding.EncodeToString(hashToken(verifier))
	return subtle.ConstantTimeCompare([]byte(got), []byte(challenge)) == 1
}

func isUniqueViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}

type tokenPair struct {
	AccessToken  string `json:"access_token,omitempty"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`

	refreshExpires time.Time
}

// issue stores a new refresh token in the family and signs an access token.
func (s *Service) issue(ctx context.Context, q *db.Queries, userID, family uuid.UUID, client string, expires time.Time) (tokenPair, error) {
	refresh := randomToken()
	if err := q.CreateRefreshToken(ctx, db.CreateRefreshTokenParams{
		ID: newID(), UserID: userID, FamilyID: family, TokenHash: hashToken(refresh), Client: client, ExpiresAt: expires,
	}); err != nil {
		return tokenPair{}, fmt.Errorf("store refresh token: %w", err)
	}
	access, err := s.sign(claims{RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()}}, audAccess, s.cfg.AccessTTL)
	if err != nil {
		return tokenPair{}, fmt.Errorf("sign access token: %w", err)
	}
	return tokenPair{access, int(s.cfg.AccessTTL / time.Second), refresh, expires}, nil
}

// ErrInvalidToken means a token or grant was refused, as opposed to a server
// failure.
var ErrInvalidToken = errors.New("invalid grant")

// withSession runs fn on a refresh token with its account locked. Without the
// lock, a rotation committing during a revocation would insert a token that the
// revocation never sees. Each statement after the lock reads a fresh snapshot.
func (s *Service) withSession(ctx context.Context, refresh string, fn func(*db.Queries, db.LockRefreshTokenRow) error) error {
	hash := hashToken(refresh)
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		owner, err := q.GetRefreshTokenOwner(ctx, hash)
		if err == nil {
			_, err = q.LockUser(ctx, owner)
		}
		var row db.LockRefreshTokenRow
		if err == nil {
			row, err = q.LockRefreshToken(ctx, hash)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidToken
		}
		if err != nil {
			return err
		}
		return fn(q, row)
	})
}

// rotate exchanges a refresh token for a new pair. Presenting a token that was
// already used means it leaked, so the whole family is revoked.
func (s *Service) rotate(ctx context.Context, refresh, wantClient string) (tokenPair, error) {
	var pair tokenPair
	reused := false
	err := s.withSession(ctx, refresh, func(q *db.Queries, row db.LockRefreshTokenRow) error {
		// A web token is only good from the cookie and a native token only
		// from the body.
		if row.RevokedAt != nil || row.Expired || (row.Client == ClientWeb) != (wantClient == ClientWeb) {
			return ErrInvalidToken
		}
		if row.UsedAt != nil {
			reused = true
			s.log.Warn("refresh token reuse; revoking the session", "user_id", row.UserID, "client", row.Client)
			return endSession(ctx, q, row.FamilyID) // commit the revocation
		}
		if err := q.MarkRefreshTokenUsed(ctx, row.ID); err != nil {
			return err
		}
		// The new token keeps the family's expiry: 30 days is absolute.
		var err error
		pair, err = s.issue(ctx, q, row.UserID, row.FamilyID, row.Client, row.ExpiresAt)
		return err
	})
	if err == nil && reused {
		err = ErrInvalidToken
	}
	return pair, err
}

// endSession revokes a session's tokens and stops the alerts it turned on.
func endSession(ctx context.Context, q *db.Queries, session uuid.UUID) error {
	if err := q.RevokeFamily(ctx, session); err != nil {
		return err
	}
	return q.DeleteSessionPushSubscriptions(ctx, session)
}

type tokenRequest struct {
	GrantType    string `json:"grant_type"`
	Code         string `json:"code"`
	CodeVerifier string `json:"code_verifier"`
	RefreshToken string `json:"refresh_token"`
}

func (s *Service) token(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var req tokenRequest
	if !httpx.Decode(w, r, &req) {
		return
	}
	invalid := func() {
		s.log.Warn("token request refused", "grant_type", req.GrantType)
		httpx.Error(w, http.StatusBadRequest, "invalid_grant", "That sign-in is no longer valid.", "Sign in again.")
	}
	switch req.GrantType {
	case "authorization_code":
		// Single use: the code is consumed even if the verifier is wrong.
		code, err := s.q.ConsumeLoginCode(r.Context(), db.ConsumeLoginCodeParams{CodeHash: hashToken(req.Code), Purpose: purposeExchange})
		if errors.Is(err, pgx.ErrNoRows) {
			invalid()
			return
		}
		if err != nil {
			httpx.Internal(w, r, err)
			return
		}
		if code.UserID == nil || code.Client == nil || code.CodeChallenge == nil || !verifyPKCE(req.CodeVerifier, *code.CodeChallenge) {
			invalid()
			return
		}
		pair, err := s.issue(r.Context(), s.q, *code.UserID, newID(), *code.Client, time.Now().Add(s.cfg.RefreshTTL))
		if err != nil {
			httpx.Internal(w, r, err)
			return
		}
		s.log.Info("signed in", "user_id", *code.UserID, "client", *code.Client)
		httpx.JSON(w, http.StatusOK, pair)

	case "refresh_token":
		refresh, wantClient := req.RefreshToken, ClientIOS
		fromCookie := refresh == ""
		if fromCookie {
			c, err := r.Cookie(s.cookieName(cookieRefresh))
			if err != nil || r.Header.Get(csrfHeader) == "" {
				invalid()
				return
			}
			refresh, wantClient = c.Value, ClientWeb
		}
		pair, err := s.rotate(r.Context(), refresh, wantClient)
		if errors.Is(err, ErrInvalidToken) {
			if fromCookie {
				s.clearSessionCookies(w)
			}
			invalid()
			return
		}
		if err != nil {
			httpx.Internal(w, r, err)
			return
		}
		if fromCookie {
			// The web page never sees the tokens.
			s.setSessionCookies(w, pair)
			pair.AccessToken, pair.RefreshToken = "", ""
		}
		httpx.JSON(w, http.StatusOK, pair)

	default:
		httpx.Error(w, http.StatusBadRequest, "unsupported_grant_type", "grant_type must be authorization_code or refresh_token.", "")
	}
}

// logout revokes the refresh token's family. It always answers 204.
func (s *Service) logout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if r.ContentLength != 0 && !httpx.Decode(w, r, &req) {
		return
	}
	refresh, fromCookie := req.RefreshToken, false
	if refresh == "" {
		if c, err := r.Cookie(s.cookieName(cookieRefresh)); err == nil {
			if r.Header.Get(csrfHeader) == "" {
				httpx.Error(w, http.StatusForbidden, "csrf", "Missing "+csrfHeader+" header.", "")
				return
			}
			refresh, fromCookie = c.Value, true
		}
	}
	if refresh != "" {
		err := s.withSession(r.Context(), refresh, func(q *db.Queries, row db.LockRefreshTokenRow) error {
			s.log.Info("signed out", "user_id", row.UserID)
			return endSession(r.Context(), q, row.FamilyID)
		})
		if err != nil && !errors.Is(err, ErrInvalidToken) {
			httpx.Internal(w, r, err)
			return
		}
	}
	if fromCookie {
		s.clearSessionCookies(w)
	}
	w.WriteHeader(http.StatusNoContent)
}

// cookieName adds the __Host- prefix over HTTPS, which the browser only accepts
// from this exact host with Path=/.
func (s *Service) cookieName(base string) string {
	if s.secure {
		return "__Host-" + base
	}
	return base
}

func (s *Service) cookie(base, value string, expires time.Time) *http.Cookie {
	return &http.Cookie{
		Name: s.cookieName(base), Value: value, Path: "/", Expires: expires,
		HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteLaxMode,
	}
}

func (s *Service) setSessionCookies(w http.ResponseWriter, p tokenPair) {
	http.SetCookie(w, s.cookie(cookieAccess, p.AccessToken, time.Now().Add(s.cfg.AccessTTL)))
	http.SetCookie(w, s.cookie(cookieRefresh, p.RefreshToken, p.refreshExpires))
}

func (s *Service) clearSessionCookies(w http.ResponseWriter) {
	for _, name := range []string{cookieAccess, cookieRefresh} {
		c := s.cookie(name, "", time.Unix(0, 0))
		c.MaxAge = -1
		http.SetCookie(w, c)
	}
}

type userKey struct{}

// UserID returns the account set by RequireUser.
func UserID(ctx context.Context) uuid.UUID {
	id, _ := ctx.Value(userKey{}).(uuid.UUID)
	return id
}

// Authenticate resolves an access token to a live account.
func (s *Service) Authenticate(ctx context.Context, accessToken string) (uuid.UUID, error) {
	c, err := s.parse(accessToken, audAccess)
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}
	id, err := uuid.Parse(c.Subject)
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}
	// A deleted account must stop working before its token expires.
	if _, err := s.q.GetActiveUser(ctx, id); errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrInvalidToken
	} else if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// IsInvalidToken reports whether Authenticate refused the token, as opposed to
// failing for a server reason.
func IsInvalidToken(err error) bool { return errors.Is(err, ErrInvalidToken) }

// BearerToken returns the Authorization bearer token, or "".
func BearerToken(r *http.Request) string {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}

// WithUser returns a context carrying the authenticated account.
func WithUser(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, userKey{}, id)
}

// RequireUser accepts a bearer token, or the web session cookie plus the CSRF
// header on mutations.
func (s *Service) RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		token := BearerToken(r)
		if token == "" {
			if c, err := r.Cookie(s.cookieName(cookieAccess)); err == nil {
				safe := r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions
				if !safe && r.Header.Get(csrfHeader) == "" {
					httpx.Error(w, http.StatusForbidden, "csrf", "Missing "+csrfHeader+" header.", "")
					return
				}
				token = c.Value
			}
		}
		id, err := s.Authenticate(r.Context(), token)
		if IsInvalidToken(err) {
			Unauthorized(w)
			return
		}
		if err != nil {
			httpx.Internal(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), id)))
	})
}

// Unauthorized writes the 401 response.
func Unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	httpx.Error(w, http.StatusUnauthorized, "unauthorized", "You are not signed in.", "Sign in again.")
}
