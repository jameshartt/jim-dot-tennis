package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
)

type PopulateConfig struct {
	CSVDir  string
	DBPath  string
	DBType  string
	DryRun  bool
	Verbose bool
}

type TeamInfo struct {
	Name   string
	Club   string
	Suffix string
}

func main() {
	config := &PopulateConfig{}

	flag.StringVar(&config.CSVDir, "csv-dir", "test_pdf_output_fixed", "Directory containing CSV files")
	flag.StringVar(&config.DBPath, "db-path", "./tennis.db", "Path to SQLite database file")
	flag.StringVar(&config.DBType, "db-type", "sqlite3", "Database type (sqlite3 or postgres)")
	flag.BoolVar(&config.DryRun, "dry-run", false, "Print what would be done without making changes")
	flag.BoolVar(&config.Verbose, "verbose", false, "Enable verbose logging")
	flag.Parse()

	if config.Verbose {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}

	if err := populateDatabase(config); err != nil {
		log.Fatalf("Failed to populate database: %v", err)
	}

	log.Println("Database population completed successfully!")
}

func populateDatabase(config *PopulateConfig) error {
	ctx := context.Background()

	// Connect to database
	dbConfig := database.Config{
		Driver:   config.DBType,
		FilePath: config.DBPath,
	}

	db, err := database.New(dbConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Run migrations first
	if !config.DryRun {
		migrationsPath := "./migrations"
		if err := db.ExecuteMigrations(migrationsPath); err != nil {
			log.Printf("Warning: Failed to run migrations: %v", err)
		}
	}

	// Initialize repositories
	seasonRepo := repository.NewSeasonRepository(db)
	leagueRepo := repository.NewLeagueRepository(db)
	divisionRepo := repository.NewDivisionRepository(db)
	clubRepo := repository.NewClubRepository(db)
	teamRepo := repository.NewTeamRepository(db)
	fixtureRepo := repository.NewFixtureRepository(db)

	// Create 2025 season
	season, err := createSeason(ctx, seasonRepo, config)
	if err != nil {
		return fmt.Errorf("failed to create season: %w", err)
	}

	// Create Parks League
	league, err := createLeague(ctx, leagueRepo, season, config)
	if err != nil {
		return fmt.Errorf("failed to create league: %w", err)
	}

	// Process each division CSV file
	csvFiles := []string{
		"Div_1_2025_fixtures.csv",
		"Div_2_2025_fixtures.csv",
		"Div_3_2025_fixtures.csv",
		"Div_4_2025_fixtures.csv",
	}

	for i, csvFile := range csvFiles {
		divisionLevel := i + 1
		divisionName := fmt.Sprintf("Division %d", divisionLevel)

		if config.Verbose {
			log.Printf("Processing %s (Level %d)", csvFile, divisionLevel)
		}

		// Create division
		division, err := createDivision(ctx, divisionRepo, league, season, divisionName, divisionLevel, config)
		if err != nil {
			return fmt.Errorf("failed to create division %s: %w", divisionName, err)
		}

		// Process CSV file
		csvPath := filepath.Join(config.CSVDir, csvFile)
		if err := processCSVFile(ctx, csvPath, division, season, clubRepo, teamRepo, fixtureRepo, config); err != nil {
			return fmt.Errorf("failed to process CSV file %s: %w", csvFile, err)
		}
	}

	return nil
}

func createSeason(ctx context.Context, repo repository.SeasonRepository, config *PopulateConfig) (*models.Season, error) {
	season := &models.Season{
		Name:      "2025 Season",
		Year:      2025,
		StartDate: time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2025, 9, 30, 0, 0, 0, 0, time.UTC),
		IsActive:  true,
	}

	if config.DryRun {
		log.Printf("[DRY RUN] Would create season: %s", season.Name)
		season.ID = 1 // Mock ID for dry run
		return season, nil
	}

	// Check if season already exists
	existing, err := repo.FindByYear(ctx, 2025)
	if err == nil && len(existing) > 0 {
		if config.Verbose {
			log.Printf("Season 2025 already exists, using existing season (ID: %d)", existing[0].ID)
		}
		return &existing[0], nil
	}

	if err := repo.Create(ctx, season); err != nil {
		return nil, err
	}

	if config.Verbose {
		log.Printf("Created season: %s (ID: %d)", season.Name, season.ID)
	}

	return season, nil
}

