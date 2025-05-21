package models

import (
	"time"
)

// AccessRole represents the role of a user in the system
type AccessRole string

const (
	RolePlayer  AccessRole = "player"
	RoleCaptain AccessRole = "captain"
	RoleAdmin   AccessRole = "admin"
)

// PlayerAccessToken represents a simple URL-based access token for players
type PlayerAccessToken struct {
	ID         int64     `json:"id"`
	Token      string    `json:"token"`       // The URL token based on tennis pro names
	PlayerID   string    `json:"player_id"`   // Reference to the player
	IsActive   bool      `json:"is_active"`   // Whether the token is still valid
	LastUsedAt time.Time `json:"last_used_at"` // When the token was last used
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// MagicLink represents a temporary access token for captains and admins
type MagicLink struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`      // Email address of the user
	Token     string    `json:"token"`      // The magic link token
	Role      AccessRole `json:"role"`      // Role of the user (captain or admin)
	ExpiresAt time.Time `json:"expires_at"` // When the magic link expires
	UsedAt    time.Time `json:"used_at"`    // When the magic link was used
	CreatedAt time.Time `json:"created_at"`
}

// AccessLog represents a log entry for access attempts
type AccessLog struct {
	ID           int64     `json:"id"`
	TokenType    string    `json:"token_type"`    // 'player' or 'magic'
	TokenID      int64     `json:"token_id"`      // ID from respective token table
	IPAddress    string    `json:"ip_address"`    // IP address of the request
	UserAgent    string    `json:"user_agent"`    // User agent of the request
	AccessedAt   time.Time `json:"accessed_at"`   // When the access was attempted
	Success      bool      `json:"success"`       // Whether the access was successful
	FailureReason string   `json:"failure_reason"` // Reason for failure if any
}

// AccessStats represents statistics about access attempts
type AccessStats struct {
	IPAddress     string    `json:"ip_address"`
	TokenType     string    `json:"token_type"`
	AccessCount   int       `json:"access_count"`
	FailureCount  int       `json:"failure_count"`
	FirstAttempt  time.Time `json:"first_attempt"`
	LastAttempt   time.Time `json:"last_attempt"`
}

// IsSuspicious returns true if the access pattern is suspicious
func (s *AccessStats) IsSuspicious() bool {
	// Consider suspicious if:
	// - More than 10 attempts in an hour
	// - More than 5 failures in an hour
	// - Multiple attempts from same IP with different tokens
	return s.AccessCount > 10 || s.FailureCount > 5
}

// ValidateTokenType ensures the token type is valid
func ValidateTokenType(tokenType string) bool {
	return tokenType == "player" || tokenType == "magic"
}

// ValidateRole ensures the role is valid
func ValidateRole(role AccessRole) bool {
	return role == RolePlayer || role == RoleCaptain || role == RoleAdmin
} 