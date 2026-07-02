// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package database

import (
	"path/filepath"
	"testing"
)

func TestNewSQLiteAppliesPragmas(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")

	db, err := New(Config{Driver: "sqlite3", FilePath: dbPath})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer db.Close()

	var journalMode string
	if err := db.Get(&journalMode, "PRAGMA journal_mode"); err != nil {
		t.Fatalf("querying journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("journal_mode = %q, want %q", journalMode, "wal")
	}

	var foreignKeys int
	if err := db.Get(&foreignKeys, "PRAGMA foreign_keys"); err != nil {
		t.Fatalf("querying foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Errorf("foreign_keys = %d, want 1", foreignKeys)
	}

	var busyTimeout int
	if err := db.Get(&busyTimeout, "PRAGMA busy_timeout"); err != nil {
		t.Fatalf("querying busy_timeout: %v", err)
	}
	if busyTimeout != 5000 {
		t.Errorf("busy_timeout = %d, want 5000", busyTimeout)
	}
}
