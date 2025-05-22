package repository

import (
	"database/sql"
	"errors"
	"time"

	"jim-dot-tennis/internal/models"
)

// AuthRepository handles database operations for authentication
type AuthRepository struct {
	db *sql.DB
}

// NewAuthRepository creates a new AuthRepository
func NewAuthRepository(db *sql.DB) *AuthRepository {
	return &AuthRepository{db: db}
}

// CreatePlayerAccessToken creates a new player access token
func (r *AuthRepository) CreatePlayerAccessToken(token *models.PlayerAccessToken) error {
	query := `
		INSERT INTO player_access_tokens (token, player_id, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`
	result, err := r.db.Exec(query, token.Token, token.PlayerID, token.IsActive, time.Now(), time.Now())
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	token.ID = id
	return nil
}

// GetPlayerAccessToken retrieves a player access token by token string
func (r *AuthRepository) GetPlayerAccessToken(token string) (*models.PlayerAccessToken, error) {
	query := `
		SELECT id, token, player_id, is_active, last_used_at, created_at, updated_at
		FROM player_access_tokens
		WHERE token = ? AND is_active = true
	`
	t := &models.PlayerAccessToken{}
	err := r.db.QueryRow(query, token).Scan(
		&t.ID, &t.Token, &t.PlayerID, &t.IsActive, &t.LastUsedAt,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, errors.New("token not found")
	}
	if err != nil {
		return nil, err
	}
	return t, nil
}

// UpdatePlayerAccessTokenLastUsed updates the last used timestamp of a player access token
func (r *AuthRepository) UpdatePlayerAccessTokenLastUsed(tokenID int64) error {
	query := `
		UPDATE player_access_tokens
		SET last_used_at = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query, time.Now(), time.Now(), tokenID)
	return err
}

// LogAccess logs an access attempt
func (r *AuthRepository) LogAccess(log *models.AccessLog) error {
	query := `
		INSERT INTO access_logs (token_type, token_id, ip_address, user_agent, accessed_at, success, failure_reason)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	result, err := r.db.Exec(query,
		log.TokenType, log.TokenID, log.IPAddress, log.UserAgent,
		log.AccessedAt, log.Success, log.FailureReason,
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	log.ID = id
	return nil
}

// GetAccessStats retrieves access statistics for the last hour
func (r *AuthRepository) GetAccessStats(ipAddress string, tokenType string) (*models.AccessStats, error) {
	query := `
		SELECT 
			ip_address,
			token_type,
			COUNT(*) as access_count,
			COUNT(CASE WHEN success = 0 THEN 1 END) as failure_count,
			MIN(accessed_at) as first_attempt,
			MAX(accessed_at) as last_attempt
		FROM access_logs
		WHERE ip_address = ? AND token_type = ? AND accessed_at > datetime('now', '-1 hour')
		GROUP BY ip_address, token_type
	`
	stats := &models.AccessStats{}
	err := r.db.QueryRow(query, ipAddress, tokenType).Scan(
		&stats.IPAddress, &stats.TokenType, &stats.AccessCount,
		&stats.FailureCount, &stats.FirstAttempt, &stats.LastAttempt,
	)
	if err == sql.ErrNoRows {
		// No access attempts found, return empty stats
		stats.IPAddress = ipAddress
		stats.TokenType = tokenType
		return stats, nil
	}
	if err != nil {
		return nil, err
	}
	return stats, nil
}

// DeactivatePlayerAccessToken deactivates a player access token
func (r *AuthRepository) DeactivatePlayerAccessToken(tokenID int64) error {
	query := `
		UPDATE player_access_tokens
		SET is_active = false, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query, time.Now(), tokenID)
	return err
}

// CreateUser creates a new user account
func (r *AuthRepository) CreateUser(user *models.User) error {
	query := `
		INSERT INTO users (username, password_hash, role, player_id, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	result, err := r.db.Exec(query,
		user.Username, user.PasswordHash, user.Role, user.PlayerID,
		user.IsActive, time.Now(), time.Now(),
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	user.ID = id
	return nil
}

// GetUserByUsername retrieves a user by username
func (r *AuthRepository) GetUserByUsername(username string) (*models.User, error) {
	query := `
		SELECT id, username, password_hash, role, player_id, is_active, last_login_at, created_at, updated_at
		FROM users
		WHERE username = ?
	`
	user := &models.User{}
	err := r.db.QueryRow(query, username).Scan(
		&user.ID, &user.Username, &user.PasswordHash, &user.Role,
		&user.PlayerID, &user.IsActive, &user.LastLoginAt,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, errors.New("user not found")
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

// GetUserByID retrieves a user by ID
func (r *AuthRepository) GetUserByID(id int64) (*models.User, error) {
	query := `
		SELECT id, username, password_hash, role, player_id, is_active, last_login_at, created_at, updated_at
		FROM users
		WHERE id = ?
	`
	user := &models.User{}
	err := r.db.QueryRow(query, id).Scan(
		&user.ID, &user.Username, &user.PasswordHash, &user.Role,
		&user.PlayerID, &user.IsActive, &user.LastLoginAt,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, errors.New("user not found")
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

// GetUserByPlayerID retrieves a user by player ID
func (r *AuthRepository) GetUserByPlayerID(playerID string) (*models.User, error) {
	query := `
		SELECT id, username, password_hash, role, player_id, is_active, last_login_at, created_at, updated_at
		FROM users
		WHERE player_id = ?
	`
	user := &models.User{}
	err := r.db.QueryRow(query, playerID).Scan(
		&user.ID, &user.Username, &user.PasswordHash, &user.Role,
		&user.PlayerID, &user.IsActive, &user.LastLoginAt,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, errors.New("user not found")
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

// UpdateUserLastLogin updates the last login timestamp for a user
func (r *AuthRepository) UpdateUserLastLogin(userID int64) error {
	query := `
		UPDATE users
		SET last_login_at = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query, time.Now(), time.Now(), userID)
	return err
}

// UpdateUserPlayerID updates the player ID for a user
func (r *AuthRepository) UpdateUserPlayerID(userID int64, playerID *string) error {
	query := `
		UPDATE users
		SET player_id = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query, playerID, time.Now(), userID)
	return err
}

// DeactivateUser deactivates a user account
func (r *AuthRepository) DeactivateUser(userID int64) error {
	query := `
		UPDATE users
		SET is_active = false, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query, time.Now(), userID)
	return err
}

// PlayerExists checks if a player exists
func (r *AuthRepository) PlayerExists(playerID string) (bool, error) {
	query := `
		SELECT EXISTS(SELECT 1 FROM players WHERE id = ?)
	`
	var exists bool
	err := r.db.QueryRow(query, playerID).Scan(&exists)
	return exists, err
}
