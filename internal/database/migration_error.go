// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package database

import (
	"fmt"

	"jim-dot-tennis/internal/version"
)

// MigrationError is a migration that failed, or that an earlier start left
// half done (Dirty). Its message names the migration and says what to do.
type MigrationError struct {
	Version uint  // the migration number, e.g. 36 for 036_...
	Dirty   bool  // found half done at startup (rather than failing now)
	Cause   error // why it failed now (nil when Dirty)
}

func (e *MigrationError) Error() string {
	var what string
	if e.Dirty {
		what = fmt.Sprintf("migration %03d did not finish the last time the app started, so the database is only part-way through an upgrade", e.Version)
	} else {
		what = fmt.Sprintf("migration %03d failed: %v", e.Version, e.Cause)
	}
	return what + ". The app has not started and nothing was served. To recover: " +
		"(1) stop the app; " +
		"(2) put the database back as it was before the upgrade, from the backup the app made just before running migrations " +
		"(jim-dot-tennis-pre-migration-<date>.tar.gz in the backups folder, normally backups/ next to the database): " +
		"run \"jim-dot-tennis restore <that file>\" with the previous version of the app, in a one-off container while the app is stopped " +
		"(a half-finished migration cannot be safely repeated or skipped; the steps are in docs/upgrade.md, \"Going back to the previous version (rolling back)\"); " +
		"(3) start the previous version of the app; " +
		"(4) report this message to whoever maintains the app before trying the upgrade again"
}

func (e *MigrationError) Unwrap() error { return e.Cause }

// NewerSchemaError is a database whose schema is past the newest migration
// this build ships: a newer version of the software has used (and upgraded)
// it, and this older version is being started on it. Nothing is changed.
type NewerSchemaError struct {
	Found  uint // the schema version recorded in the database
	Latest uint // the newest migration this build has
}

func (e *NewerSchemaError) Error() string {
	return fmt.Sprintf("the database was last used by a newer version of jim-dot-tennis than this one (%s). "+
		"The newer version upgraded it (to schema %d; this version only knows up to schema %d), and an older version cannot use it. "+
		"Nothing was changed. Do one of these: "+
		"(1) start the newer version again (in your .env file, set JIM_DOT_TENNIS_VERSION back to it); or "+
		"(2) to go back to this version, restore the backup the newer version took just before it upgraded the database "+
		"(its name starts jim-dot-tennis-pre-migration-). "+
		"The steps are in docs/upgrade.md, \"Going back to the previous version (rolling back)\", under \"If the database was upgraded\"",
		version.Short(), e.Found, e.Latest)
}
