// Package push stores Web Push subscriptions and sends notifications.
package push

import (
	"context"
	"crypto/ecdh"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ahmadteeb/sideorbit/server/internal/auth"
	"github.com/ahmadteeb/sideorbit/server/internal/httpx"
	"github.com/ahmadteeb/sideorbit/server/internal/metrics"
	"github.com/ahmadteeb/sideorbit/server/internal/ratelimit"
	"github.com/ahmadteeb/sideorbit/server/internal/store/db"
)

const (
	maxSubscriptions = 10
	// A subscription that keeps failing is dead.
	maxFailures = 10
	// sendTimeout bounds one notification to all of an account's browsers, so a
	// slow endpoint can't hold up other people's alerts.
	sendTimeout = 20 * time.Second
)

// Keys is the server's VAPID identity.
type Keys struct {
	Public, Private string
	Subject         string // mailto: or https: contact for push services
}

// Message is what the service worker shows.
type Message struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"` // path to open on tap
}

// Service serves /api/push and sends notifications.
type Service struct {
	keys        Keys
	q           *db.Queries
	log         *slog.Logger
	requireUser func(http.Handler) http.Handler
	sessionOf   func(*http.Request) uuid.UUID
	client      *http.Client // must refuse non-public addresses: endpoints are user input
	retryDelays []time.Duration
	tests       *ratelimit.Limiter
}

// New builds the service. client must be a safehttp client. sessionOf names the
// caller's web session (the zero UUID if none), whose sign-out ends its alerts.
func New(keys Keys, pool *pgxpool.Pool, log *slog.Logger, requireUser func(http.Handler) http.Handler,
	sessionOf func(*http.Request) uuid.UUID, client *http.Client) *Service {
	return &Service{keys: keys, q: db.New(pool), log: log, requireUser: requireUser, sessionOf: sessionOf, client: client,
		retryDelays: []time.Duration{time.Second, 4 * time.Second},
		tests:       ratelimit.New(5, time.Minute)}
}

// Routes registers the endpoints on r.
func (s *Service) Routes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(s.requireUser)
		r.Get("/api/push/vapid-public-key", func(w http.ResponseWriter, _ *http.Request) {
			httpx.JSON(w, http.StatusOK, map[string]string{"public_key": s.keys.Public})
		})
		r.Post("/api/push/subscriptions", s.subscribe)
		r.Delete("/api/push/subscriptions/{id}", s.unsubscribe)
		r.Post("/api/push/test", s.test)
	})
}

func decodeKey(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
}

// subscribe takes the browser's PushSubscription JSON.
func (s *Service) subscribe(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Endpoint       string   `json:"endpoint"`
		ExpirationTime *float64 `json:"expirationTime"` // sent by browsers, unused
		Keys           struct {
			P256dh string `json:"p256dh"`
			Auth   string `json:"auth"`
		} `json:"keys"`
	}
	if !httpx.Decode(w, r, &req) {
		return
	}
	u, err := url.Parse(req.Endpoint)
	valid := err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && len(req.Endpoint) <= 2048
	if valid {
		var point, secret []byte
		if point, err = decodeKey(req.Keys.P256dh); err == nil {
			_, err = ecdh.P256().NewPublicKey(point)
		}
		if err == nil {
			secret, err = decodeKey(req.Keys.Auth)
		}
		valid = err == nil && len(secret) == 16
	}
	if !valid {
		httpx.Error(w, http.StatusBadRequest, "invalid_subscription", "That isn't a valid push subscription.", "Turn alerts off and on again.")
		return
	}
	agent := strings.ToValidUTF8(r.UserAgent(), "")
	if len(agent) > 200 {
		agent = strings.ToValidUTF8(agent[:200], "")
	}
	userID := auth.UserID(r.Context())
	// Alerts end with the session that turned them on, so there has to be a live
	// one: a revoked browser's access token must not be able to subscribe again.
	session := s.sessionOf(r)
	if session == uuid.Nil {
		httpx.Error(w, http.StatusForbidden, "session_required", "Alerts can only be turned on from a signed-in browser.", "Sign out, sign in again, then turn on alerts.")
		return
	}
	sub, err := s.q.SavePushSubscription(r.Context(), db.SavePushSubscriptionParams{
		ID: uuid.Must(uuid.NewV7()), UserID: userID, Endpoint: req.Endpoint,
		P256dh: req.Keys.P256dh, Auth: req.Keys.Auth, UserAgent: &agent, SessionID: &session,
	})
	if err == nil {
		err = s.q.TrimPushSubscriptions(r.Context(), db.TrimPushSubscriptionsParams{UserID: userID, Keep: maxSubscriptions})
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"id": sub.ID})
}

