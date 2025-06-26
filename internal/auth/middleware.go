package auth

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
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

// RequireAuth middleware ensures the request has a valid session
func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("RequireAuth middleware triggered for path: %s", r.URL.Path)

		// Get session cookie
		cookie, err := r.Cookie(m.service.config.CookieName)
		if err != nil {
			log.Printf("No session cookie found: %v", err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		log.Printf("Found session cookie: %s", cookie.Value)

		// Validate session
		session, err := m.service.ValidateSession(cookie.Value, r)
		if err != nil {
			log.Printf("Invalid session: %v", err)

			// Clear cookie if session is invalid or expired
			m.service.ClearSessionCookie(w)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		log.Printf("Session validated: %s (User ID: %d, Role: %s)", session.ID, session.UserID, session.Role)

		// Get user details
		var user models.User
		err = m.service.db.Get(&user, `
			SELECT * FROM users 
			WHERE id = ? AND is_active = true
		`, session.UserID)
		if err != nil {
			log.Printf("User not found or inactive: %v", err)
			m.service.InvalidateSession(session.ID)
			m.service.ClearSessionCookie(w)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		log.Printf("User details retrieved: ID=%d, Username=%s, Role=%s", user.ID, user.Username, user.Role)

		// Add user and role to request context
		ctx := context.WithValue(r.Context(), UserContextKey, user)
		ctx = context.WithValue(ctx, RoleContextKey, user.Role)
		log.Printf("Added user and role to request context")

		// Call the next handler with the enriched context
		log.Printf("Proceeding to next handler, %s", r.URL.Path)
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

// GetUserFromContext gets the user from the request context
func GetUserFromContext(ctx context.Context) (models.User, error) {
	user, ok := ctx.Value(UserContextKey).(models.User)
	if !ok {
		return models.User{}, errors.New("user not found in context")
	}
	return user, nil
}

// GetRoleFromContext gets the role from the request context
func GetRoleFromContext(ctx context.Context) (models.Role, error) {
	role, ok := ctx.Value(RoleContextKey).(models.Role)
	if !ok {
		return "", errors.New("role not found in context")
	}
	return role, nil
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
func (m *Middleware) RequireFantasyTokenAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("FantasyTokenAuth middleware for path: %s", r.URL.Path)

		// Extract auth token from URL path
		authToken := extractFantasyTokenFromPath(r.URL.Path)
		if authToken == "" {
			log.Printf("No fantasy auth token found in URL path: %s", r.URL.Path)
			http.Error(w, "Invalid fantasy auth token in URL", http.StatusBadRequest)
			return
		}
		log.Printf("Extracted fantasy auth token: %s", authToken)

		// Find the fantasy match by auth token
		ctx := r.Context()
		fantasyMatch, err := m.fantasyMatchRepo.FindByAuthToken(ctx, authToken)
		if err != nil {
			log.Printf("Fantasy match not found for token %s: %v", authToken, err)
			http.Error(w, "Invalid fantasy match token", http.StatusNotFound)
			return
		}

		if !fantasyMatch.IsActive {
			log.Printf("Fantasy match is inactive for token %s", authToken)
			http.Error(w, "Fantasy match is not active", http.StatusForbidden)
			return
		}
		log.Printf("Found active fantasy match ID: %d", fantasyMatch.ID)

		// Create a virtual player context based on the fantasy match
		// This allows any user to access the availability system using the fantasy token
		player := &models.Player{
			ID:        fmt.Sprintf("fantasy_%d", fantasyMatch.ID), // Virtual player ID
			FirstName: "Fantasy",
			LastName:  "Player",
			Email:     fmt.Sprintf("fantasy_%d@example.com", fantasyMatch.ID),
		}
		log.Printf("Created virtual player context: %s %s (ID: %s)", player.FirstName, player.LastName, player.ID)

		// Add player to request context
		ctx = context.WithValue(ctx, PlayerContextKey, *player)
		log.Printf("Added player to request context")

		// Call the next handler with the enriched context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractFantasyTokenFromPath extracts the fantasy auth token from URL paths
// Expected format: /my-availability/Sabalenka_Djokovic_Gauff_Sinner
func extractFantasyTokenFromPath(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if part == "my-availability" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}
