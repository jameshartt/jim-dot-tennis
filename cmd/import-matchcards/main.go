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
		dbPath        = flag.String("db", "", "Database path")
		nonce         = flag.String("nonce", "", "BHPLTA nonce (if empty, will try to extract automatically)")
		clubCode      = flag.String("club-code", "", "Club code")
		week          = flag.Int("week", 0, "Week number")
		year          = flag.Int("year", 0, "Year")
		clubID        = flag.Int("club-id", 0, "Club ID")
		clubName      = flag.String("club-name", "", "Club name")
		dryRun        = flag.Bool("dry-run", false, "Dry run mode")
		verbose       = flag.Bool("verbose", false, "Verbose output")
		clearExisting = flag.Bool("clear-existing", false, "Clear existing matchups before importing")
		autoNonce     = flag.Bool("auto-nonce", false, "Automatically extract nonce from website")
	)
	flag.Parse()

	// Validate required flags
	if *nonce == "" && !*autoNonce {
		log.Fatal("nonce is required (or use -auto-nonce to extract automatically)")
	}
	if *clubCode == "" && *nonce == "" {
		log.Fatal("club-code is required when auto-extracting nonce")
	}

	// Parse rate limit
	rateLimit, rateLimitErr := time.ParseDuration("1s") // Default rate limit
	if rateLimitErr != nil {
		log.Fatalf("Invalid rate limit: %v", rateLimitErr)
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
		ClubName:              *clubName,
		ClubID:                *clubID,
		Year:                  *year,
		Nonce:                 *nonce,
		ClubCode:              *clubCode,
		BaseURL:               "https://www.bhplta.co.uk/wp-admin/admin-ajax.php", // Default base URL
		RateLimit:             rateLimit,
		DryRun:                *dryRun,
		Verbose:               *verbose,
		ClearExistingMatchups: *clearExisting,
	}

	// Import match cards for the specified week
	ctx := context.Background()
	var result *services.ImportResult

	// Use auto-nonce extraction if requested or if no nonce provided
	if *autoNonce || *nonce == "" {
		result, err = service.ImportWeekMatchCardsWithAutoNonce(ctx, config, *week)
	} else {
		result, err = service.ImportWeekMatchCards(ctx, config, *week)
	}

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

	if *clearExisting {
		fmt.Printf("\n*** CLEAR EXISTING MODE - Existing matchups were cleared before importing ***\n")
	}
}
