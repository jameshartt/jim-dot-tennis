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
	if strings.Contains(r.URL.Path, "/teams/") && r.URL.Path != "/admin/league/teams/" {
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
	case http.MethodPost:
		h.handleTeamsPost(w, r, user)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleTeamsPost handles POST requests for team management
func (h *TeamsHandler) handleTeamsPost(w http.ResponseWriter, r *http.Request, user *models.User) {
	action := r.FormValue("action")

	if action == "create" {
		h.handleCreateTeam(w, r)
		return
	}

	http.Error(w, "Unknown action", http.StatusBadRequest)
}

// handleCreateTeam handles creating a new team
func (h *TeamsHandler) handleCreateTeam(w http.ResponseWriter, r *http.Request) {
	// Get the active season - teams can only be created for the active season
	activeSeason, err := h.service.GetActiveSeason()
	if err != nil {
		http.Error(w, "Failed to get active season", http.StatusInternalServerError)
		return
	}
	if activeSeason == nil {
		http.Error(w, "No active season found. Please create an active season first.", http.StatusBadRequest)
		return
	}

	// Get form values
	name := strings.TrimSpace(r.FormValue("name"))
	divisionIDStr := r.FormValue("division_id")

	if name == "" {
		http.Error(w, "Team name is required", http.StatusBadRequest)
		return
	}

	divisionID, err := strconv.ParseUint(divisionIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid division ID", http.StatusBadRequest)
		return
	}

	// Get St Ann's club
	clubs, err := h.service.GetClubsByName("St Ann")
	if err != nil || len(clubs) == 0 {
		http.Error(w, "Failed to find St Ann's club", http.StatusInternalServerError)
		return
	}
	stAnnsClub := clubs[0]

	// Create the team
	team := &models.Team{
		Name:       name,
		ClubID:     stAnnsClub.ID,
		DivisionID: uint(divisionID),
		SeasonID:   activeSeason.ID,
	}

	if err := h.service.CreateTeam(team); err != nil {
		logAndError(w, "Failed to create team", err, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/league/teams", http.StatusSeeOther)
}

// handleTeamsGet handles GET requests for team management
func (h *TeamsHandler) handleTeamsGet(w http.ResponseWriter, r *http.Request, user *models.User) {
	// Get St. Ann's teams with related data
	club, teams, err := h.service.GetStAnnsTeams()
	if err != nil {
		logAndError(w, "Failed to load teams", err, http.StatusInternalServerError)
		return
	}

	// Get active season
	activeSeason, err := h.service.GetActiveSeason()
	if err != nil {
		log.Printf("Failed to load active season: %v", err)
	}

	// Get divisions for the active season
	var divisions []models.Division
	var lowestDivisionID uint
	if activeSeason != nil {
		divisions, err = h.service.GetDivisionsBySeason(activeSeason.ID)
		if err != nil {
			log.Printf("Failed to load divisions: %v", err)
		}

		// Find the lowest level division (highest level number)
		for _, div := range divisions {
			if lowestDivisionID == 0 || div.Level > divisions[0].Level {
				lowestDivisionID = div.ID
			}
		}
	}

	// Create division data with IsLowest flag
	type DivisionData struct {
		models.Division
		IsLowest bool
	}
	var divisionData []DivisionData
	for _, div := range divisions {
		divisionData = append(divisionData, DivisionData{
			Division: div,
			IsLowest: div.ID == lowestDivisionID,
		})
	}

	// Load the teams template
	tmpl, err := parseTemplate(h.templateDir, "admin/teams.html")
	if err != nil {
		log.Printf("Error parsing teams template: %v", err)
		// Fallback to simple HTML response
		renderFallbackHTML(w, "Admin - Teams", "Team Management",
			"Team management page - coming soon", "/admin/league")
		return
	}

	// Execute the template with data
	if err := renderTemplate(w, tmpl, map[string]interface{}{
		"User":         user,
		"Club":         club,
		"Teams":        teams,
		"ActiveSeason": activeSeason,
		"Divisions":    divisionData,
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

	// Check for remove captain action
	if strings.HasSuffix(r.URL.Path, "/remove-captain") {
		h.handleRemoveCaptain(w, r)
		return
	}

	// Check for add players action
	if strings.HasSuffix(r.URL.Path, "/add-players") {
		h.handleAddPlayers(w, r)
		return
	}

	// Get user from context
	user, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	// Extract team ID from URL path
	teamID, err := parseIDFromPath(r.URL.Path, "/admin/league/teams/")
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

	// Get upcoming fixtures for the team (limit to next 2 including today)
	upcomingFixtures, err := h.service.GetUpcomingFixturesForTeam(teamID, 2)
	if err != nil {
		log.Printf("Failed to get upcoming fixtures for team: %v", err)
		// Continue without fixtures
		upcomingFixtures = nil
	}

	// Load the team detail template
	tmpl, err := parseTemplate(h.templateDir, "admin/team_detail.html")
	if err != nil {
		log.Printf("Error parsing team detail template: %v", err)
		// Fallback to simple HTML response
		renderFallbackHTML(w, "Team Detail", "Team Detail",
			"Team detail page - coming soon", "/admin/league/teams")
		return
	}

	// Execute the template with data
	if err := renderTemplate(w, tmpl, map[string]interface{}{
		"User":             user,
		"TeamDetail":       teamDetail,
		"AvailablePlayers": availablePlayers,
		"UpcomingFixtures": upcomingFixtures,
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
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/admin/league/teams/"), "/")
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
	http.Redirect(w, r, fmt.Sprintf("/admin/league/teams/%d", teamID), http.StatusSeeOther)
}

// handleRemoveCaptain handles the remove captain functionality
func (h *TeamsHandler) handleRemoveCaptain(w http.ResponseWriter, r *http.Request) {
	log.Printf("Admin remove captain handler called with path: %s, method: %s", r.URL.Path, r.Method)

	// Get user from context
	user, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	// Extract team ID from URL path
	// Path format: /admin/teams/{id}/remove-captain
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/admin/league/teams/"), "/")
	if len(pathParts) < 2 || pathParts[0] == "" || pathParts[1] != "remove-captain" {
		http.Error(w, "Invalid remove captain URL", http.StatusBadRequest)
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
		h.handleRemoveCaptainPost(w, r, user, uint(teamID))
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleRemoveCaptainPost processes the form submission to remove a captain
func (h *TeamsHandler) handleRemoveCaptainPost(w http.ResponseWriter, r *http.Request, user *models.User, teamID uint) {
	// Parse form data
	if err := r.ParseForm(); err != nil {
		logAndError(w, "Invalid form data", err, http.StatusBadRequest)
		return
	}

	// Get form values
	playerID := strings.TrimSpace(r.FormValue("player_id"))

	// Validate required fields
	if playerID == "" {
		http.Error(w, "Player ID is required", http.StatusBadRequest)
		return
	}

	// Remove the captain
	if err := h.service.RemoveTeamCaptain(teamID, playerID); err != nil {
		log.Printf("Failed to remove captain: %v", err)
		http.Error(w, fmt.Sprintf("Failed to remove captain: %v", err), http.StatusInternalServerError)
		return
	}

	// Redirect back to team detail page
	http.Redirect(w, r, fmt.Sprintf("/admin/league/teams/%d", teamID), http.StatusSeeOther)
}

// handleAddPlayers handles the add players functionality
func (h *TeamsHandler) handleAddPlayers(w http.ResponseWriter, r *http.Request) {
	log.Printf("Admin add players handler called with path: %s, method: %s", r.URL.Path, r.Method)

	// Get user from context
	user, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	// Extract team ID from URL path
	// Path format: /admin/teams/{id}/add-players
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/admin/league/teams/"), "/")
	if len(pathParts) < 2 || pathParts[0] == "" || pathParts[1] != "add-players" {
		http.Error(w, "Invalid add players URL", http.StatusBadRequest)
		return
	}

	teamIDStr := pathParts[0]
	teamID, err := strconv.ParseUint(teamIDStr, 10, 32)
	if err != nil {
		logAndError(w, "Invalid team ID", err, http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleAddPlayersGet(w, r, user, uint(teamID))
	case http.MethodPost:
		h.handleAddPlayersPost(w, r, user, uint(teamID))
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAddPlayersGet displays the add players page
func (h *TeamsHandler) handleAddPlayersGet(w http.ResponseWriter, r *http.Request, user *models.User, teamID uint) {
	// Get the team details
	teamDetail, err := h.service.GetTeamDetail(teamID)
	if err != nil {
		logAndError(w, "Team not found", err, http.StatusNotFound)
		return
	}

	// Get filter parameters from URL query
	query := r.URL.Query().Get("q")             // search query
	statusFilter := r.URL.Query().Get("status") // "all", "active", "inactive"

	// Default to showing active players if no filter specified
	if statusFilter == "" {
		statusFilter = "active"
	}

	// Get eligible players for this team
	eligiblePlayers, err := h.service.GetEligiblePlayersForTeam(teamID, query, statusFilter)
	if err != nil {
		logAndError(w, "Failed to load eligible players", err, http.StatusInternalServerError)
		return
	}

	// Check if this is an HTMX request for just the table body
	if r.Header.Get("HX-Request") == "true" {
		h.renderAddPlayersTableBody(w, eligiblePlayers)
		return
	}

	// Load the add players template
	tmpl, err := parseTemplate(h.templateDir, "admin/team_add_players.html")
	if err != nil {
		log.Printf("Error parsing add players template: %v", err)
		// Fallback to simple HTML response
		renderFallbackHTML(w, "Add Players", "Add Players to Team",
			"Add players page - coming soon", fmt.Sprintf("/admin/league/teams/%d", teamID))
		return
	}

	// Execute the template with data
	if err := renderTemplate(w, tmpl, map[string]interface{}{
		"User":            user,
		"TeamDetail":      teamDetail,
		"EligiblePlayers": eligiblePlayers,
		"SearchQuery":     query,
		"StatusFilter":    statusFilter,
	}); err != nil {
		logAndError(w, err.Error(), err, http.StatusInternalServerError)
	}
}

// renderAddPlayersTableBody renders just the table body for HTMX requests
func (h *TeamsHandler) renderAddPlayersTableBody(w http.ResponseWriter, players []models.Player) {
	w.Header().Set("Content-Type", "text/html")

	if len(players) > 0 {
		for _, player := range players {
			// No more active/inactive distinction - all players get the same styling
			activeClass := "player-active"

			w.Write([]byte(fmt.Sprintf(`
				<tr data-player-id="%s" data-player-name="%s %s" class="%s">
					<td class="col-checkbox">
						<input type="checkbox" name="player_ids" value="%s" class="player-checkbox">
					</td>
					<td class="col-name">%s %s</td>
				</tr>
			`, player.ID, player.FirstName, player.LastName, activeClass,
				player.ID,
				player.FirstName, player.LastName)))
		}
	} else {
		w.Write([]byte(`
			<tr>
				<td colspan="4" style="text-align: center; padding: 2rem;">
					No eligible players found matching your criteria.
				</td>
			</tr>
		`))
	}
}

// handleAddPlayersPost processes the form submission to add players to the team
func (h *TeamsHandler) handleAddPlayersPost(w http.ResponseWriter, r *http.Request, user *models.User, teamID uint) {
	// Parse form data
	if err := r.ParseForm(); err != nil {
		logAndError(w, "Invalid form data", err, http.StatusBadRequest)
		return
	}

	// Get selected player IDs from form
	selectedPlayerIDs := r.Form["player_ids"] // This will be an array of player IDs

	// Validate that at least one player was selected
	if len(selectedPlayerIDs) == 0 {
		http.Error(w, "No players selected", http.StatusBadRequest)
		return
	}

	// Add the players to the team
	if err := h.service.AddPlayersToTeam(teamID, selectedPlayerIDs); err != nil {
		log.Printf("Failed to add players to team: %v", err)
		http.Error(w, fmt.Sprintf("Failed to add players: %v", err), http.StatusInternalServerError)
		return
	}

	// Redirect back to team detail page with success message
	http.Redirect(w, r, fmt.Sprintf("/admin/league/teams/%d?success=players_added", teamID), http.StatusSeeOther)
}
