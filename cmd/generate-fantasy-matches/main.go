package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/repository"
)

func main() {
	// Command line flags
	var (
		count   = flag.Int("count", 5, "Number of fantasy matches to generate")
		dbPath  = flag.String("db", "./tennis.db", "Path to the SQLite database")
		verbose = flag.Bool("v", false, "Verbose output")
	)
	flag.Parse()

	log.Printf("Generating %d fantasy mixed doubles matches...", *count)

	// Connect to database
	dbConfig := database.Config{
		Driver:   "sqlite3",
		FilePath: *dbPath,
	}

	db, err := database.New(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize repository
	fantasyRepo := repository.NewFantasyMixedDoublesRepository(db)
	ctx := context.Background()

	// Check existing matches
	existingMatches, err := fantasyRepo.FindAll(ctx)
	if err != nil {
		log.Fatalf("Failed to check existing matches: %v", err)
	}

	if *verbose {
		log.Printf("Found %d existing fantasy matches in database", len(existingMatches))
	}

	// Generate new matches
	log.Printf("Generating %d new fantasy matches...", *count)
	if err := fantasyRepo.GenerateRandomMatches(ctx, *count); err != nil {
		log.Fatalf("Failed to generate fantasy matches: %v", err)
	}

	// List all active matches
	activeMatches, err := fantasyRepo.FindActive(ctx)
	if err != nil {
		log.Fatalf("Failed to retrieve active matches: %v", err)
	}

	log.Printf("Successfully generated matches! Total active matches: %d", len(activeMatches))

	if *verbose {
		fmt.Println("\nActive Fantasy Matches:")
		fmt.Println("=" + fmt.Sprintf("%0*s", 50, ""))
		for i, match := range activeMatches {
			fmt.Printf("%d. Auth Token: %s\n", i+1, match.AuthToken)
			fmt.Printf("   URL: /my-availability/%s\n", match.AuthToken)
			fmt.Println()
		}
	} else {
		fmt.Println("\nSample URLs for testing:")
		fmt.Println("=" + fmt.Sprintf("%0*s", 30, ""))
		maxDisplay := *count
		if len(activeMatches) < maxDisplay {
			maxDisplay = len(activeMatches)
		}
		for i := 0; i < maxDisplay; i++ {
			fmt.Printf("/my-availability/%s\n", activeMatches[i].AuthToken)
		}
	}
}
