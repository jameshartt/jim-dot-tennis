// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"jim-dot-tennis/internal/database"
)

// WI-134 I5: a failed migration stops the server instead of being a warning.
func TestFailedMigrationStopsStartup(t *testing.T) {
	db, err := database.New(database.Config{Driver: database.DriverSQLite, FilePath: filepath.Join(t.TempDir(), "t.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var stopped []string
	fatalf := func(format string, args ...any) { stopped = append(stopped, fmt.Sprintf(format, args...)) }

	migrateOrStop(db, filepath.Join("..", "..", "migrations"), fatalf)
	if len(stopped) != 0 {
		t.Fatalf("good migrations stopped startup: %v", stopped)
	}
	missing := filepath.Join(t.TempDir(), "no-such-migrations")
	migrateOrStop(db, missing, fatalf)
	if len(stopped) != 1 || !strings.Contains(stopped[0], "cannot start") || !strings.Contains(stopped[0], missing) {
		t.Fatalf("a failed migration did not stop startup with a clear message: %v", stopped)
	}
}
