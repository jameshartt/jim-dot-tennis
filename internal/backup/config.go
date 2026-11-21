// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

// Package backup makes and restores backups of an instance: one archive
// (gzip-compressed tar) holding a consistent copy of the SQLite database, the
// uploaded files on the data volume (the club logo) and a manifest that
// records the app version, the database schema version, when it was made and
// a SHA-256 checksum of every file.
//
// The database copy is made with SQLite's VACUUM INTO, which reads the live
// database inside one read transaction, so it is consistent while the server
// keeps writing (the live file is never copied: with WAL, the file alone can
// be missing committed changes, or be caught half-written).
//
// Restore (restore.go) only runs while the server is stopped, which it checks
// with an operating-system lock on a file in the data directory that the
// server holds for as long as it runs (lock_unix.go).
//
// An archive holds personal data, password hashes, passkey public keys, the
// web push private key and every other secret the database stores. Archives
// are written 0600 in a 0700 directory and are never served by the web app
// (the static handler serves only the files built into the binary and the
// branding handler only the current logo by its content-hash name).
package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Environment variables read by FromEnv.
const (
	// EnvDir is the folder backups are kept in (default: "backups" next to
	// the database, on the data volume).
	EnvDir = "BACKUP_DIR"
	// EnvKeep is how many scheduled and manual backups are kept (default 14).
	EnvKeep = "BACKUP_KEEP"
	// EnvInterval is how often the server takes a backup, as a Go duration
	// such as 24h or 6h, or "off" (default 24h).
	EnvInterval = "BACKUP_INTERVAL"
)

// Defaults.
const (
	DefaultKeep     = 14
	DefaultInterval = 24 * time.Hour
	// MinInterval is the shortest schedule accepted.
	MinInterval = 15 * time.Minute
	// staleWhenOff is when a backup counts as old if the schedule is off.
	staleWhenOff = 7 * 24 * time.Hour
)

// Config says where the instance's data is and how backups are kept.
type Config struct {
	DBPath   string        // the SQLite database (DB_PATH)
	Dir      string        // where archives are kept
	Keep     int           // how many regular archives to keep (at least 1)
	Interval time.Duration // scheduled backup interval; 0 = no schedule
}

// DataDir is the directory holding the database: the data volume.
func (c Config) DataDir() string { return filepath.Dir(c.DBPath) }

// UploadsDir is the directory of uploaded club logos (config.BrandingDir).
func (c Config) UploadsDir() string { return filepath.Join(c.DataDir(), uploadsDirName) }

// LockPath is the file the server holds a lock on while it runs.
func (c Config) LockPath() string { return filepath.Join(c.DataDir(), lockFileName) }

// StaleAfter is how old the newest backup may get before the backups page
// warns: two missed scheduled runs, or a week when there is no schedule.
func (c Config) StaleAfter() time.Duration {
	if c.Interval <= 0 {
		return staleWhenOff
	}
	return 2 * c.Interval
}

// FromEnv builds the configuration from DB_PATH and the BACKUP_* variables,
// with a plain-words error for a value that cannot be used.
func FromEnv() (Config, error) {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./tennis.db"
	}
	c := Config{DBPath: dbPath, Keep: DefaultKeep, Interval: DefaultInterval}
	c.Dir = filepath.Join(c.DataDir(), "backups")
	if v := strings.TrimSpace(os.Getenv(EnvDir)); v != "" {
		c.Dir = v
	}
	if v := strings.TrimSpace(os.Getenv(EnvKeep)); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return c, fmt.Errorf("%s is %q: set it to how many backups to keep, a whole number of 1 or more (for example 14), or remove it to keep %d", EnvKeep, v, DefaultKeep)
		}
		c.Keep = n
	}
	if v := strings.TrimSpace(os.Getenv(EnvInterval)); v != "" {
		switch strings.ToLower(v) {
		case "off", "0", "false", "none":
			c.Interval = 0
		default:
			d, err := time.ParseDuration(v)
			if err != nil || d < MinInterval {
				return c, fmt.Errorf("%s is %q: set it to how often to take a backup, for example 24h or 6h (at least %s), or to off; remove it for daily backups", EnvInterval, v, MinInterval)
			}
			c.Interval = d
		}
	}
	abs, err := filepath.Abs(c.Dir)
	if err == nil {
		c.Dir = abs
	}
	return c, nil
}
