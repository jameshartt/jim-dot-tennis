// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"jim-dot-tennis/internal/models"
)

// Account operations (create, role, activation, passwords, session
// revocation) live in internal/accounts, shared with the server command line;
// the users and sessions handlers call s.accounts.

// SessionWithUser represents a session joined with its user info
type SessionWithUser struct {
	models.Session
	Username string `db:"username"`
}

// GetActiveSessions retrieves all valid sessions with usernames
func (s *Service) GetActiveSessions() ([]SessionWithUser, error) {
	var sessions []SessionWithUser
	err := s.db.Select(&sessions, `
		SELECT s.id, s.user_id, s.role, s.created_at, s.expires_at,
		       s.last_activity_at, s.ip, s.user_agent, s.device_info, s.is_valid,
		       u.username
		FROM sessions s
		JOIN users u ON s.user_id = u.id
		WHERE s.is_valid = true AND s.expires_at > CURRENT_TIMESTAMP
		ORDER BY s.last_activity_at DESC
	`)
	return sessions, err
}

// GetRecentLoginAttempts retrieves recent login attempts
func (s *Service) GetRecentLoginAttempts(limit int) ([]models.LoginAttempt, error) {
	var attempts []models.LoginAttempt
	err := s.db.Select(&attempts, `
		SELECT id, username, ip, user_agent, success, created_at
		FROM login_attempts
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	return attempts, err
}
