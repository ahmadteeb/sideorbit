// Package metrics holds the Prometheus series. Labels are bounded and carry no
// user, device, address or URL data.
package metrics

import (
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var registry = prometheus.NewRegistry()

var (
	requests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "orbit_http_requests_total", Help: "HTTP requests by route and status. Status 429 counts rate-limit hits.",
	}, []string{"method", "route", "status"})
	duration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "orbit_http_request_duration_seconds", Help: "Request duration by route. Provisioning WebSockets last as long as the session.",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30, 120},
	}, []string{"route"})
	// Push counts delivery attempts per subscription.
	Push = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "orbit_push_notifications_total", Help: "Web push notifications by result.",
	}, []string{"result"})
	// SchedulerSuccess lets an alert fire if expiry warnings stop being sent.
	SchedulerSuccess = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "orbit_scheduler_last_success_timestamp_seconds", Help: "When the scheduled jobs last all succeeded.",
	})
)

func init() {
	registry.MustRegister(requests, duration, Push, SchedulerSuccess,
		collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
}

var methods = []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions}

// Request records one finished request. route is the pattern, never the path.
func Request(method, route string, status int, took time.Duration) {
	if !slices.Contains(methods, method) {
		method = "other"
	}
	if route == "" {
		route = "other"
	}
	// A hijacked connection (WebSocket) leaves no status behind.
	if status == 0 {
		status = http.StatusSwitchingProtocols
	}
	requests.WithLabelValues(method, route, strconv.Itoa(status)).Inc()
	duration.WithLabelValues(route).Observe(took.Seconds())
}

// WatchPool adds the database connection gauges. Call once.
func WatchPool(pool *pgxpool.Pool) {
	gauge := func(name, help string, value func(*pgxpool.Stat) int32) {
		registry.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: name, Help: help},
			func() float64 { return float64(value(pool.Stat())) }))
	}
	gauge("orbit_db_connections_in_use", "Database connections in use.", (*pgxpool.Stat).AcquiredConns)
	gauge("orbit_db_connections_max", "Largest size of the connection pool.", (*pgxpool.Stat).MaxConns)
}

// Handler serves /metrics.
func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	return mux
}
