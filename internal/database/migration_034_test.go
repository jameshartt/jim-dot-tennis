// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package database

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
)

// migrateTo moves db to exactly version, through the same golang-migrate
// driver (and so the same per-file transaction) that ExecuteMigrations uses
// at startup.
func migrateTo(t *testing.T, db *DB, version uint) {
	t.Helper()
	m, err := db.NewMigrate(os.DirFS(filepath.Join("..", "..", "migrations")))
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Migrate(version); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("migrate to %d: %v", version, err)
	}
	st, err := db.ReadSchemaStatus(t.Context())
	if err != nil || st.Version != version || st.Dirty {
		t.Fatalf("after migrating to %d: %+v, %v", version, st, err)
	}
}

// dump returns every row of query as text, for before/after comparison.
func dump(t *testing.T, db *DB, query string) []string {
	t.Helper()
	rows, err := db.Queryx(query)
	if err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		vals, err := rows.SliceScan()
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, fmt.Sprintf("%q", vals))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestMigration034KeepsEverythingThatReferencesUsers is the safety proof for
// making users.password_hash nullable on a live database.
//
// Foreign keys are enforced on every connection and golang-migrate runs each
// file in a transaction, where they cannot be switched off. A table rebuild
// that drops users would therefore delete, by ON DELETE CASCADE, every
// session, passkey, passkey user handle and captain note. Migration 034
// replaces the column in place instead. This test migrates a populated
// database 33 -> 34 -> 33 -> 34 through the real migration driver and checks
// that every referencing row, every other users column, the constraints and
// the indexes are exactly as they were.
func TestMigration034KeepsEverythingThatReferencesUsers(t *testing.T) {
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
			t.Errorf("%s: statement succeeded, want a constraint error", what)
		}
	}

	migrateTo(t, db, 33)
	// The property under test only matters because foreign keys are on.
	if count(`PRAGMA foreign_keys`) != 1 {
		t.Fatal("foreign keys are not enforced on this connection; the test would prove nothing")
	}

	// A populated database as migration 033 leaves it.
	exec(`INSERT INTO clubs (id, name) VALUES (1, 'Club')`)
	exec(`INSERT INTO players (id, first_name, last_name, club_id) VALUES ('p-ann', 'Ann', 'Admin', 1), ('p-cap', 'Cap', 'Tain', 1)`)
	exec(`INSERT INTO users (id, username, password_hash, role, player_id, is_active, created_at, last_login_at, must_change_password) VALUES
		(1, 'ann',  '$2a$12$annhash',  'admin',   'p-ann', TRUE,  '2026-01-02 03:04:05', '2026-02-03 04:05:06', FALSE),
		(2, 'cap',  '$2a$12$caphash',  'captain', 'p-cap', TRUE,  '2026-01-03 03:04:05', '2026-02-04 04:05:06', FALSE),
		(5, 'temp', '$2a$12$temphash', 'captain', NULL,    TRUE,  '2026-01-04 03:04:05', '2026-01-04 03:04:05', TRUE),
		(9, 'gone', '$2a$12$gonehash', 'player',  NULL,    FALSE, '2026-01-05 03:04:05', '2026-01-05 03:04:05', FALSE)`)
	for i, userID := range []int{1, 1, 2, 5, 9} {
		exec(`INSERT INTO sessions (id, user_id, role, created_at, expires_at, last_activity_at, ip, user_agent, device_info, is_valid)
			VALUES (?, ?, 'admin', '2026-03-01 00:00:00', '2026-03-08 00:00:00', '2026-03-01 00:00:00', '203.0.113.1', 'ua', 'ua', TRUE)`,
			fmt.Sprintf("session-%d", i), userID)
	}
	exec(`INSERT INTO webauthn_user_handles (user_id, handle) VALUES (1, x'0101'), (2, x'0202'), (5, x'0505')`)
	exec(`INSERT INTO webauthn_credentials (user_id, credential_id, public_key, name) VALUES
		(1, x'a1', x'b1', 'Ann phone'), (1, x'a2', x'b2', 'Ann laptop'), (2, x'a3', x'b3', 'Cap phone'), (5, x'a4', x'b4', 'Temp key')`)
	exec(`INSERT INTO captain_player_notes (player_id, author_user_id, kind, body) VALUES
		('p-cap', 1, 'general', 'note by ann'), ('p-ann', 2, 'partnership', 'note by cap')`)

	// Everything that points at users, and users itself without the column
	// being changed.
	referencing := map[string]string{
		"sessions":              `SELECT * FROM sessions ORDER BY id`,
		"webauthn_user_handles": `SELECT * FROM webauthn_user_handles ORDER BY user_id`,
		"webauthn_credentials":  `SELECT * FROM webauthn_credentials ORDER BY id`,
		"captain_player_notes":  `SELECT * FROM captain_player_notes ORDER BY id`,
		"users (other columns)": `SELECT id, username, role, player_id, is_active, created_at, last_login_at FROM users ORDER BY id`,
		"users sequence":        `SELECT seq FROM sqlite_sequence WHERE name = 'users'`,
		"indexes on users":      `SELECT name FROM sqlite_master WHERE type = 'index' AND tbl_name = 'users' ORDER BY name`,
	}
	// The test must notice a table that starts referencing users later.
	for _, row := range dump(t, db, `SELECT name FROM sqlite_master WHERE type = 'table' AND sql LIKE '%REFERENCES users%' ORDER BY name`) {
		name := strings.Trim(row, `["]`)
		if _, ok := referencing[name]; !ok {
			t.Fatalf("table %s references users but is not covered by this test: add it", name)
		}
	}
	before := map[string][]string{}
	for name, query := range referencing {
		before[name] = dump(t, db, query)
		if len(before[name]) == 0 {
			t.Fatalf("%s: nothing to compare", name)
		}
	}
	unchanged := func(stage string) {
		t.Helper()
		for name, query := range referencing {
			after := dump(t, db, query)
			if strings.Join(after, "\n") != strings.Join(before[name], "\n") {
				t.Errorf("%s: %s changed\nbefore: %v\nafter:  %v", stage, name, before[name], after)
			}
		}
		if rows := dump(t, db, `PRAGMA foreign_key_check`); len(rows) != 0 {
			t.Errorf("%s: foreign key violations: %v", stage, rows)
		}
		if rows := dump(t, db, `PRAGMA integrity_check`); len(rows) != 1 || !strings.Contains(rows[0], "ok") {
			t.Errorf("%s: integrity check: %v", stage, rows)
		}
		// The constraints on users still hold.
		fails(stage+": duplicate username", `INSERT INTO users (username, password_hash, role) VALUES ('ann', 'x', 'admin')`)
		fails(stage+": duplicate player link", `INSERT INTO users (username, password_hash, role, player_id) VALUES ('other', 'x', 'admin', 'p-ann')`)
		fails(stage+": role check", `INSERT INTO users (username, password_hash, role) VALUES ('other', 'x', 'owner')`)
		fails(stage+": session for a missing user", `INSERT INTO sessions (id, user_id, role, expires_at, ip, user_agent, device_info) VALUES ('s', 404, 'admin', '2027-01-01', '', '', '')`)
	}
	hashes := func() string {
		t.Helper()
		var rows []string
		if err := db.Select(&rows, `SELECT id || '=' || COALESCE(password_hash, 'NULL') || '/' || must_change_password FROM users ORDER BY id`); err != nil {
			t.Fatal(err)
		}
		return strings.Join(rows, " ")
	}

	// Up: 33 -> 34.
	migrateTo(t, db, 34)
	unchanged("after up")
	// Passwords are kept, except the one somebody else chose (must_change),
	// which is removed; the mark is cleared.
	if got, want := hashes(), "1=$2a$12$annhash/0 2=$2a$12$caphash/0 5=NULL/0 9=$2a$12$gonehash/0"; got != want {
		t.Errorf("password hashes after up:\n got %s\nwant %s", got, want)
	}
	// The column is nullable now, and new ids continue after the old ones.
	exec(`INSERT INTO users (username, password_hash, role) VALUES ('invited', NULL, 'captain')`)
	if id := count(`SELECT id FROM users WHERE username = 'invited'`); id != 10 {
		t.Errorf("new user got id %d, want 10 (AUTOINCREMENT sequence kept)", id)
	}
	// account_links exists and goes with its user, like the other tables.
	exec(`INSERT INTO account_links (user_id, kind, token_hash, expires_at) VALUES (10, 'invite', 'hash-1', '2027-01-01 00:00:00')`)
	fails("link kind check", `INSERT INTO account_links (user_id, kind, token_hash, expires_at) VALUES (10, 'other', 'hash-2', '2027-01-01')`)
	fails("link hash unique", `INSERT INTO account_links (user_id, kind, token_hash, expires_at) VALUES (1, 'invite', 'hash-1', '2027-01-01')`)
	exec(`DELETE FROM users WHERE id = 10`)
	if n := count(`SELECT COUNT(*) FROM account_links`); n != 0 {
		t.Errorf("%d links left after their user was deleted", n)
	}
	exec(`DELETE FROM sqlite_sequence WHERE name = 'users'`)
	exec(`INSERT INTO sqlite_sequence (name, seq) VALUES ('users', 9)`)

	// Down: 34 -> 33. A missing password becomes the empty string, which no
	// password matches; the column is NOT NULL again.
	exec(`INSERT INTO account_links (user_id, kind, token_hash, expires_at) VALUES (1, 'recovery', 'hash-3', '2027-01-01 00:00:00')`)
	migrateTo(t, db, 33)
	unchanged("after down")
	if got, want := hashes(), "1=$2a$12$annhash/0 2=$2a$12$caphash/0 5=/0 9=$2a$12$gonehash/0"; got != want {
		t.Errorf("password hashes after down:\n got %s\nwant %s", got, want)
	}
	fails("after down: NULL password", `INSERT INTO users (username, password_hash, role) VALUES ('nullpw', NULL, 'captain')`)
	if n := count(`SELECT COUNT(*) FROM sqlite_master WHERE name IN ('account_links', 'idx_account_links_user')`); n != 0 {
		t.Errorf("account_links objects left after down: %d", n)
	}

	// Up again: the empty string becomes "no password".
	migrateTo(t, db, 34)
	unchanged("after up again")
	if n := count(`SELECT COUNT(*) FROM users WHERE id = 5 AND password_hash IS NULL`); n != 1 {
		t.Error("after up again: the empty password hash did not become NULL")
	}
	if n := count(`SELECT COUNT(*) FROM account_links`); n != 0 {
		t.Errorf("after up again: %d links, want a fresh empty table", n)
	}
}
