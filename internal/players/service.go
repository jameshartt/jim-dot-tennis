package players

import (
	"context"
	"fmt"
	"time"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
	"jim-dot-tennis/internal/services"
)

// Service provides business logic for player operations
type Service struct {
	db                      *database.DB
	playerRepository        repository.PlayerRepository
	fantasyRepository       repository.FantasyMixedDoublesRepository
	tennisPlayerRepository  repository.ProTennisPlayerRepository
	availabilityRepository  repository.AvailabilityRepository
	seasonRepository        repository.SeasonRepository
	fixtureRepository       repository.FixtureRepository
	teamRepository          repository.TeamRepository
	divisionRepository      repository.DivisionRepository
	weekRepository          repository.WeekRepository
	clubRepository          repository.ClubRepository
	matchupRepository       repository.MatchupRepository
	venueOverrideRepository repository.VenueOverrideRepository
}

// NewService creates a new players service
func NewService(db *database.DB) *Service {
	return &Service{
		db:                      db,
		playerRepository:        repository.NewPlayerRepository(db),
		fantasyRepository:       repository.NewFantasyMixedDoublesRepository(db),
		tennisPlayerRepository:  repository.NewProTennisPlayerRepository(db),
		availabilityRepository:  repository.NewAvailabilityRepository(db),
		seasonRepository:        repository.NewSeasonRepository(db),
		fixtureRepository:       repository.NewFixtureRepository(db),
		teamRepository:          repository.NewTeamRepository(db),
		divisionRepository:      repository.NewDivisionRepository(db),
		weekRepository:          repository.NewWeekRepository(db),
		clubRepository:          repository.NewClubRepository(db),
		matchupRepository:       repository.NewMatchupRepository(db),
		venueOverrideRepository: repository.NewVenueOverrideRepository(db),
	}
}

// PlayerProfileData aggregates all profile information
type PlayerProfileData struct {
	Player             models.Player
	Club               *models.Club
	CurrentSeasonTeams []TeamWithDetails
	HistoricalTeams    []TeamWithDetails
	UpcomingFixtures   []PlayerUpcomingFixture
	AvailabilityStats  AvailabilityStats
}

// TeamWithDetails contains team info with captain and roster count
type TeamWithDetails struct {
	Team            models.Team
	Division        *models.Division
	Season          *models.Season
	CaptainNames    []string
	RosterCount     int
	IsPlayerCaptain bool
}

