package players

import (
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
	// Expected format: /my-availability/Sabalenka_Alcaraz_Guaff_Sinner
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/my-availability/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		http.Error(w, "Invalid availability URL - missing auth token", http.StatusBadRequest)
		return
	}
	authToken := pathParts[0]

	// Get player from fantasy token context (set by RequireFantasyTokenAuth middleware)
	player, err := auth.GetPlayerFromContext(r.Context())
	if err != nil {
		logAndError(w, "Player not found in context", err, http.StatusUnauthorized)
		return
	}

	log.Printf("Authenticated player: %s %s (ID: %s) for auth token: %s",
		player.FirstName, player.LastName, player.ID, authToken)

	switch r.Method {
	case http.MethodGet:
		h.handleAvailabilityGet(w, r, &player, authToken)
	case http.MethodPost:
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
