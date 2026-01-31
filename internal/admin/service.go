package admin

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
)

// Service handles admin business logic
type Service struct {
	db                     *database.DB
	loginAttemptRepository repository.LoginAttemptRepository
	playerRepository       repository.PlayerRepository
	clubRepository         repository.ClubRepository
	fixtureRepository      repository.FixtureRepository
	teamRepository         repository.TeamRepository
	weekRepository         repository.WeekRepository
	divisionRepository     repository.DivisionRepository
	seasonRepository       repository.SeasonRepository
	matchupRepository      repository.MatchupRepository
	fantasyRepository      repository.FantasyMixedDoublesRepository
	tennisPlayerRepository repository.ProTennisPlayerRepository
	availabilityRepository repository.AvailabilityRepository
	teamEligibilityService *TeamEligibilityService
}

// NewService creates a new admin service
func NewService(db *database.DB) *Service {
	service := &Service{
		db:                     db,
		loginAttemptRepository: repository.NewLoginAttemptRepository(db),
		playerRepository:       repository.NewPlayerRepository(db),
		clubRepository:         repository.NewClubRepository(db),
		fixtureRepository:      repository.NewFixtureRepository(db),
		teamRepository:         repository.NewTeamRepository(db),
		weekRepository:         repository.NewWeekRepository(db),
		divisionRepository:     repository.NewDivisionRepository(db),
		seasonRepository:       repository.NewSeasonRepository(db),
		matchupRepository:      repository.NewMatchupRepository(db),
		fantasyRepository:      repository.NewFantasyMixedDoublesRepository(db),
		tennisPlayerRepository: repository.NewProTennisPlayerRepository(db),
		availabilityRepository: repository.NewAvailabilityRepository(db),
	}

	// Initialize team eligibility service with reference to the main service
	service.teamEligibilityService = NewTeamEligibilityService(service)

	return service
}

// DashboardData represents the data needed for the admin dashboard
type DashboardData struct {
	Stats         Stats          `json:"stats"`
	LoginAttempts []LoginAttempt `json:"login_attempts"`
}

// Stats represents admin dashboard statistics
type Stats struct {
	PlayerCount                  int `json:"player_count"`
	FixtureCount                 int `json:"fixture_count"`
	TeamCount                    int `json:"team_count"`
	PendingPreferredNameRequests int `json:"pending_preferred_name_requests"`
}

// LoginAttempt represents a login attempt record
type LoginAttempt struct {
	Username  string    `json:"username"`
	IP        string    `json:"ip"`
	CreatedAt time.Time `json:"created_at"`
	Success   bool      `json:"success"`
}

// FixtureWithRelations represents a fixture with its related entities loaded
type FixtureWithRelations struct {
	models.Fixture
	HomeTeam           *models.Team     `json:"home_team,omitempty"`
	AwayTeam           *models.Team     `json:"away_team,omitempty"`
	Week               *models.Week     `json:"week,omitempty"`
	Division           *models.Division `json:"division,omitempty"`
	Season             *models.Season   `json:"season,omitempty"`
	IsStAnnsHome       bool             `json:"is_stanns_home"`
	IsStAnnsAway       bool             `json:"is_stanns_away"`
	IsDerby            bool             `json:"is_derby"`                       // Both teams are St Ann's
	DefaultTeamContext *models.Team     `json:"default_team_context,omitempty"` // Which team to manage by default
}

// FixtureDetail represents a fixture with comprehensive related data for detail view
type FixtureDetail struct {
	models.Fixture
	HomeTeam          *models.Team             `json:"home_team,omitempty"`
	AwayTeam          *models.Team             `json:"away_team,omitempty"`
	Week              *models.Week             `json:"week,omitempty"`
	Division          *models.Division         `json:"division,omitempty"`
	Season            *models.Season           `json:"season,omitempty"`
	DayCaptain        *models.Player           `json:"day_captain,omitempty"`
	Matchups          []MatchupWithPlayers     `json:"matchups,omitempty"`
	SelectedPlayers   []SelectedPlayerInfo     `json:"selected_players,omitempty"`
	DuplicateWarnings []DuplicatePlayerWarning `json:"duplicate_warnings,omitempty"`
}

// SelectedPlayerInfo represents a player selected for a fixture with additional context
type SelectedPlayerInfo struct {
	models.FixturePlayer
	Player             models.Player             `json:"player"`
	AvailabilityStatus models.AvailabilityStatus `json:"availability_status"`
	AvailabilityNotes  string                    `json:"availability_notes"`
}

// MatchupPlayerWithInfo represents a matchup player with their details
type MatchupPlayerWithInfo struct {
	MatchupPlayer models.MatchupPlayer `json:"matchup_player"`
	Player        models.Player        `json:"player"`
}

// MatchupWithPlayers represents a matchup with its assigned players
type MatchupWithPlayers struct {
	Matchup models.Matchup          `json:"matchup"`
	Players []MatchupPlayerWithInfo `json:"players"`
}

// DuplicatePlayerWarning represents a warning about duplicate player assignments
type DuplicatePlayerWarning struct {
	PlayerID   string   `json:"player_id"`
	PlayerName string   `json:"player_name"`
	Matchups   []string `json:"matchups"` // List of matchup types where this player appears
}

// FixtureDetailWithMatchups represents fixture detail with matchups and players
type FixtureDetailWithMatchups struct {
	FixtureDetail       FixtureDetail        `json:"fixture_detail"`
	MatchupsWithPlayers []MatchupWithPlayers `json:"matchups_with_players"`
}

// TeamWithRelations represents a team with its related data for display
type TeamWithRelations struct {
	models.Team
	Division    *models.Division `json:"division,omitempty"`
	Season      *models.Season   `json:"season,omitempty"`
	Captain     *models.Player   `json:"captain,omitempty"`
	PlayerCount int              `json:"player_count"`
}

// TeamDetail represents a team with comprehensive related data for detail view
type TeamDetail struct {
	models.Team
	Club        *models.Club      `json:"club,omitempty"`
	Division    *models.Division  `json:"division,omitempty"`
	Season      *models.Season    `json:"season,omitempty"`
	Captains    []CaptainWithInfo `json:"captains,omitempty"`
	Players     []PlayerInTeam    `json:"players,omitempty"`
	PlayerCount int               `json:"player_count"`
}

// CaptainWithInfo represents a captain with their player information
type CaptainWithInfo struct {
	models.Captain
	Player models.Player `json:"player"`
}

// PlayerInTeam represents a player with their team membership details
type PlayerInTeam struct {
	models.Player
	PlayerTeam models.PlayerTeam `json:"player_team"`
}

// PlayerWithAvailability combines player information with their availability status for a fixture
type PlayerWithAvailability struct {
	Player             models.Player
	AvailabilityStatus models.AvailabilityStatus
	AvailabilityNotes  string
}

// PlayerWithEligibility combines player information with availability and eligibility for team selection
type PlayerWithEligibility struct {
	Player             models.Player
	AvailabilityStatus models.AvailabilityStatus
	AvailabilityNotes  string
	Eligibility        *PlayerEligibilityInfo
}

// PlayerWithAvailabilityInfo combines player information with availability status for admin display
type PlayerWithAvailabilityInfo struct {
	Player                   models.Player `json:"player"`
	HasAvailabilityURL       bool          `json:"has_availability_url"`
	HasSetNextWeekAvail      bool          `json:"has_set_next_week_avail"`
	NextWeekAvailCount       int           `json:"next_week_avail_count"`
	TeamAppearanceCounts     map[uint]int  `json:"team_appearance_counts"`
	DivisionAppearanceCounts map[uint]int  `json:"division_appearance_counts"`
}

