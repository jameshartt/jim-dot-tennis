package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"

	"github.com/google/uuid"
	"golang.org/x/net/html"
)

type PopulateConfig struct {
	CSVDir               string
	DBPath               string
	DBType               string
	DryRun               bool
	Verbose              bool
	PlayersFile          string // New field for the players HTML file
	ProTennisPlayersFile string // New field for the tennis players JSON file
}

type TeamInfo struct {
	Name   string
	Club   string
	Suffix string
}

func main() {
	config := &PopulateConfig{}

	flag.StringVar(&config.CSVDir, "csv-dir", "pdf_output", "Directory containing CSV files")
	flag.StringVar(&config.DBPath, "db-path", "./tennis.db", "Path to SQLite database file")
	flag.StringVar(&config.DBType, "db-type", "sqlite3", "Database type (sqlite3 or postgres)")
	flag.BoolVar(&config.DryRun, "dry-run", false, "Print what would be done without making changes")
	flag.BoolVar(&config.Verbose, "verbose", false, "Enable verbose logging")
	flag.StringVar(&config.PlayersFile, "players-file", "players-import/players.html", "Path to the players HTML file")
	flag.StringVar(&config.ProTennisPlayersFile, "tennis-players-file", "cmd/collect_tennis_data/tennis_players.json", "Path to the tennis players JSON file")
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
	weekRepo := repository.NewWeekRepository(db)
	playerRepo := repository.NewPlayerRepository(db)

	// Create 2025 season
	season, err := createSeason(ctx, seasonRepo, config)
	if err != nil {
		return fmt.Errorf("failed to create season: %w", err)
	}

	// Create weeks for the season
	weeks, err := createWeeks(ctx, weekRepo, season, config)
	if err != nil {
		return fmt.Errorf("failed to create weeks: %w", err)
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
		if err := processCSVFile(ctx, csvPath, division, season, weeks, clubRepo, teamRepo, fixtureRepo, config); err != nil {
			return fmt.Errorf("failed to process CSV file %s: %w", csvFile, err)
		}
	}

	// Import players from HTML
	if err := importPlayers(ctx, config.PlayersFile, clubRepo, playerRepo, config); err != nil {
		return fmt.Errorf("failed to import players: %w", err)
	}

	// Import tennis players from JSON
	if err := importProTennisPlayers(ctx, config.ProTennisPlayersFile, config); err != nil {
		return fmt.Errorf("failed to import tennis players: %w", err)
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

func createWeeks(ctx context.Context, repo repository.WeekRepository, season *models.Season, config *PopulateConfig) (map[int]*models.Week, error) {
	weekMap := make(map[int]*models.Week)

	// Create 18 weeks for the season (typical tennis league season)
	for weekNum := 1; weekNum <= 18; weekNum++ {
		// Calculate week dates (assuming season starts April 1st, each week is 7 days)
		weekStart := season.StartDate.AddDate(0, 0, (weekNum-1)*7)
		weekEnd := weekStart.AddDate(0, 0, 6)

		week := &models.Week{
			WeekNumber: weekNum,
			SeasonID:   season.ID,
			StartDate:  weekStart,
			EndDate:    weekEnd,
			Name:       fmt.Sprintf("Week %d", weekNum),
			IsActive:   weekNum == 1, // First week is active by default
		}

		if config.DryRun {
			log.Printf("[DRY RUN] Would create week: %s (Week %d)", week.Name, week.WeekNumber)
			week.ID = uint(weekNum) // Mock ID for dry run
		} else {
			// Check if week already exists
			existing, err := repo.FindByWeekNumber(ctx, season.ID, weekNum)
			if err == nil && existing != nil {
				if config.Verbose {
					log.Printf("Week %d already exists, using existing week (ID: %d)", weekNum, existing.ID)
				}
				weekMap[weekNum] = existing
				continue
			}

			if err := repo.Create(ctx, week); err != nil {
				return nil, fmt.Errorf("failed to create week %d: %w", weekNum, err)
			}

			if config.Verbose {
				log.Printf("Created week: %s (ID: %d)", week.Name, week.ID)
			}
		}

		weekMap[weekNum] = week
	}

	return weekMap, nil
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

func processCSVFile(ctx context.Context, csvPath string, division *models.Division, season *models.Season, weeks map[int]*models.Week, clubRepo repository.ClubRepository, teamRepo repository.TeamRepository, fixtureRepo repository.FixtureRepository, config *PopulateConfig) error {
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

			weekObj := weeks[week]
			if weekObj == nil {
				log.Printf("Warning: Week %d not found, skipping fixture", week)
				continue
			}

			if err := createFixture(ctx, fixtureRepo, homeTeamID, awayTeamID, division.ID, season.ID, weekObj.ID, fixtureDate, week, config); err != nil {
				log.Printf("Warning: failed to create fixture %s vs %s: %v", record[2], record[3], err)
			}
		}

		// Second half fixtures
		if record[4] != "" && record[5] != "" {
			homeTeamID := clubTeamMap[record[4]]
			awayTeamID := clubTeamMap[record[5]]

			weekObj := weeks[week]
			if weekObj == nil {
				log.Printf("Warning: Week %d not found, skipping fixture", week)
				continue
			}

			if err := createFixture(ctx, fixtureRepo, homeTeamID, awayTeamID, division.ID, season.ID, weekObj.ID, fixtureDate, week, config); err != nil {
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

func createFixture(ctx context.Context, repo repository.FixtureRepository, homeTeamID, awayTeamID, divisionID, seasonID uint, weekID uint, fixtureDate time.Time, week int, config *PopulateConfig) error {
	if config.DryRun {
		log.Printf("[DRY RUN] Would create fixture: Home Team ID %d vs Away Team ID %d on %s (Week %d)", homeTeamID, awayTeamID, fixtureDate.Format("2006-01-02"), week)
		return nil
	}

	// Check if fixture already exists
	existing, err := repo.FindByDivisionAndSeason(ctx, divisionID, seasonID)
	if err == nil {
		for _, fixture := range existing {
			if fixture.HomeTeamID == homeTeamID && fixture.AwayTeamID == awayTeamID &&
				fixture.ScheduledDate.Format("2006-01-02") == fixtureDate.Format("2006-01-02") &&
				fixture.WeekID == weekID {
				if config.Verbose {
					log.Printf("Fixture already exists: Home Team ID %d vs Away Team ID %d on %s (Week %d)", homeTeamID, awayTeamID, fixtureDate.Format("2006-01-02"), week)
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
		WeekID:        weekID,
		ScheduledDate: fixtureDate,
		VenueLocation: "TBD", // To be determined
		Status:        models.Scheduled,
		Notes:         fmt.Sprintf("Week %d fixture", week),
	}

	if err := repo.Create(ctx, fixture); err != nil {
		return err
	}

	if config.Verbose {
		log.Printf("Created fixture: Home Team ID %d vs Away Team ID %d on %s (Week %d, ID: %d)",
			fixture.HomeTeamID, fixture.AwayTeamID, fixture.ScheduledDate.Format("2006-01-02"), week, fixture.ID)
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

// generateRandomEmail creates a random email address based on the player's name
func generateRandomEmail(firstName, lastName string) string {
	// Convert to lowercase and remove spaces
	first := strings.ToLower(strings.ReplaceAll(firstName, " ", ""))
	last := strings.ToLower(strings.ReplaceAll(lastName, " ", ""))

	// Random email domains
	domains := []string{"gmail.com", "yahoo.co.uk", "hotmail.com", "outlook.com", "btinternet.com"}
	domain := domains[len(first+last)%len(domains)]

	// Different email patterns
	patterns := []string{
		fmt.Sprintf("%s.%s@%s", first, last, domain),
		fmt.Sprintf("%s_%s@%s", first, last, domain),
		fmt.Sprintf("%s%s@%s", first, last, domain),
		fmt.Sprintf("%s.%s%d@%s", first, last, (len(firstName)+len(lastName))%99+1, domain),
	}

	pattern := patterns[len(firstName)%len(patterns)]
	return pattern
}

// generateRandomPhone creates a random UK phone number
func generateRandomPhone(firstName, lastName string) string {
	// Use name length to create some variation but keep it deterministic for same names
	seed := len(firstName) + len(lastName)*3

	// UK mobile number patterns
	prefixes := []string{"07700", "07701", "07702", "07703", "07704", "07705"}
	prefix := prefixes[seed%len(prefixes)]

	// Generate last 6 digits based on name
	suffix := fmt.Sprintf("%06d", (seed*12345+67890)%1000000)

	return fmt.Sprintf("%s %s", prefix, suffix)
}

// parsePlayersFromHTML extracts player names from the HTML file
func parsePlayersFromHTML(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open HTML file: %w", err)
	}
	defer file.Close()

	doc, err := html.Parse(file)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var players []string
	var extractText func(*html.Node)
	extractText = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "li" {
			// Look for the selectable option class
			hasSelectableClass := false
			for _, attr := range n.Attr {
				if attr.Key == "class" && strings.Contains(attr.Val, "select2-results__option--selectable") {
					hasSelectableClass = true
					break
				}
			}

			if hasSelectableClass {
				// Extract text content
				var textContent strings.Builder
				var getText func(*html.Node)
				getText = func(node *html.Node) {
					if node.Type == html.TextNode {
						textContent.WriteString(node.Data)
					}
					for child := node.FirstChild; child != nil; child = child.NextSibling {
						getText(child)
					}
				}
				getText(n)

				playerName := strings.TrimSpace(textContent.String())
				if playerName != "" {
					players = append(players, playerName)
				}
			}
		}

		for child := n.FirstChild; child != nil; child = child.NextSibling {
			extractText(child)
		}
	}

	extractText(doc)
	return players, nil
}

// filterPlayerNames removes non-player entries from the list
func filterPlayerNames(names []string) []string {
	// Define patterns to exclude
	excludePatterns := []string{
		"Select Player",
		"Player not listed",
		"enter manually",
		"Conceded by",
		"Given to",
		"St Ann's",
	}

	var filtered []string
	for _, name := range names {
		shouldExclude := false
		for _, pattern := range excludePatterns {
			if strings.Contains(name, pattern) {
				shouldExclude = true
				break
			}
		}

		// Also exclude if it's too short or contains suspicious patterns
		if !shouldExclude && len(strings.TrimSpace(name)) > 2 {
			// Check if it looks like a real name (has at least one letter)
			if matched, _ := regexp.MatchString(`[a-zA-Z]`, name); matched {
				filtered = append(filtered, strings.TrimSpace(name))
			}
		}
	}

	return filtered
}

// splitPlayerName splits a full name into first and last name
func splitPlayerName(fullName string) (string, string) {
	parts := strings.Fields(fullName)
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}

	// First name is the first part, last name is everything else joined
	firstName := parts[0]
	lastName := strings.Join(parts[1:], " ")
	return firstName, lastName
}

// importPlayers imports players from the HTML file into the database
func importPlayers(ctx context.Context, filePath string, clubRepo repository.ClubRepository, playerRepo repository.PlayerRepository, config *PopulateConfig) error {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		if config.Verbose {
			log.Printf("Players file %s not found, skipping player import", filePath)
		}
		return nil
	}

	if config.Verbose {
		log.Printf("Starting player import from %s", filePath)
	}

	var stAnnsClub models.Club

	if config.DryRun {
		// In dry run mode, create a mock club
		stAnnsClub = models.Club{ID: 1, Name: "St. Ann's"}
		if config.Verbose {
			log.Printf("[DRY RUN] Using mock St. Ann's club (ID: %d)", stAnnsClub.ID)
		}
	} else {
		// Find St. Ann's club
		clubs, err := clubRepo.FindByNameLike(ctx, "St Ann")
		if err != nil {
			return fmt.Errorf("failed to find St. Ann's club: %w", err)
		}
		if len(clubs) == 0 {
			return fmt.Errorf("St. Ann's club not found in database")
		}
		stAnnsClub = clubs[0]

		if config.Verbose {
			log.Printf("Found St. Ann's club (ID: %d)", stAnnsClub.ID)
		}
	}

	// Parse player names from HTML
	playerNames, err := parsePlayersFromHTML(filePath)
	if err != nil {
		return fmt.Errorf("failed to parse players from HTML: %w", err)
	}

	if config.Verbose {
		log.Printf("Extracted %d potential player names from HTML", len(playerNames))
	}

	// Filter out non-player entries
	filteredNames := filterPlayerNames(playerNames)

	if config.Verbose {
		log.Printf("Filtered to %d valid player names", len(filteredNames))
	}

	// Track imported players
	importedCount := 0
	skippedCount := 0

	// Create players
	for _, fullName := range filteredNames {
		firstName, lastName := splitPlayerName(fullName)

		if config.DryRun {
			email := generateRandomEmail(firstName, lastName)
			phone := generateRandomPhone(firstName, lastName)
			log.Printf("[DRY RUN] Would create player: %s %s (Club: %s, Email: %s, Phone: %s)", firstName, lastName, stAnnsClub.Name, email, phone)
			importedCount++
			continue
		}

		// Check if player already exists with the same name and club
		existing, err := playerRepo.FindByName(ctx, firstName, lastName)
		if err == nil {
			// Check if any existing player is from the same club
			playerExists := false
			for _, existingPlayer := range existing {
				if existingPlayer.ClubID == stAnnsClub.ID {
					playerExists = true
					break
				}
			}
			if playerExists {
				if config.Verbose {
					log.Printf("Player %s %s already exists in St. Ann's club, skipping", firstName, lastName)
				}
				skippedCount++
				continue
			}
		}

		// Create new player
		player := &models.Player{
			ID:        uuid.New().String(),
			FirstName: firstName,
			LastName:  lastName,
			Email:     generateRandomEmail(firstName, lastName),
			Phone:     generateRandomPhone(firstName, lastName),
			ClubID:    stAnnsClub.ID,
		}

		if err := playerRepo.Create(ctx, player); err != nil {
			log.Printf("Warning: failed to create player %s %s: %v", firstName, lastName, err)
			continue
		}

		if config.Verbose {
			log.Printf("Created player: %s %s (ID: %s)", firstName, lastName, player.ID)
		}
		importedCount++
	}

	log.Printf("Player import completed: %d imported, %d skipped", importedCount, skippedCount)
	return nil
}

// Import tennis players from JSON
func importProTennisPlayers(ctx context.Context, filePath string, config *PopulateConfig) error {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		if config.Verbose {
			log.Printf("Tennis players file %s not found, skipping tennis player import", filePath)
		}
		return nil
	}

	if config.Verbose {
		log.Printf("Starting tennis player import from %s", filePath)
	}

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

	// Initialize tennis player repository
	tennisPlayerRepo := repository.NewProTennisPlayerRepository(db)

	if config.DryRun {
		log.Printf("[DRY RUN] Would import tennis players from %s", filePath)
		return nil
	}

	// Import tennis players
	if err := tennisPlayerRepo.ImportFromJSON(ctx, filePath); err != nil {
		return fmt.Errorf("failed to import tennis players: %w", err)
	}

	// Get counts for reporting
	atpCount, err := tennisPlayerRepo.CountByTour(ctx, "ATP")
	if err != nil {
		log.Printf("Warning: Failed to count ATP players: %v", err)
	}

	wtaCount, err := tennisPlayerRepo.CountByTour(ctx, "WTA")
	if err != nil {
		log.Printf("Warning: Failed to count WTA players: %v", err)
	}

	if config.Verbose {
		log.Printf("Successfully imported %d ATP players and %d WTA players", atpCount, wtaCount)
	}

	return nil
}
