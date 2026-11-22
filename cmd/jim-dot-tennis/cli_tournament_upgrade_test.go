// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jim-dot-tennis/internal/backup"
	"jim-dot-tennis/internal/repository"
)

// fakePostgresTools puts stand-ins for pg_dump, pg_restore and psql first in
// PATH. The "database" has TABLES tables; pg_dump fails when FAKE_DUMP_FAILS is set.
func fakePostgresTools(t *testing.T) {
	t.Helper()
	bin := t.TempDir()
	tools := map[string]string{
		"pg_dump": `for a; do case $a in --file=*) printf 'PGDMP fake' > "${a#--file=}";; esac; done
[ -n "$FAKE_DUMP_FAILS" ] && { echo 'pg_dump: error: connection to server at "postgres" (172.18.0.2), port 5432 failed: Connection refused' >&2; exit 1; }
exit 0`,
		"pg_restore": `case "$*" in *--list*) printf ';     Dumped from database version: 18.4\n1; 1259 1 TABLE public tournaments courthive\n';; esac
exit 0`,
		"psql": `[ -n "$FAKE_DB_DOWN" ] && { echo 'psql: error: connection to server at "postgres" failed: Connection refused' >&2; exit 2; }
case "$*" in *pg_tables*) echo "${TABLES:-0}";; *to_regclass*) echo t;; *schema_migrations*) echo '64|064-support-cases.sql';; *pg_stat_activity*) echo 0;; esac
exit 0`,
	}
	for name, body := range tools {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv(backup.EnvPGHost, "postgres")
	t.Setenv(backup.EnvPGPort, "5432")
	t.Setenv(backup.EnvPGUser, "courthive")
	t.Setenv(backup.EnvPGDatabase, "courthive")
	t.Setenv(backup.EnvPGPassword, "a-long-test-password-not-real")
	t.Setenv(backup.EnvDir, "")
	t.Setenv(backup.EnvKeep, "")
	t.Setenv(backup.EnvInterval, "")
	t.Setenv("FAKE_DUMP_FAILS", "")
	t.Setenv("FAKE_DB_DOWN", "")
}

func preUpgradeArchives(t *testing.T) []backup.Archive {
	t.Helper()
	cfg, err := backup.FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	list, _ := backup.List(cfg.Dir)
	var out []backup.Archive
	for _, a := range list {
		if a.PreUpgrade {
			out = append(out, a)
		}
	}
	return out
}

// tournaments pre-upgrade: a new empty database only records the image; the
// same image again does nothing; a new image makes a backup with the
// tournament data first; a failure exits non-zero and records nothing.
func TestTournamentsPreUpgrade(t *testing.T) {
	db := cliTestDB(t)
	fakePostgresTools(t)
	const v1, v2 = "ghcr.io/jameshartt/courthive-server:4.19.1-jdt.1", "ghcr.io/jameshartt/courthive-server:4.20.0-jdt.1"
	recorded := func() string {
		v, _, _ := repository.NewSettingsRepository(db).GetString(t.Context(), repository.SettingTournamentServerImage)
		return v
	}

	t.Setenv(backup.EnvServerImage, v1)
	t.Setenv("TABLES", "0")
	r := runCLITest(t, "tournaments", "pre-upgrade")
	if r.code != exitOK || !strings.Contains(r.stdout, "new and empty") || recorded() != v1 || len(preUpgradeArchives(t)) != 0 {
		t.Fatalf("new database: %+v", r)
	}

	t.Setenv("TABLES", "40")
	r = runCLITest(t, "tournaments", "pre-upgrade")
	if r.code != exitOK || !strings.Contains(r.stdout, "unchanged") || len(preUpgradeArchives(t)) != 0 {
		t.Fatalf("ordinary restart: %+v", r)
	}

	// A new image, with the tournament database down: stop, record nothing.
	t.Setenv(backup.EnvServerImage, v2)
	t.Setenv("FAKE_DB_DOWN", "1")
	r = runCLITest(t, "tournaments", "pre-upgrade")
	if r.code != exitError || !strings.Contains(r.stderr, "Stopped before the tournament server started") ||
		!strings.Contains(r.stderr, "cannot be reached") || recorded() != v1 || len(preUpgradeArchives(t)) != 0 {
		t.Fatalf("database down: %+v", r)
	}
	t.Setenv("FAKE_DB_DOWN", "")
	t.Setenv("FAKE_DUMP_FAILS", "1")
	r = runCLITest(t, "tournaments", "pre-upgrade")
	if r.code != exitError || !strings.Contains(r.stderr, "the backup could not be made") || recorded() != v1 || len(preUpgradeArchives(t)) != 0 {
		t.Fatalf("dump failing: %+v", r)
	}
	if strings.Contains(r.stderr, "a-long-test-password-not-real") {
		t.Error("the password is in the message")
	}

	t.Setenv("FAKE_DUMP_FAILS", "")
	r = runCLITest(t, "tournaments", "pre-upgrade")
	if r.code != exitOK || !strings.Contains(r.stdout, "changes from "+v1+" to "+v2) || recorded() != v2 {
		t.Fatalf("upgrade: %+v", r)
	}
	list := preUpgradeArchives(t)
	if len(list) != 1 {
		t.Fatalf("pre-upgrade archives: %+v", list)
	}
	m, err := backup.VerifyFile(list[0].Path)
	if err != nil || !m.HasTournaments() || m.Kind != backup.KindPreUpgrade || m.Tournaments.ServerImage != v2 {
		t.Errorf("archive: %+v %v", m, err)
	}

	r = runCLITest(t, "tournaments", "pre-upgrade")
	if r.code != exitOK || !strings.Contains(r.stdout, "unchanged") || len(preUpgradeArchives(t)) != 1 {
		t.Fatalf("restart after the upgrade: %+v", r)
	}

	// Without the tournament settings it refuses.
	t.Setenv(backup.EnvPGHost, "")
	t.Setenv(backup.EnvPGPort, "")
	t.Setenv(backup.EnvPGUser, "")
	t.Setenv(backup.EnvPGDatabase, "")
	t.Setenv(backup.EnvPGPassword, "")
	if r := runCLITest(t, "tournaments", "pre-upgrade"); r.code != exitError {
		t.Errorf("without settings: %+v", r)
	}
}

// backup says whether the tournament data is included; restore of such a
// backup on a deployment without the settings says it was not restored and
// how to restore it later.
func TestCLIBackupWithTournamentData(t *testing.T) {
	cliTestDB(t)
	fakePostgresTools(t)
	t.Setenv("TABLES", "40")
	r := runCLITest(t, "backup")
	if r.code != exitOK || !strings.Contains(r.stdout, "Includes tournament data") {
		t.Fatalf("backup: %+v", r)
	}
	archive := backupPathRE.FindStringSubmatch(r.stdout)[1]

	t.Setenv("FAKE_DUMP_FAILS", "1")
	r = runCLITest(t, "backup")
	if r.code != exitError || !strings.Contains(r.stderr, "no backup was made") || backupPathRE.MatchString(r.stdout) {
		t.Fatalf("backup with the dump failing: %+v", r)
	}
	t.Setenv("FAKE_DUMP_FAILS", "")

	t.Setenv(backup.EnvPGHost, "")
	t.Setenv(backup.EnvPGPort, "")
	t.Setenv(backup.EnvPGUser, "")
	t.Setenv(backup.EnvPGDatabase, "")
	t.Setenv(backup.EnvPGPassword, "")
	r = runCLITest(t, "restore", archive)
	if r.code != exitOK || !strings.Contains(r.stdout, "was NOT restored") || !strings.Contains(r.stdout, "restore --tournaments-only "+archive) {
		t.Fatalf("restore without the settings: %+v", r)
	}
	r = runCLITest(t, "backup")
	if r.code != exitOK || !strings.Contains(r.stdout, "Does not include tournament data") {
		t.Fatalf("backup without the settings: %+v", r)
	}
	if r := runCLITest(t, "restore", "--tournaments-only", archive); r.code != exitError || !strings.Contains(r.stderr, "no tournament database settings") {
		t.Errorf("tournaments-only without settings: %+v", r)
	}
	if r := runCLITest(t, "backup", "--tournaments-only"); r.code != exitUsage {
		t.Errorf("--tournaments-only on backup: %+v", r)
	}
}
