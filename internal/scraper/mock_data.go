package scraper

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"jim-dot-tennis/internal/models"
)

// ImportMockData imports mock tennis league data
func (s *Scraper) ImportMockData(ctx context.Context, seasonYear int, seasonName string) error {
	// Step 1: Initialize or get the season
	season, err := s.getOrCreateSeason(ctx, seasonYear, seasonName)
	if err != nil {
		return fmt.Errorf("failed to get or create season: %w", err)
	}
	log.Printf("Using season: %s (ID: %d)", season.Name, season.ID)

	// Step 2: Initialize or get the league
	league, err := s.getOrCreateLeague(ctx, "Brighton & Hove Parks Tennis League", "Parks", seasonYear, "Brighton and Hove")
	if err != nil {
		return fmt.Errorf("failed to get or create league: %w", err)
	}
	log.Printf("Using league: %s (ID: %d)", league.Name, league.ID)

	// Step 3: Associate the league with the season if needed
	err = s.associateLeagueWithSeason(ctx, league.ID, season.ID)
	if err != nil {
		return fmt.Errorf("failed to associate league with season: %w", err)
	}

	// Step 4: Create divisions
	divisions := []struct {
		Name  string
		Level int
	}{
		{"Division 1", 1},
		{"Division 2", 2},
		{"Division 3", 3},
		{"Division 4", 4},
	}

	for _, div := range divisions {
		division, err := s.getOrCreateDivision(ctx, div.Name, div.Level, "Thursday", league.ID, season.ID)
		if err != nil {
			return fmt.Errorf("failed to create division %s: %w", div.Name, err)
		}
		log.Printf("Created division: %s (ID: %d)", division.Name, division.ID)
	}

	// Step 5: Create clubs
	clubs := []string{
		"Dyke Park",
		"Hove Park",
		"King Alfred",
		"Queens Park",
		"Preston Park",
		"Saltdean",
		"St Ann's",
		"Blakers",
		"Hollingbury Park",
		"Park Avenue",
		"Rookery",
	}

	for _, clubName := range clubs {
		club, err := s.getOrCreateClub(ctx, clubName)
		if err != nil {
			return fmt.Errorf("failed to create club %s: %w", clubName, err)
		}
		log.Printf("Created club: %s (ID: %d)", club.Name, club.ID)
	}

	// Step 6: Create teams
	// Division 1 teams
	div1Teams := []string{
		"Dyke Park A", "Dyke Park", "Hove Park A", "Hove Park", "King Alfred", 
		"Queens Park", "Queens Park A", "Preston Park", "St Ann's", "Saltdean",
	}
	div1, err := s.getDivisionByName(ctx, "Division 1", league.ID, season.ID)
	if err != nil {
		return fmt.Errorf("failed to get Division 1: %w", err)
	}

	for _, teamName := range div1Teams {
		clubName, _ := s.parseTeamName(teamName)
		club, err := s.getClubByName(ctx, clubName)
		if err != nil {
			return fmt.Errorf("failed to get club %s: %w", clubName, err)
		}
		
		team, err := s.getOrCreateTeam(ctx, teamName, club.ID, div1.ID, season.ID)
		if err != nil {
			return fmt.Errorf("failed to create team %s: %w", teamName, err)
		}
		log.Printf("Created team: %s (ID: %d) in Division 1", team.Name, team.ID)
	}

	// Division 2 teams
	div2Teams := []string{
		"Blakers", "Dyke Park B", "Hollingbury Park", "Hove Park B", 
		"King Alfred A", "Park Avenue", "Preston Park A", 
		"Queens Park B", "St Ann's A", "Saltdean A",
	}
	div2, err := s.getDivisionByName(ctx, "Division 2", league.ID, season.ID)
	if err != nil {
		return fmt.Errorf("failed to get Division 2: %w", err)
	}

	for _, teamName := range div2Teams {
		clubName, _ := s.parseTeamName(teamName)
		club, err := s.getClubByName(ctx, clubName)
		if err != nil {
			return fmt.Errorf("failed to get club %s: %w", clubName, err)
		}
		
		team, err := s.getOrCreateTeam(ctx, teamName, club.ID, div2.ID, season.ID)
		if err != nil {
			return fmt.Errorf("failed to create team %s: %w", teamName, err)
		}
		log.Printf("Created team: %s (ID: %d) in Division 2", team.Name, team.ID)
	}

	// Division 3 teams
	div3Teams := []string{
		"Dyke Park C", "Hove Park C", "Hove Park D", "King Alfred D", 
		"Preston Park B", "Queens Park C", "Queens Park D", 
		"Rookery", "Saltdean B", "St Ann's B",
	}
	div3, err := s.getDivisionByName(ctx, "Division 3", league.ID, season.ID)
	if err != nil {
		return fmt.Errorf("failed to get Division 3: %w", err)
	}

	for _, teamName := range div3Teams {
		clubName, _ := s.parseTeamName(teamName)
		club, err := s.getClubByName(ctx, clubName)
		if err != nil {
			return fmt.Errorf("failed to get club %s: %w", clubName, err)
		}
		
		team, err := s.getOrCreateTeam(ctx, teamName, club.ID, div3.ID, season.ID)
		if err != nil {
			return fmt.Errorf("failed to create team %s: %w", teamName, err)
		}
		log.Printf("Created team: %s (ID: %d) in Division 3", team.Name, team.ID)
	}

	// Division 4 teams
	div4Teams := []string{
		"Blakers A", "Hollingbury Park", "Hollingbury Park A", "Hove Park F", 
		"King Alfred B", "King Alfred C", "Park Avenue", 
		"Preston Park C", "Queens Park E", "Saltdean C", "Saltdean D", "St Ann's D",
	}
	div4, err := s.getDivisionByName(ctx, "Division 4", league.ID, season.ID)
	if err != nil {
		return fmt.Errorf("failed to get Division 4: %w", err)
	}

	for _, teamName := range div4Teams {
		clubName, _ := s.parseTeamName(teamName)
		club, err := s.getClubByName(ctx, clubName)
		if err != nil {
			return fmt.Errorf("failed to get club %s: %w", clubName, err)
		}
		
		team, err := s.getOrCreateTeam(ctx, teamName, club.ID, div4.ID, season.ID)
		if err != nil {
			return fmt.Errorf("failed to create team %s: %w", teamName, err)
		}
		log.Printf("Created team: %s (ID: %d) in Division 4", team.Name, team.ID)
	}

	// Step 7: Create fixtures for Division 1
	err = s.createMockFixturesDiv1(ctx, div1.ID, season.ID)
	if err != nil {
		return fmt.Errorf("failed to create Division 1 fixtures: %w", err)
	}

	log.Println("Mock data import completed successfully")
	return nil
}

