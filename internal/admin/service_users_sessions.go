package admin

import (
	"fmt"

	"jim-dot-tennis/internal/models"

	"golang.org/x/crypto/bcrypt"
)

// SessionWithUser represents a session joined with its user info
type SessionWithUser struct {
	models.Session
	Username string `db:"username"`
}

// GetAllUsers retrieves all users for admin management
func (s *Service) GetAllUsers() ([]models.User, error) {
	var users []models.User
	err := s.db.Select(&users, `
		SELECT id, username, password_hash, role, player_id, is_active, created_at, last_login_at
		FROM users
		ORDER BY username ASC
	`)
	return users, err
}

// GetUserByID retrieves a single user by ID
func (s *Service) GetUserByID(id int64) (*models.User, error) {
	var user models.User
	err := s.db.Get(&user, `
		SELECT id, username, password_hash, role, player_id, is_active, created_at, last_login_at
		FROM users WHERE id = ?
	`, id)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// CreateUser creates a new user with a hashed password
func (s *Service) CreateUser(username, password string, role models.Role) (int64, error) {
	hashedPassword, err := hashPassword(password)
	if err != nil {
		return 0, fmt.Errorf("failed to hash password: %w", err)
	}

	result, err := s.db.Exec(`
		INSERT INTO users (username, password_hash, role, is_active, created_at, last_login_at)
		VALUES (?, ?, ?, true, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, username, hashedPassword, role)
	if err != nil {
		return 0, fmt.Errorf("failed to create user: %w", err)
	}

	return result.LastInsertId()
}

// UpdateUserRole changes a user's role
func (s *Service) UpdateUserRole(id int64, role models.Role) error {
	_, err := s.db.Exec(`UPDATE users SET role = ? WHERE id = ?`, role, id)
	return err
}

// ToggleUserActive toggles a user's active status
func (s *Service) ToggleUserActive(id int64) error {
	_, err := s.db.Exec(`UPDATE users SET is_active = NOT is_active WHERE id = ?`, id)
	return err
}

// ResetUserPassword resets a user's password
func (s *Service) ResetUserPassword(id int64, newPassword string) error {
	hashedPassword, err := hashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	_, err = s.db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, hashedPassword, id)
	return err
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

// InvalidateSession marks a session as invalid
func (s *Service) InvalidateSession(sessionID string) error {
	_, err := s.db.Exec(`UPDATE sessions SET is_valid = false WHERE id = ?`, sessionID)
	return err
}

// InvalidateAllUserSessions invalidates all sessions for a user
func (s *Service) InvalidateAllUserSessions(userID int64) error {
	_, err := s.db.Exec(`UPDATE sessions SET is_valid = false WHERE user_id = ?`, userID)
	return err
}

// CleanupExpiredSessions marks all expired sessions as invalid
func (s *Service) CleanupExpiredSessions() error {
	_, err := s.db.Exec(`UPDATE sessions SET is_valid = false WHERE expires_at < CURRENT_TIMESTAMP`)
	return err
}

// hashPassword generates a bcrypt hash for a password
func hashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
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
