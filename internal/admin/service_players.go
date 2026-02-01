package admin

import (
	"context"
	"strings"

	"jim-dot-tennis/internal/models"
)

// PlayerWithAvailabilityInfo combines player information with availability status for admin display
type PlayerWithAvailabilityInfo struct {
	Player                   models.Player `json:"player"`
	HasAvailabilityURL       bool          `json:"has_availability_url"`
	HasSetNextWeekAvail      bool          `json:"has_set_next_week_avail"`
	NextWeekAvailCount       int           `json:"next_week_avail_count"`
	TeamAppearanceCounts     map[uint]int  `json:"team_appearance_counts"`
	DivisionAppearanceCounts map[uint]int  `json:"division_appearance_counts"`
}

// GetPlayers retrieves all players for admin management
func (s *Service) GetPlayers() ([]models.Player, error) {
	// Use the player repository to fetch all players
	players, err := s.playerRepository.FindAll(context.Background())
	if err != nil {
		return nil, err
	}
	return players, nil
}

// GetFilteredPlayers retrieves players based on search query
func (s *Service) GetFilteredPlayers(query string, activeFilter string, seasonID uint) ([]models.Player, error) {
	ctx := context.Background()

	var players []models.Player
	var err error

	// Just search based on query, ignore activeFilter as it's no longer relevant
	if query != "" {
		players, err = s.playerRepository.SearchPlayers(ctx, query)
	} else {
		players, err = s.playerRepository.FindAll(ctx)
	}

	if err != nil {
		return nil, err
	}

	return players, nil
}

// GetFilteredPlayersWithAvailability retrieves players with availability information
// Supports additional filters: teamID (specific team) and divisionID
func (s *Service) GetFilteredPlayersWithAvailability(query string, activeFilter string, seasonID uint, teamIDs []uint, divisionIDs []uint) ([]PlayerWithAvailabilityInfo, error) {
	ctx := context.Background()

	var players []models.Player
	var err error

	// Handle team/division appearance filters (union)
	if len(teamIDs) > 0 || len(divisionIDs) > 0 {
		playerByID := make(map[string]models.Player)
		activeSeason, seasonErr := s.seasonRepository.FindActive(ctx)
		if seasonErr != nil {
			return nil, seasonErr
		}
		for _, tID := range teamIDs {
			pls, e := s.playerRepository.FindPlayersWhoPlayedForTeamInSeason(ctx, tID, activeSeason.ID)
			if e == nil {
				for _, p := range pls {
					playerByID[p.ID] = p
				}
			}
		}
		if len(divisionIDs) > 0 {
			clubs, clubErr := s.clubRepository.FindByNameLike(ctx, "St Ann")
			if clubErr == nil && len(clubs) > 0 {
				clubID := clubs[0].ID
				for _, dID := range divisionIDs {
					pls, e := s.playerRepository.FindPlayersWhoPlayedForClubDivisionInSeason(ctx, clubID, dID, activeSeason.ID)
					if e == nil {
						for _, p := range pls {
							playerByID[p.ID] = p
						}
					}
				}
			}
		}
		for _, p := range playerByID {
			players = append(players, p)
		}
		if query != "" {
			players = filterPlayersByQuery(players, query)
		}
	} else if activeFilter == "stanns_played" {
		// Find St. Ann's club
		clubs, clubErr := s.clubRepository.FindByNameLike(ctx, "St Ann")
		if clubErr != nil {
			return nil, clubErr
		}
		if len(clubs) == 0 {
			players = []models.Player{}
		} else {
			// Get active season
			activeSeason, seasonErr := s.seasonRepository.FindActive(ctx)
			if seasonErr != nil {
				return nil, seasonErr
			}
			players, err = s.playerRepository.FindPlayersWhoPlayedForClubInSeason(ctx, clubs[0].ID, activeSeason.ID)
			if err != nil {
				return nil, err
			}
			// If a search query is present, further filter results client-side
			if query != "" {
				players = filterPlayersByQuery(players, query)
			}
		}
	} else if query != "" {
		players, err = s.playerRepository.SearchPlayers(ctx, query)
	} else {
		players, err = s.playerRepository.FindAll(ctx)
	}

	if err != nil {
		return nil, err
	}

	// Get next week date range
	weekStart, weekEnd := s.getNextWeekDateRange()

	// Convert to PlayerWithAvailabilityInfo
	var playersWithAvailInfo []PlayerWithAvailabilityInfo
	for _, player := range players {
		playerWithAvail := PlayerWithAvailabilityInfo{
			Player:                   player,
			HasAvailabilityURL:       player.FantasyMatchID != nil,
			HasSetNextWeekAvail:      false,
			NextWeekAvailCount:       0,
			TeamAppearanceCounts:     map[uint]int{},
			DivisionAppearanceCounts: map[uint]int{},
		}

		// Check if player has set availability for next week
		if availRecords, err := s.availabilityRepository.GetPlayerAvailabilityByDateRange(ctx, player.ID, weekStart, weekEnd); err == nil {
			playerWithAvail.HasSetNextWeekAvail = len(availRecords) > 0
			playerWithAvail.NextWeekAvailCount = len(availRecords)
		}

		// If multi-select filters are present, compute counts per selected team/division
		if len(teamIDs) > 0 || len(divisionIDs) > 0 {
			if activeSeason, err := s.seasonRepository.FindActive(ctx); err == nil {
				for _, tID := range teamIDs {
					if c, err := s.playerRepository.CountPlayerAppearancesForTeamInSeason(ctx, player.ID, tID, activeSeason.ID); err == nil {
						playerWithAvail.TeamAppearanceCounts[tID] = c
					}
				}
				if len(divisionIDs) > 0 {
					if clubs, err := s.clubRepository.FindByNameLike(ctx, "St Ann"); err == nil && len(clubs) > 0 {
						clubID := clubs[0].ID
						for _, dID := range divisionIDs {
							if c, err := s.playerRepository.CountPlayerAppearancesForClubDivisionInSeason(ctx, player.ID, clubID, dID, activeSeason.ID); err == nil {
								playerWithAvail.DivisionAppearanceCounts[dID] = c
							}
						}
					}
				}
			}
		}

		playersWithAvailInfo = append(playersWithAvailInfo, playerWithAvail)
	}

	return playersWithAvailInfo, nil
}

// filterPlayersByQuery performs client-side filtering of players by search query
// This is used when we need to combine activity filtering with search
func filterPlayersByQuery(players []models.Player, query string) []models.Player {
	if query == "" {
		return players
	}

	queryLower := strings.ToLower(query)
	var filtered []models.Player

	for _, player := range players {
		// Check if query matches name
		fullName := strings.ToLower(player.FirstName + " " + player.LastName)

		if strings.Contains(fullName, queryLower) {
			filtered = append(filtered, player)
		}
	}

	return filtered
}

// GetPlayerByID retrieves a player by ID for editing
func (s *Service) GetPlayerByID(id string) (*models.Player, error) {
	player, err := s.playerRepository.FindByID(context.Background(), id)
	if err != nil {
		return nil, err
	}
	return player, nil
}

// UpdatePlayer updates a player's information
func (s *Service) UpdatePlayer(player *models.Player) error {
	return s.playerRepository.Update(context.Background(), player)
}

// CreatePlayer creates a new player
func (s *Service) CreatePlayer(player *models.Player) error {
	return s.playerRepository.Create(context.Background(), player)
}
