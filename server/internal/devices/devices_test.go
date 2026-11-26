package devices_test

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ahmadteeb/sideorbit/server/internal/apitest"
	"github.com/ahmadteeb/sideorbit/server/internal/devices"
	"github.com/ahmadteeb/sideorbit/server/internal/store/storetest"
)

type env struct {
	h            http.Handler
	pool         *pgxpool.Pool
	owner, rival uuid.UUID
}

func newEnv(t *testing.T) env {
	pool := storetest.New(t)
	h := apitest.Router(devices.New(pool, slog.New(slog.DiscardHandler), apitest.RequireUser, 2))
	return env{h, pool, apitest.NewUser(t, pool), apitest.NewUser(t, pool)}
}

type device struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	IOSVersion *string `json:"ios_version"`
	VPNOk      *bool   `json:"vpn_ok"`
}

func registration(client string) string {
	return fmt.Sprintf(`{"name":"My iPhone","model":"iPhone16,1","ios_version":"19.1","client_id":%q}`, client)
}

func (e env) register(t *testing.T, client string) device {
	t.Helper()
	rec := apitest.Do(t, e.h, e.owner, "POST", "/api/devices", registration(client))
	if rec.Code != http.StatusCreated {
		t.Fatalf("register: %d %s", rec.Code, rec.Body)
	}
	return apitest.JSON[device](t, rec)
}

func TestRegister(t *testing.T) {
	e := newEnv(t)
	first := e.register(t, "client-id-one")
	tests := []struct {
		name string
		user uuid.UUID
		body string
		want int
	}{
		{"signed out", uuid.Nil, registration("client-id-two"), 401},
		{"same device again", e.owner, registration("client-id-one"), 200},
		{"another account's device", e.rival, registration("client-id-one"), 409},
		{"no name", e.owner, `{"client_id":"client-id-two"}`, 400},
		{"no client id", e.owner, `{"name":"x"}`, 400},
		{"client id with spaces", e.owner, registration("not a client id"), 400},
		{"control character", e.owner, `{"name":"a\nb","client_id":"client-id-two"}`, 400},
		{"unknown field", e.owner, `{"name":"x","client_id":"client-id-two","udid":"x"}`, 400},
		{"second device", e.owner, registration("client-id-two"), 201},
		{"over the cap", e.owner, registration("client-id-three"), 403},
		{"existing device still allowed at the cap", e.owner, registration("client-id-two"), 200},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if rec := apitest.Do(t, e.h, tt.user, "POST", "/api/devices", tt.body); rec.Code != tt.want {
				t.Errorf("status = %d, want %d: %s", rec.Code, tt.want, rec.Body)
			}
		})
	}
	list := apitest.JSON[struct{ Devices []device }](t, apitest.Do(t, e.h, e.owner, "GET", "/api/devices", ""))
	if len(list.Devices) != 2 || list.Devices[0].ID != first.ID {
		t.Errorf("list = %+v", list)
	}
	if rec := apitest.Do(t, e.h, e.rival, "GET", "/api/devices", ""); rec.Body.String() != `{"devices":[]}`+"\n" {
		t.Errorf("rival sees %s", rec.Body)
	}
}

func TestRenameAndRemove(t *testing.T) {
	e := newEnv(t)
	id := e.register(t, "client-id-one").ID
	tests := []struct {
		name, method, id, body string
		user                   uuid.UUID
		want                   int
	}{
		{"rival cannot rename", "PATCH", id, `{"name":"mine now"}`, e.rival, 404},
		{"blank name", "PATCH", id, `{"name":"  "}`, e.owner, 400},
		{"rename", "PATCH", id, `{"name":"Work phone"}`, e.owner, 200},
		{"malformed id", "PATCH", "abc", `{"name":"x"}`, e.owner, 404},
		{"rival cannot delete", "DELETE", id, "", e.rival, 404},
		{"delete", "DELETE", id, "", e.owner, 204},
		{"already gone", "DELETE", id, "", e.owner, 404},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if rec := apitest.Do(t, e.h, tt.user, tt.method, "/api/devices/"+tt.id, tt.body); rec.Code != tt.want {
				t.Errorf("status = %d, want %d: %s", rec.Code, tt.want, rec.Body)
			}
		})
	}
}

func stamp(d time.Duration) string { return time.Now().Add(d).UTC().Format(time.RFC3339Nano) }

func fullReport(at string) string {
	return `{"reported_at":"` + at + `","vpn_ok":true,"pairing_ok":true,
	"apple_account":{"team_id":"ABCDE12345","label":"j***@icloud.com","app_ids_used_7d":4},
	"apps":[{"bundle_id":"com.example.app","name":"Example","version":"1.2.0","expires_at":"` + stamp(168*time.Hour) + `","source_url":"https://example.com/source.json"},
	        {"bundle_id":"dev.orbitstore.Orbit","name":"Orbit"}],
	"event":{"kind":"refresh","result":"success","trigger":"shortcut","error_code":null,"duration_ms":8123}}`
}

