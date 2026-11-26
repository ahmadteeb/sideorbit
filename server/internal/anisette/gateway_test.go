package anisette

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/ahmadteeb/sideorbit/server/internal/auth"
)

var (
	userA   = uuid.New()
	userB   = uuid.New()
	deviceA = uuid.New()
)

type fakeAuth struct{}

func (fakeAuth) Authenticate(_ context.Context, token string) (uuid.UUID, error) {
	switch token {
	case "token-a":
		return userA, nil
	case "token-b":
		return userB, nil
	}
	return uuid.Nil, auth.ErrInvalidToken
}

type fakeDevices struct{}

func (fakeDevices) Owns(_ context.Context, user, device uuid.UUID) (bool, error) {
	return user == userA && device == deviceA, nil
}

// upstream imitates anisette-v3-server and records what reaches it.
type upstream struct {
	mu      sync.Mutex
	headers []http.Header
	bodies  []string
	paths   []string
}

func (u *upstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body := []byte{}
	if r.Header.Get("Upgrade") == "" {
		body, _ = io.ReadAll(r.Body)
	}
	u.mu.Lock()
	u.headers, u.bodies, u.paths = append(u.headers, r.Header.Clone()), append(u.bodies, string(body)), append(u.paths, r.URL.Path)
	u.mu.Unlock()
	if r.URL.Path != "/v3/provisioning_session" {
		w.Header().Set("Set-Cookie", "upstream=1")
		_, _ = io.WriteString(w, "SECRET-RESPONSE")
		return
	}
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.CloseNow()
	ctx := context.Background()
	if r.UserAgent() == "hang-up" {
		return // the upstream closes first
	}
	if conn.Write(ctx, websocket.MessageText, []byte(`{"result":"GiveIdentifier"}`)) != nil {
		return
	}
	for {
		kind, msg, err := conn.Read(ctx)
		if err != nil {
			return
		}
		if conn.Write(ctx, kind, append([]byte("echo:"), msg...)) != nil {
			return
		}
	}
}

type env struct {
	srv  *httptest.Server
	up   *upstream
	logs *logBuffer
}

// logBuffer is written by handler goroutines while tests read it.
type logBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (l *logBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *logBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// waitFor returns the log once it contains want.
func (l *logBuffer) waitFor(t *testing.T, want string) string {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if s := l.String(); strings.Contains(s, want) {
			return s
		}
	}
	t.Fatalf("log never contained %q:\n%s", want, l.String())
	return ""
}

// newEnv serves the gateway over real HTTP behind the same logging wrapper and
// read timeout as production.
func newEnv(t *testing.T, change func(*Config)) env {
	t.Helper()
	up := &upstream{}
	upSrv := httptest.NewServer(up)
	t.Cleanup(upSrv.Close)
	target, err := url.Parse(upSrv.URL)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{Upstream: target, HeadersPerMinUser: 60, ProvisionPerHourUser: 5, ProvisionPerHourDevice: 3,
		ProvisionPerHourIP: 100, ProvisionPerHourTotal: 1000,
		RequestsPerMinIP: 1000, SessionTimeout: time.Minute}
	if change != nil {
		change(&cfg)
	}
	logs := &logBuffer{}
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(middleware.NewWrapResponseWriter(w, r.ProtoMajor), r)
		})
	})
	New(cfg, fakeAuth{}, fakeDevices{}, slog.New(slog.NewTextHandler(logs, nil))).Routes(r)
	srv := httptest.NewUnstartedServer(r)
	srv.Config.ReadTimeout = 300 * time.Millisecond
	srv.Start()
	t.Cleanup(srv.Close)
	return env{srv, up, logs}
}

