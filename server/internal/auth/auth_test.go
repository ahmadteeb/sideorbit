package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"

	"github.com/ahmadteeb/sideorbit/server/internal/store/storetest"
)

const verifier = "test-verifier-0123456789-0123456789-0123456789"

var challenge = base64.RawURLEncoding.EncodeToString(hashToken(verifier))

// fakeProvider stands in for GitHub and Google. Its token endpoint enforces PKCE.
type fakeProvider struct {
	srv           *httptest.Server
	email         string
	verified      bool
	hostedDomain  string
	gotChallenges map[string]string // code -> code_challenge
}

func newFakeProvider(t *testing.T) *fakeProvider {
	f := &fakeProvider{email: "Ada@Example.com", verified: true, gotChallenges: map[string]string{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /authorize", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("code_challenge_method") != "S256" {
			http.Error(w, "PKCE required", http.StatusBadRequest)
			return
		}
		code := randomToken()
		f.gotChallenges[code] = q.Get("code_challenge")
		back := q.Get("redirect_uri") + "?" + url.Values{"code": {code}, "state": {q.Get("state")}}.Encode()
		http.Redirect(w, r, back, http.StatusFound)
	})
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		want, ok := f.gotChallenges[r.PostFormValue("code")]
		if !ok || !verifyPKCE(r.PostFormValue("code_verifier"), want) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error":"invalid_grant"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"access_token":"provider-token","token_type":"Bearer"}`)
	})
	reply := func(w http.ResponseWriter, r *http.Request, v any) {
		if r.Header.Get("Authorization") != "Bearer provider-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(v)
	}
	mux.HandleFunc("GET /user", func(w http.ResponseWriter, r *http.Request) {
		reply(w, r, map[string]any{"id": 42, "login": "ada"})
	})
	mux.HandleFunc("GET /user/emails", func(w http.ResponseWriter, r *http.Request) {
		reply(w, r, []map[string]any{{"email": f.email, "primary": true, "verified": f.verified}})
	})
	mux.HandleFunc("GET /v1/userinfo", func(w http.ResponseWriter, r *http.Request) {
		reply(w, r, map[string]any{"sub": "g-1", "email": f.email, "email_verified": f.verified, "name": "Ada", "hd": f.hostedDomain})
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

type fakeMail struct {
	to, body string
	err      error
}

func (m *fakeMail) Send(_ context.Context, to, _, body string) error {
	m.to, m.body = to, body
	return m.err
}

type env struct {
	t    *testing.T
	h    http.Handler
	svc  *Service
	pool *pgxpool.Pool
	fake *fakeProvider
	mail *fakeMail
}

func newEnv(t *testing.T) *env {
	pool := storetest.New(t)
	fake := newFakeProvider(t)
	mail := &fakeMail{}
	providers := []*Provider{GitHub("gh-id", "gh-secret"), Google("g-id", "g-secret")}
	for _, p := range providers {
		p.oauth.Endpoint = oauth2.Endpoint{AuthURL: fake.srv.URL + "/authorize", TokenURL: fake.srv.URL + "/token"}
		p.api = fake.srv.URL
	}
	public, err := url.Parse("https://orbit.test")
	if err != nil {
		t.Fatal(err)
	}
	svc := New(Config{
		PublicURL: public, JWTSecret: []byte(strings.Repeat("k", 32)),
		AccessTTL: 15 * time.Minute, RefreshTTL: 720 * time.Hour,
		NativeRedirects: []string{"orbit://auth/callback", "http://127.0.0.1/callback"},
		Providers:       providers, Mailer: mail, MaxDevices: 5,
	}, pool, slog.New(slog.DiscardHandler))
	r := chi.NewRouter()
	svc.Routes(r)
	return &env{t: t, h: r, svc: svc, pool: pool, fake: fake, mail: mail}
}

type req struct {
	method, target, body string
	header               map[string]string
	cookies              []*http.Cookie
}

func (e *env) do(in req) *httptest.ResponseRecorder {
	e.t.Helper()
	r := httptest.NewRequest(in.method, in.target, strings.NewReader(in.body))
	if strings.HasPrefix(in.body, "{") {
		r.Header.Set("Content-Type", "application/json")
	}
	for k, v := range in.header {
		r.Header.Set(k, v)
	}
	for _, c := range in.cookies {
		r.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, r)
	return rec
}

var hiddenToken = regexp.MustCompile(`name="token" value="([^"]+)"`)

func (e *env) confirm(token, site string) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.do(req{method: "POST", target: "/api/auth/confirm", body: url.Values{"token": {token}}.Encode(),
		header: map[string]string{"Content-Type": "application/x-www-form-urlencoded", "Sec-Fetch-Site": site}})
}

// oauth runs the whole sign-in, pressing "Sign in" when a native client is asked to.
func (e *env) oauth(provider string, params url.Values) *httptest.ResponseRecorder {
	e.t.Helper()
	rec := e.callback(provider, params)
	if m := hiddenToken.FindStringSubmatch(rec.Body.String()); rec.Code == http.StatusOK && m != nil {
		return e.confirm(m[1], "same-origin")
	}
	return rec
}

// callback runs start -> provider -> callback and returns the callback response.
func (e *env) callback(provider string, params url.Values) *httptest.ResponseRecorder {
	e.t.Helper()
	start := e.do(req{method: "GET", target: "/api/auth/" + provider + "/start?" + params.Encode()})
	if start.Code != http.StatusFound {
		e.t.Fatalf("start: %d %s", start.Code, start.Body)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Get(start.Header().Get("Location"))
	if err != nil {
		e.t.Fatal(err)
	}
	defer res.Body.Close()
	back, err := res.Location()
	if err != nil {
		e.t.Fatalf("provider did not redirect: %v", err)
	}
	if want := "https://orbit.test/api/auth/" + provider + "/callback"; !strings.HasPrefix(back.String(), want+"?") {
		e.t.Fatalf("redirect_uri = %s", back)
	}
	return e.do(req{method: "GET", target: back.RequestURI(), cookies: start.Result().Cookies()})
}

func nativeParams(client, redirect string) url.Values {
	return url.Values{"client": {client}, "code_challenge": {challenge}, "redirect": {redirect}}
}

func codeFrom(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil || loc.Query().Get("code") == "" {
		t.Fatalf("no code: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	return loc.Query().Get("code")
}

func (e *env) token(body map[string]string) (*httptest.ResponseRecorder, tokenPair) {
	e.t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		e.t.Fatal(err)
	}
	rec := e.do(req{method: "POST", target: "/api/auth/token", body: string(b)})
	var pair tokenPair
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &pair); err != nil {
			e.t.Fatal(err)
		}
	}
	return rec, pair
}

// signIn returns tokens for an iOS client.
func (e *env) signIn() tokenPair {
	e.t.Helper()
	code := codeFrom(e.t, e.oauth("github", nativeParams("ios", "")))
	rec, pair := e.token(map[string]string{"grant_type": "authorization_code", "code": code, "code_verifier": verifier})
	if rec.Code != http.StatusOK || pair.AccessToken == "" || pair.RefreshToken == "" || pair.ExpiresIn != 900 {
		e.t.Fatalf("token: %d %s", rec.Code, rec.Body)
	}
	return pair
}

func bearer(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

func cookieMap(rec *httptest.ResponseRecorder) map[string]*http.Cookie {
	m := map[string]*http.Cookie{}
	for _, c := range rec.Result().Cookies() {
		m[strings.TrimPrefix(c.Name, "__Host-")] = c
	}
	return m
}

type meBody struct {
	ID         string   `json:"id"`
	Email      string   `json:"email"`
	Identities []string `json:"identities"`
}

func (e *env) me(in req) (int, meBody) {
	e.t.Helper()
	in.method, in.target = "GET", "/api/me"
	rec := e.do(in)
	var m meBody
	_ = json.Unmarshal(rec.Body.Bytes(), &m)
	return rec.Code, m
}

func TestOAuthWeb(t *testing.T) {
	for _, provider := range []string{"github", "google"} {
		t.Run(provider, func(t *testing.T) {
			e := newEnv(t)
			rec := e.oauth(provider, url.Values{})
			if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/" {
				t.Fatalf("callback: %d %q", rec.Code, rec.Header().Get("Location"))
			}
			cookies := cookieMap(rec)
			for _, name := range []string{cookieAccess, cookieRefresh} {
				c := cookies[name]
				if c == nil || c.Name != "__Host-"+name || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.Domain != "" {
					t.Fatalf("cookie %s = %+v", name, c)
				}
			}
			status, me := e.me(req{cookies: []*http.Cookie{cookies[cookieAccess]}})
			if status != http.StatusOK || me.Email != "ada@example.com" || len(me.Identities) != 1 || me.Identities[0] != provider {
				t.Fatalf("me: %d %+v", status, me)
			}
		})
	}
}

func TestStartRejects(t *testing.T) {
	e := newEnv(t)
	tests := []struct {
		name, target string
		want         int
	}{
		{"unknown provider", "/api/auth/gitlab/start", 404},
		{"unknown client", "/api/auth/github/start?client=tv", 400},
		{"native without challenge", "/api/auth/github/start?client=ios", 400},
		{"short challenge", "/api/auth/github/start?client=ios&code_challenge=abc", 400},
		{"foreign redirect", "/api/auth/github/start?" + nativeParams("ios", "https://evil.example/cb").Encode(), 400},
		{"redirect with query", "/api/auth/github/start?" + nativeParams("ios", "orbit://auth/callback?x=1").Encode(), 400},
		{"ios with loopback", "/api/auth/github/start?" + nativeParams("ios", "http://127.0.0.1:5000/callback").Encode(), 400},
		{"desktop with app scheme", "/api/auth/github/start?" + nativeParams("desktop", "orbit://auth/callback").Encode(), 400},
		{"desktop without port", "/api/auth/github/start?" + nativeParams("desktop", "http://127.0.0.1/callback").Encode(), 400},
		{"desktop wrong path", "/api/auth/github/start?" + nativeParams("desktop", "http://127.0.0.1:5000/other").Encode(), 400},
		{"desktop lookalike host", "/api/auth/github/start?" + nativeParams("desktop", "http://127.0.0.1.evil.example:5000/callback").Encode(), 400},
		{"desktop userinfo", "/api/auth/github/start?" + nativeParams("desktop", "http://127.0.0.1:5000@evil.example/callback").Encode(), 400},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if rec := e.do(req{method: "GET", target: tt.target}); rec.Code != tt.want {
				t.Errorf("status = %d, want %d: %s", rec.Code, tt.want, rec.Body)
			}
		})
	}
}