// createMockFixturesDiv1 creates mock fixtures for Division 1
func (s *Scraper) createMockFixturesDiv1(ctx context.Context, divisionID, seasonID uint) error {
	// Fixtures for Division 1, Week 6 (May 22, 2025)
	fixtures := []struct {
		Week     int
		Date     string // Format: "2006-01-02"
		HomeTeam string
		AwayTeam string
	}{
		// Week 6
		{6, "2025-05-22", "Dyke Park A", "Hove Park"},
		{6, "2025-05-22", "Hove Park", "Saltdean"},
		{6, "2025-05-22", "King Alfred", "Queens Park"},
		{6, "2025-05-22", "Queens Park A", "Preston Park"},
		{6, "2025-05-22", "St Ann's", "Dyke Park"},
		
		// Week 7
		{7, "2025-05-29", "Dyke Park", "Hove Park"},
		{7, "2025-05-29", "Hove Park", "Preston Park"},
		{7, "2025-05-29", "King Alfred", "Dyke Park A"},
		{7, "2025-05-29", "Queens Park", "St Ann's"},
		{7, "2025-05-29", "Saltdean", "Queens Park A"},
		
		// Week 8
		{8, "2025-06-05", "Dyke Park A", "Queens Park"},
		{8, "2025-06-05", "Hove Park", "King Alfred"},
		{8, "2025-06-05", "Preston Park", "Dyke Park"},
		{8, "2025-06-05", "Queens Park A", "Hove Park"},
		{8, "2025-06-05", "St Ann's", "Saltdean"},
		
		// Week 9
		{9, "2025-06-12", "Dyke Park", "Queens Park A"},
		{9, "2025-06-12", "Hove Park", "St Ann's"},
		{9, "2025-06-12", "Preston Park", "King Alfred"},
		{9, "2025-06-12", "Queens Park", "Hove Park"},
		{9, "2025-06-12", "Saltdean", "Dyke Park A"},
		
		// Week 10
		{10, "2025-06-19", "Dyke Park", "Saltdean"},
		{10, "2025-06-19", "Hove Park", "Dyke Park A"},
		{10, "2025-06-19", "Preston Park", "Hove Park"},
		{10, "2025-06-19", "Queens Park A", "Queens Park"},
		{10, "2025-06-19", "St Ann's", "King Alfred"},
	}

	for _, fix := range fixtures {
		// Parse the date
		fixtureDate, err := time.Parse("2006-01-02", fix.Date)
		if err != nil {
			return fmt.Errorf("failed to parse date %s: %w", fix.Date, err)
		}
		
		// Get the home team
		homeTeam, err := s.getTeamByName(ctx, fix.HomeTeam, divisionID, seasonID)
		if err != nil {
			return fmt.Errorf("failed to get home team %s: %w", fix.HomeTeam, err)
		}
		
		// Get the away team
		awayTeam, err := s.getTeamByName(ctx, fix.AwayTeam, divisionID, seasonID)
		if err != nil {
			return fmt.Errorf("failed to get away team %s: %w", fix.AwayTeam, err)
		}
		
		// Get home club name for venue
		homeClubName, _ := s.parseTeamName(fix.HomeTeam)
		
		// Create the fixture
		fixture := &models.Fixture{
			HomeTeamID:    homeTeam.ID,
			AwayTeamID:    awayTeam.ID,
			DivisionID:    divisionID,
			SeasonID:      seasonID,
			ScheduledDate: fixtureDate,
			VenueLocation: fmt.Sprintf("%s Courts", homeClubName),
			Status:        models.Scheduled,
			Notes:         fmt.Sprintf("Week %d fixture", fix.Week),
		}
		
		// Check if fixture already exists
		exists, err := s.fixtureExists(ctx, fixture)
		if err != nil {
			return fmt.Errorf("failed to check if fixture exists: %w", err)
		}
		
		if !exists {
			err = s.fixtureRepo.Create(ctx, fixture)
			if err != nil {
				return fmt.Errorf("failed to create fixture: %w", err)
			}
			
			log.Printf("Created fixture: %s vs %s on %s (ID: %d)", 
				fix.HomeTeam, fix.AwayTeam, fixtureDate.Format("2006-01-02"), fixture.ID)
			
			// Create default matchups for this fixture
			s.createDefaultMatchups(ctx, fixture.ID)
		}
	}
	
	return nil
}

