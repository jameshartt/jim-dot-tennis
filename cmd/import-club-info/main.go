package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/repository"
	"jim-dot-tennis/internal/services"
)

func main() {
	// Define command line flags
	var (
		dbPath   = flag.String("db", "./tennis.db", "Database path")
		dryRun   = flag.Bool("dry-run", false, "Show what would happen without writing to the database")
		verbose  = flag.Bool("verbose", false, "Enable verbose output")
		clubSlug = flag.String("club-slug", "", "Scrape a single club by slug (e.g. 'st-anns', 'blakers')")
	)
	flag.Parse()

	if *verbose {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
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

	// Create repository
	clubRepo := repository.NewClubRepository(db)

	// Create scraper service
	scraper := services.NewClubScraper(db, clubRepo, *dryRun, *verbose)

	ctx := context.Background()

	var summary *services.ClubScraperSummary

	if *clubSlug != "" {
		// Scrape a single club
		fmt.Printf("Scraping club: %s\n", *clubSlug)

		// Validate the slug is known
		found := false
		for _, m := range services.KnownClubSlugs {
			if m.Slug == *clubSlug {
				found = true
				break
			}
		}
		if !found {
			fmt.Printf("WARNING: '%s' is not in the known club slugs list. Proceeding anyway.\n", *clubSlug)
			fmt.Printf("Known slugs: ")
			for i, m := range services.KnownClubSlugs {
				if i > 0 {
					fmt.Printf(", ")
				}
				fmt.Printf("%s", m.Slug)
			}
			fmt.Println()
		}

		summary, err = scraper.ScrapeClub(ctx, *clubSlug)
		if err != nil {
			log.Fatalf("Failed to scrape club %s: %v", *clubSlug, err)
		}
	} else {
		// Scrape all known clubs
		fmt.Printf("Scraping all %d known BHPLTA clubs...\n", len(services.KnownClubSlugs))
		summary, err = scraper.ScrapeAll(ctx)
		if err != nil {
			log.Fatalf("Failed to scrape clubs: %v", err)
		}
	}

	// Print summary
	fmt.Printf("\n=== Club Import Summary ===\n")
	fmt.Printf("Matched to existing: %d\n", summary.Matched)
	fmt.Printf("Created new:         %d\n", summary.Created)
	fmt.Printf("Updated:             %d\n", summary.Updated)
	fmt.Printf("Skipped (no change): %d\n", summary.Skipped)

	if len(summary.Errors) > 0 {
		fmt.Printf("\nErrors (%d):\n", len(summary.Errors))
		for _, e := range summary.Errors {
			fmt.Printf("  - %s\n", e)
		}
	}

	if *dryRun {
		fmt.Printf("\n*** DRY RUN MODE - No changes were saved to the database ***\n")
	}
}
