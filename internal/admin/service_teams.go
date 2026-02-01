package admin

import (
	"context"
	"fmt"

	"jim-dot-tennis/internal/models"
)

// TeamWithRelations represents a team with its related data for display
type TeamWithRelations struct {
	models.Team
	Division    *models.Division `json:"division,omitempty"`
	Season      *models.Season   `json:"season,omitempty"`
	Captain     *models.Player   `json:"captain,omitempty"`
	PlayerCount int              `json:"player_count"`
}

// TeamDetail represents a team with comprehensive related data for detail view
type TeamDetail struct {
	models.Team
	Club        *models.Club      `json:"club,omitempty"`
	Division    *models.Division  `json:"division,omitempty"`
	Season      *models.Season    `json:"season,omitempty"`
	Captains    []CaptainWithInfo `json:"captains,omitempty"`
	Players     []PlayerInTeam    `json:"players,omitempty"`
	PlayerCount int               `json:"player_count"`
}

// CaptainWithInfo represents a captain with their player information
type CaptainWithInfo struct {
	models.Captain
	Player models.Player `json:"player"`
}

// PlayerInTeam represents a player with their team membership details
type PlayerInTeam struct {
	models.Player
	PlayerTeam models.PlayerTeam `json:"player_team"`
}

// CreateTeam creates a new team
func (s *Service) CreateTeam(team *models.Team) error {
	ctx := context.Background()
	return s.teamRepository.Create(ctx, team)
}

// GetTeams retrieves all teams for admin management
func (s *Service) GetTeams() (interface{}, error) {
	// TODO: Implement team retrieval from database
	return nil, nil
}

// GetStAnnsTeams retrieves teams for St. Ann's club with related data
func (s *Service) GetStAnnsTeams() (*models.Club, []TeamWithRelations, error) {
	ctx := context.Background()

	// Find St. Ann's club
	clubs, err := s.clubRepository.FindByNameLike(ctx, "St Ann")
	if err != nil {
		return nil, nil, err
	}
	if len(clubs) == 0 {
		return nil, nil, nil // No club found
	}
	stAnnsClub := &clubs[0]

	// Get all teams for St. Ann's club
	teams, err := s.teamRepository.FindByClub(ctx, stAnnsClub.ID)
	if err != nil {
		return stAnnsClub, nil, err
	}

	if len(teams) == 0 {
		return stAnnsClub, nil, nil // No teams found
	}

	// Build TeamsWithRelations by fetching related data
	var teamsWithRelations []TeamWithRelations
	for _, team := range teams {
		teamWithRelations := TeamWithRelations{
			Team: team,
		}

		// Get division
		if division, err := s.divisionRepository.FindByID(ctx, team.DivisionID); err == nil {
			teamWithRelations.Division = division
		}

		// Get season
		if season, err := s.seasonRepository.FindByID(ctx, team.SeasonID); err == nil {
			teamWithRelations.Season = season
		}

		// Get team captain
		if captain, err := s.teamRepository.FindTeamCaptain(ctx, team.ID, team.SeasonID); err == nil {
			// Get captain player details
			if playerDetails, err := s.playerRepository.FindByID(ctx, captain.PlayerID); err == nil {
				teamWithRelations.Captain = playerDetails
			}
		}

		// Get player count
		if playerCount, err := s.teamRepository.CountPlayers(ctx, team.ID, team.SeasonID); err == nil {
			teamWithRelations.PlayerCount = playerCount
		}

		teamsWithRelations = append(teamsWithRelations, teamWithRelations)
	}

	return stAnnsClub, teamsWithRelations, nil
}

