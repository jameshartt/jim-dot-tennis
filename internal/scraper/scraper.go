package scraper

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
)

// Scraper is a service to scrape tennis league data
type Scraper struct {
	db                *database.DB
	clubRepo          *repository.ClubRepository
	seasonRepo        *repository.SeasonRepository
	leagueRepo        *repository.LeagueRepository
	divisionRepo      *repository.DivisionRepository
	teamRepo          *repository.TeamRepository
	fixtureRepo       *repository.FixtureRepository
	playerRepo        *repository.PlayerRepository
	clubNameMap       map[string]uint // Maps club name -> club ID
	teamNameMap       map[string]uint // Maps team name -> team ID
	divisionNameMap   map[string]uint // Maps division name -> division ID
}

// ImportConfig holds configuration for the import process
type ImportConfig struct {
	FixturesURL string
	ResultsURL  string
	TablesURL   string
	SeasonYear  int
	SeasonName  string
}

// NewScraper creates a new scraper instance
func NewScraper(db *database.DB) *Scraper {
	return &Scraper{
		db:              db,
		clubRepo:        repository.NewClubRepository(db),
		seasonRepo:      repository.NewSeasonRepository(db),
		leagueRepo:      repository.NewLeagueRepository(db),
		divisionRepo:    repository.NewDivisionRepository(db),
		teamRepo:        repository.NewTeamRepository(db),
		fixtureRepo:     repository.NewFixtureRepository(db),
		playerRepo:      repository.NewPlayerRepository(db),
		clubNameMap:     make(map[string]uint),
		teamNameMap:     make(map[string]uint),
		divisionNameMap: make(map[string]uint),
	}
}

// ImportData is the main function to import data from the website
func (s *Scraper) ImportData(ctx context.Context, config ImportConfig) error {
	// Step 1: Initialize or get the season
	season, err := s.getOrCreateSeason(ctx, config.SeasonYear, config.SeasonName)
	if err != nil {
		return fmt.Errorf("failed to get or create season: %w", err)
	}
	log.Printf("Using season: %s (ID: %d)", season.Name, season.ID)

	// Step 2: Initialize or get the league
	league, err := s.getOrCreateLeague(ctx, "Parks Tennis League", "Parks", config.SeasonYear, "Brighton and Hove")
	if err != nil {
		return fmt.Errorf("failed to get or create league: %w", err)
	}
	log.Printf("Using league: %s (ID: %d)", league.Name, league.ID)

	// Step 3: Associate the league with the season if needed
	err = s.associateLeagueWithSeason(ctx, league.ID, season.ID)
	if err != nil {
		return fmt.Errorf("failed to associate league with season: %w", err)
	}

	// Step 4: Scrape divisions and teams
	err = s.scrapeFixturesPage(ctx, config.FixturesURL, league.ID, season.ID)
	if err != nil {
		return fmt.Errorf("failed to scrape fixtures page: %w", err)
	}

	// Step 5: Scrape league tables for standings
	err = s.scrapeTablesPage(ctx, config.TablesURL, season.ID)
	if err != nil {
		return fmt.Errorf("failed to scrape tables page: %w", err)
	}

	// Step 6: Scrape results
	err = s.scrapeResultsPage(ctx, config.ResultsURL, season.ID)
	if err != nil {
		return fmt.Errorf("failed to scrape results page: %w", err)
	}

	return nil
}

