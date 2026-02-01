package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"jim-dot-tennis/internal/database"
)

func main() {
	// Define command line flags
	migrationsPath := flag.String("path", "./migrations", "Path to migration files")
	dbType := flag.String("db", "", "Database type (postgres or sqlite3), defaults to env DB_TYPE")
	dbHost := flag.String("host", "", "Database host (for postgres), defaults to env DB_HOST")
	dbPort := flag.String("port", "", "Database port (for postgres), defaults to env DB_PORT")
	dbUser := flag.String("user", "", "Database user (for postgres), defaults to env DB_USER")
	dbPass := flag.String("pass", "", "Database password (for postgres), defaults to env DB_PASSWORD")
	dbName := flag.String("name", "", "Database name (for postgres), defaults to env DB_NAME")
	dbPath := flag.String("db-path", "", "Database file path (for sqlite3), defaults to env DB_PATH")
	flag.Parse()

	// Get database configuration from flags or environment variables
	config := database.Config{
		Driver: getStringValue(*dbType, "DB_TYPE", "sqlite3"),
	}

	if config.Driver == "postgres" {
		config.Host = getStringValue(*dbHost, "DB_HOST", "localhost")
		portStr := getStringValue(*dbPort, "DB_PORT", "5432")
		config.Port, _ = strconv.Atoi(portStr)
		config.User = getStringValue(*dbUser, "DB_USER", "postgres")
		config.Password = getStringValue(*dbPass, "DB_PASSWORD", "postgres")
		config.DBName = getStringValue(*dbName, "DB_NAME", "tennis")
		config.SSLMode = "disable"
	} else if config.Driver == "sqlite3" {
		config.FilePath = getStringValue(*dbPath, "DB_PATH", "./tennis.db")
	} else {
		log.Fatalf("Unsupported database driver: %s", config.Driver)
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
