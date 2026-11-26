// Package anisette is the authenticated gateway to the internal
// anisette-v3-server. Only signed-in users reach it, within quotas, and nothing
// from a request or response body is ever logged.
package anisette

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ahmadteeb/sideorbit/server/internal/auth"
	"github.com/ahmadteeb/sideorbit/server/internal/httpx"
	"github.com/ahmadteeb/sideorbit/server/internal/ratelimit"
)

const (
	// DeviceHeader names the registered device making the request, if any.
	DeviceHeader = "X-Orbit-Device"
	// wsTokenPrefix carries the access token for clients that can't set an
	// Authorization header on a WebSocket.
	wsTokenPrefix = "orbit.bearer."

	maxBody = 64 << 10
)

// forwarded lists the only request headers the upstream sees. In particular it
// never receives the caller's token, cookies or address.
var forwarded = []string{
	"Accept", "Content-Type", "Content-Length", "User-Agent",
	"Connection", "Upgrade", "Sec-Websocket-Key", "Sec-Websocket-Version", "Sec-Websocket-Extensions",
}

// Config holds the upstream address and quotas.
type Config struct {
	Upstream               *url.URL
	HeadersPerMinUser      int
	ProvisionPerHourUser   int
	ProvisionPerHourDevice int
	RequestsPerMinIP       int
	SessionTimeout         time.Duration // longest provisioning WebSocket
}

// Authenticator resolves an access token to an account.
type Authenticator interface {
	Authenticate(ctx context.Context, token string) (uuid.UUID, error)
}

// Devices answers whether a device belongs to an account.
type Devices interface {
	Owns(ctx context.Context, userID, deviceID uuid.UUID) (bool, error)
}

// Gateway proxies the three anisette v3 endpoints.
type Gateway struct {
	cfg     Config
	auth    Authenticator
	devices Devices
	log     *slog.Logger
	proxy   *httputil.ReverseProxy

	ip, headers, provisionUser, provisionDevice *ratelimit.Limiter
}

// call is what one request's proxying and logging need to share.
type call struct {
	path        string
	subprotocol string
	status      int
}

type callKey struct{}

// New builds the gateway.
func New(cfg Config, authn Authenticator, devices Devices, log *slog.Logger) *Gateway {
	g := &Gateway{
		cfg: cfg, auth: authn, devices: devices, log: log,
		ip:              ratelimit.New(cfg.RequestsPerMinIP, time.Minute),
		headers:         ratelimit.New(cfg.HeadersPerMinUser, time.Minute),
		provisionUser:   ratelimit.New(cfg.ProvisionPerHourUser, time.Hour),
		provisionDevice: ratelimit.New(cfg.ProvisionPerHourDevice, time.Hour),
	}
	g.proxy = &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			c := pr.In.Context().Value(callKey{}).(*call)
			target := *cfg.Upstream
			// Not JoinPath: with an empty base path it drops the leading slash.
			target.Path, target.RawPath = strings.TrimSuffix(target.Path, "/")+"/"+c.path, ""
			target.RawQuery = ""
			pr.Out.URL = &target
			pr.Out.Host = cfg.Upstream.Host
			header := http.Header{}
			for _, name := range forwarded {
				if v := pr.Out.Header.Values(name); len(v) > 0 {
					header[name] = v
				}
			}
			pr.Out.Header = header
		},
		ModifyResponse: func(res *http.Response) error {
			c := res.Request.Context().Value(callKey{}).(*call)
			c.status = res.StatusCode
			res.Header.Del("Set-Cookie")
			// A client that offered a subprotocol expects one back.
			if res.StatusCode == http.StatusSwitchingProtocols && c.subprotocol != "" {
				res.Header.Set("Sec-WebSocket-Protocol", c.subprotocol)
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			c := r.Context().Value(callKey{}).(*call)
			var tooBig *http.MaxBytesError
			if errors.As(err, &tooBig) {
				c.status = http.StatusRequestEntityTooLarge
				httpx.Error(w, c.status, "body_too_large", "The request body is too large.", "")
				return
			}
			c.status = http.StatusBadGateway
			log.Error("anisette upstream failed", "endpoint", c.path, "err", err)
			httpx.Error(w, c.status, "anisette_unavailable", "The anisette server isn't responding.", "Try again in a few minutes.")
		},
		ErrorLog: slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}
	return g
}