// AvailabilityStats summarizes player availability patterns
type AvailabilityStats struct {
	TotalAvailable      int
	TotalUnavailable    int
	TotalIfNeeded       int
	AvailabilityPercent float64
	Last28Days          []AvailabilityDay
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
	Player           PlayerInfo              `json:"player"`
	Availability     []AvailabilityDay       `json:"availability"`
	UpcomingFixtures []PlayerUpcomingFixture `json:"upcoming_fixtures"`
	ClubFixtureDates []string                `json:"club_fixture_dates"`
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

	// Get upcoming fixtures for this player
	upcomingFixtures, err := s.GetPlayerUpcomingFixtures(playerID)
	if err != nil {
		// Log the error but don't fail the whole request
		fmt.Printf("Error loading upcoming fixtures for player %s: %v\n", playerID, err)
		upcomingFixtures = []PlayerUpcomingFixture{}
	}

	// Determine the player's club to derive club-wide fixture dates
	clubFixtureDates := []string{}
	if player, err := s.playerRepository.FindByID(ctx, playerID); err == nil {
		clubID := player.ClubID
		// Query fixtures in the same date window that involve this club (home or away)
		if clubID > 0 {
			if clubFixtures, err := s.fixtureRepository.FindByClubAndDateRange(ctx, clubID, startDate, endDate); err == nil {
				// Build a set of distinct dates
				dateSet := make(map[string]struct{})
				for _, f := range clubFixtures {
					dateStr := f.ScheduledDate.Format("2006-01-02")
					dateSet[dateStr] = struct{}{}
				}
				for dateStr := range dateSet {
					clubFixtureDates = append(clubFixtureDates, dateStr)
				}
			}
		}
	}

	// For now, return mock player info with real club fixture dates derived above
	return &AvailabilityData{
		Player: PlayerInfo{
			ID:        playerID,
			FirstName: "Player", // This should be fetched from database
			LastName:  "Name",   // This should be fetched from database
		},
		Availability:     availabilityDays,
		UpcomingFixtures: upcomingFixtures,
		ClubFixtureDates: clubFixtureDates,
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

// RequestPreferredName submits a preferred name request for a player
func (s *Service) RequestPreferredName(playerID string, preferredName string) error {
	ctx := context.Background()

	// Check if the preferred name is available
	isAvailable, err := s.playerRepository.IsPreferredNameAvailable(ctx, preferredName)
	if err != nil {
		return fmt.Errorf("failed to check preferred name availability: %w", err)
	}

	if !isAvailable {
		return fmt.Errorf("preferred name already exists or is pending approval")
	}

	// Create the preferred name request
	request := &models.PreferredNameRequest{
		PlayerID:      playerID,
		RequestedName: preferredName,
		Status:        models.PreferredNamePending,
	}

	if err := s.playerRepository.CreatePreferredNameRequest(ctx, request); err != nil {
		return fmt.Errorf("failed to create preferred name request: %w", err)
	}

	return nil
}

// PlayerUpcomingFixture represents upcoming fixture information for a player (privacy-focused)
type PlayerUpcomingFixture struct {
	FixtureID         uint      `json:"fixture_id"`
	ScheduledDate     time.Time `json:"scheduled_date"`
	Division          string    `json:"division"`            // e.g. "Div. 1", "Div. 2"
	WeekNumber        int       `json:"week_number"`         // e.g. 1, 2, 3
	IsHome            bool      `json:"is_home"`             // Whether player's team is at home
	IsAway            bool      `json:"is_away"`             // Whether player's team is away
	IsDerby           bool      `json:"is_derby"`            // Whether both teams are from same club
	MyTeam            string    `json:"my_team"`             // The team the player is playing FOR
	OpponentTeam      string    `json:"opponent_team"`       // The opposing team name (no player names)
	VenueHint         string    `json:"venue_hint"`          // General location hint if available
	VenueClubName     string    `json:"venue_club_name"`     // Resolved venue club name
	IsVenueOverridden bool      `json:"is_venue_overridden"` // Whether venue has been overridden
}

// GetPlayerUpcomingFixtures retrieves upcoming fixtures where the player has been selected
func (s *Service) GetPlayerUpcomingFixtures(playerID string) ([]PlayerUpcomingFixture, error) {
	ctx := context.Background()

	// Get upcoming fixtures where this player has been selected
	fixtures, err := s.fixtureRepository.FindUpcomingFixturesForPlayer(ctx, playerID)
	if err != nil {
		return nil, fmt.Errorf("failed to find upcoming fixtures for player: %w", err)
	}

	var playerFixtures []PlayerUpcomingFixture
	for _, fixture := range fixtures {
		playerFixture := PlayerUpcomingFixture{
			FixtureID:     fixture.ID,
			ScheduledDate: fixture.ScheduledDate,
			VenueHint:     fixture.VenueLocation,
		}

		// Get division information
		if division, err := s.divisionRepository.FindByID(ctx, fixture.DivisionID); err == nil {
			playerFixture.Division = s.formatDivisionName(division.Name)
		}

		// Get week information
		if week, err := s.weekRepository.FindByID(ctx, fixture.WeekID); err == nil {
			playerFixture.WeekNumber = week.WeekNumber
		}

		// Get team information
		homeTeam, homeErr := s.teamRepository.FindByID(ctx, fixture.HomeTeamID)
		awayTeam, awayErr := s.teamRepository.FindByID(ctx, fixture.AwayTeamID)

		if homeErr == nil && awayErr == nil {
			// Determine which team the player belongs to and set appropriate information
			playerTeamID, isPlayerInHomeTeam, isPlayerInAwayTeam := s.determinePlayerTeamContext(ctx, playerID, fixture.ID, homeTeam.ID, awayTeam.ID)

			if playerTeamID > 0 {
				if isPlayerInHomeTeam {
					playerFixture.IsHome = true
					playerFixture.MyTeam = homeTeam.Name
					playerFixture.OpponentTeam = awayTeam.Name
				} else if isPlayerInAwayTeam {
					playerFixture.IsAway = true
					playerFixture.MyTeam = awayTeam.Name
					playerFixture.OpponentTeam = homeTeam.Name
				}

				// Check if it's a derby match (both teams from same club)
				if homeTeam.ClubID == awayTeam.ClubID {
					playerFixture.IsDerby = true
				}
			}
		}

		// Resolve venue
		venueResolver := services.NewVenueResolver(s.clubRepository, s.teamRepository, s.venueOverrideRepository)
		if resolution, err := venueResolver.ResolveFixtureVenue(ctx, &fixture); err == nil && resolution.Club != nil {
			playerFixture.VenueClubName = resolution.Club.Name
			playerFixture.IsVenueOverridden = resolution.IsOverridden
		}

		playerFixtures = append(playerFixtures, playerFixture)
	}

	return playerFixtures, nil
}

// formatDivisionName formats division names for display (e.g. "Division 1" -> "Div. 1")
func (s *Service) formatDivisionName(divisionName string) string {
	switch divisionName {
	case "Division 1":
		return "Div. 1"
	case "Division 2":
		return "Div. 2"
	case "Division 3":
		return "Div. 3"
	case "Division 4":
		return "Div. 4"
	default:
		return divisionName
	}
}

// determinePlayerTeamContext determines which team the player belongs to for a given fixture
// For derby matches (St Ann's vs St Ann's), uses ManagingTeamID to determine which team
// For regular matches, always assigns player to the St Ann's team regardless of stored flags
func (s *Service) determinePlayerTeamContext(ctx context.Context, playerID string, fixtureID uint, homeTeamID, awayTeamID uint) (playerTeamID uint, isHome, isAway bool) {
	// First, always check all selected players for this fixture - this is the most reliable method
	allFixturePlayers, err := s.fixtureRepository.FindSelectedPlayers(ctx, fixtureID)
	if err == nil {
		for _, fp := range allFixturePlayers {
			if fp.PlayerID == playerID {
				// Determine which teams are St Ann's first - this is the source of truth
				stAnnsClubs, clubErr := s.clubRepository.FindByNameLike(ctx, "St Ann")
				if clubErr == nil && len(stAnnsClubs) > 0 {
					stAnnsClubID := stAnnsClubs[0].ID

					homeTeam, homeErr := s.teamRepository.FindByID(ctx, homeTeamID)
					awayTeam, awayErr := s.teamRepository.FindByID(ctx, awayTeamID)

					if homeErr == nil && awayErr == nil {
						isHomeStAnns := homeTeam.ClubID == stAnnsClubID
						isAwayStAnns := awayTeam.ClubID == stAnnsClubID
						isDerby := isHomeStAnns && isAwayStAnns

						if isDerby {
							// For derby matches, use ManagingTeamID to determine which St Ann's team
							if fp.ManagingTeamID != nil {
								if *fp.ManagingTeamID == homeTeamID {
									return homeTeamID, true, false
								} else if *fp.ManagingTeamID == awayTeamID {
									return awayTeamID, false, true
								}
							}
						} else {
							// For regular matches, always assign player to St Ann's team (ignore ManagingTeamID)
							if isHomeStAnns {
								return homeTeamID, true, false
							} else if isAwayStAnns {
								return awayTeamID, false, true
							}
						}
					}
				}

				// Final fallback to stored IsHome flag if we can't determine club membership
				if fp.IsHome {
					return homeTeamID, true, false
				} else {
					return awayTeamID, false, true
				}
			}
		}
	}

	// Fallback: try team-specific queries (for derby matches)
	homeFixturePlayers, err := s.fixtureRepository.FindSelectedPlayersByTeam(ctx, fixtureID, homeTeamID)
	if err == nil {
		for _, fp := range homeFixturePlayers {
			if fp.PlayerID == playerID {
				return homeTeamID, true, false
			}
		}
	}

	awayFixturePlayers, err := s.fixtureRepository.FindSelectedPlayersByTeam(ctx, fixtureID, awayTeamID)
	if err == nil {
		for _, fp := range awayFixturePlayers {
			if fp.PlayerID == playerID {
				return awayTeamID, false, true
			}
		}
	}
	return 0, false, false
}

// GetPlayerProfileData retrieves complete profile data for a player
func (s *Service) GetPlayerProfileData(playerID string) (*PlayerProfileData, error) {
	ctx := context.Background()

	// Load player
	player, err := s.playerRepository.FindByID(ctx, playerID)
	if err != nil {
		return nil, fmt.Errorf("failed to find player: %w", err)
	}

	// Load club details
	var club *models.Club
	if player.ClubID > 0 {
		clubData, err := s.clubRepository.FindByID(ctx, player.ClubID)
		if err == nil {
			club = clubData
		}
	}

	// Get active season
	activeSeason, err := s.seasonRepository.FindActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find active season: %w", err)
	}

	// Load current season teams (returns PlayerTeam records)
	currentPlayerTeams, err := s.playerRepository.FindTeamsForPlayer(ctx, playerID, activeSeason.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to find current teams: %w", err)
	}

	// Load all historical teams (returns PlayerTeam records)
	allPlayerTeams, err := s.playerRepository.FindAllTeamsForPlayer(ctx, playerID)
	if err != nil {
		return nil, fmt.Errorf("failed to find all teams: %w", err)
	}

	// Build current team details
	currentTeamDetails := []TeamWithDetails{}
	currentTeamIDs := make(map[uint]bool)
	for _, playerTeam := range currentPlayerTeams {
		currentTeamIDs[playerTeam.TeamID] = true
		// Fetch the actual Team using TeamID
		team, err := s.teamRepository.FindByID(ctx, playerTeam.TeamID)
		if err != nil {
			continue
		}
		details, err := s.buildTeamDetails(ctx, *team, playerID, activeSeason.ID)
		if err == nil {
			currentTeamDetails = append(currentTeamDetails, details)
		}
	}

	// Build historical team details (exclude current teams)
	historicalTeamDetails := []TeamWithDetails{}
	for _, playerTeam := range allPlayerTeams {
		if !currentTeamIDs[playerTeam.TeamID] {
			// Fetch the actual Team using TeamID
			team, err := s.teamRepository.FindByID(ctx, playerTeam.TeamID)
			if err != nil {
				continue
			}
			details, err := s.buildTeamDetails(ctx, *team, playerID, playerTeam.SeasonID)
			if err == nil {
				historicalTeamDetails = append(historicalTeamDetails, details)
			}
		}
	}

	// Get upcoming fixtures
	upcomingFixtures, err := s.GetPlayerUpcomingFixtures(playerID)
	if err != nil {
		// Log but don't fail
		fmt.Printf("Error loading upcoming fixtures for player %s: %v\n", playerID, err)
		upcomingFixtures = []PlayerUpcomingFixture{}
	}

	// Get availability data and calculate stats
	availabilityData, err := s.GetPlayerAvailabilityData(playerID)
	if err != nil {
		// Log but don't fail
		fmt.Printf("Error loading availability data for player %s: %v\n", playerID, err)
		availabilityData = &AvailabilityData{Availability: []AvailabilityDay{}}
	}

	// Calculate availability statistics
	stats := s.calculateAvailabilityStats(availabilityData.Availability)

	return &PlayerProfileData{
		Player:             *player,
		Club:               club,
		CurrentSeasonTeams: currentTeamDetails,
		HistoricalTeams:    historicalTeamDetails,
		UpcomingFixtures:   upcomingFixtures,
		AvailabilityStats:  stats,
	}, nil
}

// buildTeamDetails builds detailed team information including captains and roster
func (s *Service) buildTeamDetails(ctx context.Context, team models.Team, playerID string, seasonID uint) (TeamWithDetails, error) {
	details := TeamWithDetails{
		Team: team,
	}

	// Load division
	if team.DivisionID > 0 {
		division, err := s.divisionRepository.FindByID(ctx, team.DivisionID)
		if err == nil {
			details.Division = division
		}
	}

	// Load season if provided
	if seasonID > 0 {
		season, err := s.seasonRepository.FindByID(ctx, seasonID)
		if err == nil {
			details.Season = season
		}
	}

	// Load captains
	captains, err := s.teamRepository.FindCaptainsInTeam(ctx, team.ID, seasonID)
	if err == nil {
		for _, captain := range captains {
			// Fetch player details for the captain
			captainPlayer, err := s.playerRepository.FindByID(ctx, captain.PlayerID)
			if err != nil {
				continue
			}
			captainName := captainPlayer.FirstName + " " + captainPlayer.LastName
			details.CaptainNames = append(details.CaptainNames, captainName)
			if captain.PlayerID == playerID {
				details.IsPlayerCaptain = true
			}
		}
	}

	// Count roster
	roster, err := s.teamRepository.FindPlayersInTeam(ctx, team.ID, seasonID)
	if err == nil {
		details.RosterCount = len(roster)
	}

	return details, nil
}

// calculateAvailabilityStats calculates statistics from availability data
func (s *Service) calculateAvailabilityStats(availability []AvailabilityDay) AvailabilityStats {
	stats := AvailabilityStats{
		Last28Days: availability,
	}

	for _, day := range availability {
		switch day.Status {
		case models.Available:
			stats.TotalAvailable++
		case models.Unavailable:
			stats.TotalUnavailable++
		case models.IfNeeded:
			stats.TotalIfNeeded++
		}
	}

	// Calculate percentage (available + if-needed as "potentially available")
	total := stats.TotalAvailable + stats.TotalUnavailable + stats.TotalIfNeeded
	if total > 0 {
		stats.AvailabilityPercent = float64(stats.TotalAvailable+stats.TotalIfNeeded) / float64(total) * 100
	}

	return stats
}

// GeneralAvailabilityPreference represents a day-of-week availability preference
type GeneralAvailabilityPreference struct {
	DayOfWeek string                    `json:"day_of_week"`
	Status    models.AvailabilityStatus `json:"status"`
	Notes     string                    `json:"notes,omitempty"`
}

// GetPlayerGeneralAvailability retrieves general availability preferences for a player
func (s *Service) GetPlayerGeneralAvailability(playerID string) ([]GeneralAvailabilityPreference, error) {
	ctx := context.Background()

	// Get active season
	activeSeason, err := s.seasonRepository.FindActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find active season: %w", err)
	}

	// Get general availability from repository
	availabilities, err := s.availabilityRepository.GetPlayerGeneralAvailability(ctx, playerID, activeSeason.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get general availability: %w", err)
	}

	// Convert to response format
	preferences := make([]GeneralAvailabilityPreference, 0, len(availabilities))
	for _, avail := range availabilities {
		preferences = append(preferences, GeneralAvailabilityPreference{
			DayOfWeek: avail.DayOfWeek,
			Status:    avail.Status,
			Notes:     avail.Notes,
		})
	}

	return preferences, nil
}

