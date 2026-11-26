package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecode(t *testing.T) {
	tests := []struct {
		name, body string
		wantStatus int
	}{
		{"valid", `{"name":"a"}`, http.StatusOK},
		{"unknown field", `{"name":"a","extra":1}`, http.StatusBadRequest},
		{"trailing data", `{"name":"a"}{}`, http.StatusBadRequest},
		{"not json", `name=a`, http.StatusBadRequest},
		{"too large", `{"name":"` + strings.Repeat("a", MaxBody) + `"}`, http.StatusRequestEntityTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			var v struct {
				Name string `json:"name"`
			}
			ok := Decode(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body)), &v)
			if rec.Code != tt.wantStatus || ok != (tt.wantStatus == http.StatusOK) {
				t.Errorf("status = %d ok = %v", rec.Code, ok)
			}
		})
	}
}

func TestClientIP(t *testing.T) {
	tests := []struct{ name, remote, xff, want string }{
		{"direct", "203.0.113.5:1000", "", "203.0.113.5"},
		{"spoofed header from public peer", "203.0.113.5:1000", "198.51.100.1", "203.0.113.5"},
		{"behind proxy", "172.18.0.2:1000", "198.51.100.1", "198.51.100.1"},
		{"last hop wins", "172.18.0.2:1000", "1.2.3.4, 198.51.100.1", "198.51.100.1"},
		{"ipv6 grouped", "[2001:db8:1:2:3:4:5:6]:1000", "", "2001:db8:1:2::/64"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tt.remote
			if tt.xff != "" {
				r.Header.Set("X-Forwarded-For", tt.xff)
			}
			if got := ClientIP(r); got != tt.want {
				t.Errorf("ClientIP = %q, want %q", got, tt.want)
			}
		})
	}
}
