// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"

	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
)

// ContextKey is a type for context keys
type ContextKey string

const (
	// UserContextKey is the key for user in request context
	UserContextKey ContextKey = "user"
	// RoleContextKey is the key for role in request context
	RoleContextKey ContextKey = "role"
	// PlayerContextKey is the key for player in request context (for fantasy token auth)
	PlayerContextKey ContextKey = "player"
)

// ChangePasswordPath is the page where a signed-in user who has a password
// chooses a new one.
const ChangePasswordPath = "/account/password"

// Middleware provides authentication middleware
type Middleware struct {
	service          *Service
	playerRepo       repository.PlayerRepository
	fantasyMatchRepo repository.FantasyMixedDoublesRepository
}

// NewMiddleware creates a new auth middleware
func NewMiddleware(service *Service, playerRepo repository.PlayerRepository, fantasyMatchRepo repository.FantasyMixedDoublesRepository) *Middleware {
	return &Middleware{
		service:          service,
		playerRepo:       playerRepo,
		fantasyMatchRepo: fantasyMatchRepo,
	}
}

// RequireAuth middleware ensures the request has a valid session. It logs
// nothing on the normal path (no per-request lines, no usernames).
func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get session cookie
		cookie, err := r.Cookie(m.service.config.CookieName)
		if err != nil {
			// Redirect to login with the original URL as a redirect parameter
			loginURL := loginRedirectURL(r)
			http.Redirect(w, r, loginURL, http.StatusSeeOther)
			return
		}

		// Validate session
		session, err := m.service.ValidateSession(cookie.Value, r)
		if err != nil {
			// Clear cookie if session is invalid or expired
			m.service.ClearSessionCookie(w)
			// Redirect to login with the original URL as a redirect parameter
			loginURL := loginRedirectURL(r)
			http.Redirect(w, r, loginURL, http.StatusSeeOther)
			return
		}

		// Get user details
		var user models.User
		err = m.service.db.Get(&user, `
			SELECT * FROM users
			WHERE id = ? AND is_active = true
		`, session.UserID)
		if err != nil {
			m.service.invalidateSessionByID(session.ID)
			m.service.ClearSessionCookie(w)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		// The user in the context never carries the password hash: handlers
		// pass it to templates. Anything that needs the hash (sign-in,
		// changing one's own password) reads it from the database itself.
		user.PasswordHash = ""

		// Add user, role and the session's (non-secret) id to the context
		ctx := context.WithValue(r.Context(), UserContextKey, user)
		ctx = context.WithValue(ctx, RoleContextKey, user.Role)
		ctx = context.WithValue(ctx, SessionContextKey, session.ID)

		// Call the next handler with the enriched context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRole middleware ensures the user has the required role
func (m *Middleware) RequireRole(requiredRoles ...models.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get role from context
			role, ok := r.Context().Value(RoleContextKey).(models.Role)
			if !ok {
				log.Printf("No role found in context")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Check if user has any of the required roles
			hasRequiredRole := false
			for _, requiredRole := range requiredRoles {
				if role == requiredRole {
					hasRequiredRole = true
					break
				}
			}

			if !hasRequiredRole {
				log.Printf("User with role %s does not have required role", role)
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			// User has required role, proceed
			next.ServeHTTP(w, r)
		})
	}
}

// GetUserFromContext gets the user from the request context. Its
// PasswordHash is always empty (RequireAuth removes it), whether or not the
// account has a password.
func GetUserFromContext(ctx context.Context) (models.User, error) {
	user, ok := ctx.Value(UserContextKey).(models.User)
	if !ok {
		return models.User{}, errors.New("user not found in context")
	}
	return user, nil
}

// GetPlayerFromContext gets the player from the request context (for fantasy token auth)
func GetPlayerFromContext(ctx context.Context) (models.Player, error) {
	player, ok := ctx.Value(PlayerContextKey).(models.Player)
	if !ok {
		return models.Player{}, errors.New("player not found in context")
	}
	return player, nil
}

// RequireFantasyTokenAuth middleware validates fantasy mixed doubles auth tokens from URL
// Expected URL format: /my-availability/Sabalenka_Djokovic_Gauff_Sinner
//
// The token is the player's credential, so neither it nor the request path
// (which contains it) is ever logged; log lines name the fantasy match id.
func (m *Middleware) RequireFantasyTokenAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract auth token from URL path
		authToken := extractFantasyTokenFromPath(r.URL.Path)
		if authToken == "" {
			http.Error(w, "Invalid fantasy auth token in URL", http.StatusBadRequest)
			return
		}

		// Find the fantasy match by auth token
		ctx := r.Context()
		fantasyMatch, err := m.fantasyMatchRepo.FindByAuthToken(ctx, authToken)
		if err != nil {
			http.Error(w, "Invalid fantasy match token", http.StatusNotFound)
			return
		}

		if !fantasyMatch.IsActive {
			http.Error(w, "Fantasy match is not active", http.StatusForbidden)
			return
		}

		// Find the actual player assigned to this fantasy match
		player, err := m.playerRepo.FindByFantasyMatchID(ctx, fantasyMatch.ID)
		if err != nil {
			log.Printf("No player found for fantasy match ID %d: %v", fantasyMatch.ID, err)
			http.Error(w, "No player assigned to this fantasy match", http.StatusNotFound)
			return
		}

		// Add player to request context
		ctx = context.WithValue(ctx, PlayerContextKey, *player)

		// Call the next handler with the enriched context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractFantasyTokenFromPath extracts the fantasy auth token from URL paths
// Expected format: /my-availability/Sabalenka_Djokovic_Gauff_Sinner or /my-profile/Sabalenka_Djokovic_Gauff_Sinner
func extractFantasyTokenFromPath(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if (part == "my-availability" || part == "my-profile") && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// loginRedirectURL is the login page URL that returns the user to the
// requested page (path and query) after signing in.
func loginRedirectURL(r *http.Request) string {
	return "/login?redirect=" + url.QueryEscape(r.URL.RequestURI())
}
