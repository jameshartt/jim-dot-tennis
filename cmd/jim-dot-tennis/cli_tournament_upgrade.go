// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"jim-dot-tennis/internal/backup"
	"jim-dot-tennis/internal/repository"
)

// runTournamentsPreUpgrade is `jim-dot-tennis tournaments pre-upgrade`. The
// deployment runs it as the one-shot service courthive-pre-upgrade before the
// tournament server starts (deploy/docker-compose.yml): the tournament server
// migrates its database when it starts, and some of its migrations cannot be
// undone, so a new server image must never start on data that has no fresh
// backup.
//
// It compares the server image the deployment is about to run
// (COURTHIVE_SERVER_IMAGE) with the one recorded in the app's database:
//
//   - the same: nothing to do (an ordinary restart);
//   - different, or none recorded, and the tournament database has tables:
//     a full backup (the app's data and a dump of the tournament database)
//     is made and checked, then the new image is recorded;
//   - the tournament database is new and empty: the image is recorded, as
//     there is nothing to keep.
//
// Any failure exits non-zero and records nothing, so the tournament server
// does not start (it waits for this service to complete successfully) and
// the next `docker compose up -d` tries again. The app and Caddy do not
// depend on it.
func runTournamentsPreUpgrade(ctx context.Context, c cliOut, stderr io.Writer) int {
	stop := func(format string, args ...any) int {
		fmt.Fprintf(stderr, "Stopped before the tournament server started: %s.\n", fmt.Sprintf(format, args...))
		fmt.Fprintf(stderr, "Nothing in the tournament database was changed, and the league site keeps working.\n"+
			"Fix the cause above, then run \"docker compose up -d\" again in the folder holding docker-compose.yml.\n"+
			"The tournament server starts once this step has made its backup. docs/troubleshooting.md\n"+
			"(\"service courthive-pre-upgrade didn't complete successfully\") says what else you can do.\n")
		return exitError
	}
	cfg, err := backup.FromEnv()
	if err != nil {
		return stop("%v", err)
	}
	if cfg.Tournament == nil {
		return stop("the tournament database settings (%s and the others) are empty, so there is nothing to back up. "+
			"This step only runs with the tournament stack on", backup.EnvPGHost)
	}
	image := cfg.ServerImage
	if image == "" {
		return stop("%s is empty: the deployment's compose file sets it to the tournament server image", backup.EnvServerImage)
	}
	db, err := openCLIDatabase(ctx)
	if err != nil {
		return stop("%v", err)
	}
	defer db.Close()
	settings := repository.NewSettingsRepository(db)
	prev, found, err := settings.GetString(ctx, repository.SettingTournamentServerImage)
	if err != nil {
		return stop("cannot read the app's database: %v", err)
	}
	if found && prev == image {
		c.printf("Tournament server image unchanged (%s): no backup needed.\n", image)
		return exitOK
	}
	st, err := cfg.Tournament.State(ctx)
	if err != nil {
		return stop("%s could not be opened (is it running, and is its password right?): %v", cfg.Tournament.Describe(), err)
	}
	if st.Tables == 0 {
		if err := settings.SetString(ctx, repository.SettingTournamentServerImage, image); err != nil {
			return stop("cannot write to the app's database: %v", err)
		}
		c.printf("The tournament database is new and empty: nothing to back up. Recorded the tournament server image %s.\n", image)
		return exitOK
	}
	if found {
		c.printf("The tournament server image changes from %s to %s: backing up first.\n", prev, image)
	} else {
		c.printf("The tournament server image %s has not been seen with this data before: backing up first.\n", image)
	}
	res, err := backup.Create(ctx, db, cfg, backup.KindPreUpgrade)
	if err != nil {
		return stop("the backup could not be made: %v", err)
	}
	if err := settings.SetString(ctx, repository.SettingTournamentServerImage, image); err != nil {
		return stop("the backup %s was made, but the app's database could not record it: %v", filepath.Base(res.Path), err)
	}
	t := res.Manifest.Tournaments
	c.printf("Backup made and checked, with the tournament data:\n\n    %s\n\n", res.Path)
	c.printf("%d bytes; tournament database at migration %s (%d applied). The tournament server may now start and upgrade it.\n",
		res.Size, orNone(t.LatestMigration), t.Migrations)
	return exitOK
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}

// tournamentLine describes an archive's tournament data, for backup's output.
func tournamentLine(m backup.Manifest) string {
	if !m.HasTournaments() {
		return "Does not include tournament data (this deployment has no tournament stack)."
	}
	t := m.Tournaments
	return fmt.Sprintf("Includes tournament data: a dump of the tournament database (%d bytes, Postgres %s, tournament server migration %s).",
		t.Size, orNone(t.ServerVersion), orNone(t.LatestMigration))
}

// printTournamentRestore says what a full restore did with the tournament
// database.
func printTournamentRestore(c cliOut, res backup.RestoreResult, archive string) {
	switch res.Tournament {
	case backup.TournamentRestored:
		c.printf("\nThe tournament database was restored from the backup too.\n")
		c.printf("The tournament database it replaced is kept as %s in the data folder.\n", filepath.Base(res.TournamentSafetyCopy))
		c.printf("Start the tournament server and identity service again after the app (docker compose up -d, in the folder holding docker-compose.yml).\n")
	case backup.TournamentNotInBackup:
		c.printf("\nThis backup has no tournament data, so the tournament database was left as it is.\n")
		c.printf("People's tournament accounts are matched up again when they next open tournament management.\n")
	case backup.TournamentNotConfigured:
		c.printf("\nThis backup also holds tournament data, which was NOT restored: this deployment has no tournament\n" +
			"database settings (the tournament stack is off). To restore it later, turn the tournament stack on,\n" +
			"stop the app, the tournament server and the identity service, and run:\n\n")
		c.printf("    jim-dot-tennis restore --tournaments-only %s\n", archive)
	}
}

// runRestoreTournaments is `jim-dot-tennis restore --tournaments-only <file>`.
func runRestoreTournaments(ctx context.Context, c cliOut, stderr io.Writer, file string) int {
	cfg, err := backup.FromEnv()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitError
	}
	ctx, stop := restoreContext(ctx)
	defer stop()
	c.printf("Checking %s and restoring its tournament data ...\n", file)
	res, err := backup.RestoreTournaments(ctx, cfg, file)
	if err != nil {
		msg := err.Error()
		if !strings.Contains(msg, "Nothing was changed") && !strings.Contains(msg, "before anything was changed") && !errors.Is(err, backup.ErrServerRunning) {
			msg += ". The tournament database was not changed"
		}
		fmt.Fprintf(stderr, "Not restored: %s.\nThe app's database was not touched.\n", msg)
		return exitError
	}
	if res.Manifest != nil {
		c.printf("\nRestored the tournament data of %s.\n", describeBackup(*res.Manifest))
	} else {
		c.printf("\nRestored the tournament database from the dump %s.\n", filepath.Base(file))
	}
	c.printf("The app's database was not touched. The tournament database it replaced is kept as %s in the data folder.\n",
		filepath.Base(res.SafetyCopy))
	c.printf("\nNow start the app, then the tournament server and identity service (docker compose up -d, in the folder holding docker-compose.yml).\n")
	return exitOK
}
