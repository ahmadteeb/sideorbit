package safehttp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"
)

func TestPublic(t *testing.T) {
	tests := []struct {
		addr string
		want bool
	}{
		{"93.184.216.34", true}, {"2606:2800:220:1:248:1893:25c8:1946", true},
		{"127.0.0.1", false}, {"::1", false}, {"0.0.0.0", false}, {"::", false},
		{"10.1.2.3", false}, {"172.16.0.1", false}, {"192.168.1.1", false}, {"fd00::1", false},
		{"169.254.169.254", false}, {"fe80::1", false}, {"100.64.0.1", false}, {"224.0.0.1", false},
		{"::ffff:127.0.0.1", false}, {"::ffff:10.0.0.1", false}, {"64:ff9b::a00:1", false},
		{"2002:a00:1::", false}, {"::127.0.0.1", false}, {"255.255.255.255", false},
	}
	for _, tt := range tests {
		if got := Public(netip.MustParseAddr(tt.addr)); got != tt.want {
			t.Errorf("Public(%s) = %v, want %v", tt.addr, got, tt.want)
		}
	}
}

func TestClientRefusesLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("the request reached a loopback server")
	}))
	defer srv.Close()
	_, err := New(time.Second).Get(srv.URL)
	if !errors.Is(err, ErrBlocked) {
		t.Errorf("err = %v", err)
	}
}

func TestRequireHTTPS(t *testing.T) {
	tests := []struct {
		url     string
		hops    int
		wantErr bool
	}{
		{"https://example.com", 1, false},
		{"http://example.com", 1, true},
		{"https://example.com", 4, true},
	}
	for _, tt := range tests {
		r := httptest.NewRequest(http.MethodGet, tt.url, nil)
		if err := RequireHTTPS(r, make([]*http.Request, tt.hops)); (err != nil) != tt.wantErr {
			t.Errorf("%s after %d hops: %v", tt.url, tt.hops, err)
		}
	}
}
