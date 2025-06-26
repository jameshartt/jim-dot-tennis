package main

import (
	"context"
	"flag"
	"log"
	"os"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/repository"
)

func main() {
	var (
		dbPath   = flag.String("db-path", "./tennis.db", "Path to SQLite database file")
		jsonFile = flag.String("json-file", "cmd/collect_tennis_data/tennis_players.json", "Path to tennis players JSON file")
		verbose  = flag.Bool("verbose", false, "Enable verbose logging")
		dryRun   = flag.Bool("dry-run", false, "Show what would be imported without making changes")
	)
	flag.Parse()

	if *verbose {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}

	log.Printf("🎾 Tennis Player Import Tool")
	log.Printf("Database: %s", *dbPath)
	log.Printf("JSON File: %s", *jsonFile)

	// Check if JSON file exists
	if _, err := os.Stat(*jsonFile); os.IsNotExist(err) {
		log.Fatalf("❌ Tennis players JSON file not found: %s", *jsonFile)
	}

	// Connect to database
	dbConfig := database.Config{
		Driver:   "sqlite3",
		FilePath: *dbPath,
	}

	db, err := database.New(dbConfig)
	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize tennis player repository
	tennisPlayerRepo := repository.NewProTennisPlayerRepository(db)
	ctx := context.Background()

	if *dryRun {
		log.Printf("🔍 [DRY RUN] Would import tennis players from %s", *jsonFile)

		// Check current player count
		currentCount, err := tennisPlayerRepo.CountAll(ctx)
		if err != nil {
			log.Printf("⚠️  Warning: Failed to count current players: %v", err)
		} else {
			log.Printf("📊 Current players in database: %d", currentCount)
		}

		log.Printf("✅ Dry run complete - no changes made")
		return
	}

	// Check current state
	if *verbose {
		currentCount, err := tennisPlayerRepo.CountAll(ctx)
		if err != nil {
			log.Printf("⚠️  Warning: Failed to count current players: %v", err)
		} else {
			log.Printf("📊 Current players in database: %d", currentCount)
		}
	}

	// Import tennis players
	log.Printf("🔄 Importing tennis players...")
	if err := tennisPlayerRepo.ImportFromJSON(ctx, *jsonFile); err != nil {
		log.Fatalf("❌ Failed to import tennis players: %v", err)
	}

	// Get final counts for reporting
	atpCount, err := tennisPlayerRepo.CountByTour(ctx, "ATP")
	if err != nil {
		log.Printf("⚠️  Warning: Failed to count ATP players: %v", err)
		atpCount = 0
	}

	wtaCount, err := tennisPlayerRepo.CountByTour(ctx, "WTA")
	if err != nil {
		log.Printf("⚠️  Warning: Failed to count WTA players: %v", err)
		wtaCount = 0
	}

	totalCount, err := tennisPlayerRepo.CountAll(ctx)
	if err != nil {
		log.Printf("⚠️  Warning: Failed to count total players: %v", err)
		totalCount = atpCount + wtaCount
	}

	log.Printf("✅ Import completed successfully!")
	log.Printf("📊 Final counts:")
	log.Printf("   • ATP Players: %d", atpCount)
	log.Printf("   • WTA Players: %d", wtaCount)
	log.Printf("   • Total Players: %d", totalCount)
}
