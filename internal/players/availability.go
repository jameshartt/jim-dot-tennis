package players

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/models"
)

// AvailabilityHandler handles player availability requests
type AvailabilityHandler struct {
	service     *Service
	templateDir string
}

// NewAvailabilityHandler creates a new availability handler
func NewAvailabilityHandler(service *Service, templateDir string) *AvailabilityHandler {
	return &AvailabilityHandler{
		service:     service,
		templateDir: templateDir,
	}
}

// HandleAvailability handles fantasy mixed doubles availability routes
func (h *AvailabilityHandler) HandleAvailability(w http.ResponseWriter, r *http.Request) {
	log.Printf("Player availability handler called with path: %s, method: %s", r.URL.Path, r.Method)

	// Extract auth token from URL path
	// Expected format: /my-availability/Sabalenka_Alcaraz_Guaff_Sinner[/action]
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/my-availability/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		http.Error(w, "Invalid availability URL - missing auth token", http.StatusBadRequest)
		return
	}
	authToken := pathParts[0]
	action := ""
	if len(pathParts) > 1 {
		action = pathParts[1]
	}

	// Get player from fantasy token context (set by RequireFantasyTokenAuth middleware)
	player, err := auth.GetPlayerFromContext(r.Context())
	if err != nil {
		logAndError(w, "Player not found in context", err, http.StatusUnauthorized)
		return
	}

	log.Printf("Authenticated player: %s %s (ID: %s) for auth token: %s, action: %s",
		player.FirstName, player.LastName, player.ID, authToken, action)

	// Route based on action and method
	switch {
	case action == "data" && r.Method == http.MethodGet:
		h.handleAvailabilityDataGet(w, r, &player, authToken)
	case action == "update" && r.Method == http.MethodPost:
		h.handleAvailabilityUpdate(w, r, &player, authToken)
	case action == "batch-update" && r.Method == http.MethodPost:
		h.handleAvailabilityBatchUpdate(w, r, &player, authToken)
	case action == "request-preferred-name" && r.Method == http.MethodPost:
		h.handlePreferredNameRequest(w, r, &player, authToken)
	case action == "wrapped-auth" && r.Method == http.MethodPost:
		h.handleWrappedPasswordAuth(w, r)
	case action == "" && r.Method == http.MethodGet:
		h.handleAvailabilityGet(w, r, &player, authToken)
	case action == "" && r.Method == http.MethodPost:
		h.handleAvailabilityPost(w, r, &player, authToken)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAvailabilityGet displays the availability page for a fantasy match
func (h *AvailabilityHandler) handleAvailabilityGet(w http.ResponseWriter, r *http.Request, player *models.Player, authToken string) {
	// Get the fantasy match details
	fantasyMatch, err := h.service.GetFantasyMatchByToken(authToken)
	if err != nil {
		log.Printf("Fantasy match not found for token %s: %v", authToken, err)
		http.Error(w, "Fantasy match not found", http.StatusNotFound)
		return
	}

	// Load the availability template
	tmpl, err := parseTemplate(h.templateDir, "players/availability.html")
	if err != nil {
		log.Printf("Error parsing availability template: %v", err)
		// Fallback to simple HTML response
		renderFallbackHTML(w, "Player Availability", "My Availability",
			fmt.Sprintf("Availability page for fantasy match: %s vs %s",
				getTeamName(fantasyMatch.TeamAWoman, fantasyMatch.TeamAMan),
				getTeamName(fantasyMatch.TeamBWoman, fantasyMatch.TeamBMan)),
			"/")
		return
	}

	// Execute the template with data
	if err := renderTemplate(w, tmpl, map[string]interface{}{
		"Player":       player,
		"FantasyMatch": fantasyMatch,
		"AuthToken":    authToken,
		"TeamAName":    getTeamName(fantasyMatch.TeamAWoman, fantasyMatch.TeamAMan),
		"TeamBName":    getTeamName(fantasyMatch.TeamBWoman, fantasyMatch.TeamBMan),
	}); err != nil {
		logAndError(w, err.Error(), err, http.StatusInternalServerError)
	}
}

// handleAvailabilityPost handles POST requests for updating availability
func (h *AvailabilityHandler) handleAvailabilityPost(w http.ResponseWriter, r *http.Request, player *models.Player, authToken string) {
	// TODO: Implement availability updates
	// This would involve updating player availability records for specific fixtures/dates
	http.Error(w, "Availability updates not yet implemented", http.StatusNotImplemented)
}

// handleAvailabilityDataGet returns availability data as JSON
func (h *AvailabilityHandler) handleAvailabilityDataGet(w http.ResponseWriter, r *http.Request, player *models.Player, authToken string) {
	availabilityData, err := h.service.GetPlayerAvailabilityData(player.ID)
	if err != nil {
		logAndError(w, "Failed to get availability data", err, http.StatusInternalServerError)
		return
	}

	// Update with actual player data
	availabilityData.Player.FirstName = player.FirstName
	availabilityData.Player.LastName = player.LastName

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(availabilityData); err != nil {
		logAndError(w, "Failed to encode availability data", err, http.StatusInternalServerError)
	}
}

// handleAvailabilityUpdate handles single availability updates
func (h *AvailabilityHandler) handleAvailabilityUpdate(w http.ResponseWriter, r *http.Request, player *models.Player, authToken string) {
	var updateReq struct {
		Date   string `json:"date"`
		Status string `json:"status"`
	}

	if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	log.Printf("Updating availability for player %s: date=%s, status=%s",
		player.ID, updateReq.Date, updateReq.Status)

	if err := h.service.UpdatePlayerAvailability(player.ID, updateReq.Date, updateReq.Status); err != nil {
		logAndError(w, "Failed to update availability", err, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// handleAvailabilityBatchUpdate handles multiple availability updates
func (h *AvailabilityHandler) handleAvailabilityBatchUpdate(w http.ResponseWriter, r *http.Request, player *models.Player, authToken string) {
	var batchReq struct {
		Updates []AvailabilityUpdateRequest `json:"updates"`
	}

	if err := json.NewDecoder(r.Body).Decode(&batchReq); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	log.Printf("Batch updating availability for player %s: %d updates",
		player.ID, len(batchReq.Updates))

	if err := h.service.BatchUpdatePlayerAvailability(player.ID, batchReq.Updates); err != nil {
		logAndError(w, "Failed to batch update availability", err, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// handlePreferredNameRequest handles preferred name requests from players
func (h *AvailabilityHandler) handlePreferredNameRequest(w http.ResponseWriter, r *http.Request, player *models.Player, authToken string) {
	var req struct {
		PreferredName string `json:"preferredName"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate the preferred name
	preferredName := strings.TrimSpace(req.PreferredName)
	if preferredName == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Preferred name cannot be empty",
		})
		return
	}

	if len(preferredName) < 2 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Preferred name must be at least 2 characters long",
		})
		return
	}

	if len(preferredName) > 50 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Preferred name must be 50 characters or less",
		})
		return
	}

	log.Printf("Processing preferred name request from player %s (%s): '%s'",
		player.ID, player.FirstName+" "+player.LastName, preferredName)

	// Submit the preferred name request
	if err := h.service.RequestPreferredName(player.ID, preferredName); err != nil {
		log.Printf("Failed to submit preferred name request for player %s: %v", player.ID, err)

		// Check if it's a uniqueness error
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "pending approval") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "This preferred name is already taken or pending approval",
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Failed to submit preferred name request",
		})
		return
	}

	log.Printf("Successfully submitted preferred name request for player %s: '%s'", player.ID, preferredName)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Your preferred name request for '%s' has been submitted for admin approval", preferredName),
	})
}

// handleWrappedPasswordAuth validates a shared password and sets a short-lived cookie to view club wrapped
func (h *AvailabilityHandler) handleWrappedPasswordAuth(w http.ResponseWriter, r *http.Request) {
	// Expect JSON: { "password": "..." }
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Simple in-code secret. Replace via build-time env in future if desired.
	// Intentionally low-friction deterrent, not robust security.
	const expected = "st-anns-2025"
	if strings.TrimSpace(req.Password) == expected {
		// Set short-lived access cookie (15 minutes)
		http.SetCookie(w, &http.Cookie{
			Name:     "wrapped_access",
			Value:    "granted",
			Path:     "/",
			HttpOnly: true,
			MaxAge:   15 * 60,
			SameSite: http.SameSiteLaxMode,
		})
		// Also set a non-HttpOnly cookie with player id to personalize wrapped view
		if player, err := auth.GetPlayerFromContext(r.Context()); err == nil {
			http.SetCookie(w, &http.Cookie{
				Name:     "wrapped_player_id",
				Value:    player.ID,
				Path:     "/",
				HttpOnly: false,
				MaxAge:   15 * 60,
				SameSite: http.SameSiteLaxMode,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Incorrect password"})
}

// getTeamName creates a formatted team name from two tennis players
func getTeamName(woman, man models.ProTennisPlayer) string {
	return fmt.Sprintf("%s & %s", woman.LastName, man.LastName)
}

// Helper functions (similar to admin helpers)

// getUserFromContext extracts user from request context
func getUserFromContext(r *http.Request) (*models.User, error) {
	user, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// logAndError logs an error and sends HTTP error response
func logAndError(w http.ResponseWriter, message string, err error, statusCode int) {
	log.Printf("%s: %v", message, err)
	http.Error(w, message, statusCode)
}
