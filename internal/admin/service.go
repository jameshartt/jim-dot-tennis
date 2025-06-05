package admin

import (
	"context"
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
}

// NewService creates a new admin service
func NewService(db *database.DB) *Service {
	return &Service{
		db:                     db,
		loginAttemptRepository: repository.NewLoginAttemptRepository(db),
		playerRepository:       repository.NewPlayerRepository(db),
		clubRepository:         repository.NewClubRepository(db),
		fixtureRepository:      repository.NewFixtureRepository(db),
		teamRepository:         repository.NewTeamRepository(db),
		weekRepository:         repository.NewWeekRepository(db),
		divisionRepository:     repository.NewDivisionRepository(db),
		seasonRepository:       repository.NewSeasonRepository(db),
	}
}

// DashboardData represents the data needed for the admin dashboard
type DashboardData struct {
	Stats         Stats          `json:"stats"`
	LoginAttempts []LoginAttempt `json:"login_attempts"`
}

// Stats represents admin dashboard statistics
type Stats struct {
	PlayerCount  int `json:"player_count"`
	FixtureCount int `json:"fixture_count"`
	TeamCount    int `json:"team_count"`
}

// LoginAttempt represents a login attempt record
type LoginAttempt struct {
	Username  string    `json:"username"`
	IP        string    `json:"ip"`
	CreatedAt time.Time `json:"created_at"`
	Success   bool      `json:"success"`
}

// PlayerWithStatus represents a player with their activity status
type PlayerWithStatus struct {
	models.Player
	IsActive bool `json:"is_active"`
}

// FixtureWithRelations represents a fixture with its related team and week data
type FixtureWithRelations struct {
	models.Fixture
	HomeTeam *models.Team `json:"home_team,omitempty"`
	AwayTeam *models.Team `json:"away_team,omitempty"`
	Week     *models.Week `json:"week,omitempty"`
}

// FixtureDetail represents a fixture with comprehensive related data for detail view
type FixtureDetail struct {
	models.Fixture
	HomeTeam   *models.Team     `json:"home_team,omitempty"`
	AwayTeam   *models.Team     `json:"away_team,omitempty"`
	Week       *models.Week     `json:"week,omitempty"`
	Division   *models.Division `json:"division,omitempty"`
	Season     *models.Season   `json:"season,omitempty"`
	DayCaptain *models.Player   `json:"day_captain,omitempty"`
	Matchups   []models.Matchup `json:"matchups,omitempty"`
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
	Club        *models.Club     `json:"club,omitempty"`
	Division    *models.Division `json:"division,omitempty"`
	Season      *models.Season   `json:"season,omitempty"`
	Captains    []models.Captain `json:"captains,omitempty"`
	Players     []PlayerInTeam   `json:"players,omitempty"`
	PlayerCount int              `json:"player_count"`
}

