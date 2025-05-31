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
}

// NewService creates a new admin service
func NewService(db *database.DB) *Service {
	return &Service{
		db:                     db,
		loginAttemptRepository: repository.NewLoginAttemptRepository(db),
		playerRepository:       repository.NewPlayerRepository(db),
		clubRepository:         repository.NewClubRepository(db),
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
	SessionCount int `json:"session_count"`
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

// GetDashboardData retrieves data for the admin dashboard
func (s *Service) GetDashboardData(user *models.User) (*DashboardData, error) {
	// For now, return mock data for stats - these can be implemented later
	// TODO: Replace with actual database queries for stats
	stats := Stats{
		PlayerCount:  12,
		FixtureCount: 8,
		SessionCount: 1,
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
