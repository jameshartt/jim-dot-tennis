// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"context"
	"time"

	"jim-dot-tennis/internal/models"
)

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
	ClubCount                    int `json:"club_count"`
	PendingPreferredNameRequests int `json:"pending_preferred_name_requests"`
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
	ctx := context.Background()

	// Get actual player count from database
	playerCount, err := s.playerRepository.CountAll(ctx)
	if err != nil {
		return nil, err
	}

	// Get team count for home club
	teamCount, err := s.getHomeClubTeamCount(ctx)
	if err != nil {
		teamCount = 0 // Default to 0 if error
	}

	// Get fixture count for home club
	fixtureCount, err := s.getHomeClubFixtureCount(ctx)
	if err != nil {
		fixtureCount = 0 // Default to 0 if error
	}

	// Get pending preferred name requests count
	pendingPreferredNameRequests, err := s.playerRepository.CountPendingPreferredNameRequests(ctx)
	if err != nil {
		pendingPreferredNameRequests = 0 // Default to 0 if error
	}

	// Get club count
	clubs, err := s.clubRepository.FindAll(ctx)
	clubCount := 0
	if err == nil {
		clubCount = len(clubs)
	}

	stats := Stats{
		PlayerCount:                  playerCount,
		FixtureCount:                 fixtureCount,
		TeamCount:                    teamCount,
		ClubCount:                    clubCount,
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

// getHomeClubTeamCount gets the count of teams for the home club in the active season
func (s *Service) getHomeClubTeamCount(ctx context.Context) (int, error) {
	// Get the active season
	activeSeason, err := s.seasonRepository.FindActive(ctx)
	if err != nil {
		return 0, err
	}
	if activeSeason == nil {
		return 0, nil // No active season
	}

	// Get teams for home club in the active season
	teams, err := s.teamRepository.FindByClubAndSeason(ctx, s.homeClubID, activeSeason.ID)
	if err != nil {
		return 0, err
	}

	return len(teams), nil
}

// getHomeClubFixtureCount gets the count of remaining fixtures for the home club
func (s *Service) getHomeClubFixtureCount(ctx context.Context) (int, error) {
	// Get all teams for home club
	teams, err := s.teamRepository.FindByClub(ctx, s.homeClubID)
	if err != nil {
		return 0, err
	}

	if len(teams) == 0 {
		return 0, nil // No teams found
	}

	// Count remaining fixtures (today or later) for home club teams
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
