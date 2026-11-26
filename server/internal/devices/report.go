package devices

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ahmadteeb/sideorbit/server/internal/auth"
	"github.com/ahmadteeb/sideorbit/server/internal/httpx"
	"github.com/ahmadteeb/sideorbit/server/internal/store/db"
)

// These types are the complete list of data a device may send. Decoding rejects
// anything else, and every string is bounded, so credentials have nowhere to go
// (AGENTS.md hard rule 1). Adding a field needs a DECISIONS.md entry.
type reportBody struct {
	ReportedAt   time.Time     `json:"reported_at"`
	VPNOk        *bool         `json:"vpn_ok"`
	PairingOk    *bool         `json:"pairing_ok"`
	AppleAccount *appleAccount `json:"apple_account"`
	Apps         *[]reportApp  `json:"apps"` // absent: unchanged; empty: no apps
	Event        *reportEvent  `json:"event"`
}

type appleAccount struct {
	TeamID       string `json:"team_id"`
	Label        string `json:"label"` // masked on the device
	AppIDsUsed7d *int16 `json:"app_ids_used_7d"`
}

type reportApp struct {
	BundleID  string     `json:"bundle_id"`
	Name      string     `json:"name"`
	Version   string     `json:"version"`
	ExpiresAt *time.Time `json:"expires_at"`
	SourceURL string     `json:"source_url"`
}

type reportEvent struct {
	Kind       string  `json:"kind"`
	Result     string  `json:"result"`
	Trigger    *string `json:"trigger"`
	ErrorCode  *string `json:"error_code"`
	DurationMs *int32  `json:"duration_ms"`
}

const (
	maxApps = 100
	// Reports queued offline may be old, but not older than events are kept.
	maxReportAge  = 90 * 24 * time.Hour
	maxClockAhead = 10 * time.Minute
)

var (
	teamIDPattern    = regexp.MustCompile(`^[A-Z0-9]{10}$`)
	bundleIDPattern  = regexp.MustCompile(`^[A-Za-z0-9._-]{1,155}$`)
	errorCodePattern = regexp.MustCompile(`^[A-Z0-9_]{1,64}$`)
)

// masked requires an Apple ID label with most of the identifier hidden, so an
// app bug can't send the full address.
func masked(label string) bool {
	local, _, _ := strings.Cut(label, "@")
	visible := len(local) - strings.Count(local, "*")
	return text(label, 64) && strings.Contains(local, "***") && visible <= 4
}