// GetTeamDetail retrieves comprehensive details for a specific team
func (s *Service) GetTeamDetail(teamID uint) (*TeamDetail, error) {
	ctx := context.Background()

	// Get the base team
	team, err := s.teamRepository.FindByID(ctx, teamID)
	if err != nil {
		return nil, err
	}

	// Create the detail struct
	detail := &TeamDetail{
		Team: *team,
	}

	// Get club
	if club, err := s.clubRepository.FindByID(ctx, team.ClubID); err == nil {
		detail.Club = club
	}

	// Get division
	if division, err := s.divisionRepository.FindByID(ctx, team.DivisionID); err == nil {
		detail.Division = division
	}

	// Get season
	if season, err := s.seasonRepository.FindByID(ctx, team.SeasonID); err == nil {
		detail.Season = season
	}

	// Get captains
	if captains, err := s.teamRepository.FindCaptainsInTeam(ctx, teamID, team.SeasonID); err == nil {
		var captainsWithInfo []CaptainWithInfo
		for _, captain := range captains {
			if player, err := s.playerRepository.FindByID(ctx, captain.PlayerID); err == nil {
				captainsWithInfo = append(captainsWithInfo, CaptainWithInfo{
					Captain: captain,
					Player:  *player,
				})
			}
		}
		detail.Captains = captainsWithInfo
	}

	// Get players in team
	if playerTeams, err := s.teamRepository.FindPlayersInTeam(ctx, teamID, team.SeasonID); err == nil {
		var playersInTeam []PlayerInTeam
		for _, playerTeam := range playerTeams {
			if player, err := s.playerRepository.FindByID(ctx, playerTeam.PlayerID); err == nil {
				playersInTeam = append(playersInTeam, PlayerInTeam{
					Player:     *player,
					PlayerTeam: playerTeam,
				})
			}
		}
		detail.Players = playersInTeam
		detail.PlayerCount = len(playersInTeam)
	}

	return detail, nil
}

// GetAvailablePlayersForCaptain retrieves players who can be made captains for a team
// This includes players from the same club who are not already captains of this team
func (s *Service) GetAvailablePlayersForCaptain(teamID uint) ([]models.Player, error) {
	ctx := context.Background()

	// Get the team to find its club and season
	team, err := s.teamRepository.FindByID(ctx, teamID)
	if err != nil {
		return nil, err
	}

	// Get all players from the same club
	clubPlayers, err := s.playerRepository.FindByClub(ctx, team.ClubID)
	if err != nil {
		return nil, err
	}

	// Get existing captains for this team in this season
	existingCaptains, err := s.teamRepository.FindCaptainsInTeam(ctx, teamID, team.SeasonID)
	if err != nil {
		return nil, err
	}

	// Create a map of existing captain IDs for fast lookup
	captainMap := make(map[string]bool)
	for _, captain := range existingCaptains {
		captainMap[captain.PlayerID] = true
	}

	// Filter out players who are already captains
	var availablePlayers []models.Player
	for _, player := range clubPlayers {
		if !captainMap[player.ID] {
			availablePlayers = append(availablePlayers, player)
		}
	}

	return availablePlayers, nil
}

// AddTeamCaptain adds a player as a captain to a team
func (s *Service) AddTeamCaptain(teamID uint, playerID string, role models.CaptainRole) error {
	ctx := context.Background()

	// Get the team to find its season
	team, err := s.teamRepository.FindByID(ctx, teamID)
	if err != nil {
		return err
	}

	// Verify the player exists
	_, err = s.playerRepository.FindByID(ctx, playerID)
	if err != nil {
		return err
	}

	// Check if player is already a captain with this role for this team
	existingCaptains, err := s.teamRepository.FindCaptainsInTeam(ctx, teamID, team.SeasonID)
	if err != nil {
		return err
	}

	for _, captain := range existingCaptains {
		if captain.PlayerID == playerID && captain.Role == role {
			return fmt.Errorf("player is already a %s captain for this team", role)
		}
	}

	// Add the captain
	return s.teamRepository.AddCaptain(ctx, teamID, playerID, role, team.SeasonID)
}

