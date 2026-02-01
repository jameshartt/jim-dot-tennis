package admin

import (
	"context"
	"fmt"
	"time"

	"jim-dot-tennis/internal/models"
)

// SelectedPlayerInfo represents a player selected for a fixture with additional context
type SelectedPlayerInfo struct {
	models.FixturePlayer
	Player             models.Player             `json:"player"`
	AvailabilityStatus models.AvailabilityStatus `json:"availability_status"`
	AvailabilityNotes  string                    `json:"availability_notes"`
}

// PlayerWithAvailability combines player information with their availability status for a fixture
type PlayerWithAvailability struct {
	Player             models.Player
	AvailabilityStatus models.AvailabilityStatus
	AvailabilityNotes  string
}

// PlayerWithEligibility combines player information with availability and eligibility for team selection
type PlayerWithEligibility struct {
	Player             models.Player
	AvailabilityStatus models.AvailabilityStatus
	AvailabilityNotes  string
	Eligibility        *PlayerEligibilityInfo
}

// PlayerAvailabilityInfo holds availability information for a player
type PlayerAvailabilityInfo struct {
	Status models.AvailabilityStatus
	Notes  string
}

// GetAvailablePlayersForFixture retrieves players available for selection in a fixture
// Returns team players first, then other St Ann players (deduplicated)
func (s *Service) GetAvailablePlayersForFixture(fixtureID uint) ([]models.Player, []models.Player, error) {
	ctx := context.Background()

	// Get the fixture to determine the home team
	fixture, err := s.fixtureRepository.FindByID(ctx, fixtureID)
	if err != nil {
		return nil, nil, err
	}

	// Find the St Ann's club ID dynamically
	stAnnsClubs, err := s.clubRepository.FindByNameLike(ctx, "St Ann")
	if err != nil {
		return nil, nil, err
	}
	if len(stAnnsClubs) == 0 {
		return nil, nil, fmt.Errorf("St Ann's club not found")
	}
	stAnnsClubID := stAnnsClubs[0].ID

	// Find the St Ann's team
	var stAnnsTeam *models.Team

	// Get home team
	homeTeam, err := s.teamRepository.FindByID(ctx, fixture.HomeTeamID)
	if err != nil {
		return nil, nil, err
	}

	// Get away team
	awayTeam, err := s.teamRepository.FindByID(ctx, fixture.AwayTeamID)
	if err != nil {
		return nil, nil, err
	}

	// Find which team is St Ann's
	if homeTeam.ClubID == stAnnsClubID {
		stAnnsTeam = homeTeam
	} else if awayTeam.ClubID == stAnnsClubID {
		stAnnsTeam = awayTeam
	} else {
		return nil, nil, fmt.Errorf("no St Ann's team found in this fixture")
	}

	teamPlayerTeams, err := s.teamRepository.FindPlayersInTeam(ctx, stAnnsTeam.ID, stAnnsTeam.SeasonID)
	if err != nil {
		return nil, nil, err
	}

	var teamPlayers []models.Player
	teamPlayerMap := make(map[string]bool) // Track team player IDs for deduplication
	for _, pt := range teamPlayerTeams {
		if player, err := s.playerRepository.FindByID(ctx, pt.PlayerID); err == nil {
			teamPlayers = append(teamPlayers, *player)
			teamPlayerMap[player.ID] = true
		}
	}

	// Get all St Ann players
	clubs, err := s.clubRepository.FindByNameLike(ctx, "St Ann")
	if err != nil {
		return teamPlayers, nil, err
	}
	if len(clubs) == 0 {
		return teamPlayers, nil, nil
	}

	allStAnnPlayers, err := s.playerRepository.FindByClub(ctx, clubs[0].ID)
	if err != nil {
		return teamPlayers, nil, err
	}

	// Deduplicate: remove team players from the "other St Ann players" list
	var otherStAnnPlayers []models.Player
	for _, player := range allStAnnPlayers {
		if !teamPlayerMap[player.ID] {
			otherStAnnPlayers = append(otherStAnnPlayers, player)
		}
	}

	return teamPlayers, otherStAnnPlayers, nil
}

