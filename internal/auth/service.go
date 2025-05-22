package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidToken       = errors.New("invalid token")
	ErrTokenExpired       = errors.New("token expired")
	ErrTooManyAttempts    = errors.New("too many access attempts")
	ErrSuspiciousAccess   = errors.New("suspicious access pattern detected")
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrUserInactive       = errors.New("user account is inactive")
)

// Service handles authentication business logic
type Service struct {
	repo *repository.AuthRepository
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

// CreateUser creates a new user account for captains/admins
func (s *Service) CreateUser(username, password string, role models.AccessRole, playerID *string) (*models.User, error) {
	if !models.ValidateRole(role) {
		return nil, errors.New("invalid role")
	}

	// If playerID is provided, verify it exists
	if playerID != nil {
		exists, err := s.repo.PlayerExists(*playerID)
		if err != nil {
			return nil, fmt.Errorf("failed to verify player: %w", err)
		}
		if !exists {
			return nil, errors.New("player not found")
		}

		// Check if player is already associated with a user
		existingUser, err := s.repo.GetUserByPlayerID(*playerID)
		if err == nil && existingUser != nil {
			return nil, errors.New("player is already associated with a user account")
		}
	}

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &models.User{
		Username:     username,
		PasswordHash: string(hashedPassword),
		Role:         role,
		PlayerID:     playerID,
		IsActive:     true,
	}

	if err := s.repo.CreateUser(user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// AuthenticateUser validates user credentials and logs the attempt
func (s *Service) AuthenticateUser(username, password string, r *http.Request) (*models.User, error) {
	log.Printf("Starting authentication process for username: %s from IP: %s", username, r.RemoteAddr)

	// Check access stats for suspicious patterns
	stats, err := s.repo.GetAccessStats(r.RemoteAddr, "user")
	if err != nil {
		log.Printf("Error getting access stats for IP %s: %v", r.RemoteAddr, err)
		return nil, fmt.Errorf("failed to get access stats: %w", err)
	}

	log.Printf("Access stats for IP %s: attempts=%d, failures=%d", r.RemoteAddr, stats.AccessCount, stats.FailureCount)

	if stats.IsSuspicious() {
		log.Printf("Suspicious access pattern detected for IP %s: %d attempts, %d failures in the last hour",
			r.RemoteAddr, stats.AccessCount, stats.FailureCount)
		s.logAccessAttempt("user", 0, r, false, ErrSuspiciousAccess.Error())
		return nil, ErrSuspiciousAccess
	}

	// Get user by username
	user, err := s.repo.GetUserByUsername(username)
	if err != nil {
		log.Printf("Failed to find user with username: %s", username)
		s.logAccessAttempt("user", 0, r, false, ErrInvalidCredentials.Error())
		return nil, ErrInvalidCredentials
	}

	// Check if user is active
	if !user.IsActive {
		log.Printf("Login attempt for inactive user account: %s (ID: %d)", username, user.ID)
		s.logAccessAttempt("user", user.ID, r, false, ErrUserInactive.Error())
		return nil, ErrUserInactive
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		log.Printf("Invalid password for user: %s (ID: %d)", username, user.ID)
		s.logAccessAttempt("user", user.ID, r, false, ErrInvalidCredentials.Error())
		return nil, ErrInvalidCredentials
	}

	// Log successful access
	log.Printf("Password verified successfully for user: %s (ID: %d)", username, user.ID)
	if err := s.logAccessAttempt("user", user.ID, r, true, ""); err != nil {
		log.Printf("Error logging successful access attempt: %v", err)
		return nil, fmt.Errorf("failed to log access: %w", err)
	}

	// Update last login timestamp
	if err := s.repo.UpdateUserLastLogin(user.ID); err != nil {
		log.Printf("Error updating last login timestamp for user %s (ID: %d): %v", username, user.ID, err)
		return nil, fmt.Errorf("failed to update last login: %w", err)
	}
	log.Printf("Updated last login timestamp for user: %s (ID: %d)", username, user.ID)

	return user, nil
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

// DeactivatePlayerToken deactivates a player access token
func (s *Service) DeactivatePlayerToken(tokenID int64) error {
	return s.repo.DeactivatePlayerAccessToken(tokenID)
}

// DeactivateUser deactivates a user account
func (s *Service) DeactivateUser(userID int64) error {
	return s.repo.DeactivateUser(userID)
}

// AssociatePlayerWithUser links a player to a user account
func (s *Service) AssociatePlayerWithUser(userID int64, playerID string) error {
	// Verify user exists and is active
	user, err := s.repo.GetUserByID(userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}
	if !user.IsActive {
		return errors.New("user account is inactive")
	}

	// Verify player exists
	exists, err := s.repo.PlayerExists(playerID)
	if err != nil {
		return fmt.Errorf("failed to verify player: %w", err)
	}
	if !exists {
		return errors.New("player not found")
	}

	// Check if player is already associated with a user
	existingUser, err := s.repo.GetUserByPlayerID(playerID)
	if err == nil && existingUser != nil {
		return errors.New("player is already associated with a user account")
	}

	// Update user with player ID
	return s.repo.UpdateUserPlayerID(userID, &playerID)
}

// DisassociatePlayerFromUser removes the player association from a user account
func (s *Service) DisassociatePlayerFromUser(userID int64) error {
	// Verify user exists
	user, err := s.repo.GetUserByID(userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Only allow if user is not an admin (admins might need their player association)
	if user.Role == models.RoleAdmin {
		return errors.New("cannot remove player association from admin accounts")
	}

	// Update user to remove player ID
	return s.repo.UpdateUserPlayerID(userID, nil)
}

// GetUserByPlayerID retrieves a user account by player ID
func (s *Service) GetUserByPlayerID(playerID string) (*models.User, error) {
	return s.repo.GetUserByPlayerID(playerID)
}
