// Command server is the Orbit API server.
package main

import (
	"cmp"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"
)

func newHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ok\n")
	})
	return mux
}

func main() {
	srv := &http.Server{
		Addr:              cmp.Or(os.Getenv("HTTP_ADDR"), ":8080"),
		Handler:           newHandler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	slog.Info("listening", "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
