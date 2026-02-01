package admin

import (
	"context"

	"jim-dot-tennis/internal/models"
)

// GetAllDivisions retrieves all divisions for filtering
func (s *Service) GetAllDivisions() ([]models.Division, error) {
	ctx := context.Background()
	return s.divisionRepository.FindAll(ctx)
}

// GetDivisionsBySeason retrieves all divisions for a specific season
func (s *Service) GetDivisionsBySeason(seasonID uint) ([]models.Division, error) {
	ctx := context.Background()
	return s.divisionRepository.FindBySeason(ctx, seasonID)
}
