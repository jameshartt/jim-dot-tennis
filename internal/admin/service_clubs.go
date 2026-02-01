package admin

import (
	"context"

	"jim-dot-tennis/internal/models"
)

// GetClubs retrieves all clubs for player club selection
func (s *Service) GetClubs() ([]models.Club, error) {
	ctx := context.Background()
	return s.clubRepository.FindAll(ctx)
}

// GetClubsByName retrieves clubs by name (using LIKE search)
func (s *Service) GetClubsByName(name string) ([]models.Club, error) {
	ctx := context.Background()
	return s.clubRepository.FindByNameLike(ctx, name)
}
