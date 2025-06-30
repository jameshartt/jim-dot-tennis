package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/repository"
	"jim-dot-tennis/internal/services"
)

func main() {
	// Define command line flags
	var (
		dbPath       = flag.String("db", "data/jim-dot-tennis.db", "Path to SQLite database")
		clubName     = flag.String("club-name", "St Anns", "Club name")
		clubID       = flag.Int("club-id", 10, "Club ID")
		year         = flag.Int("year", 2025, "Year")
		week         = flag.Int("week", 1, "Week number to import")
		nonce        = flag.String("nonce", "", "WordPress nonce")
		clubCode     = flag.String("club-code", "", "Club code (from clubcode cookie)")
		wpLoggedIn   = flag.String("wp-logged-in", "", "WordPress logged in cookie")
		wpSec        = flag.String("wp-sec", "", "WordPress security cookie")
		baseURL      = flag.String("url", "https://www.bhplta.co.uk/wp-admin/admin-ajax.php", "Base URL for API requests")
		rateLimitStr = flag.String("rate-limit", "1s", "Rate limit between requests (e.g., 1s, 500ms)")
		dryRun       = flag.Bool("dry-run", false, "Dry run mode - don't save to database")
		verbose      = flag.Bool("verbose", true, "Verbose output")
	)
	flag.Parse()

	// Validate required flags
	if *nonce == "" {
		log.Fatal("nonce is required")
	}
	if *clubCode == "" {
		log.Fatal("club-code is required")
	}

	// Parse rate limit
	rateLimit, err := time.ParseDuration(*rateLimitStr)
	if err != nil {
		log.Fatalf("Invalid rate limit: %v", err)
	}

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

	// Create repositories
	fixtureRepo := repository.NewFixtureRepository(db)
	matchupRepo := repository.NewMatchupRepository(db)
	teamRepo := repository.NewTeamRepository(db)
	clubRepo := repository.NewClubRepository(db)
	playerRepo := repository.NewPlayerRepository(db)

	// Create match card service
	service := services.NewMatchCardService(
		fixtureRepo,
		matchupRepo,
		teamRepo,
		clubRepo,
		playerRepo,
	)

	// Create import configuration
	config := services.ImportConfig{
		ClubName:                *clubName,
		ClubID:                  *clubID,
		Year:                    *year,
		Nonce:                   *nonce,
		ClubCode:                *clubCode,
		WordPressLoggedInCookie: *wpLoggedIn,
		WordPressSecCookie:      *wpSec,
		BaseURL:                 *baseURL,
		RateLimit:               rateLimit,
		DryRun:                  *dryRun,
		Verbose:                 *verbose,
	}

	// Import match cards for the specified week
	ctx := context.Background()
	result, err := service.ImportWeekMatchCards(ctx, config, *week)
	if err != nil {
		log.Fatalf("Failed to import match cards: %v", err)
	}

	// Print results
	fmt.Printf("\n=== Import Results ===\n")
	fmt.Printf("Processed matches: %d\n", result.ProcessedMatches)
	fmt.Printf("Updated fixtures: %d\n", result.UpdatedFixtures)
	fmt.Printf("Created matchups: %d\n", result.CreatedMatchups)
	fmt.Printf("Updated matchups: %d\n", result.UpdatedMatchups)
	fmt.Printf("Matched players: %d\n", result.MatchedPlayers)

	if len(result.UnmatchedPlayers) > 0 {
		fmt.Printf("\nUnmatched players (%d):\n", len(result.UnmatchedPlayers))
		for _, player := range result.UnmatchedPlayers {
			fmt.Printf("  - %s\n", player)
		}
	}

	if len(result.Errors) > 0 {
		fmt.Printf("\nErrors (%d):\n", len(result.Errors))
		for _, err := range result.Errors {
			fmt.Printf("  - %s\n", err)
		}
	}

	if *dryRun {
		fmt.Printf("\n*** DRY RUN MODE - No changes were saved to the database ***\n")
	}
}