// Routes registers the endpoints on r.
func (g *Gateway) Routes(r chi.Router) {
	r.Get("/anisette/v3/client_info", g.handle("v3/client_info", false))
	r.Post("/anisette/v3/get_headers", g.handle("v3/get_headers", false))
	r.Get("/anisette/v3/provisioning_session", g.handle("v3/provisioning_session", true))
}

// token returns the bearer token and, for a WebSocket that sent it that way,
// the subprotocol that carried it. Cookies are not accepted here.
func token(r *http.Request, websocket bool) (tok, subprotocol string) {
	if t := auth.BearerToken(r); t != "" {
		return t, ""
	}
	if websocket {
		for _, line := range r.Header.Values("Sec-Websocket-Protocol") {
			for p := range strings.SplitSeq(line, ",") {
				p = strings.TrimSpace(p)
				if t, ok := strings.CutPrefix(p, wsTokenPrefix); ok {
					return t, p
				}
			}
		}
	}
	return "", ""
}

func (g *Gateway) handle(path string, websocket bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		start := time.Now()
		var userID uuid.UUID
		var deviceID string
		c := &call{path: path}
		defer func() {
			// Metadata only (docs/SECURITY.md).
			g.log.Info("anisette", "user_id", userID, "device_id", deviceID, "endpoint", path,
				"status", c.status, "duration_ms", time.Since(start).Milliseconds())
		}()
		limited := func(scope string, retry time.Duration) {
			c.status = http.StatusTooManyRequests
			g.log.Warn("rate limit hit", "endpoint", path, "scope", scope, "user_id", userID)
			w.Header().Set("Retry-After", strconv.Itoa(int(retry/time.Second)))
			httpx.Error(w, c.status, "rate_limited", "Too many anisette requests.", "Wait a while, then try again.")
		}

		// By address first, so unauthenticated floods stay cheap.
		if !g.ip.Allow(httpx.ClientIP(r)) {
			limited("ip", time.Minute)
			return
		}
		tok, subprotocol := token(r, websocket)
		id, err := g.auth.Authenticate(r.Context(), tok)
		if auth.IsInvalidToken(err) {
			c.status = http.StatusUnauthorized
			auth.Unauthorized(w)
			return
		}
		if err != nil {
			c.status = http.StatusInternalServerError
			httpx.Internal(w, r, err)
			return
		}
		userID, c.subprotocol = id, subprotocol

		var device uuid.UUID
		if v := r.Header.Get(DeviceHeader); v != "" {
			device, err = uuid.Parse(v)
			owned := false
			if err == nil {
				if owned, err = g.devices.Owns(r.Context(), userID, device); err != nil {
					c.status = http.StatusInternalServerError
					httpx.Internal(w, r, err)
					return
				}
			}
			if !owned {
				c.status = http.StatusForbidden
				httpx.Error(w, c.status, "unknown_device", "That device isn't registered to this account.", "Register the device again.")
				return
			}
			deviceID = device.String()
		}

		ctx := context.WithValue(r.Context(), callKey{}, c)
		if websocket {
			if !g.provisionUser.Allow(userID.String()) {
				limited("user", time.Hour)
				return
			}
			if deviceID != "" && !g.provisionDevice.Allow(deviceID) {
				limited("device", time.Hour)
				return
			}
			if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
				c.status = http.StatusUpgradeRequired
				w.Header().Set("Upgrade", "websocket")
				httpx.Error(w, c.status, "upgrade_required", "This endpoint is a WebSocket.", "")
				return
			}
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, g.cfg.SessionTimeout)
			defer cancel()
		} else {
			if !g.headers.Allow(userID.String()) {
				limited("user", time.Minute)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		}
		g.proxy.ServeHTTP(w, r.WithContext(ctx))
	}
}
