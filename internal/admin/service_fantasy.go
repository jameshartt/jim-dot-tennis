package admin

import (
	"context"
	"fmt"
	"log"

	"jim-dot-tennis/internal/models"
)

// FantasyDoublesDetail contains detailed information about a fantasy doubles pairing
type FantasyDoublesDetail struct {
	Match      models.FantasyMixedDoubles `json:"match"`
	TeamAWoman models.ProTennisPlayer     `json:"team_a_woman"`
	TeamAMan   models.ProTennisPlayer     `json:"team_a_man"`
	TeamBWoman models.ProTennisPlayer     `json:"team_b_woman"`
	TeamBMan   models.ProTennisPlayer     `json:"team_b_man"`
}

// GetUnassignedFantasyDoubles retrieves fantasy doubles pairings that are not assigned to any player
// or are assigned to the specified player (to allow changing current assignment)
func (s *Service) GetUnassignedFantasyDoubles(currentPlayerID string) ([]models.FantasyMixedDoubles, error) {
	ctx := context.Background()

	// Get all active fantasy pairings
	allPairings, err := s.fantasyRepository.FindActive(ctx)
	if err != nil {
		return nil, err
	}

	// Get the current player to check their fantasy match ID
	var currentPlayerFantasyMatchID *uint
	if currentPlayerID != "" {
		currentPlayer, err := s.playerRepository.FindByID(ctx, currentPlayerID)
		if err == nil && currentPlayer.FantasyMatchID != nil {
			currentPlayerFantasyMatchID = currentPlayer.FantasyMatchID
		}
	}

	// Get all players with assigned fantasy matches
	allPlayers, err := s.playerRepository.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	// Create a set of assigned fantasy match IDs (excluding the current player's)
	assignedMatchIDs := make(map[uint]bool)
	for _, player := range allPlayers {
		if player.FantasyMatchID != nil && player.ID != currentPlayerID {
			assignedMatchIDs[*player.FantasyMatchID] = true
		}
	}

	// Filter pairings to include only unassigned ones or the current player's pairing
	var unassignedPairings []models.FantasyMixedDoubles
	for _, pairing := range allPairings {
		isAssignedToOther := assignedMatchIDs[pairing.ID]
		isCurrentPlayersPairing := currentPlayerFantasyMatchID != nil && *currentPlayerFantasyMatchID == pairing.ID

		if !isAssignedToOther || isCurrentPlayersPairing {
			unassignedPairings = append(unassignedPairings, pairing)
		}
	}

	return unassignedPairings, nil
}

// CreateFantasyDoubles creates a new fantasy doubles pairing
func (s *Service) CreateFantasyDoubles(teamAWomanID, teamAManID, teamBWomanID, teamBManID int) (*models.FantasyMixedDoubles, error) {
	ctx := context.Background()

	// Get the tennis players to generate auth token
	teamAWoman, err := s.tennisPlayerRepository.FindByID(ctx, teamAWomanID)
	if err != nil {
		return nil, fmt.Errorf("failed to get Team A woman: %w", err)
	}

	teamAMan, err := s.tennisPlayerRepository.FindByID(ctx, teamAManID)
	if err != nil {
		return nil, fmt.Errorf("failed to get Team A man: %w", err)
	}

	teamBWoman, err := s.tennisPlayerRepository.FindByID(ctx, teamBWomanID)
	if err != nil {
		return nil, fmt.Errorf("failed to get Team B woman: %w", err)
	}

	teamBMan, err := s.tennisPlayerRepository.FindByID(ctx, teamBManID)
	if err != nil {
		return nil, fmt.Errorf("failed to get Team B man: %w", err)
	}

	// Generate auth token
	authToken := s.fantasyRepository.GenerateAuthToken(teamAWoman, teamAMan, teamBWoman, teamBMan)

	// Create the fantasy doubles match
	fantasyMatch := &models.FantasyMixedDoubles{
		TeamAWomanID: teamAWomanID,
		TeamAManID:   teamAManID,
		TeamBWomanID: teamBWomanID,
		TeamBManID:   teamBManID,
		AuthToken:    authToken,
		IsActive:     true,
	}

	err = s.fantasyRepository.Create(ctx, fantasyMatch)
	if err != nil {
		return nil, err
	}

	return fantasyMatch, nil
}