// scrapeFixturesPage scrapes the fixtures page for divisions, teams, and fixtures
func (s *Scraper) scrapeFixturesPage(ctx context.Context, url string, leagueID, seasonID uint) error {
	log.Printf("Scraping fixtures from %s", url)
	
	// Fetch the document
	doc, err := s.fetchDocument(url)
	if err != nil {
		return err
	}

	// Find each division section
	var currentDivision *models.Division
	
	// Process each heading and the tables that follow it
	doc.Find("h2, h2 + table, h2 + p + table").Each(func(i int, sel *goquery.Selection) {
		// If this is a heading, it's a division name
		if sel.Is("h2") {
			divName := strings.TrimSpace(sel.Text())
			if strings.HasPrefix(divName, "Division") {
				// Extract division level
				level, _ := strconv.Atoi(strings.TrimSpace(divName[8:]))
				
				// Create or get division
				div, err := s.getOrCreateDivision(ctx, divName, level, "Thursday", leagueID, seasonID)
				if err != nil {
					log.Printf("Failed to create division %s: %v", divName, err)
					return
				}
				
				currentDivision = div
				log.Printf("Processing division: %s (ID: %d)", div.Name, div.ID)
			}
		} else if sel.Is("table") && currentDivision != nil {
			// This is a fixtures table, find the week header first
			weekHeader := sel.Find("tbody > tr:first-child td:first-child").Text()
			weekHeader = strings.TrimSpace(weekHeader)
			
			var weekMatch = regexp.MustCompile(`Week (\d+)`)
			var dateMatch = regexp.MustCompile(`(\d{2} [A-Za-z]{3} \d{4})`)
			
			var weekNumber int
			var fixtureDate time.Time
			
			// Extract week number and date
			if matches := weekMatch.FindStringSubmatch(weekHeader); len(matches) > 1 {
				weekNumber, _ = strconv.Atoi(matches[1])
			}
			
			if matches := dateMatch.FindStringSubmatch(weekHeader); len(matches) > 1 {
				// Parse date like "22 May 2025"
				fixtureDate, err = time.Parse("02 Jan 2006", matches[1])
				if err != nil {
					log.Printf("Failed to parse date %s: %v", matches[1], err)
				}
			}
			
			log.Printf("Processing fixtures for week %d (%s)", weekNumber, fixtureDate.Format("2006-01-02"))
			
			// Now parse each fixture row
			sel.Find("tbody > tr").Each(func(j int, row *goquery.Selection) {
				// Skip first row (header)
				if j == 0 {
					return
				}
				
				columns := row.Find("td")
				if columns.Length() >= 3 {
					homeTeamCell := columns.Eq(0)
					vsCell := columns.Eq(1)
					awayTeamCell := columns.Eq(2)
					
					homeTeamName := strings.TrimSpace(homeTeamCell.Text())
					awayTeamName := strings.TrimSpace(awayTeamCell.Text())
					
					// Clean up team names
					homeTeamName = strings.TrimSuffix(homeTeamName, " ")
					awayTeamName = strings.TrimSuffix(awayTeamName, " ")
					
					if homeTeamName == "" || awayTeamName == "" || vsCell.Text() != "v" {
						return
					}
					
					// Try to extract the club name and team identifier
					homeClubName, _ := s.parseTeamName(homeTeamName)
					awayClubName, _ := s.parseTeamName(awayTeamName)
					
					// Create or get clubs
					homeClub, err := s.getOrCreateClub(ctx, homeClubName)
					if err != nil {
						log.Printf("Failed to create home club %s: %v", homeClubName, err)
						return
					}
					
					awayClub, err := s.getOrCreateClub(ctx, awayClubName)
					if err != nil {
						log.Printf("Failed to create away club %s: %v", awayClubName, err)
						return
					}
					
					// Create or get teams
					homeTeam, err := s.getOrCreateTeam(ctx, homeTeamName, homeClub.ID, currentDivision.ID, seasonID)
					if err != nil {
						log.Printf("Failed to create home team %s: %v", homeTeamName, err)
						return
					}
					
					awayTeam, err := s.getOrCreateTeam(ctx, awayTeamName, awayClub.ID, currentDivision.ID, seasonID)
					if err != nil {
						log.Printf("Failed to create away team %s: %v", awayTeamName, err)
						return
					}
					
					// Create fixture
					fixture := &models.Fixture{
						HomeTeamID:    homeTeam.ID,
						AwayTeamID:    awayTeam.ID,
						DivisionID:    currentDivision.ID,
						SeasonID:      seasonID,
						ScheduledDate: fixtureDate,
						VenueLocation: fmt.Sprintf("%s Courts", homeClubName),
						Status:        models.Scheduled,
					}
					
					// Skip if the fixture already exists
					exists, err := s.fixtureExists(ctx, fixture)
					if err != nil {
						log.Printf("Failed to check if fixture exists: %v", err)
						return
					}
					
					if !exists {
						err = s.fixtureRepo.Create(ctx, fixture)
						if err != nil {
							log.Printf("Failed to create fixture: %v", err)
							return
						}
						log.Printf("Created fixture: %s vs %s on %s (ID: %d)", 
							homeTeamName, awayTeamName, fixtureDate.Format("2006-01-02"), fixture.ID)
						
						// Create default matchups for this fixture
						s.createDefaultMatchups(ctx, fixture.ID)
					}
				}
			})
		}
	})
	
	return nil
}

// scrapeTablesPage scrapes the league tables page for standings
func (s *Scraper) scrapeTablesPage(ctx context.Context, url string, seasonID uint) error {
	// This would scrape the league tables, but we'll implement it after the fixtures
	// since it will complement the data
	log.Printf("Skipping tables scraping - not implemented yet")
	return nil
}

