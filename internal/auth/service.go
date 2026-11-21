// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
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
	SessionDuration time.Duration
	// AbsoluteSessionDuration is the hard ceiling on a session's lifetime,
	// measured from when it was created. Because ExpiresAt slides forward on
	// every request, an active (or stolen-and-active) session would otherwise
	// never expire; this caps how long any single login can live. Set to 0 to
	// disable the absolute cap.
	AbsoluteSessionDuration time.Duration
	CookieName              string
	CookieSecure            bool
	CookieHttpOnly          bool
	CookieSameSite          http.SameSite
	CookiePath              string
	MaxLoginAttempts        int
	LoginAttemptWindow      time.Duration
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		SessionDuration:         7 * 24 * time.Hour,  // 7 days sliding
		AbsoluteSessionDuration: 30 * 24 * time.Hour, // 30 day hard ceiling
		CookieName:              "session_token",
		CookieSecure:            true,
		CookieHttpOnly:          true,
		CookieSameSite:          http.SameSiteStrictMode,
		CookiePath:              "/",
		MaxLoginAttempts:        5,
		LoginAttemptWindow:      15 * time.Minute,
	}
}

// Service provides authentication-related functionality
type Service struct {
	db                     *database.DB
	config                 Config
	loginAttemptRepository repository.LoginAttemptRepository
	signInAudit            SignInAudit // see SetSignInAudit (throttle.go)
}

// NewService creates a new auth service
func NewService(db *database.DB, config Config) *Service {
	return &Service{
		db:                     db,
		config:                 config,
		loginAttemptRepository: repository.NewLoginAttemptRepository(db),
	}
}

// Login authenticates a user and creates a new session. Attempted usernames
// and failure reasons are not logged (a username field often receives a
// password typed in the wrong box); they are in login_attempts, which
// administrators can see.
func (s *Service) Login(username, password string, r *http.Request) (*models.Session, error) {
	// Throttled attempts are refused without being recorded, so hammering a
	// locked-out pair does not extend the lockout (see loginThrottled).
	if throttled, err := s.loginThrottled(username, r); err != nil {
		log.Printf("Error checking login attempts: %v", err)
	} else if throttled {
		log.Printf("Password sign-in throttled for %s", ThrottleKey(r))
		return nil, ErrTooManyAttempts
	}

	// Get user by username
	var user models.User
	err := s.db.Get(&user, `
		SELECT * FROM users
		WHERE username = ? AND is_active = true
	`, username)
	if err != nil {
		s.recordLoginAttempt(username, r, false)
		return nil, ErrInvalidCredentials
	}

	// Verify password. An account with no password (passkeys only, or an
	// invite not yet accepted) can never sign in this way.
	if user.PasswordHash == "" {
		s.recordLoginAttempt(username, r, false)
		return nil, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		s.recordLoginAttempt(username, r, false)
		return nil, ErrInvalidCredentials
	}

	// Record successful login attempt
	s.recordLoginAttempt(username, r, true)

	return s.completeLogin(&user, r, SignInPassword)
}

// completeLogin is the one path from "this user has proven who they are" to
// a session, whatever the sign-in method (password, passkey): it stamps the
// last sign-in time and creates the session. Because both methods end here,
// their sessions are identical and follow the same expiry rules. method (one
// of the SignIn constants) is only recorded: an administrator's sign-in goes
// to the account audit log through SetSignInAudit.
func (s *Service) completeLogin(user *models.User, r *http.Request, method string) (*models.Session, error) {
	if _, err := s.db.Exec(`
		UPDATE users
		SET last_login_at = ?
		WHERE id = ?
	`, time.Now(), user.ID); err != nil {
		log.Printf("Failed to update last login time: %v", err)
	}

	session, err := s.createSession(user, r)
	if err != nil {
		return nil, err
	}
	s.auditSignIn(r.Context(), user, method, r)
	return session, nil
}

// CountSignInMethods is the number of ways userID can sign in (passkeys plus
// a password). Removing a sign-in method is only allowed while this is more
// than one; see repository.CountSignInMethods, the single definition.
func (s *Service) CountSignInMethods(ctx context.Context, userID int64) (int, error) {
	return repository.CountSignInMethods(ctx, s.db, userID)
}

// sessionFingerprint is a non-reversible fingerprint of the request's session
// cookie ("" without one), used to tie a passkey enrolment to the session
// that began it.
func (s *Service) sessionFingerprint(r *http.Request) string {
	cookie, err := r.Cookie(s.config.CookieName)
	if err != nil || cookie.Value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte("passkey-ceremony:" + cookie.Value))
	return hex.EncodeToString(sum[:])
}

