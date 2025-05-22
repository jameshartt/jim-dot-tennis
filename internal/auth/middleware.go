package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"jim-dot-tennis/internal/models"
)

// Context keys
type contextKey string

const (
	PlayerIDKey contextKey = "player_id"
	RoleKey     contextKey = "role"
)

// AuthMiddleware provides authentication middleware
type AuthMiddleware struct {
	service           *Service
	allowInsecureHTTP bool
}

// NewAuthMiddleware creates a new auth middleware
func NewAuthMiddleware(service *Service, allowInsecureHTTP bool) *AuthMiddleware {
	return &AuthMiddleware{service: service, allowInsecureHTTP: allowInsecureHTTP}
}

// RequirePlayerAccess ensures the request has a valid player access token
func (m *AuthMiddleware) RequirePlayerAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get player token from cookie
		cookie, err := r.Cookie("player_token")
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Validate the token
		playerToken, err := m.service.ValidatePlayerAccess(cookie.Value, r)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Add player info to context
		ctx := context.WithValue(r.Context(), PlayerIDKey, playerToken.PlayerID)
		ctx = context.WithValue(ctx, RoleKey, models.RolePlayer)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRole ensures the request has a specific role
func (m *AuthMiddleware) RequireRole(roles ...models.AccessRole) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, ok := r.Context().Value(RoleKey).(models.AccessRole)
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Check if the role is allowed
			allowed := false
			for _, r := range roles {
				if role == r {
					allowed = true
					break
				}
			}

			if !allowed {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GetPlayerID gets the player ID from the context
func GetPlayerID(ctx context.Context) (string, error) {
	playerID, ok := ctx.Value(PlayerIDKey).(string)
	if !ok {
		return "", errors.New("player ID not found in context")
	}
	return playerID, nil
}

// GetRole gets the role from the context
func GetRole(ctx context.Context) (models.AccessRole, error) {
	role, ok := ctx.Value(RoleKey).(models.AccessRole)
	if !ok {
		return "", errors.New("role not found in context")
	}
	return role, nil
}

// RequireHTTPS ensures the request is using HTTPS
func (m *AuthMiddleware) RequireHTTPS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.allowInsecureHTTP {
			next.ServeHTTP(w, r)
			return
		}
		if r.TLS == nil && !strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
			http.Error(w, "HTTPS required", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Example usage in main.go:
/*
func main() {
	// ... setup code ...

	// Create auth middleware
	authMiddleware := auth.NewAuthMiddleware(authService)

	// Public routes
	mux.HandleFunc("/", handleHome)

	// Player routes (protected by player access token)
	playerRoutes := http.NewServeMux()
	playerRoutes.HandleFunc("/player/fixtures", handlePlayerFixtures)
	playerRoutes.HandleFunc("/player/availability", handlePlayerAvailability)
	mux.Handle("/player/", authMiddleware.RequirePlayerAccess(playerRoutes))

	// ... start server ...
}
*/
