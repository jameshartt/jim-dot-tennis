package auth

import (
	"context"
	"errors"
	"log"
	"net/http"

	"jim-dot-tennis/internal/models"
)

// ContextKey is a type for context keys
type ContextKey string

const (
	// UserContextKey is the key for user in request context
	UserContextKey ContextKey = "user"
	// RoleContextKey is the key for role in request context
	RoleContextKey ContextKey = "role"
)

// Middleware provides authentication middleware
type Middleware struct {
	service *Service
}

// NewMiddleware creates a new auth middleware
func NewMiddleware(service *Service) *Middleware {
	return &Middleware{service: service}
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
		log.Printf("Proceeding to next handler")
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