// UpdatePlayerGeneralAvailability updates a player's general availability for a specific day
func (s *Service) UpdatePlayerGeneralAvailability(playerID string, dayOfWeek string, status string, notes string) error {
	ctx := context.Background()

	// Get active season
	activeSeason, err := s.seasonRepository.FindActive(ctx)
	if err != nil {
		return fmt.Errorf("failed to find active season: %w", err)
	}

	// If status is "clear", delete the preference instead of upserting
	if status == "clear" {
		// Delete by setting status to Unknown (which effectively clears it)
		// We use Unknown as the default "not set" status
		if err := s.availabilityRepository.UpsertPlayerGeneralAvailability(ctx, playerID, activeSeason.ID, dayOfWeek, models.Unknown, notes); err != nil {
			return fmt.Errorf("failed to clear general availability: %w", err)
		}
		return nil
	}

	// Convert status string to AvailabilityStatus
	availStatus := s.convertFrontendStatus(status)

	// Update via repository
	if err := s.availabilityRepository.UpsertPlayerGeneralAvailability(ctx, playerID, activeSeason.ID, dayOfWeek, availStatus, notes); err != nil {
		return fmt.Errorf("failed to update general availability: %w", err)
	}

	return nil
}

// AvailabilityException represents a date range exception
type AvailabilityException struct {
	ID        uint                      `json:"id"`
	StartDate string                    `json:"start_date"` // ISO format YYYY-MM-DD
	EndDate   string                    `json:"end_date"`   // ISO format YYYY-MM-DD
	Status    models.AvailabilityStatus `json:"status"`
	Reason    string                    `json:"reason,omitempty"`
}

