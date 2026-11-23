// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	jimdottennis "jim-dot-tennis"
	"jim-dot-tennis/internal/config"
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

	migrateOrStop(db, jimdottennis.Migrations(), "embedded", fatalf)
	if len(stopped) != 0 {
		t.Fatalf("good migrations stopped startup: %v", stopped)
	}
	missing := filepath.Join(t.TempDir(), "no-such-migrations")
	migrateOrStop(db, os.DirFS(missing), missing, fatalf)
	if len(stopped) != 1 || !strings.Contains(stopped[0], "cannot start") || !strings.Contains(stopped[0], missing) {
		t.Fatalf("a failed migration did not stop startup with a clear message: %v", stopped)
	}
}

// Starting an older version on a database a newer one upgraded stops with
// the operator's explanation, not golang-migrate's "no migration found".
func TestOlderVersionOnNewerDatabaseExplains(t *testing.T) {
	db, err := database.New(database.Config{Driver: database.DriverSQLite, FilePath: filepath.Join(t.TempDir(), "t.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	fatalf := func(format string, args ...any) { t.Fatalf(format, args...) }
	migrateOrStop(db, jimdottennis.Migrations(), "embedded", fatalf)

	// An "older build": every migration but the newest.
	latest, err := database.LatestMigration(jimdottennis.Migrations())
	if err != nil {
		t.Fatal(err)
	}
	older := fstest.MapFS{}
	entries, _ := fs.ReadDir(jimdottennis.Migrations(), ".")
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), fmt.Sprintf("%03d_", latest)) {
			continue
		}
		data, err := fs.ReadFile(jimdottennis.Migrations(), e.Name())
		if err != nil {
			t.Fatal(err)
		}
		older[e.Name()] = &fstest.MapFile{Data: data}
	}

	var stopped []string
	migrateOrStop(db, older, "older", func(format string, args ...any) { stopped = append(stopped, fmt.Sprintf(format, args...)) })
	if len(stopped) != 1 {
		t.Fatalf("want one stop, got %v", stopped)
	}
	msg := stopped[0]
	for _, want := range []string{"Cannot start", "last used by a newer version", "docs/upgrade.md"} {
		if !strings.Contains(msg, want) {
			t.Errorf("stop message %q missing %q", msg, want)
		}
	}
	if strings.Contains(msg, "CLAUDE.md") || strings.Contains(msg, "no migration found") {
		t.Errorf("stop message %q still has the old wording", msg)
	}
}

// The start-up line names the address to open when there is one, and says
// the localhost address is internal.
func TestWhereToOpen(t *testing.T) {
	t.Setenv(config.EnvPublicURL, "https://tennis.example.org")
	got := whereToOpen(config.NewClubSettings(context.Background(), nil), ":8080")
	for _, want := range []string{"Open https://tennis.example.org in a browser", "inside its container", "not the address to open"} {
		if !strings.Contains(got, want) {
			t.Errorf("whereToOpen = %q, missing %q", got, want)
		}
	}
	t.Setenv(config.EnvPublicURL, "")
	got = whereToOpen(config.NewClubSettings(context.Background(), nil), ":8080")
	if strings.Contains(got, "Open ") || !strings.Contains(got, "No public address is set yet") || !strings.Contains(got, "http://localhost:8080") {
		t.Errorf("whereToOpen without a public URL = %q", got)
	}
}
