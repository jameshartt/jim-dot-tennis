package admin

import (
	"context"
	"fmt"
	"log"

	"jim-dot-tennis/internal/models"
)

// SeasonStats holds statistics for a season
type SeasonStats struct {
	SeasonID       uint
	TeamCount      int
	PlayerCount    int
	DivisionCount  int
	FixtureCount   int
	CompletedCount int
}

// DivisionWithTeams holds a division and its teams
type DivisionWithTeams struct {
	Division  models.Division
	Teams     []TeamWithPlayers
	TeamCount int
}

// TeamWithPlayers holds a team with its players and captains
type TeamWithPlayers struct {
	Team         models.Team
	Club         models.Club
	Players      []models.Player
	Captains     []models.Captain
	PlayerCount  int
	CaptainCount int
}

// SeasonSetupData holds all data needed for the season setup page
type SeasonSetupData struct {
	Divisions []DivisionWithTeams
}

// GetAllSeasons retrieves all seasons ordered by year descending
func (s *Service) GetAllSeasons() ([]models.Season, error) {
	ctx := context.Background()
	return s.seasonRepository.FindAll(ctx)
}

// GetActiveSeason retrieves the currently active season
func (s *Service) GetActiveSeason() (*models.Season, error) {
	ctx := context.Background()
	return s.seasonRepository.FindActive(ctx)
}

// CreateSeasonWithWeeks creates a season and automatically generates weeks for it
func (s *Service) CreateSeasonWithWeeks(season *models.Season, numWeeks int) error {
	ctx := context.Background()

	// Create the season first
	if err := s.seasonRepository.Create(ctx, season); err != nil {
		return fmt.Errorf("failed to create season: %w", err)
	}

	// Calculate the duration of each week
	totalDays := season.EndDate.Sub(season.StartDate).Hours() / 24
	daysPerWeek := totalDays / float64(numWeeks)

	// Create weeks
	for i := 1; i <= numWeeks; i++ {
		weekStart := season.StartDate.AddDate(0, 0, int(float64(i-1)*daysPerWeek))
		weekEnd := season.StartDate.AddDate(0, 0, int(float64(i)*daysPerWeek)-1)

		// For the last week, use the season end date
		if i == numWeeks {
			weekEnd = season.EndDate
		}

		week := &models.Week{
			WeekNumber: i,
			SeasonID:   season.ID,
			StartDate:  weekStart,
			EndDate:    weekEnd,
			Name:       fmt.Sprintf("Week %d", i),
			IsActive:   false,
		}

		if err := s.weekRepository.Create(ctx, week); err != nil {
			log.Printf("Failed to create week %d for season %d: %v", i, season.ID, err)
			return fmt.Errorf("failed to create week %d: %w", i, err)
		}
	}

	log.Printf("Successfully created season '%s' with %d weeks", season.Name, numWeeks)
	return nil
}

// SetActiveSeason sets a season as active and deactivates all others
func (s *Service) SetActiveSeason(seasonID uint) error {
	ctx := context.Background()

	// Deactivate all seasons first
	seasons, err := s.seasonRepository.FindAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to get all seasons: %w", err)
	}

	for _, season := range seasons {
		if season.IsActive {
			season.IsActive = false
			if err := s.seasonRepository.Update(ctx, &season); err != nil {
				return fmt.Errorf("failed to deactivate season %d: %w", season.ID, err)
			}
		}
	}

	// Activate the specified season
	season, err := s.seasonRepository.FindByID(ctx, seasonID)
	if err != nil {
		return fmt.Errorf("failed to find season %d: %w", seasonID, err)
	}

	season.IsActive = true
	if err := s.seasonRepository.Update(ctx, season); err != nil {
		return fmt.Errorf("failed to activate season %d: %w", seasonID, err)
	}

	return nil
}

// GetWeeksBySeason retrieves weeks for a specific season
func (s *Service) GetWeeksBySeason(seasonID uint) ([]models.Week, error) {
	ctx := context.Background()
	return s.weekRepository.FindBySeason(ctx, seasonID)
}