func TestCallbackRejects(t *testing.T) {
	e := newEnv(t)
	start := e.do(req{method: "GET", target: "/api/auth/github/start"})
	cookies := start.Result().Cookies()
	loc, err := url.Parse(start.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	state := loc.Query().Get("state")

	tests := []struct {
		name, target string
		cookies      []*http.Cookie
		wantStatus   int
		wantLocation string
	}{
		{"no cookie", "/api/auth/github/callback?code=x&state=" + state, nil, 400, ""},
		{"wrong state", "/api/auth/github/callback?code=x&state=forged", cookies, 400, ""},
		{"other provider's cookie", "/api/auth/google/callback?code=x&state=" + state, cookies, 400, ""},
		{"user declined", "/api/auth/github/callback?error=access_denied&state=" + state, cookies, 302, "/login?error=access_denied"},
		{"bad code", "/api/auth/github/callback?code=x&state=" + state, cookies, 302, "/login?error=provider_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := e.do(req{method: "GET", target: tt.target, cookies: tt.cookies})
			if rec.Code != tt.wantStatus || rec.Header().Get("Location") != tt.wantLocation {
				t.Errorf("got %d %q", rec.Code, rec.Header().Get("Location"))
			}
			if cookieMap(rec)[cookieAccess] != nil {
				t.Error("a session was created")
			}
		})
	}

	t.Run("unverified email", func(t *testing.T) {
		e.fake.verified = false
		for _, p := range []string{"github", "google"} {
			if got := e.oauth(p, url.Values{}).Header().Get("Location"); got != "/login?error=email_unverified" {
				t.Errorf("%s: location = %q", p, got)
			}
		}
	})
}