// GetPlayerAvailabilityExceptions retrieves all availability exceptions for a player in a date range
func (s *Service) GetPlayerAvailabilityExceptions(playerID string, startDate, endDate time.Time) ([]AvailabilityException, error) {
	ctx := context.Background()

	// Get exceptions from repository
	exceptions, err := s.availabilityRepository.GetPlayerAvailabilityByDateRange(ctx, playerID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get availability exceptions: %w", err)
	}

	// Convert to response format
	result := make([]AvailabilityException, 0, len(exceptions))
	for _, exc := range exceptions {
		result = append(result, AvailabilityException{
			ID:        exc.ID,
			StartDate: exc.StartDate.Format("2006-01-02"),
			EndDate:   exc.EndDate.Format("2006-01-02"),
			Status:    exc.Status,
			Reason:    exc.Reason,
		})
	}

	return result, nil
}

// CreateAvailabilityException creates a new availability exception for a date range
func (s *Service) CreateAvailabilityException(playerID string, startDateStr, endDateStr, status, reason string) error {
	ctx := context.Background()

	// Parse dates
	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		return fmt.Errorf("invalid start date format: %w", err)
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		return fmt.Errorf("invalid end date format: %w", err)
	}

	// Validate date range
	if endDate.Before(startDate) {
		return fmt.Errorf("end date must be after or equal to start date")
	}

	// Convert status
	availStatus := s.convertFrontendStatus(status)
	if availStatus == "clear" || availStatus == models.Unknown {
		return fmt.Errorf("invalid status for exception: cannot create exception with 'clear' or 'unknown' status")
	}

	// Create exception for each day in the range
	// This approach allows for easy deletion of individual days if needed
	currentDate := startDate
	for !currentDate.After(endDate) {
		if err := s.availabilityRepository.UpsertPlayerAvailability(ctx, playerID, currentDate, availStatus, reason); err != nil {
			return fmt.Errorf("failed to create availability exception: %w", err)
		}
		currentDate = currentDate.AddDate(0, 0, 1)
	}

	return nil
}

