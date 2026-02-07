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

// GetTeamsBySeason retrieves all teams for a specific season
func (s *Service) GetTeamsBySeason(seasonID uint) ([]models.Team, error) {
	ctx := context.Background()
	return s.teamRepository.FindBySeason(ctx, seasonID)
}

// GetTeamByID retrieves a team by its ID
func (s *Service) GetTeamByID(teamID uint) (*models.Team, error) {
	ctx := context.Background()
	return s.teamRepository.FindByID(ctx, teamID)
}

// UpdateTeam updates a team record
func (s *Service) UpdateTeam(team *models.Team) error {
	ctx := context.Background()
	return s.teamRepository.Update(ctx, team)
}

// DeleteTeam deletes a team by ID
func (s *Service) DeleteTeam(teamID uint) error {
	ctx := context.Background()
	return s.teamRepository.Delete(ctx, teamID)
}

// IsStAnnsClub checks if a club ID belongs to St Ann's
func (s *Service) IsStAnnsClub(clubID uint) bool {
	ctx := context.Background()
	clubs, err := s.clubRepository.FindByNameLike(ctx, "St Ann")
	if err != nil || len(clubs) == 0 {
		return false
	}
	return clubs[0].ID == clubID
}

// GetAwayTeams retrieves all non-St Ann's teams grouped by club
func (s *Service) GetAwayTeams() ([]AwayTeamGroupedByClub, error) {
	ctx := context.Background()

	// Find St. Ann's club ID
	stAnnsClubs, err := s.clubRepository.FindByNameLike(ctx, "St Ann")
	if err != nil {
		return nil, err
	}
	stAnnsID := uint(0)
	if len(stAnnsClubs) > 0 {
		stAnnsID = stAnnsClubs[0].ID
	}

	// Get active season
	activeSeason, _ := s.seasonRepository.FindActive(ctx)
	if activeSeason == nil {
		return nil, nil
	}

	// Get all teams for the active season
	allTeams, err := s.teamRepository.FindBySeason(ctx, activeSeason.ID)
	if err != nil {
		return nil, err
	}

	// Group non-St Ann's teams by club
	clubMap := make(map[uint]*AwayTeamGroupedByClub)
	clubOrder := []uint{}

	for _, team := range allTeams {
		if team.ClubID == stAnnsID {
			continue // Skip St Ann's teams
		}

		if _, exists := clubMap[team.ClubID]; !exists {
			club, _ := s.clubRepository.FindByID(ctx, team.ClubID)
			clubName := "Unknown Club"
			if club != nil {
				clubName = club.Name
			}
			clubMap[team.ClubID] = &AwayTeamGroupedByClub{
				ClubID:   team.ClubID,
				ClubName: clubName,
			}
			clubOrder = append(clubOrder, team.ClubID)
		}

		divName := ""
		if div, err := s.divisionRepository.FindByID(ctx, team.DivisionID); err == nil {
			divName = div.Name
		}

		clubMap[team.ClubID].Teams = append(clubMap[team.ClubID].Teams, AwayTeamInfo{
			Team:         team,
			DivisionName: divName,
		})
	}

	var result []AwayTeamGroupedByClub
	for _, cid := range clubOrder {
		result = append(result, *clubMap[cid])
	}

	return result, nil
}

// AwayTeamInfo represents an away team with display info
type AwayTeamInfo struct {
	models.Team
	DivisionName string
}

// AwayTeamGroupedByClub groups away teams by their club
type AwayTeamGroupedByClub struct {
	ClubID   uint
	ClubName string
	Teams    []AwayTeamInfo
}

// AwayTeamReviewItem represents a single away team for the post-copy review
type AwayTeamReviewItem struct {
	Team             models.Team
	ClubName         string
	CurrentDivision  string
	PreviousDivision string // empty if team didn't exist in previous season
	IsNew            bool   // true if no match found in previous season
}

// AwayTeamReviewData holds all data for the away team review page
type AwayTeamReviewData struct {
	Teams []AwayTeamReviewItem
}

// GetAwayTeamReviewData builds review data comparing away teams between current and previous season
func (s *Service) GetAwayTeamReviewData(seasonID uint) (*AwayTeamReviewData, error) {
	ctx := context.Background()

	// Find St Ann's club ID
	stAnnsClubs, err := s.clubRepository.FindByNameLike(ctx, "St Ann")
	if err != nil {
		return nil, err
	}
	stAnnsID := uint(0)
	if len(stAnnsClubs) > 0 {
		stAnnsID = stAnnsClubs[0].ID
	}

	// Get the season to find previous year
	season, err := s.seasonRepository.FindByID(ctx, seasonID)
	if err != nil {
		return nil, err
	}

	// Get all teams for this season
	allTeams, err := s.teamRepository.FindBySeason(ctx, seasonID)
	if err != nil {
		return nil, err
	}

	// Get previous season teams for comparison
	previousYear := season.Year - 1
	prevSeasons, _ := s.seasonRepository.FindByYear(ctx, previousYear)
	prevTeamMap := make(map[string]string) // "clubID-name" -> division name
	if len(prevSeasons) > 0 {
		prevTeams, _ := s.teamRepository.FindBySeason(ctx, prevSeasons[0].ID)
		for _, pt := range prevTeams {
			if pt.ClubID == stAnnsID {
				continue
			}
			divName := ""
			if div, err := s.divisionRepository.FindByID(ctx, pt.DivisionID); err == nil {
				divName = div.Name
			}
			key := fmt.Sprintf("%d-%s", pt.ClubID, pt.Name)
			prevTeamMap[key] = divName
		}
	}

	// Build review items for away teams only
	var items []AwayTeamReviewItem
	for _, team := range allTeams {
		if team.ClubID == stAnnsID {
			continue
		}

		clubName := "Unknown Club"
		if club, err := s.clubRepository.FindByID(ctx, team.ClubID); err == nil {
			clubName = club.Name
		}

		currentDiv := ""
		if div, err := s.divisionRepository.FindByID(ctx, team.DivisionID); err == nil {
			currentDiv = div.Name
		}

		key := fmt.Sprintf("%d-%s", team.ClubID, team.Name)
		prevDiv, hadPrev := prevTeamMap[key]

		items = append(items, AwayTeamReviewItem{
			Team:             team,
			ClubName:         clubName,
			CurrentDivision:  currentDiv,
			PreviousDivision: prevDiv,
			IsNew:            !hadPrev,
		})
	}

	return &AwayTeamReviewData{Teams: items}, nil
}
