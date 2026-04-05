// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/normalize"
	"jim-dot-tennis/internal/repository"
	"jim-dot-tennis/internal/services"
)

func main() {
	dbPath := flag.String("db", "./tennis.db", "Path to SQLite database file")
	url := flag.String("url", "https://www.bhplta.co.uk/bhplta_tables/fixtures/", "BHPLTA fixtures URL")
	numWeeks := flag.Int("weeks", 18, "Number of weeks in the season")
	dryRun := flag.Bool("dry-run", false, "Scrape and print what would be imported without writing to DB")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	flag.Parse()

	if *verbose {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}

	// Scrape fixtures
	log.Printf("Scraping fixtures from %s ...", *url)
	scrapedDivisions, err := services.ScrapeFixtures(*url)
	if err != nil {
		log.Fatalf("Failed to scrape fixtures: %v", err)
	}

	// Determine season year and date range from scraped fixtures
	var allDates []time.Time
	for _, d := range scrapedDivisions {
		for _, f := range d.Fixtures {
			if !f.Date.IsZero() {
				allDates = append(allDates, f.Date)
			}
		}
	}
	if len(allDates) == 0 {
		log.Fatal("No fixture dates found in scraped data")
	}
	sort.Slice(allDates, func(i, j int) bool { return allDates[i].Before(allDates[j]) })
	seasonStartDate := allDates[0]
	seasonYear := seasonStartDate.Year()
	// Season end is Sep 30 to allow for rescheduled fixtures
	seasonEndDate := time.Date(seasonYear, time.September, 30, 0, 0, 0, 0, time.UTC)

	// Print summary
	totalFixtures := 0
	totalTeams := 0
	for _, d := range scrapedDivisions {
		totalFixtures += len(d.Fixtures)
		totalTeams += len(d.Teams)
		fmt.Printf("\n%s (%d teams, %d fixtures)\n", d.Name, len(d.Teams), len(d.Fixtures))
		if *verbose {
			fmt.Println("  Teams:")
			for _, t := range d.Teams {
				fmt.Printf("    - %s\n", t)
			}
			fmt.Println("  Fixtures (first 5):")
			for i, f := range d.Fixtures {
				if i >= 5 {
					fmt.Printf("    ... and %d more\n", len(d.Fixtures)-5)
					break
				}
				fmt.Printf("    Week %d (%s): %s v %s\n", f.Week, f.Date.Format("02 Jan"), f.HomeTeam, f.AwayTeam)
			}
		}
	}
	fmt.Printf("\nTotal: %d divisions, %d teams, %d fixtures\n", len(scrapedDivisions), totalTeams, totalFixtures)
	fmt.Printf("Season: %d (%s to %s)\n", seasonYear, seasonStartDate.Format("02 Jan 2006"), seasonEndDate.Format("02 Jan 2006"))

	if *dryRun {
		log.Println("Dry run complete. No database changes made.")
		return
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

	// Run migrations
	if err := db.ExecuteMigrations("./migrations"); err != nil {
		log.Printf("Warning: Failed to run migrations: %v", err)
	}

	ctx := context.Background()

	// Initialize repositories
	seasonRepo := repository.NewSeasonRepository(db)
	divisionRepo := repository.NewDivisionRepository(db)
	teamRepo := repository.NewTeamRepository(db)
	clubRepo := repository.NewClubRepository(db)
	fixtureRepo := repository.NewFixtureRepository(db)
	weekRepo := repository.NewWeekRepository(db)

	// Find or create the target season
	season, err := findOrCreateSeason(ctx, seasonRepo, weekRepo, seasonYear, seasonStartDate, seasonEndDate, *numWeeks)
	if err != nil {
		log.Fatalf("Failed to find or create season: %v", err)
	}
	targetSeasonID := season.ID
	log.Printf("Target season: %s (ID %d, Year %d)", season.Name, season.ID, season.Year)

	// Get previous season
	previousYear := season.Year - 1
	previousSeasons, err := seasonRepo.FindByYear(ctx, previousYear)
	if err != nil || len(previousSeasons) == 0 {
		log.Fatalf("No season found for year %d", previousYear)
	}
	prevSeason := previousSeasons[0]
	log.Printf("Previous season: %s (Year %d)", prevSeason.Name, prevSeason.Year)

	// Get previous season's divisions for config
	prevDivisions, err := divisionRepo.FindBySeason(ctx, prevSeason.ID)
	if err != nil {
		log.Fatalf("Failed to find previous divisions: %v", err)
	}
	prevDivByName := make(map[string]models.Division)
	for _, d := range prevDivisions {
		prevDivByName[d.Name] = d
	}

	// Track results
	var (
		divisionsCreated int
		teamsCreated     int
		fixturesCreated  int
		playersCopied    int
		captainsCopied   int
		importErrors     []string
	)

	// Create divisions
	newDivByName := make(map[string]uint)
	for _, sd := range scrapedDivisions {
		newDiv := &models.Division{
			Name:     sd.Name,
			SeasonID: targetSeasonID,
		}
		if prevDiv, ok := prevDivByName[sd.Name]; ok {
			newDiv.Level = prevDiv.Level
			newDiv.PlayDay = prevDiv.PlayDay
			newDiv.LeagueID = prevDiv.LeagueID
			newDiv.MaxTeamsPerClub = prevDiv.MaxTeamsPerClub
		} else if len(prevDivisions) > 0 {
			newDiv.LeagueID = prevDivisions[0].LeagueID
		}

		if err := divisionRepo.Create(ctx, newDiv); err != nil {
			log.Fatalf("Failed to create division %s: %v", sd.Name, err)
		}
		newDivByName[sd.Name] = newDiv.ID
		divisionsCreated++
		log.Printf("Created division: %s (ID %d)", sd.Name, newDiv.ID)
	}

	// Get previous season teams for player/captain copying
	prevTeams, _ := teamRepo.FindBySeason(ctx, prevSeason.ID)
	prevTeamByName := make(map[string]models.Team)
	for _, t := range prevTeams {
		prevTeamByName[t.Name] = t
	}

	// Create teams
	teamIDMap := make(map[string]uint)
	for _, sd := range scrapedDivisions {
		divisionID := newDivByName[sd.Name]
		for _, teamName := range sd.Teams {
			clubName := repository.ExtractClubNameFromTeamName(teamName)
			// Normalize apostrophes then truncate before apostrophe for LIKE search
			searchName := normalize.Apostrophes(clubName)
			if idx := strings.IndexByte(searchName, '\''); idx > 0 {
				searchName = searchName[:idx]
			}
			clubs, err := clubRepo.FindByNameLike(ctx, searchName)
			if err != nil || len(clubs) == 0 {
				importErrors = append(importErrors, fmt.Sprintf("club not found for %q (looked for %q)", teamName, clubName))
				continue
			}

			newTeam := &models.Team{
				Name:       teamName,
				ClubID:     clubs[0].ID,
				DivisionID: divisionID,
				SeasonID:   targetSeasonID,
			}
			if err := teamRepo.Create(ctx, newTeam); err != nil {
				importErrors = append(importErrors, fmt.Sprintf("failed to create team %q: %v", teamName, err))
				continue
			}
			teamIDMap[teamName] = newTeam.ID
			teamsCreated++
			log.Printf("Created team: %s (ID %d) in %s", teamName, newTeam.ID, sd.Name)

			// Copy players and captains from previous season
			if prevTeam, ok := prevTeamByName[teamName]; ok {
				players, _ := teamRepo.FindPlayersInTeam(ctx, prevTeam.ID, prevSeason.ID)
				for _, pt := range players {
					if err := teamRepo.AddPlayer(ctx, newTeam.ID, pt.PlayerID, targetSeasonID); err == nil {
						playersCopied++
					}
				}
				captains, _ := teamRepo.FindCaptainsInTeam(ctx, prevTeam.ID, prevSeason.ID)
				for _, c := range captains {
					if err := teamRepo.AddCaptain(ctx, newTeam.ID, c.PlayerID, c.Role, targetSeasonID); err == nil {
						captainsCopied++
					}
				}
			}
		}
	}

	// Create fixtures
	for _, sd := range scrapedDivisions {
		divisionID := newDivByName[sd.Name]
		for _, sf := range sd.Fixtures {
			homeID, homeOk := teamIDMap[sf.HomeTeam]
			awayID, awayOk := teamIDMap[sf.AwayTeam]
			if !homeOk || !awayOk {
				importErrors = append(importErrors, fmt.Sprintf("skip fixture %s v %s: team not found", sf.HomeTeam, sf.AwayTeam))
				continue
			}

			week, err := weekRepo.FindByWeekNumber(ctx, targetSeasonID, sf.Week)
			if err != nil {
				importErrors = append(importErrors, fmt.Sprintf("week %d not found: %v", sf.Week, err))
				continue
			}

			fixture := &models.Fixture{
				HomeTeamID:    homeID,
				AwayTeamID:    awayID,
				DivisionID:    divisionID,
				SeasonID:      targetSeasonID,
				WeekID:        week.ID,
				ScheduledDate: sf.Date,
				VenueLocation: "TBD",
				Status:        models.Scheduled,
			}
			if err := fixtureRepo.Create(ctx, fixture); err != nil {
				importErrors = append(importErrors, fmt.Sprintf("failed to create fixture %s v %s: %v", sf.HomeTeam, sf.AwayTeam, err))
				continue
			}
			fixturesCreated++
		}
	}

	fmt.Printf("\nImport complete:\n")
	fmt.Printf("  Divisions created: %d\n", divisionsCreated)
	fmt.Printf("  Teams created:     %d\n", teamsCreated)
	fmt.Printf("  Fixtures created:  %d\n", fixturesCreated)
	fmt.Printf("  Players copied:    %d\n", playersCopied)
	fmt.Printf("  Captains copied:   %d\n", captainsCopied)
	if len(importErrors) > 0 {
		fmt.Printf("  Errors (%d):\n", len(importErrors))
		for _, e := range importErrors {
			fmt.Printf("    - %s\n", e)
		}
	}
}

// findOrCreateSeason looks up an existing season by year or creates a new one with weeks
func findOrCreateSeason(
	ctx context.Context,
	seasonRepo repository.SeasonRepository,
	weekRepo repository.WeekRepository,
	year int,
	startDate, endDate time.Time,
	numWeeks int,
) (*models.Season, error) {
	// Check if season already exists for this year
	seasons, err := seasonRepo.FindByYear(ctx, year)
	if err == nil && len(seasons) > 0 {
		// Verify it has weeks
		weekCount, _ := weekRepo.CountBySeason(ctx, seasons[0].ID)
		if weekCount > 0 {
			log.Printf("Found existing season: %s (ID %d) with %d weeks", seasons[0].Name, seasons[0].ID, weekCount)
			return &seasons[0], nil
		}
		// Season exists but has no weeks — create them
		log.Printf("Found existing season %s (ID %d) but no weeks — creating %d weeks", seasons[0].Name, seasons[0].ID, numWeeks)
		if err := createWeeks(ctx, weekRepo, seasons[0].ID, startDate, endDate, numWeeks); err != nil {
			return nil, fmt.Errorf("failed to create weeks: %w", err)
		}
		return &seasons[0], nil
	}

	// Create new season
	season := &models.Season{
		Name:      fmt.Sprintf("%d Season", year),
		Year:      year,
		StartDate: startDate,
		EndDate:   endDate,
		IsActive:  false,
	}

	if err := seasonRepo.Create(ctx, season); err != nil {
		return nil, fmt.Errorf("failed to create season: %w", err)
	}
	log.Printf("Created season: %s (ID %d)", season.Name, season.ID)

	// Create weeks
	if err := createWeeks(ctx, weekRepo, season.ID, startDate, endDate, numWeeks); err != nil {
		return nil, fmt.Errorf("failed to create weeks: %w", err)
	}

	return season, nil
}

// createWeeks generates evenly-spaced weeks for a season
func createWeeks(
	ctx context.Context,
	weekRepo repository.WeekRepository,
	seasonID uint,
	startDate, endDate time.Time,
	numWeeks int,
) error {
	totalDays := endDate.Sub(startDate).Hours() / 24
	daysPerWeek := totalDays / float64(numWeeks)

	for i := 1; i <= numWeeks; i++ {
		weekStart := startDate.AddDate(0, 0, int(float64(i-1)*daysPerWeek))
		weekEnd := startDate.AddDate(0, 0, int(float64(i)*daysPerWeek)-1)
		if i == numWeeks {
			weekEnd = endDate
		}

		week := &models.Week{
			WeekNumber: i,
			SeasonID:   seasonID,
			StartDate:  weekStart,
			EndDate:    weekEnd,
			Name:       fmt.Sprintf("Week %d", i),
			IsActive:   false,
		}

		if err := weekRepo.Create(ctx, week); err != nil {
			return fmt.Errorf("failed to create week %d: %w", i, err)
		}
		log.Printf("  Created week %d (%s to %s)", i, weekStart.Format("02 Jan"), weekEnd.Format("02 Jan"))
	}

	return nil
}