// getClubByName retrieves a club by name
func (s *Scraper) getClubByName(ctx context.Context, name string) (*models.Club, error) {
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
		if strings.EqualFold(club.Name, name) {
			s.clubNameMap[name] = club.ID
			return &club, nil
		}
	}
	
	return nil, fmt.Errorf("club not found: %s", name)
}

// getDivisionByName retrieves a division by name
func (s *Scraper) getDivisionByName(ctx context.Context, name string, leagueID, seasonID uint) (*models.Division, error) {
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
		if strings.EqualFold(div.Name, name) {
			s.divisionNameMap[name] = div.ID
			return &div, nil
		}
	}
	
	return nil, fmt.Errorf("division not found: %s", name)
}

// getTeamByName retrieves a team by name in a specific division and season
func (s *Scraper) getTeamByName(ctx context.Context, name string, divisionID, seasonID uint) (*models.Team, error) {
	// Check if we already have this team in our map
	if teamID, ok := s.teamNameMap[name]; ok {
		team, err := s.teamRepo.GetByID(ctx, teamID)
		if err == nil && team.DivisionID == divisionID && team.SeasonID == seasonID {
			return team, nil
		}
	}
	
	// Try to find the team by name, division ID, and season ID
	teams, err := s.teamRepo.GetByDivisionAndSeason(ctx, divisionID, seasonID)
	if err != nil {
		return nil, err
	}
	
	for _, team := range teams {
		if strings.EqualFold(team.Name, name) {
			s.teamNameMap[name] = team.ID
			return &team, nil
		}
	}
	
	return nil, fmt.Errorf("team not found: %s in division ID %d", name, divisionID)
} 