package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHandler(t *testing.T) {
	Request("GET", "/api/devices/{id}", 200, time.Millisecond)
	Request("BREW", "", 404, time.Millisecond)
	Request("GET", "/anisette/v3/provisioning_session", 0, time.Second)
	Push.WithLabelValues("sent").Inc()

	get := func(path string) (int, string) {
		rec := httptest.NewRecorder()
		Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec.Code, rec.Body.String()
	}
	code, body := get("/metrics")
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	for _, want := range []string{
		`orbit_http_requests_total{method="GET",route="/api/devices/{id}",status="200"} 1`,
		`orbit_http_requests_total{method="other",route="other",status="404"} 1`,
		`route="/anisette/v3/provisioning_session",status="101"} 1`,
		`orbit_push_notifications_total{result="sent"} 1`,
		"orbit_scheduler_last_success_timestamp_seconds",
		"go_goroutines",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s", want)
		}
	}
	if code, _ := get("/healthz"); code != 404 {
		t.Errorf("the metrics port serves more than metrics: %d", code)
	}
}
