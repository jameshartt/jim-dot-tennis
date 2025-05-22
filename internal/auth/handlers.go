package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"jim-dot-tennis/internal/models"
)

const (
	// Cookie durations
	playerSessionDuration = 180 * 24 * time.Hour // ~6 months (April to September)
	userSessionDuration   = 180 * 24 * time.Hour // Same duration for consistency
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

	// User authentication routes
	mux.HandleFunc("/auth/login", h.handleLogin)
	mux.HandleFunc("/auth/logout", h.handleLogout)
	mux.HandleFunc("/auth/user", h.handleCreateUser) // Admin only

	// Player association routes
	mux.HandleFunc("/auth/user/associate", h.handleAssociatePlayer)       // Admin only
	mux.HandleFunc("/auth/user/disassociate", h.handleDisassociatePlayer) // Admin only
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
		MaxAge:   int(playerSessionDuration.Seconds()),
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
		PlayerID string   `json:"player_id"`
		ProNames []string `json:"pro_names"`
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

	// Return the token in the response
	json.NewEncoder(w).Encode(map[string]string{
		"token": token.Token,
	})
}

// handleLogin handles user login
func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	log.Printf("Login attempt from IP: %s, User-Agent: %s", r.RemoteAddr, r.UserAgent())

	if r.Method != http.MethodPost {
		log.Printf("Invalid method %s for login attempt", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Failed to decode login request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("Login attempt for username: %s", req.Username)

	user, err := h.service.AuthenticateUser(req.Username, req.Password, r)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidCredentials):
			log.Printf("Invalid credentials for username: %s", req.Username)
			http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		case errors.Is(err, ErrUserInactive):
			log.Printf("Inactive account attempt for username: %s", req.Username)
			http.Error(w, "Account is inactive", http.StatusUnauthorized)
		case errors.Is(err, ErrSuspiciousAccess):
			log.Printf("Suspicious access attempt for username: %s from IP: %s", req.Username, r.RemoteAddr)
			http.Error(w, "Access denied", http.StatusTooManyRequests)
		default:
			log.Printf("Internal error during login for username %s: %v", req.Username, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	// Set session cookie for user access
	cookie := &http.Cookie{
		Name:     "auth_token",
		Value:    fmt.Sprintf("%d", user.ID),
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(userSessionDuration.Seconds()),
	}
	http.SetCookie(w, cookie)
	log.Printf("Successful login for username: %s (ID: %d, Role: %s)", user.Username, user.ID, user.Role)

	// Return user info
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"username": user.Username,
		"role":     user.Role,
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding login response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	log.Printf("Login response sent successfully for username: %s", user.Username)
}

// handleLogout handles user logout
func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Clear both auth cookies
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "player_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})

	w.WriteHeader(http.StatusOK)
}

// handleCreateUser handles user creation (admin only)
func (h *Handler) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string            `json:"username"`
		Password string            `json:"password"`
		Role     models.AccessRole `json:"role"`
		PlayerID *string           `json:"player_id,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	user, err := h.service.CreateUser(req.Username, req.Password, req.Role, req.PlayerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Return user info (without password)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":        user.ID,
		"username":  user.Username,
		"role":      user.Role,
		"player_id": user.PlayerID,
	})
}

// handleAssociatePlayer handles associating a player with a user account (admin only)
func (h *Handler) handleAssociatePlayer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UserID   int64  `json:"user_id"`
		PlayerID string `json:"player_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.service.AssociatePlayerWithUser(req.UserID, req.PlayerID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// handleDisassociatePlayer handles removing a player association from a user account (admin only)
func (h *Handler) handleDisassociatePlayer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UserID int64 `json:"user_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.service.DisassociatePlayerFromUser(req.UserID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}
