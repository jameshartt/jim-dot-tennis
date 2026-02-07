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

	// Group teams by season, ordered with active season first then by year descending
	type SeasonTeams struct {
		Season *models.Season
		Teams  []TeamWithRelations
	}
	seasonOrder := []uint{}
	seasonMap := map[uint]*SeasonTeams{}
	for _, team := range teams {
		sid := team.SeasonID
		if _, exists := seasonMap[sid]; !exists {
			seasonMap[sid] = &SeasonTeams{Season: team.Season}
			seasonOrder = append(seasonOrder, sid)
		}
		seasonMap[sid].Teams = append(seasonMap[sid].Teams, team)
	}
	// Sort: active season first, then by year descending
	var groupedTeams []SeasonTeams
	// Add active season first if present
	if activeSeason != nil {
		if st, ok := seasonMap[activeSeason.ID]; ok {
			groupedTeams = append(groupedTeams, *st)
		}
	}
	// Add remaining seasons sorted by year descending
	for _, sid := range seasonOrder {
		if activeSeason != nil && sid == activeSeason.ID {
			continue
		}
		groupedTeams = append(groupedTeams, *seasonMap[sid])
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
		"GroupedTeams": groupedTeams,
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

	// Guard: prevent adding captains to away teams
	if team, err := h.service.GetTeamByID(uint(teamID)); err == nil {
		if !h.service.IsStAnnsClub(team.ClubID) {
			http.Error(w, "Cannot add captains to away teams", http.StatusForbidden)
			return
		}
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

	// Guard: prevent adding players to away teams
	if team, err := h.service.GetTeamByID(uint(teamID)); err == nil {
		if !h.service.IsStAnnsClub(team.ClubID) {
			http.Error(w, "Cannot add players to away teams", http.StatusForbidden)
			return
		}
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

// HandleToggleActive handles toggling a team's active/inactive status
func (h *TeamsHandler) HandleToggleActive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	teamID, err := parseIDFromPath(r.URL.Path, "/admin/league/teams/toggle-active/")
	if err != nil {
		logAndError(w, "Invalid team ID", err, http.StatusBadRequest)
		return
	}

	team, err := h.service.GetTeamByID(teamID)
	if err != nil {
		logAndError(w, "Team not found", err, http.StatusNotFound)
		return
	}

	team.Active = !team.Active
	if err := h.service.UpdateTeam(team); err != nil {
		logAndError(w, "Failed to toggle team status", err, http.StatusInternalServerError)
		return
	}

	// Redirect back to referring page
	referer := r.Header.Get("Referer")
	if referer == "" {
		referer = "/admin/league/teams/away"
	}
	http.Redirect(w, r, referer, http.StatusSeeOther)
}

// HandleDivisionReview handles the bulk division review page
func (h *TeamsHandler) HandleDivisionReview(w http.ResponseWriter, r *http.Request) {
	user, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleDivisionReviewGet(w, r, user)
	case http.MethodPost:
		h.handleDivisionReviewPost(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleDivisionReviewGet shows the division review page
func (h *TeamsHandler) handleDivisionReviewGet(w http.ResponseWriter, r *http.Request, user *models.User) {
	activeSeason, _ := h.service.GetActiveSeason()
	if activeSeason == nil {
		http.Error(w, "No active season", http.StatusBadRequest)
		return
	}

	setupData, err := h.service.GetSeasonSetupData(activeSeason.ID)
	if err != nil {
		logAndError(w, "Failed to load division data", err, http.StatusInternalServerError)
		return
	}

	divisions, _ := h.service.GetDivisionsBySeason(activeSeason.ID)

	successMsg := ""
	if r.URL.Query().Get("success") == "updated" {
		successMsg = "Division assignments updated successfully."
	}

	tmpl, err := parseTemplate(h.templateDir, "admin/division_review.html")
	if err != nil {
		log.Printf("Error parsing division review template: %v", err)
		renderFallbackHTML(w, "Division Review", "Division Review",
			"Division review page", "/admin/league/teams/away")
		return
	}

	if err := renderTemplate(w, tmpl, map[string]interface{}{
		"User":         user,
		"Season":       activeSeason,
		"SetupData":    setupData,
		"Divisions":    divisions,
		"Success":      successMsg,
	}); err != nil {
		logAndError(w, err.Error(), err, http.StatusInternalServerError)
	}
}

// handleDivisionReviewPost processes bulk division reassignment
func (h *TeamsHandler) handleDivisionReviewPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		logAndError(w, "Failed to parse form data", err, http.StatusBadRequest)
		return
	}

	// Process division changes: form fields are like "team_123_division" = "5"
	for key, values := range r.Form {
		if !strings.HasPrefix(key, "team_") || !strings.HasSuffix(key, "_division") {
			continue
		}
		if len(values) == 0 {
			continue
		}

		// Extract team ID from key
		teamIDStr := strings.TrimPrefix(key, "team_")
		teamIDStr = strings.TrimSuffix(teamIDStr, "_division")
		teamID, err := strconv.ParseUint(teamIDStr, 10, 32)
		if err != nil {
			continue
		}

		newDivID, err := strconv.ParseUint(values[0], 10, 32)
		if err != nil {
			continue
		}

		// Only update if division actually changed
		team, err := h.service.GetTeamByID(uint(teamID))
		if err != nil {
			continue
		}
		if team.DivisionID != uint(newDivID) {
			h.service.MoveTeamToDivision(uint(teamID), uint(newDivID))
		}
	}

	activeSeason, _ := h.service.GetActiveSeason()
	if activeSeason != nil {
		http.Redirect(w, r, "/admin/league/divisions/review?success=updated", http.StatusSeeOther)
	} else {
		http.Redirect(w, r, "/admin/league/teams/away", http.StatusSeeOther)
	}
}

// AwayTeamWithRelations represents an away team with its related data
type AwayTeamWithRelations struct {
	models.Team
	ClubName     string
	DivisionName string
	SeasonName   string
}

// HandleAwayTeams handles away team management routes
func (h *TeamsHandler) HandleAwayTeams(w http.ResponseWriter, r *http.Request) {
	log.Printf("Admin away teams handler called with path: %s, method: %s", r.URL.Path, r.Method)

	// Check if this is a specific away team detail request
	trimmed := strings.TrimPrefix(r.URL.Path, "/admin/league/teams/away/")
	if trimmed != "" && trimmed != r.URL.Path {
		h.handleAwayTeamDetail(w, r)
		return
	}

	user, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleAwayTeamsGet(w, r, user)
	case http.MethodPost:
		h.handleAwayTeamCreate(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAwayTeamsGet handles GET requests for the away teams list
func (h *TeamsHandler) handleAwayTeamsGet(w http.ResponseWriter, r *http.Request, user *models.User) {
	awayTeams, err := h.service.GetAwayTeams()
	if err != nil {
		logAndError(w, "Failed to load away teams", err, http.StatusInternalServerError)
		return
	}

	activeSeason, _ := h.service.GetActiveSeason()
	var divisions []models.Division
	if activeSeason != nil {
		divisions, _ = h.service.GetDivisionsBySeason(activeSeason.ID)
	}

	clubs, _ := h.service.GetAllClubs()

	successMsg := ""
	switch r.URL.Query().Get("success") {
	case "created":
		successMsg = "Away team created successfully."
	case "deleted":
		successMsg = "Away team deleted successfully."
	}

	tmpl, err := parseTemplate(h.templateDir, "admin/away_teams.html")
	if err != nil {
		log.Printf("Error parsing away teams template: %v", err)
		renderFallbackHTML(w, "Admin - Away Teams", "Away Team Management",
			"Away team management page", "/admin/league/teams")
		return
	}

	if err := renderTemplate(w, tmpl, map[string]interface{}{
		"User":         user,
		"AwayTeams":    awayTeams,
		"ActiveSeason": activeSeason,
		"Divisions":    divisions,
		"Clubs":        clubs,
		"Success":      successMsg,
	}); err != nil {
		logAndError(w, err.Error(), err, http.StatusInternalServerError)
	}
}

// handleAwayTeamCreate handles POST requests to create a new away team
func (h *TeamsHandler) handleAwayTeamCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		logAndError(w, "Failed to parse form data", err, http.StatusBadRequest)
		return
	}

	action := r.FormValue("action")
	if action != "create" {
		http.Error(w, "Unknown action", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	clubIDStr := r.FormValue("club_id")
	divisionIDStr := r.FormValue("division_id")

	if name == "" || clubIDStr == "" || divisionIDStr == "" {
		http.Error(w, "Team name, club, and division are required", http.StatusBadRequest)
		return
	}

	clubID, err := strconv.ParseUint(clubIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid club ID", http.StatusBadRequest)
		return
	}

	divisionID, err := strconv.ParseUint(divisionIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid division ID", http.StatusBadRequest)
		return
	}

	// Verify the club is NOT St Ann's (away teams only)
	if h.service.IsStAnnsClub(uint(clubID)) {
		http.Error(w, "Cannot create away team for St Ann's - use Our Teams instead", http.StatusBadRequest)
		return
	}

	activeSeason, err := h.service.GetActiveSeason()
	if err != nil || activeSeason == nil {
		http.Error(w, "No active season found", http.StatusBadRequest)
		return
	}

	team := &models.Team{
		Name:       name,
		ClubID:     uint(clubID),
		DivisionID: uint(divisionID),
		SeasonID:   activeSeason.ID,
	}

	if err := h.service.CreateTeam(team); err != nil {
		logAndError(w, "Failed to create away team", err, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/league/teams/away?success=created", http.StatusSeeOther)
}

// handleAwayTeamDetail handles requests for individual away team details
func (h *TeamsHandler) handleAwayTeamDetail(w http.ResponseWriter, r *http.Request) {
	log.Printf("Admin away team detail handler called with path: %s, method: %s", r.URL.Path, r.Method)

	// Check for delete action
	if strings.HasSuffix(r.URL.Path, "/delete") {
		h.handleAwayTeamDelete(w, r)
		return
	}

	user, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	teamID, err := parseIDFromPath(r.URL.Path, "/admin/league/teams/away/")
	if err != nil {
		logAndError(w, "Invalid team ID", err, http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleAwayTeamDetailGet(w, r, user, teamID)
	case http.MethodPost:
		h.handleAwayTeamUpdate(w, r, user, teamID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAwayTeamDetailGet handles GET requests for away team detail page
func (h *TeamsHandler) handleAwayTeamDetailGet(w http.ResponseWriter, r *http.Request, user *models.User, teamID uint) {
	teamDetail, err := h.service.GetTeamDetail(teamID)
	if err != nil {
		logAndError(w, "Team not found", err, http.StatusNotFound)
		return
	}

	// Verify this is an away team
	if h.service.IsStAnnsClub(teamDetail.ClubID) {
		http.Redirect(w, r, fmt.Sprintf("/admin/league/teams/%d", teamID), http.StatusSeeOther)
		return
	}

	upcomingFixtures, err := h.service.GetUpcomingFixturesForTeam(teamID, 5)
	if err != nil {
		log.Printf("Failed to get upcoming fixtures for away team: %v", err)
	}

	activeSeason, _ := h.service.GetActiveSeason()
	var divisions []models.Division
	if activeSeason != nil {
		divisions, _ = h.service.GetDivisionsBySeason(activeSeason.ID)
	}

	clubs, _ := h.service.GetAllClubs()

	successMsg := ""
	if r.URL.Query().Get("success") == "updated" {
		successMsg = "Away team updated successfully."
	}

	tmpl, err := parseTemplate(h.templateDir, "admin/away_team_detail.html")
	if err != nil {
		log.Printf("Error parsing away team detail template: %v", err)
		renderFallbackHTML(w, "Away Team Detail", "Away Team Detail",
			"Away team detail page", "/admin/league/teams/away")
		return
	}

	if err := renderTemplate(w, tmpl, map[string]interface{}{
		"User":             user,
		"TeamDetail":       teamDetail,
		"UpcomingFixtures": upcomingFixtures,
		"ActiveSeason":     activeSeason,
		"Divisions":        divisions,
		"Clubs":            clubs,
		"Success":          successMsg,
	}); err != nil {
		logAndError(w, err.Error(), err, http.StatusInternalServerError)
	}
}

// handleAwayTeamUpdate handles POST requests to update an away team
func (h *TeamsHandler) handleAwayTeamUpdate(w http.ResponseWriter, r *http.Request, user *models.User, teamID uint) {
	if err := r.ParseForm(); err != nil {
		logAndError(w, "Failed to parse form data", err, http.StatusBadRequest)
		return
	}

	team, err := h.service.GetTeamByID(teamID)
	if err != nil {
		logAndError(w, "Team not found", err, http.StatusNotFound)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name != "" {
		team.Name = name
	}

	if clubIDStr := r.FormValue("club_id"); clubIDStr != "" {
		if clubID, err := strconv.ParseUint(clubIDStr, 10, 32); err == nil {
			team.ClubID = uint(clubID)
		}
	}

	if divIDStr := r.FormValue("division_id"); divIDStr != "" {
		if divID, err := strconv.ParseUint(divIDStr, 10, 32); err == nil {
			team.DivisionID = uint(divID)
		}
	}

	if err := h.service.UpdateTeam(team); err != nil {
		logAndError(w, "Failed to update away team", err, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/admin/league/teams/away/%d?success=updated", teamID), http.StatusSeeOther)
}

// handleAwayTeamDelete handles POST requests to delete an away team
func (h *TeamsHandler) handleAwayTeamDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/admin/league/teams/away/"), "/")
	if len(pathParts) < 2 || pathParts[0] == "" || pathParts[1] != "delete" {
		http.Error(w, "Invalid delete URL", http.StatusBadRequest)
		return
	}

	teamID, err := strconv.ParseUint(pathParts[0], 10, 32)
	if err != nil {
		logAndError(w, "Invalid team ID", err, http.StatusBadRequest)
		return
	}

	if err := h.service.DeleteTeam(uint(teamID)); err != nil {
		logAndError(w, "Failed to delete away team", err, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/league/teams/away?success=deleted", http.StatusSeeOther)
}
