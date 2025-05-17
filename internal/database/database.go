package database

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"          // PostgreSQL driver
	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// Config holds the database configuration
type Config struct {
	Driver   string
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
	FilePath string // For SQLite
}

// DB is a wrapper around sqlx.DB
type DB struct {
	*sqlx.DB
}

// New creates a new database connection
func New(cfg Config) (*DB, error) {
	var db *sqlx.DB
	var err error
	var dataSourceName string

	switch cfg.Driver {
	case "postgres":
		dataSourceName = fmt.Sprintf(
			"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
		)
	case "sqlite3":
		dataSourceName = cfg.FilePath
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}

	db, err = sqlx.Connect(cfg.Driver, dataSourceName)
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

// ExecuteMigrations runs the database migrations
func (db *DB) ExecuteMigrations(migrationsPath string) error {
	log.Printf("Running migrations from: %s", migrationsPath)
	
	var driver database.Driver
	var err error
	
	switch db.DriverName() {
	case "postgres":
		driver, err = postgres.WithInstance(db.DB.DB, &postgres.Config{})
	case "sqlite3":
		driver, err = sqlite3.WithInstance(db.DB.DB, &sqlite3.Config{})
	default:
		return fmt.Errorf("unsupported database driver for migrations: %s", db.DriverName())
	}
	
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}
	
	// Convert to absolute path for file:// URL
	absPath, err := filepath.Abs(migrationsPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute migrations path: %w", err)
	}
	
	// Create proper file:// URL without escaping path separators
	sourceURL := fmt.Sprintf("file://%s", absPath)
	m, err := migrate.NewWithDatabaseInstance(sourceURL, db.DriverName(), driver)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}
	
	// Check if the database is in a dirty state
	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("failed to get migration version: %w", err)
	}
	
	// If the database is in a dirty state, force the version
	if dirty {
		log.Printf("Database is in a dirty state at version %d. Forcing version.", version)
		if err := m.Force(int(version)); err != nil {
			return fmt.Errorf("failed to force migration version: %w", err)
		}
	}
	
	// Run migrations
	if err := m.Up(); err != nil {
		if err == migrate.ErrNoChange {
			log.Println("No migrations to apply - database is up to date")
			return nil
		}
		return fmt.Errorf("failed to apply migrations: %w", err)
	}
	
	log.Println("Migrations applied successfully")
	return nil
} 