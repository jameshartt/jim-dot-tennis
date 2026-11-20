// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"jim-dot-tennis/internal/database"
)

func main() {
	// Define command line flags
	migrationsPath := flag.String("path", "./migrations", "Path to migration files")
	dbType := flag.String("db", "", "Database type (only sqlite3 is supported), defaults to env DB_TYPE")
	dbPath := flag.String("db-path", "", "Database file path, defaults to env DB_PATH")
	flag.Parse()

	// SQLite is the only supported database.
	driver := getStringValue(*dbType, "DB_TYPE", database.DriverSQLite)
	if err := database.RequireSQLite(driver); err != nil {
		log.Fatalf("%v", err)
	}

	config := database.Config{
		Driver:   database.DriverSQLite,
		FilePath: getStringValue(*dbPath, "DB_PATH", "./tennis.db"),
	}

	// Connect to the database
	db, err := database.New(config)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := db.ExecuteMigrations(*migrationsPath); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	fmt.Println("Migrations completed successfully")
}

// getStringValue gets a string value from a flag, environment variable, or default value
func getStringValue(flagValue, envName, defaultValue string) string {
	if flagValue != "" {
		return flagValue
	}

	if envValue := os.Getenv(envName); envValue != "" {
		return envValue
	}

	return defaultValue
}
