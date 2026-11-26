package push

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ahmadteeb/sideorbit/server/internal/apitest"
	"github.com/ahmadteeb/sideorbit/server/internal/store/storetest"
)

// fakePush is a push service. Each endpoint path answers with its queued
// statuses, then 201.
type fakePush struct {
	*httptest.Server
	mu      sync.Mutex
	answers map[string][]int
	got     []*http.Request
}

func newFakePush(t *testing.T) *fakePush {
	f := &fakePush{answers: map[string][]int{}}
	f.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		defer f.mu.Unlock()
		if len(body) == 0 {
			t.Error("empty notification")
		}
		f.got = append(f.got, r.Clone(context.Background()))
		status := http.StatusCreated
		if q := f.answers[r.URL.Path]; len(q) > 0 {
			status, f.answers[r.URL.Path] = q[0], q[1:]
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(f.Close)
	return f
}

type env struct {
	h    http.Handler
	svc  *Service
	pool *pgxpool.Pool
	fake *fakePush
	user uuid.UUID
}

func newEnv(t *testing.T) env {
	t.Helper()
	pool := storetest.New(t)
	private, public, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	fake := newFakePush(t)
	svc := New(Keys{Public: public, Private: private, Subject: "mailto:ops@example.com"}, pool,
		slog.New(slog.DiscardHandler), apitest.RequireUser, fake.Client())
	svc.retryDelays = []time.Duration{0, 0}
	return env{apitest.Router(svc), svc, pool, fake, apitest.NewUser(t, pool)}
}

func subscription(t *testing.T, endpoint string) string {
	t.Helper()
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	secret := make([]byte, 16)
	_, _ = rand.Read(secret)
	enc := base64.RawURLEncoding.EncodeToString
	return `{"endpoint":"` + endpoint + `","expirationTime":null,"keys":{"p256dh":"` + enc(key.PublicKey().Bytes()) + `","auth":"` + enc(secret) + `"}}`
}

func (e env) subscribe(t *testing.T, user uuid.UUID, path string) string {
	t.Helper()
	rec := apitest.Do(t, e.h, user, "POST", "/api/push/subscriptions", subscription(t, e.fake.URL+path))
	if rec.Code != http.StatusCreated {
		t.Fatalf("subscribe: %d %s", rec.Code, rec.Body)
	}
	return apitest.JSON[struct{ ID string }](t, rec).ID
}

func (e env) count(t *testing.T, where string) int {
	return apitest.Count(t, e.pool, "SELECT count(*) FROM push_subscriptions "+where)
}

func TestSubscribeRejects(t *testing.T) {
	e := newEnv(t)
	good := subscription(t, "https://push.example.com/x")
	tests := []struct {
		name string
		user uuid.UUID
		body string
		want int
	}{
		{"signed out", uuid.Nil, good, 401},
		{"http endpoint", e.user, subscription(t, "http://push.example.com/x"), 400},
		{"no keys", e.user, `{"endpoint":"https://push.example.com/x"}`, 400},
		{"short auth", e.user, strings.Replace(good, `"auth":"`, `"auth":"A`, 1)[:len(good)-10] + `"}}`, 400},
		{"point not on curve", e.user, `{"endpoint":"https://push.example.com/x","keys":{"p256dh":"BAAA","auth":"AAAAAAAAAAAAAAAAAAAAAA"}}`, 400},
		{"unknown field", e.user, `{"endpoint":"https://push.example.com/x","extra":1}`, 400},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if rec := apitest.Do(t, e.h, tt.user, "POST", "/api/push/subscriptions", tt.body); rec.Code != tt.want {
				t.Errorf("status = %d: %s", rec.Code, rec.Body)
			}
		})
	}
}

