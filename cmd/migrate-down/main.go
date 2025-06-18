package main

import (
	"flag"
	"fmt"
	"log"

	"jim-dot-tennis/internal/database"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	// Define command line flags
	migrationsPath := flag.String("path", "./migrations", "Path to migration files")
	dbPath := flag.String("db-path", "./tennis.db", "Database file path")
	targetVersion := flag.Uint("version", 5, "Target migration version to migrate down to")
	flag.Parse()

	// Get database configuration
	config := database.Config{
		Driver:   "sqlite3",
		FilePath: *dbPath,
	}

	// Connect to the database
	db, err := database.New(config)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Create migration driver
	driver, err := sqlite3.WithInstance(db.DB.DB, &sqlite3.Config{})
	if err != nil {
		log.Fatalf("Failed to create migration driver: %v", err)
	}

	// Create migration instance
	sourceURL := fmt.Sprintf("file://%s", *migrationsPath)
	m, err := migrate.NewWithDatabaseInstance(sourceURL, "sqlite3", driver)
	if err != nil {
		log.Fatalf("Failed to create migration instance: %v", err)
	}

	// Get current version
	currentVersion, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		log.Fatalf("Failed to get current migration version: %v", err)
	}

	log.Printf("Current migration version: %d, dirty: %v", currentVersion, dirty)

	// Run down migration to target version
	if err := m.Migrate(*targetVersion); err != nil {
		if err == migrate.ErrNoChange {
			log.Printf("No migration changes needed - already at version %d", *targetVersion)
			return
		}
		log.Fatalf("Failed to migrate down to version %d: %v", *targetVersion, err)
	}

	log.Printf("Successfully migrated down to version %d", *targetVersion)
}