// scrapeResultsPage scrapes the results page to update fixture statuses and scores
func (s *Scraper) scrapeResultsPage(ctx context.Context, url string, seasonID uint) error {
	// This would scrape the match results, but we'll implement it after the fixtures
	// since it depends on the fixtures existing
	log.Printf("Skipping results scraping - not implemented yet")
	return nil
}

// getOrCreateSeason retrieves or creates a season
func (s *Scraper) getOrCreateSeason(ctx context.Context, year int, name string) (*models.Season, error) {
	// Try to find the season by year
	seasons, err := s.seasonRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	
	for _, season := range seasons {
		if season.Year == year {
			return &season, nil
		}
	}
	
	// Create a new season
	startDate := time.Date(year, time.May, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(year, time.September, 30, 0, 0, 0, 0, time.UTC)
	
	newSeason := &models.Season{
		Name:      name,
		Year:      year,
		StartDate: startDate,
		EndDate:   endDate,
		IsActive:  true,
	}
	
	err = s.seasonRepo.Create(ctx, newSeason)
	if err != nil {
		return nil, err
	}
	
	return newSeason, nil
}

// getOrCreateLeague retrieves or creates a league
func (s *Scraper) getOrCreateLeague(ctx context.Context, name string, leagueType string, year int, region string) (*models.League, error) {
	// Try to find the league by name and year
	leagues, err := s.leagueRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	
	for _, league := range leagues {
		if league.Name == name && league.Year == year {
			return &league, nil
		}
	}
	
	// Create a new league
	newLeague := &models.League{
		Name:   name,
		Type:   models.LeagueType(leagueType),
		Year:   year,
		Region: region,
	}
	
	err = s.leagueRepo.Create(ctx, newLeague)
	if err != nil {
		return nil, err
	}
	
	return newLeague, nil
}

// getOrCreateDivision retrieves or creates a division
func (s *Scraper) getOrCreateDivision(ctx context.Context, name string, level int, playDay string, leagueID, seasonID uint) (*models.Division, error) {
	// Check if we already have this division in our map
	if divID, ok := s.divisionNameMap[name]; ok {
		div, err := s.divisionRepo.GetByID(ctx, divID)
		if err == nil {
			return div, nil
		}
	}
	
	// Try to find the division by name, league ID, and season ID
	divisions, err := s.divisionRepo.ListByLeagueAndSeason(ctx, leagueID, seasonID)
	if err != nil {
		return nil, err
	}
	
	for _, div := range divisions {
		if div.Name == name {
			s.divisionNameMap[name] = div.ID
			return &div, nil
		}
	}
	
	// Create a new division
	newDivision := &models.Division{
		Name:           name,
		Level:          level,
		PlayDay:        playDay,
		LeagueID:       leagueID,
		SeasonID:       seasonID,
		MaxTeamsPerClub: 2,
	}
	
	err = s.divisionRepo.Create(ctx, newDivision)
	if err != nil {
		return nil, err
	}
	
	s.divisionNameMap[name] = newDivision.ID
	return newDivision, nil
}

// getOrCreateClub retrieves or creates a club
func (s *Scraper) getOrCreateClub(ctx context.Context, name string) (*models.Club, error) {
	// Check if we already have this club in our map
	if clubID, ok := s.clubNameMap[name]; ok {
		club, err := s.clubRepo.GetByID(ctx, clubID)
		if err == nil {
			return club, nil
		}
	}
	
	// Try to find the club by name
	clubs, err := s.clubRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	
	for _, club := range clubs {
		if club.Name == name {
			s.clubNameMap[name] = club.ID
			return &club, nil
		}
	}
	
	// Create a new club
	newClub := &models.Club{
		Name:    name,
		Address: fmt.Sprintf("%s Tennis Club, Brighton & Hove", name),
		Website: fmt.Sprintf("https://www.bhplta.co.uk/clubs/%s/", strings.ToLower(strings.ReplaceAll(name, " ", "-"))),
	}
	
	err = s.clubRepo.Create(ctx, newClub)
	if err != nil {
		return nil, err
	}
	
	s.clubNameMap[name] = newClub.ID
	return newClub, nil
}

// getOrCreateTeam retrieves or creates a team
func (s *Scraper) getOrCreateTeam(ctx context.Context, name string, clubID, divisionID, seasonID uint) (*models.Team, error) {
	// Check if we already have this team in our map
	if teamID, ok := s.teamNameMap[name]; ok {
		team, err := s.teamRepo.GetByID(ctx, teamID)
		if err == nil {
			return team, nil
		}
	}
	
	// Try to find the team by name, club ID, division ID, and season ID
	teams, err := s.teamRepo.ListByClubDivisionSeason(ctx, clubID, divisionID, seasonID)
	if err != nil {
		return nil, err
	}
	
	for _, team := range teams {
		if team.Name == name {
			s.teamNameMap[name] = team.ID
			return &team, nil
		}
	}
	
	// Create a new team
	newTeam := &models.Team{
		Name:       name,
		ClubID:     clubID,
		DivisionID: divisionID,
		SeasonID:   seasonID,
	}
	
	err = s.teamRepo.Create(ctx, newTeam)
	if err != nil {
		return nil, err
	}
	
	s.teamNameMap[name] = newTeam.ID
	return newTeam, nil
}

// associateLeagueWithSeason associates a league with a season if not already done
func (s *Scraper) associateLeagueWithSeason(ctx context.Context, leagueID, seasonID uint) error {
	query := `
		SELECT COUNT(*) FROM league_seasons 
		WHERE league_id = ? AND season_id = ?
	`
	
	var count int
	err := s.db.GetContext(ctx, &count, query, leagueID, seasonID)
	if err != nil {
		return err
	}
	
	if count == 0 {
		// Insert the association
		query = `
			INSERT INTO league_seasons (league_id, season_id, created_at, updated_at)
			VALUES (?, ?, ?, ?)
		`
		
		now := time.Now()
		_, err = s.db.ExecContext(ctx, query, leagueID, seasonID, now, now)
		if err != nil {
			return err
		}
		
		log.Printf("Associated league ID %d with season ID %d", leagueID, seasonID)
	}
	
	return nil
}

// createDefaultMatchups creates the default four matchups for a fixture
func (s *Scraper) createDefaultMatchups(ctx context.Context, fixtureID uint) error {
	matchupTypes := []models.MatchupType{
		models.Mens,
		models.Womens,
		models.FirstMixed,
		models.SecondMixed,
	}
	
	for _, matchupType := range matchupTypes {
		matchup := &models.Matchup{
			FixtureID: fixtureID,
			Type:      matchupType,
			Status:    models.Pending,
		}
		
		query := `
			INSERT INTO matchups (fixture_id, type, status, home_score, away_score, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`
		
		now := time.Now()
		_, err := s.db.ExecContext(ctx, query, 
			matchup.FixtureID, 
			matchup.Type, 
			matchup.Status, 
			matchup.HomeScore, 
			matchup.AwayScore, 
			now, now)
			
		if err != nil {
			return err
		}
	}
	
	return nil
}

// fixtureExists checks if a fixture already exists
func (s *Scraper) fixtureExists(ctx context.Context, fixture *models.Fixture) (bool, error) {
	query := `
		SELECT COUNT(*) FROM fixtures 
		WHERE home_team_id = ? AND away_team_id = ? 
		AND division_id = ? AND season_id = ?
		AND DATE(scheduled_date) = DATE(?)
	`
	
	var count int
	err := s.db.GetContext(ctx, &count, query, 
		fixture.HomeTeamID, 
		fixture.AwayTeamID, 
		fixture.DivisionID, 
		fixture.SeasonID,
		fixture.ScheduledDate)
		
	if err != nil {
		return false, err
	}
	
	return count > 0, nil
}

// parseTeamName extracts the club name and team letter from a team name
func (s *Scraper) parseTeamName(teamName string) (string, string) {
	// Handle cases like "Hove A", "Dyke Park", "St Ann's D"
	teamName = strings.TrimSpace(teamName)
	
	// Check if the team name ends with a single letter (A-Z)
	if len(teamName) > 2 && teamName[len(teamName)-2] == ' ' {
		lastChar := teamName[len(teamName)-1]
		if lastChar >= 'A' && lastChar <= 'Z' {
			return strings.TrimSpace(teamName[:len(teamName)-2]), string(lastChar)
		}
	}
	
	// No team letter found, return the whole name as club name and empty team letter
	return teamName, ""
}

// fetchDocument fetches an HTML document from a URL
func (s *Scraper) fetchDocument(url string) (*goquery.Document, error) {
	// Create an HTTP client with a timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	
	// Create a request with a realistic user agent
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set a realistic user agent to avoid being blocked
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Cache-Control", "max-age=0")
	
	// Make the HTTP request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL %s: %w", url, err)
	}
	defer resp.Body.Close()
	
	// Check if the response was successful
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad response from server: %s", resp.Status)
	}
	
	// Parse the HTML document
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}
	
	return doc, nil
} 