func TestNativeSignInAsksFirst(t *testing.T) {
	e := newEnv(t)
	rec := e.callback("github", nativeParams("ios", ""))
	body := rec.Body.String()
	if rec.Code != 200 || rec.Header().Get("Location") != "" || !strings.Contains(body, "ada@example.com") || !strings.Contains(body, "Orbit iPhone app") {
		t.Fatalf("callback: %d %s", rec.Code, body)
	}
	if n := e.count("SELECT count(*) FROM login_codes"); n != 0 {
		t.Fatalf("%d codes issued before the user agreed", n)
	}
	token := hiddenToken.FindStringSubmatch(body)[1]
	access := e.signIn().AccessToken
	tests := []struct {
		name, token, site string
		want              int
	}{
		{"another site presses it", token, "cross-site", 403},
		{"forged", "nonsense", "same-origin", 400},
		{"a different kind of token", access, "same-origin", 400},
		{"the user", token, "same-origin", 303},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if rec := e.confirm(tt.token, tt.site); rec.Code != tt.want {
				t.Errorf("status = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

func TestCodeExchange(t *testing.T) {
	e := newEnv(t)
	tests := []struct {
		name, client, redirect, verifier string
		want                             int
	}{
		{"ios", "ios", "", verifier, 200},
		{"desktop on any port", "desktop", "http://127.0.0.1:51234/callback", verifier, 200},
		{"wrong verifier", "ios", "", strings.Repeat("w", 43), 400},
		{"no verifier", "ios", "", "", 400},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := e.oauth("github", nativeParams(tt.client, tt.redirect))
			wantPrefix := tt.redirect
			if wantPrefix == "" {
				wantPrefix = "orbit://auth/callback"
			}
			if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, wantPrefix+"?code=") {
				t.Fatalf("location = %q", loc)
			}
			code := codeFrom(t, rec)
			res, _ := e.token(map[string]string{"grant_type": "authorization_code", "code": code, "code_verifier": tt.verifier})
			if res.Code != tt.want {
				t.Fatalf("status = %d, want %d", res.Code, tt.want)
			}
			// Single use either way.
			again, _ := e.token(map[string]string{"grant_type": "authorization_code", "code": code, "code_verifier": verifier})
			if again.Code != http.StatusBadRequest {
				t.Errorf("reused code: %d", again.Code)
			}
		})
	}

	t.Run("expired code", func(t *testing.T) {
		code := codeFrom(t, e.oauth("github", nativeParams("ios", "")))
		e.exec("UPDATE login_codes SET expires_at = now() - interval '1 second'")
		res, _ := e.token(map[string]string{"grant_type": "authorization_code", "code": code, "code_verifier": verifier})
		if res.Code != http.StatusBadRequest {
			t.Errorf("status = %d", res.Code)
		}
	})
}

func (e *env) exec(sql string) {
	e.t.Helper()
	if _, err := e.pool.Exec(context.Background(), sql); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) count(sql string) (n int) {
	e.t.Helper()
	if err := e.pool.QueryRow(context.Background(), sql).Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}

func refresh(token string) map[string]string {
	return map[string]string{"grant_type": "refresh_token", "refresh_token": token}
}

func TestTokenRequestRejects(t *testing.T) {
	e := newEnv(t)
	tests := []struct {
		name, body string
		want       int
	}{
		{"unknown grant", `{"grant_type":"password"}`, 400},
		{"unknown field", `{"grant_type":"refresh_token","apple_id":"x"}`, 400},
		{"unknown token", `{"grant_type":"refresh_token","refresh_token":"nope"}`, 400},
		{"no token, no cookie", `{"grant_type":"refresh_token"}`, 400},
		{"not json", `grant_type=refresh_token`, 415},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if rec := e.do(req{method: "POST", target: "/api/auth/token", body: tt.body}); rec.Code != tt.want {
				t.Errorf("status = %d: %s", rec.Code, rec.Body)
			}
		})
	}
}

