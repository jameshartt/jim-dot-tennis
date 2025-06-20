package players

import (
	"context"
	"fmt"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
)

// Service provides business logic for player operations
type Service struct {
	db                     *database.DB
	playerRepository       repository.PlayerRepository
	fantasyRepository      repository.FantasyMixedDoublesRepository
	tennisPlayerRepository repository.TennisPlayerRepository
}

// NewService creates a new players service
func NewService(db *database.DB) *Service {
	return &Service{
		db:                     db,
		playerRepository:       repository.NewPlayerRepository(db),
		fantasyRepository:      repository.NewFantasyMixedDoublesRepository(db),
		tennisPlayerRepository: repository.NewTennisPlayerRepository(db),
	}
}

// GetFantasyMatchByToken retrieves a fantasy mixed doubles match by its auth token
func (s *Service) GetFantasyMatchByToken(authToken string) (*FantasyMatchDetail, error) {
	ctx := context.Background()

	// Find the fantasy match
	match, err := s.fantasyRepository.FindByAuthToken(ctx, authToken)
	if err != nil {
		return nil, fmt.Errorf("fantasy match not found for token: %s", authToken)
	}

	// Get the tennis players for this match
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

	return &FantasyMatchDetail{
		Match:      *match,
		TeamAWoman: *teamAWoman,
		TeamAMan:   *teamAMan,
		TeamBWoman: *teamBWoman,
		TeamBMan:   *teamBMan,
	}, nil
}

// GenerateFantasyMatchForPlayer creates a new random fantasy match and returns its auth token
func (s *Service) GenerateFantasyMatchForPlayer() (string, error) {
	ctx := context.Background()

	// Generate one random match
	if err := s.fantasyRepository.GenerateRandomMatches(ctx, 1); err != nil {
		return "", fmt.Errorf("failed to generate fantasy match: %w", err)
	}

	// Get the most recently created active match
	activeMatches, err := s.fantasyRepository.FindActive(ctx)
	if err != nil || len(activeMatches) == 0 {
		return "", fmt.Errorf("failed to retrieve generated match")
	}

	// Return the auth token of the most recent match
	return activeMatches[0].AuthToken, nil
}

// FantasyMatchDetail contains all details about a fantasy mixed doubles match
type FantasyMatchDetail struct {
	Match      models.FantasyMixedDoubles `json:"match"`
	TeamAWoman models.TennisPlayer        `json:"team_a_woman"`
	TeamAMan   models.TennisPlayer        `json:"team_a_man"`
	TeamBWoman models.TennisPlayer        `json:"team_b_woman"`
	TeamBMan   models.TennisPlayer        `json:"team_b_man"`
}