// GetAvailablePlayersForFixtureWithTeamContext gets available players for a fixture with team context
// For derby matches, managingTeamID specifies which team to prioritize (0 means auto-detect)
// Returns team players first, then other St Ann players (deduplicated)
func (s *Service) GetAvailablePlayersForFixtureWithTeamContext(fixtureID uint, managingTeamID uint) ([]models.Player, []models.Player, error) {
	ctx := context.Background()

	// Get the fixture to determine the teams
	fixture, err := s.fixtureRepository.FindByID(ctx, fixtureID)
	if err != nil {
		return nil, nil, err
	}

	// Find the St Ann's club ID dynamically
	stAnnsClubs, err := s.clubRepository.FindByNameLike(ctx, "St Ann")
	if err != nil {
		return nil, nil, err
	}
	if len(stAnnsClubs) == 0 {
		return nil, nil, fmt.Errorf("St Ann's club not found")
	}
	stAnnsClubID := stAnnsClubs[0].ID

	// Get home and away teams
	homeTeam, err := s.teamRepository.FindByID(ctx, fixture.HomeTeamID)
	if err != nil {
		return nil, nil, err
	}

	awayTeam, err := s.teamRepository.FindByID(ctx, fixture.AwayTeamID)
	if err != nil {
		return nil, nil, err
	}

	// Determine if this is a derby match
	isHomeStAnns := homeTeam.ClubID == stAnnsClubID
	isAwayStAnns := awayTeam.ClubID == stAnnsClubID
	isDerby := isHomeStAnns && isAwayStAnns

	var stAnnsTeam *models.Team

	if isDerby {
		// For derby matches, use the specified managing team
		if managingTeamID > 0 {
			if homeTeam.ID == managingTeamID {
				stAnnsTeam = homeTeam
			} else if awayTeam.ID == managingTeamID {
				stAnnsTeam = awayTeam
			} else {
				// Default to home team if managing team not found
				stAnnsTeam = homeTeam
			}
		} else {
			// Default to home team for derby matches
			stAnnsTeam = homeTeam
		}
	} else {
		// Regular match - find which team is St Ann's
		if isHomeStAnns {
			stAnnsTeam = homeTeam
		} else if isAwayStAnns {
			stAnnsTeam = awayTeam
		} else {
			return nil, nil, fmt.Errorf("no St Ann's team found in this fixture")
		}
	}

	teamPlayerTeams, err := s.teamRepository.FindPlayersInTeam(ctx, stAnnsTeam.ID, stAnnsTeam.SeasonID)
	if err != nil {
		return nil, nil, err
	}

	var teamPlayers []models.Player
	teamPlayerMap := make(map[string]bool) // Track team player IDs for deduplication
	for _, pt := range teamPlayerTeams {
		if player, err := s.playerRepository.FindByID(ctx, pt.PlayerID); err == nil {
			teamPlayers = append(teamPlayers, *player)
			teamPlayerMap[player.ID] = true
		}
	}

	// Get all St Ann players
	clubs, err := s.clubRepository.FindByNameLike(ctx, "St Ann")
	if err != nil {
		return teamPlayers, nil, err
	}
	if len(clubs) == 0 {
		return teamPlayers, nil, nil
	}

	allStAnnPlayers, err := s.playerRepository.FindByClub(ctx, clubs[0].ID)
	if err != nil {
		return teamPlayers, nil, err
	}

	// Deduplicate: remove team players from the "other St Ann players" list
	var otherStAnnPlayers []models.Player
	for _, player := range allStAnnPlayers {
		if !teamPlayerMap[player.ID] {
			otherStAnnPlayers = append(otherStAnnPlayers, player)
		}
	}

	return teamPlayers, otherStAnnPlayers, nil
}

// AddPlayerToFixture adds a player to the fixture selection
func (s *Service) AddPlayerToFixture(fixtureID uint, playerID string, isHome bool) error {
	ctx := context.Background()

	// Check if player is already selected for this fixture
	selectedPlayers, err := s.fixtureRepository.FindSelectedPlayers(ctx, fixtureID)
	if err != nil {
		return err
	}

	for _, sp := range selectedPlayers {
		if sp.PlayerID == playerID {
			return fmt.Errorf("player is already selected for this fixture")
		}
	}

	// Calculate next position
	position := len(selectedPlayers) + 1

	fixturePlayer := &models.FixturePlayer{
		FixtureID: fixtureID,
		PlayerID:  playerID,
		IsHome:    isHome,
		Position:  position,
	}

	return s.fixtureRepository.AddSelectedPlayer(ctx, fixturePlayer)
}

// RemovePlayerFromFixture removes a player from the fixture selection
func (s *Service) RemovePlayerFromFixture(fixtureID uint, playerID string) error {
	ctx := context.Background()
	return s.fixtureRepository.RemoveSelectedPlayer(ctx, fixtureID, playerID)
}

// UpdatePlayerPositionInFixture updates the position/order of a selected player
func (s *Service) UpdatePlayerPositionInFixture(fixtureID uint, playerID string, position int) error {
	ctx := context.Background()
	return s.fixtureRepository.UpdateSelectedPlayerPosition(ctx, fixtureID, playerID, position)
}

// ClearFixturePlayerSelection removes all selected players from a fixture
func (s *Service) ClearFixturePlayerSelection(fixtureID uint) error {
	ctx := context.Background()
	return s.fixtureRepository.ClearSelectedPlayers(ctx, fixtureID)
}

