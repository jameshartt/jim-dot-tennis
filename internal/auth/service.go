package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrSessionExpired     = errors.New("session has expired")
	ErrSessionInvalid     = errors.New("invalid session")
	ErrUserInactive       = errors.New("user account is inactive")
	ErrTooManyAttempts    = errors.New("too many login attempts")
)

// Config holds configuration for the auth service
type Config struct {
	SessionDuration    time.Duration
	CookieName         string
	CookieSecure       bool
	CookieHttpOnly     bool
	CookieSameSite     http.SameSite
	CookiePath         string
	MaxLoginAttempts   int
	LoginAttemptWindow time.Duration
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		SessionDuration:    7 * 24 * time.Hour, // 7 days
		CookieName:         "session_token",
		CookieSecure:       true,
		CookieHttpOnly:     true,
		CookieSameSite:     http.SameSiteStrictMode,
		CookiePath:         "/",
		MaxLoginAttempts:   5,
		LoginAttemptWindow: 15 * time.Minute,
	}
}

// Service provides authentication-related functionality
type Service struct {
	db                     *database.DB
	config                 Config
	loginAttemptRepository repository.LoginAttemptRepository
}

// NewService creates a new auth service
func NewService(db *database.DB, config Config) *Service {
	return &Service{
		db:                     db,
		config:                 config,
		loginAttemptRepository: repository.NewLoginAttemptRepository(db),
	}
}

// Login authenticates a user and creates a new session
func (s *Service) Login(username, password string, r *http.Request) (*models.Session, error) {
	log.Printf("Login attempt for username: %s from IP: %s", username, r.RemoteAddr)

	// Check for too many failed login attempts
	if tooMany, err := s.tooManyFailedAttempts(username, r.RemoteAddr); err != nil {
		log.Printf("Error checking login attempts: %v", err)
	} else if tooMany {
		log.Printf("Too many failed login attempts for %s", username)
		s.recordLoginAttempt(username, r, false)
		return nil, ErrTooManyAttempts
	}

	// Get user by username
	var user models.User
	err := s.db.Get(&user, `
		SELECT * FROM users 
		WHERE username = ? AND is_active = true
	`, username)
	if err != nil {
		log.Printf("User not found or inactive: %s, error: %v", username, err)
		s.recordLoginAttempt(username, r, false)
		return nil, ErrInvalidCredentials
	}
	log.Printf("User found: %s (ID: %d, Role: %s)", user.Username, user.ID, user.Role)

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		log.Printf("Password verification failed for user %s: %v", username, err)
		s.recordLoginAttempt(username, r, false)
		return nil, ErrInvalidCredentials
	}
	log.Printf("Password verification successful for user %s", username)

	// Record successful login attempt
	s.recordLoginAttempt(username, r, true)

	// Update last login time
	if _, err := s.db.Exec(`
		UPDATE users 
		SET last_login_at = ? 
		WHERE id = ?
	`, time.Now(), user.ID); err != nil {
		log.Printf("Failed to update last login time: %v", err)
	}

	// Create new session
	sessionID, err := generateSecureToken(32)
	if err != nil {
		log.Printf("Failed to generate session token: %v", err)
		return nil, fmt.Errorf("failed to generate session token: %w", err)
	}
	log.Printf("Generated new session token for user %s", username)

	// Extract device info
	deviceInfo := extractDeviceInfo(r)

	session := &models.Session{
		ID:             sessionID,
		UserID:         user.ID,
		Role:           user.Role,
		CreatedAt:      time.Now(),
		ExpiresAt:      time.Now().Add(s.config.SessionDuration),
		LastActivityAt: time.Now(),
		IP:             r.RemoteAddr,
		UserAgent:      r.UserAgent(),
		DeviceInfo:     deviceInfo,
		IsValid:        true,
	}

	// Save session to database
	_, err = s.db.NamedExec(`
		INSERT INTO sessions (
			id, user_id, role, created_at, expires_at, 
			last_activity_at, ip, user_agent, device_info, is_valid
		) VALUES (
			:id, :user_id, :role, :created_at, :expires_at, 
			:last_activity_at, :ip, :user_agent, :device_info, :is_valid
		)
	`, session)
	if err != nil {
		log.Printf("Failed to create session in database: %v", err)
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	log.Printf("Successfully created session in database for user %s", username)

	return session, nil
}

// ValidateSession checks if a session is valid and refreshes it
func (s *Service) ValidateSession(sessionID string, r *http.Request) (*models.Session, error) {
	log.Printf("Validating session: %s", sessionID)

	var session models.Session
	err := s.db.Get(&session, `
		SELECT * FROM sessions 
		WHERE id = ? AND is_valid = true
	`, sessionID)
	if err != nil {
		log.Printf("Session validation failed - invalid or not found: %v", err)
		return nil, ErrSessionInvalid
	}

	// Check if session has expired
	if time.Now().After(session.ExpiresAt) {
		log.Printf("Session expired: %s (expired at %v)", sessionID, session.ExpiresAt)
		s.InvalidateSession(sessionID)
		return nil, ErrSessionExpired
	}

	// Optional: You could validate IP and user agent for additional security
	// This is a trade-off between security and user experience
	// Uncomment this if you want stricter security
	/*
		if session.IP != r.RemoteAddr || session.UserAgent != r.UserAgent() {
			log.Printf("Session security warning: IP/UserAgent mismatch for session %s", sessionID)
			// You might choose to invalidate or just log the discrepancy
			// s.InvalidateSession(sessionID)
			// return nil, ErrSessionInvalid
		}
	*/

	// Update last activity time
	if _, err := s.db.Exec(`
		UPDATE sessions 
		SET last_activity_at = ?, 
		    expires_at = ? 
		WHERE id = ?
	`, time.Now(), time.Now().Add(s.config.SessionDuration), sessionID); err != nil {
		log.Printf("Failed to update session activity: %v", err)
	} else {
		log.Printf("Session validated and refreshed: %s (user ID: %d, role: %s)", sessionID, session.UserID, session.Role)
	}

	return &session, nil
}

