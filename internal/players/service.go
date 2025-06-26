package players

import (
	"context"
	"fmt"
	"time"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
)

// Service provides business logic for player operations
type Service struct {
	db                     *database.DB
	playerRepository       repository.PlayerRepository
	fantasyRepository      repository.FantasyMixedDoublesRepository
	tennisPlayerRepository repository.ProTennisPlayerRepository
	availabilityRepository repository.AvailabilityRepository
	seasonRepository       repository.SeasonRepository
}

// NewService creates a new players service
func NewService(db *database.DB) *Service {
	return &Service{
		db:                     db,
		playerRepository:       repository.NewPlayerRepository(db),
		fantasyRepository:      repository.NewFantasyMixedDoublesRepository(db),
		tennisPlayerRepository: repository.NewProTennisPlayerRepository(db),
		availabilityRepository: repository.NewAvailabilityRepository(db),
		seasonRepository:       repository.NewSeasonRepository(db),
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
	TeamAWoman models.ProTennisPlayer     `json:"team_a_woman"`
	TeamAMan   models.ProTennisPlayer     `json:"team_a_man"`
	TeamBWoman models.ProTennisPlayer     `json:"team_b_woman"`
	TeamBMan   models.ProTennisPlayer     `json:"team_b_man"`
}

// AvailabilityData represents a player's availability response
type AvailabilityData struct {
	Player       PlayerInfo        `json:"player"`
	Availability []AvailabilityDay `json:"availability"`
}

// PlayerInfo represents basic player information
type PlayerInfo struct {
	ID        string `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// AvailabilityDay represents availability for a specific date
type AvailabilityDay struct {
	Date   string                    `json:"date"` // ISO date format (YYYY-MM-DD)
	Status models.AvailabilityStatus `json:"status"`
}

// GetPlayerAvailabilityData retrieves a player's availability data for the next 4 weeks
func (s *Service) GetPlayerAvailabilityData(playerID string) (*AvailabilityData, error) {
	ctx := context.Background()

	// Calculate 4 weeks from now
	now := time.Now()
	startDate := now.Truncate(24 * time.Hour)
	endDate := startDate.AddDate(0, 0, 28) // 4 weeks

	// Get availability exceptions for this date range
	availabilities, err := s.availabilityRepository.GetPlayerAvailabilityByDateRange(ctx, playerID, startDate, endDate)
	if err != nil {
		return nil, err
	}

	// Convert to map for easier lookup
	availabilityMap := make(map[string]models.AvailabilityStatus)
	for _, avail := range availabilities {
		// For single-day exceptions, start_date and end_date should be the same
		dateStr := avail.StartDate.Format("2006-01-02")
		availabilityMap[dateStr] = avail.Status
	}

	// Build response data
	var availabilityDays []AvailabilityDay
	for d := startDate; d.Before(endDate); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")
		status := availabilityMap[dateStr]
		if status == "" {
			status = models.Unknown // Default status
		}

		// Convert backend status to frontend format
		frontendStatus := s.convertBackendStatus(status)

		availabilityDays = append(availabilityDays, AvailabilityDay{
			Date:   dateStr,
			Status: models.AvailabilityStatus(frontendStatus),
		})
	}

	// For now, return mock player info since we don't have direct access to player repository
	// In a real implementation, you'd fetch this data
	return &AvailabilityData{
		Player: PlayerInfo{
			ID:        playerID,
			FirstName: "Player", // This should be fetched from database
			LastName:  "Name",   // This should be fetched from database
		},
		Availability: availabilityDays,
	}, nil
}

// UpdatePlayerAvailability updates a player's availability for a specific date
func (s *Service) UpdatePlayerAvailability(playerID string, dateStr string, status string) error {
	ctx := context.Background()

	// Parse date
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return err
	}

	// Convert frontend status to backend AvailabilityStatus
	availStatus := s.convertFrontendStatus(status)

	// Update availability
	return s.availabilityRepository.UpsertPlayerAvailability(ctx, playerID, date, availStatus, "")
}

// BatchUpdatePlayerAvailability updates multiple availability records
func (s *Service) BatchUpdatePlayerAvailability(playerID string, updates []AvailabilityUpdateRequest) error {
	ctx := context.Background()

	var availabilityUpdates []repository.AvailabilityUpdate
	for _, update := range updates {
		date, err := time.Parse("2006-01-02", update.Date)
		if err != nil {
			continue // Skip invalid dates
		}

		availabilityUpdates = append(availabilityUpdates, repository.AvailabilityUpdate{
			Date:   date,
			Status: s.convertFrontendStatus(update.Status),
			Reason: "",
		})
	}

	return s.availabilityRepository.BatchUpsertPlayerAvailability(ctx, playerID, availabilityUpdates)
}

// convertFrontendStatus converts frontend status strings to backend AvailabilityStatus
func (s *Service) convertFrontendStatus(frontendStatus string) models.AvailabilityStatus {
	switch frontendStatus {
	case "available":
		return models.Available
	case "unavailable":
		return models.Unavailable
	case "if-needed":
		return models.IfNeeded
	case "clear":
		return "clear" // Special case - indicates to delete the record
	default:
		return models.Unknown
	}
}

// convertBackendStatus converts backend AvailabilityStatus to frontend strings
func (s *Service) convertBackendStatus(backendStatus models.AvailabilityStatus) string {
	switch backendStatus {
	case models.Available:
		return "available"
	case models.Unavailable:
		return "unavailable"
	case models.IfNeeded:
		return "if-needed"
	case models.Unknown:
		return "clear"
	default:
		return "clear"
	}
}

// AvailabilityUpdateRequest represents a single availability update request
type AvailabilityUpdateRequest struct {
	Date   string `json:"date"`
	Status string `json:"status"`
}