func TestRefreshRotationAndReuse(t *testing.T) {
	e := newEnv(t)
	first := e.signIn()
	rec, second := e.token(refresh(first.RefreshToken))
	if rec.Code != http.StatusOK || second.RefreshToken == first.RefreshToken {
		t.Fatalf("rotate: %d %s", rec.Code, rec.Body)
	}
	if n := e.count("SELECT count(DISTINCT expires_at) FROM refresh_tokens"); n != 1 {
		t.Errorf("rotation extended the session: %d expiry values", n)
	}
	if n := e.count("SELECT count(*) FROM refresh_tokens WHERE token_hash = convert_to('" + second.RefreshToken + "', 'utf8')"); n != 0 {
		t.Error("refresh token stored unhashed")
	}
	if rec, _ := e.token(refresh(first.RefreshToken)); rec.Code != http.StatusBadRequest {
		t.Fatalf("reuse accepted: %d", rec.Code)
	}
	if rec, _ := e.token(refresh(second.RefreshToken)); rec.Code != http.StatusBadRequest {
		t.Fatalf("family not revoked after reuse: %d", rec.Code)
	}
}

func TestRefreshExpiry(t *testing.T) {
	e := newEnv(t)
	pair := e.signIn()
	e.exec("UPDATE refresh_tokens SET expires_at = now() - interval '1 second'")
	if rec, _ := e.token(refresh(pair.RefreshToken)); rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestWebRefreshAndLogout(t *testing.T) {
	e := newEnv(t)
	session := cookieMap(e.oauth("github", url.Values{}))[cookieRefresh]
	csrf := map[string]string{csrfHeader: "1"}
	const body = `{"grant_type":"refresh_token"}`

	// A web token presented in the body, as a thief would, is refused.
	if rec, _ := e.token(refresh(session.Value)); rec.Code != http.StatusBadRequest {
		t.Errorf("web token accepted from the body: %d", rec.Code)
	}
	if rec := e.do(req{method: "POST", target: "/api/auth/token", body: body, cookies: []*http.Cookie{session}}); rec.Code != http.StatusBadRequest {
		t.Errorf("refresh without CSRF header: %d", rec.Code)
	}
	rec := e.do(req{method: "POST", target: "/api/auth/token", body: body, header: csrf, cookies: []*http.Cookie{session}})
	next := cookieMap(rec)[cookieRefresh]
	if rec.Code != http.StatusOK || next == nil || strings.Contains(rec.Body.String(), "_token") {
		t.Fatalf("web refresh: %d %s", rec.Code, rec.Body)
	}

	if rec := e.do(req{method: "POST", target: "/api/auth/logout", cookies: []*http.Cookie{next}}); rec.Code != http.StatusForbidden {
		t.Errorf("logout without CSRF header: %d", rec.Code)
	}
	out := e.do(req{method: "POST", target: "/api/auth/logout", header: csrf, cookies: []*http.Cookie{next}})
	if out.Code != http.StatusNoContent || cookieMap(out)[cookieRefresh].MaxAge >= 0 {
		t.Fatalf("logout: %d", out.Code)
	}
	after := e.do(req{method: "POST", target: "/api/auth/token", body: body, header: csrf, cookies: []*http.Cookie{next}})
	if after.Code != http.StatusBadRequest {
		t.Errorf("refresh after logout: %d", after.Code)
	}
}

func TestNativeLogout(t *testing.T) {
	e := newEnv(t)
	pair := e.signIn()
	rec := e.do(req{method: "POST", target: "/api/auth/logout", body: `{"refresh_token":"` + pair.RefreshToken + `"}`})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout: %d %s", rec.Code, rec.Body)
	}
	if rec, _ := e.token(refresh(pair.RefreshToken)); rec.Code != http.StatusBadRequest {
		t.Errorf("refresh after logout: %d", rec.Code)
	}
}

