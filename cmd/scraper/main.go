package main

import (
	"context"
	"flag"
	"log"
	"os"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/scraper"
)

func main() {
	// Define command-line flags
	dbPath := flag.String("db", "tennis.db", "Path to SQLite database file")
	seasonYear := flag.Int("year", 2025, "Season year")
	seasonName := flag.String("season", "Summer 2025", "Season name")
	useMock := flag.Bool("mock", true, "Use mock data instead of scraping website")
	importFixtures := flag.Bool("fixtures", true, "Import fixtures data")
	importPlayers := flag.Bool("players", true, "Import player data")
	flag.Parse()

	// Ensure the database file exists
	if _, err := os.Stat(*dbPath); os.IsNotExist(err) {
		log.Fatalf("Database file %s does not exist", *dbPath)
	}

	// Set up the database connection
	dbConfig := database.Config{
		Driver:   "sqlite3",
		FilePath: *dbPath,
	}

	db, err := database.New(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Create a context
	ctx := context.Background()

	// Initialize the scraper
	s := scraper.NewScraper(db)

	// Import fixtures data if requested
	if *importFixtures {
		// Run the appropriate import function
		if *useMock {
			// Use mock data
			err = s.ImportMockData(ctx, *seasonYear, *seasonName)
			if err != nil {
				log.Fatalf("Error importing mock data: %v", err)
			}
			log.Println("Mock fixture data import completed successfully")
		} else {
			// Scrape from website
			err = s.ImportData(ctx, scraper.ImportConfig{
				FixturesURL: "https://www.bhplta.co.uk/bhplta_tables/fixtures/",
				ResultsURL:  "https://www.bhplta.co.uk/bhplta_tables/results/",
				TablesURL:   "https://www.bhplta.co.uk/bhplta_tables/league-table/",
				SeasonYear:  *seasonYear,
				SeasonName:  *seasonName,
			})
			if err != nil {
				log.Fatalf("Error importing data: %v", err)
			}
			log.Println("Web fixture data import completed successfully")
		}
	}

	// Import player data if requested
	if *importPlayers {
		err = s.ImportMockPlayers(ctx)
		if err != nil {
			log.Fatalf("Error importing player data: %v", err)
		}
		log.Println("Player data import completed successfully")
	}

	log.Println("All requested data imports completed")
} 