// RemoveTeamCaptain removes a captain from a team
func (s *Service) RemoveTeamCaptain(teamID uint, playerID string) error {
	ctx := context.Background()

	// Get the team to find its season
	team, err := s.teamRepository.FindByID(ctx, teamID)
	if err != nil {
		return err
	}

	// Remove the captain
	return s.teamRepository.RemoveCaptain(ctx, teamID, playerID, team.SeasonID)
}

// GetEligiblePlayersForTeam retrieves players who can be added to a team
// This excludes players who are already on the team
func (s *Service) GetEligiblePlayersForTeam(teamID uint, query, statusFilter string) ([]models.Player, error) {
	ctx := context.Background()

	// Get the team to find its club and season
	team, err := s.teamRepository.FindByID(ctx, teamID)
	if err != nil {
		return nil, err
	}

	// Get all players from the same club (or all players if needed)
	var allPlayers []models.Player
	if query != "" {
		// If there's a search query, search across all players but prioritize same club
		searchResults, err := s.playerRepository.SearchPlayers(ctx, query)
		if err != nil {
			return nil, err
		}
		allPlayers = searchResults
	} else {
		// Default to players from the same club
		clubPlayers, err := s.playerRepository.FindByClub(ctx, team.ClubID)
		if err != nil {
			return nil, err
		}
		allPlayers = clubPlayers
	}

	// Get current team members to exclude them
	currentPlayerTeams, err := s.teamRepository.FindPlayersInTeam(ctx, teamID, team.SeasonID)
	if err != nil {
		return nil, err
	}

	// Create a map of current team member IDs for fast lookup
	currentMemberMap := make(map[string]bool)
	for _, playerTeam := range currentPlayerTeams {
		currentMemberMap[playerTeam.PlayerID] = true
	}

	// Filter out players who are already on the team
	var eligiblePlayers []models.Player
	for _, player := range allPlayers {
		// Skip players who are already on the team
		if currentMemberMap[player.ID] {
			continue
		}

		eligiblePlayers = append(eligiblePlayers, player)
	}

	return eligiblePlayers, nil
}

// AddPlayersToTeam adds multiple players to a team at once
func (s *Service) AddPlayersToTeam(teamID uint, playerIDs []string) error {
	ctx := context.Background()

	// Get the team to find its season
	team, err := s.teamRepository.FindByID(ctx, teamID)
	if err != nil {
		return err
	}

	// Validate all player IDs exist
	for _, playerID := range playerIDs {
		_, err := s.playerRepository.FindByID(ctx, playerID)
		if err != nil {
			return fmt.Errorf("player %s not found: %v", playerID, err)
		}
	}

	// Check if any players are already on the team
	currentPlayerTeams, err := s.teamRepository.FindPlayersInTeam(ctx, teamID, team.SeasonID)
	if err != nil {
		return err
	}

	currentMemberMap := make(map[string]bool)
	for _, playerTeam := range currentPlayerTeams {
		currentMemberMap[playerTeam.PlayerID] = true
	}

	// Add each player to the team (skip if already a member)
	var addedCount int
	for _, playerID := range playerIDs {
		if currentMemberMap[playerID] {
			continue // Skip players already on team
		}

		err := s.teamRepository.AddPlayer(ctx, teamID, playerID, team.SeasonID)
		if err != nil {
			return fmt.Errorf("failed to add player %s: %v", playerID, err)
		}
		addedCount++
	}

	if addedCount == 0 {
		return fmt.Errorf("no new players were added (all selected players are already on the team)")
	}

	return nil
}

// GetAllTeams retrieves all teams
func (s *Service) GetAllTeams() ([]models.Team, error) {
	ctx := context.Background()
	return s.teamRepository.FindAll(ctx)
}

// GetTeamsBySeason retrieves all teams for a specific season
func (s *Service) GetTeamsBySeason(seasonID uint) ([]models.Team, error) {
	ctx := context.Background()
	return s.teamRepository.FindBySeason(ctx, seasonID)
}