// AddPlayerToFixtureWithTeam adds a player to the fixture selection for a specific managing team (for derby matches)
func (s *Service) AddPlayerToFixtureWithTeam(fixtureID uint, playerID string, isHome bool, managingTeamID uint) error {
	ctx := context.Background()

	// Check if player is already selected for this fixture by this team
	selectedPlayers, err := s.fixtureRepository.FindSelectedPlayersByTeam(ctx, fixtureID, managingTeamID)
	if err != nil {
		return err
	}

	for _, sp := range selectedPlayers {
		if sp.PlayerID == playerID {
			return fmt.Errorf("player is already selected for this fixture by this team")
		}
	}

	// Calculate next position for this team
	position := len(selectedPlayers) + 1

	fixturePlayer := &models.FixturePlayer{
		FixtureID:      fixtureID,
		PlayerID:       playerID,
		IsHome:         isHome,
		Position:       position,
		ManagingTeamID: &managingTeamID,
	}

	return s.fixtureRepository.AddSelectedPlayer(ctx, fixturePlayer)
}

// RemovePlayerFromFixtureByTeam removes a player from the fixture selection for a specific team
func (s *Service) RemovePlayerFromFixtureByTeam(fixtureID uint, playerID string, managingTeamID uint) error {
	ctx := context.Background()
	return s.fixtureRepository.RemoveSelectedPlayerByTeam(ctx, fixtureID, managingTeamID, playerID)
}

// ClearFixturePlayerSelectionByTeam removes all selected players from a fixture for a specific team
func (s *Service) ClearFixturePlayerSelectionByTeam(fixtureID uint, managingTeamID uint) error {
	ctx := context.Background()
	return s.fixtureRepository.ClearSelectedPlayersByTeam(ctx, fixtureID, managingTeamID)
}

// GetAvailablePlayersForFixtureWithAvailability returns players with their availability status for a fixture
func (s *Service) GetAvailablePlayersForFixtureWithAvailability(fixtureID uint) ([]PlayerWithAvailability, []PlayerWithAvailability, error) {
	ctx := context.Background()

	// Get the basic player lists first
	teamPlayers, allStAnnPlayers, err := s.GetAvailablePlayersForFixture(fixtureID)
	if err != nil {
		return nil, nil, err
	}

	// Get the fixture to get its date
	fixture, err := s.fixtureRepository.FindByID(ctx, fixtureID)
	if err != nil {
		return nil, nil, err
	}

	// Convert players to PlayerWithAvailability
	teamPlayersWithAvail := make([]PlayerWithAvailability, 0, len(teamPlayers))
	for _, player := range teamPlayers {
		availability := s.determinePlayerAvailabilityForFixture(ctx, player.ID, fixtureID, fixture.ScheduledDate)
		teamPlayersWithAvail = append(teamPlayersWithAvail, PlayerWithAvailability{
			Player:             player,
			AvailabilityStatus: availability.Status,
			AvailabilityNotes:  availability.Notes,
		})
	}

	allStAnnPlayersWithAvail := make([]PlayerWithAvailability, 0, len(allStAnnPlayers))
	for _, player := range allStAnnPlayers {
		availability := s.determinePlayerAvailabilityForFixture(ctx, player.ID, fixtureID, fixture.ScheduledDate)
		allStAnnPlayersWithAvail = append(allStAnnPlayersWithAvail, PlayerWithAvailability{
			Player:             player,
			AvailabilityStatus: availability.Status,
			AvailabilityNotes:  availability.Notes,
		})
	}

	return teamPlayersWithAvail, allStAnnPlayersWithAvail, nil
}