// createSession stores a new session for user and returns it. Only the
// token's hash is stored; the returned session carries the token itself
// (Token) for the cookie.
func (s *Service) createSession(user *models.User, r *http.Request) (*models.Session, error) {
	token, err := generateSecureToken(32)
	if err != nil {
		log.Printf("Failed to generate session token: %v", err)
		return nil, fmt.Errorf("failed to generate session token: %w", err)
	}

	now := time.Now()
	session := &models.Session{
		Token:          token,
		TokenHash:      HashSessionToken(token),
		UserID:         user.ID,
		Role:           user.Role,
		CreatedAt:      now,
		ExpiresAt:      now.Add(s.config.SessionDuration),
		LastActivityAt: now,
		IP:             clientIP(r),
		UserAgent:      r.UserAgent(),
		DeviceInfo:     extractDeviceInfo(r),
		IsValid:        true,
	}

	res, err := s.db.NamedExec(`
		INSERT INTO sessions (
			token_hash, user_id, role, created_at, expires_at,
			last_activity_at, ip, user_agent, device_info, is_valid
		) VALUES (
			:token_hash, :user_id, :role, :created_at, :expires_at,
			:last_activity_at, :ip, :user_agent, :device_info, :is_valid
		)
	`, session)
	if err != nil {
		log.Printf("Failed to create session in database: %v", err)
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	if session.ID, err = res.LastInsertId(); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	return session, nil
}

// StartSession signs the active user userID in on this browser without a
// password check (the caller has just proven who they are, for example by
// creating the account in first-run setup or by using an invite or recovery
// link). It goes through completeLogin like every other sign-in. Any session the request already
// carries is invalidated first, so a session ID planted before sign-in is
// never promoted (session fixation); the new session always has a fresh ID.
func (s *Service) StartSession(w http.ResponseWriter, r *http.Request, userID int64) error {
	if cookie, err := r.Cookie(s.config.CookieName); err == nil && cookie.Value != "" {
		if err := s.InvalidateSession(cookie.Value); err != nil {
			log.Printf("Failed to invalidate the previous session: %v", err)
		}
	}
	var user models.User
	if err := s.db.Get(&user, `SELECT * FROM users WHERE id = ? AND is_active = true`, userID); err != nil {
		return fmt.Errorf("start session: load user %d: %w", userID, err)
	}
	session, err := s.completeLogin(&user, r, SignInNewSession)
	if err != nil {
		return err
	}
	s.SetSessionCookie(w, session)
	return nil
}

// SignedInUser returns the active user behind the request's session cookie,
// or ok=false when there is no valid session.
func (s *Service) SignedInUser(r *http.Request) (*models.User, bool) {
	cookie, err := r.Cookie(s.config.CookieName)
	if err != nil || cookie.Value == "" {
		return nil, false
	}
	session, err := s.ValidateSession(cookie.Value, r)
	if err != nil {
		return nil, false
	}
	var user models.User
	if err := s.db.Get(&user, `SELECT * FROM users WHERE id = ? AND is_active = true`, session.UserID); err != nil {
		return nil, false
	}
	return &user, true
}

// CookieSecure reports whether cookies the app sets are marked Secure.
func (s *Service) CookieSecure() bool { return s.config.CookieSecure }

// ValidateSession checks if the session behind a cookie token is valid and
// refreshes it. The token is looked up by its hash (HashSessionToken).
func (s *Service) ValidateSession(token string, r *http.Request) (*models.Session, error) {
	if token == "" {
		return nil, ErrSessionInvalid
	}
	var session models.Session
	err := s.db.Get(&session, `
		SELECT * FROM sessions
		WHERE token_hash = ? AND is_valid = true
	`, HashSessionToken(token))
	if err != nil {
		return nil, ErrSessionInvalid
	}

	// Check if session has expired (sliding window)
	if time.Now().After(session.ExpiresAt) {
		s.invalidateSessionByID(session.ID)
		return nil, ErrSessionExpired
	}

	// Enforce the absolute lifetime cap so a continuously-refreshed session
	// cannot live forever (a stolen token in active use must eventually die).
	if s.config.AbsoluteSessionDuration > 0 {
		absoluteExpiry := session.CreatedAt.Add(s.config.AbsoluteSessionDuration)
		if time.Now().After(absoluteExpiry) {
			s.invalidateSessionByID(session.ID)
			return nil, ErrSessionExpired
		}
	}

	// Update last activity time
	if _, err := s.db.Exec(`
		UPDATE sessions
		SET last_activity_at = ?,
		    expires_at = ?
		WHERE id = ?
	`, time.Now(), time.Now().Add(s.config.SessionDuration), session.ID); err != nil {
		log.Printf("Failed to update activity of session #%d: %v", session.ID, err)
	}

	return &session, nil
}

// InvalidateSession marks the session behind a cookie token invalid (logout).
func (s *Service) InvalidateSession(token string) error {
	_, err := s.db.Exec(`
		UPDATE sessions
		SET is_valid = false
		WHERE token_hash = ?
	`, HashSessionToken(token))
	return err
}

// invalidateSessionByID marks session id invalid.
func (s *Service) invalidateSessionByID(id int64) {
	if _, err := s.db.Exec(`UPDATE sessions SET is_valid = false WHERE id = ?`, id); err != nil {
		log.Printf("Failed to invalidate session #%d: %v", id, err)
	}
}

// SetSessionCookie sets the session cookie in the response. session must be
// one createSession has just made (only it carries the token).
func (s *Service) SetSessionCookie(w http.ResponseWriter, session *models.Session) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.config.CookieName,
		Value:    session.Token,
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

// recordLoginAttempt records a login attempt. The address stored is the
// throttle key (ThrottleKey: the IPv4 address, or the /64 of an IPv6 one).
func (s *Service) recordLoginAttempt(username string, r *http.Request, success bool) {
	attempt := &models.LoginAttempt{
		Username:  username,
		IP:        ThrottleKey(r),
		UserAgent: r.UserAgent(),
		Success:   success,
		CreatedAt: time.Now(),
	}

	if err := s.loginAttemptRepository.Create(attempt); err != nil {
		log.Printf("Failed to record login attempt: %v", err)
	}
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
