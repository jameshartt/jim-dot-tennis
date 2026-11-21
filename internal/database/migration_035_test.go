// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package database

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestMigration035HashedSessionsAndAuditLog migrates a populated database
// 34 -> 35 -> 34 -> 35 through the real migration driver. Migration 035
// replaces the sessions table (raw tokens as keys) with one keyed by a number
// and holding only token hashes, ending every session once, and adds the
// append-only account audit log. Nothing else may change: replacing sessions
// is only safe because no table references it, which this test checks, and
// every table that references users must come through untouched.
func TestMigration035HashedSessionsAndAuditLog(t *testing.T) {
	db, err := New(Config{Driver: DriverSQLite, FilePath: filepath.Join(t.TempDir(), "test.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := db.Exec(query, args...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	count := func(query string, args ...any) int {
		t.Helper()
		var n int
		if err := db.Get(&n, query, args...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		return n
	}
	fails := func(what, query string, args ...any) {
		t.Helper()
		if _, err := db.Exec(query, args...); err == nil {
			t.Errorf("%s: statement succeeded, want an error", what)
		}
	}

	migrateTo(t, db, 34)
	if count(`PRAGMA foreign_keys`) != 1 {
		t.Fatal("foreign keys are not enforced on this connection; the test would prove nothing")
	}

	// A populated database as migration 034 leaves it, with live sessions
	// keyed by their raw tokens.
	exec(`INSERT INTO clubs (id, name) VALUES (1, 'Club')`)
	exec(`INSERT INTO players (id, first_name, last_name, club_id) VALUES ('p-ann', 'Ann', 'Admin', 1), ('p-cap', 'Cap', 'Tain', 1)`)
	exec(`INSERT INTO users (id, username, password_hash, role, player_id, is_active, created_at, last_login_at) VALUES
		(1, 'ann', '$2a$12$annhash', 'admin',   'p-ann', TRUE, '2026-01-02 03:04:05', '2026-02-03 04:05:06'),
		(2, 'cap', NULL,             'captain', 'p-cap', TRUE, '2026-01-03 03:04:05', '2026-02-04 04:05:06')`)
	rawTokens := []string{"raw-token-AAAAAAAAAAAAAAAAAAAAAAAA", "raw-token-BBBBBBBBBBBBBBBBBBBBBBBB", "raw-token-CCCCCCCCCCCCCCCCCCCCCCCC"}
	for i, token := range rawTokens {
		exec(`INSERT INTO sessions (id, user_id, role, created_at, expires_at, last_activity_at, ip, user_agent, device_info, is_valid)
			VALUES (?, ?, 'admin', '2026-03-01 00:00:00', '2099-03-08 00:00:00', '2026-03-01 00:00:00', '203.0.113.1', 'ua', 'ua', TRUE)`,
			token, 1+i%2)
	}
	exec(`INSERT INTO webauthn_user_handles (user_id, handle) VALUES (1, x'0101'), (2, x'0202')`)
	exec(`INSERT INTO webauthn_credentials (user_id, credential_id, public_key, name) VALUES (1, x'a1', x'b1', 'Ann phone'), (2, x'a2', x'b2', 'Cap key')`)
	exec(`INSERT INTO captain_player_notes (player_id, author_user_id, kind, body) VALUES ('p-cap', 1, 'general', 'note by ann')`)
	exec(`INSERT INTO account_links (user_id, kind, token_hash, expires_at) VALUES (2, 'invite', 'link-hash-1', '2099-01-01 00:00:00')`)
	exec(`INSERT INTO login_attempts (username, ip, user_agent, success) VALUES ('ann', '203.0.113.1', 'ua', TRUE)`)

	// Replacing sessions is only safe while nothing references it.
	if rows := dump(t, db, `SELECT name FROM sqlite_master WHERE type = 'table' AND sql LIKE '%REFERENCES sessions%'`); len(rows) != 0 {
		t.Fatalf("tables reference sessions; migration 035 would cascade into them: %v", rows)
	}

	kept := map[string]string{
		"users":                 `SELECT id, username, password_hash, role, player_id, is_active, created_at, last_login_at FROM users ORDER BY id`,
		"webauthn_user_handles": `SELECT * FROM webauthn_user_handles ORDER BY user_id`,
		"webauthn_credentials":  `SELECT * FROM webauthn_credentials ORDER BY id`,
		"captain_player_notes":  `SELECT * FROM captain_player_notes ORDER BY id`,
		"account_links":         `SELECT * FROM account_links ORDER BY id`,
		"login_attempts":        `SELECT * FROM login_attempts ORDER BY id`,
		"users sequence":        `SELECT seq FROM sqlite_sequence WHERE name = 'users'`,
	}
	// The test must notice a table that starts referencing users later.
	for _, row := range dump(t, db, `SELECT name FROM sqlite_master WHERE type = 'table' AND sql LIKE '%REFERENCES users%' AND name <> 'sessions' ORDER BY name`) {
		name := strings.Trim(row, `["]`)
		if _, ok := kept[name]; !ok {
			t.Fatalf("table %s references users but is not covered by this test: add it", name)
		}
	}
	before := map[string][]string{}
	for name, query := range kept {
		if before[name] = dump(t, db, query); len(before[name]) == 0 {
			t.Fatalf("%s: nothing to compare", name)
		}
	}
	unchanged := func(stage string) {
		t.Helper()
		for name, query := range kept {
			if after := dump(t, db, query); strings.Join(after, "\n") != strings.Join(before[name], "\n") {
				t.Errorf("%s: %s changed\nbefore: %v\nafter:  %v", stage, name, before[name], after)
			}
		}
		if rows := dump(t, db, `PRAGMA foreign_key_check`); len(rows) != 0 {
			t.Errorf("%s: foreign key violations: %v", stage, rows)
		}
		if rows := dump(t, db, `PRAGMA integrity_check`); len(rows) != 1 || !strings.Contains(rows[0], "ok") {
			t.Errorf("%s: integrity check: %v", stage, rows)
		}
	}
	noRawTokens := func(stage string) {
		t.Helper()
		all := strings.Join(dump(t, db, `SELECT * FROM sessions`), "\n")
		for _, token := range rawTokens {
			if strings.Contains(all, token) {
				t.Errorf("%s: a raw session token is still stored", stage)
			}
		}
	}

	// Up: 34 -> 35. Everyone is signed out once; nothing else changes.
	migrateTo(t, db, 35)
	unchanged("after up")
	noRawTokens("after up")
	if n := count(`SELECT COUNT(*) FROM sessions`); n != 0 {
		t.Errorf("%d sessions after up, want 0 (everyone signed out once)", n)
	}
	// The new sessions table: numeric id, unique token hash, still tied to users.
	exec(`INSERT INTO sessions (token_hash, user_id, role, expires_at, ip, user_agent, device_info) VALUES ('h1', 1, 'admin', '2099-01-01', '', '', '')`)
	if id := count(`SELECT id FROM sessions WHERE token_hash = 'h1'`); id != 1 {
		t.Errorf("first new session id %d, want 1", id)
	}
	fails("duplicate token hash", `INSERT INTO sessions (token_hash, user_id, role, expires_at, ip, user_agent, device_info) VALUES ('h1', 2, 'admin', '2099-01-01', '', '', '')`)
	fails("session for a missing user", `INSERT INTO sessions (token_hash, user_id, role, expires_at, ip, user_agent, device_info) VALUES ('h2', 404, 'admin', '2099-01-01', '', '', '')`)
	fails("session without a token hash", `INSERT INTO sessions (user_id, role, expires_at, ip, user_agent, device_info) VALUES (1, 'admin', '2099-01-01', '', '', '')`)
	if n := count(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name IN ('idx_sessions_user_id', 'idx_sessions_expires_at', 'idx_sessions_is_valid')`); n != 3 {
		t.Errorf("%d session indexes after up, want 3", n)
	}
	// The audit log: appendable, not editable, no foreign keys (history
	// outlives accounts).
	exec(`INSERT INTO account_audit_log (created_at, actor_kind, actor, action, target_user_id, target_username) VALUES ('2026-11-21 10:00:00', 'cli', 'user create', 'user.create', 404, 'gone')`)
	fails("actor kind check", `INSERT INTO account_audit_log (created_at, actor_kind, action) VALUES ('2026-11-21', 'robot', 'x')`)
	fails("audit update", `UPDATE account_audit_log SET detail = 'edited'`)
	fails("audit delete", `DELETE FROM account_audit_log`)
	if n := count(`SELECT COUNT(*) FROM account_audit_log`); n != 1 {
		t.Errorf("%d audit rows, want 1", n)
	}

	// Down: 35 -> 34. The audit log goes; sessions are keyed by token again
	// and, since a hash cannot become a token, empty.
	migrateTo(t, db, 34)
	unchanged("after down")
	if n := count(`SELECT COUNT(*) FROM sqlite_master WHERE name LIKE 'account_audit_log%'`); n != 0 {
		t.Errorf("audit log objects left after down: %d", n)
	}
	if n := count(`SELECT COUNT(*) FROM sessions`); n != 0 {
		t.Errorf("%d sessions after down, want 0", n)
	}
	exec(`INSERT INTO sessions (id, user_id, role, expires_at, ip, user_agent, device_info) VALUES ('old-style-token', 1, 'admin', '2099-01-01', '', '', '')`)
	fails("after down: session for a missing user", `INSERT INTO sessions (id, user_id, role, expires_at, ip, user_agent, device_info) VALUES ('s', 404, 'admin', '2099-01-01', '', '', '')`)

	// Up again.
	migrateTo(t, db, 35)
	unchanged("after up again")
	if n := count(`SELECT COUNT(*) FROM sessions`) + count(`SELECT COUNT(*) FROM account_audit_log`); n != 0 {
		t.Errorf("after up again: %d sessions and audit rows, want fresh empty tables", n)
	}

	// Sessions still go with their user.
	exec(`INSERT INTO sessions (token_hash, user_id, role, expires_at, ip, user_agent, device_info) VALUES ('h3', 2, 'captain', '2099-01-01', '', '', '')`)
	exec(`DELETE FROM captain_player_notes`)
	exec(`DELETE FROM users WHERE id = 2`)
	if n := count(`SELECT COUNT(*) FROM sessions WHERE user_id = 2`); n != 0 {
		t.Errorf("%d sessions left after their user was deleted", n)
	}
}
