// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"jim-dot-tennis/internal/database"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	// Define command line flags.
	// -version has no safe default: rolling back is destructive, so it must be
	// given explicitly (a no-arg run used to silently roll back to version 5,
	// destroying every migration above it).
	migrationsPath := flag.String("path", "./migrations", "Path to migration files")
	dbPath := flag.String("db-path", "./tennis.db", "Database file path")
	targetVersion := flag.Int("version", -1, "Target migration version to migrate down to (required)")
	assumeYes := flag.Bool("yes", false, "Skip the interactive confirmation prompt")
	flag.Parse()

	if *targetVersion < 0 {
		fmt.Fprintln(os.Stderr, "error: -version is required (the target version to roll back to)")
		fmt.Fprintln(os.Stderr, "example: migrate-down -db-path ./tennis.db -version 27")
		flag.Usage()
		os.Exit(2)
	}

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

	if uint(*targetVersion) >= currentVersion {
		log.Printf("Target version %d is not below current version %d - nothing to roll back", *targetVersion, currentVersion)
		return
	}

	// Confirm before running a destructive rollback.
	if !*assumeYes {
		fmt.Printf("About to roll back %s from version %d DOWN to version %d.\n", *dbPath, currentVersion, *targetVersion)
		fmt.Printf("This runs the down migrations for versions %d..%d and can drop data. Continue? [y/N]: ", currentVersion, *targetVersion+1)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(answer)) != "y" {
			log.Println("Aborted.")
			return
		}
	}

	// Run down migration to target version
	if err := m.Migrate(uint(*targetVersion)); err != nil {
		if err == migrate.ErrNoChange {
			log.Printf("No migration changes needed - already at version %d", *targetVersion)
			return
		}
		log.Fatalf("Failed to migrate down to version %d: %v", *targetVersion, err)
	}

	log.Printf("Successfully migrated down to version %d", *targetVersion)
}