func (b *reportBody) validate(now time.Time) error {
	if b.ReportedAt.IsZero() || b.ReportedAt.Before(now.Add(-maxReportAge)) || b.ReportedAt.After(now.Add(maxClockAhead)) {
		return errors.New("reported_at is missing or out of range")
	}
	if a := b.AppleAccount; a != nil {
		if a.TeamID != "" && !teamIDPattern.MatchString(a.TeamID) {
			return errors.New("team_id must be 10 characters, A-Z and 0-9")
		}
		if a.Label != "" && !masked(a.Label) {
			return errors.New("label must be masked, like j***@icloud.com")
		}
		if a.AppIDsUsed7d != nil && (*a.AppIDsUsed7d < 0 || *a.AppIDsUsed7d > 1000) {
			return errors.New("app_ids_used_7d is out of range")
		}
	}
	if b.Apps != nil {
		if len(*b.Apps) > maxApps {
			return fmt.Errorf("more than %d apps", maxApps)
		}
		seen := map[string]bool{}
		for i, a := range *b.Apps {
			if !bundleIDPattern.MatchString(a.BundleID) || seen[a.BundleID] {
				return fmt.Errorf("apps[%d]: bundle_id is invalid or repeated", i)
			}
			seen[a.BundleID] = true
			if a.Name == "" || !text(a.Name, 200) || !text(a.Version, 64) {
				return fmt.Errorf("apps[%d]: name or version is invalid", i)
			}
			if a.SourceURL != "" {
				u, err := url.Parse(a.SourceURL)
				if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || !text(a.SourceURL, 2048) {
					return fmt.Errorf("apps[%d]: source_url must be an https URL", i)
				}
			}
		}
	}
	if e := b.Event; e != nil {
		if !slices.Contains([]string{"refresh", "install", "remove", "repair"}, e.Kind) {
			return errors.New("event.kind is invalid")
		}
		if e.Result != "success" && e.Result != "failure" {
			return errors.New("event.result is invalid")
		}
		if e.Trigger != nil && !slices.Contains([]string{"background", "shortcut", "manual", "launch", "setup"}, *e.Trigger) {
			return errors.New("event.trigger is invalid")
		}
		// A code, never free text: error messages can quote account details.
		if e.ErrorCode != nil && !errorCodePattern.MatchString(*e.ErrorCode) {
			return errors.New("event.error_code must be an UPPER_SNAKE_CASE code")
		}
		if e.DurationMs != nil && (*e.DurationMs < 0 || *e.DurationMs > 86_400_000) {
			return errors.New("event.duration_ms is out of range")
		}
	}
	return nil
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (s *Service) report(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.ID(w, r, "id")
	if !ok {
		return
	}
	userID := auth.UserID(r.Context())
	if !s.reports.Allow(userID.String()) {
		s.log.Warn("rate limit hit", "endpoint", "report", "user_id", userID)
		w.Header().Set("Retry-After", "60")
		httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "Too many reports.", "Wait a minute.")
		return
	}
	var b reportBody
	if !httpx.Decode(w, r, &b) {
		return
	}
	if err := b.validate(time.Now()); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_report", err.Error(), "")
		return
	}
	at := b.ReportedAt.UTC().Truncate(time.Microsecond) // Postgres precision

	err := pgx.BeginFunc(r.Context(), s.pool, func(tx pgx.Tx) error {
		q, ctx := s.q.WithTx(tx), r.Context()
		device, err := q.LockDevice(ctx, db.LockDeviceParams{ID: id, UserID: userID})
		if err != nil {
			return err
		}
		if err := q.MarkDeviceSeen(ctx, id); err != nil {
			return err
		}
		if e := b.Event; e != nil {
			if err := q.InsertEvent(ctx, db.InsertEventParams{
				ID: uuid.Must(uuid.NewV7()), DeviceID: id, Kind: e.Kind, Trigger: e.Trigger, Result: e.Result,
				ErrorCode: e.ErrorCode, DurationMs: e.DurationMs, OccurredAt: at,
			}); err != nil {
				return err
			}
		}
		// An older report only adds history; it must not undo newer state.
		if device.LastReportAt != nil && at.Before(*device.LastReportAt) {
			return nil
		}
		status := db.ApplyDeviceStatusParams{ID: id, ReportedAt: &at, VpnOk: b.VPNOk, PairingOk: b.PairingOk}
		if a := b.AppleAccount; a != nil {
			status.AppleTeamID, status.AppleLabel, status.AppIdsUsed7d = optional(a.TeamID), optional(a.Label), a.AppIDsUsed7d
		}
		if err := q.ApplyDeviceStatus(ctx, status); err != nil {
			return err
		}
		if b.Apps == nil {
			return nil
		}
		keep := make([]string, 0, len(*b.Apps))
		for _, a := range *b.Apps {
			keep = append(keep, a.BundleID)
			if err := q.UpsertDeviceApp(ctx, db.UpsertDeviceAppParams{
				DeviceID: id, BundleID: a.BundleID, Name: a.Name, Version: optional(a.Version),
				ExpiresAt: a.ExpiresAt, SourceUrl: optional(a.SourceURL),
			}); err != nil {
				return err
			}
		}
		return q.DeleteDeviceAppsExcept(ctx, db.DeleteDeviceAppsExceptParams{DeviceID: id, Keep: keep})
	})
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.NotFound(w)
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