func createLeague(ctx context.Context, repo repository.LeagueRepository, season *models.Season, config *PopulateConfig) (*models.League, error) {
	league := &models.League{
		Name:   "Brighton & Hove Parks Tennis League",
		Type:   models.ParksLeague,
		Year:   2025,
		Region: "Brighton & Hove",
	}

	if config.DryRun {
		log.Printf("[DRY RUN] Would create league: %s", league.Name)
		league.ID = 1 // Mock ID for dry run
		return league, nil
	}

	// Check if league already exists
	existing, err := repo.FindByTypeAndYear(ctx, models.ParksLeague, 2025)
	if err == nil && len(existing) > 0 {
		if config.Verbose {
			log.Printf("League already exists, using existing league (ID: %d)", existing[0].ID)
		}
		return &existing[0], nil
	}

	if err := repo.Create(ctx, league); err != nil {
		return nil, err
	}

	// Associate league with season
	if err := repo.AddSeason(ctx, league.ID, season.ID); err != nil {
		return nil, fmt.Errorf("failed to associate league with season: %w", err)
	}

	if config.Verbose {
		log.Printf("Created league: %s (ID: %d)", league.Name, league.ID)
	}

	return league, nil
}

func createDivision(ctx context.Context, repo repository.DivisionRepository, league *models.League, season *models.Season, name string, level int, config *PopulateConfig) (*models.Division, error) {
	// Determine play day based on division level (this is a guess, can be adjusted)
	playDays := []string{"Thursday", "Tuesday", "Wednesday", "Monday"}
	playDay := playDays[(level-1)%len(playDays)]

	division := &models.Division{
		Name:            name,
		Level:           level,
		PlayDay:         playDay,
		LeagueID:        league.ID,
		SeasonID:        season.ID,
		MaxTeamsPerClub: 2, // Default for parks league
	}

	if config.DryRun {
		log.Printf("[DRY RUN] Would create division: %s (Level %d, Play Day: %s)", division.Name, division.Level, division.PlayDay)
		division.ID = uint(level) // Mock ID for dry run
		return division, nil
	}

	// Check if division already exists
	existing, err := repo.FindByLeagueAndSeason(ctx, league.ID, season.ID)
	if err == nil {
		for _, div := range existing {
			if div.Level == level {
				if config.Verbose {
					log.Printf("Division %s already exists, using existing division (ID: %d)", name, div.ID)
				}
				return &div, nil
			}
		}
	}

	if err := repo.Create(ctx, division); err != nil {
		return nil, err
	}

	if config.Verbose {
		log.Printf("Created division: %s (ID: %d, Level: %d, Play Day: %s)", division.Name, division.ID, division.Level, division.PlayDay)
	}

	return division, nil
}

