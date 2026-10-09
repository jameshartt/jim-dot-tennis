// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"context"
	"fmt"
	"log"

	"jim-dot-tennis/internal/models"
)

// SelectedPlayerInfo represents a player selected for a fixture with additional context
type SelectedPlayerInfo struct {
	models.FixturePlayer
	Player             models.Player             `json:"player"`
	AvailabilityStatus models.AvailabilityStatus `json:"availability_status"`
	AvailabilityNotes  string                    `json:"availability_notes"`
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
// Returns team players first, then other home club players (deduplicated)
func (s *Service) GetAvailablePlayersForFixture(fixtureID uint) ([]models.Player, []models.Player, error) {
	ctx := context.Background()

	// Get the fixture to determine the home team
	fixture, err := s.fixtureRepository.FindByID(ctx, fixtureID)
	if err != nil {
		return nil, nil, err
	}

	homeClubID := s.homeClubID

	// Find the home club team
	var homeClubTeam *models.Team

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

	// Find which team is the home club
	if homeTeam.ClubID == homeClubID {
		homeClubTeam = homeTeam
	} else if awayTeam.ClubID == homeClubID {
		homeClubTeam = awayTeam
	} else {
		return nil, nil, fmt.Errorf("home club team not found in this fixture")
	}

	teamPlayerTeams, err := s.teamRepository.FindPlayersInTeam(ctx, homeClubTeam.ID, homeClubTeam.SeasonID)
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

	// Get all home club players
	allHomeClubPlayers, err := s.playerRepository.FindByClub(ctx, homeClubID)
	if err != nil {
		return teamPlayers, nil, err
	}

	// Deduplicate: remove team players from the "other home club players" list
	var otherHomeClubPlayers []models.Player
	for _, player := range allHomeClubPlayers {
		if !teamPlayerMap[player.ID] {
			otherHomeClubPlayers = append(otherHomeClubPlayers, player)
		}
	}

	return teamPlayers, otherHomeClubPlayers, nil
}

// GetAvailablePlayersForFixtureWithTeamContext gets available players for a fixture with team context
// For derby matches, managingTeamID specifies which team to prioritize (0 means auto-detect)
// Returns team players first, then other home club players (deduplicated)
func (s *Service) GetAvailablePlayersForFixtureWithTeamContext(fixtureID uint, managingTeamID uint) ([]models.Player, []models.Player, error) {
	ctx := context.Background()

	// Get the fixture to determine the teams
	fixture, err := s.fixtureRepository.FindByID(ctx, fixtureID)
	if err != nil {
		return nil, nil, err
	}

	homeClubID := s.homeClubID

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
	isHomeClubTeam := homeTeam.ClubID == homeClubID
	isAwayClubTeam := awayTeam.ClubID == homeClubID
	isDerby := isHomeClubTeam && isAwayClubTeam

	var homeClubTeam *models.Team

	if isDerby {
		// For derby matches, use the specified managing team
		if managingTeamID > 0 {
			if homeTeam.ID == managingTeamID {
				homeClubTeam = homeTeam
			} else if awayTeam.ID == managingTeamID {
				homeClubTeam = awayTeam
			} else {
				// Default to home team if managing team not found
				homeClubTeam = homeTeam
			}
		} else {
			// Default to home team for derby matches
			homeClubTeam = homeTeam
		}
	} else {
		// Regular match - find which team is the home club
		if isHomeClubTeam {
			homeClubTeam = homeTeam
		} else if isAwayClubTeam {
			homeClubTeam = awayTeam
		} else {
			return nil, nil, fmt.Errorf("home club team not found in this fixture")
		}
	}

	teamPlayerTeams, err := s.teamRepository.FindPlayersInTeam(ctx, homeClubTeam.ID, homeClubTeam.SeasonID)
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

	// Get all home club players
	allHomeClubPlayers, err := s.playerRepository.FindByClub(ctx, homeClubID)
	if err != nil {
		return teamPlayers, nil, err
	}

	// Deduplicate: remove team players from the "other home club players" list
	var otherHomeClubPlayers []models.Player
	for _, player := range allHomeClubPlayers {
		if !teamPlayerMap[player.ID] {
			otherHomeClubPlayers = append(otherHomeClubPlayers, player)
		}
	}

	return teamPlayers, otherHomeClubPlayers, nil
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

// GetAvailablePlayersWithEligibilityForTeamSelection retrieves players with both availability and eligibility information
func (s *Service) GetAvailablePlayersWithEligibilityForTeamSelection(fixtureID uint, managingTeamID uint) ([]PlayerWithEligibility, []PlayerWithEligibility, error) {
	ctx := context.Background()

	// Get available players lists based on managing team (for derby matches)
	var teamPlayers, allHomeClubPlayers []models.Player
	var err error

	if managingTeamID > 0 {
		teamPlayers, allHomeClubPlayers, err = s.GetAvailablePlayersForFixtureWithTeamContext(fixtureID, managingTeamID)
	} else {
		teamPlayers, allHomeClubPlayers, err = s.GetAvailablePlayersForFixture(fixtureID)
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
		// For non-derby matches, determine the home club team
		homeClubID := s.homeClubID

		// Check if home team is the home club
		homeTeam, err := s.teamRepository.FindByID(ctx, fixture.HomeTeamID)
		if err == nil && homeTeam.ClubID == homeClubID {
			teamID = homeTeam.ID
		} else {
			// Check if away team is the home club
			awayTeam, err := s.teamRepository.FindByID(ctx, fixture.AwayTeamID)
			if err == nil && awayTeam.ClubID == homeClubID {
				teamID = awayTeam.ID
			}
		}
	}

	// Fixture-wide availability and eligibility context are loaded once; only
	// the per-player rule checks run for each player.
	availability, err := s.loadFixtureAvailability(ctx, fixture)
	if err != nil {
		return nil, nil, err
	}
	var teamFixture *teamFixtureEligibility
	var teamFixtureErr error
	if teamID > 0 {
		teamFixture, teamFixtureErr = s.teamEligibilityService.forTeamFixture(ctx, teamID, fixtureID)
	}

	withEligibility := func(players []models.Player) []PlayerWithEligibility {
		var out []PlayerWithEligibility
		for _, player := range players {
			var eligibility *PlayerEligibilityInfo
			if teamID > 0 {
				err := teamFixtureErr
				if err == nil {
					eligibility, err = teamFixture.forPlayer(ctx, player)
				}
				if err != nil {
					// Log error but continue - default to allowing play
					log.Printf("Eligibility check failed for player %s in fixture %d: %v", player.ID, fixtureID, err)
					eligibility = &PlayerEligibilityInfo{
						Player:  player,
						CanPlay: true,
					}
				}
			}

			info := availability.forPlayer(player.ID)
			out = append(out, PlayerWithEligibility{
				Player:             player,
				AvailabilityStatus: info.Status,
				AvailabilityNotes:  info.Notes,
				Eligibility:        eligibility,
			})
		}
		return out
	}

	return withEligibility(teamPlayers), withEligibility(allHomeClubPlayers), nil
}

// fixtureAvailability holds every player's availability answers relevant to
// one fixture, loaded with three queries instead of up to four per player.
type fixtureAvailability struct {
	byFixture map[string]PlayerAvailabilityInfo
	byDate    map[string]PlayerAvailabilityInfo
	byWeekday map[string]PlayerAvailabilityInfo
}

// loadFixtureAvailability reads the fixture-specific answers, the date
// exceptions covering the fixture date and the weekday patterns for the
// fixture's season.
func (s *Service) loadFixtureAvailability(ctx context.Context, fixture *models.Fixture) (*fixtureAvailability, error) {
	fa := &fixtureAvailability{
		byFixture: map[string]PlayerAvailabilityInfo{},
		byDate:    map[string]PlayerAvailabilityInfo{},
		byWeekday: map[string]PlayerAvailabilityInfo{},
	}

	fixtureRows, err := s.availabilityRepository.FindFixtureAvailabilityByFixture(ctx, fixture.ID)
	if err != nil {
		return nil, fmt.Errorf("fixture availability: %w", err)
	}
	for _, a := range fixtureRows {
		fa.byFixture[a.PlayerID] = PlayerAvailabilityInfo{Status: a.Status, Notes: a.Notes}
	}

	exceptions, err := s.availabilityRepository.FindExceptionsCoveringDate(ctx, fixture.ScheduledDate)
	if err != nil {
		return nil, fmt.Errorf("availability exceptions: %w", err)
	}
	for _, a := range exceptions {
		// Rows come newest first: the first one per player wins.
		if _, seen := fa.byDate[a.PlayerID]; !seen {
			fa.byDate[a.PlayerID] = PlayerAvailabilityInfo{Status: a.Status, Notes: a.Reason}
		}
	}

	weekday, err := s.availabilityRepository.FindGeneralAvailabilityForDay(ctx, fixture.SeasonID, fixture.ScheduledDate.Weekday().String())
	if err != nil {
		return nil, fmt.Errorf("general availability: %w", err)
	}
	for _, a := range weekday {
		fa.byWeekday[a.PlayerID] = PlayerAvailabilityInfo{Status: a.Status, Notes: a.Notes}
	}
	return fa, nil
}

// forPlayer applies the priority order: fixture-specific > date exception >
// general day-of-week > unknown.
func (fa *fixtureAvailability) forPlayer(playerID string) PlayerAvailabilityInfo {
	for _, answers := range []map[string]PlayerAvailabilityInfo{fa.byFixture, fa.byDate, fa.byWeekday} {
		if info, ok := answers[playerID]; ok {
			return info
		}
	}
	return PlayerAvailabilityInfo{Status: models.Unknown}
}