func TestRequireUser(t *testing.T) {
	e := newEnv(t)
	pair := e.signIn()
	sign := func(aud string, ttl time.Duration, secret string) string {
		other := &Service{cfg: Config{JWTSecret: []byte(secret)}}
		_, me := e.me(req{header: bearer(pair.AccessToken)})
		tok, err := other.sign(claims{RegisteredClaims: jwt.RegisteredClaims{Subject: me.ID}}, aud, ttl)
		if err != nil {
			t.Fatal(err)
		}
		return tok
	}
	good := strings.Repeat("k", 32)
	unsigned, err := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{"aud": audAccess, "sub": "x", "exp": time.Now().Add(time.Hour).Unix()}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, token string
		want        int
	}{
		{"valid", pair.AccessToken, 200},
		{"missing", "", 401},
		{"garbage", "abc", 401},
		{"expired", sign(audAccess, -time.Minute, good), 401},
		{"other audience", sign(audDelete, time.Minute, good), 401},
		{"other secret", sign(audAccess, time.Minute, strings.Repeat("x", 32)), 401},
		{"alg none", unsigned, 401},
		{"refresh token", pair.RefreshToken, 401},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if status, _ := e.me(req{header: bearer(tt.token)}); status != tt.want {
				t.Errorf("status = %d, want %d", status, tt.want)
			}
		})
	}
}

func TestAccountLinking(t *testing.T) {
	magic := func(e *env) *httptest.ResponseRecorder {
		e.emailStart(`{"email":"` + strings.ToLower(e.fake.email) + `"}`)
		return e.submit(e.sentToken(), "same-origin")
	}
	oauth := func(provider string) func(*env) *httptest.ResponseRecorder {
		return func(e *env) *httptest.ResponseRecorder { return e.oauth(provider, url.Values{}) }
	}
	tests := []struct {
		name, email, hd string
		first, second   func(*env) *httptest.ResponseRecorder
		linked          bool
	}{
		{"magic link joins", "ada@corp.test", "", oauth("github"), magic, true},
		{"gmail is authoritative", "ada@gmail.com", "", oauth("github"), oauth("google"), true},
		{"workspace domain is authoritative", "ada@corp.test", "corp.test", oauth("github"), oauth("google"), true},
		{"google for a foreign mailbox", "ada@corp.test", "", oauth("github"), oauth("google"), false},
		{"another workspace domain", "ada@corp.test", "other.test", oauth("github"), oauth("google"), false},
		{"github never joins", "ada@gmail.com", "", magic, oauth("github"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			e.fake.email, e.fake.hostedDomain = tt.email, tt.hd
			tt.first(e)
			rec := tt.second(e)
			want := 1
			if tt.linked {
				want = 2
			} else if got := rec.Header().Get("Location"); got != "/login?error=account_exists" || cookieMap(rec)[cookieAccess] != nil {
				t.Errorf("second sign-in: %d %q", rec.Code, got)
			}
			if users, ids := e.count("SELECT count(*) FROM users"), e.count("SELECT count(*) FROM identities"); users != 1 || ids != want {
				t.Errorf("users = %d, identities = %d, want %d", users, ids, want)
			}
		})
	}
}