// GetWeekCountForSeason returns the number of weeks in a season
func (s *Service) GetWeekCountForSeason(seasonID uint) (int, error) {
	ctx := context.Background()
	return s.weekRepository.CountBySeason(ctx, seasonID)
}

// GetSeasonStats retrieves statistics for a season
func (s *Service) GetSeasonStats(seasonID uint) (*SeasonStats, error) {
	ctx := context.Background()

	stats := &SeasonStats{
		SeasonID: seasonID,
	}

	// Count teams in this season
	teams, err := s.teamRepository.FindBySeason(ctx, seasonID)
	if err == nil {
		stats.TeamCount = len(teams)
	}

	// Count divisions in this season
	divisions, err := s.divisionRepository.FindBySeason(ctx, seasonID)
	if err == nil {
		stats.DivisionCount = len(divisions)
	}

	// Count fixtures in this season
	fixtures, err := s.fixtureRepository.FindBySeason(ctx, seasonID)
	if err == nil {
		stats.FixtureCount = len(fixtures)
		// Count completed fixtures
		for _, f := range fixtures {
			if f.Status == models.Completed {
				stats.CompletedCount++
			}
		}
	}

	// Count unique players in this season (via player_teams table)
	// For now, we'll just set it to 0 or implement if needed
	stats.PlayerCount = 0

	return stats, nil
}

// GetSeasonByID retrieves a season by its ID
func (s *Service) GetSeasonByID(seasonID uint) (*models.Season, error) {
	ctx := context.Background()
	return s.seasonRepository.FindByID(ctx, seasonID)
}

// GetSeasonSetupData retrieves all data needed for season setup page
func (s *Service) GetSeasonSetupData(seasonID uint) (*SeasonSetupData, error) {
	ctx := context.Background()

	setupData := &SeasonSetupData{
		Divisions: []DivisionWithTeams{},
	}

	// Get all divisions for this season
	divisions, err := s.divisionRepository.FindBySeason(ctx, seasonID)
	if err != nil {
		return nil, err
	}

	// For each division, get its teams with details
	for _, division := range divisions {
		divisionData := DivisionWithTeams{
			Division:  division,
			Teams:     []TeamWithPlayers{},
			TeamCount: 0,
		}

		// Get teams in this division
		teams, err := s.teamRepository.FindByDivisionAndSeason(ctx, division.ID, seasonID)
		if err != nil {
			continue
		}

		divisionData.TeamCount = len(teams)

		// For each team, get its players and captains
		for _, team := range teams {
			teamData := TeamWithPlayers{
				Team:         team,
				Players:      []models.Player{},
				Captains:     []models.Captain{},
				PlayerCount:  0,
				CaptainCount: 0,
			}

			// Get club for this team
			club, err := s.clubRepository.FindByID(ctx, team.ClubID)
			if err == nil {
				teamData.Club = *club
			}

			// Get player count for this team
			playerCount, err := s.teamRepository.CountPlayers(ctx, team.ID, seasonID)
			if err == nil {
				teamData.PlayerCount = playerCount
			}

			// Get captains for this team
			captains, err := s.teamRepository.FindCaptainsInTeam(ctx, team.ID, seasonID)
			if err == nil {
				teamData.Captains = captains
				teamData.CaptainCount = len(captains)
			}

			divisionData.Teams = append(divisionData.Teams, teamData)
		}

		setupData.Divisions = append(setupData.Divisions, divisionData)
	}

	return setupData, nil
}

// MoveTeamToDivision promotes/demotes a team to a different division
func (s *Service) MoveTeamToDivision(teamID uint, targetDivisionID uint) error {
	ctx := context.Background()
	return s.teamRepository.UpdateDivision(ctx, teamID, targetDivisionID)
}