func (s *Service) unsubscribe(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.ID(w, r, "id")
	if !ok {
		return
	}
	n, err := s.q.DeletePushSubscription(r.Context(), db.DeletePushSubscriptionParams{ID: id, UserID: auth.UserID(r.Context())})
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if n == 0 {
		httpx.NotFound(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) test(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	if !s.tests.Allow(userID.String()) {
		s.log.Warn("rate limit hit", "endpoint", "push_test", "user_id", userID)
		w.Header().Set("Retry-After", "60")
		httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "Too many test alerts.", "Wait a minute.")
		return
	}
	sent, failed, err := s.Send(r.Context(), userID, Message{Title: "Orbit alerts are on", Body: "This is a test. Nothing to do.", URL: "/settings"})
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	if sent+failed == 0 {
		httpx.Error(w, http.StatusConflict, "no_subscriptions", "Alerts aren't turned on for any browser.", "Turn on alerts first.")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]int{"sent": sent, "failed": failed})
}

// Send delivers msg to every subscription of the account. Subscriptions the push
// service reports as gone are removed.
func (s *Service) Send(ctx context.Context, userID uuid.UUID, msg Message) (sent, failed int, err error) {
	subs, err := s.q.ListPushSubscriptions(ctx, userID)
	if err != nil {
		return 0, 0, fmt.Errorf("list subscriptions: %w", err)
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return 0, 0, err
	}
	// Deliveries run together under one deadline; bookkeeping uses the caller's
	// context so it still happens when that deadline passes.
	type result struct {
		status int
		err    error
	}
	results := make([]result, len(subs))
	sendCtx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	var wg sync.WaitGroup
	for i, sub := range subs {
		wg.Go(func() {
			status, err := s.deliver(sendCtx, sub, payload)
			results[i] = result{status, err}
		})
	}
	wg.Wait()
	for i, sub := range subs {
		status, sendErr := results[i].status, results[i].err
		switch {
		case sendErr == nil && status >= 200 && status < 300:
			sent++
			metrics.Push.WithLabelValues("sent").Inc()
			err = s.q.ResetPushFailures(ctx, sub.ID)
		case status == http.StatusNotFound || status == http.StatusGone:
			failed++
			metrics.Push.WithLabelValues("gone").Inc()
			err = s.q.DeletePushSubscriptionByID(ctx, sub.ID)
		default:
			failed++
			metrics.Push.WithLabelValues("failed").Inc()
			// The endpoint is a secret URL and transport errors quote it.
			var urlErr *url.Error
			if errors.As(sendErr, &urlErr) {
				sendErr = urlErr.Err
			}
			s.log.Warn("push failed", "user_id", userID, "subscription_id", sub.ID, "status", status, "err", sendErr)
			var n int16
			if n, err = s.q.CountPushFailure(ctx, sub.ID); err == nil && n >= maxFailures {
				err = s.q.DeletePushSubscriptionByID(ctx, sub.ID)
			}
		}
		if err != nil {
			return sent, failed, fmt.Errorf("update subscription: %w", err)
		}
	}
	return sent, failed, nil
}

// deliver posts one notification, retrying network errors, 429 and 5xx with
// backoff.
func (s *Service) deliver(ctx context.Context, sub db.PushSubscription, payload []byte) (status int, err error) {
	for attempt := 0; ; attempt++ {
		status, wait, err := s.post(ctx, sub, payload)
		retryable := err != nil || status == http.StatusTooManyRequests || status >= 500
		if !retryable || attempt >= len(s.retryDelays) {
			return status, err
		}
		// Honour a short Retry-After; a long one waits for the next failure.
		delay := max(s.retryDelays[attempt], min(wait, 10*time.Second))
		select {
		case <-ctx.Done():
			return status, ctx.Err()
		case <-time.After(delay):
		}
	}
}

func (s *Service) post(ctx context.Context, sub db.PushSubscription, payload []byte) (int, time.Duration, error) {
	res, err := webpush.SendNotificationWithContext(ctx, payload,
		&webpush.Subscription{Endpoint: sub.Endpoint, Keys: webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth}},
		&webpush.Options{
			HTTPClient:      s.client,
			Subscriber:      strings.TrimPrefix(s.keys.Subject, "mailto:"), // the library adds it
			VAPIDPublicKey:  s.keys.Public,
			VAPIDPrivateKey: s.keys.Private,
			TTL:             24 * 60 * 60,
			Urgency:         webpush.UrgencyHigh,
		})
	if err != nil {
		return 0, 0, err
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4<<10))
	seconds, _ := strconv.Atoi(res.Header.Get("Retry-After"))
	return res.StatusCode, time.Duration(seconds) * time.Second, nil
}