// GetDashboardData retrieves data for the admin dashboard
func (s *Service) GetDashboardData(user *models.User) (*DashboardData, error) {
	ctx := context.Background()

	// Get actual player count from database
	playerCount, err := s.playerRepository.CountAll(ctx)
	if err != nil {
		return nil, err
	}

	// Get team count for St. Ann's club
	teamCount, err := s.getStAnnsTeamCount(ctx)
	if err != nil {
		teamCount = 0 // Default to 0 if error
	}

	// Get fixture count for St. Ann's club
	fixtureCount, err := s.getStAnnsFixtureCount(ctx)
	if err != nil {
		fixtureCount = 0 // Default to 0 if error
	}

	// Get pending preferred name requests count
	pendingPreferredNameRequests, err := s.playerRepository.CountPendingPreferredNameRequests(ctx)
	if err != nil {
		pendingPreferredNameRequests = 0 // Default to 0 if error
	}

	stats := Stats{
		PlayerCount:                  playerCount,
		FixtureCount:                 fixtureCount,
		TeamCount:                    teamCount,
		PendingPreferredNameRequests: pendingPreferredNameRequests,
	}

	// Query login attempts for the current user using repository
	dbLoginAttempts, err := s.loginAttemptRepository.FindByUsername(user.Username, 10)
	if err != nil {
		return nil, err
	}

	// Convert to admin service LoginAttempt struct (which doesn't include user_agent)
	loginAttempts := make([]LoginAttempt, len(dbLoginAttempts))
	for i, attempt := range dbLoginAttempts {
		loginAttempts[i] = LoginAttempt{
			Username:  attempt.Username,
			IP:        attempt.IP,
			CreatedAt: attempt.CreatedAt,
			Success:   attempt.Success,
		}
	}

	return &DashboardData{
		Stats:         stats,
		LoginAttempts: loginAttempts,
	}, nil
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

// getCurrentWeekDateRange returns the start and end dates of the current week (Monday to Sunday)
func (s *Service) getCurrentWeekDateRange() (time.Time, time.Time) {
	now := time.Now()

	// Get the day of the week (0=Sunday, 1=Monday, etc.)
	dayOfWeek := int(now.Weekday())

	// Calculate days since Monday (convert Sunday=0 to Sunday=6)
	if dayOfWeek == 0 {
		dayOfWeek = 7
	}
	daysSinceMonday := dayOfWeek - 1

	// Get start of week (Monday)
	weekStart := now.AddDate(0, 0, -daysSinceMonday)
	weekStart = time.Date(weekStart.Year(), weekStart.Month(), weekStart.Day(), 0, 0, 0, 0, weekStart.Location())

	// Get end of week (Sunday)
	weekEnd := weekStart.AddDate(0, 0, 6)
	weekEnd = time.Date(weekEnd.Year(), weekEnd.Month(), weekEnd.Day(), 23, 59, 59, 999999999, weekEnd.Location())

	return weekStart, weekEnd
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

// UpdatePlayer updates a player's information
func (s *Service) UpdatePlayer(player *models.Player) error {
	return s.playerRepository.Update(context.Background(), player)
}

// GetFixtures retrieves all fixtures for admin management
func (s *Service) GetFixtures() (interface{}, error) {
	// TODO: Implement fixture retrieval from database
	return nil, nil
}

// GetStAnnsFixtures retrieves upcoming fixtures for St. Ann's club with related data
func (s *Service) GetStAnnsFixtures() (*models.Club, []FixtureWithRelations, error) {
	ctx := context.Background()

	// Find St. Ann's club
	clubs, err := s.clubRepository.FindByNameLike(ctx, "St Ann")
	if err != nil {
		return nil, nil, err
	}
	if len(clubs) == 0 {
		return nil, nil, nil // No club found
	}
	stAnnsClub := &clubs[0]

	// Get all teams for St. Ann's club
	teams, err := s.teamRepository.FindByClub(ctx, stAnnsClub.ID)
	if err != nil {
		return stAnnsClub, nil, err
	}

	if len(teams) == 0 {
		return stAnnsClub, nil, nil // No teams found
	}

	// Get upcoming fixtures for all St. Ann's teams
	var allFixtures []models.Fixture
	fixtureMap := make(map[uint]models.Fixture) // Use map to deduplicate fixtures by ID

	for _, team := range teams {
		teamFixtures, err := s.fixtureRepository.FindByTeam(ctx, team.ID)
		if err != nil {
			continue // Skip this team if there's an error
		}
		// Add fixtures to map to automatically deduplicate
		for _, fixture := range teamFixtures {
			fixtureMap[fixture.ID] = fixture
		}
	}

	// Convert map back to slice
	for _, fixture := range fixtureMap {
		allFixtures = append(allFixtures, fixture)
	}

	// Filter for upcoming fixtures (scheduled or in progress) from tomorrow onwards
	var upcomingFixtures []models.Fixture
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrowStart := todayStart.Add(24 * time.Hour)
	for _, fixture := range allFixtures {
		if fixture.Status == models.Scheduled || fixture.Status == models.InProgress {
			// Upcoming list excludes today's fixtures; those are shown separately
			if !fixture.ScheduledDate.Before(tomorrowStart) {
				upcomingFixtures = append(upcomingFixtures, fixture)
			}
		}
	}

	// Build FixtureWithRelations by fetching related data
	fixturesWithRelations := s.buildFixturesWithRelations(ctx, upcomingFixtures, stAnnsClub)

	// Sort fixtures by scheduled date (nearest first), then by division (descending)
	sort.Slice(fixturesWithRelations, func(i, j int) bool {
		// First sort by date (ascending)
		if fixturesWithRelations[i].ScheduledDate.Before(fixturesWithRelations[j].ScheduledDate) {
			return true
		}
		if fixturesWithRelations[i].ScheduledDate.After(fixturesWithRelations[j].ScheduledDate) {
			return false
		}

		// If dates are equal, sort by division (descending - Division 4 before Division 3)
		divisionI := ""
		divisionJ := ""
		if fixturesWithRelations[i].Division != nil {
			divisionI = fixturesWithRelations[i].Division.Name
		}
		if fixturesWithRelations[j].Division != nil {
			divisionJ = fixturesWithRelations[j].Division.Name
		}

		// For descending order, return i > j
		return divisionI > divisionJ
	})

	return stAnnsClub, fixturesWithRelations, nil
}

// GetStAnnsPastFixtures retrieves past fixtures for St. Ann's club with related data
func (s *Service) GetStAnnsPastFixtures() (*models.Club, []FixtureWithRelations, error) {
	ctx := context.Background()

	// Find St. Ann's club
	clubs, err := s.clubRepository.FindByNameLike(ctx, "St Ann")
	if err != nil {
		return nil, nil, err
	}
	if len(clubs) == 0 {
		return nil, nil, nil // No club found
	}
	stAnnsClub := &clubs[0]

	// Get all teams for St. Ann's club
	teams, err := s.teamRepository.FindByClub(ctx, stAnnsClub.ID)
	if err != nil {
		return stAnnsClub, nil, err
	}

	if len(teams) == 0 {
		return stAnnsClub, nil, nil // No teams found
	}

	// Get all fixtures for all St. Ann's teams
	var allFixtures []models.Fixture
	fixtureMap := make(map[uint]models.Fixture) // Use map to deduplicate fixtures by ID

	for _, team := range teams {
		teamFixtures, err := s.fixtureRepository.FindByTeam(ctx, team.ID)
		if err != nil {
			continue // Skip this team if there's an error
		}
		// Add fixtures to map to automatically deduplicate
		for _, fixture := range teamFixtures {
			fixtureMap[fixture.ID] = fixture
		}
	}

	// Convert map back to slice
	for _, fixture := range fixtureMap {
		allFixtures = append(allFixtures, fixture)
	}

	// Filter for past fixtures (completed/cancelled/postponed or scheduled/in-progress before today)
	var pastFixtures []models.Fixture
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	for _, fixture := range allFixtures {
		if fixture.Status == models.Completed || fixture.Status == models.Cancelled || fixture.Status == models.Postponed {
			pastFixtures = append(pastFixtures, fixture)
		} else if fixture.Status == models.Scheduled || fixture.Status == models.InProgress {
			if fixture.ScheduledDate.Before(todayStart) {
				pastFixtures = append(pastFixtures, fixture)
			}
		}
	}

	// Build FixtureWithRelations by fetching related data
	fixturesWithRelations := s.buildFixturesWithRelations(ctx, pastFixtures, stAnnsClub)

	// Sort fixtures by scheduled date (most recent first), then by division (ascending)
	sort.Slice(fixturesWithRelations, func(i, j int) bool {
		// First sort by date (descending - most recent first)
		if fixturesWithRelations[i].ScheduledDate.After(fixturesWithRelations[j].ScheduledDate) {
			return true
		}
		if fixturesWithRelations[i].ScheduledDate.Before(fixturesWithRelations[j].ScheduledDate) {
			return false
		}

		// If dates are equal, sort by division (ascending - Division 3 before Division 4)
		divisionI := ""
		divisionJ := ""
		if fixturesWithRelations[i].Division != nil {
			divisionI = fixturesWithRelations[i].Division.Name
		}
		if fixturesWithRelations[j].Division != nil {
			divisionJ = fixturesWithRelations[j].Division.Name
		}

		// For ascending order, return i < j
		return divisionI < divisionJ
	})

	return stAnnsClub, fixturesWithRelations, nil
}

// GetStAnnsTodaysFixtures retrieves today's fixtures for St. Ann's club with related data
func (s *Service) GetStAnnsTodaysFixtures() (*models.Club, []FixtureWithRelations, error) {
	ctx := context.Background()

	// Find St. Ann's club
	clubs, err := s.clubRepository.FindByNameLike(ctx, "St Ann")
	if err != nil {
		return nil, nil, err
	}
	if len(clubs) == 0 {
		return nil, nil, nil // No club found
	}
	stAnnsClub := &clubs[0]

	// Get all teams for St. Ann's club
	teams, err := s.teamRepository.FindByClub(ctx, stAnnsClub.ID)
	if err != nil {
		return stAnnsClub, nil, err
	}

	if len(teams) == 0 {
		return stAnnsClub, nil, nil // No teams found
	}

	// Collect fixtures for today across all teams (deduped)
	var allFixtures []models.Fixture
	fixtureMap := make(map[uint]models.Fixture)

	for _, team := range teams {
		teamFixtures, err := s.fixtureRepository.FindByTeam(ctx, team.ID)
		if err != nil {
			continue
		}
		for _, fixture := range teamFixtures {
			fixtureMap[fixture.ID] = fixture
		}
	}

	for _, fixture := range fixtureMap {
		allFixtures = append(allFixtures, fixture)
	}

	// Compute today's boundaries
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrowStart := todayStart.Add(24 * time.Hour)

	// Filter for today's fixtures
	var todaysFixtures []models.Fixture
	for _, fixture := range allFixtures {
		if fixture.Status == models.Scheduled || fixture.Status == models.InProgress {
			if !fixture.ScheduledDate.Before(todayStart) && fixture.ScheduledDate.Before(tomorrowStart) {
				todaysFixtures = append(todaysFixtures, fixture)
			}
		}
	}

	fixturesWithRelations := s.buildFixturesWithRelations(ctx, todaysFixtures, stAnnsClub)

	// Sort by time then division name desc
	sort.Slice(fixturesWithRelations, func(i, j int) bool {
		if fixturesWithRelations[i].ScheduledDate.Before(fixturesWithRelations[j].ScheduledDate) {
			return true
		}
		if fixturesWithRelations[i].ScheduledDate.After(fixturesWithRelations[j].ScheduledDate) {
			return false
		}
		divisionI := ""
		divisionJ := ""
		if fixturesWithRelations[i].Division != nil {
			divisionI = fixturesWithRelations[i].Division.Name
		}
		if fixturesWithRelations[j].Division != nil {
			divisionJ = fixturesWithRelations[j].Division.Name
		}
		return divisionI > divisionJ
	})

	return stAnnsClub, fixturesWithRelations, nil
}

// buildFixturesWithRelations is a helper method to build FixtureWithRelations from fixtures
func (s *Service) buildFixturesWithRelations(ctx context.Context, fixtures []models.Fixture, stAnnsClub *models.Club) []FixtureWithRelations {
	var fixturesWithRelations []FixtureWithRelations

	for _, fixture := range fixtures {
		fixtureWithRelations := FixtureWithRelations{
			Fixture: fixture,
		}

		// Declare team variables for later use
		var homeTeam, awayTeam *models.Team

		// Get home team
		if team, err := s.teamRepository.FindByID(ctx, fixture.HomeTeamID); err == nil {
			homeTeam = team
			fixtureWithRelations.HomeTeam = homeTeam
		}

		// Get away team
		if team, err := s.teamRepository.FindByID(ctx, fixture.AwayTeamID); err == nil {
			awayTeam = team
			fixtureWithRelations.AwayTeam = awayTeam
		}

		// Get week
		if week, err := s.weekRepository.FindByID(ctx, fixture.WeekID); err == nil {
			fixtureWithRelations.Week = week
		}

		// Get division
		if division, err := s.getDivisionByID(ctx, fixture.DivisionID); err == nil {
			fixtureWithRelations.Division = division
		}

		// Get season
		if season, err := s.getSeasonByID(ctx, fixture.SeasonID); err == nil {
			fixtureWithRelations.Season = season
		}

		// Determine if St. Ann's is home or away (only if teams were loaded successfully)
		if homeTeam != nil && homeTeam.ClubID == stAnnsClub.ID {
			fixtureWithRelations.IsStAnnsHome = true
		}
		if awayTeam != nil && awayTeam.ClubID == stAnnsClub.ID {
			fixtureWithRelations.IsStAnnsAway = true
		}

		// Determine if it's a derby match (both teams are St Ann's)
		if homeTeam != nil && awayTeam != nil &&
			homeTeam.ClubID == stAnnsClub.ID && awayTeam.ClubID == stAnnsClub.ID {

			// For derby matches, create TWO separate entries - one for each team's perspective

			// First entry: Home team perspective
			homeFixture := fixtureWithRelations
			homeFixture.IsDerby = true
			homeFixture.DefaultTeamContext = homeTeam
			fixturesWithRelations = append(fixturesWithRelations, homeFixture)

			// Second entry: Away team perspective
			awayFixture := fixtureWithRelations
			awayFixture.IsDerby = true
			awayFixture.DefaultTeamContext = awayTeam
			fixturesWithRelations = append(fixturesWithRelations, awayFixture)
		} else {
			// Regular match: only one entry
			fixturesWithRelations = append(fixturesWithRelations, fixtureWithRelations)
		}
	}

	return fixturesWithRelations
}

// GetAllDivisions retrieves all divisions for filtering
func (s *Service) GetAllDivisions() ([]models.Division, error) {
	ctx := context.Background()
	return s.divisionRepository.FindAll(ctx)
}

// GetUsers retrieves all users for admin management
func (s *Service) GetUsers() (interface{}, error) {
	// TODO: Implement user retrieval from database
	return nil, nil
}

// GetSessions retrieves all active sessions for admin viewing
func (s *Service) GetSessions() (interface{}, error) {
	// TODO: Implement session retrieval from database
	return nil, nil
}

// GetFixtureDetail retrieves comprehensive details for a specific fixture
func (s *Service) GetFixtureDetail(fixtureID uint) (*FixtureDetail, error) {
	ctx := context.Background()

	// Get the base fixture
	fixture, err := s.fixtureRepository.FindByID(ctx, fixtureID)
	if err != nil {
		return nil, err
	}

	// Create the detail struct
	detail := &FixtureDetail{
		Fixture: *fixture,
	}

	// Get home team
	if homeTeam, err := s.teamRepository.FindByID(ctx, fixture.HomeTeamID); err == nil {
		detail.HomeTeam = homeTeam
	}

	// Get away team
	if awayTeam, err := s.teamRepository.FindByID(ctx, fixture.AwayTeamID); err == nil {
		detail.AwayTeam = awayTeam
	}

	// Get week
	if week, err := s.weekRepository.FindByID(ctx, fixture.WeekID); err == nil {
		detail.Week = week
	}

	// Get division
	if division, err := s.getDivisionByID(ctx, fixture.DivisionID); err == nil {
		detail.Division = division
	}

	// Get season
	if season, err := s.getSeasonByID(ctx, fixture.SeasonID); err == nil {
		detail.Season = season
	}

	// Get day captain if assigned
	if fixture.DayCaptainID != nil {
		if dayCaptain, err := s.playerRepository.FindByID(ctx, *fixture.DayCaptainID); err == nil {
			detail.DayCaptain = dayCaptain
		}
	}

	// Get matchups with players for the fixture
	if matchups, err := s.matchupRepository.FindByFixture(ctx, fixtureID); err == nil {
		var matchupsWithPlayers []MatchupWithPlayers
		for _, matchup := range matchups {
			// Get players for this matchup
			matchupPlayers, err := s.matchupRepository.FindPlayersInMatchup(ctx, matchup.ID)
			if err != nil {
				// If we can't get players, still include the matchup with empty players
				matchupsWithPlayers = append(matchupsWithPlayers, MatchupWithPlayers{
					Matchup: matchup,
					Players: []MatchupPlayerWithInfo{},
				})
				continue
			}

			var playersWithInfo []MatchupPlayerWithInfo
			for _, mp := range matchupPlayers {
				if player, err := s.playerRepository.FindByID(ctx, mp.PlayerID); err == nil {
					playersWithInfo = append(playersWithInfo, MatchupPlayerWithInfo{
						MatchupPlayer: mp,
						Player:        *player,
					})
				}
			}

			matchupsWithPlayers = append(matchupsWithPlayers, MatchupWithPlayers{
				Matchup: matchup,
				Players: playersWithInfo,
			})
		}

		// Sort matchups in the desired order: 1st Mixed, 2nd Mixed, Mens, Womens
		sort.Slice(matchupsWithPlayers, func(i, j int) bool {
			return getMatchupOrder(matchupsWithPlayers[i].Matchup.Type) < getMatchupOrder(matchupsWithPlayers[j].Matchup.Type)
		})

		detail.Matchups = matchupsWithPlayers

		// Check for duplicate players across matchups
		detail.DuplicateWarnings = s.detectDuplicatePlayersInMatchups(matchupsWithPlayers)
	}

	// Get selected players for the fixture
	if selectedPlayers, err := s.fixtureRepository.FindSelectedPlayers(ctx, fixtureID); err == nil {
		var selectedPlayerInfos []SelectedPlayerInfo
		for _, sp := range selectedPlayers {
			if player, err := s.playerRepository.FindByID(ctx, sp.PlayerID); err == nil {
				// Get availability information for this player and fixture
				availability := s.determinePlayerAvailabilityForFixture(ctx, sp.PlayerID, fixtureID, fixture.ScheduledDate)

				selectedPlayerInfos = append(selectedPlayerInfos, SelectedPlayerInfo{
					FixturePlayer:      sp,
					Player:             *player,
					AvailabilityStatus: availability.Status,
					AvailabilityNotes:  availability.Notes,
				})
			}
		}
		detail.SelectedPlayers = selectedPlayerInfos
	}

	return detail, nil
}

// detectDuplicatePlayersInMatchups checks for players assigned to multiple matchups
func (s *Service) detectDuplicatePlayersInMatchups(matchups []MatchupWithPlayers) []DuplicatePlayerWarning {
	// Map to track which matchups each player appears in
	playerMatchups := make(map[string][]string)
	playerNames := make(map[string]string)

	// Collect all player assignments
	for _, matchup := range matchups {
		matchupType := string(matchup.Matchup.Type)
		for _, player := range matchup.Players {
			playerID := player.Player.ID
			playerName := player.Player.FirstName + " " + player.Player.LastName

			playerMatchups[playerID] = append(playerMatchups[playerID], matchupType)
			playerNames[playerID] = playerName
		}
	}

	// Find duplicates
	var warnings []DuplicatePlayerWarning
	for playerID, matchupTypes := range playerMatchups {
		if len(matchupTypes) > 1 {
			warnings = append(warnings, DuplicatePlayerWarning{
				PlayerID:   playerID,
				PlayerName: playerNames[playerID],
				Matchups:   matchupTypes,
			})
		}
	}

	return warnings
}

// getMatchupOrder returns the sort order for matchup types
// Order: 1st Mixed, 2nd Mixed, Mens, Womens
func getMatchupOrder(matchupType models.MatchupType) int {
	switch matchupType {
	case models.FirstMixed:
		return 0
	case models.SecondMixed:
		return 1
	case models.Mens:
		return 2
	case models.Womens:
		return 3
	default:
		return 4 // Unknown types go last
	}
}

// Helper method to get division by ID
func (s *Service) getDivisionByID(ctx context.Context, divisionID uint) (*models.Division, error) {
	return s.divisionRepository.FindByID(ctx, divisionID)
}

// Helper method to get season by ID
func (s *Service) getSeasonByID(ctx context.Context, seasonID uint) (*models.Season, error) {
	return s.seasonRepository.FindByID(ctx, seasonID)
}

// getStAnnsTeamCount gets the count of teams for St. Ann's club
func (s *Service) getStAnnsTeamCount(ctx context.Context) (int, error) {
	// Find St. Ann's club
	clubs, err := s.clubRepository.FindByNameLike(ctx, "St Ann")
	if err != nil {
		return 0, err
	}
	if len(clubs) == 0 {
		return 0, nil // No club found
	}
	stAnnsClub := &clubs[0]

	// Get all teams for St. Ann's club
	teams, err := s.teamRepository.FindByClub(ctx, stAnnsClub.ID)
	if err != nil {
		return 0, err
	}

	return len(teams), nil
}

// getStAnnsFixtureCount gets the count of remaining fixtures for St. Ann's club
func (s *Service) getStAnnsFixtureCount(ctx context.Context) (int, error) {
	// Find St. Ann's club
	clubs, err := s.clubRepository.FindByNameLike(ctx, "St Ann")
	if err != nil {
		return 0, err
	}
	if len(clubs) == 0 {
		return 0, nil // No club found
	}
	stAnnsClub := &clubs[0]

	// Get all teams for St. Ann's club
	teams, err := s.teamRepository.FindByClub(ctx, stAnnsClub.ID)
	if err != nil {
		return 0, err
	}

	if len(teams) == 0 {
		return 0, nil // No teams found
	}

	// Count remaining fixtures (today or later) for St. Ann's teams
	totalRemainingFixtures := 0
	now := time.Now()

	for _, team := range teams {
		teamFixtures, err := s.fixtureRepository.FindByTeam(ctx, team.ID)
		if err != nil {
			continue // Skip this team if there's an error
		}

		// Filter for remaining fixtures (scheduled or in progress, today or later)
		for _, fixture := range teamFixtures {
			if fixture.Status == models.Scheduled || fixture.Status == models.InProgress {
				if fixture.ScheduledDate.After(now) || fixture.ScheduledDate.Equal(now.Truncate(24*time.Hour)) {
					totalRemainingFixtures++
				}
			}
		}
	}

	return totalRemainingFixtures, nil
}

// GetTeams retrieves all teams for admin management
func (s *Service) GetTeams() (interface{}, error) {
	// TODO: Implement team retrieval from database
	return nil, nil
}

// GetStAnnsTeams retrieves teams for St. Ann's club with related data
func (s *Service) GetStAnnsTeams() (*models.Club, []TeamWithRelations, error) {
	ctx := context.Background()

	// Find St. Ann's club
	clubs, err := s.clubRepository.FindByNameLike(ctx, "St Ann")
	if err != nil {
		return nil, nil, err
	}
	if len(clubs) == 0 {
		return nil, nil, nil // No club found
	}
	stAnnsClub := &clubs[0]

	// Get all teams for St. Ann's club
	teams, err := s.teamRepository.FindByClub(ctx, stAnnsClub.ID)
	if err != nil {
		return stAnnsClub, nil, err
	}

	if len(teams) == 0 {
		return stAnnsClub, nil, nil // No teams found
	}

	// Build TeamsWithRelations by fetching related data
	var teamsWithRelations []TeamWithRelations
	for _, team := range teams {
		teamWithRelations := TeamWithRelations{
			Team: team,
		}

		// Get division
		if division, err := s.divisionRepository.FindByID(ctx, team.DivisionID); err == nil {
			teamWithRelations.Division = division
		}

		// Get season
		if season, err := s.seasonRepository.FindByID(ctx, team.SeasonID); err == nil {
			teamWithRelations.Season = season
		}

		// Get team captain
		if captain, err := s.teamRepository.FindTeamCaptain(ctx, team.ID, team.SeasonID); err == nil {
			// Get captain player details
			if playerDetails, err := s.playerRepository.FindByID(ctx, captain.PlayerID); err == nil {
				teamWithRelations.Captain = playerDetails
			}
		}

		// Get player count
		if playerCount, err := s.teamRepository.CountPlayers(ctx, team.ID, team.SeasonID); err == nil {
			teamWithRelations.PlayerCount = playerCount
		}

		teamsWithRelations = append(teamsWithRelations, teamWithRelations)
	}

	return stAnnsClub, teamsWithRelations, nil
}

// GetTeamDetail retrieves comprehensive details for a specific team
func (s *Service) GetTeamDetail(teamID uint) (*TeamDetail, error) {
	ctx := context.Background()

	// Get the base team
	team, err := s.teamRepository.FindByID(ctx, teamID)
	if err != nil {
		return nil, err
	}

	// Create the detail struct
	detail := &TeamDetail{
		Team: *team,
	}

	// Get club
	if club, err := s.clubRepository.FindByID(ctx, team.ClubID); err == nil {
		detail.Club = club
	}

	// Get division
	if division, err := s.divisionRepository.FindByID(ctx, team.DivisionID); err == nil {
		detail.Division = division
	}

	// Get season
	if season, err := s.seasonRepository.FindByID(ctx, team.SeasonID); err == nil {
		detail.Season = season
	}

	// Get captains
	if captains, err := s.teamRepository.FindCaptainsInTeam(ctx, teamID, team.SeasonID); err == nil {
		var captainsWithInfo []CaptainWithInfo
		for _, captain := range captains {
			if player, err := s.playerRepository.FindByID(ctx, captain.PlayerID); err == nil {
				captainsWithInfo = append(captainsWithInfo, CaptainWithInfo{
					Captain: captain,
					Player:  *player,
				})
			}
		}
		detail.Captains = captainsWithInfo
	}

	// Get players in team
	if playerTeams, err := s.teamRepository.FindPlayersInTeam(ctx, teamID, team.SeasonID); err == nil {
		var playersInTeam []PlayerInTeam
		for _, playerTeam := range playerTeams {
			if player, err := s.playerRepository.FindByID(ctx, playerTeam.PlayerID); err == nil {
				playersInTeam = append(playersInTeam, PlayerInTeam{
					Player:     *player,
					PlayerTeam: playerTeam,
				})
			}
		}
		detail.Players = playersInTeam
		detail.PlayerCount = len(playersInTeam)
	}

	return detail, nil
}

// GetAvailablePlayersForCaptain retrieves players who can be made captains for a team
// This includes players from the same club who are not already captains of this team
func (s *Service) GetAvailablePlayersForCaptain(teamID uint) ([]models.Player, error) {
	ctx := context.Background()

	// Get the team to find its club and season
	team, err := s.teamRepository.FindByID(ctx, teamID)
	if err != nil {
		return nil, err
	}

	// Get all players from the same club
	clubPlayers, err := s.playerRepository.FindByClub(ctx, team.ClubID)
	if err != nil {
		return nil, err
	}

	// Get existing captains for this team in this season
	existingCaptains, err := s.teamRepository.FindCaptainsInTeam(ctx, teamID, team.SeasonID)
	if err != nil {
		return nil, err
	}

	// Create a map of existing captain IDs for fast lookup
	captainMap := make(map[string]bool)
	for _, captain := range existingCaptains {
		captainMap[captain.PlayerID] = true
	}

	// Filter out players who are already captains
	var availablePlayers []models.Player
	for _, player := range clubPlayers {
		if !captainMap[player.ID] {
			availablePlayers = append(availablePlayers, player)
		}
	}

	return availablePlayers, nil
}

// AddTeamCaptain adds a player as a captain to a team
func (s *Service) AddTeamCaptain(teamID uint, playerID string, role models.CaptainRole) error {
	ctx := context.Background()

	// Get the team to find its season
	team, err := s.teamRepository.FindByID(ctx, teamID)
	if err != nil {
		return err
	}

	// Verify the player exists
	_, err = s.playerRepository.FindByID(ctx, playerID)
	if err != nil {
		return err
	}

	// Check if player is already a captain with this role for this team
	existingCaptains, err := s.teamRepository.FindCaptainsInTeam(ctx, teamID, team.SeasonID)
	if err != nil {
		return err
	}

	for _, captain := range existingCaptains {
		if captain.PlayerID == playerID && captain.Role == role {
			return fmt.Errorf("player is already a %s captain for this team", role)
		}
	}

	// Add the captain
	return s.teamRepository.AddCaptain(ctx, teamID, playerID, role, team.SeasonID)
}

// RemoveTeamCaptain removes a captain from a team
func (s *Service) RemoveTeamCaptain(teamID uint, playerID string) error {
	ctx := context.Background()

	// Get the team to find its season
	team, err := s.teamRepository.FindByID(ctx, teamID)
	if err != nil {
		return err
	}

	// Remove the captain
	return s.teamRepository.RemoveCaptain(ctx, teamID, playerID, team.SeasonID)
}

// GetEligiblePlayersForTeam retrieves players who can be added to a team
// This excludes players who are already on the team
func (s *Service) GetEligiblePlayersForTeam(teamID uint, query, statusFilter string) ([]models.Player, error) {
	ctx := context.Background()

	// Get the team to find its club and season
	team, err := s.teamRepository.FindByID(ctx, teamID)
	if err != nil {
		return nil, err
	}

	// Get all players from the same club (or all players if needed)
	var allPlayers []models.Player
	if query != "" {
		// If there's a search query, search across all players but prioritize same club
		searchResults, err := s.playerRepository.SearchPlayers(ctx, query)
		if err != nil {
			return nil, err
		}
		allPlayers = searchResults
	} else {
		// Default to players from the same club
		clubPlayers, err := s.playerRepository.FindByClub(ctx, team.ClubID)
		if err != nil {
			return nil, err
		}
		allPlayers = clubPlayers
	}

	// Get current team members to exclude them
	currentPlayerTeams, err := s.teamRepository.FindPlayersInTeam(ctx, teamID, team.SeasonID)
	if err != nil {
		return nil, err
	}

	// Create a map of current team member IDs for fast lookup
	currentMemberMap := make(map[string]bool)
	for _, playerTeam := range currentPlayerTeams {
		currentMemberMap[playerTeam.PlayerID] = true
	}

	// Filter out players who are already on the team
	var eligiblePlayers []models.Player
	for _, player := range allPlayers {
		// Skip players who are already on the team
		if currentMemberMap[player.ID] {
			continue
		}

		eligiblePlayers = append(eligiblePlayers, player)
	}

	return eligiblePlayers, nil
}

// AddPlayersToTeam adds multiple players to a team at once
func (s *Service) AddPlayersToTeam(teamID uint, playerIDs []string) error {
	ctx := context.Background()

	// Get the team to find its season
	team, err := s.teamRepository.FindByID(ctx, teamID)
	if err != nil {
		return err
	}

	// Validate all player IDs exist
	for _, playerID := range playerIDs {
		_, err := s.playerRepository.FindByID(ctx, playerID)
		if err != nil {
			return fmt.Errorf("player %s not found: %v", playerID, err)
		}
	}

	// Check if any players are already on the team
	currentPlayerTeams, err := s.teamRepository.FindPlayersInTeam(ctx, teamID, team.SeasonID)
	if err != nil {
		return err
	}

	currentMemberMap := make(map[string]bool)
	for _, playerTeam := range currentPlayerTeams {
		currentMemberMap[playerTeam.PlayerID] = true
	}

	// Add each player to the team (skip if already a member)
	var addedCount int
	for _, playerID := range playerIDs {
		if currentMemberMap[playerID] {
			continue // Skip players already on team
		}

		err := s.teamRepository.AddPlayer(ctx, teamID, playerID, team.SeasonID)
		if err != nil {
			return fmt.Errorf("failed to add player %s: %v", playerID, err)
		}
		addedCount++
	}

	if addedCount == 0 {
		return fmt.Errorf("no new players were added (all selected players are already on the team)")
	}

	return nil
}

// GetUpcomingFixturesForTeam retrieves upcoming fixtures for a specific team
// Limited to a specific count and includes today's fixtures
func (s *Service) GetUpcomingFixturesForTeam(teamID uint, limit int) ([]FixtureWithRelations, error) {
	ctx := context.Background()

	// Get all fixtures for the team
	teamFixtures, err := s.fixtureRepository.FindByTeam(ctx, teamID)
	if err != nil {
		return nil, err
	}

	// Filter for upcoming fixtures (today or later) that are scheduled or in progress
	var upcomingFixtures []models.Fixture
	now := time.Now()
	today := now.Truncate(24 * time.Hour)

	for _, fixture := range teamFixtures {
		// Include fixtures that are today or in the future, and are scheduled or in progress
		if (fixture.ScheduledDate.After(now) || fixture.ScheduledDate.After(today)) &&
			(fixture.Status == models.Scheduled || fixture.Status == models.InProgress) {
			upcomingFixtures = append(upcomingFixtures, fixture)
		}
	}

	// Sort by scheduled date (earliest first)
	// Note: Go's slice sorting would be better, but we'll keep it simple for now
	// since the repository should already return them in order

	// Limit the results
	if limit > 0 && len(upcomingFixtures) > limit {
		upcomingFixtures = upcomingFixtures[:limit]
	}

	// Build FixtureWithRelations by fetching related data
	var fixturesWithRelations []FixtureWithRelations
	for _, fixture := range upcomingFixtures {
		fixtureWithRelations := FixtureWithRelations{
			Fixture: fixture,
		}

		// Declare team variables for later use
		var homeTeam, awayTeam *models.Team

		// Get home team
		if homeTeamResult, err := s.teamRepository.FindByID(ctx, fixture.HomeTeamID); err == nil {
			homeTeam = homeTeamResult
			fixtureWithRelations.HomeTeam = homeTeam
		}

		// Get away team
		if awayTeamResult, err := s.teamRepository.FindByID(ctx, fixture.AwayTeamID); err == nil {
			awayTeam = awayTeamResult
			fixtureWithRelations.AwayTeam = awayTeam
		}

		// Get week
		if week, err := s.weekRepository.FindByID(ctx, fixture.WeekID); err == nil {
			fixtureWithRelations.Week = week
		}

		// Get division
		if division, err := s.getDivisionByID(ctx, fixture.DivisionID); err == nil {
			fixtureWithRelations.Division = division
		}

		// Get season
		if season, err := s.getSeasonByID(ctx, fixture.SeasonID); err == nil {
			fixtureWithRelations.Season = season
		}

		// Determine if the requesting team is home or away (only if teams were loaded successfully)
		if homeTeam != nil && homeTeam.ID == teamID {
			fixtureWithRelations.IsStAnnsHome = true
		}
		if awayTeam != nil && awayTeam.ID == teamID {
			fixtureWithRelations.IsStAnnsAway = true
		}

		// Determine if it's a derby match (both teams are from the same club)
		if homeTeam != nil && awayTeam != nil && homeTeam.ClubID == awayTeam.ClubID {
			fixtureWithRelations.IsDerby = true

			// For derby matches, set the default team context to the requesting team
			if homeTeam.ID == teamID {
				fixtureWithRelations.DefaultTeamContext = homeTeam
			} else if awayTeam.ID == teamID {
				fixtureWithRelations.DefaultTeamContext = awayTeam
			}
		}

		fixturesWithRelations = append(fixturesWithRelations, fixtureWithRelations)
	}

	return fixturesWithRelations, nil
}

// GetAvailablePlayersForFixture retrieves players available for selection in a fixture
// Returns team players first, then other St Ann players (deduplicated)
func (s *Service) GetAvailablePlayersForFixture(fixtureID uint) ([]models.Player, []models.Player, error) {
	ctx := context.Background()

	// Get the fixture to determine the home team
	fixture, err := s.fixtureRepository.FindByID(ctx, fixtureID)
	if err != nil {
		return nil, nil, err
	}

	// Find the St Ann's club ID dynamically
	stAnnsClubs, err := s.clubRepository.FindByNameLike(ctx, "St Ann")
	if err != nil {
		return nil, nil, err
	}
	if len(stAnnsClubs) == 0 {
		return nil, nil, fmt.Errorf("St Ann's club not found")
	}
	stAnnsClubID := stAnnsClubs[0].ID

	// Find the St Ann's team
	var stAnnsTeam *models.Team

	// Get home team
	homeTeam, err := s.teamRepository.FindByID(ctx, fixture.HomeTeamID)
	if err != nil {
		return nil, nil, err
	}

	// Get away team
	awayTeam, err := s.teamRepository.FindByID(ctx, fixture.AwayTeamID)
	if err != nil {
		return nil, nil, err
	}

	// Find which team is St Ann's
	if homeTeam.ClubID == stAnnsClubID {
		stAnnsTeam = homeTeam
	} else if awayTeam.ClubID == stAnnsClubID {
		stAnnsTeam = awayTeam
	} else {
		return nil, nil, fmt.Errorf("no St Ann's team found in this fixture")
	}

	teamPlayerTeams, err := s.teamRepository.FindPlayersInTeam(ctx, stAnnsTeam.ID, stAnnsTeam.SeasonID)
	if err != nil {
		return nil, nil, err
	}

	var teamPlayers []models.Player
	teamPlayerMap := make(map[string]bool) // Track team player IDs for deduplication
	for _, pt := range teamPlayerTeams {
		if player, err := s.playerRepository.FindByID(ctx, pt.PlayerID); err == nil {
			teamPlayers = append(teamPlayers, *player)
			teamPlayerMap[player.ID] = true
		}
	}

	// Get all St Ann players
	clubs, err := s.clubRepository.FindByNameLike(ctx, "St Ann")
	if err != nil {
		return teamPlayers, nil, err
	}
	if len(clubs) == 0 {
		return teamPlayers, nil, nil
	}

	allStAnnPlayers, err := s.playerRepository.FindByClub(ctx, clubs[0].ID)
	if err != nil {
		return teamPlayers, nil, err
	}

	// Deduplicate: remove team players from the "other St Ann players" list
	var otherStAnnPlayers []models.Player
	for _, player := range allStAnnPlayers {
		if !teamPlayerMap[player.ID] {
			otherStAnnPlayers = append(otherStAnnPlayers, player)
		}
	}

	return teamPlayers, otherStAnnPlayers, nil
}

// GetAvailablePlayersForFixtureWithTeamContext gets available players for a fixture with team context
// For derby matches, managingTeamID specifies which team to prioritize (0 means auto-detect)
// Returns team players first, then other St Ann players (deduplicated)
func (s *Service) GetAvailablePlayersForFixtureWithTeamContext(fixtureID uint, managingTeamID uint) ([]models.Player, []models.Player, error) {
	ctx := context.Background()

	// Get the fixture to determine the teams
	fixture, err := s.fixtureRepository.FindByID(ctx, fixtureID)
	if err != nil {
		return nil, nil, err
	}

	// Find the St Ann's club ID dynamically
	stAnnsClubs, err := s.clubRepository.FindByNameLike(ctx, "St Ann")
	if err != nil {
		return nil, nil, err
	}
	if len(stAnnsClubs) == 0 {
		return nil, nil, fmt.Errorf("St Ann's club not found")
	}
	stAnnsClubID := stAnnsClubs[0].ID

	// Get home and away teams
	homeTeam, err := s.teamRepository.FindByID(ctx, fixture.HomeTeamID)
	if err != nil {
		return nil, nil, err
	}

	awayTeam, err := s.teamRepository.FindByID(ctx, fixture.AwayTeamID)
	if err != nil {
		return nil, nil, err
	}

	// Determine if this is a derby match
	isHomeStAnns := homeTeam.ClubID == stAnnsClubID
	isAwayStAnns := awayTeam.ClubID == stAnnsClubID
	isDerby := isHomeStAnns && isAwayStAnns

	var stAnnsTeam *models.Team

	if isDerby {
		// For derby matches, use the specified managing team
		if managingTeamID > 0 {
			if homeTeam.ID == managingTeamID {
				stAnnsTeam = homeTeam
			} else if awayTeam.ID == managingTeamID {
				stAnnsTeam = awayTeam
			} else {
				// Default to home team if managing team not found
				stAnnsTeam = homeTeam
			}
		} else {
			// Default to home team for derby matches
			stAnnsTeam = homeTeam
		}
	} else {
		// Regular match - find which team is St Ann's
		if isHomeStAnns {
			stAnnsTeam = homeTeam
		} else if isAwayStAnns {
			stAnnsTeam = awayTeam
		} else {
			return nil, nil, fmt.Errorf("no St Ann's team found in this fixture")
		}
	}

	teamPlayerTeams, err := s.teamRepository.FindPlayersInTeam(ctx, stAnnsTeam.ID, stAnnsTeam.SeasonID)
	if err != nil {
		return nil, nil, err
	}

	var teamPlayers []models.Player
	teamPlayerMap := make(map[string]bool) // Track team player IDs for deduplication
	for _, pt := range teamPlayerTeams {
		if player, err := s.playerRepository.FindByID(ctx, pt.PlayerID); err == nil {
			teamPlayers = append(teamPlayers, *player)
			teamPlayerMap[player.ID] = true
		}
	}

	// Get all St Ann players
	clubs, err := s.clubRepository.FindByNameLike(ctx, "St Ann")
	if err != nil {
		return teamPlayers, nil, err
	}
	if len(clubs) == 0 {
		return teamPlayers, nil, nil
	}

	allStAnnPlayers, err := s.playerRepository.FindByClub(ctx, clubs[0].ID)
	if err != nil {
		return teamPlayers, nil, err
	}

	// Deduplicate: remove team players from the "other St Ann players" list
	var otherStAnnPlayers []models.Player
	for _, player := range allStAnnPlayers {
		if !teamPlayerMap[player.ID] {
			otherStAnnPlayers = append(otherStAnnPlayers, player)
		}
	}

	return teamPlayers, otherStAnnPlayers, nil
}

// AddPlayerToFixture adds a player to the fixture selection
func (s *Service) AddPlayerToFixture(fixtureID uint, playerID string, isHome bool) error {
	ctx := context.Background()

	// Check if player is already selected for this fixture
	selectedPlayers, err := s.fixtureRepository.FindSelectedPlayers(ctx, fixtureID)
	if err != nil {
		return err
	}

	for _, sp := range selectedPlayers {
		if sp.PlayerID == playerID {
			return fmt.Errorf("player is already selected for this fixture")
		}
	}

	// Calculate next position
	position := len(selectedPlayers) + 1

	fixturePlayer := &models.FixturePlayer{
		FixtureID: fixtureID,
		PlayerID:  playerID,
		IsHome:    isHome,
		Position:  position,
	}

	return s.fixtureRepository.AddSelectedPlayer(ctx, fixturePlayer)
}

// RemovePlayerFromFixture removes a player from the fixture selection
func (s *Service) RemovePlayerFromFixture(fixtureID uint, playerID string) error {
	ctx := context.Background()
	return s.fixtureRepository.RemoveSelectedPlayer(ctx, fixtureID, playerID)
}

// UpdatePlayerPositionInFixture updates the position/order of a selected player
func (s *Service) UpdatePlayerPositionInFixture(fixtureID uint, playerID string, position int) error {
	ctx := context.Background()
	return s.fixtureRepository.UpdateSelectedPlayerPosition(ctx, fixtureID, playerID, position)
}

// ClearFixturePlayerSelection removes all selected players from a fixture
func (s *Service) ClearFixturePlayerSelection(fixtureID uint) error {
	ctx := context.Background()
	return s.fixtureRepository.ClearSelectedPlayers(ctx, fixtureID)
}

// AddPlayerToFixtureWithTeam adds a player to the fixture selection for a specific managing team (for derby matches)
func (s *Service) AddPlayerToFixtureWithTeam(fixtureID uint, playerID string, isHome bool, managingTeamID uint) error {
	ctx := context.Background()

	// Check if player is already selected for this fixture by this team
	selectedPlayers, err := s.fixtureRepository.FindSelectedPlayersByTeam(ctx, fixtureID, managingTeamID)
	if err != nil {
		return err
	}

	for _, sp := range selectedPlayers {
		if sp.PlayerID == playerID {
			return fmt.Errorf("player is already selected for this fixture by this team")
		}
	}

	// Calculate next position for this team
	position := len(selectedPlayers) + 1

	fixturePlayer := &models.FixturePlayer{
		FixtureID:      fixtureID,
		PlayerID:       playerID,
		IsHome:         isHome,
		Position:       position,
		ManagingTeamID: &managingTeamID,
	}

	return s.fixtureRepository.AddSelectedPlayer(ctx, fixturePlayer)
}

// RemovePlayerFromFixtureByTeam removes a player from the fixture selection for a specific team
func (s *Service) RemovePlayerFromFixtureByTeam(fixtureID uint, playerID string, managingTeamID uint) error {
	ctx := context.Background()
	return s.fixtureRepository.RemoveSelectedPlayerByTeam(ctx, fixtureID, managingTeamID, playerID)
}

// ClearFixturePlayerSelectionByTeam removes all selected players from a fixture for a specific team
func (s *Service) ClearFixturePlayerSelectionByTeam(fixtureID uint, managingTeamID uint) error {
	ctx := context.Background()
	return s.fixtureRepository.ClearSelectedPlayersByTeam(ctx, fixtureID, managingTeamID)
}

// CreateMatchup creates a new matchup for a fixture
func (s *Service) CreateMatchup(fixtureID uint, matchupType models.MatchupType) (*models.Matchup, error) {
	ctx := context.Background()

	// Determine the managing team ID
	managingTeamID, err := s.determineManagingTeamID(ctx, fixtureID)
	if err != nil {
		return nil, err
	}

	return s.CreateMatchupWithTeam(fixtureID, matchupType, managingTeamID)
}

// CreateMatchupWithTeam creates a new matchup with explicit managing team ID (for derby matches)
func (s *Service) CreateMatchupWithTeam(fixtureID uint, matchupType models.MatchupType, managingTeamID uint) (*models.Matchup, error) {
	ctx := context.Background()

	// Check if matchup already exists for this fixture, type, and managing team
	existingMatchup, err := s.matchupRepository.FindByFixtureTypeAndTeam(ctx, fixtureID, matchupType, managingTeamID)
	if err == nil && existingMatchup != nil {
		return existingMatchup, fmt.Errorf("matchup of type %s already exists for this fixture and team", matchupType)
	}

	// Create new matchup
	matchup := &models.Matchup{
		FixtureID:      fixtureID,
		Type:           matchupType,
		Status:         models.Pending,
		HomeScore:      0,
		AwayScore:      0,
		Notes:          "",
		ManagingTeamID: &managingTeamID,
	}

	err = s.matchupRepository.Create(ctx, matchup)
	if err != nil {
		return nil, err
	}

	return matchup, nil
}

// IsStAnnsHomeInFixture determines whether St Ann's is the home team in a fixture
// Uses the exact same logic as buildFixturesWithRelations to ensure consistency
func (s *Service) IsStAnnsHomeInFixture(fixtureID uint) bool {
	ctx := context.Background()

	// Get the fixture
	fixture, err := s.fixtureRepository.FindByID(ctx, fixtureID)
	if err != nil {
		return false // Default to away if we can't determine
	}

	// Find St. Ann's club (using same logic as buildFixturesWithRelations)
	clubs, err := s.clubRepository.FindByNameLike(ctx, "St Ann")
	if err != nil || len(clubs) == 0 {
		return false // Default to away if we can't find St Ann's
	}
	stAnnsClub := &clubs[0]

	// Get home team (using same logic as buildFixturesWithRelations)
	homeTeam, err := s.teamRepository.FindByID(ctx, fixture.HomeTeamID)
	if err != nil {
		return false // Default to away if we can't get home team
	}

	// Use exact same logic as buildFixturesWithRelations:
	// "Determine if St. Ann's is home or away (only if teams were loaded successfully)"
	if homeTeam != nil && homeTeam.ClubID == stAnnsClub.ID {
		return true // IsStAnnsHome = true
	}

	return false // IsStAnnsHome = false (either away or not St Ann's fixture)
}

// determineManagingTeamID determines which team should manage a matchup for a given fixture
func (s *Service) determineManagingTeamID(ctx context.Context, fixtureID uint) (uint, error) {
	// Get the fixture to determine the teams
	fixture, err := s.fixtureRepository.FindByID(ctx, fixtureID)
	if err != nil {
		return 0, err
	}

	// Find the St Ann's club ID
	stAnnsClubs, err := s.clubRepository.FindByNameLike(ctx, "St Ann")
	if err != nil {
		return 0, err
	}
	if len(stAnnsClubs) == 0 {
		return 0, fmt.Errorf("St Ann's club not found")
	}
	stAnnsClubID := stAnnsClubs[0].ID

	// Get home and away teams
	homeTeam, err := s.teamRepository.FindByID(ctx, fixture.HomeTeamID)
	if err != nil {
		return 0, err
	}

	awayTeam, err := s.teamRepository.FindByID(ctx, fixture.AwayTeamID)
	if err != nil {
		return 0, err
	}

	// Check which team is St Ann's - prefer home team if both are St Ann's
	if homeTeam.ClubID == stAnnsClubID {
		return homeTeam.ID, nil
	} else if awayTeam.ClubID == stAnnsClubID {
		return awayTeam.ID, nil
	} else {
		return 0, fmt.Errorf("no St Ann's team found in this fixture")
	}
}

// GetOrCreateMatchup gets an existing matchup or creates a new one (legacy version for regular matches)
func (s *Service) GetOrCreateMatchup(fixtureID uint, matchupType models.MatchupType) (*models.Matchup, error) {
	ctx := context.Background()

	// Determine the managing team ID
	managingTeamID, err := s.determineManagingTeamID(ctx, fixtureID)
	if err != nil {
		return nil, err
	}

	return s.GetOrCreateMatchupWithTeam(fixtureID, matchupType, managingTeamID)
}

// GetOrCreateMatchupWithTeam gets an existing matchup or creates a new one for a specific managing team (for derby matches)
func (s *Service) GetOrCreateMatchupWithTeam(fixtureID uint, matchupType models.MatchupType, managingTeamID uint) (*models.Matchup, error) {
	ctx := context.Background()

	// Try to find existing matchup for this team
	matchup, err := s.matchupRepository.FindByFixtureTypeAndTeam(ctx, fixtureID, matchupType, managingTeamID)
	if err == nil && matchup != nil {
		return matchup, nil
	}

	// Create new matchup if it doesn't exist
	return s.CreateMatchupWithTeam(fixtureID, matchupType, managingTeamID)
}

// UpdateMatchupPlayers updates the players assigned to a matchup
func (s *Service) UpdateMatchupPlayers(matchupID uint, homePlayer1ID, homePlayer2ID, awayPlayer1ID, awayPlayer2ID string) error {
	ctx := context.Background()

	// Clear existing players
	err := s.matchupRepository.ClearPlayers(ctx, matchupID)
	if err != nil {
		return err
	}

	// Add home players
	if homePlayer1ID != "" {
		err = s.matchupRepository.AddPlayer(ctx, matchupID, homePlayer1ID, true)
		if err != nil {
			return err
		}
	}
	if homePlayer2ID != "" {
		err = s.matchupRepository.AddPlayer(ctx, matchupID, homePlayer2ID, true)
		if err != nil {
			return err
		}
	}

	// Add away players
	if awayPlayer1ID != "" {
		err = s.matchupRepository.AddPlayer(ctx, matchupID, awayPlayer1ID, false)
		if err != nil {
			return err
		}
	}
	if awayPlayer2ID != "" {
		err = s.matchupRepository.AddPlayer(ctx, matchupID, awayPlayer2ID, false)
		if err != nil {
			return err
		}
	}

	// Update status to Playing if all 4 players are assigned
	if homePlayer1ID != "" && homePlayer2ID != "" && awayPlayer1ID != "" && awayPlayer2ID != "" {
		err = s.matchupRepository.UpdateStatus(ctx, matchupID, models.Playing)
		if err != nil {
			return err
		}
	}

	return nil
}

// UpdateStAnnsMatchupPlayers updates St Ann's players for a matchup, determining if they're home or away
func (s *Service) UpdateStAnnsMatchupPlayers(matchupID uint, fixtureID uint, stAnnsPlayer1ID, stAnnsPlayer2ID string) error {
	ctx := context.Background()

	// Get the fixture to determine if St Ann's is home or away
	fixture, err := s.fixtureRepository.FindByID(ctx, fixtureID)
	if err != nil {
		return err
	}

	// Find the St Ann's club ID
	stAnnsClubs, err := s.clubRepository.FindByNameLike(ctx, "St Ann")
	if err != nil {
		return err
	}
	if len(stAnnsClubs) == 0 {
		return fmt.Errorf("St Ann's club not found")
	}
	stAnnsClubID := stAnnsClubs[0].ID

	// Get home and away teams
	homeTeam, err := s.teamRepository.FindByID(ctx, fixture.HomeTeamID)
	if err != nil {
		return err
	}

	awayTeam, err := s.teamRepository.FindByID(ctx, fixture.AwayTeamID)
	if err != nil {
		return err
	}

	// Determine if St Ann's is home or away
	var isStAnnsHome bool
	if homeTeam.ClubID == stAnnsClubID {
		isStAnnsHome = true
	} else if awayTeam.ClubID == stAnnsClubID {
		isStAnnsHome = false
	} else {
		return fmt.Errorf("no St Ann's team found in this fixture")
	}

	// Clear existing players
	err = s.matchupRepository.ClearPlayers(ctx, matchupID)
	if err != nil {
		return err
	}

	// Add St Ann's players with correct home/away designation
	if stAnnsPlayer1ID != "" {
		err = s.matchupRepository.AddPlayer(ctx, matchupID, stAnnsPlayer1ID, isStAnnsHome)
		if err != nil {
			return err
		}
	}
	if stAnnsPlayer2ID != "" {
		err = s.matchupRepository.AddPlayer(ctx, matchupID, stAnnsPlayer2ID, isStAnnsHome)
		if err != nil {
			return err
		}
	}

	// Update status to Playing if both St Ann's players are assigned
	// (In a real-world scenario, you'd need opponent players too, but for St Ann's tool this is sufficient)
	if stAnnsPlayer1ID != "" && stAnnsPlayer2ID != "" {
		err = s.matchupRepository.UpdateStatus(ctx, matchupID, models.Playing)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetFixtureWithMatchups gets fixture details including matchups and their players
func (s *Service) GetFixtureWithMatchups(fixtureID uint) (*FixtureDetailWithMatchups, error) {
	ctx := context.Background()

	// Get basic fixture detail
	fixtureDetail, err := s.GetFixtureDetail(fixtureID)
	if err != nil {
		return nil, err
	}

	// Get matchups for this fixture
	matchups, err := s.matchupRepository.FindByFixture(ctx, fixtureID)
	if err != nil {
		return nil, err
	}

	// Get players for each matchup
	var matchupsWithPlayers []MatchupWithPlayers
	for _, matchup := range matchups {
		matchupPlayers, err := s.matchupRepository.FindPlayersInMatchup(ctx, matchup.ID)
		if err != nil {
			return nil, err
		}

		var playersWithInfo []MatchupPlayerWithInfo
		for _, mp := range matchupPlayers {
			if player, err := s.playerRepository.FindByID(ctx, mp.PlayerID); err == nil {
				playersWithInfo = append(playersWithInfo, MatchupPlayerWithInfo{
					MatchupPlayer: mp,
					Player:        *player,
				})
			}
		}

		matchupsWithPlayers = append(matchupsWithPlayers, MatchupWithPlayers{
			Matchup: matchup,
			Players: playersWithInfo,
		})
	}

	return &FixtureDetailWithMatchups{
		FixtureDetail:       *fixtureDetail,
		MatchupsWithPlayers: matchupsWithPlayers,
	}, nil
}

// GetAvailablePlayersForMatchup gets available players for a specific matchup
// Returns selected players if any, otherwise falls back to all St Ann's team players
func (s *Service) GetAvailablePlayersForMatchup(fixtureID uint) ([]models.Player, error) {
	ctx := context.Background()

	// First try to get selected players from the fixture
	selectedPlayers, err := s.fixtureRepository.FindSelectedPlayers(ctx, fixtureID)
	if err != nil {
		return nil, err
	}

	if len(selectedPlayers) > 0 {
		// Use selected players from fixture
		var players []models.Player
		for _, sp := range selectedPlayers {
			if player, err := s.playerRepository.FindByID(ctx, sp.PlayerID); err == nil {
				players = append(players, *player)
			}
		}
		return players, nil
	}

	// Fallback to St Ann's team players if no players selected
	teamPlayers, allStAnnPlayers, err := s.GetAvailablePlayersForFixture(fixtureID)
	if err != nil {
		return nil, err
	}

	// Prefer team players, but if none exist, use all St Ann's players
	if len(teamPlayers) > 0 {
		return teamPlayers, nil
	}

	// Combine team players and all St Ann's players as final fallback
	allPlayers := append(teamPlayers, allStAnnPlayers...)
	return allPlayers, nil
}

// GetAvailablePlayersForFixtureWithAvailability returns players with their availability status for a fixture
func (s *Service) GetAvailablePlayersForFixtureWithAvailability(fixtureID uint) ([]PlayerWithAvailability, []PlayerWithAvailability, error) {
	ctx := context.Background()

	// Get the basic player lists first
	teamPlayers, allStAnnPlayers, err := s.GetAvailablePlayersForFixture(fixtureID)
	if err != nil {
		return nil, nil, err
	}

	// Get the fixture to get its date
	fixture, err := s.fixtureRepository.FindByID(ctx, fixtureID)
	if err != nil {
		return nil, nil, err
	}

	// Convert players to PlayerWithAvailability
	teamPlayersWithAvail := make([]PlayerWithAvailability, 0, len(teamPlayers))
	for _, player := range teamPlayers {
		availability := s.determinePlayerAvailabilityForFixture(ctx, player.ID, fixtureID, fixture.ScheduledDate)
		teamPlayersWithAvail = append(teamPlayersWithAvail, PlayerWithAvailability{
			Player:             player,
			AvailabilityStatus: availability.Status,
			AvailabilityNotes:  availability.Notes,
		})
	}

	allStAnnPlayersWithAvail := make([]PlayerWithAvailability, 0, len(allStAnnPlayers))
	for _, player := range allStAnnPlayers {
		availability := s.determinePlayerAvailabilityForFixture(ctx, player.ID, fixtureID, fixture.ScheduledDate)
		allStAnnPlayersWithAvail = append(allStAnnPlayersWithAvail, PlayerWithAvailability{
			Player:             player,
			AvailabilityStatus: availability.Status,
			AvailabilityNotes:  availability.Notes,
		})
	}

	return teamPlayersWithAvail, allStAnnPlayersWithAvail, nil
}

// GetAvailablePlayersWithEligibilityForTeamSelection retrieves players with both availability and eligibility information
func (s *Service) GetAvailablePlayersWithEligibilityForTeamSelection(fixtureID uint, managingTeamID uint) ([]PlayerWithEligibility, []PlayerWithEligibility, error) {
	ctx := context.Background()

	// Get available players lists based on managing team (for derby matches)
	var teamPlayers, allStAnnPlayers []models.Player
	var err error

	if managingTeamID > 0 {
		teamPlayers, allStAnnPlayers, err = s.GetAvailablePlayersForFixtureWithTeamContext(fixtureID, managingTeamID)
	} else {
		teamPlayers, allStAnnPlayers, err = s.GetAvailablePlayersForFixture(fixtureID)
	}

	if err != nil {
		return nil, nil, err
	}

	// Get fixture for date context
	fixture, err := s.fixtureRepository.FindByID(ctx, fixtureID)
	if err != nil {
		return nil, nil, err
	}

	// Determine which team we're selecting for
	var teamID uint
	if managingTeamID > 0 {
		teamID = managingTeamID
	} else {
		// For non-derby matches, determine the St Ann's team
		stAnnsClubs, err := s.clubRepository.FindByNameLike(ctx, "St Ann")
		if err != nil || len(stAnnsClubs) == 0 {
			return nil, nil, fmt.Errorf("St Ann's club not found")
		}
		stAnnsClubID := stAnnsClubs[0].ID

		// Check if home team is St Ann's
		homeTeam, err := s.teamRepository.FindByID(ctx, fixture.HomeTeamID)
		if err == nil && homeTeam.ClubID == stAnnsClubID {
			teamID = homeTeam.ID
		} else {
			// Check if away team is St Ann's
			awayTeam, err := s.teamRepository.FindByID(ctx, fixture.AwayTeamID)
			if err == nil && awayTeam.ClubID == stAnnsClubID {
				teamID = awayTeam.ID
			}
		}
	}

	// Convert team players to players with availability and eligibility
	var teamPlayersWithEligibility []PlayerWithEligibility
	for _, player := range teamPlayers {
		availability := s.determinePlayerAvailabilityForFixture(ctx, player.ID, fixtureID, fixture.ScheduledDate)

		// Get eligibility information
		var eligibility *PlayerEligibilityInfo
		if teamID > 0 {
			eligibility, err = s.teamEligibilityService.GetPlayerEligibilityForTeam(ctx, player.ID, teamID, fixtureID)
			if err != nil {
				// Log error but continue - default to allowing play
				eligibility = &PlayerEligibilityInfo{
					Player:  player,
					CanPlay: true,
				}
			}
		}

		teamPlayersWithEligibility = append(teamPlayersWithEligibility, PlayerWithEligibility{
			Player:             player,
			AvailabilityStatus: availability.Status,
			AvailabilityNotes:  availability.Notes,
			Eligibility:        eligibility,
		})
	}

	// Convert all St Ann players to players with availability and eligibility
	var allStAnnPlayersWithEligibility []PlayerWithEligibility
	for _, player := range allStAnnPlayers {
		availability := s.determinePlayerAvailabilityForFixture(ctx, player.ID, fixtureID, fixture.ScheduledDate)

		// Get eligibility information
		var eligibility *PlayerEligibilityInfo
		if teamID > 0 {
			eligibility, err = s.teamEligibilityService.GetPlayerEligibilityForTeam(ctx, player.ID, teamID, fixtureID)
			if err != nil {
				// Log error but continue - default to allowing play
				eligibility = &PlayerEligibilityInfo{
					Player:  player,
					CanPlay: true,
				}
			}
		}

		allStAnnPlayersWithEligibility = append(allStAnnPlayersWithEligibility, PlayerWithEligibility{
			Player:             player,
			AvailabilityStatus: availability.Status,
			AvailabilityNotes:  availability.Notes,
			Eligibility:        eligibility,
		})
	}

	return teamPlayersWithEligibility, allStAnnPlayersWithEligibility, nil
}

// PlayerAvailabilityInfo holds availability information for a player
type PlayerAvailabilityInfo struct {
	Status models.AvailabilityStatus
	Notes  string
}

// determinePlayerAvailabilityForFixture determines a player's availability for a specific fixture
// following the priority order: fixture-specific > date exception > general day-of-week > unknown
func (s *Service) determinePlayerAvailabilityForFixture(ctx context.Context, playerID string, fixtureID uint, fixtureDate time.Time) PlayerAvailabilityInfo {
	// 1. Check fixture-specific availability first (highest priority)
	if fixtureAvail, err := s.availabilityRepository.GetPlayerFixtureAvailability(ctx, playerID, fixtureID); err == nil && fixtureAvail != nil {
		return PlayerAvailabilityInfo{
			Status: fixtureAvail.Status,
			Notes:  fixtureAvail.Notes,
		}
	}

	// 2. Check for date-specific exceptions
	if dateAvail, err := s.availabilityRepository.GetPlayerAvailabilityByDate(ctx, playerID, fixtureDate); err == nil && dateAvail != nil {
		return PlayerAvailabilityInfo{
			Status: dateAvail.Status,
			Notes:  dateAvail.Reason,
		}
	}

	// 3. Check general day-of-week availability
	// First get the current season - we'll need to implement this
	// For now, we'll assume season ID 1 or get it from the fixture
	fixture, err := s.fixtureRepository.FindByID(ctx, fixtureID)
	if err != nil {
		return PlayerAvailabilityInfo{Status: models.Unknown}
	}

	dayOfWeek := fixtureDate.Weekday().String()
	if generalAvails, err := s.availabilityRepository.GetPlayerGeneralAvailability(ctx, playerID, fixture.SeasonID); err == nil {
		for _, avail := range generalAvails {
			if avail.DayOfWeek == dayOfWeek {
				return PlayerAvailabilityInfo{
					Status: avail.Status,
					Notes:  avail.Notes,
				}
			}
		}
	}

	// 4. Default to Unknown if nothing is specified
	return PlayerAvailabilityInfo{Status: models.Unknown}
}

// GetFantasyDoubles retrieves all fantasy doubles pairings
func (s *Service) GetFantasyDoubles() ([]models.FantasyMixedDoubles, error) {
	return s.fantasyRepository.FindAll(context.Background())
}

// GetActiveFantasyDoubles retrieves active fantasy doubles pairings
func (s *Service) GetActiveFantasyDoubles() ([]models.FantasyMixedDoubles, error) {
	return s.fantasyRepository.FindActive(context.Background())
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

// GetFantasyDoublesByID retrieves a fantasy doubles pairing by ID
func (s *Service) GetFantasyDoublesByID(id uint) (*models.FantasyMixedDoubles, error) {
	return s.fantasyRepository.FindByID(context.Background(), id)
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

// FantasyDoublesDetail contains detailed information about a fantasy doubles pairing
type FantasyDoublesDetail struct {
	Match      models.FantasyMixedDoubles `json:"match"`
	TeamAWoman models.ProTennisPlayer     `json:"team_a_woman"`
	TeamAMan   models.ProTennisPlayer     `json:"team_a_man"`
	TeamBWoman models.ProTennisPlayer     `json:"team_b_woman"`
	TeamBMan   models.ProTennisPlayer     `json:"team_b_man"`
}

// GetFixtureDetailWithTeamContext gets fixture details filtered for a specific managing team (for derby matches)
func (s *Service) GetFixtureDetailWithTeamContext(fixtureID uint, managingTeamID uint) (*FixtureDetail, error) {
	ctx := context.Background()

	// Get the base fixture
	fixture, err := s.fixtureRepository.FindByID(ctx, fixtureID)
	if err != nil {
		return nil, err
	}

	// Create the detail struct
	detail := &FixtureDetail{
		Fixture: *fixture,
	}

	// Get home team
	if homeTeam, err := s.teamRepository.FindByID(ctx, fixture.HomeTeamID); err == nil {
		detail.HomeTeam = homeTeam
	}

	// Get away team
	if awayTeam, err := s.teamRepository.FindByID(ctx, fixture.AwayTeamID); err == nil {
		detail.AwayTeam = awayTeam
	}

	// Get week
	if week, err := s.weekRepository.FindByID(ctx, fixture.WeekID); err == nil {
		detail.Week = week
	}

	// Get division
	if division, err := s.getDivisionByID(ctx, fixture.DivisionID); err == nil {
		detail.Division = division
	}

	// Get season
	if season, err := s.getSeasonByID(ctx, fixture.SeasonID); err == nil {
		detail.Season = season
	}

	// Get day captain if assigned
	if fixture.DayCaptainID != nil {
		if dayCaptain, err := s.playerRepository.FindByID(ctx, *fixture.DayCaptainID); err == nil {
			detail.DayCaptain = dayCaptain
		}
	}

	// Get matchups with players for the fixture, filtered by managing team
	if matchups, err := s.getMatchupsForTeam(ctx, fixtureID, managingTeamID); err == nil {
		var matchupsWithPlayers []MatchupWithPlayers
		for _, matchup := range matchups {
			// Get players for this matchup
			matchupPlayers, err := s.matchupRepository.FindPlayersInMatchup(ctx, matchup.ID)
			if err != nil {
				// If we can't get players, still include the matchup with empty players
				matchupsWithPlayers = append(matchupsWithPlayers, MatchupWithPlayers{
					Matchup: matchup,
					Players: []MatchupPlayerWithInfo{},
				})
				continue
			}

			var playersWithInfo []MatchupPlayerWithInfo
			for _, mp := range matchupPlayers {
				if player, err := s.playerRepository.FindByID(ctx, mp.PlayerID); err == nil {
					playersWithInfo = append(playersWithInfo, MatchupPlayerWithInfo{
						MatchupPlayer: mp,
						Player:        *player,
					})
				}
			}

			matchupsWithPlayers = append(matchupsWithPlayers, MatchupWithPlayers{
				Matchup: matchup,
				Players: playersWithInfo,
			})
		}

		// Sort matchups in the desired order: 1st Mixed, 2nd Mixed, Mens, Womens
		sort.Slice(matchupsWithPlayers, func(i, j int) bool {
			return getMatchupOrder(matchupsWithPlayers[i].Matchup.Type) < getMatchupOrder(matchupsWithPlayers[j].Matchup.Type)
		})

		detail.Matchups = matchupsWithPlayers

		// Check for duplicate players across matchups
		detail.DuplicateWarnings = s.detectDuplicatePlayersInMatchups(matchupsWithPlayers)
	}

	// Get selected players for the fixture, filtered by managing team
	if selectedPlayers, err := s.fixtureRepository.FindSelectedPlayersByTeam(ctx, fixtureID, managingTeamID); err == nil {
		var selectedPlayerInfos []SelectedPlayerInfo
		for _, sp := range selectedPlayers {
			if player, err := s.playerRepository.FindByID(ctx, sp.PlayerID); err == nil {
				// Get availability information for this player and fixture
				availability := s.determinePlayerAvailabilityForFixture(ctx, sp.PlayerID, fixtureID, fixture.ScheduledDate)

				selectedPlayerInfos = append(selectedPlayerInfos, SelectedPlayerInfo{
					FixturePlayer:      sp,
					Player:             *player,
					AvailabilityStatus: availability.Status,
					AvailabilityNotes:  availability.Notes,
				})
			}
		}
		detail.SelectedPlayers = selectedPlayerInfos
	}

	return detail, nil
}

// getMatchupsForTeam gets matchups for a specific team - used for derby matches
func (s *Service) getMatchupsForTeam(ctx context.Context, fixtureID uint, managingTeamID uint) ([]models.Matchup, error) {
	// Get all matchups for the fixture
	allMatchups, err := s.matchupRepository.FindByFixture(ctx, fixtureID)
	if err != nil {
		return nil, err
	}

	// Filter by managing team ID
	var teamMatchups []models.Matchup
	for _, matchup := range allMatchups {
		// Include matchups that belong to this managing team
		if matchup.ManagingTeamID != nil && *matchup.ManagingTeamID == managingTeamID {
			teamMatchups = append(teamMatchups, matchup)
		} else if matchup.ManagingTeamID == nil {
			// Legacy matchups without managing team ID - include them for backward compatibility
			teamMatchups = append(teamMatchups, matchup)
		}
	}

	return teamMatchups, nil
}

// AddPlayerToMatchup adds a single player to a matchup without replacing existing players
func (s *Service) AddPlayerToMatchup(matchupID uint, playerID string, fixtureID uint) error {
	ctx := context.Background()

	// Get the fixture to determine if St Ann's is home or away
	fixture, err := s.fixtureRepository.FindByID(ctx, fixtureID)
	if err != nil {
		return err
	}

	// Find the St Ann's club ID
	stAnnsClubs, err := s.clubRepository.FindByNameLike(ctx, "St Ann")
	if err != nil {
		return err
	}
	if len(stAnnsClubs) == 0 {
		return fmt.Errorf("St Ann's club not found")
	}
	stAnnsClubID := stAnnsClubs[0].ID

	// Get home and away teams
	homeTeam, err := s.teamRepository.FindByID(ctx, fixture.HomeTeamID)
	if err != nil {
		return err
	}

	awayTeam, err := s.teamRepository.FindByID(ctx, fixture.AwayTeamID)
	if err != nil {
		return err
	}

	// Determine if St Ann's is home or away
	var isStAnnsHome bool
	if homeTeam.ClubID == stAnnsClubID {
		isStAnnsHome = true
	} else if awayTeam.ClubID == stAnnsClubID {
		isStAnnsHome = false
	} else {
		return fmt.Errorf("no St Ann's team found in this fixture")
	}

	// Check if player is already in this matchup
	existingPlayers, err := s.matchupRepository.FindPlayersInMatchup(ctx, matchupID)
	if err != nil {
		return err
	}

	for _, existingPlayer := range existingPlayers {
		if existingPlayer.PlayerID == playerID {
			return fmt.Errorf("player is already assigned to this matchup")
		}
	}

	// Add the player to the matchup
	err = s.matchupRepository.AddPlayer(ctx, matchupID, playerID, isStAnnsHome)
	if err != nil {
		return err
	}

	return nil
}

// RemovePlayerFromMatchup removes a single player from a matchup
func (s *Service) RemovePlayerFromMatchup(matchupID uint, playerID string) error {
	ctx := context.Background()

	// Remove the player from the matchup
	if err := s.matchupRepository.RemovePlayer(ctx, matchupID, playerID); err != nil {
		return err
	}

	// If no players remain in this matchup, delete the matchup to keep the UI clean
	remainingPlayers, err := s.matchupRepository.FindPlayersInMatchup(ctx, matchupID)
	if err != nil {
		return err
	}

	if len(remainingPlayers) == 0 {
		if err := s.matchupRepository.Delete(ctx, matchupID); err != nil {
			return err
		}
	}

	return nil
}

// CreatePlayer creates a new player
func (s *Service) CreatePlayer(player *models.Player) error {
	return s.playerRepository.Create(context.Background(), player)
}

// UpdateFixtureNotes updates the notes field of a fixture
func (s *Service) UpdateFixtureNotes(fixtureID uint, notes string) error {
	ctx := context.Background()

	// Get the current fixture
	fixture, err := s.fixtureRepository.FindByID(ctx, fixtureID)
	if err != nil {
		return err
	}

	// Update the notes
	fixture.Notes = notes

	// Save the updated fixture
	return s.fixtureRepository.Update(ctx, fixture)
}

// SetFixtureDayCaptain sets the day captain for a fixture
func (s *Service) SetFixtureDayCaptain(fixtureID uint, playerID string) error {
	ctx := context.Background()

	// Get the current fixture
	fixture, err := s.fixtureRepository.FindByID(ctx, fixtureID)
	if err != nil {
		return err
	}

	// Update the day captain
	fixture.DayCaptainID = &playerID

	// Save the updated fixture
	return s.fixtureRepository.Update(ctx, fixture)
}

// GetStAnnsNextWeekFixturesByDivision retrieves St Ann's fixtures for the next week organized by division
func (s *Service) GetStAnnsNextWeekFixturesByDivision() (map[string][]FixtureWithRelations, error) {
	ctx := context.Background()

	// Find St. Ann's club
	clubs, err := s.clubRepository.FindByNameLike(ctx, "St Ann")
	if err != nil {
		return nil, err
	}
	if len(clubs) == 0 {
		return make(map[string][]FixtureWithRelations), nil // No club found
	}
	stAnnsClub := &clubs[0]

	// Get all teams for St. Ann's club
	teams, err := s.teamRepository.FindByClub(ctx, stAnnsClub.ID)
	if err != nil {
		return nil, err
	}

	if len(teams) == 0 {
		return make(map[string][]FixtureWithRelations), nil // No teams found
	}

	// Get next week date range
	weekStart, weekEnd := s.getNextWeekDateRange()

	// Get all fixtures for all St. Ann's teams within the next week
	var allFixtures []models.Fixture
	fixtureMap := make(map[uint]models.Fixture) // Use map to deduplicate fixtures by ID

	for _, team := range teams {
		teamFixtures, err := s.fixtureRepository.FindByTeam(ctx, team.ID)
		if err != nil {
			continue // Skip this team if there's an error
		}
		// Filter fixtures for next week and add to map to automatically deduplicate
		for _, fixture := range teamFixtures {
			if fixture.ScheduledDate.After(weekStart) && fixture.ScheduledDate.Before(weekEnd) {
				if fixture.Status == models.Scheduled || fixture.Status == models.InProgress {
					fixtureMap[fixture.ID] = fixture
				}
			}
		}
	}

	// Convert map back to slice
	for _, fixture := range fixtureMap {
		allFixtures = append(allFixtures, fixture)
	}

	// Build FixtureWithRelations by fetching related data
	fixturesWithRelations := s.buildFixturesWithRelations(ctx, allFixtures, stAnnsClub)

	// Organize fixtures by division
	fixturesByDivision := make(map[string][]FixtureWithRelations)

	// Initialize division groups in the order we want (1, 2, 3, 4)
	fixturesByDivision["Division 1"] = []FixtureWithRelations{}
	fixturesByDivision["Division 2"] = []FixtureWithRelations{}
	fixturesByDivision["Division 3"] = []FixtureWithRelations{}
	fixturesByDivision["Division 4"] = []FixtureWithRelations{}

	for _, fixture := range fixturesWithRelations {
		if fixture.Division != nil {
			divisionName := fixture.Division.Name
			fixturesByDivision[divisionName] = append(fixturesByDivision[divisionName], fixture)
		} else {
			// If no division, put in a "Other" category
			if fixturesByDivision["Other"] == nil {
				fixturesByDivision["Other"] = []FixtureWithRelations{}
			}
			fixturesByDivision["Other"] = append(fixturesByDivision["Other"], fixture)
		}
	}

	return fixturesByDivision, nil
}

// UpdateFixtureSchedule updates a fixture's scheduled date and adds the previous date to history
func (s *Service) UpdateFixtureSchedule(fixtureID uint, newScheduledDate time.Time, rescheduleReason models.RescheduledReason, notes string) error {
	ctx := context.Background()

	// Get the current fixture to retrieve the current scheduled date
	currentFixture, err := s.fixtureRepository.FindByID(ctx, fixtureID)
	if err != nil {
		return fmt.Errorf("failed to get current fixture: %w", err)
	}

	// Check if fixture is completed
	if currentFixture.Status == models.Completed {
		return fmt.Errorf("cannot reschedule completed fixture")
	}

	// Prepare the previous dates array
	var previousDates []time.Time

	// Parse existing previous dates from JSON
	if len(currentFixture.PreviousDates) > 0 {
		// Note: PreviousDates is already a []time.Time slice from the model
		previousDates = currentFixture.PreviousDates
	}

	// Add the current scheduled date to previous dates if it's different from the new date
	if !currentFixture.ScheduledDate.Equal(newScheduledDate) {
		// Check if this date is already in the previous dates to avoid duplicates
		dateExists := false
		for _, prevDate := range previousDates {
			if prevDate.Equal(currentFixture.ScheduledDate) {
				dateExists = true
				break
			}
		}

		if !dateExists {
			previousDates = append(previousDates, currentFixture.ScheduledDate)
		}
	}

	// Update the fixture with new data
	updatedFixture := *currentFixture
	updatedFixture.ScheduledDate = newScheduledDate
	updatedFixture.PreviousDates = previousDates
	updatedFixture.RescheduledReason = &rescheduleReason
	if notes != "" {
		updatedFixture.Notes = notes
	}
	updatedFixture.UpdatedAt = time.Now()

	// Save the updated fixture
	err = s.fixtureRepository.Update(ctx, &updatedFixture)
	if err != nil {
		return fmt.Errorf("failed to update fixture: %w", err)
	}

	log.Printf("Fixture %d rescheduled from %v to %v for reason: %s",
		fixtureID, currentFixture.ScheduledDate, newScheduledDate, rescheduleReason)

	return nil
}

// Season management methods

// GetAllSeasons retrieves all seasons ordered by year descending
func (s *Service) GetAllSeasons() ([]models.Season, error) {
	ctx := context.Background()
	return s.seasonRepository.FindAll(ctx)
}

// GetActiveSeason retrieves the currently active season
func (s *Service) GetActiveSeason() (*models.Season, error) {
	ctx := context.Background()
	return s.seasonRepository.FindActive(ctx)
}

// CreateSeason creates a new season
func (s *Service) CreateSeason(season *models.Season) error {
	ctx := context.Background()
	return s.seasonRepository.Create(ctx, season)
}

// SetActiveSeason sets a season as active and deactivates all others
func (s *Service) SetActiveSeason(seasonID uint) error {
	ctx := context.Background()
	
	// Deactivate all seasons first
	seasons, err := s.seasonRepository.FindAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to get all seasons: %w", err)
	}
	
	for _, season := range seasons {
		if season.IsActive {
			season.IsActive = false
			if err := s.seasonRepository.Update(ctx, &season); err != nil {
				return fmt.Errorf("failed to deactivate season %d: %w", season.ID, err)
			}
		}
	}
	
	// Activate the specified season
	season, err := s.seasonRepository.FindByID(ctx, seasonID)
	if err != nil {
		return fmt.Errorf("failed to find season %d: %w", seasonID, err)
	}
	
	season.IsActive = true
	if err := s.seasonRepository.Update(ctx, season); err != nil {
		return fmt.Errorf("failed to activate season %d: %w", seasonID, err)
	}
	
	return nil
}

// CreateFixture creates a new fixture
func (s *Service) CreateFixture(fixture *models.Fixture) error {
	ctx := context.Background()
	return s.fixtureRepository.Create(ctx, fixture)
}

// GetAllWeeks retrieves all weeks
func (s *Service) GetAllWeeks() ([]models.Week, error) {
	ctx := context.Background()
	return s.weekRepository.FindAll(ctx)
}

// GetAllTeams retrieves all teams
func (s *Service) GetAllTeams() ([]models.Team, error) {
	ctx := context.Background()
	return s.teamRepository.FindAll(ctx)
}