// PlayerInTeam represents a player with their team membership details
type PlayerInTeam struct {
	models.Player
	PlayerTeam models.PlayerTeam `json:"player_team"`
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

	stats := Stats{
		PlayerCount:  playerCount,
		FixtureCount: fixtureCount,
		TeamCount:    teamCount,
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

// GetFilteredPlayers retrieves players based on search query and activity filter
func (s *Service) GetFilteredPlayers(query string, activeFilter string, seasonID uint) ([]PlayerWithStatus, error) {
	ctx := context.Background()

	// If no seasonID provided, use a default (this would need to be improved to get current season)
	if seasonID == 0 {
		seasonID = 1 // Default to season 1 for now
	}

	var players []models.Player
	var err error

	// Apply activity filter first, then search within those results
	switch activeFilter {
	case "active":
		if query != "" {
			// Get active players, then filter by search
			activePlayers, err := s.playerRepository.FindActivePlayersInSeason(ctx, seasonID)
			if err != nil {
				return nil, err
			}
			// Filter active players by search query
			players = filterPlayersByQuery(activePlayers, query)
		} else {
			players, err = s.playerRepository.FindActivePlayersInSeason(ctx, seasonID)
		}
	case "inactive":
		if query != "" {
			// Get inactive players, then filter by search
			inactivePlayers, err := s.playerRepository.FindInactivePlayersInSeason(ctx, seasonID)
			if err != nil {
				return nil, err
			}
			// Filter inactive players by search query
			players = filterPlayersByQuery(inactivePlayers, query)
		} else {
			players, err = s.playerRepository.FindInactivePlayersInSeason(ctx, seasonID)
		}
	default: // "all" or empty
		if query != "" {
			players, err = s.playerRepository.SearchPlayers(ctx, query)
		} else {
			players, err = s.playerRepository.FindAll(ctx)
		}
	}

	if err != nil {
		return nil, err
	}

	// Convert to PlayerWithStatus and determine activity status
	var playersWithStatus []PlayerWithStatus

	// If we filtered by active/inactive, we already know the status
	if activeFilter == "active" {
		for _, player := range players {
			playersWithStatus = append(playersWithStatus, PlayerWithStatus{
				Player:   player,
				IsActive: true,
			})
		}
	} else if activeFilter == "inactive" {
		for _, player := range players {
			playersWithStatus = append(playersWithStatus, PlayerWithStatus{
				Player:   player,
				IsActive: false,
			})
		}
	} else {
		// For "all" filter, we need to check each player's status
		activePlayerIDs, err := s.getActivePlayerIDsMap(ctx, seasonID)
		if err != nil {
			return nil, err
		}

		for _, player := range players {
			isActive := activePlayerIDs[player.ID]
			playersWithStatus = append(playersWithStatus, PlayerWithStatus{
				Player:   player,
				IsActive: isActive,
			})
		}
	}

	return playersWithStatus, nil
}

// getActivePlayerIDsMap returns a map of player IDs that are active in the given season
func (s *Service) getActivePlayerIDsMap(ctx context.Context, seasonID uint) (map[string]bool, error) {
	activePlayers, err := s.playerRepository.FindActivePlayersInSeason(ctx, seasonID)
	if err != nil {
		return nil, err
	}

	activeMap := make(map[string]bool)
	for _, player := range activePlayers {
		activeMap[player.ID] = true
	}

	return activeMap, nil
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
		// Check if query matches name, email, or phone
		fullName := strings.ToLower(player.FirstName + " " + player.LastName)
		email := strings.ToLower(player.Email)
		phone := strings.ToLower(player.Phone)

		if strings.Contains(fullName, queryLower) ||
			strings.Contains(email, queryLower) ||
			strings.Contains(phone, queryLower) {
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
	clubs, err := s.clubRepository.FindAll(context.Background())
	if err != nil {
		return nil, err
	}
	return clubs, nil
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
	for _, team := range teams {
		teamFixtures, err := s.fixtureRepository.FindByTeam(ctx, team.ID)
		if err != nil {
			continue // Skip this team if there's an error
		}
		allFixtures = append(allFixtures, teamFixtures...)
	}

	// Filter for upcoming fixtures (scheduled or in progress) and sort by date
	var upcomingFixtures []models.Fixture
	now := time.Now()
	for _, fixture := range allFixtures {
		if fixture.Status == models.Scheduled || fixture.Status == models.InProgress {
			if fixture.ScheduledDate.After(now) || fixture.ScheduledDate.Equal(now.Truncate(24*time.Hour)) {
				upcomingFixtures = append(upcomingFixtures, fixture)
			}
		}
	}

	// Sort fixtures by scheduled date (nearest first)
	for i := 0; i < len(upcomingFixtures); i++ {
		for j := i + 1; j < len(upcomingFixtures); j++ {
			if upcomingFixtures[i].ScheduledDate.After(upcomingFixtures[j].ScheduledDate) {
				upcomingFixtures[i], upcomingFixtures[j] = upcomingFixtures[j], upcomingFixtures[i]
			}
		}
	}

	// Build FixtureWithRelations by fetching related data
	var fixturesWithRelations []FixtureWithRelations
	for _, fixture := range upcomingFixtures {
		fixtureWithRelations := FixtureWithRelations{
			Fixture: fixture,
		}

		// Get home team
		if homeTeam, err := s.teamRepository.FindByID(ctx, fixture.HomeTeamID); err == nil {
			fixtureWithRelations.HomeTeam = homeTeam
		}

		// Get away team
		if awayTeam, err := s.teamRepository.FindByID(ctx, fixture.AwayTeamID); err == nil {
			fixtureWithRelations.AwayTeam = awayTeam
		}

		// Get week
		if week, err := s.weekRepository.FindByID(ctx, fixture.WeekID); err == nil {
			fixtureWithRelations.Week = week
		}

		fixturesWithRelations = append(fixturesWithRelations, fixtureWithRelations)
	}

	return stAnnsClub, fixturesWithRelations, nil
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

	// Get matchups with the fixture
	if fixtureWithMatchups, err := s.fixtureRepository.FindWithMatchups(ctx, fixtureID); err == nil {
		detail.Matchups = fixtureWithMatchups.Matchups
	}

	return detail, nil
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
		detail.Captains = captains
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
