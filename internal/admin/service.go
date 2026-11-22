// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"context"
	"net/http"
	"time"

	"jim-dot-tennis/internal/accounts"
	"jim-dot-tennis/internal/config"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
	"jim-dot-tennis/internal/services"
	"jim-dot-tennis/internal/webpush"
)

// Service handles admin business logic
type Service struct {
	db                           *database.DB
	templateDir                  string // where the two pages rendered by the service itself find templates; set by New
	homeClub                     config.HomeClubSource
	clubSettings                 *config.ClubSettings // live club settings; nil when homeClub has none (tests)
	bhpltaClubCode               string               // fallback BHPLTA code when clubSettings is nil
	loginAttemptRepository       repository.LoginAttemptRepository
	playerRepository             repository.PlayerRepository
	clubRepository               repository.ClubRepository
	fixtureRepository            repository.FixtureRepository
	teamRepository               repository.TeamRepository
	weekRepository               repository.WeekRepository
	divisionRepository           repository.DivisionRepository
	seasonRepository             repository.SeasonRepository
	leagueRepository             repository.LeagueRepository
	matchupRepository            repository.MatchupRepository
	fantasyRepository            repository.FantasyMixedDoublesRepository
	tennisPlayerRepository       repository.ProTennisPlayerRepository
	availabilityRepository       repository.AvailabilityRepository
	venueOverrideRepository      repository.VenueOverrideRepository
	tournamentProviderRepository repository.TournamentProviderRepository
	tournamentRepository         repository.TournamentRepository
	tennisPreferenceRepository   repository.PlayerTennisPreferenceRepository
	captainNoteRepository        repository.CaptainNoteRepository
	weatherService               *services.WeatherService
	teamEligibilityService       *TeamEligibilityService
	pushService                  *webpush.Service
	courthiveAPIURL              string
	accounts                     *accounts.Service
	// passkeysAvailable reports whether passkeys work at a request's address
	// (set by Handler.SetPasskeysAvailable; nil in tests).
	passkeysAvailable func(*http.Request) bool
}

// homeClubID returns the current home club ID (0 while the instance is still
// in setup). It is read live on every call: the home club can change at
// runtime when setup completes.
func (s *Service) homeClubID() uint {
	if s.homeClub == nil {
		return 0
	}
	return s.homeClub.ID()
}

// currentBHPLTAClubCode returns the effective BHPLTA club code, read live.
func (s *Service) currentBHPLTAClubCode() string {
	if s.clubSettings != nil {
		return s.clubSettings.BHPLTAClubCode()
	}
	return s.bhpltaClubCode
}

// baseURL is the prefix for absolute links in messages: the public URL, with
// the request host only as a development fallback (see config.ClubSettings.BaseURL).
func (s *Service) baseURL(r *http.Request) string { return s.clubSettings.BaseURL(r) }

// NewService creates a new admin service. homeClub is normally the server's
// *config.HomeClubHolder, which also provides the live club settings; then
// bhpltaClubCode is ignored in favour of the effective stored/env code.
func NewService(db *database.DB, courthiveAPIURL string, homeClub config.HomeClubSource, bhpltaClubCode string, pushService ...*webpush.Service) *Service {
	service := &Service{
		db:                           db,
		homeClub:                     homeClub,
		bhpltaClubCode:               bhpltaClubCode,
		loginAttemptRepository:       repository.NewLoginAttemptRepository(db),
		playerRepository:             repository.NewPlayerRepository(db),
		clubRepository:               repository.NewClubRepository(db),
		fixtureRepository:            repository.NewFixtureRepository(db),
		teamRepository:               repository.NewTeamRepository(db),
		weekRepository:               repository.NewWeekRepository(db),
		divisionRepository:           repository.NewDivisionRepository(db),
		seasonRepository:             repository.NewSeasonRepository(db),
		leagueRepository:             repository.NewLeagueRepository(db),
		matchupRepository:            repository.NewMatchupRepository(db),
		fantasyRepository:            repository.NewFantasyMixedDoublesRepository(db),
		tennisPlayerRepository:       repository.NewProTennisPlayerRepository(db),
		availabilityRepository:       repository.NewAvailabilityRepository(db),
		venueOverrideRepository:      repository.NewVenueOverrideRepository(db),
		tournamentProviderRepository: repository.NewTournamentProviderRepository(db),
		tournamentRepository:         repository.NewTournamentRepository(db),
		tennisPreferenceRepository:   repository.NewPlayerTennisPreferenceRepository(db),
		captainNoteRepository:        repository.NewCaptainNoteRepository(db),
		weatherService:               services.NewWeatherService(),
		courthiveAPIURL:              courthiveAPIURL,
		accounts:                     accounts.New(db),
	}

	if len(pushService) > 0 {
		service.pushService = pushService[0]
	}

	// The server's *config.HomeClubHolder also carries the live club settings.
	if p, ok := homeClub.(config.ClubSettingsProvider); ok {
		service.clubSettings = p.ClubSettings()
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