func (e env) do(t *testing.T, method, path, body string, header map[string]string) (*http.Response, string) {
	t.Helper()
	r, err := http.NewRequest(method, e.srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range header {
		r.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res, string(b)
}

func bearer(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

func (e env) dial(t *testing.T, header http.Header, protocols ...string) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return websocket.Dial(ctx, e.srv.URL+"/anisette/v3/provisioning_session",
		&websocket.DialOptions{HTTPHeader: header, Subprotocols: protocols})
}

var endpoints = []struct{ method, path string }{
	{"GET", "/anisette/v3/client_info"},
	{"POST", "/anisette/v3/get_headers"},
	{"GET", "/anisette/v3/provisioning_session"},
}

func TestRequiresToken(t *testing.T) {
	e := newEnv(t, nil)
	tests := []struct {
		name   string
		header map[string]string
	}{
		{"none", nil},
		{"wrong", bearer("nope")},
		{"cookie only", map[string]string{"Cookie": "__Host-orbit_access=token-a; orbit_access=token-a"}},
		{"query string", nil},
	}
	for _, ep := range endpoints {
		for _, tt := range tests {
			t.Run(ep.path+" "+tt.name, func(t *testing.T) {
				path := ep.path
				if tt.name == "query string" {
					path += "?access_token=token-a"
				}
				if res, _ := e.do(t, ep.method, path, "", tt.header); res.StatusCode != http.StatusUnauthorized {
					t.Errorf("status = %d", res.StatusCode)
				}
			})
		}
	}
	// The subprotocol form works only for the WebSocket.
	if res, _ := e.do(t, "GET", "/anisette/v3/client_info", "", map[string]string{"Sec-WebSocket-Protocol": "orbit.bearer.token-a"}); res.StatusCode != 401 {
		t.Errorf("subprotocol token on HTTP: %d", res.StatusCode)
	}
	if len(e.up.paths) != 0 {
		t.Errorf("unauthenticated requests reached the upstream: %v", e.up.paths)
	}
}

func TestProxyHTTP(t *testing.T) {
	e := newEnv(t, nil)
	header := bearer("token-a")
	header["Cookie"] = "session=abc"
	header["X-Orbit-Device"] = deviceA.String()
	header["Content-Type"] = "application/json"

	res, body := e.do(t, "GET", "/anisette/v3/client_info", "", header)
	if res.StatusCode != 200 || body != "SECRET-RESPONSE" || res.Header.Get("Set-Cookie") != "" {
		t.Fatalf("client_info: %d %q %v", res.StatusCode, body, res.Header)
	}
	res, _ = e.do(t, "POST", "/anisette/v3/get_headers", `{"adi_pb":"SECRET-BODY"}`, header)
	if res.StatusCode != 200 || e.up.bodies[1] != `{"adi_pb":"SECRET-BODY"}` {
		t.Fatalf("get_headers: %d %q", res.StatusCode, e.up.bodies)
	}
	if e.up.paths[0] != "/v3/client_info" || e.up.paths[1] != "/v3/get_headers" {
		t.Errorf("paths = %v", e.up.paths)
	}
	for _, h := range e.up.headers {
		for _, name := range []string{"Authorization", "Cookie", "X-Orbit-Device", "X-Forwarded-For", "Forwarded"} {
			if h.Get(name) != "" {
				t.Errorf("upstream received %s", name)
			}
		}
	}
	logs := e.logs.String()
	for _, want := range []string{userA.String(), deviceA.String(), "endpoint=v3/get_headers", "status=200", "duration_ms="} {
		if !strings.Contains(logs, want) {
			t.Errorf("log is missing %s", want)
		}
	}
	for _, leak := range []string{"SECRET-BODY", "SECRET-RESPONSE", "token-a", "127.0.0.1:"} {
		if strings.Contains(logs, leak) {
			t.Errorf("log contains %q:\n%s", leak, logs)
		}
	}

	if res, _ := e.do(t, "POST", "/anisette/v3/get_headers", strings.Repeat("a", maxBody+1), header); res.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized body: %d", res.StatusCode)
	}
}

func TestProxyWebSocket(t *testing.T) {
	tests := []struct {
		name      string
		header    http.Header
		protocols []string
	}{
		{"authorization header", http.Header{"Authorization": {"Bearer token-a"}}, nil},
		{"subprotocol", nil, []string{"orbit.bearer.token-a"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t, nil)
			conn, res, err := e.dial(t, tt.header, tt.protocols...)
			if err != nil {
				t.Fatalf("dial: %v", err)
			}
			defer conn.CloseNow()
			if len(tt.protocols) > 0 && res.Header.Get("Sec-WebSocket-Protocol") != tt.protocols[0] {
				t.Errorf("subprotocol = %q", res.Header.Get("Sec-WebSocket-Protocol"))
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			read := func(want string) {
				t.Helper()
				_, msg, err := conn.Read(ctx)
				if err != nil || string(msg) != want {
					t.Fatalf("read %q, %v; want %q", msg, err, want)
				}
			}
			read(`{"result":"GiveIdentifier"}`)
			time.Sleep(500 * time.Millisecond) // outlive the server's ReadTimeout
			if err := conn.Write(ctx, websocket.MessageText, []byte("SECRET-MESSAGE")); err != nil {
				t.Fatal(err)
			}
			read("echo:SECRET-MESSAGE")
			_ = conn.Close(websocket.StatusNormalClosure, "")

			e.up.mu.Lock()
			h := e.up.headers[0]
			e.up.mu.Unlock()
			if h.Get("Authorization") != "" || h.Get("Sec-Websocket-Protocol") != "" {
				t.Errorf("upstream received the token: %v", h)
			}
			if logs := e.logs.waitFor(t, "status=101"); strings.Contains(logs, "SECRET-MESSAGE") || strings.Contains(logs, "token-a") {
				t.Errorf("log = %s", logs)
			}
		})
	}
}

func TestSessionTimeout(t *testing.T) {
	e := newEnv(t, func(c *Config) { c.SessionTimeout = 200 * time.Millisecond })
	conn, _, err := e.dial(t, http.Header{"Authorization": {"Bearer token-a"}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		if _, _, err := conn.Read(ctx); err != nil {
			break
		}
	}
	if ctx.Err() != nil {
		t.Fatal("an idle session was never closed")
	}
}

// The upstream hangs up and the client neither reads nor closes.
func TestSessionEndsWhenClientStaysSilent(t *testing.T) {
	e := newEnv(t, func(c *Config) { c.SessionTimeout = 300 * time.Millisecond })
	conn, err := net.Dial("tcp", e.srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, err = io.WriteString(conn, "GET /anisette/v3/provisioning_session HTTP/1.1\r\nHost: x\r\n"+
		"Authorization: Bearer token-a\r\nUser-Agent: hang-up\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n"+
		"Sec-WebSocket-Version: 13\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n\r\n")
	if err != nil {
		t.Fatal(err)
	}
	e.logs.waitFor(t, "status=101") // logged only once the handler returns
}

func TestUpgradeAndBodyRestrictions(t *testing.T) {
	e := newEnv(t, nil)
	upgrade := bearer("token-a")
	upgrade["Connection"], upgrade["Upgrade"] = "Upgrade", "websocket"
	if res, _ := e.do(t, "GET", "/anisette/v3/client_info", "", upgrade); res.StatusCode != 200 {
		t.Fatalf("client_info: %d", res.StatusCode)
	}
	if got := e.up.headers[0].Get("Upgrade"); got != "" {
		t.Errorf("a plain route forwarded Upgrade: %q", got)
	}
	if res, _ := e.do(t, "GET", "/anisette/v3/provisioning_session", "body", upgrade); res.StatusCode != 400 {
		t.Errorf("WebSocket request with a body: %d", res.StatusCode)
	}
	if len(e.up.paths) != 1 {
		t.Errorf("upstream saw %v", e.up.paths)
	}
}

func TestLimits(t *testing.T) {
	withDevice := func(token string) map[string]string {
		h := bearer(token)
		h["X-Orbit-Device"] = deviceA.String()
		return h
	}
	status := func(t *testing.T, e env, path string, h map[string]string) int {
		t.Helper()
		method := "GET"
		if strings.HasSuffix(path, "get_headers") {
			method = "POST"
		}
		res, _ := e.do(t, method, "/anisette/v3/"+path, "", h)
		if res.StatusCode == 429 && res.Header.Get("Retry-After") == "" {
			t.Error("429 without Retry-After")
		}
		return res.StatusCode
	}
	// A provisioning request without an upgrade gets 426, after the quotas.
	tests := []struct {
		name   string
		change func(*Config)
		path   string
		header map[string]string
		want   []int
	}{
		{"headers per user", func(c *Config) { c.HeadersPerMinUser = 2 }, "get_headers", bearer("token-a"), []int{200, 200, 429}},
		{"provisioning per user", func(c *Config) { c.ProvisionPerHourUser = 2 }, "provisioning_session", bearer("token-a"), []int{426, 426, 429}},
		{"provisioning per device", func(c *Config) { c.ProvisionPerHourDevice = 1 }, "provisioning_session", withDevice("token-a"), []int{426, 429}},
		{"provisioning per address", func(c *Config) { c.ProvisionPerHourIP = 1 }, "provisioning_session", bearer("token-a"), []int{426, 429}},
		{"per address", func(c *Config) { c.RequestsPerMinIP = 2 }, "client_info", nil, []int{401, 401, 429}},
		{"someone else's device", nil, "get_headers", withDevice("token-b"), []int{403}},
		{"malformed device", nil, "get_headers", map[string]string{"Authorization": "Bearer token-a", "X-Orbit-Device": "x"}, []int{403}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t, tt.change)
			for i, want := range tt.want {
				if got := status(t, e, tt.path, tt.header); got != want {
					t.Fatalf("request %d: %d, want %d", i+1, got, want)
				}
			}
		})
	}
	t.Run("overall provisioning cap spans accounts", func(t *testing.T) {
		e := newEnv(t, func(c *Config) { c.ProvisionPerHourTotal = 1 })
		a, b := status(t, e, "provisioning_session", bearer("token-a")), status(t, e, "provisioning_session", bearer("token-b"))
		if a != 426 || b != 429 {
			t.Errorf("got %d %d", a, b)
		}
	})
	t.Run("one user's limit leaves others alone", func(t *testing.T) {
		e := newEnv(t, func(c *Config) { c.HeadersPerMinUser = 1 })
		got := []int{status(t, e, "get_headers", bearer("token-a")), status(t, e, "get_headers", bearer("token-a")), status(t, e, "get_headers", bearer("token-b"))}
		if got[0] != 200 || got[1] != 429 || got[2] != 200 {
			t.Errorf("got %v", got)
		}
	})
}

func TestUpstreamDown(t *testing.T) {
	e := newEnv(t, func(c *Config) { c.Upstream = &url.URL{Scheme: "http", Host: "127.0.0.1:1"} })
	res, body := e.do(t, "GET", "/anisette/v3/client_info", "", bearer("token-a"))
	if res.StatusCode != http.StatusBadGateway || !strings.Contains(body, "anisette_unavailable") {
		t.Errorf("got %d %s", res.StatusCode, body)
	}
}