func TestLookalikeEmailRefused(t *testing.T) {
	e := newEnv(t)
	e.fake.email = "ada@gma\u0130l.com"
	if got := e.oauth("github", url.Values{}).Header().Get("Location"); got != "/login?error=email_unverified" {
		t.Errorf("provider: %q", got)
	}
	if rec := e.emailStart(`{"email":"ada@gma\u0130l.com"}`); rec.Code != 400 {
		t.Errorf("magic link: %d", rec.Code)
	}
}

// A rotation racing a revocation must not leave a live token behind.
func TestRevocationRace(t *testing.T) {
	e := newEnv(t)
	for i := range 25 {
		first := e.signIn()
		_, second := e.token(refresh(first.RefreshToken))
		var wg sync.WaitGroup
		wg.Go(func() { _, _ = e.svc.rotate(context.Background(), second.RefreshToken, ClientIOS) })
		wg.Go(func() { _, _ = e.svc.rotate(context.Background(), first.RefreshToken, ClientIOS) }) // reuse
		wg.Wait()
		if live := e.count("SELECT count(*) FROM refresh_tokens WHERE revoked_at IS NULL AND used_at IS NULL"); live != 0 {
			t.Fatalf("round %d: %d token(s) survived reuse detection", i, live)
		}
	}
}

func TestDeleteAccount(t *testing.T) {
	e := newEnv(t)
	pair := e.signIn()
	auth := bearer(pair.AccessToken)
	e.exec("INSERT INTO devices (id, user_id, name) SELECT gen_random_uuid(), id, 'phone' FROM users")
	e.exec("INSERT INTO sources (id, user_id, url) SELECT gen_random_uuid(), id, 'https://example.com/s.json' FROM users")

	session := cookieMap(e.oauth("github", url.Values{}))[cookieAccess]
	if rec := e.do(req{method: "DELETE", target: "/api/me", body: `{}`, cookies: []*http.Cookie{session}}); rec.Code != http.StatusForbidden {
		t.Errorf("cookie delete without CSRF header: %d", rec.Code)
	}
	for _, body := range []string{`{}`, `{"confirmation_token":"` + pair.AccessToken + `"}`} {
		if rec := e.do(req{method: "DELETE", target: "/api/me", body: body, header: auth}); rec.Code != http.StatusBadRequest {
			t.Errorf("delete with %s: %d", body, rec.Code)
		}
	}

	intent := e.do(req{method: "POST", target: "/api/me/delete-intent", header: auth})
	var got struct {
		Token string `json:"confirmation_token"`
	}
	if err := json.Unmarshal(intent.Body.Bytes(), &got); err != nil || got.Token == "" {
		t.Fatalf("intent: %d %s", intent.Code, intent.Body)
	}
	rec := e.do(req{method: "DELETE", target: "/api/me", body: `{"confirmation_token":"` + got.Token + `"}`, header: auth})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	for _, table := range []string{"identities", "refresh_tokens", "login_codes", "devices", "sources", "push_subscriptions"} {
		if n := e.count("SELECT count(*) FROM " + table); n != 0 {
			t.Errorf("%s still has %d rows", table, n)
		}
	}
	if n := e.count("SELECT count(*) FROM users WHERE email IS NULL AND display_name IS NULL AND deleted_at IS NOT NULL"); n != 1 {
		t.Error("user row was not anonymised")
	}
	if status, _ := e.me(req{header: auth}); status != http.StatusUnauthorized {
		t.Errorf("deleted account still authenticates: %d", status)
	}
	// The address is free to register again.
	if status, me := e.me(req{header: bearer(e.signIn().AccessToken)}); status != 200 || me.Email != "ada@example.com" {
		t.Errorf("re-register: %d %+v", status, me)
	}
}

