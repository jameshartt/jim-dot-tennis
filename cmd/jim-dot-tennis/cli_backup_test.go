// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	jimdottennis "jim-dot-tennis"
	"jim-dot-tennis/internal/backup"
	"jim-dot-tennis/internal/database"
)

var backupPathRE = regexp.MustCompile(`(?m)^    (/\S+\.tar\.gz)$`)

// `backup` then `restore` from the command line: restore refuses while the
// server's lock is held and works once it is released.
func TestCLIBackupAndRestore(t *testing.T) {
	db := cliTestDB(t)
	t.Setenv(backup.EnvDir, "")
	t.Setenv(backup.EnvKeep, "")
	t.Setenv(backup.EnvInterval, "")
	if _, err := db.Exec(`INSERT INTO clubs (name) VALUES ('Round Trip Club')`); err != nil {
		t.Fatal(err)
	}

	r := runCLITest(t, "backup")
	if r.code != exitOK {
		t.Fatalf("backup: %+v", r)
	}
	m := backupPathRE.FindStringSubmatch(r.stdout)
	if m == nil || !strings.Contains(r.stdout, "keep it private") {
		t.Fatalf("backup output: %s", r.stdout)
	}
	archive := m[1]
	if _, err := db.Exec(`DELETE FROM clubs`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	cfg, err := backup.FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	lock, err := backup.AcquireLock(cfg.LockPath()) // the server
	if err != nil {
		t.Fatal(err)
	}
	r = runCLITest(t, "restore", archive)
	if r.code != exitError || !strings.Contains(r.stderr, "the app is running") || !strings.Contains(r.stderr, "Nothing was changed") {
		t.Fatalf("restore while the server runs: %+v", r)
	}
	lock.Release()

	r = runCLITest(t, "restore", archive)
	if r.code != exitOK || !strings.Contains(r.stdout, "Restored the backup") || !strings.Contains(r.stdout, "sign in again") {
		t.Fatalf("restore: %+v", r)
	}
	after, err := database.New(database.Config{Driver: database.DriverSQLite, FilePath: cfg.DBPath})
	if err != nil {
		t.Fatal(err)
	}
	defer after.Close()
	var n int
	after.Get(&n, `SELECT COUNT(*) FROM clubs WHERE name = 'Round Trip Club'`)
	if n != 1 {
		t.Error("the restored database does not have the backed-up club")
	}

	if r := runCLITest(t, "restore"); r.code != exitUsage {
		t.Errorf("restore with no file: %+v", r)
	}
	if r := runCLITest(t, "restore", filepath.Join(t.TempDir(), "missing.tar.gz")); r.code != exitError || !strings.Contains(r.stderr, "not changed") {
		t.Errorf("restore of a missing file: %+v", r)
	}
}

// migrationsDirUpTo copies the built-in migrations numbered at most n into a
// folder.
func migrationsDirUpTo(t *testing.T, n int) fs.FS {
	t.Helper()
	dir := t.TempDir()
	src := jimdottennis.Migrations()
	entries, _ := fs.ReadDir(src, ".")
	for _, e := range entries {
		num, err := strconv.Atoi(strings.SplitN(e.Name(), "_", 2)[0])
		if err != nil || num > n {
			continue
		}
		data, _ := fs.ReadFile(src, e.Name())
		os.WriteFile(filepath.Join(dir, e.Name()), data, 0o600)
	}
	return os.DirFS(dir)
}

// Before migrations change an existing database a backup is taken; if it
// cannot be, the migrations do not run and startup stops.
func TestPreMigrationBackupGatesMigrations(t *testing.T) {
	for _, possible := range []bool{false, true} {
		t.Run(fmt.Sprintf("backup possible=%v", possible), func(t *testing.T) {
			dir := t.TempDir()
			db, err := database.New(database.Config{Driver: database.DriverSQLite, FilePath: filepath.Join(dir, "tennis.db")})
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if err := db.ExecuteMigrationsFS(migrationsDirUpTo(t, 33), "old"); err != nil {
				t.Fatal(err)
			}
			cfg := backup.Config{DBPath: filepath.Join(dir, "tennis.db"), Dir: filepath.Join(dir, "backups"), Keep: 3}
			if !possible {
				os.WriteFile(filepath.Join(dir, "file"), nil, 0o600)
				cfg.Dir = filepath.Join(dir, "file", "backups")
			}
			var stopped []string
			fatalf := func(format string, args ...any) { stopped = append(stopped, fmt.Sprintf(format, args...)) }
			backupThenMigrate(db, cfg, jimdottennis.Migrations(), "embedded", fatalf)

			st, _ := db.ReadSchemaStatus(context.Background())
			latest, _ := database.LatestMigration(jimdottennis.Migrations())
			list, _ := backup.List(cfg.Dir)
			if possible {
				if len(stopped) != 0 || st.Version != latest || len(list) != 1 || !list[0].PreMigration {
					t.Errorf("stopped %v, version %d, backups %+v", stopped, st.Version, list)
				}
				return
			}
			if len(stopped) != 1 || !strings.Contains(stopped[0], "nothing was changed") || st.Version != 33 {
				t.Errorf("stopped %v, version %d (want the migrations not run)", stopped, st.Version)
			}
		})
	}
}
