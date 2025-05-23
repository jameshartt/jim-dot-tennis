package admin

import (
	"time"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
)

// Service handles admin business logic
type Service struct {
	db                     *database.DB
	loginAttemptRepository repository.LoginAttemptRepository
}

// NewService creates a new admin service
func NewService(db *database.DB) *Service {
	return &Service{
		db:                     db,
		loginAttemptRepository: repository.NewLoginAttemptRepository(db),
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
func (s *Service) GetPlayers() (interface{}, error) {
	// TODO: Implement player retrieval from database
	return nil, nil
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