var linkToken = regexp.MustCompile(`https://orbit\.test/api/auth/email/verify\?token=([A-Za-z0-9_-]+)`)

func (e *env) emailStart(body string) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.do(req{method: "POST", target: "/api/auth/email/start", body: body})
}

func (e *env) sentToken() string {
	e.t.Helper()
	m := linkToken.FindStringSubmatch(e.mail.body)
	if m == nil {
		e.t.Fatalf("no link in %q", e.mail.body)
	}
	return m[1]
}

func (e *env) submit(token, site string) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.do(req{method: "POST", target: "/api/auth/email/verify", body: url.Values{"token": {token}}.Encode(),
		header: map[string]string{"Content-Type": "application/x-www-form-urlencoded", "Sec-Fetch-Site": site}})
}

func TestEmailStart(t *testing.T) {
	e := newEnv(t)
	tests := []struct {
		name, body string
		want       int
	}{
		{"web", `{"email":"New@Example.com"}`, 202},
		{"ios", `{"email":"a@example.com","client":"ios","code_challenge":"` + challenge + `"}`, 202},
		{"bad address", `{"email":"not-an-email"}`, 400},
		{"display name", `{"email":"Ada <a@example.com>"}`, 400},
		{"header injection", `{"email":"a@example.com\r\nBcc: b@example.com"}`, 400},
		{"native without challenge", `{"email":"a@example.com","client":"ios"}`, 400},
		{"unknown field", `{"email":"a@example.com","password":"x"}`, 400},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if rec := e.emailStart(tt.body); rec.Code != tt.want {
				t.Errorf("status = %d: %s", rec.Code, rec.Body)
			}
		})
	}
}

func TestEmailRateLimit(t *testing.T) {
	e := newEnv(t)
	for i, want := range []int{202, 202, 202, 429} {
		if rec := e.emailStart(`{"email":"a@example.com"}`); rec.Code != want {
			t.Fatalf("request %d: %d", i+1, rec.Code)
		}
	}
}

func TestEmailSendFailure(t *testing.T) {
	e := newEnv(t)
	e.mail.err = errors.New("smtp down")
	if rec := e.emailStart(`{"email":"a@example.com"}`); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestMagicLinkWeb(t *testing.T) {
	e := newEnv(t)
	e.emailStart(`{"email":"New@Example.com"}`)
	if e.mail.to != "new@example.com" {
		t.Fatalf("sent to %q", e.mail.to)
	}
	token := e.sentToken()

	for range 2 { // opening the link must not use it up
		page := e.do(req{method: "GET", target: "/api/auth/email/verify?token=" + token})
		if page.Code != 200 || !strings.Contains(page.Body.String(), "new@example.com") || !strings.Contains(page.Header().Get("Content-Security-Policy"), "default-src 'none'") {
			t.Fatalf("confirm page: %d %s", page.Code, page.Body)
		}
	}
	if rec := e.submit(token, "cross-site"); rec.Code != http.StatusForbidden {
		t.Fatalf("cross-site submit: %d", rec.Code)
	}
	rec := e.submit(token, "same-origin")
	if rec.Code != http.StatusSeeOther || cookieMap(rec)[cookieAccess] == nil {
		t.Fatalf("verify: %d %s", rec.Code, rec.Body)
	}
	status, me := e.me(req{cookies: []*http.Cookie{cookieMap(rec)[cookieAccess]}})
	if status != 200 || me.Email != "new@example.com" || me.Identities[0] != "email" {
		t.Fatalf("me: %d %+v", status, me)
	}
	if rec := e.submit(token, "same-origin"); rec.Code != http.StatusBadRequest {
		t.Errorf("link worked twice: %d", rec.Code)
	}
}

func TestMagicLinkNative(t *testing.T) {
	e := newEnv(t)
	e.emailStart(`{"email":"a@example.com","client":"desktop","code_challenge":"` + challenge + `","redirect":"http://127.0.0.1:5000/callback"}`)
	rec := e.submit(e.sentToken(), "same-origin")
	if !strings.HasPrefix(rec.Header().Get("Location"), "http://127.0.0.1:5000/callback?code=") {
		t.Fatalf("location = %q", rec.Header().Get("Location"))
	}
	res, pair := e.token(map[string]string{"grant_type": "authorization_code", "code": codeFrom(t, rec), "code_verifier": verifier})
	if res.Code != 200 || pair.AccessToken == "" {
		t.Fatalf("exchange: %d %s", res.Code, res.Body)
	}
}

func TestMagicLinkRejects(t *testing.T) {
	e := newEnv(t)
	e.emailStart(`{"email":"a@example.com"}`)
	token := e.sentToken()
	// A magic-link token is not an app exchange code.
	if res, _ := e.token(map[string]string{"grant_type": "authorization_code", "code": token, "code_verifier": verifier}); res.Code != 400 {
		t.Errorf("magic token exchanged: %d", res.Code)
	}
	e.exec("UPDATE login_codes SET expires_at = now() - interval '1 second'")
	tests := []struct{ name, token string }{{"expired", token}, {"unknown", "nope"}, {"empty", ""}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if rec := e.do(req{method: "GET", target: "/api/auth/email/verify?token=" + tt.token}); rec.Code != 400 {
				t.Errorf("GET: %d", rec.Code)
			}
			if rec := e.submit(tt.token, "same-origin"); rec.Code != 400 {
				t.Errorf("POST: %d", rec.Code)
			}
		})
	}
}

