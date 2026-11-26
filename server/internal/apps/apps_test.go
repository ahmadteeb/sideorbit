package apps_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ahmadteeb/sideorbit/server/internal/apitest"
	"github.com/ahmadteeb/sideorbit/server/internal/apps"
	"github.com/ahmadteeb/sideorbit/server/internal/safehttp"
	"github.com/ahmadteeb/sideorbit/server/internal/store/storetest"
)

func sourceServer(t *testing.T) *httptest.Server {
	bodies := map[string]string{
		"/good.json": `{"name":"Test Source","identifier":"x","apps":[]}`,
		"/noapps":    `{"name":"x"}`,
		"/noname":    `{"apps":[]}`,
		"/html":      `<html></html>`,
		"/huge":      `{"name":"x","apps":[],"pad":"` + strings.Repeat("a", 2<<20) + `"}`,
	}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/to-http" {
			http.Redirect(w, r, "http://example.com/s.json", http.StatusFound)
			return
		}
		body, ok := bodies[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

type source struct {
	ID   string `json:"id"`
	URL  string `json:"url"`
	Name string `json:"name"`
}

func TestSources(t *testing.T) {
	pool := storetest.New(t)
	srv := sourceServer(t)
	// The test server is on loopback, so use its client with the real redirect rule.
	fetch := srv.Client()
	fetch.CheckRedirect = safehttp.RequireHTTPS
	h := apitest.Router(apps.New(pool, slog.New(slog.DiscardHandler), apitest.RequireUser, fetch))
	owner, rival := apitest.NewUser(t, pool), apitest.NewUser(t, pool)
	add := func(user uuid.UUID, url string) *httptest.ResponseRecorder {
		return apitest.Do(t, h, user, "POST", "/api/sources", `{"url":"`+url+`"}`)
	}

	rec := add(owner, srv.URL+"/good.json#frag")
	if rec.Code != http.StatusCreated {
		t.Fatalf("add: %d %s", rec.Code, rec.Body)
	}
	saved := apitest.JSON[source](t, rec)
	if saved.Name != "Test Source" || saved.URL != srv.URL+"/good.json" {
		t.Fatalf("saved = %+v", saved)
	}

	tests := []struct {
		name, url string
		want      int
	}{
		{"again", srv.URL + "/good.json", 200},
		{"plain http", "http://example.com/s.json", 400},
		{"credentials in url", "https://u:p@example.com/s.json", 400},
		{"not a url", "nonsense", 400},
		{"no apps", srv.URL + "/noapps", 422},
		{"no name", srv.URL + "/noname", 422},
		{"not json", srv.URL + "/html", 422},
		{"too big", srv.URL + "/huge", 422},
		{"missing", srv.URL + "/missing", 422},
		{"redirect to http", srv.URL + "/to-http", 422},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if rec := add(owner, tt.url); rec.Code != tt.want {
				t.Errorf("status = %d, want %d: %s", rec.Code, tt.want, rec.Body)
			}
		})
	}
	// Eight requests have fetched so far; the limit is ten a minute.
	for i, want := range []int{200, 200, 429} {
		if rec := add(owner, srv.URL+"/good.json"); rec.Code != want {
			t.Errorf("fetch %d: %d, want %d", 9+i, rec.Code, want)
		}
	}

	list := func(user uuid.UUID) []source {
		return apitest.JSON[struct{ Sources []source }](t, apitest.Do(t, h, user, "GET", "/api/sources", "")).Sources
	}
	if len(list(owner)) != 1 || len(list(rival)) != 0 {
		t.Errorf("owner %d, rival %d", len(list(owner)), len(list(rival)))
	}
	if rec := apitest.Do(t, h, rival, "DELETE", "/api/sources/"+saved.ID, ""); rec.Code != 404 {
		t.Errorf("rival delete: %d", rec.Code)
	}
	if rec := apitest.Do(t, h, owner, "DELETE", "/api/sources/"+saved.ID, ""); rec.Code != 204 {
		t.Errorf("delete: %d", rec.Code)
	}
}

func TestSourcesBlockPrivateAddresses(t *testing.T) {
	pool := storetest.New(t)
	srv := sourceServer(t)
	h := apitest.Router(apps.New(pool, slog.New(slog.DiscardHandler), apitest.RequireUser, safehttp.New(time.Second)))
	owner := apitest.NewUser(t, pool)
	for _, url := range []string{srv.URL + "/good.json", "https://169.254.169.254/latest", "https://[::1]/s.json"} {
		if rec := apitest.Do(t, h, owner, "POST", "/api/sources", `{"url":"`+url+`"}`); rec.Code != 422 {
			t.Errorf("%s: %d", url, rec.Code)
		}
	}
}

func TestDeviceApps(t *testing.T) {
	pool := storetest.New(t)
	h := apitest.Router(apps.New(pool, slog.New(slog.DiscardHandler), apitest.RequireUser, http.DefaultClient))
	owner, rival := apitest.NewUser(t, pool), apitest.NewUser(t, pool)
	device := uuid.New()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "INSERT INTO devices (id, user_id, name) VALUES ($1, $2, 'phone')", device, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO device_apps (device_id, bundle_id, name) VALUES ($1, 'a.b', 'App')", device); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		user   uuid.UUID
		id     string
		want   int
		inBody string
	}{
		{"owner", owner, device.String(), 200, `"bundle_id":"a.b"`},
		{"rival", rival, device.String(), 404, ""},
		{"unknown", owner, uuid.NewString(), 404, ""},
		{"signed out", uuid.Nil, device.String(), 401, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := apitest.Do(t, h, tt.user, "GET", "/api/devices/"+tt.id+"/apps", "")
			if rec.Code != tt.want || !strings.Contains(rec.Body.String(), tt.inBody) {
				t.Errorf("got %d %s", rec.Code, rec.Body)
			}
		})
	}
}