func TestSubscriptionLifecycle(t *testing.T) {
	e := newEnv(t)
	other := apitest.NewUser(t, e.pool)
	id := e.subscribe(t, e.user, "/a")
	if again := e.subscribe(t, e.user, "/a"); again != id || e.count(t, "") != 1 {
		t.Errorf("re-subscribing made a second record")
	}
	if rec := apitest.Do(t, e.h, other, "DELETE", "/api/push/subscriptions/"+id, ""); rec.Code != 404 {
		t.Errorf("rival delete: %d", rec.Code)
	}
	if rec := apitest.Do(t, e.h, e.user, "DELETE", "/api/push/subscriptions/"+id, ""); rec.Code != 204 {
		t.Errorf("delete: %d", rec.Code)
	}
	for i := range maxSubscriptions + 3 {
		e.subscribe(t, e.user, "/many/"+string(rune('a'+i)))
	}
	if n := e.count(t, ""); n != maxSubscriptions {
		t.Errorf("%d subscriptions kept", n)
	}
	rec := apitest.Do(t, e.h, e.user, "GET", "/api/push/vapid-public-key", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), e.svc.keys.Public) || strings.Contains(rec.Body.String(), e.svc.keys.Private) {
		t.Errorf("public key: %d %s", rec.Code, rec.Body)
	}
}

func TestSend(t *testing.T) {
	tests := []struct {
		name               string
		answers            []int
		wantSent, requests int
		wantRows           string // WHERE clause that must match one row, or "" for none left
	}{
		{"delivered", nil, 1, 1, "WHERE failures = 0"},
		{"gone is removed", []int{410}, 0, 1, ""},
		{"not found is removed", []int{404}, 0, 1, ""},
		{"retried then delivered", []int{500, 429}, 1, 3, "WHERE failures = 0"},
		{"keeps failing", []int{503, 503, 503}, 0, 3, "WHERE failures = 1"},
		{"permanent error isn't retried", []int{400}, 0, 1, "WHERE failures = 1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			e.subscribe(t, e.user, "/s")
			e.fake.answers["/s"] = tt.answers
			sent, failed, err := e.svc.Send(context.Background(), e.user, Message{Title: "t"})
			if err != nil || sent != tt.wantSent || sent+failed != 1 || len(e.fake.got) != tt.requests {
				t.Fatalf("sent=%d failed=%d err=%v requests=%d", sent, failed, err, len(e.fake.got))
			}
			if tt.wantRows == "" && e.count(t, "") != 0 || tt.wantRows != "" && e.count(t, tt.wantRows) != 1 {
				t.Errorf("subscription state wrong (%s)", tt.wantRows)
			}
			h := e.fake.got[0].Header
			if !strings.HasPrefix(h.Get("Authorization"), "vapid t=") || h.Get("Content-Encoding") != "aes128gcm" || h.Get("TTL") != "86400" {
				t.Errorf("headers = %v", h)
			}
		})
	}
}

func TestPersistentFailureRemovesSubscription(t *testing.T) {
	e := newEnv(t)
	e.subscribe(t, e.user, "/s")
	for range maxFailures {
		e.fake.answers["/s"] = []int{400}
		if _, _, err := e.svc.Send(context.Background(), e.user, Message{}); err != nil {
			t.Fatal(err)
		}
	}
	if n := e.count(t, ""); n != 0 {
		t.Errorf("%d subscriptions left", n)
	}
}

func TestTestEndpoint(t *testing.T) {
	e := newEnv(t)
	post := func() (int, string) {
		rec := apitest.Do(t, e.h, e.user, "POST", "/api/push/test", "")
		return rec.Code, rec.Body.String()
	}
	if code, body := post(); code != 409 {
		t.Errorf("without subscriptions: %d %s", code, body)
	}
	e.subscribe(t, e.user, "/s")
	if code, body := post(); code != 200 || body != `{"failed":0,"sent":1}`+"\n" {
		t.Errorf("test: %d %s", code, body)
	}
	post()
	post()
	post()
	if code, _ := post(); code != 429 {
		t.Errorf("sixth call: %d", code)
	}
}
