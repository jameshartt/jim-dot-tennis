// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"net"
	"net/http"
	"strings"
)

// clientIP returns the address of the client behind a request, without the
// per-connection port. It is the only place the client address is derived;
// every throttle (password, passkey, link, setup code) and every recorded
// address goes through it (throttles via ThrottleKey).
//
// Trust assumption: the app's own port is reachable only through the reverse
// proxy (Caddy) on a private network, as in deploy/docker-compose.yml, where
// only Caddy publishes ports. When the direct peer is a private or loopback
// address, the rightmost X-Forwarded-For entry is used: that is the one the
// proxy itself appended (Caddy replaces or appends to whatever the client
// sent, so anything a client writes in the header is to the left of it). A
// public peer is the client itself and its header is ignored, so a direct
// caller cannot choose its own address, dodge a throttle or put a throttle on
// somebody else's address. If the app port were published directly (for
// example `ports: "8080:8080"`, as the development compose file does), Docker
// would make every client appear to come from a private gateway address and
// clients could then pick their throttle address with the header: never
// expose the app port without the proxy in front of it in production.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	peer := net.ParseIP(host)
	if peer == nil || !(peer.IsPrivate() || peer.IsLoopback()) {
		return host
	}
	hops := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	if last := strings.TrimSpace(hops[len(hops)-1]); net.ParseIP(last) != nil {
		return last
	}
	return host
}
