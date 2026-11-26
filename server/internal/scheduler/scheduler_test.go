package scheduler

import (
	"context"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ahmadteeb/sideorbit/server/internal/apitest"
	"github.com/ahmadteeb/sideorbit/server/internal/push"
	"github.com/ahmadteeb/sideorbit/server/internal/store/storetest"
)

type fakeSender struct {
	titles []string
	fail   bool
}

func (f *fakeSender) Send(_ context.Context, _ uuid.UUID, m push.Message) (int, int, error) {
	if f.fail {
		return 0, 1, nil
	}
	f.titles = append(f.titles, m.Title)
	return 1, 0, nil
}

// take returns what was sent since the last call.
func (f *fakeSender) take() []string {
	t := f.titles
	f.titles = nil
	slices.Sort(t)
	return t
}

type env struct {
	t      *testing.T
	pool   *pgxpool.Pool
	s      *Scheduler
	sender *fakeSender
	user   uuid.UUID
	device uuid.UUID
	start  time.Time
	now    time.Time
}

func newEnv(t *testing.T, subscribed bool) *env {
	pool := storetest.New(t)
	e := &env{t: t, pool: pool, sender: &fakeSender{}, user: apitest.NewUser(t, pool), device: uuid.New(),
		start: time.Now().UTC().Truncate(time.Second)}
	e.now = e.start
	e.s = New(pool, e.sender, slog.New(slog.DiscardHandler), 90*24*time.Hour)
	e.s.now = func() time.Time { return e.now }
	e.exec("INSERT INTO devices (id, user_id, name, last_seen_at) VALUES ($1, $2, 'My iPhone', $3)", e.device, e.user, e.start)
	if subscribed {
		e.subscribe()
	}
	return e
}

func (e *env) subscribe() {
	e.exec("INSERT INTO push_subscriptions (id, user_id, endpoint, p256dh, auth) VALUES (gen_random_uuid(), $1, gen_random_uuid()::text, 'k', 'a')", e.user)
}

func (e *env) exec(sql string, args ...any) {
	e.t.Helper()
	if _, err := e.pool.Exec(context.Background(), sql, args...); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) app(bundle string, expiresIn time.Duration) {
	e.exec(`INSERT INTO device_apps (device_id, bundle_id, name, expires_at) VALUES ($1, $2, $2, $3)
		ON CONFLICT (device_id, bundle_id) DO UPDATE SET expires_at = EXCLUDED.expires_at`, e.device, bundle, e.now.Add(expiresIn))
}

// tick advances the clock to start+at and runs the jobs. The device keeps
// checking in unless silent.
func (e *env) tick(at time.Duration, silent ...bool) []string {
	e.t.Helper()
	e.now = e.start.Add(at)
	if len(silent) == 0 {
		e.exec("UPDATE devices SET last_seen_at = $1", e.now)
	}
	if err := e.s.Tick(context.Background()); err != nil {
		e.t.Fatal(err)
	}
	return e.sender.take()
}

func TestExpiryAlerts(t *testing.T) {
	e := newEnv(t, true)
	e.app("a.one", 30*time.Hour)
	e.app("a.two", 30*time.Hour)
	steps := []struct {
		name string
		at   time.Duration
		want []string
	}{
		{"too early", 0, nil},
		{"24 hours", 7 * time.Hour, []string{"2 apps expire within 24 hours"}},
		{"not repeated", 8 * time.Hour, nil},
		{"6 hours", 25 * time.Hour, []string{"2 apps expire within 6 hours"}},
		{"not repeated again", 26 * time.Hour, nil},
		{"expired: nothing more", 31 * time.Hour, nil},
	}
	for _, s := range steps {
		if got := e.tick(s.at); !slices.Equal(got, s.want) {
			t.Fatalf("%s: %v, want %v", s.name, got, s.want)
		}
	}
	// A refresh moves the expiry, which re-arms the alerts.
	e.app("a.one", 20*time.Hour)
	if got := e.tick(31 * time.Hour); !slices.Equal(got, []string{"1 app expires within 24 hours"}) {
		t.Errorf("after refresh: %v", got)
	}
}

func TestFirstSeenInsideSixHours(t *testing.T) {
	e := newEnv(t, true)
	e.app("a.one", 5*time.Hour)
	if got := e.tick(0); !slices.Equal(got, []string{"1 app expires within 6 hours"}) {
		t.Errorf("%v", got)
	}
	if got := e.tick(time.Minute); got != nil {
		t.Errorf("a 24-hour alert followed: %v", got)
	}
}

func TestSilentDevice(t *testing.T) {
	e := newEnv(t, true)
	if got := e.tick(72*time.Hour, true); got != nil {
		t.Errorf("a device with no apps: %v", got)
	}
	e.app("a.one", 240*time.Hour)
	e.exec("UPDATE devices SET last_seen_at = $1", e.start)
	steps := []struct {
		name string
		at   time.Duration
		want int
	}{{"47 hours", 47 * time.Hour, 0}, {"49 hours", 49 * time.Hour, 1}, {"not repeated", 60 * time.Hour, 0}}
	for _, s := range steps {
		e.now = e.start.Add(s.at)
		got := e.tick(s.at, true)
		if len(got) != s.want || (s.want == 1 && !strings.Contains(got[0], "My iPhone")) {
			t.Fatalf("%s: %v", s.name, got)
		}
	}
	e.tick(61 * time.Hour) // checks in again
	if got := e.tick(110*time.Hour, true); len(got) != 1 {
		t.Errorf("silent again: %v", got)
	}
}

func TestAlertWaitsForDelivery(t *testing.T) {
	e := newEnv(t, false)
	e.app("a.one", 20*time.Hour)
	if got := e.tick(0); got != nil {
		t.Fatalf("no subscription: %v", got)
	}
	e.subscribe()
	e.sender.fail = true
	e.tick(time.Minute)
	e.sender.fail = false
	if got := e.tick(2 * time.Minute); len(got) != 1 {
		t.Errorf("after a failed delivery: %v", got)
	}
}

func TestDeletedAccountGetsNothing(t *testing.T) {
	e := newEnv(t, true)
	e.app("a.one", time.Hour)
	e.exec("UPDATE users SET deleted_at = now()")
	if got := e.tick(0); got != nil {
		t.Errorf("%v", got)
	}
}

func TestPrune(t *testing.T) {
	e := newEnv(t, false)
	day := 24 * time.Hour
	code := "INSERT INTO login_codes (code_hash, purpose, expires_at) VALUES ($1, 'magic_link', $2)"
	e.exec(code, []byte("old"), e.start.Add(-25*time.Hour))
	e.exec(code, []byte("recent"), e.start.Add(-23*time.Hour))
	token := "INSERT INTO refresh_tokens (id, user_id, family_id, token_hash, client, expires_at) VALUES (gen_random_uuid(), $1, gen_random_uuid(), $2, 'ios', $3)"
	e.exec(token, e.user, []byte("old"), e.start.Add(-2*day))
	e.exec(token, e.user, []byte("live"), e.start.Add(day))
	event := "INSERT INTO refresh_events (id, device_id, kind, result, occurred_at) VALUES (gen_random_uuid(), $1, 'refresh', 'success', $2)"
	e.exec(event, e.device, e.start.Add(-91*day))
	e.exec(event, e.device, e.start.Add(-89*day))
	e.tick(0)
	for _, table := range []string{"login_codes", "refresh_tokens", "refresh_events"} {
		if n := apitest.Count(t, e.pool, "SELECT count(*) FROM "+table); n != 1 {
			t.Errorf("%s: %d rows, want 1", table, n)
		}
	}
}
