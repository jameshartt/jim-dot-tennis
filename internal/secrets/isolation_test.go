// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package secrets

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	migratesqlite "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"jim-dot-tennis/internal/database"
)

// tableCounts returns the row count of every table in the database except
// golang-migrate's bookkeeping.
func tableCounts(t *testing.T, db *database.DB) map[string]int {
	t.Helper()
	var tables []string
	if err := db.Select(&tables, `SELECT name FROM sqlite_master WHERE type = 'table'
		AND name NOT LIKE 'sqlite_%' AND name != 'schema_migrations' ORDER BY name`); err != nil {
		t.Fatal(err)
	}
	if len(tables) < 10 {
		t.Fatalf("only %d tables after migrating; expected the full schema", len(tables))
	}
	counts := map[string]int{}
	for _, name := range tables {
		var n int
		if err := db.Get(&n, fmt.Sprintf(`SELECT COUNT(*) FROM %q`, name)); err != nil {
			t.Fatal(err)
		}
		counts[name] = n
	}
	return counts
}

// A database created from scratch holds no rows at all: no accounts,
// sessions, tokens, keys, settings or seeded data of any kind. After the
// startup self-check, the only row is this instance's own generated Club
// Wrapped code.
func TestFreshDatabaseHasNoAccountsOrSecrets(t *testing.T) {
	db := newTestDB(t)
	for table, n := range tableCounts(t, db) {
		if n != 0 {
			t.Errorf("fresh database: table %s has %d row(s), want 0", table, n)
		}
	}

	t.Setenv("APP_ENV", "")
	t.Setenv(EnvWrappedAccess, "")
	if err := StartupCheck(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	for table, n := range tableCounts(t, db) {
		want := 0
		if table == "app_settings" {
			want = 1
		}
		if n != want {
			t.Errorf("after startup check: table %s has %d row(s), want %d", table, n, want)
		}
	}
	var key string
	if err := db.Get(&key, `SELECT key FROM app_settings`); err != nil || key != SettingWrappedAccessCode {
		t.Fatalf("unexpected app_settings row %q (err %v)", key, err)
	}
}

type accountRow struct {
	ID           int64  `db:"id"`
	Username     string `db:"username"`
	PasswordHash string `db:"password_hash"`
	Role         string `db:"role"`
	IsActive     bool   `db:"is_active"`
	CreatedAt    string `db:"created_at"`
	LastLoginAt  string `db:"last_login_at"`
}

func accounts(t *testing.T, db *database.DB) ([]accountRow, []map[string]any) {
	t.Helper()
	var users []accountRow
	if err := db.Select(&users, `SELECT id, username, password_hash, role, is_active,
		CAST(created_at AS TEXT) AS created_at, CAST(last_login_at AS TEXT) AS last_login_at FROM users ORDER BY id`); err != nil {
		t.Fatal(err)
	}
	rows, err := db.Queryx(`SELECT user_id, role, CAST(expires_at AS TEXT) AS expires_at, ip, is_valid FROM sessions ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var sessions []map[string]any
	for rows.Next() {
		m := map[string]any{}
		if err := rows.MapScan(m); err != nil {
			t.Fatal(err)
		}
		sessions = append(sessions, m)
	}
	return users, sessions
}

// An install created by an earlier version (here: stopped at migration 004,
// with accounts, a session and push keys) upgrades to the current schema and
// passes the startup self-check without any account changing. Its sessions
// are ended once, by migration 035 (sessions stored hashed from then on).
func TestUpgradeKeepsExistingAccounts(t *testing.T) {
	db, err := database.New(database.Config{Driver: "sqlite3", FilePath: filepath.Join(t.TempDir(), "old.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	migrations := repoDir(t, "migrations")
	driver, err := migratesqlite.WithInstance(db.DB.DB, &migratesqlite.Config{})
	if err != nil {
		t.Fatal(err)
	}
	m, err := migrate.NewWithDatabaseInstance("file://"+migrations, "sqlite3", driver)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Migrate(4); err != nil {
		t.Fatal(err)
	}

	db.MustExec(`INSERT INTO users (id, username, password_hash, role, is_active, created_at, last_login_at) VALUES
		(1, 'club-admin', '$2a$10$abcdefghijklmnopqrstuuG6z3i3yQ1z8yGQ9bD0eV9m0Xb7vH5W.', 'admin', TRUE, '2025-03-01 10:00:00', '2025-06-01 09:00:00'),
		(2, 'a-captain',  '$2a$10$zyxwvutsrqponmlkjihgfeG6z3i3yQ1z8yGQ9bD0eV9m0Xb7vH5W.', 'captain', TRUE, '2025-03-02 10:00:00', '2025-06-02 09:00:00'),
		(3, 'retired',    '$2a$10$0123456789abcdefghijkuG6z3i3yQ1z8yGQ9bD0eV9m0Xb7vH5W.', 'player', FALSE, '2025-03-03 10:00:00', '2025-03-03 10:00:00')`)
	db.MustExec(`INSERT INTO sessions (id, user_id, role, expires_at, ip, user_agent, device_info, is_valid)
		VALUES ('sess-1', 1, 'admin', '2099-01-01 00:00:00', '192.0.2.1', 'ua', 'dev', TRUE)`)
	db.MustExec(`INSERT INTO vapid_keys (id, public_key, private_key) VALUES (1, 'pub', 'priv')`)
	usersBefore, sessionsBefore := accounts(t, db)
	if len(sessionsBefore) != 1 {
		t.Fatalf("sessions before upgrade: %v", sessionsBefore)
	}

	if err := db.ExecuteMigrations(migrations); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APP_ENV", "production")
	t.Setenv(EnvWrappedAccess, "")
	if err := StartupCheck(context.Background(), db); err != nil {
		t.Fatal(err)
	}

	usersAfter, sessionsAfter := accounts(t, db)
	if !reflect.DeepEqual(usersBefore, usersAfter) {
		t.Fatalf("accounts changed by upgrade:\nbefore %+v\nafter  %+v", usersBefore, usersAfter)
	}
	// Migration 035 signs everyone out once: sessions used to be stored by
	// their raw token, and are deleted rather than converted (see the
	// migration). Accounts are untouched.
	if len(sessionsAfter) != 0 {
		t.Fatalf("sessions after upgrade: %v, want none (migration 035 signs everyone out once)", sessionsAfter)
	}
	var pub, priv string
	if err := db.QueryRow(`SELECT public_key, private_key FROM vapid_keys WHERE id = 1`).Scan(&pub, &priv); err != nil || pub != "pub" || priv != "priv" {
		t.Fatalf("VAPID keys changed by upgrade (err %v)", err)
	}
}
