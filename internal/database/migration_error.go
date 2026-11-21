// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package database

import "fmt"

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
		"(2) restore the database file from the backup you took before upgrading (a half-finished migration cannot be safely repeated or skipped); " +
		"(3) start the previous version of the app; " +
		"(4) report this message to whoever maintains the app before trying the upgrade again"
}

func (e *MigrationError) Unwrap() error { return e.Cause }