// GetAvailablePlayersWithEligibilityForTeamSelection retrieves players with both availability and eligibility information
func (s *Service) GetAvailablePlayersWithEligibilityForTeamSelection(fixtureID uint, managingTeamID uint) ([]PlayerWithEligibility, []PlayerWithEligibility, error) {
	ctx := context.Background()

	// Get available players lists based on managing team (for derby matches)
	var teamPlayers, allStAnnPlayers []models.Player
	var err error

	if managingTeamID > 0 {
		teamPlayers, allStAnnPlayers, err = s.GetAvailablePlayersForFixtureWithTeamContext(fixtureID, managingTeamID)
	} else {
		teamPlayers, allStAnnPlayers, err = s.GetAvailablePlayersForFixture(fixtureID)
	}

	if err != nil {
		return nil, nil, err
	}

	// Get fixture for date context
	fixture, err := s.fixtureRepository.FindByID(ctx, fixtureID)
	if err != nil {
		return nil, nil, err
	}

	// Determine which team we're selecting for
	var teamID uint
	if managingTeamID > 0 {
		teamID = managingTeamID
	} else {
		// For non-derby matches, determine the St Ann's team
		stAnnsClubs, err := s.clubRepository.FindByNameLike(ctx, "St Ann")
		if err != nil || len(stAnnsClubs) == 0 {
			return nil, nil, fmt.Errorf("St Ann's club not found")
		}
		stAnnsClubID := stAnnsClubs[0].ID

		// Check if home team is St Ann's
		homeTeam, err := s.teamRepository.FindByID(ctx, fixture.HomeTeamID)
		if err == nil && homeTeam.ClubID == stAnnsClubID {
			teamID = homeTeam.ID
		} else {
			// Check if away team is St Ann's
			awayTeam, err := s.teamRepository.FindByID(ctx, fixture.AwayTeamID)
			if err == nil && awayTeam.ClubID == stAnnsClubID {
				teamID = awayTeam.ID
			}
		}
	}

	// Convert team players to players with availability and eligibility
	var teamPlayersWithEligibility []PlayerWithEligibility
	for _, player := range teamPlayers {
		availability := s.determinePlayerAvailabilityForFixture(ctx, player.ID, fixtureID, fixture.ScheduledDate)

		// Get eligibility information
		var eligibility *PlayerEligibilityInfo
		if teamID > 0 {
			eligibility, err = s.teamEligibilityService.GetPlayerEligibilityForTeam(ctx, player.ID, teamID, fixtureID)
			if err != nil {
				// Log error but continue - default to allowing play
				eligibility = &PlayerEligibilityInfo{
					Player:  player,
					CanPlay: true,
				}
			}
		}

		teamPlayersWithEligibility = append(teamPlayersWithEligibility, PlayerWithEligibility{
			Player:             player,
			AvailabilityStatus: availability.Status,
			AvailabilityNotes:  availability.Notes,
			Eligibility:        eligibility,
		})
	}

	// Convert all St Ann players to players with availability and eligibility
	var allStAnnPlayersWithEligibility []PlayerWithEligibility
	for _, player := range allStAnnPlayers {
		availability := s.determinePlayerAvailabilityForFixture(ctx, player.ID, fixtureID, fixture.ScheduledDate)

		// Get eligibility information
		var eligibility *PlayerEligibilityInfo
		if teamID > 0 {
			eligibility, err = s.teamEligibilityService.GetPlayerEligibilityForTeam(ctx, player.ID, teamID, fixtureID)
			if err != nil {
				// Log error but continue - default to allowing play
				eligibility = &PlayerEligibilityInfo{
					Player:  player,
					CanPlay: true,
				}
			}
		}

		allStAnnPlayersWithEligibility = append(allStAnnPlayersWithEligibility, PlayerWithEligibility{
			Player:             player,
			AvailabilityStatus: availability.Status,
			AvailabilityNotes:  availability.Notes,
			Eligibility:        eligibility,
		})
	}

	return teamPlayersWithEligibility, allStAnnPlayersWithEligibility, nil
}

// determinePlayerAvailabilityForFixture determines a player's availability for a specific fixture
// following the priority order: fixture-specific > date exception > general day-of-week > unknown
func (s *Service) determinePlayerAvailabilityForFixture(ctx context.Context, playerID string, fixtureID uint, fixtureDate time.Time) PlayerAvailabilityInfo {
	// 1. Check fixture-specific availability first (highest priority)
	if fixtureAvail, err := s.availabilityRepository.GetPlayerFixtureAvailability(ctx, playerID, fixtureID); err == nil && fixtureAvail != nil {
		return PlayerAvailabilityInfo{
			Status: fixtureAvail.Status,
			Notes:  fixtureAvail.Notes,
		}
	}

	// 2. Check for date-specific exceptions
	if dateAvail, err := s.availabilityRepository.GetPlayerAvailabilityByDate(ctx, playerID, fixtureDate); err == nil && dateAvail != nil {
		return PlayerAvailabilityInfo{
			Status: dateAvail.Status,
			Notes:  dateAvail.Reason,
		}
	}

	// 3. Check general day-of-week availability
	// First get the current season - we'll need to implement this
	// For now, we'll assume season ID 1 or get it from the fixture
	fixture, err := s.fixtureRepository.FindByID(ctx, fixtureID)
	if err != nil {
		return PlayerAvailabilityInfo{Status: models.Unknown}
	}

	dayOfWeek := fixtureDate.Weekday().String()
	if generalAvails, err := s.availabilityRepository.GetPlayerGeneralAvailability(ctx, playerID, fixture.SeasonID); err == nil {
		for _, avail := range generalAvails {
			if avail.DayOfWeek == dayOfWeek {
				return PlayerAvailabilityInfo{
					Status: avail.Status,
					Notes:  avail.Notes,
				}
			}
		}
	}

	// 4. Default to Unknown if nothing is specified
	return PlayerAvailabilityInfo{Status: models.Unknown}
}
