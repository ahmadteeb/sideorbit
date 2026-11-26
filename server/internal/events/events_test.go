package events_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/ahmadteeb/sideorbit/server/internal/apitest"
	"github.com/ahmadteeb/sideorbit/server/internal/events"
	"github.com/ahmadteeb/sideorbit/server/internal/store/storetest"
)

type page struct {
	Events []struct {
		ID         string `json:"id"`
		DeviceName string `json:"device_name"`
		Result     string `json:"result"`
		OccurredAt string `json:"occurred_at"`
	} `json:"events"`
	NextCursor *string `json:"next_cursor"`
}

func TestList(t *testing.T) {
	pool := storetest.New(t)
	h := apitest.Router(events.New(pool, apitest.RequireUser))
	owner, rival := apitest.NewUser(t, pool), apitest.NewUser(t, pool)
	phone, tablet := uuid.New(), uuid.New()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec("INSERT INTO devices (id, user_id, name) VALUES ($1, $3, 'phone'), ($2, $3, 'tablet')", phone, tablet, owner)
	// 60 successes on the phone, many sharing a timestamp so paging must also order by id.
	exec("INSERT INTO refresh_events (id, device_id, kind, result, occurred_at) SELECT gen_random_uuid(), $1, 'refresh', 'success', now() - (i/10) * interval '1 hour' FROM generate_series(1, 60) i", phone)
	exec("INSERT INTO refresh_events (id, device_id, kind, result, occurred_at) VALUES (gen_random_uuid(), $1, 'refresh', 'failure', now() + interval '1 second')", tablet)

	get := func(user uuid.UUID, query string) page {
		t.Helper()
		rec := apitest.Do(t, h, user, "GET", "/api/events"+query, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", query, rec.Code, rec.Body)
		}
		return apitest.JSON[page](t, rec)
	}

	first := get(owner, "")
	if len(first.Events) != 50 || first.NextCursor == nil || first.Events[0].DeviceName != "tablet" {
		t.Fatalf("first page: %d events", len(first.Events))
	}
	second := get(owner, "?cursor="+*first.NextCursor)
	if len(second.Events) != 11 || second.NextCursor != nil {
		t.Fatalf("second page: %d events", len(second.Events))
	}
	seen := map[string]bool{}
	for _, e := range append(first.Events, second.Events...) {
		seen[e.ID] = true
	}
	if len(seen) != 61 {
		t.Errorf("paging returned %d distinct events, want 61", len(seen))
	}

	tests := []struct {
		name  string
		user  uuid.UUID
		query string
		want  int
	}{
		{"by device", owner, "?device_id=" + tablet.String(), 1},
		{"by result", owner, "?result=failure", 1},
		{"both", owner, "?device_id=" + phone.String() + "&result=failure", 0},
		{"rival sees nothing", rival, "", 0},
		{"rival cannot filter to the owner's device", rival, "?device_id=" + phone.String(), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := len(get(tt.user, tt.query).Events); got != tt.want {
				t.Errorf("%d events, want %d", got, tt.want)
			}
		})
	}
	for _, query := range []string{"?device_id=x", "?result=maybe", "?cursor=%21", "?cursor=YWJj"} {
		if rec := apitest.Do(t, h, owner, "GET", "/api/events"+query, ""); rec.Code != 400 {
			t.Errorf("%s: %d", query, rec.Code)
		}
	}
	if rec := apitest.Do(t, h, uuid.Nil, "GET", "/api/events", ""); rec.Code != 401 {
		t.Errorf("signed out: %d", rec.Code)
	}
}
