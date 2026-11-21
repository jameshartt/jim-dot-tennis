// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

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

func TestThrottleKey(t *testing.T) {
	for _, tc := range []struct{ addr, want string }{
		{"203.0.113.9", "203.0.113.9"},
		{"::ffff:203.0.113.9", "203.0.113.9"},                     // IPv4-mapped IPv6 is the IPv4 client
		{"2001:db8:1:2:3:4:5:6", "2001:db8:1:2::/64"},             // one /64 is one client
		{"2001:db8:1:2:ffff:ffff:ffff:ffff", "2001:db8:1:2::/64"}, // ...whatever the interface id
		{"2001:db8:1:3::1", "2001:db8:1:3::/64"},                  // the next /64 is another client
		{"not-an-addr", "not-an-addr"},
	} {
		if got := throttleKey(tc.addr); got != tc.want {
			t.Errorf("throttleKey(%q) = %q, want %q", tc.addr, got, tc.want)
		}
	}
	// ThrottleKey uses the same client address as everything else.
	r := httptest.NewRequest("POST", "/login", nil)
	r.RemoteAddr = "172.18.0.5:40001"
	r.Header.Set("X-Forwarded-For", "2001:db8:aa:bb::77")
	if got := ThrottleKey(r); got != "2001:db8:aa:bb::/64" {
		t.Errorf("ThrottleKey behind the proxy = %q", got)
	}
}

func loginAs(s *Service, username, remoteAddr, password string) error {
	r := httptest.NewRequest("POST", "/login", nil)
	r.RemoteAddr = remoteAddr
	_, err := s.Login(username, password, r)
	return err
}

func failedAttempts(t *testing.T, s *Service, where string, args ...any) int {
	t.Helper()
	var n int
	if err := s.db.Get(&n, `SELECT COUNT(*) FROM login_attempts WHERE success = FALSE AND `+where, args...); err != nil {
		t.Fatal(err)
	}
	return n
}

// One password tried against many usernames from one address (spraying) is
// throttled per address; other addresses are unaffected.
func TestLoginThrottlePerAddress(t *testing.T) {
	s := newTestService(t, DefaultConfig())
	hash, _ := bcrypt.GenerateFromPassword([]byte("right-password"), bcrypt.MinCost)
	if _, err := s.db.Exec(`INSERT INTO users (username, password_hash, role) VALUES ('captain', ?, 'admin')`, string(hash)); err != nil {
		t.Fatal(err)
	}
	const sprayer = "203.0.113.50:1000"
	for i := 0; i < MaxLoginFailuresPerAddress; i++ {
		if err := loginAs(s, fmt.Sprintf("user%d", i), sprayer, "Summer2026!"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("spray %d: %v", i, err)
		}
	}
	// Every name is now refused from that address, even with the right password.
	if err := loginAs(s, "captain", sprayer, "right-password"); !errors.Is(err, ErrTooManyAttempts) {
		t.Fatalf("after spraying: got %v, want ErrTooManyAttempts", err)
	}
	// The named user signs in from anywhere else.
	if err := loginAs(s, "captain", "198.51.100.7:1000", "right-password"); err != nil {
		t.Fatalf("captain from another address: %v", err)
	}
	// Passkey failures recorded for the address do not count towards it.
	s2 := newTestService(t, DefaultConfig())
	for i := 0; i < MaxLoginFailuresPerAddress; i++ {
		r := httptest.NewRequest("POST", "/login/passkey/finish", nil)
		r.RemoteAddr = sprayer
		s2.recordLoginAttempt(passkeyAttemptUsername, r, false)
	}
	fromSprayer := httptest.NewRequest("POST", "/login", nil)
	fromSprayer.RemoteAddr = sprayer
	if throttled, err := s2.loginThrottled("captain", fromSprayer); err != nil || throttled {
		t.Fatalf("passkey failures throttled password sign-in: %v %v", throttled, err)
	}
}

// A throttled attempt is refused without being recorded, so retrying while
// locked out does not extend the lockout, and the lockout is for that name
// from that address only.
func TestThrottledAttemptsDoNotExtendTheLockout(t *testing.T) {
	s := newTestService(t, DefaultConfig())
	hash, _ := bcrypt.GenerateFromPassword([]byte("right-password"), bcrypt.MinCost)
	if _, err := s.db.Exec(`INSERT INTO users (username, password_hash, role) VALUES ('captain', ?, 'admin')`, string(hash)); err != nil {
		t.Fatal(err)
	}
	const attacker = "203.0.113.60:1000"
	for i := 0; i < s.config.MaxLoginAttempts; i++ {
		if err := loginAs(s, "captain", attacker, "guess"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("guess %d: %v", i, err)
		}
	}
	for i := 0; i < 10; i++ {
		if err := loginAs(s, "captain", attacker, "guess"); !errors.Is(err, ErrTooManyAttempts) {
			t.Fatalf("throttled guess %d: %v", i, err)
		}
	}
	if n := failedAttempts(t, s, `username = 'captain'`); n != s.config.MaxLoginAttempts {
		t.Errorf("%d failed attempts recorded, want %d (throttled ones are not recorded)", n, s.config.MaxLoginAttempts)
	}
	// The attacker cannot lock captain out from elsewhere...
	if err := loginAs(s, "captain", "198.51.100.8:1000", "right-password"); err != nil {
		t.Fatalf("captain from another address: %v", err)
	}
	// ...and the lockout ends with the window, however often it was retried.
	if _, err := s.db.Exec(`UPDATE login_attempts SET created_at = ?`, time.Now().Add(-s.config.LoginAttemptWindow-time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := loginAs(s, "captain", attacker, "right-password"); err != nil {
		t.Fatalf("after the window: %v", err)
	}
}

// Password failures against a name never block that person's passkey, not
// even from the attacker's own address.
func TestPasswordLockoutNeverBlocksPasskeySignIn(t *testing.T) {
	env := newPasskeyEnv(t)
	a, cred, _ := env.enrolledUser("alice", false)
	attacker := env.browser("203.0.113.66")
	for i := 0; i < MaxLoginFailuresPerAddress; i++ {
		username := "alice"
		if i >= DefaultConfig().MaxLoginAttempts {
			username = fmt.Sprintf("someone%d", i)
		}
		attacker.postForm("/login", url.Values{"username": {username}, "password": {"wrong"}})
	}
	if rec := attacker.postForm("/login", url.Values{"username": {"alice"}, "password": {testPassword}}); attacker.signedIn() || !strings.Contains(rec.Body.String(), ErrTooManyAttempts.Error()) {
		t.Fatalf("password sign-in from the throttled address was not refused: %d", rec.Code)
	}
	// Alice's passkey works from her address and from the attacker's.
	wantStatus(t, env.browser("198.51.100.1").passkeySignIn(a, cred), http.StatusOK)
	wantStatus(t, env.browser("203.0.113.66").passkeySignIn(a, cred), http.StatusOK)
	// And her password from her own address.
	env.browser("198.51.100.1").passwordSignIn("alice")
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
