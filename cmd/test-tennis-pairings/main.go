package main

import (
	"context"
	"fmt"
	"log"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
)

func main() {
	ctx := context.Background()

	// Connect to database
	dbConfig := database.Config{
		Driver:   "sqlite3",
		FilePath: "./tennis.db",
	}

	db, err := database.New(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize repositories
	tennisPlayerRepo := repository.NewTennisPlayerRepository(db)
	fantasyRepo := repository.NewFantasyMixedDoublesRepository(db)

	// Get ATP players (top 20 to ensure we have enough for multiple matches)
	atpPlayers, err := tennisPlayerRepo.FindByTour(ctx, "ATP")
	if err != nil {
		log.Fatalf("Failed to get ATP players: %v", err)
	}

	// Get WTA players (top 20 to ensure we have enough for multiple matches)
	wtaPlayers, err := tennisPlayerRepo.FindByTour(ctx, "WTA")
	if err != nil {
		log.Fatalf("Failed to get WTA players: %v", err)
	}

	fmt.Printf("Found %d ATP players and %d WTA players\n", len(atpPlayers), len(wtaPlayers))

	// Generate some test pairings (proper mixed doubles: Team A Woman+Man vs Team B Woman+Man)
	// Use different players for each match to ensure uniqueness
	for i := 0; i < 3 && i*2+1 < len(atpPlayers) && i*2+1 < len(wtaPlayers); i++ {
		teamAWoman := wtaPlayers[i*2]
		teamAMan := atpPlayers[i*2]
		teamBWoman := wtaPlayers[i*2+1]
		teamBMan := atpPlayers[i*2+1]

		// Generate auth token
		authToken := fantasyRepo.GenerateAuthToken(&teamAWoman, &teamAMan, &teamBWoman, &teamBMan)

		fmt.Printf("Match %d: Team A (%s %s + %s %s) vs Team B (%s %s + %s %s) = Auth Token: %s\n",
			i+1,
			teamAWoman.FirstName, teamAWoman.LastName, teamAMan.FirstName, teamAMan.LastName,
			teamBWoman.FirstName, teamBWoman.LastName, teamBMan.FirstName, teamBMan.LastName,
			authToken)

		// Create the match
		match := &models.FantasyMixedDoubles{
			TeamAWomanID: teamAWoman.ID,
			TeamAManID:   teamAMan.ID,
			TeamBWomanID: teamBWoman.ID,
			TeamBManID:   teamBMan.ID,
			AuthToken:    authToken,
			IsActive:     true,
		}

		if err := fantasyRepo.Create(ctx, match); err != nil {
			log.Printf("Failed to create match: %v", err)
		} else {
			fmt.Printf("  Created match with ID: %d\n", match.ID)
		}
	}

	// List all matches
	matches, err := fantasyRepo.FindAll(ctx)
	if err != nil {
		log.Fatalf("Failed to get matches: %v", err)
	}

	fmt.Printf("\nAll matches (%d total):\n", len(matches))
	for _, match := range matches {
		fmt.Printf("ID: %d, Auth Token: %s, Active: %t\n", match.ID, match.AuthToken, match.IsActive)
	}

	// Test finding by auth token
	if len(matches) > 0 {
		testToken := matches[0].AuthToken
		found, err := fantasyRepo.FindByAuthToken(ctx, testToken)
		if err != nil {
			log.Printf("Failed to find by auth token: %v", err)
		} else {
			fmt.Printf("\nFound match by token '%s': ID %d\n", testToken, found.ID)
		}
	}
}