// GetFantasyDoublesDetailByID retrieves detailed fantasy doubles information including player names
func (s *Service) GetFantasyDoublesDetailByID(id uint) (*FantasyDoublesDetail, error) {
	ctx := context.Background()

	// Get the fantasy match
	match, err := s.fantasyRepository.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Get the tennis players
	teamAWoman, err := s.tennisPlayerRepository.FindByID(ctx, match.TeamAWomanID)
	if err != nil {
		return nil, fmt.Errorf("failed to get Team A woman: %w", err)
	}

	teamAMan, err := s.tennisPlayerRepository.FindByID(ctx, match.TeamAManID)
	if err != nil {
		return nil, fmt.Errorf("failed to get Team A man: %w", err)
	}

	teamBWoman, err := s.tennisPlayerRepository.FindByID(ctx, match.TeamBWomanID)
	if err != nil {
		return nil, fmt.Errorf("failed to get Team B woman: %w", err)
	}

	teamBMan, err := s.tennisPlayerRepository.FindByID(ctx, match.TeamBManID)
	if err != nil {
		return nil, fmt.Errorf("failed to get Team B man: %w", err)
	}

	return &FantasyDoublesDetail{
		Match:      *match,
		TeamAWoman: *teamAWoman,
		TeamAMan:   *teamAMan,
		TeamBWoman: *teamBWoman,
		TeamBMan:   *teamBMan,
	}, nil
}

// GetATPPlayers retrieves ATP players for fantasy doubles creation
func (s *Service) GetATPPlayers() ([]models.ProTennisPlayer, error) {
	return s.tennisPlayerRepository.FindATPPlayers(context.Background())
}

// GetWTAPlayers retrieves WTA players for fantasy doubles creation
func (s *Service) GetWTAPlayers() ([]models.ProTennisPlayer, error) {
	return s.tennisPlayerRepository.FindWTAPlayers(context.Background())
}

// UpdatePlayerFantasyMatch assigns a fantasy match to a player
func (s *Service) UpdatePlayerFantasyMatch(playerID string, fantasyMatchID *uint) error {
	ctx := context.Background()

	log.Printf("UpdatePlayerFantasyMatch called: playerID=%s, fantasyMatchID=%v", playerID, fantasyMatchID)

	// Get the player
	player, err := s.playerRepository.FindByID(ctx, playerID)
	if err != nil {
		log.Printf("Failed to find player %s: %v", playerID, err)
		return err
	}

	log.Printf("Found player: %s %s, current fantasy match ID: %v", player.FirstName, player.LastName, player.FantasyMatchID)

	// Update the fantasy match ID
	player.FantasyMatchID = fantasyMatchID

	log.Printf("Setting player fantasy match ID to: %v", fantasyMatchID)

	err = s.playerRepository.Update(ctx, player)
	if err != nil {
		log.Printf("Failed to update player %s: %v", playerID, err)
		return err
	}

	log.Printf("Successfully updated player %s with fantasy match ID: %v", playerID, fantasyMatchID)

	return nil
}

// GenerateAndAssignRandomFantasyMatch creates a random fantasy doubles pairing and assigns it to a player
func (s *Service) GenerateAndAssignRandomFantasyMatch(playerID string) (*FantasyDoublesDetail, error) {
	ctx := context.Background()

	// Generate one random fantasy match
	if err := s.fantasyRepository.GenerateRandomMatches(ctx, 1); err != nil {
		return nil, fmt.Errorf("failed to generate random fantasy match: %w", err)
	}

	// Get the most recently created active match
	activeMatches, err := s.fantasyRepository.FindActive(ctx)
	if err != nil || len(activeMatches) == 0 {
		return nil, fmt.Errorf("failed to retrieve generated match")
	}

	// Get the most recent match (should be the one we just created)
	latestMatch := activeMatches[0]

	// Assign it to the player
	err = s.UpdatePlayerFantasyMatch(playerID, &latestMatch.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to assign fantasy match to player: %w", err)
	}

	// Return the detailed fantasy match information
	return s.GetFantasyDoublesDetailByID(latestMatch.ID)
}
