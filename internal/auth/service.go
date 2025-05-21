package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
)

var (
	ErrInvalidToken     = errors.New("invalid token")
	ErrTokenExpired     = errors.New("token expired")
	ErrTokenUsed        = errors.New("token already used")
	ErrTooManyAttempts  = errors.New("too many access attempts")
	ErrSuspiciousAccess = errors.New("suspicious access pattern detected")
)

// Service handles authentication business logic
type Service struct {
	repo *repository.AuthRepository
	// TODO: Add email service for magic links
}

// NewService creates a new auth service
func NewService(repo *repository.AuthRepository) *Service {
	return &Service{repo: repo}
}

// generateSecureToken generates a cryptographically secure random token
func generateSecureToken(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// CreatePlayerAccessToken creates a new player access token
// The token will be based on tennis pro names (to be provided by the user)
func (s *Service) CreatePlayerAccessToken(playerID string, proNames []string) (*models.PlayerAccessToken, error) {
	if len(proNames) != 3 {
		return nil, errors.New("exactly three pro names are required")
	}

	// Convert pro names to lowercase and join with hyphens
	token := strings.ToLower(strings.Join(proNames, "-"))

	t := &models.PlayerAccessToken{
		Token:    token,
		PlayerID: playerID,
		IsActive: true,
	}

	if err := s.repo.CreatePlayerAccessToken(t); err != nil {
		return nil, fmt.Errorf("failed to create player access token: %w", err)
	}

	return t, nil
}

// ValidatePlayerAccess validates a player access token and logs the attempt
func (s *Service) ValidatePlayerAccess(token string, r *http.Request) (*models.PlayerAccessToken, error) {
	// Check access stats for suspicious patterns
	stats, err := s.repo.GetAccessStats(r.RemoteAddr, "player")
	if err != nil {
		return nil, fmt.Errorf("failed to get access stats: %w", err)
	}

	if stats.IsSuspicious() {
		s.logAccessAttempt("player", 0, r, false, ErrSuspiciousAccess.Error())
		return nil, ErrSuspiciousAccess
	}

	// Get and validate the token
	t, err := s.repo.GetPlayerAccessToken(token)
	if err != nil {
		s.logAccessAttempt("player", 0, r, false, err.Error())
		return nil, ErrInvalidToken
	}

	// Log successful access
	if err := s.logAccessAttempt("player", t.ID, r, true, ""); err != nil {
		return nil, fmt.Errorf("failed to log access: %w", err)
	}

	// Update last used timestamp
	if err := s.repo.UpdatePlayerAccessTokenLastUsed(t.ID); err != nil {
		return nil, fmt.Errorf("failed to update token last used: %w", err)
	}

	return t, nil
}

// CreateMagicLink creates a new magic link for captain/admin access
func (s *Service) CreateMagicLink(email string, role models.AccessRole) (*models.MagicLink, error) {
	if !models.ValidateRole(role) {
		return nil, errors.New("invalid role")
	}

	// Generate a secure token
	token, err := generateSecureToken(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Create magic link with 1-hour expiration
	link := &models.MagicLink{
		Email:     email,
		Token:     token,
		Role:      role,
		ExpiresAt: time.Now().Add(time.Hour),
	}

	if err := s.repo.CreateMagicLink(link); err != nil {
		return nil, fmt.Errorf("failed to create magic link: %w", err)
	}

	// TODO: Send magic link via email
	// For now, just return the token (in production, this would be sent via email)
	return link, nil
}

// ValidateMagicLink validates a magic link and logs the attempt
func (s *Service) ValidateMagicLink(token string, r *http.Request) (*models.MagicLink, error) {
	// Check access stats for suspicious patterns
	stats, err := s.repo.GetAccessStats(r.RemoteAddr, "magic")
	if err != nil {
		return nil, fmt.Errorf("failed to get access stats: %w", err)
	}

	if stats.IsSuspicious() {
		s.logAccessAttempt("magic", 0, r, false, ErrSuspiciousAccess.Error())
		return nil, ErrSuspiciousAccess
	}

	// Get and validate the magic link
	link, err := s.repo.GetMagicLink(token)
	if err != nil {
		s.logAccessAttempt("magic", 0, r, false, err.Error())
		if errors.Is(err, errors.New("magic link not found or expired")) {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}

	// Mark the magic link as used
	if err := s.repo.MarkMagicLinkAsUsed(link.ID); err != nil {
		return nil, fmt.Errorf("failed to mark magic link as used: %w", err)
	}

	// Log successful access
	if err := s.logAccessAttempt("magic", link.ID, r, true, ""); err != nil {
		return nil, fmt.Errorf("failed to log access: %w", err)
	}

	return link, nil
}

// logAccessAttempt logs an access attempt
func (s *Service) logAccessAttempt(tokenType string, tokenID int64, r *http.Request, success bool, failureReason string) error {
	log := &models.AccessLog{
		TokenType:     tokenType,
		TokenID:       tokenID,
		IPAddress:     r.RemoteAddr,
		UserAgent:     r.UserAgent(),
		AccessedAt:    time.Now(),
		Success:       success,
		FailureReason: failureReason,
	}

	return s.repo.LogAccess(log)
}

// CleanupExpiredTokens removes expired magic links
// This should be called periodically (e.g., via a background job)
func (s *Service) CleanupExpiredTokens() error {
	return s.repo.CleanupExpiredMagicLinks()
}

// DeactivatePlayerToken deactivates a player access token
func (s *Service) DeactivatePlayerToken(tokenID int64) error {
	return s.repo.DeactivatePlayerAccessToken(tokenID)
} 