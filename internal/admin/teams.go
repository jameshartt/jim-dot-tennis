package admin

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"jim-dot-tennis/internal/models"
)

// TeamsHandler handles team-related requests
type TeamsHandler struct {
	service     *Service
	templateDir string
}

// NewTeamsHandler creates a new teams handler
func NewTeamsHandler(service *Service, templateDir string) *TeamsHandler {
	return &TeamsHandler{
		service:     service,
		templateDir: templateDir,
	}
}

// HandleTeams handles team management routes
func (h *TeamsHandler) HandleTeams(w http.ResponseWriter, r *http.Request) {
	log.Printf("Admin teams handler called with path: %s, method: %s", r.URL.Path, r.Method)

	// Check if this is a specific team detail request
	if strings.Contains(r.URL.Path, "/teams/") && r.URL.Path != "/admin/teams/" {
		h.handleTeamDetail(w, r)
		return
	}

	// Get user from context
	user, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleTeamsGet(w, r, user)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleTeamsGet handles GET requests for team management
func (h *TeamsHandler) handleTeamsGet(w http.ResponseWriter, r *http.Request, user *models.User) {
	// Get St. Ann's teams with related data
	club, teams, err := h.service.GetStAnnsTeams()
	if err != nil {
		logAndError(w, "Failed to load teams", err, http.StatusInternalServerError)
		return
	}

	// Load the teams template
	tmpl, err := parseTemplate(h.templateDir, "admin/teams.html")
	if err != nil {
		log.Printf("Error parsing teams template: %v", err)
		// Fallback to simple HTML response
		renderFallbackHTML(w, "Admin - Teams", "Team Management",
			"Team management page - coming soon", "/admin")
		return
	}

	// Execute the template with data
	if err := renderTemplate(w, tmpl, map[string]interface{}{
		"User":  user,
		"Club":  club,
		"Teams": teams,
	}); err != nil {
		logAndError(w, err.Error(), err, http.StatusInternalServerError)
	}
}

// handleTeamDetail handles requests for individual team details
func (h *TeamsHandler) handleTeamDetail(w http.ResponseWriter, r *http.Request) {
	log.Printf("Admin team detail handler called with path: %s, method: %s", r.URL.Path, r.Method)

	// Check for add captain action
	if strings.HasSuffix(r.URL.Path, "/add-captain") {
		h.handleAddCaptain(w, r)
		return
	}

	// Get user from context
	user, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	// Extract team ID from URL path
	teamID, err := parseIDFromPath(r.URL.Path, "/admin/teams/")
	if err != nil {
		logAndError(w, "Invalid team ID", err, http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleTeamDetailGet(w, r, user, teamID)
	case http.MethodPost:
		h.handleTeamDetailPost(w, r, user, teamID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleTeamDetailGet handles GET requests to show the team detail page
func (h *TeamsHandler) handleTeamDetailGet(w http.ResponseWriter, r *http.Request, user *models.User, teamID uint) {
	// Get the team with full details
	teamDetail, err := h.service.GetTeamDetail(teamID)
	if err != nil {
		logAndError(w, "Team not found", err, http.StatusNotFound)
		return
	}

	// Get available players for captain selection
	availablePlayers, err := h.service.GetAvailablePlayersForCaptain(teamID)
	if err != nil {
		log.Printf("Failed to get available players for captain: %v", err)
		// Continue without available players
		availablePlayers = []models.Player{}
	}

	// Load the team detail template
	tmpl, err := parseTemplate(h.templateDir, "admin/team_detail.html")
	if err != nil {
		log.Printf("Error parsing team detail template: %v", err)
		// Fallback to simple HTML response
		renderFallbackHTML(w, "Team Detail", "Team Detail",
			"Team detail page - coming soon", "/admin/teams")
		return
	}

	// Execute the template with data
	if err := renderTemplate(w, tmpl, map[string]interface{}{
		"User":             user,
		"TeamDetail":       teamDetail,
		"AvailablePlayers": availablePlayers,
	}); err != nil {
		logAndError(w, err.Error(), err, http.StatusInternalServerError)
	}
}

// handleTeamDetailPost handles POST requests to update team details
func (h *TeamsHandler) handleTeamDetailPost(w http.ResponseWriter, r *http.Request, user *models.User, teamID uint) {
	// TODO: Implement team detail updates (adding/removing players, etc.)
	http.Error(w, "Team detail updates not yet implemented", http.StatusNotImplemented)
}

// handleAddCaptain handles the add captain functionality
func (h *TeamsHandler) handleAddCaptain(w http.ResponseWriter, r *http.Request) {
	log.Printf("Admin add captain handler called with path: %s, method: %s", r.URL.Path, r.Method)

	// Get user from context
	user, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	// Extract team ID from URL path
	// Path format: /admin/teams/{id}/add-captain
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/admin/teams/"), "/")
	if len(pathParts) < 2 || pathParts[0] == "" || pathParts[1] != "add-captain" {
		http.Error(w, "Invalid add captain URL", http.StatusBadRequest)
		return
	}

	teamIDStr := pathParts[0]
	teamID, err := strconv.ParseUint(teamIDStr, 10, 32)
	if err != nil {
		logAndError(w, "Invalid team ID", err, http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPost:
		h.handleAddCaptainPost(w, r, user, uint(teamID))
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAddCaptainPost processes the form submission to add a captain
func (h *TeamsHandler) handleAddCaptainPost(w http.ResponseWriter, r *http.Request, user *models.User, teamID uint) {
	// Parse form data
	if err := r.ParseForm(); err != nil {
		logAndError(w, "Invalid form data", err, http.StatusBadRequest)
		return
	}

	// Get form values
	playerID := strings.TrimSpace(r.FormValue("player_id"))
	roleStr := strings.TrimSpace(r.FormValue("role"))

	// Validate required fields
	if playerID == "" {
		http.Error(w, "Player ID is required", http.StatusBadRequest)
		return
	}

	if roleStr == "" {
		http.Error(w, "Role is required", http.StatusBadRequest)
		return
	}

	// Convert role string to CaptainRole
	var role models.CaptainRole
	switch roleStr {
	case "Team":
		role = models.TeamCaptain
	case "Day":
		role = models.DayCaptain
	default:
		http.Error(w, "Invalid role specified", http.StatusBadRequest)
		return
	}

	// Add the captain
	if err := h.service.AddTeamCaptain(teamID, playerID, role); err != nil {
		log.Printf("Failed to add captain: %v", err)
		http.Error(w, fmt.Sprintf("Failed to add captain: %v", err), http.StatusInternalServerError)
		return
	}

	// Redirect back to team detail page
	http.Redirect(w, r, fmt.Sprintf("/admin/teams/%d", teamID), http.StatusSeeOther)
}
