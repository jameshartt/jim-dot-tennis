package admin

import (
	"log"
	"net/http"
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
		"User":       user,
		"TeamDetail": teamDetail,
	}); err != nil {
		logAndError(w, err.Error(), err, http.StatusInternalServerError)
	}
}

// handleTeamDetailPost handles POST requests to update team details
func (h *TeamsHandler) handleTeamDetailPost(w http.ResponseWriter, r *http.Request, user *models.User, teamID uint) {
	// TODO: Implement team detail updates (adding/removing players, etc.)
	http.Error(w, "Team detail updates not yet implemented", http.StatusNotImplemented)
}