// DeleteAvailabilityException deletes an availability exception by removing all days in the date range
func (s *Service) DeleteAvailabilityException(playerID string, startDateStr, endDateStr string) error {
	ctx := context.Background()

	// Parse dates
	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		return fmt.Errorf("invalid start date format: %w", err)
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		return fmt.Errorf("invalid end date format: %w", err)
	}

	// Delete exception for each day in the range
	// Note: The repository stores exceptions with end_date = start_date + 24 hours,
	// so we need to delete using the stored format
	currentDate := startDate
	for !currentDate.After(endDate) {
		// Delete using SQL directly to match the actual storage format
		_, err := s.db.ExecContext(ctx, `
			DELETE FROM player_availability_exceptions
			WHERE player_id = ?
			AND start_date >= ?
			AND start_date < ?
		`, playerID, currentDate, currentDate.AddDate(0, 0, 1))

		if err != nil {
			// Log but continue - some days might not have exceptions
			fmt.Printf("Warning: failed to delete exception for %s: %v\n", currentDate.Format("2006-01-02"), err)
		}
		currentDate = currentDate.AddDate(0, 0, 1)
	}

	return nil
}

// PlayerMatchRecord represents a single match record for a player
type PlayerMatchRecord struct {
	FixtureDate   time.Time `json:"fixture_date"`
	DivisionName  string    `json:"division_name"`
	MatchupType   string    `json:"matchup_type"`
	PartnerName   string    `json:"partner_name"`
	OpponentNames string    `json:"opponent_names"`
	SetScores     string    `json:"set_scores"`
	IsHome        bool      `json:"is_home"`
	WonMatch      bool      `json:"won_match"`
	DrawnMatch    bool      `json:"drawn_match"`
	HomeTeamName  string    `json:"home_team_name"`
	AwayTeamName  string    `json:"away_team_name"`
}

