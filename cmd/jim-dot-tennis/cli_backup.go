// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	jimdottennis "jim-dot-tennis"
	"jim-dot-tennis/internal/backup"
	"jim-dot-tennis/internal/database"
)

// runBackup is `jim-dot-tennis backup`: a backup now, alongside the running
// server (the copy comes from SQLite's VACUUM INTO, never from the file).
// Unlike the account commands it works on a database at any schema version.
func runBackup(ctx context.Context, c cliOut, stderr io.Writer) int {
	cfg, err := backup.FromEnv()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitError
	}
	if err := database.RequireSQLite(getEnv("DB_TYPE", database.DriverSQLite)); err != nil {
		fmt.Fprintln(stderr, err)
		return exitError
	}
	if _, err := os.Stat(cfg.DBPath); err != nil {
		fmt.Fprintf(stderr, "There is no database at %s, so there is nothing to back up. Run this command inside the app's container "+
			"(docker exec jim-dot-tennis ./jim-dot-tennis backup), where DB_PATH points at the app's database.\n", cfg.DBPath)
		return exitError
	}
	db, err := setupDatabase()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitError
	}
	defer db.Close()
	res, err := backup.Create(ctx, db, cfg, backup.KindManual)
	if err != nil {
		fmt.Fprintf(stderr, "Not done: %v\n", err)
		return exitError
	}
	c.printf("Backup made and checked:\n\n    %s\n\n", res.Path)
	c.printf("%d bytes; database version %d; %d uploaded file(s); app version %s.\n",
		res.Size, res.Manifest.SchemaVersion, len(res.Manifest.Files), res.Manifest.AppVersion)
	c.printf("Backups kept in %s: the newest %d.\n\n", cfg.Dir, max(cfg.Keep, 1))
	c.printf("It holds everyone's personal details and password hashes: keep it private.\n" +
		"It is on the same disk as the app; copy it off this server too (see the operations guide).\n")
	return exitOK
}

// runRestore is `jim-dot-tennis restore <file>`.
func runRestore(ctx context.Context, c cliOut, stderr io.Writer, archive string) int {
	cfg, err := backup.FromEnv()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitError
	}
	if err := database.RequireSQLite(getEnv("DB_TYPE", database.DriverSQLite)); err != nil {
		fmt.Fprintln(stderr, err)
		return exitError
	}
	c.printf("Checking %s and restoring it into %s ...\n", archive, cfg.DataDir())
	res, err := backup.Restore(ctx, cfg, archive, jimdottennis.Migrations())
	if err != nil {
		switch {
		case errors.Is(err, backup.ErrServerRunning), errors.Is(err, backup.ErrNewerBackup):
			// The message already says nothing was changed.
			fmt.Fprintf(stderr, "Not restored: %v.\n", err)
		default:
			fmt.Fprintf(stderr, "Not restored: %v.\nThe current database and files were not changed.\n", err)
		}
		return exitError
	}
	m := res.Manifest
	c.printf("\nRestored the backup made %s (%s backup, app version %s).\n",
		m.CreatedAt.Local().Format("2 Jan 2006 15:04"), m.Kind, m.AppVersion)
	if res.SchemaFrom != res.SchemaTo {
		c.printf("Its database was upgraded from version %d to %d.\n", res.SchemaFrom, res.SchemaTo)
	}
	if res.ReplacedDatabase != "" {
		c.printf("The database it replaced is kept as %s", filepath.Base(res.ReplacedDatabase))
		if res.ReplacedUploads != "" {
			c.printf(" and the uploaded files as %s", filepath.Base(res.ReplacedUploads))
		}
		c.printf(" in %s.\nDelete the replaced copies once you are sure the restore is right.\n", cfg.DataDir())
	}
	c.printf("\nEveryone has to sign in again: %d session(s) from the backup were ended and %d unused invite or\n"+
		"recovery link(s) cancelled. Accounts, passwords and passkeys are as they were when the backup was made;\n"+
		"check them with \"jim-dot-tennis user list\" once the app is running.\n", res.SessionsEnded, res.LinksCancelled)
	c.printf("\nNow start the app again.\n")
	return exitOK
}