func processCSVFile(ctx context.Context, csvPath string, division *models.Division, season *models.Season, clubRepo repository.ClubRepository, teamRepo repository.TeamRepository, fixtureRepo repository.FixtureRepository, config *PopulateConfig) error {
	file, err := os.Open(csvPath)
	if err != nil {
		return fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to read CSV: %w", err)
	}

	if len(records) < 2 {
		return fmt.Errorf("CSV file has no data rows")
	}

	// Skip header row
	records = records[1:]

	// Collect all unique teams first
	teamNames := make(map[string]bool)
	for _, record := range records {
		if len(record) < 6 {
			continue
		}

		// First half fixtures
		if record[2] != "" && record[3] != "" {
			teamNames[record[2]] = true // Home team
			teamNames[record[3]] = true // Away team
		}

		// Second half fixtures
		if record[4] != "" && record[5] != "" {
			teamNames[record[4]] = true // Home team
			teamNames[record[5]] = true // Away team
		}
	}

	// Create clubs and teams
	clubTeamMap := make(map[string]uint) // team name -> team ID
	for teamName := range teamNames {
		teamInfo := parseTeamName(teamName)

		// Create or get club
		club, err := createOrGetClub(ctx, clubRepo, teamInfo.Club, config)
		if err != nil {
			return fmt.Errorf("failed to create club %s: %w", teamInfo.Club, err)
		}

		// Create team
		team, err := createOrGetTeam(ctx, teamRepo, teamName, club.ID, division.ID, season.ID, config)
		if err != nil {
			return fmt.Errorf("failed to create team %s: %w", teamName, err)
		}

		clubTeamMap[teamName] = team.ID
	}

	// Create fixtures
	for _, record := range records {
		if len(record) < 6 {
			continue
		}

		week, err := strconv.Atoi(record[0])
		if err != nil {
			continue
		}

		dateStr := record[1]
		fixtureDate, err := parseFixtureDate(dateStr, 2025)
		if err != nil {
			log.Printf("Warning: failed to parse date %s: %v", dateStr, err)
			continue
		}

		// First half fixtures
		if record[2] != "" && record[3] != "" {
			homeTeamID := clubTeamMap[record[2]]
			awayTeamID := clubTeamMap[record[3]]

			if err := createFixture(ctx, fixtureRepo, homeTeamID, awayTeamID, division.ID, season.ID, fixtureDate, week, config); err != nil {
				log.Printf("Warning: failed to create fixture %s vs %s: %v", record[2], record[3], err)
			}
		}

		// Second half fixtures
		if record[4] != "" && record[5] != "" {
			homeTeamID := clubTeamMap[record[4]]
			awayTeamID := clubTeamMap[record[5]]

			if err := createFixture(ctx, fixtureRepo, homeTeamID, awayTeamID, division.ID, season.ID, fixtureDate, week, config); err != nil {
				log.Printf("Warning: failed to create fixture %s vs %s: %v", record[4], record[5], err)
			}
		}
	}

	return nil
}

func parseTeamName(teamName string) TeamInfo {
	// Extract club name and suffix
	// Examples: "Dyke A" -> Club: "Dyke", Suffix: "A"
	//          "King Alfred" -> Club: "King Alfred", Suffix: ""
	//          "St Ann's" -> Club: "St Ann's", Suffix: ""

	parts := strings.Fields(teamName)
	if len(parts) == 0 {
		return TeamInfo{Name: teamName, Club: teamName, Suffix: ""}
	}

	// Check if last part is a single letter (team suffix)
	lastPart := parts[len(parts)-1]
	if len(lastPart) == 1 && lastPart >= "A" && lastPart <= "Z" {
		// Has suffix
		club := strings.Join(parts[:len(parts)-1], " ")
		return TeamInfo{
			Name:   teamName,
			Club:   club,
			Suffix: lastPart,
		}
	}

	// No suffix
	return TeamInfo{
		Name:   teamName,
		Club:   teamName,
		Suffix: "",
	}
}

func createOrGetClub(ctx context.Context, repo repository.ClubRepository, clubName string, config *PopulateConfig) (*models.Club, error) {
	if config.DryRun {
		log.Printf("[DRY RUN] Would create/get club: %s", clubName)
		return &models.Club{ID: 1, Name: clubName}, nil
	}

	// Check if club already exists
	existing, err := repo.FindByName(ctx, clubName)
	if err == nil && len(existing) > 0 {
		if config.Verbose {
			log.Printf("Club %s already exists (ID: %d)", clubName, existing[0].ID)
		}
		return &existing[0], nil
	}

	// Create new club with default values
	club := &models.Club{
		Name:        clubName,
		Address:     fmt.Sprintf("%s Tennis Club, Brighton & Hove", clubName),
		Website:     "",
		PhoneNumber: "",
	}

	if err := repo.Create(ctx, club); err != nil {
		return nil, err
	}

	if config.Verbose {
		log.Printf("Created club: %s (ID: %d)", club.Name, club.ID)
	}

	return club, nil
}

