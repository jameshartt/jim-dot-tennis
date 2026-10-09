// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"net"
	"net/http"
	"strings"
)

// clientIP returns the address of the client behind a request, without the
// per-connection port. In production every request arrives through Caddy on
// the Docker network, so when the direct peer is a private or loopback
// address the rightmost X-Forwarded-For entry (the one the proxy appended)
// is used. A public peer is the client itself and its header is ignored, so
// a direct caller cannot pick its own address.
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
