// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

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
