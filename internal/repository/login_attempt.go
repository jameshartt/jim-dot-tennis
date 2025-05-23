package repository

import (
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
)

// LoginAttemptRepository defines the interface for login attempt data access
type LoginAttemptRepository interface {
	FindByUsername(username string, limit int) ([]models.LoginAttempt, error)
	FindByUsernameAndIP(username, ip string, limit int) ([]models.LoginAttempt, error)
	Create(attempt *models.LoginAttempt) error
}

// loginAttemptRepository implements LoginAttemptRepository
type loginAttemptRepository struct {
	db *database.DB
}

// NewLoginAttemptRepository creates a new login attempt repository
func NewLoginAttemptRepository(db *database.DB) LoginAttemptRepository {
	return &loginAttemptRepository{
		db: db,
	}
}

// FindByUsername retrieves login attempts for a specific username
func (r *loginAttemptRepository) FindByUsername(username string, limit int) ([]models.LoginAttempt, error) {
	var attempts []models.LoginAttempt
	err := r.db.Select(&attempts, `
		SELECT id, username, ip, user_agent, success, created_at 
		FROM login_attempts 
		WHERE username = ? 
		ORDER BY created_at DESC 
		LIMIT ?
	`, username, limit)
	return attempts, err
}

// FindByUsernameAndIP retrieves login attempts for a specific username and IP
func (r *loginAttemptRepository) FindByUsernameAndIP(username, ip string, limit int) ([]models.LoginAttempt, error) {
	var attempts []models.LoginAttempt
	err := r.db.Select(&attempts, `
		SELECT id, username, ip, user_agent, success, created_at 
		FROM login_attempts 
		WHERE username = ? AND ip = ? 
		ORDER BY created_at DESC 
		LIMIT ?
	`, username, ip, limit)
	return attempts, err
}

// Create inserts a new login attempt record
func (r *loginAttemptRepository) Create(attempt *models.LoginAttempt) error {
	_, err := r.db.NamedExec(`
		INSERT INTO login_attempts (username, ip, user_agent, success, created_at)
		VALUES (:username, :ip, :user_agent, :success, :created_at)
	`, attempt)
	return err
}
