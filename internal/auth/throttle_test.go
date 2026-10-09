// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"errors"
	"net/http/httptest"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func loginFrom(s *Service, remoteAddr, forwardedFor, password string) error {
	r := httptest.NewRequest("POST", "/login", nil)
	r.RemoteAddr = remoteAddr
	if forwardedFor != "" {
		r.Header.Set("X-Forwarded-For", forwardedFor)
	}
	_, err := s.Login("captain", password, r)
	return err
}

// The throttle must recognise the same client across connections. Every
// request reaches the app through Caddy, so RemoteAddr is the proxy's
// address plus a per-connection port, and the client is in X-Forwarded-For.
func TestLoginThrottleSpansConnections(t *testing.T) {
	s := newTestService(t, DefaultConfig())
	hash, _ := bcrypt.GenerateFromPassword([]byte("right-password"), bcrypt.MinCost)
	if _, err := s.db.Exec(`INSERT INTO users (username, password_hash, role) VALUES ('captain', ?, 'admin')`, string(hash)); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	// Five wrong guesses from one client, each over a new proxy connection.
	ports := []string{"40001", "40002", "40003", "40004", "40005"}
	for _, port := range ports {
		if err := loginFrom(s, "172.18.0.5:"+port, "203.0.113.9", "guess"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("guess via port %s: got %v", port, err)
		}
	}
	if err := loginFrom(s, "172.18.0.5:40006", "203.0.113.9", "right-password"); !errors.Is(err, ErrTooManyAttempts) {
		t.Fatalf("sixth attempt from the same client: got %v, want ErrTooManyAttempts", err)
	}

	// A different client behind the same proxy is unaffected.
	if err := loginFrom(s, "172.18.0.5:40007", "198.51.100.7", "right-password"); err != nil {
		t.Fatalf("other client: got %v, want a successful login", err)
	}

	// A directly connected client can't dodge the throttle by sending its
	// own X-Forwarded-For: it is only trusted from a private-network proxy.
	for i, port := range ports {
		spoof := "192.0.2." + string(rune('1'+i))
		if err := loginFrom(s, "203.0.113.50:"+port, spoof, "guess"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("direct guess %d: got %v", i, err)
		}
	}
	if err := loginFrom(s, "203.0.113.50:50000", "192.0.2.99", "right-password"); !errors.Is(err, ErrTooManyAttempts) {
		t.Fatalf("direct client rotating X-Forwarded-For: got %v, want ErrTooManyAttempts", err)
	}
}

func TestClientIP(t *testing.T) {
	for _, tc := range []struct{ remote, xff, want string }{
		{"203.0.113.9:1234", "", "203.0.113.9"},
		{"203.0.113.9:1234", "10.0.0.1", "203.0.113.9"},               // public peer: header ignored
		{"172.18.0.5:40001", "198.51.100.7", "198.51.100.7"},          // proxy peer
		{"172.18.0.5:40001", "6.6.6.6, 198.51.100.7", "198.51.100.7"}, // rightmost hop is the proxy's
		{"127.0.0.1:5555", "", "127.0.0.1"},
		{"[::1]:5555", "2001:db8::1", "2001:db8::1"},
		{"not-an-addr", "", "not-an-addr"},
	} {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = tc.remote
		if tc.xff != "" {
			r.Header.Set("X-Forwarded-For", tc.xff)
		}
		if got := clientIP(r); got != tc.want {
			t.Errorf("clientIP(%q, xff %q) = %q, want %q", tc.remote, tc.xff, got, tc.want)
		}
	}
}
