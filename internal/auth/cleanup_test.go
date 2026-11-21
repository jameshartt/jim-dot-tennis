// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCleanupDeletesWhatHasNoFurtherUse(t *testing.T) {
	s := newTestService(t, DefaultConfig()) // 7 day sliding, 30 day absolute
	now := time.Now()
	day := 24 * time.Hour
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := s.db.Exec(query, args...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	session := func(name string, created, expires time.Time, valid bool) {
		exec(`INSERT INTO sessions (token_hash, user_id, role, created_at, expires_at, last_activity_at, ip, user_agent, device_info, is_valid)
			VALUES (?, 1, 'admin', ?, ?, ?, '', '', '', ?)`, name, created, expires, created, valid)
	}
	session("live", now.Add(-day), now.Add(6*day), true)
	session("expired", now.Add(-10*day), now.Add(-3*day), true)
	session("signed-out", now.Add(-day), now.Add(6*day), false)
	session("past-absolute-cap", now.Add(-31*day), now.Add(day), true)

	attempt := func(name string, at time.Time) {
		exec(`INSERT INTO login_attempts (username, ip, user_agent, success, created_at) VALUES (?, '203.0.113.1', '', FALSE, ?)`, name, at)
	}
	attempt("recent", now.Add(-time.Hour))
	attempt("89-days", now.Add(-89*day))
	attempt("91-days", now.Add(-91*day))

	link := func(name string, created, expires time.Time, used, revoked bool) {
		var usedAt, revokedAt any
		if used {
			usedAt = created.Add(time.Hour)
		}
		if revoked {
			revokedAt = created.Add(time.Hour)
		}
		exec(`INSERT INTO account_links (user_id, kind, token_hash, created_at, expires_at, used_at, revoked_at) VALUES (1, 'invite', ?, ?, ?, ?, ?)`,
			name, created, expires, usedAt, revokedAt)
	}
	link("live", now.Add(-time.Hour), now.Add(47*time.Hour), false, false)
	link("used-recently", now.Add(-2*day), now.Add(-day), true, false)
	link("used-long-ago", now.Add(-40*day), now.Add(-38*day), true, false)
	link("revoked-long-ago", now.Add(-40*day), now.Add(-38*day), false, true)
	link("expired-long-ago", now.Add(-40*day), now.Add(-38*day), false, false)

	res, err := s.Cleanup(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if res != (CleanupResult{Sessions: 3, LoginAttempts: 1, Links: 3}) {
		t.Errorf("cleanup result %+v", res)
	}
	left := func(query string) string {
		t.Helper()
		var names []string
		if err := s.db.Select(&names, query); err != nil {
			t.Fatal(err)
		}
		return strings.Join(names, ",")
	}
	if got := left(`SELECT token_hash FROM sessions ORDER BY id`); got != "live" {
		t.Errorf("sessions left: %s", got)
	}
	if got := left(`SELECT username FROM login_attempts ORDER BY id`); got != "recent,89-days" {
		t.Errorf("login attempts left: %s", got)
	}
	if got := left(`SELECT token_hash FROM account_links ORDER BY id`); got != "live,used-recently" {
		t.Errorf("links left: %s", got)
	}

	// The audit log is never cleaned up.
	exec(`INSERT INTO account_audit_log (created_at, actor_kind, action) VALUES (?, 'system', 'old')`, now.Add(-5*365*day))
	if _, err := s.Cleanup(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if got := left(`SELECT action FROM account_audit_log`); got != "old" {
		t.Errorf("audit log after cleanup: %q", got)
	}
}

// The scheduled cleanup runs shortly after it is started and stops when told.
func TestStartCleanupRunsAndStops(t *testing.T) {
	s := newTestService(t, DefaultConfig())
	if _, err := s.db.Exec(`INSERT INTO sessions (token_hash, user_id, role, expires_at, ip, user_agent, device_info, is_valid)
		VALUES ('gone', 1, 'admin', ?, '', '', '', FALSE)`, time.Now()); err != nil {
		t.Fatal(err)
	}
	stop := s.StartCleanup(context.Background(), 10*time.Millisecond, time.Hour)
	deadline := time.Now().Add(5 * time.Second)
	for {
		var n int
		if err := s.db.Get(&n, `SELECT COUNT(*) FROM sessions`); err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the scheduled cleanup did not run")
		}
		time.Sleep(10 * time.Millisecond)
	}
	stopped := make(chan struct{})
	go func() { stop(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("stop did not return")
	}

	// Cancelling the context stops it too.
	ctx, cancel := context.WithCancel(context.Background())
	stop2 := s.StartCleanup(ctx, time.Hour, time.Hour)
	cancel()
	stop2()
}