// CopyFromPreviousSeason copies divisions and/or teams from the previous season to the target season
func (s *Service) CopyFromPreviousSeason(targetSeasonID uint, copyDivisions, copyTeams bool) error {
	ctx := context.Background()

	// Get the target season
	targetSeason, err := s.seasonRepository.FindByID(ctx, targetSeasonID)
	if err != nil {
		return fmt.Errorf("failed to find target season: %w", err)
	}

	// Find the previous season (by year)
	previousYear := targetSeason.Year - 1
	previousSeasons, err := s.seasonRepository.FindByYear(ctx, previousYear)
	if err != nil || len(previousSeasons) == 0 {
		return fmt.Errorf("no season found for year %d", previousYear)
	}
	previousSeason := previousSeasons[0]

	// Map to track old division ID -> new division ID
	divisionIDMap := make(map[uint]uint)

	// Copy divisions if requested
	if copyDivisions {
		oldDivisions, err := s.divisionRepository.FindBySeason(ctx, previousSeason.ID)
		if err != nil {
			return fmt.Errorf("failed to find divisions from previous season: %w", err)
		}

		for _, oldDiv := range oldDivisions {
			newDiv := &models.Division{
				Name:            oldDiv.Name,
				Level:           oldDiv.Level,
				PlayDay:         oldDiv.PlayDay,
				LeagueID:        oldDiv.LeagueID,
				SeasonID:        targetSeasonID,
				MaxTeamsPerClub: oldDiv.MaxTeamsPerClub,
			}

			if err := s.divisionRepository.Create(ctx, newDiv); err != nil {
				return fmt.Errorf("failed to create division %s: %w", oldDiv.Name, err)
			}

			divisionIDMap[oldDiv.ID] = newDiv.ID
		}
	}

	// Copy teams if requested
	if copyTeams {
		// If divisions weren't copied, we need to build the division map
		if !copyDivisions {
			oldDivisions, err := s.divisionRepository.FindBySeason(ctx, previousSeason.ID)
			if err != nil {
				return fmt.Errorf("failed to find divisions from previous season: %w", err)
			}

			newDivisions, err := s.divisionRepository.FindBySeason(ctx, targetSeasonID)
			if err != nil {
				return fmt.Errorf("failed to find divisions in target season: %w", err)
			}

			// Map by name (assuming division names match)
			newDivsByName := make(map[string]uint)
			for _, div := range newDivisions {
				newDivsByName[div.Name] = div.ID
			}

			for _, oldDiv := range oldDivisions {
				if newDivID, ok := newDivsByName[oldDiv.Name]; ok {
					divisionIDMap[oldDiv.ID] = newDivID
				}
			}
		}

		oldTeams, err := s.teamRepository.FindBySeason(ctx, previousSeason.ID)
		if err != nil {
			return fmt.Errorf("failed to find teams from previous season: %w", err)
		}

		for _, oldTeam := range oldTeams {
			newDivisionID, ok := divisionIDMap[oldTeam.DivisionID]
			if !ok {
				// Skip teams whose division doesn't have a match in the new season
				continue
			}

			newTeam := &models.Team{
				Name:       oldTeam.Name,
				ClubID:     oldTeam.ClubID,
				DivisionID: newDivisionID,
				SeasonID:   targetSeasonID,
			}

			if err := s.teamRepository.Create(ctx, newTeam); err != nil {
				return fmt.Errorf("failed to create team %s: %w", oldTeam.Name, err)
			}

			// Copy players to the new team
			oldPlayers, err := s.teamRepository.FindPlayersInTeam(ctx, oldTeam.ID, previousSeason.ID)
			if err != nil {
				continue // Skip if can't get players
			}

			for _, playerTeam := range oldPlayers {
				_ = s.teamRepository.AddPlayer(ctx, newTeam.ID, playerTeam.PlayerID, targetSeasonID)
			}

			// Copy captains to the new team
			oldCaptains, err := s.teamRepository.FindCaptainsInTeam(ctx, oldTeam.ID, previousSeason.ID)
			if err != nil {
				continue // Skip if can't get captains
			}

			for _, captain := range oldCaptains {
				_ = s.teamRepository.AddCaptain(ctx, newTeam.ID, captain.PlayerID, captain.Role, targetSeasonID)
			}
		}
	}

	return nil
}
