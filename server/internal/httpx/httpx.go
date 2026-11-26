// Package httpx holds the JSON request and response helpers shared by handlers.
// Errors follow the shape in docs/API.md.
package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// MaxBody is the largest JSON request body accepted.
const MaxBody = 256 << 10

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Fix     string `json:"fix,omitempty"`
}

// JSON writes v with the given status.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Error writes {"error": {code, message, fix}}.
func Error(w http.ResponseWriter, status int, code, message, fix string) {
	JSON(w, status, map[string]errorBody{"error": {code, message, fix}})
}

// Internal logs err and answers 500 without leaking the cause.
func Internal(w http.ResponseWriter, r *http.Request, err error) {
	slog.ErrorContext(r.Context(), "internal error", "err", err)
	Error(w, http.StatusInternalServerError, "internal", "Something went wrong on the server.", "Try again.")
}

// Decode reads one JSON object into v. Unknown fields are rejected so that data
// the server must never receive cannot slip in unnoticed. On failure it writes
// the error response and returns false.
func Decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, MaxBody))
	dec.DisallowUnknownFields()
	err := dec.Decode(v)
	if err == nil && dec.Decode(&struct{}{}) != io.EOF {
		err = errors.New("unexpected data after the JSON object")
	}
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			Error(w, http.StatusRequestEntityTooLarge, "body_too_large", "The request body is too large.", "")
			return false
		}
		// The decoder's message can quote input, so it is not echoed.
		Error(w, http.StatusBadRequest, "invalid_body", "The request body is not valid JSON for this endpoint.", "Check the field names and types.")
		return false
	}
	return true
}

// ClientIP returns the caller's address for rate limiting. X-Forwarded-For is
// trusted only when the direct peer is a private address (our reverse proxy),
// and only its last entry, which that proxy wrote. IPv6 is grouped by /64.
func ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return host
	}
	if addr.IsPrivate() || addr.IsLoopback() {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			if fwd, err := netip.ParseAddr(strings.TrimSpace(parts[len(parts)-1])); err == nil {
				addr = fwd
			}
		}
	}
	if addr = addr.Unmap(); addr.Is6() {
		if p, err := addr.Prefix(64); err == nil {
			return p.String()
		}
	}
	return addr.String()
}
