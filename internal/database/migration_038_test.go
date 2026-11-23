// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package database

import (
	"path/filepath"
	"strings"
	"testing"
)

// col returns the single text column of each row.
func col(t *testing.T, db *DB, query string) []string {
	t.Helper()
	var out []string
	if err := db.Select(&out, query); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return out
}

// TestMigration038TournamentProviderChoice migrates a database whose club is
// already set up on the tournament server 37 -> 38 -> 37 -> 38. Migration 038
// gives tournament_providers the tournament server's id, copies the club's
// own (stored in app_settings by set-up) onto its row whatever the row's
// case, leaves other rows unmanaged, and adds the per-person picks.
func TestMigration038TournamentProviderChoice(t *testing.T) {
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
	rows := func() string {
		t.Helper()
		return strings.Join(col(t, db, `SELECT id || '|' || name || '|' || provider_abbr || '|' || courthive_provider_id FROM tournament_providers ORDER BY id`), ";")
	}

	migrateTo(t, db, 37)
	exec(`INSERT INTO users (id, username, password_hash, role, is_active, tournament_access) VALUES (5, 'tina', NULL, 'player', TRUE, 'own')`)
	exec(`INSERT INTO tournament_providers (id, name, provider_abbr) VALUES (1, 'St Ann''s', 'stanns '), (2, 'Parks Cup', 'BHPLTA')`)
	exec(`INSERT INTO tournaments (name, courthive_tournament_id, provider_id) VALUES ('Club champs', 't1', 1)`)
	exec(`INSERT INTO app_settings (key, value) VALUES ('tournament_provider_id', 'uuid-own'), ('tournament_provider_abbr', 'STANNS')`)

	migrateTo(t, db, 38)
	if got := rows(); got != "1|St Ann's|stanns |uuid-own;2|Parks Cup|BHPLTA|" {
		t.Fatalf("after 038: %s", got)
	}
	exec(`INSERT INTO user_tournament_providers (user_id, tournament_provider_id) VALUES (5, 1)`)
	if _, err := db.Exec(`INSERT INTO user_tournament_providers (user_id, tournament_provider_id) VALUES (5, 1)`); err == nil {
		t.Error("the same pick was stored twice")
	}
	if _, err := db.Exec(`INSERT INTO user_tournament_providers (user_id, tournament_provider_id) VALUES (5, 99)`); err == nil {
		t.Error("a pick of a provider that does not exist was stored")
	}
	// Removing the account removes its picks.
	exec(`DELETE FROM users WHERE id = 5`)
	if n := col(t, db, `SELECT COUNT(*) FROM user_tournament_providers`); n[0] != "0" {
		t.Errorf("picks left after the account went: %v", n)
	}

	// Down and up again: the rows and the tournament survive, and the id is
	// copied again.
	migrateTo(t, db, 37)
	if got := strings.Join(col(t, db, `SELECT id || '|' || name || '|' || provider_abbr FROM tournament_providers ORDER BY id`), ";"); got != "1|St Ann's|stanns ;2|Parks Cup|BHPLTA" {
		t.Fatalf("after going back to 37: %s", got)
	}
	migrateTo(t, db, 38)
	if got := rows(); got != "1|St Ann's|stanns |uuid-own;2|Parks Cup|BHPLTA|" {
		t.Fatalf("after 038 again: %s", got)
	}
	if n := col(t, db, `SELECT COUNT(*) FROM tournaments WHERE provider_id = 1`); n[0] != "1" {
		t.Errorf("the tournament was lost: %v", n)
	}
}

// A club that is not set up yet: nothing is managed.
func TestMigration038WithoutSetUp(t *testing.T) {
	db, err := New(Config{Driver: DriverSQLite, FilePath: filepath.Join(t.TempDir(), "test.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	migrateTo(t, db, 37)
	if _, err := db.Exec(`INSERT INTO tournament_providers (id, name, provider_abbr) VALUES (1, 'By hand', 'HAND')`); err != nil {
		t.Fatal(err)
	}
	// An abbreviation from a set-up that never finished is not an id.
	if _, err := db.Exec(`INSERT INTO app_settings (key, value) VALUES ('tournament_provider_abbr', 'HAND')`); err != nil {
		t.Fatal(err)
	}
	migrateTo(t, db, 38)
	if got := col(t, db, `SELECT courthive_provider_id FROM tournament_providers`); len(got) != 1 || got[0] != "" {
		t.Fatalf("a provider became managed without an id: %q", got)
	}
}