// InvalidateSession marks a session as invalid (logout)
func (s *Service) InvalidateSession(sessionID string) error {
	_, err := s.db.Exec(`
		UPDATE sessions 
		SET is_valid = false 
		WHERE id = ?
	`, sessionID)
	return err
}

// InvalidateAllUserSessions invalidates all sessions for a user
func (s *Service) InvalidateAllUserSessions(userID int64) error {
	_, err := s.db.Exec(`
		UPDATE sessions 
		SET is_valid = false 
		WHERE user_id = ?
	`, userID)
	return err
}

// CreateUser creates a new user
func (s *Service) CreateUser(username, password string, role models.Role) (int64, error) {
	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return 0, fmt.Errorf("failed to hash password: %w", err)
	}

	// Insert the user
	result, err := s.db.Exec(`
		INSERT INTO users (
			username, password_hash, role, is_active, created_at, last_login_at
		) VALUES (
			?, ?, ?, true, ?, ?
		)
	`, username, string(hashedPassword), role, time.Now(), time.Now())
	if err != nil {
		return 0, fmt.Errorf("failed to create user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get user ID: %w", err)
	}

	return id, nil
}

// SetSessionCookie sets the session cookie in the response
func (s *Service) SetSessionCookie(w http.ResponseWriter, session *models.Session) {
	log.Printf("Setting session cookie: name=%s, value=%s, expires=%v, secure=%v, httpOnly=%v, path=%s",
		s.config.CookieName, session.ID, session.ExpiresAt, s.config.CookieSecure, s.config.CookieHttpOnly, s.config.CookiePath)

	http.SetCookie(w, &http.Cookie{
		Name:     s.config.CookieName,
		Value:    session.ID,
		Path:     s.config.CookiePath,
		Expires:  session.ExpiresAt,
		HttpOnly: s.config.CookieHttpOnly,
		Secure:   s.config.CookieSecure,
		SameSite: s.config.CookieSameSite,
	})
}

// ClearSessionCookie removes the session cookie
func (s *Service) ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.config.CookieName,
		Value:    "",
		Path:     s.config.CookiePath,
		MaxAge:   -1,
		HttpOnly: s.config.CookieHttpOnly,
		Secure:   s.config.CookieSecure,
		SameSite: s.config.CookieSameSite,
	})
}

// ListUsers retrieves all users
func (s *Service) ListUsers() ([]models.User, error) {
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
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	_, err = s.db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, string(hashedPassword), id)
	return err
}

// SessionWithUser represents a session joined with its user info
type SessionWithUser struct {
	models.Session
	Username string `db:"username"`
}

// ListActiveSessions retrieves all valid sessions with usernames
func (s *Service) ListActiveSessions() ([]SessionWithUser, error) {
	var sessions []SessionWithUser
	err := s.db.Select(&sessions, `
		SELECT s.id, s.user_id, s.role, s.created_at, s.expires_at,
		       s.last_activity_at, s.ip, s.user_agent, s.device_info, s.is_valid,
		       u.username
		FROM sessions s
		JOIN users u ON s.user_id = u.id
		WHERE s.is_valid = true AND s.expires_at > ?
		ORDER BY s.last_activity_at DESC
	`, time.Now())
	return sessions, err
}

// ListAllLoginAttempts retrieves recent login attempts
func (s *Service) ListAllLoginAttempts(limit int) ([]models.LoginAttempt, error) {
	var attempts []models.LoginAttempt
	err := s.db.Select(&attempts, `
		SELECT id, username, ip, user_agent, success, created_at
		FROM login_attempts
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	return attempts, err
}

// CleanupExpiredSessions removes all expired sessions
func (s *Service) CleanupExpiredSessions() error {
	_, err := s.db.Exec(`
		UPDATE sessions 
		SET is_valid = false 
		WHERE expires_at < ?
	`, time.Now())
	return err
}

// recordLoginAttempt records a login attempt
func (s *Service) recordLoginAttempt(username string, r *http.Request, success bool) {
	attempt := &models.LoginAttempt{
		Username:  username,
		IP:        r.RemoteAddr,
		UserAgent: r.UserAgent(),
		Success:   success,
		CreatedAt: time.Now(),
	}

	if err := s.loginAttemptRepository.Create(attempt); err != nil {
		log.Printf("Failed to record login attempt: %v", err)
	}
}

// tooManyFailedAttempts checks if there have been too many failed login attempts
func (s *Service) tooManyFailedAttempts(username, ip string) (bool, error) {
	// Use repository to check recent failed attempts
	attempts, err := s.loginAttemptRepository.FindByUsernameAndIP(username, ip, s.config.MaxLoginAttempts)
	if err != nil {
		return false, err
	}

	// Count failed attempts within the window
	count := 0
	cutoff := time.Now().Add(-s.config.LoginAttemptWindow)
	for _, attempt := range attempts {
		if !attempt.Success && attempt.CreatedAt.After(cutoff) {
			count++
		}
	}

	return count >= s.config.MaxLoginAttempts, nil
}

// generateSecureToken generates a cryptographically secure random token
func generateSecureToken(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// extractDeviceInfo extracts basic device info from the user agent
func extractDeviceInfo(r *http.Request) string {
	return r.UserAgent() // For simplicity; could use a UA parsing library for more details
}
