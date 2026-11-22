// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package database

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestMigration036TournamentAccess migrates a populated database
// 35 -> 36 -> 35 -> 36. Migration 036 adds users.tournament_access ('none',
// 'own' or 'all', default 'none', checked) without rebuilding users, so
// nothing that references users (sessions, passkeys, links) is touched.
func TestMigration036TournamentAccess(t *testing.T) {
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

	migrateTo(t, db, 35)
	exec(`INSERT INTO users (id, username, password_hash, role, is_active, created_at, last_login_at) VALUES
		(1, 'ann', '$2a$12$annhash', 'admin',   TRUE, '2026-01-02 03:04:05', '2026-02-03 04:05:06'),
		(2, 'cap', NULL,             'captain', TRUE, '2026-01-03 03:04:05', '2026-02-04 04:05:06'),
		(3, 'old', NULL,             'player',  TRUE, '2026-01-04 03:04:05', '2026-02-05 04:05:06')`)
	exec(`INSERT INTO sessions (token_hash, user_id, role, expires_at, ip, user_agent, device_info) VALUES ('h1', 2, 'captain', '2099-01-01 00:00:00', '', '', '')`)
	exec(`INSERT INTO webauthn_credentials (user_id, credential_id, public_key, attestation_type, aaguid, sign_count, transports, name)
		VALUES (2, X'0102', X'0304', 'none', X'00', 0, '', 'phone')`)
	usersBefore := dump(t, db, `SELECT id, username, password_hash, role, is_active, created_at, last_login_at FROM users ORDER BY id`)
	related := func() string {
		return strings.Join(dump(t, db, `SELECT user_id, token_hash FROM sessions ORDER BY id`), ";") + "|" +
			strings.Join(dump(t, db, `SELECT user_id, name FROM webauthn_credentials ORDER BY id`), ";")
	}
	relatedBefore := related()

	migrateTo(t, db, 36)
	var values []string
	if err := db.Select(&values, `SELECT tournament_access FROM users ORDER BY id`); err != nil {
		t.Fatal(err)
	}
	if strings.Join(values, ",") != "none,none,none" {
		t.Errorf("existing accounts have tournament access %q, want none for all", values)
	}
	if got := dump(t, db, `SELECT id, username, password_hash, role, is_active, created_at, last_login_at FROM users ORDER BY id`); strings.Join(got, "\n") != strings.Join(usersBefore, "\n") {
		t.Errorf("users changed:\n%q\n%q", usersBefore, got)
	}
	if related() != relatedBefore {
		t.Error("sessions or passkeys changed")
	}
	exec(`UPDATE users SET tournament_access = 'own' WHERE id = 3`)
	exec(`UPDATE users SET tournament_access = 'all' WHERE id = 2`)
	for _, bad := range []string{"some", "", "NONE"} {
		if _, err := db.Exec(`UPDATE users SET tournament_access = ? WHERE id = 2`, bad); err == nil {
			t.Errorf("tournament_access %q was accepted", bad)
		}
	}
	exec(`INSERT INTO users (username, role) VALUES ('new', 'captain')`)
	var fresh string
	if err := db.Get(&fresh, `SELECT tournament_access FROM users WHERE username = 'new'`); err != nil || fresh != "none" {
		t.Errorf("a new account's default: %q %v", fresh, err)
	}
	exec(`DELETE FROM users WHERE username = 'new'`)

	migrateTo(t, db, 35)
	var cols int
	if err := db.Get(&cols, `SELECT COUNT(*) FROM pragma_table_info('users') WHERE name = 'tournament_access'`); err != nil || cols != 0 {
		t.Errorf("tournament_access still there after down: %d %v", cols, err)
	}
	if related() != relatedBefore {
		t.Error("sessions or passkeys changed by the down migration")
	}
	migrateTo(t, db, 36)
}
