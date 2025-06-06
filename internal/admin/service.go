package admin

import (
	"context"
	"fmt"
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
		matchupRepository:      repository.NewMatchupRepository(db),
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
	Player models.Player `json:"player"`
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
				selectedPlayerInfos = append(selectedPlayerInfos, SelectedPlayerInfo{
					FixturePlayer: sp,
					Player:        *player,
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

// CreateMatchup creates a new matchup for a fixture
func (s *Service) CreateMatchup(fixtureID uint, matchupType models.MatchupType) (*models.Matchup, error) {
	ctx := context.Background()

	// Check if matchup already exists for this fixture and type
	existingMatchup, err := s.matchupRepository.FindByFixtureAndType(ctx, fixtureID, matchupType)
	if err == nil && existingMatchup != nil {
		return existingMatchup, fmt.Errorf("matchup of type %s already exists for this fixture", matchupType)
	}

	// Create new matchup
	matchup := &models.Matchup{
		FixtureID: fixtureID,
		Type:      matchupType,
		Status:    models.Pending,
		HomeScore: 0,
		AwayScore: 0,
		Notes:     "",
	}

	err = s.matchupRepository.Create(ctx, matchup)
	if err != nil {
		return nil, err
	}

	return matchup, nil
}

// GetOrCreateMatchup gets an existing matchup or creates a new one
func (s *Service) GetOrCreateMatchup(fixtureID uint, matchupType models.MatchupType) (*models.Matchup, error) {
	ctx := context.Background()

	// Try to find existing matchup
	matchup, err := s.matchupRepository.FindByFixtureAndType(ctx, fixtureID, matchupType)
	if err == nil && matchup != nil {
		return matchup, nil
	}

	// Create new matchup if it doesn't exist
	return s.CreateMatchup(fixtureID, matchupType)
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