func TestReport(t *testing.T) {
	e := newEnv(t)
	target := "/api/devices/" + e.register(t, "client-id-one").ID + "/report"
	send := func(body string) int {
		t.Helper()
		return apitest.Do(t, e.h, e.owner, "POST", target, body).Code
	}
	apps := func() int { return apitest.Count(t, e.pool, "SELECT count(*) FROM device_apps") }
	events := func() int { return apitest.Count(t, e.pool, "SELECT count(*) FROM refresh_events") }

	at := stamp(-time.Minute)
	for range 2 { // a resend after a lost response changes nothing
		if code := send(fullReport(at)); code != http.StatusNoContent {
			t.Fatalf("report: %d", code)
		}
	}
	if apps() != 2 || events() != 1 {
		t.Fatalf("apps = %d, events = %d", apps(), events())
	}
	if n := apitest.Count(t, e.pool, "SELECT count(*) FROM devices WHERE vpn_ok AND pairing_ok AND apple_label = 'j***@icloud.com' AND app_ids_used_7d = 4"); n != 1 {
		t.Fatal("device status not stored")
	}

	// No "apps" key: inventory untouched.
	if send(`{"reported_at":"`+stamp(0)+`","vpn_ok":false}`) != 204 || apps() != 2 {
		t.Fatalf("partial report changed the inventory: %d", apps())
	}
	// A report queued before that one only adds history.
	old := `{"reported_at":"` + stamp(-time.Hour) + `","vpn_ok":true,"apps":[],"event":{"kind":"install","result":"failure","error_code":"VPN_OFF"}}`
	if send(old) != 204 || apps() != 2 || events() != 2 {
		t.Fatalf("stale report: apps = %d events = %d", apps(), events())
	}
	if apitest.Count(t, e.pool, "SELECT count(*) FROM devices WHERE NOT vpn_ok") != 1 {
		t.Fatal("stale report overwrote newer state")
	}
	// An app missing from a newer inventory was removed.
	if send(`{"reported_at":"`+stamp(time.Second)+`","apps":[{"bundle_id":"dev.orbitstore.Orbit","name":"Orbit"}]}`) != 204 || apps() != 1 {
		t.Fatalf("inventory not replaced: %d", apps())
	}
}

func TestReportRejects(t *testing.T) {
	e := newEnv(t)
	id := e.register(t, "client-id-one").ID
	now := stamp(0)
	with := func(extra string) string { return `{"reported_at":"` + now + `",` + extra + `}` }
	tooMany := make([]string, 101)
	for i := range tooMany {
		tooMany[i] = fmt.Sprintf(`{"bundle_id":"a.b%d","name":"x"}`, i)
	}
	tests := []struct{ name, body string }{
		// Anything the server must never receive is refused, not ignored.
		{"credential at top level", with(`"apple_password":"hunter2"`)},
		{"credential in account", with(`"apple_account":{"password":"hunter2"}`)},
		{"credential in app", with(`"apps":[{"bundle_id":"a.b","name":"x","p12":"AAAA"}]`)},
		{"credential in event", with(`"event":{"kind":"refresh","result":"success","session_token":"x"}`)},
		{"unmasked label", with(`"apple_account":{"label":"john.smith@icloud.com"}`)},
		{"barely masked label", with(`"apple_account":{"label":"john***smith@icloud.com"}`)},
		{"free-text error", with(`"event":{"kind":"refresh","result":"failure","error_code":"bad password for john"}`)},
		{"bad team id", with(`"apple_account":{"team_id":"abc"}`)},
		{"bad kind", with(`"event":{"kind":"steal","result":"success"}`)},
		{"bad result", with(`"event":{"kind":"refresh","result":"success|failure"}`)},
		{"bad trigger", with(`"event":{"kind":"refresh","result":"success","trigger":"x"}`)},
		{"negative duration", with(`"event":{"kind":"refresh","result":"success","duration_ms":-1}`)},
		{"repeated app", with(`"apps":[{"bundle_id":"a.b","name":"x"},{"bundle_id":"a.b","name":"y"}]`)},
		{"source with credentials", with(`"apps":[{"bundle_id":"a.b","name":"x","source_url":"https://u:p@example.com/s"}]`)},
		{"too many apps", with(`"apps":[` + strings.Join(tooMany, ",") + `]`)},
		{"no timestamp", `{"vpn_ok":true}`},
		{"future", `{"reported_at":"` + stamp(time.Hour) + `"}`},
		{"ancient", `{"reported_at":"2020-01-01T00:00:00Z"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := apitest.Do(t, e.h, e.owner, "POST", "/api/devices/"+id+"/report", tt.body)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d: %s", rec.Code, rec.Body)
			}
			if strings.Contains(rec.Body.String(), "hunter2") {
				t.Error("the response echoes the rejected value")
			}
		})
	}
	if n := apitest.Count(t, e.pool, "SELECT count(*) FROM refresh_events"); n != 0 {
		t.Errorf("rejected reports stored %d events", n)
	}
	if rec := apitest.Do(t, e.h, e.rival, "POST", "/api/devices/"+id+"/report", with(`"vpn_ok":true`)); rec.Code != 404 {
		t.Errorf("rival report: %d", rec.Code)
	}
}
