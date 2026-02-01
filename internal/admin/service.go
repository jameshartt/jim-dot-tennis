package admin

import (
	"context"
	"time"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
)

// Service handles admin business logic
type Service struct {
	db                      *database.DB
	loginAttemptRepository  repository.LoginAttemptRepository
	playerRepository        repository.PlayerRepository
	clubRepository          repository.ClubRepository
	fixtureRepository       repository.FixtureRepository
	teamRepository          repository.TeamRepository
	weekRepository          repository.WeekRepository
	divisionRepository      repository.DivisionRepository
	seasonRepository        repository.SeasonRepository
	matchupRepository       repository.MatchupRepository
	fantasyRepository       repository.FantasyMixedDoublesRepository
	tennisPlayerRepository  repository.ProTennisPlayerRepository
	availabilityRepository  repository.AvailabilityRepository
	venueOverrideRepository repository.VenueOverrideRepository
	teamEligibilityService  *TeamEligibilityService
}

// NewService creates a new admin service
func NewService(db *database.DB) *Service {
	service := &Service{
		db:                      db,
		loginAttemptRepository:  repository.NewLoginAttemptRepository(db),
		playerRepository:        repository.NewPlayerRepository(db),
		clubRepository:          repository.NewClubRepository(db),
		fixtureRepository:       repository.NewFixtureRepository(db),
		teamRepository:          repository.NewTeamRepository(db),
		weekRepository:          repository.NewWeekRepository(db),
		divisionRepository:      repository.NewDivisionRepository(db),
		seasonRepository:        repository.NewSeasonRepository(db),
		matchupRepository:       repository.NewMatchupRepository(db),
		fantasyRepository:       repository.NewFantasyMixedDoublesRepository(db),
		tennisPlayerRepository:  repository.NewProTennisPlayerRepository(db),
		availabilityRepository:  repository.NewAvailabilityRepository(db),
		venueOverrideRepository: repository.NewVenueOverrideRepository(db),
	}

	// Initialize team eligibility service with reference to the main service
	service.teamEligibilityService = NewTeamEligibilityService(service)

	return service
}

// getNextWeekDateRange returns the start and end dates of the next week (Monday to Sunday)
func (s *Service) getNextWeekDateRange() (time.Time, time.Time) {
	now := time.Now()

	// Get the day of the week (0=Sunday, 1=Monday, etc.)
	dayOfWeek := int(now.Weekday())

	// Calculate days since Monday (convert Sunday=0 to Sunday=6)
	if dayOfWeek == 0 {
		dayOfWeek = 7
	}
	daysSinceMonday := dayOfWeek - 1

	// Get start of next week (Monday + 7 days)
	weekStart := now.AddDate(0, 0, -daysSinceMonday+7)
	weekStart = time.Date(weekStart.Year(), weekStart.Month(), weekStart.Day(), 0, 0, 0, 0, weekStart.Location())

	// Get end of next week (Sunday + 7 days)
	weekEnd := weekStart.AddDate(0, 0, 6)
	weekEnd = time.Date(weekEnd.Year(), weekEnd.Month(), weekEnd.Day(), 23, 59, 59, 999999999, weekEnd.Location())

	return weekStart, weekEnd
}

// GetNextWeekDateRange returns the start and end dates of the next week (Monday to Sunday) - public method
func (s *Service) GetNextWeekDateRange() (time.Time, time.Time) {
	return s.getNextWeekDateRange()
}

// Helper method to get division by ID
func (s *Service) getDivisionByID(ctx context.Context, divisionID uint) (*models.Division, error) {
	return s.divisionRepository.FindByID(ctx, divisionID)
}

// Helper method to get season by ID
func (s *Service) getSeasonByID(ctx context.Context, seasonID uint) (*models.Season, error) {
	return s.seasonRepository.FindByID(ctx, seasonID)
}