func createOrGetTeam(ctx context.Context, repo repository.TeamRepository, teamName string, clubID, divisionID, seasonID uint, config *PopulateConfig) (*models.Team, error) {
	if config.DryRun {
		log.Printf("[DRY RUN] Would create/get team: %s", teamName)
		return &models.Team{ID: 1, Name: teamName}, nil
	}

	// Check if team already exists
	existing, err := repo.FindByDivisionAndSeason(ctx, divisionID, seasonID)
	if err == nil {
		for _, team := range existing {
			if team.Name == teamName {
				if config.Verbose {
					log.Printf("Team %s already exists (ID: %d)", teamName, team.ID)
				}
				return &team, nil
			}
		}
	}

	// Create new team
	team := &models.Team{
		Name:       teamName,
		ClubID:     clubID,
		DivisionID: divisionID,
		SeasonID:   seasonID,
	}

	if err := repo.Create(ctx, team); err != nil {
		return nil, err
	}

	if config.Verbose {
		log.Printf("Created team: %s (ID: %d)", team.Name, team.ID)
	}

	return team, nil
}

func createFixture(ctx context.Context, repo repository.FixtureRepository, homeTeamID, awayTeamID, divisionID, seasonID uint, fixtureDate time.Time, week int, config *PopulateConfig) error {
	if config.DryRun {
		log.Printf("[DRY RUN] Would create fixture: Home Team ID %d vs Away Team ID %d on %s", homeTeamID, awayTeamID, fixtureDate.Format("2006-01-02"))
		return nil
	}

	// Check if fixture already exists
	existing, err := repo.FindByDivisionAndSeason(ctx, divisionID, seasonID)
	if err == nil {
		for _, fixture := range existing {
			if fixture.HomeTeamID == homeTeamID && fixture.AwayTeamID == awayTeamID &&
				fixture.ScheduledDate.Format("2006-01-02") == fixtureDate.Format("2006-01-02") {
				if config.Verbose {
					log.Printf("Fixture already exists: Home Team ID %d vs Away Team ID %d on %s", homeTeamID, awayTeamID, fixtureDate.Format("2006-01-02"))
				}
				return nil
			}
		}
	}

	fixture := &models.Fixture{
		HomeTeamID:    homeTeamID,
		AwayTeamID:    awayTeamID,
		DivisionID:    divisionID,
		SeasonID:      seasonID,
		ScheduledDate: fixtureDate,
		VenueLocation: "TBD", // To be determined
		Status:        models.Scheduled,
		Notes:         fmt.Sprintf("Week %d fixture", week),
	}

	if err := repo.Create(ctx, fixture); err != nil {
		return err
	}

	if config.Verbose {
		log.Printf("Created fixture: Home Team ID %d vs Away Team ID %d on %s (ID: %d)",
			fixture.HomeTeamID, fixture.AwayTeamID, fixture.ScheduledDate.Format("2006-01-02"), fixture.ID)
	}

	return nil
}

func parseFixtureDate(dateStr string, year int) (time.Time, error) {
	// Parse dates like "April 17", "June 19", etc.
	// Add the year to make it a complete date

	fullDateStr := fmt.Sprintf("%s %d", dateStr, year)

	// Try different date formats
	formats := []string{
		"January 2 2006",
		"February 2 2006",
		"March 2 2006",
		"April 2 2006",
		"May 2 2006",
		"June 2 2006",
		"July 2 2006",
		"August 2 2006",
		"September 2 2006",
		"October 2 2006",
		"November 2 2006",
		"December 2 2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, fullDateStr); err == nil {
			return t, nil
		}
	}

	// Try with short month names
	shortFormats := []string{
		"Jan 2 2006",
		"Feb 2 2006",
		"Mar 2 2006",
		"Apr 2 2006",
		"May 2 2006",
		"Jun 2 2006",
		"Jul 2 2006",
		"Aug 2 2006",
		"Sep 2 2006",
		"Oct 2 2006",
		"Nov 2 2006",
		"Dec 2 2006",
	}

	for _, format := range shortFormats {
		if t, err := time.Parse(format, fullDateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", dateStr)
}
