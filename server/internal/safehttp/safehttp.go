// Package safehttp builds HTTP clients for URLs that users supply. They refuse
// to connect to anything but public internet addresses (SSRF protection).
package safehttp

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"syscall"
	"time"
)

// ErrBlocked means the destination is not a public address.
var ErrBlocked = errors.New("destination address is not allowed")

// Ranges that net/netip calls global unicast but that are not public hosts, or
// that tunnel to an address we cannot inspect (NAT64, 6to4, Teredo).
var reserved = func() (out []netip.Prefix) {
	for _, s := range []string{
		"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24", "198.18.0.0/15",
		"198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4",
		"::/96", "64:ff9b::/96", "64:ff9b:1::/48", "100::/64", "2001::/32", "2001:db8::/32", "2002::/16", "fec0::/10",
	} {
		out = append(out, netip.MustParsePrefix(s))
	}
	return out
}()

// Public reports whether addr is an ordinary public internet address.
func Public(addr netip.Addr) bool {
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.Zone() != "" {
		return false
	}
	for _, p := range reserved {
		if p.Contains(addr) {
			return false
		}
	}
	return true
}

// control runs after DNS resolution with the address actually being dialled,
// so a name that re-resolves to a private address is still refused.
func control(_, address string, _ syscall.RawConn) error {
	ap, err := netip.ParseAddrPort(address)
	if err != nil || !Public(ap.Addr()) {
		return ErrBlocked
	}
	return nil
}

// RequireHTTPS rejects redirects that leave HTTPS or go on too long.
func RequireHTTPS(req *http.Request, via []*http.Request) error {
	if req.URL.Scheme != "https" {
		return fmt.Errorf("redirect to a non-https URL: %w", ErrBlocked)
	}
	if len(via) >= 4 {
		return errors.New("too many redirects")
	}
	return nil
}

// New returns a client limited to public destinations. It ignores proxy
// environment variables, which would bypass the address check.
func New(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext:         (&net.Dialer{Timeout: timeout, Control: control}).DialContext,
			TLSHandshakeTimeout: timeout,
			ForceAttemptHTTP2:   true,
			MaxIdleConns:        10,
			IdleConnTimeout:     30 * time.Second,
		},
		CheckRedirect: RequireHTTPS,
	}
}
