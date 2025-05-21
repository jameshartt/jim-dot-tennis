package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"jim-dot-tennis/internal/models"
)

// Handler handles HTTP requests for authentication
type Handler struct {
	service *Service
}

// NewHandler creates a new auth handler
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers the auth routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Player access routes
	mux.HandleFunc("/auth/player/access", h.handlePlayerAccess)
	mux.HandleFunc("/auth/player/token", h.handleCreatePlayerToken)

	// Magic link routes
	mux.HandleFunc("/auth/magic/request", h.handleRequestMagicLink)
	mux.HandleFunc("/auth/magic/validate", h.handleValidateMagicLink)
}

// handlePlayerAccess handles player access token validation
func (h *Handler) handlePlayerAccess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Token is required", http.StatusBadRequest)
		return
	}

	playerToken, err := h.service.ValidatePlayerAccess(token, r)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidToken):
			http.Error(w, "Invalid token", http.StatusUnauthorized)
		case errors.Is(err, ErrSuspiciousAccess):
			http.Error(w, "Access denied", http.StatusTooManyRequests)
		default:
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	// Set session cookie for player access
	http.SetCookie(w, &http.Cookie{
		Name:     "player_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(30 * 24 * time.Hour.Seconds()), // 30 days
	})

	// Return player info
	json.NewEncoder(w).Encode(map[string]interface{}{
		"player_id": playerToken.PlayerID,
		"role":      models.RolePlayer,
	})
}

// handleCreatePlayerToken handles creation of player access tokens
// This should be an admin-only endpoint
func (h *Handler) handleCreatePlayerToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		PlayerID  string   `json:"player_id"`
		ProNames  []string `json:"pro_names"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	token, err := h.service.CreatePlayerAccessToken(req.PlayerID, req.ProNames)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"token": token.Token,
		"url":   "/auth/player/access?token=" + token.Token,
	})
}

// handleRequestMagicLink handles requests for magic links
func (h *Handler) handleRequestMagicLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Email string        `json:"email"`
		Role  models.AccessRole `json:"role"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	link, err := h.service.CreateMagicLink(req.Email, req.Role)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// TODO: In production, send the magic link via email
	// For now, just return the token (this should be removed in production)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Magic link sent to email",
		"token":   link.Token, // Remove this in production
		"url":     "/auth/magic/validate?token=" + link.Token, // Remove this in production
	})
}

// handleValidateMagicLink handles magic link validation
func (h *Handler) handleValidateMagicLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Token is required", http.StatusBadRequest)
		return
	}

	link, err := h.service.ValidateMagicLink(token, r)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidToken):
			http.Error(w, "Invalid token", http.StatusUnauthorized)
		case errors.Is(err, ErrTokenExpired):
			http.Error(w, "Token expired", http.StatusUnauthorized)
		case errors.Is(err, ErrSuspiciousAccess):
			http.Error(w, "Access denied", http.StatusTooManyRequests)
		default:
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	// Set session cookie for magic link access
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(24 * time.Hour.Seconds()), // 24 hours
	})

	// Return user info
	json.NewEncoder(w).Encode(map[string]interface{}{
		"email": link.Email,
		"role":  link.Role,
	})
} 