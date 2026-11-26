// Package scheduler warns users about expiring apps and silent devices, and
// deletes data past its retention.
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ahmadteeb/sideorbit/server/internal/push"
	"github.com/ahmadteeb/sideorbit/server/internal/store/db"
)

const interval = 15 * time.Minute

// Sender delivers a notification to an account.
type Sender interface {
	Send(ctx context.Context, userID uuid.UUID, msg push.Message) (sent, failed int, err error)
}

// Scheduler runs the periodic jobs.
type Scheduler struct {
	q              *db.Queries
	sender         Sender // nil when web push isn't configured
	log            *slog.Logger
	eventRetention time.Duration
	now            func() time.Time
}

// New builds a scheduler.
func New(pool *pgxpool.Pool, sender Sender, log *slog.Logger, eventRetention time.Duration) *Scheduler {
	return &Scheduler{q: db.New(pool), sender: sender, log: log, eventRetention: eventRetention, now: time.Now}
}

// Run executes the jobs now and every 15 minutes until ctx ends.
func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := s.Tick(ctx); err != nil && ctx.Err() == nil {
			s.log.Error("scheduled jobs failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Tick runs every job once. One failing job doesn't stop the others.
func (s *Scheduler) Tick(ctx context.Context) error {
	errs := []error{s.prune(ctx)}
	if s.sender != nil {
		errs = append(errs, s.expiryAlerts(ctx), s.silentAlerts(ctx))
	}
	return errors.Join(errs...)
}

func (s *Scheduler) prune(ctx context.Context) error {
	now := s.now()
	var errs []error
	for name, job := range map[string]func() (int64, error){
		"login codes":    func() (int64, error) { return s.q.PruneLoginCodes(ctx, now) },
		"refresh tokens": func() (int64, error) { return s.q.PruneRefreshTokens(ctx, now) },
		"events":         func() (int64, error) { return s.q.PruneEvents(ctx, now.Add(-s.eventRetention)) },
		"alert records":  func() (int64, error) { return s.q.PruneAlerts(ctx, now.Add(-s.eventRetention)) },
	} {
		if n, err := job(); err != nil {
			errs = append(errs, fmt.Errorf("prune %s: %w", name, err))
		} else if n > 0 {
			s.log.Debug("pruned", "what", name, "rows", n)
		}
	}
	return errors.Join(errs...)
}

// alert is one notification and the alerts_sent rows that stop it repeating.
type alert struct {
	userID uuid.UUID
	claims []db.ClaimAlertParams
	msg    func(new int) push.Message
}

// send claims the rows first, so that replicas running together send once. If
// nothing could be delivered the claim is released for the next run.
func (s *Scheduler) send(ctx context.Context, a alert) error {
	var claimed []db.ClaimAlertParams
	for _, c := range a.claims {
		n, err := s.q.ClaimAlert(ctx, c)
		if err != nil {
			return fmt.Errorf("claim alert: %w", err)
		}
		if n == 1 {
			claimed = append(claimed, c)
		}
	}
	if len(claimed) == 0 {
		return nil
	}
	sent, _, err := s.sender.Send(ctx, a.userID, a.msg(len(claimed)))
	if sent > 0 {
		return err
	}
	for _, c := range claimed {
		if rerr := s.q.ReleaseAlert(ctx, db.ReleaseAlertParams(c)); rerr != nil {
			err = errors.Join(err, rerr)
		}
	}
	return err
}

// expiryAlerts sends one notification per device and threshold (24h, 6h).
func (s *Scheduler) expiryAlerts(ctx context.Context) error {
	now := s.now()
	apps, err := s.q.ExpiringApps(ctx, now)
	if err != nil {
		return fmt.Errorf("expiring apps: %w", err)
	}
	type key struct {
		device uuid.UUID
		hours  int
	}
	groups := map[key]*alert{}
	for _, app := range apps {
		hours := 24
		if app.ExpiresAt.Sub(now) <= 6*time.Hour {
			hours = 6
		}
		k := key{app.DeviceID, hours}
		g := groups[k]
		if g == nil {
			device, name := app.DeviceID, app.DeviceName
			g = &alert{userID: app.UserID, msg: func(n int) push.Message {
				what := "1 app"
				verb := "expires"
				if n > 1 {
					what, verb = fmt.Sprintf("%d apps", n), "expire"
				}
				return push.Message{
					Title: fmt.Sprintf("%s %s within %d hours", what, verb, hours),
					Body:  "Open Orbit on " + name + " and tap Refresh.",
					URL:   "/devices/" + device.String(),
				}
			}}
			groups[k] = g
		}
		// The window is this signing: a refresh moves the expiry and so re-arms.
		window := app.ExpiresAt.Add(-7 * 24 * time.Hour)
		g.claims = append(g.claims, db.ClaimAlertParams{DeviceID: app.DeviceID, BundleID: app.BundleID,
			Kind: fmt.Sprintf("expiry_%dh", hours), WindowStart: window})
	}
	var errs []error
	for _, g := range groups {
		errs = append(errs, s.send(ctx, *g))
	}
	return errors.Join(errs...)
}

func (s *Scheduler) silentAlerts(ctx context.Context) error {
	devices, err := s.q.SilentDevices(ctx, s.now())
	if err != nil {
		return fmt.Errorf("silent devices: %w", err)
	}
	var errs []error
	for _, d := range devices {
		errs = append(errs, s.send(ctx, alert{
			userID: d.UserID,
			claims: []db.ClaimAlertParams{{DeviceID: d.DeviceID, Kind: "device_silent", WindowStart: d.LastSeenAt}},
			msg: func(int) push.Message {
				return push.Message{
					Title: d.DeviceName + " hasn't checked in",
					Body:  "No report for 2 days. Open Orbit on it so your apps don't expire.",
					URL:   "/devices/" + d.DeviceID.String(),
				}
			},
		}))
	}
	return errors.Join(errs...)
}