func TestMethods(t *testing.T) {
	e := newEnv(t)
	rec := e.do(req{method: "GET", target: "/api/auth/methods"})
	if rec.Code != 200 || rec.Body.String() != `{"email":true,"github":true,"google":true}`+"\n" {
		t.Errorf("got %d %s", rec.Code, rec.Body)
	}
}

func TestSessions(t *testing.T) {
	e := newEnv(t)
	app := e.signIn()
	web := cookieMap(e.oauth("github", url.Values{}))
	// The request cookie carries the __Host- name the server set.
	cookies := []*http.Cookie{web[cookieAccess], web[cookieRefresh]}
	list := func() []sessionView {
		t.Helper()
		rec := e.do(req{method: "GET", target: "/api/me/sessions", cookies: cookies})
		var body struct{ Sessions []sessionView }
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("%d %s", rec.Code, rec.Body)
		}
		return body.Sessions
	}
	// Alerts turned on from a session end with it.
	subscribe := func(session uuid.UUID) {
		t.Helper()
		e.exec("INSERT INTO push_subscriptions (id, user_id, endpoint, p256dh, auth, session_id) SELECT gen_random_uuid(), u.id, '" +
			uuid.NewString() + "', 'k', 'a', '" + session.String() + "' FROM users u WHERE u.email = 'ada@example.com'")
	}
	// Rotation must not create a second session.
	_, app2 := e.token(refresh(app.RefreshToken))
	sessions := list()
	if len(sessions) != 2 {
		t.Fatalf("%d sessions", len(sessions))
	}
	var other, mine uuid.UUID
	for _, s := range sessions {
		if s.Current {
			mine = s.ID
		} else if s.Client == ClientIOS {
			other = s.ID
		}
	}
	if mine == uuid.Nil || other == uuid.Nil {
		t.Fatalf("sessions = %+v", sessions)
	}
	revoke := func(token string, id uuid.UUID) int {
		return e.do(req{method: "DELETE", target: "/api/me/sessions/" + id.String(), header: bearer(token)}).Code
	}
	e.fake.email = "rival@gmail.com"
	rival := cookieMap(e.oauth("google", url.Values{}))[cookieAccess]
	if rec := e.do(req{method: "DELETE", target: "/api/me/sessions/" + other.String(), cookies: []*http.Cookie{rival}, header: map[string]string{csrfHeader: "1"}}); rec.Code != 404 {
		t.Errorf("another account's session: %d", rec.Code)
	}
	if code := revoke(app2.AccessToken, uuid.New()); code != 404 {
		t.Errorf("unknown session: %d", code)
	}
	subscribe(other)
	subscribe(mine)
	if code := revoke(app2.AccessToken, other); code != 204 {
		t.Fatalf("revoke: %d", code)
	}
	if rec, _ := e.token(refresh(app2.RefreshToken)); rec.Code != 400 {
		t.Errorf("revoked session still refreshes: %d", rec.Code)
	}
	if got := list(); len(got) != 1 || got[0].ID != mine {
		t.Errorf("after revoke: %+v", got)
	}
	if n := e.count("SELECT count(*) FROM push_subscriptions"); n != 1 {
		t.Errorf("%d subscriptions after revoking one session", n)
	}
	out := e.do(req{method: "POST", target: "/api/auth/logout", cookies: cookies, header: map[string]string{csrfHeader: "1"}})
	if out.Code != 204 || e.count("SELECT count(*) FROM push_subscriptions") != 0 {
		t.Errorf("sign-out left alerts on (%d)", out.Code)
	}
}
