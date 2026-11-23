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
	for _, want := range []string{"migration 002 failed", "jim-dot-tennis restore", "pre-migration", "stop the app"} {
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

// An older version started on a database a newer one has upgraded says so,
// points to the rolling-back steps, and changes nothing.
func TestNewerDatabaseIsExplainedAndLeftAlone(t *testing.T) {
	newer := t.TempDir()
	writeMigration(t, newer, "001_ok.up.sql", "CREATE TABLE a (id INTEGER);")
	writeMigration(t, newer, "001_ok.down.sql", "DROP TABLE a;")
	writeMigration(t, newer, "002_more.up.sql", "CREATE TABLE b (id INTEGER);")
	writeMigration(t, newer, "002_more.down.sql", "DROP TABLE b;")
	older := t.TempDir()
	writeMigration(t, older, "001_ok.up.sql", "CREATE TABLE a (id INTEGER);")
	writeMigration(t, older, "001_ok.down.sql", "DROP TABLE a;")

	db, err := New(Config{Driver: DriverSQLite, FilePath: filepath.Join(t.TempDir(), "t.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.ExecuteMigrations(newer); err != nil {
		t.Fatal(err)
	}

	err = db.ExecuteMigrations(older)
	var ne *NewerSchemaError
	if !errors.As(err, &ne) || ne.Found != 2 || ne.Latest != 1 {
		t.Fatalf("want a NewerSchemaError (found 2, latest 1), got %v", err)
	}
	msg := err.Error()
	for _, want := range []string{"last used by a newer version", "start the newer version again", "your .env file",
		"restore the backup", "jim-dot-tennis-pre-migration-", "docs/upgrade.md", "Going back to the previous version (rolling back)", "Nothing was changed"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q missing %q", msg, want)
		}
	}
	for _, bad := range []string{"CLAUDE.md", "no migration found", "file does not exist"} {
		if strings.Contains(msg, bad) {
			t.Errorf("message %q should not contain %q", msg, bad)
		}
	}
	if st, _ := db.ReadSchemaStatus(context.Background()); st.Version != 2 || st.Dirty {
		t.Errorf("schema status changed to %+v; want version 2, clean", st)
	}
}
