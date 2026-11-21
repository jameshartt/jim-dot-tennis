// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// Config holds the database configuration
type Config struct {
	Driver   string // must be "sqlite3" (the only supported driver)
	FilePath string // SQLite database file path
}

// DriverSQLite is the only supported database driver.
const DriverSQLite = "sqlite3"

// RequireSQLite returns a clear error unless driver is empty or "sqlite3".
// SQLite is the only database jim-dot-tennis supports.
func RequireSQLite(driver string) error {
	if driver == "" || driver == DriverSQLite {
		return nil
	}
	return fmt.Errorf("unsupported database type %q: only SQLite is supported (leave DB_TYPE unset or set DB_TYPE=sqlite3)", driver)
}

// DB is a wrapper around sqlx.DB
type DB struct {
	*sqlx.DB
}

// New creates a new database connection
func New(cfg Config) (*DB, error) {
	if err := RequireSQLite(cfg.Driver); err != nil {
		return nil, err
	}

	// WAL + busy_timeout let concurrent readers/writers wait instead of
	// failing with "database is locked"; foreign_keys is off by default
	// in mattn/go-sqlite3, so cascades are unenforced without it.
	dataSourceName := fmt.Sprintf(
		"file:%s?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL&_foreign_keys=on",
		cfg.FilePath,
	)

	db, err := sqlx.Connect(DriverSQLite, dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Set reasonable defaults for the connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{db}, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	if db.DB != nil {
		if err := db.DB.Close(); err != nil {
			return fmt.Errorf("failed to close database connection: %w", err)
		}
	}
	return nil
}

// ExecuteMigrations runs the migrations in a directory on disk (tests and
// tools pointed at a checkout; the server uses the embedded copies through
// ExecuteMigrationsFS).
func (db *DB) ExecuteMigrations(migrationsPath string) error {
	if _, err := os.Stat(migrationsPath); err != nil {
		return fmt.Errorf("migrations directory: %w", err)
	}
	return db.ExecuteMigrationsFS(os.DirFS(migrationsPath), migrationsPath)
}

// NewMigrate returns a golang-migrate instance for this database reading the
// NNN_name.up.sql / .down.sql files at the root of migrations (for example
// jimdottennis.Migrations(), the copies built into the binary).
func (db *DB) NewMigrate(migrations fs.FS) (*migrate.Migrate, error) {
	driver, err := sqlite3.WithInstance(db.DB.DB, &sqlite3.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to create migration driver: %w", err)
	}
	src, err := iofs.New(migrations, ".")
	if err != nil {
		return nil, fmt.Errorf("failed to read migrations: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, db.DriverName(), driver)
	if err != nil {
		return nil, fmt.Errorf("failed to create migration instance: %w", err)
	}
	return m, nil
}

// ExecuteMigrationsFS runs the migrations in migrations; label names their
// source in the log.
func (db *DB) ExecuteMigrationsFS(migrations fs.FS, label string) error {
	log.Printf("Running migrations from: %s", label)

	m, err := db.NewMigrate(migrations)
	if err != nil {
		return err
	}

	// Check if the database is in a dirty state
	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("failed to get migration version: %w", err)
	}

	// A dirty database means an earlier migration stopped part-way, so the
	// schema may be half changed. Never carry on over that and never force the
	// version to hide it: stop and say what to do (see MigrationError).
	if dirty {
		return &MigrationError{Version: version, Dirty: true}
	}

	// Run migrations
	if err := m.Up(); err != nil {
		if err == migrate.ErrNoChange {
			log.Println("No migrations to apply - database is up to date")
			return nil
		}
		// After a failed migration golang-migrate leaves the version table
		// dirty at the migration that failed; that is the one to name.
		if v, d, verr := m.Version(); verr == nil && d {
			return &MigrationError{Version: v, Cause: err}
		}
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	log.Println("Migrations applied successfully")
	return nil
}

// SchemaStatus is the migration state recorded in a database.
type SchemaStatus struct {
	Version  uint // highest applied migration (0 when none)
	Dirty    bool // a migration started but did not finish
	Migrated bool // the database has a migration table at all
}

// ReadSchemaStatus reads the migration state without changing anything
// (unlike ExecuteMigrations it never applies or forces a migration).
func (db *DB) ReadSchemaStatus(ctx context.Context) (SchemaStatus, error) {
	var exists bool
	if err := db.GetContext(ctx, &exists, `SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = 'schema_migrations')`); err != nil {
		return SchemaStatus{}, fmt.Errorf("check migration table: %w", err)
	}
	if !exists {
		return SchemaStatus{}, nil
	}
	var row struct {
		Version uint `db:"version"`
		Dirty   bool `db:"dirty"`
	}
	err := db.GetContext(ctx, &row, `SELECT version, dirty FROM schema_migrations LIMIT 1`)
	if errors.Is(err, sql.ErrNoRows) {
		return SchemaStatus{Migrated: true}, nil
	}
	if err != nil {
		return SchemaStatus{}, fmt.Errorf("read migration version: %w", err)
	}
	return SchemaStatus{Version: row.Version, Dirty: row.Dirty, Migrated: true}, nil
}

// LatestMigration returns the highest migration number among the
// NNN_name.up.sql files at the root of migrations.
func LatestMigration(migrations fs.FS) (uint, error) {
	entries, err := fs.ReadDir(migrations, ".")
	if err != nil {
		return 0, fmt.Errorf("read migrations: %w", err)
	}
	var latest uint
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		prefix, _, ok := strings.Cut(name, "_")
		if !ok {
			continue
		}
		n, err := strconv.ParseUint(prefix, 10, 32)
		if err != nil {
			continue
		}
		if uint(n) > latest {
			latest = uint(n)
		}
	}
	if latest == 0 {
		return 0, errors.New("no migrations found")
	}
	return latest, nil
}
