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

// CreateMagicLink creates a new magic link
func (r *AuthRepository) CreateMagicLink(link *models.MagicLink) error {
	query := `
		INSERT INTO magic_links (email, token, role, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?)
	`
	result, err := r.db.Exec(query, link.Email, link.Token, link.Role, link.ExpiresAt, time.Now())
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	link.ID = id
	return nil
}

// GetMagicLink retrieves a magic link by token
func (r *AuthRepository) GetMagicLink(token string) (*models.MagicLink, error) {
	query := `
		SELECT id, email, token, role, expires_at, used_at, created_at
		FROM magic_links
		WHERE token = ? AND expires_at > ? AND used_at IS NULL
	`
	link := &models.MagicLink{}
	err := r.db.QueryRow(query, token, time.Now()).Scan(
		&link.ID, &link.Email, &link.Token, &link.Role,
		&link.ExpiresAt, &link.UsedAt, &link.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, errors.New("magic link not found or expired")
	}
	if err != nil {
		return nil, err
	}
	return link, nil
}

// MarkMagicLinkAsUsed marks a magic link as used
func (r *AuthRepository) MarkMagicLinkAsUsed(tokenID int64) error {
	query := `
		UPDATE magic_links
		SET used_at = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query, time.Now(), tokenID)
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

// CleanupExpiredMagicLinks removes expired magic links
func (r *AuthRepository) CleanupExpiredMagicLinks() error {
	query := `
		DELETE FROM magic_links
		WHERE expires_at < ? OR (used_at IS NOT NULL AND used_at < datetime('now', '-7 days'))
	`
	_, err := r.db.Exec(query, time.Now())
	return err
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