// PlayerMatchStats aggregates match statistics
type PlayerMatchStats struct {
	TotalMatches int     `json:"total_matches"`
	Wins         int     `json:"wins"`
	Losses       int     `json:"losses"`
	Draws        int     `json:"draws"`
	WinRate      float64 `json:"win_rate"`
}

// GetPlayerMatchHistory retrieves completed match records for a player
func (s *Service) GetPlayerMatchHistory(playerID string, seasonID *uint) ([]PlayerMatchRecord, *PlayerMatchStats, error) {
	ctx := context.Background()

	// Build the query based on season filter
	query := `
		SELECT
			f.scheduled_date,
			COALESCE(d.name, '') AS division_name,
			m.type AS matchup_type,
			m.home_score,
			m.away_score,
			m.home_set1, m.away_set1,
			m.home_set2, m.away_set2,
			m.home_set3, m.away_set3,
			mp.is_home,
			m.id AS matchup_id,
			f.id AS fixture_id,
			COALESCE(ht.name, '') AS home_team_name,
			COALESCE(at2.name, '') AS away_team_name
		FROM matchup_players mp
		JOIN matchups m ON mp.matchup_id = m.id
		JOIN fixtures f ON m.fixture_id = f.id
		LEFT JOIN divisions d ON f.division_id = d.id
		LEFT JOIN teams ht ON f.home_team_id = ht.id
		LEFT JOIN teams at2 ON f.away_team_id = at2.id
		WHERE mp.player_id = ?
		AND m.status = 'Finished'
		AND f.status = 'Completed'
	`

	args := []interface{}{playerID}
	if seasonID != nil {
		query += " AND f.season_id = ?"
		args = append(args, *seasonID)
	}
	query += " ORDER BY f.scheduled_date DESC, m.type ASC"

	type matchRow struct {
		ScheduledDate time.Time `db:"scheduled_date"`
		DivisionName  string    `db:"division_name"`
		MatchupType   string    `db:"matchup_type"`
		HomeScore     int       `db:"home_score"`
		AwayScore     int       `db:"away_score"`
		HomeSet1      *int      `db:"home_set1"`
		AwaySet1      *int      `db:"away_set1"`
		HomeSet2      *int      `db:"home_set2"`
		AwaySet2      *int      `db:"away_set2"`
		HomeSet3      *int      `db:"home_set3"`
		AwaySet3      *int      `db:"away_set3"`
		IsHome        bool      `db:"is_home"`
		MatchupID     uint      `db:"matchup_id"`
		FixtureID     uint      `db:"fixture_id"`
		HomeTeamName  string    `db:"home_team_name"`
		AwayTeamName  string    `db:"away_team_name"`
	}

	var rows []matchRow
	if err := s.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, nil, fmt.Errorf("failed to query match history: %w", err)
	}

	var records []PlayerMatchRecord
	stats := &PlayerMatchStats{}

	for _, row := range rows {
		// Format set scores
		setScores := ""
		if row.HomeSet1 != nil && row.AwaySet1 != nil {
			setScores = fmt.Sprintf("%d-%d", *row.HomeSet1, *row.AwaySet1)
		}
		if row.HomeSet2 != nil && row.AwaySet2 != nil {
			setScores += fmt.Sprintf(", %d-%d", *row.HomeSet2, *row.AwaySet2)
		}
		if row.HomeSet3 != nil && row.AwaySet3 != nil {
			setScores += fmt.Sprintf(", %d-%d", *row.HomeSet3, *row.AwaySet3)
		}

		// Find partner and opponents
		allPlayers, _ := s.matchupRepository.FindPlayersInMatchup(ctx, row.MatchupID)
		partnerName := ""
		var opponentNames []string
		for _, mp := range allPlayers {
			if mp.PlayerID == playerID {
				continue
			}
			p, err := s.playerRepository.FindByID(ctx, mp.PlayerID)
			if err != nil {
				continue
			}
			name := p.FirstName + " " + p.LastName
			if mp.IsHome == row.IsHome {
				partnerName = name
			} else {
				opponentNames = append(opponentNames, name)
			}
		}
		opponents := ""
		if len(opponentNames) > 0 {
			opponents = opponentNames[0]
			if len(opponentNames) > 1 {
				opponents += " & " + opponentNames[1]
			}
		}

		// Determine win/loss/draw
		wonMatch := false
		drawnMatch := false
		if row.IsHome {
			wonMatch = row.HomeScore > row.AwayScore
			drawnMatch = row.HomeScore == row.AwayScore
		} else {
			wonMatch = row.AwayScore > row.HomeScore
			drawnMatch = row.HomeScore == row.AwayScore
		}

		records = append(records, PlayerMatchRecord{
			FixtureDate:   row.ScheduledDate,
			DivisionName:  row.DivisionName,
			MatchupType:   row.MatchupType,
			PartnerName:   partnerName,
			OpponentNames: opponents,
			SetScores:     setScores,
			IsHome:        row.IsHome,
			WonMatch:      wonMatch,
			DrawnMatch:    drawnMatch,
			HomeTeamName:  row.HomeTeamName,
			AwayTeamName:  row.AwayTeamName,
		})

		stats.TotalMatches++
		if wonMatch {
			stats.Wins++
		} else if drawnMatch {
			stats.Draws++
		} else {
			stats.Losses++
		}
	}

	if stats.TotalMatches > 0 {
		stats.WinRate = float64(stats.Wins) / float64(stats.TotalMatches) * 100
	}

	return records, stats, nil
}
