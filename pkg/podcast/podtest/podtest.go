// Package podtest holds the setup a test binary needs before it runs anything
// that could reach the network or sleep on a retry.
//
// It exists because the suite used to do neither: feed fetches went to the
// real internet, and a failed fetch waited out a one- then two-second backoff.
// A single test that named an unreachable feed cost three and a half seconds,
// and the whole suite's timing depended on DNS.
package podtest

import (
	"fmt"
	"net"
	"net/http"
	"strings"
)

// offlineTransport serves loopback requests — httptest servers live there —
// and fails everything else immediately rather than waiting for DNS or a
// connect timeout.
type offlineTransport struct{ inner http.RoundTripper }

func (t offlineTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if isLoopback(req.URL.Hostname()) {
		return t.inner.RoundTrip(req)
	}
	return nil, fmt.Errorf("podtest: refusing to reach %q; tests must not use the network", req.URL.Host)
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// OfflineTransport serves loopback and refuses everything else. Install it
// with podcast.SetFeedTransport from a TestMain, alongside a zero retry delay.
//
// It deliberately does not wire itself up: podcast's own tests need it too, and
// importing podcast here would be a cycle.
func OfflineTransport() http.RoundTripper {
	return offlineTransport{inner: http.DefaultTransport}
}
