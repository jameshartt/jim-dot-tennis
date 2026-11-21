// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package database

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeMigration(t *testing.T, dir, name, sql string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(sql), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A migration that fails names itself and the recovery steps; the next start
// finds the database dirty and refuses, naming the same migration, even with
// the old MIGRATE_ALLOW_DIRTY_FORCE set (that switch no longer exists).
func TestFailedAndDirtyMigrationNameTheMigration(t *testing.T) {
	dir := t.TempDir()
	writeMigration(t, dir, "001_ok.up.sql", "CREATE TABLE a (id INTEGER);")
	writeMigration(t, dir, "001_ok.down.sql", "DROP TABLE a;")
	writeMigration(t, dir, "002_bad.up.sql", "CREATE TABLE b (id INTEGER);\nTHIS IS NOT SQL;")
	writeMigration(t, dir, "002_bad.down.sql", "DROP TABLE b;")

	db, err := New(Config{Driver: DriverSQLite, FilePath: filepath.Join(t.TempDir(), "t.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	err = db.ExecuteMigrations(dir)
	var me *MigrationError
	if !errors.As(err, &me) || me.Version != 2 || me.Dirty {
		t.Fatalf("first run: want a MigrationError for 002, got %v", err)
	}
	for _, want := range []string{"migration 002 failed", "restore the database file", "stop the app"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message %q missing %q", err.Error(), want)
		}
	}

	t.Setenv("MIGRATE_ALLOW_DIRTY_FORCE", "true")
	err = db.ExecuteMigrations(dir)
	if !errors.As(err, &me) || me.Version != 2 || !me.Dirty {
		t.Fatalf("second run: want a dirty MigrationError for 002, got %v", err)
	}
	if st, _ := db.ReadSchemaStatus(context.Background()); !st.Dirty {
		t.Error("the dirty flag was cleared; the version must never be forced")
	}
}
