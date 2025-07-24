package admin

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"jim-dot-tennis/internal/models"
)

// FixturesHandler handles fixture-related requests
type FixturesHandler struct {
	service     *Service
	templateDir string
}

// NewFixturesHandler creates a new fixtures handler
func NewFixturesHandler(service *Service, templateDir string) *FixturesHandler {
	return &FixturesHandler{
		service:     service,
		templateDir: templateDir,
	}
}

// HandleFixtures handles fixture management routes
func (h *FixturesHandler) HandleFixtures(w http.ResponseWriter, r *http.Request) {
	log.Printf("Admin fixtures handler called with path: %s, method: %s", r.URL.Path, r.Method)

	// Check if this is a specific fixture detail request
	if strings.Contains(r.URL.Path, "/fixtures/") && r.URL.Path != "/admin/fixtures/" {
		// Check if this is a notes update request
		if strings.HasSuffix(r.URL.Path, "/notes") {
			h.handleUpdateFixtureNotes(w, r)
			return
		}
		// Check if this is a team selection request
		if strings.HasSuffix(r.URL.Path, "/team-selection") {
			h.handleTeamSelection(w, r)
			return
		}
		// Check if this is a matchup selection request (for POST actions from team selection page)
		if strings.HasSuffix(r.URL.Path, "/matchup-selection") {
			h.handleMatchupSelectionPost(w, r)
			return
		}
		// Check if this is a player selection request (legacy)
		if strings.HasSuffix(r.URL.Path, "/player-selection") {
			h.handlePlayerSelection(w, r)
			return
		}
		h.handleFixtureDetail(w, r)
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
		h.handleFixturesGet(w, r, user)
	case http.MethodPost:
		h.handleFixturesPost(w, r, user)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleFixturesGet handles GET requests for fixture management
func (h *FixturesHandler) handleFixturesGet(w http.ResponseWriter, r *http.Request, user *models.User) {
	// Get St. Ann's upcoming fixtures with related data
	club, upcomingFixtures, err := h.service.GetStAnnsFixtures()
	if err != nil {
		logAndError(w, "Failed to load upcoming fixtures", err, http.StatusInternalServerError)
		return
	}

	// Get St. Ann's past fixtures with related data
	_, pastFixtures, err := h.service.GetStAnnsPastFixtures()
	if err != nil {
		logAndError(w, "Failed to load past fixtures", err, http.StatusInternalServerError)
		return
	}

	// Get all divisions for filtering
	divisions, err := h.service.GetAllDivisions()
	if err != nil {
		logAndError(w, "Failed to load divisions", err, http.StatusInternalServerError)
		return
	}

	// Load the fixtures template
	tmpl, err := parseTemplate(h.templateDir, "admin/fixtures.html")
	if err != nil {
		log.Printf("Error parsing fixtures template: %v", err)
		// Fallback to simple HTML response
		renderFallbackHTML(w, "Admin - Fixtures", "Fixture Management",
			"Fixture management page - coming soon", "/admin")
		return
	}

	// Execute the template with data
	if err := renderTemplate(w, tmpl, map[string]interface{}{
		"User":             user,
		"Club":             club,
		"UpcomingFixtures": upcomingFixtures,
		"PastFixtures":     pastFixtures,
		"Divisions":        divisions,
	}); err != nil {
		logAndError(w, err.Error(), err, http.StatusInternalServerError)
	}
}

// handleFixturesPost handles POST requests for fixture management
func (h *FixturesHandler) handleFixturesPost(w http.ResponseWriter, r *http.Request, user *models.User) {
	// TODO: Implement fixture creation/update/delete
	http.Error(w, "Fixture operations not yet implemented", http.StatusNotImplemented)
}

// handleFixtureDetail handles requests for individual fixture details
func (h *FixturesHandler) handleFixtureDetail(w http.ResponseWriter, r *http.Request) {
	log.Printf("Admin fixture detail handler called with path: %s, method: %s", r.URL.Path, r.Method)

	// Get user from context
	user, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	// Extract fixture ID from URL path
	fixtureID, err := parseIDFromPath(r.URL.Path, "/admin/fixtures/")
	if err != nil {
		logAndError(w, "Invalid fixture ID", err, http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleFixtureDetailGet(w, r, user, fixtureID)
	case http.MethodPost:
		h.handleFixtureDetailPost(w, r, user, fixtureID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleFixtureDetailGet handles GET requests to show the fixture detail page
func (h *FixturesHandler) handleFixtureDetailGet(w http.ResponseWriter, r *http.Request, user *models.User, fixtureID uint) {
	// Capture navigation context from query parameters
	navigationContext := map[string]string{
		"from":         r.URL.Query().Get("from"),
		"teamId":       r.URL.Query().Get("teamId"),
		"teamName":     r.URL.Query().Get("teamName"),
		"managingTeam": r.URL.Query().Get("managingTeam"),
	}

	// Check if we have a managing team parameter (for derby matches)
	var fixtureDetail interface{}
	var err error
	managingTeamParam := r.URL.Query().Get("managingTeam")

	if managingTeamParam != "" {
		// Parse managing team ID
		managingTeamIDUint64, parseErr := strconv.ParseUint(managingTeamParam, 10, 32)
		if parseErr == nil {
			// Use team-aware fixture detail for derby matches
			managingTeamID := uint(managingTeamIDUint64)
			fixtureDetail, err = h.service.GetFixtureDetailWithTeamContext(fixtureID, managingTeamID)
		} else {
			// Fall back to regular method if parsing fails
			fixtureDetail, err = h.service.GetFixtureDetail(fixtureID)
		}
	} else {
		// Use regular fixture detail method
		fixtureDetail, err = h.service.GetFixtureDetail(fixtureID)
	}

	if err != nil {
		logAndError(w, "Fixture not found", err, http.StatusNotFound)
		return
	}

	// Determine derby status and managing team information
	var isStAnnsHome bool
	var isStAnnsAway bool
	var isDerby bool
	var managingTeam *models.Team

	// Get fixture details to determine St Ann's position
	if detail, ok := fixtureDetail.(*FixtureDetail); ok {
		// Find St Ann's club ID
		stAnnsClubs, err := h.service.GetClubsByName("St Ann")
		if err == nil && len(stAnnsClubs) > 0 {
			stAnnsClubID := stAnnsClubs[0].ID

			// Check if home team is St Ann's
			if detail.HomeTeam != nil && detail.HomeTeam.ClubID == stAnnsClubID {
				isStAnnsHome = true
			}

			// Check if away team is St Ann's
			if detail.AwayTeam != nil && detail.AwayTeam.ClubID == stAnnsClubID {
				isStAnnsAway = true
			}

			// Determine if it's a derby match (both teams are St Ann's)
			if isStAnnsHome && isStAnnsAway {
				isDerby = true
			}
		}
	}

	// Check if we have a managing team from query parameters (indicates derby match)
	if managingTeamParam != "" {
		isDerby = true

		// Parse managing team ID and get team details
		if managingTeamIDUint64, parseErr := strconv.ParseUint(managingTeamParam, 10, 32); parseErr == nil {
			managingTeamID := uint(managingTeamIDUint64)

			// Get the managing team details from the service
			if teamDetail, teamErr := h.service.GetTeamDetail(managingTeamID); teamErr == nil {
				managingTeam = &teamDetail.Team
			}
		}
	}

	// Get available players for matchup creation
	var availablePlayers []models.Player
	if managingTeam != nil {
		// Check if we have a valid fixture detail with selected players
		if detail, ok := fixtureDetail.(*FixtureDetail); ok && len(detail.SelectedPlayers) > 0 {
			// For derby matches with selected players, use the team-specific selected players
			for _, sp := range detail.SelectedPlayers {
				availablePlayers = append(availablePlayers, sp.Player)
			}
		} else {
			// For derby matches without selected players, use standard method
			availablePlayers, err = h.service.GetAvailablePlayersForMatchup(fixtureID)
			if err != nil {
				logAndError(w, "Failed to load available players for matchup", err, http.StatusInternalServerError)
				return
			}
		}
	} else {
		// For regular matches, use standard method
		availablePlayers, err = h.service.GetAvailablePlayersForMatchup(fixtureID)
		if err != nil {
			logAndError(w, "Failed to load available players for matchup", err, http.StatusInternalServerError)
			return
		}
	}

	// Load the fixture detail template
	tmpl, err := parseTemplate(h.templateDir, "admin/fixture_detail.html")
	if err != nil {
		log.Printf("Error parsing fixture detail template: %v", err)
		// Fallback to simple HTML response
		renderFallbackHTML(w, "Fixture Detail", "Fixture Detail",
			"Fixture detail page - coming soon", "/admin/fixtures")
		return
	}

	// Execute the template with data
	if err := renderTemplate(w, tmpl, map[string]interface{}{
		"User":              user,
		"FixtureDetail":     fixtureDetail,
		"AvailablePlayers":  availablePlayers,
		"NavigationContext": navigationContext,
		"IsStAnnsHome":      isStAnnsHome,
		"IsStAnnsAway":      isStAnnsAway,
		"IsDerby":           isDerby,
		"ManagingTeam":      managingTeam,
	}); err != nil {
		logAndError(w, err.Error(), err, http.StatusInternalServerError)
	}
}

// handleFixtureDetailPost handles POST requests to update fixture details
func (h *FixturesHandler) handleFixtureDetailPost(w http.ResponseWriter, r *http.Request, user *models.User, fixtureID uint) {
	action := r.FormValue("action")

	switch action {
	case "add_player":
		h.handleAddPlayerToFixture(w, r, fixtureID)
	case "remove_player":
		h.handleRemovePlayerFromFixture(w, r, fixtureID)
	case "clear_players":
		h.handleClearFixturePlayers(w, r, fixtureID)
	case "update_matchup":
		h.handleUpdateMatchup(w, r, fixtureID)
	case "update_notes":
		h.handleUpdateFixtureNotes(w, r)
	case "set_day_captain":
		h.handleSetDayCaptain(w, r, fixtureID)
	default:
		log.Printf("Unknown action: %s", action)
		http.Error(w, "Unknown action", http.StatusBadRequest)
	}
}

// handleAddPlayerToFixture handles adding a player to the fixture selection
func (h *FixturesHandler) handleAddPlayerToFixture(w http.ResponseWriter, r *http.Request, fixtureID uint) {
	playerID := r.FormValue("player_id")
	isHome := r.FormValue("is_home") == "true"
	managingTeamParam := r.FormValue("managing_team_id")

	if playerID == "" {
		http.Error(w, "Player ID is required", http.StatusBadRequest)
		return
	}

	var err error

	// Check if this is for a specific managing team (derby match)
	if managingTeamParam != "" {
		managingTeamID, parseErr := strconv.ParseUint(managingTeamParam, 10, 32)
		if parseErr != nil {
			http.Error(w, "Invalid managing team ID", http.StatusBadRequest)
			return
		}
		err = h.service.AddPlayerToFixtureWithTeam(fixtureID, playerID, isHome, uint(managingTeamID))
	} else {
		err = h.service.AddPlayerToFixture(fixtureID, playerID, isHome)
	}

	if err != nil {
		logAndError(w, "Failed to add player to fixture", err, http.StatusInternalServerError)
		return
	}

	// Check if this is an HTMX request
	if r.Header.Get("HX-Request") == "true" {
		// Return updated team selection container
		h.renderTeamSelectionContainer(w, r, fixtureID)
		return
	}

	// Redirect back to appropriate page for non-HTMX requests
	redirectURL := h.getTeamSelectionRedirectURL(r, fixtureID)
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// handleRemovePlayerFromFixture handles removing a player from the fixture selection
func (h *FixturesHandler) handleRemovePlayerFromFixture(w http.ResponseWriter, r *http.Request, fixtureID uint) {
	playerID := r.FormValue("player_id")
	managingTeamParam := r.FormValue("managing_team_id")

	if playerID == "" {
		http.Error(w, "Player ID is required", http.StatusBadRequest)
		return
	}

	var err error

	// Check if this is for a specific managing team (derby match)
	if managingTeamParam != "" {
		managingTeamID, parseErr := strconv.ParseUint(managingTeamParam, 10, 32)
		if parseErr != nil {
			http.Error(w, "Invalid managing team ID", http.StatusBadRequest)
			return
		}
		err = h.service.RemovePlayerFromFixtureByTeam(fixtureID, playerID, uint(managingTeamID))
	} else {
		err = h.service.RemovePlayerFromFixture(fixtureID, playerID)
	}

	if err != nil {
		logAndError(w, "Failed to remove player from fixture", err, http.StatusInternalServerError)
		return
	}

	// Check if this is an HTMX request
	if r.Header.Get("HX-Request") == "true" {
		// Return updated team selection container
		h.renderTeamSelectionContainer(w, r, fixtureID)
		return
	}

	// Redirect back to appropriate page for non-HTMX requests
	redirectURL := h.getTeamSelectionRedirectURL(r, fixtureID)
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// handleClearFixturePlayers handles clearing all players from the fixture selection
func (h *FixturesHandler) handleClearFixturePlayers(w http.ResponseWriter, r *http.Request, fixtureID uint) {
	managingTeamParam := r.FormValue("managing_team_id")

	var err error

	// Check if this is for a specific managing team (derby match)
	if managingTeamParam != "" {
		managingTeamID, parseErr := strconv.ParseUint(managingTeamParam, 10, 32)
		if parseErr != nil {
			http.Error(w, "Invalid managing team ID", http.StatusBadRequest)
			return
		}
		err = h.service.ClearFixturePlayerSelectionByTeam(fixtureID, uint(managingTeamID))
	} else {
		err = h.service.ClearFixturePlayerSelection(fixtureID)
	}

	if err != nil {
		logAndError(w, "Failed to clear fixture players", err, http.StatusInternalServerError)
		return
	}

	// Check if this is an HTMX request
	if r.Header.Get("HX-Request") == "true" {
		// Return updated team selection container
		h.renderTeamSelectionContainer(w, r, fixtureID)
		return
	}

	// Redirect back to appropriate page for non-HTMX requests
	redirectURL := h.getTeamSelectionRedirectURL(r, fixtureID)
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// handleUpdateMatchup handles updating matchup player assignments for St Ann's players
func (h *FixturesHandler) handleUpdateMatchup(w http.ResponseWriter, r *http.Request, fixtureID uint) {
	matchupType := models.MatchupType(r.FormValue("matchup_type"))
	stAnnsPlayer1ID := r.FormValue("stanns_player_1")
	stAnnsPlayer2ID := r.FormValue("stanns_player_2")

	// Validate matchup type
	if matchupType == "" {
		http.Error(w, "Matchup type is required", http.StatusBadRequest)
		return
	}

	// Get or create the matchup
	matchup, err := h.service.GetOrCreateMatchup(fixtureID, matchupType)
	if err != nil {
		logAndError(w, "Failed to get or create matchup", err, http.StatusInternalServerError)
		return
	}

	// Update the St Ann's players for this matchup
	// We determine if St Ann's is home or away and assign accordingly
	err = h.service.UpdateStAnnsMatchupPlayers(matchup.ID, fixtureID, stAnnsPlayer1ID, stAnnsPlayer2ID)
	if err != nil {
		logAndError(w, "Failed to update matchup players", err, http.StatusInternalServerError)
		return
	}

	// Redirect back to fixture detail
	http.Redirect(w, r, fmt.Sprintf("/admin/fixtures/%d", fixtureID), http.StatusSeeOther)
}

// handlePlayerSelection handles requests for the player selection interface
func (h *FixturesHandler) handlePlayerSelection(w http.ResponseWriter, r *http.Request) {
	// Get user from context
	_, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	// Extract fixture ID from URL path, removing the "/player-selection" suffix
	path := strings.TrimSuffix(r.URL.Path, "/player-selection")
	fixtureID, err := parseIDFromPath(path, "/admin/fixtures/")
	if err != nil {
		logAndError(w, "Invalid fixture ID", err, http.StatusBadRequest)
		return
	}

	// Get available players for this fixture
	teamPlayers, allStAnnPlayers, err := h.service.GetAvailablePlayersForFixture(fixtureID)
	if err != nil {
		logAndError(w, "Failed to load available players", err, http.StatusInternalServerError)
		return
	}

	// Get current selected players to filter them out
	fixtureDetail, err := h.service.GetFixtureDetail(fixtureID)
	if err != nil {
		logAndError(w, "Failed to load fixture detail", err, http.StatusInternalServerError)
		return
	}

	// Create a map of already selected player IDs for quick filtering
	selectedMap := make(map[string]bool)
	for _, sp := range fixtureDetail.SelectedPlayers {
		selectedMap[sp.PlayerID] = true
	}

	// Filter out already selected players
	var availableTeamPlayers []models.Player
	for _, player := range teamPlayers {
		if !selectedMap[player.ID] {
			availableTeamPlayers = append(availableTeamPlayers, player)
		}
	}

	var availableStAnnPlayers []models.Player
	for _, player := range allStAnnPlayers {
		if !selectedMap[player.ID] {
			availableStAnnPlayers = append(availableStAnnPlayers, player)
		}
	}

	// Determine if St Ann's is home or away
	isStAnnsHome := h.service.IsStAnnsHomeInFixture(fixtureID)

	// Render inline player selection template
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`
		<div class="player-selection-form">
			<h4>Add Players to Selection</h4>
			
			` + renderPlayerGroup("Team Players", availableTeamPlayers, fixtureID, isStAnnsHome) + `
			
			` + renderPlayerGroup("All St Ann Players", availableStAnnPlayers, fixtureID, isStAnnsHome) + `
		</div>
	`))
}

// Helper function to render a group of players
func renderPlayerGroup(title string, players []models.Player, fixtureID uint, isHome bool) string {
	if len(players) == 0 {
		return `<div class="player-group"><h5>` + title + `</h5><p class="no-players">No available players</p></div>`
	}

	html := `<div class="player-group">
		<h5>` + title + ` (` + fmt.Sprintf("%d", len(players)) + ` available)</h5>
		<div class="player-buttons">`

	for _, player := range players {
		html += `
			<form method="post" style="display: inline;">
				<input type="hidden" name="action" value="add_player">
				<input type="hidden" name="player_id" value="` + player.ID + `">
				<input type="hidden" name="is_home" value="` + fmt.Sprintf("%t", isHome) + `">
				<button type="submit" class="btn-add-player" 
				        hx-post="/admin/fixtures/` + fmt.Sprintf("%d", fixtureID) + `" 
				        hx-target="body" 
				        hx-swap="outerHTML">
					` + player.FirstName + ` ` + player.LastName + `
				</button>
			</form>`
	}

	html += `</div></div>`
	return html
}

// handleTeamSelection handles requests for the dedicated team selection page
func (h *FixturesHandler) handleTeamSelection(w http.ResponseWriter, r *http.Request) {
	// Get user from context
	_, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	// Extract fixture ID from URL path, removing the "/team-selection" suffix
	path := strings.TrimSuffix(r.URL.Path, "/team-selection")
	fixtureID, err := parseIDFromPath(path, "/admin/fixtures/")
	if err != nil {
		logAndError(w, "Invalid fixture ID", err, http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleTeamSelectionGet(w, r, fixtureID)
	case http.MethodPost:
		h.handleTeamSelectionPost(w, r, fixtureID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleTeamSelectionGet handles GET requests to show the team selection page
func (h *FixturesHandler) handleTeamSelectionGet(w http.ResponseWriter, r *http.Request, fixtureID uint) {
	// Check for managing team parameter (for derby matches)
	managingTeamParam := r.URL.Query().Get("managingTeam")
	var managingTeamID uint
	var managingTeam *models.Team

	var fixtureDetail *FixtureDetail
	var err error

	if managingTeamParam != "" {
		// Parse managing team ID and get team details
		if managingTeamIDUint64, parseErr := strconv.ParseUint(managingTeamParam, 10, 32); parseErr == nil {
			managingTeamID = uint(managingTeamIDUint64)

			// Get the managing team details from the service
			if teamDetail, teamErr := h.service.GetTeamDetail(managingTeamID); teamErr == nil {
				managingTeam = &teamDetail.Team
			}

			// Use team-aware fixture detail for derby matches
			fixtureDetail, err = h.service.GetFixtureDetailWithTeamContext(fixtureID, managingTeamID)
		} else {
			// Fall back to regular method if parsing fails
			fixtureDetail, err = h.service.GetFixtureDetail(fixtureID)
		}
	} else {
		// Use regular fixture detail method
		fixtureDetail, err = h.service.GetFixtureDetail(fixtureID)
	}

	if err != nil {
		logAndError(w, "Fixture not found", err, http.StatusNotFound)
		return
	}

	// Get available players for this fixture with availability and eligibility status
	var managingTeamIDForEligibility uint
	if managingTeamParam != "" {
		if managingTeamIDUint64, parseErr := strconv.ParseUint(managingTeamParam, 10, 32); parseErr == nil {
			managingTeamIDForEligibility = uint(managingTeamIDUint64)
		}
	}

	teamPlayers, allStAnnPlayers, err := h.service.GetAvailablePlayersWithEligibilityForTeamSelection(fixtureID, managingTeamIDForEligibility)
	if err != nil {
		logAndError(w, "Failed to load available players", err, http.StatusInternalServerError)
		return
	}

	// Create a map of already selected player IDs for quick filtering
	selectedMap := make(map[string]bool)
	for _, sp := range fixtureDetail.SelectedPlayers {
		selectedMap[sp.PlayerID] = true
	}

	// Filter out already selected players
	var availableTeamPlayers []PlayerWithEligibility
	for _, player := range teamPlayers {
		if !selectedMap[player.Player.ID] {
			availableTeamPlayers = append(availableTeamPlayers, player)
		}
	}

	var availableStAnnPlayers []PlayerWithEligibility
	for _, player := range allStAnnPlayers {
		if !selectedMap[player.Player.ID] {
			availableStAnnPlayers = append(availableStAnnPlayers, player)
		}
	}

	// Load the team selection template
	tmpl, err := parseTemplate(h.templateDir, "admin/fixture_team_selection.html")
	if err != nil {
		log.Printf("Error parsing team selection template: %v", err)
		// Fallback to simple HTML response
		renderFallbackHTML(w, "Team Selection", "Team Selection",
			"Team selection page - coming soon", "/admin/fixtures/"+fmt.Sprintf("%d", fixtureID))
		return
	}

	// Calculate selection percentage
	selectedCount := len(fixtureDetail.SelectedPlayers)
	selectionPercentage := 0
	if selectedCount > 0 {
		selectionPercentage = (selectedCount * 100) / 8
	}

	// Execute the template with data
	templateData := map[string]interface{}{
		"FixtureDetail":       fixtureDetail,
		"TeamPlayers":         availableTeamPlayers,
		"AllStAnnPlayers":     availableStAnnPlayers,
		"SelectionPercentage": selectionPercentage,
	}

	// Include managing team information if present
	if managingTeam != nil {
		templateData["ManagingTeam"] = managingTeam
		templateData["ManagingTeamID"] = managingTeamID
	}

	if err := renderTemplate(w, tmpl, templateData); err != nil {
		logAndError(w, err.Error(), err, http.StatusInternalServerError)
	}
}

// handleTeamSelectionPost handles POST requests to update team selection
func (h *FixturesHandler) handleTeamSelectionPost(w http.ResponseWriter, r *http.Request, fixtureID uint) {
	action := r.FormValue("action")

	switch action {
	case "add_player":
		h.handleAddPlayerToFixture(w, r, fixtureID)
	case "remove_player":
		h.handleRemovePlayerFromFixture(w, r, fixtureID)
	case "clear_players":
		h.handleClearFixturePlayers(w, r, fixtureID)
	case "set_day_captain":
		h.handleSetDayCaptain(w, r, fixtureID)
	default:
		http.Error(w, "Unknown action", http.StatusBadRequest)
	}
}

// handleMatchupSelectionPost handles POST requests for matchup operations (called from team selection page)
func (h *FixturesHandler) handleMatchupSelectionPost(w http.ResponseWriter, r *http.Request) {
	// Get user from context
	_, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	// Extract fixture ID from URL path, removing the "/matchup-selection" suffix
	path := strings.TrimSuffix(r.URL.Path, "/matchup-selection")
	fixtureID, err := parseIDFromPath(path, "/admin/fixtures/")
	if err != nil {
		logAndError(w, "Invalid fixture ID", err, http.StatusBadRequest)
		return
	}

	// Only handle POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	action := r.FormValue("action")

	switch action {
	case "assign_player":
		h.handleAssignPlayerToMatchup(w, r, fixtureID)
	case "remove_player":
		h.handleRemovePlayerFromMatchup(w, r, fixtureID)
	case "set_day_captain":
		h.handleSetDayCaptain(w, r, fixtureID)
	default:
		http.Error(w, "Unknown action", http.StatusBadRequest)
	}
}

// handleAssignPlayerToMatchup handles assigning a single player to a matchup
func (h *FixturesHandler) handleAssignPlayerToMatchup(w http.ResponseWriter, r *http.Request, fixtureID uint) {
	playerID := r.FormValue("player_id")
	matchupType := models.MatchupType(r.FormValue("matchup_type"))
	managingTeamParam := r.FormValue("managing_team_id")

	if playerID == "" {
		http.Error(w, "Player ID is required", http.StatusBadRequest)
		return
	}

	if matchupType == "" {
		http.Error(w, "Matchup type is required", http.StatusBadRequest)
		return
	}

	var matchup *models.Matchup
	var err error

	// Check if this is for a specific managing team (derby match)
	if managingTeamParam != "" {
		managingTeamID, parseErr := strconv.ParseUint(managingTeamParam, 10, 32)
		if parseErr != nil {
			http.Error(w, "Invalid managing team ID", http.StatusBadRequest)
			return
		}
		// Use team-aware method for derby matches
		matchup, err = h.service.GetOrCreateMatchupWithTeam(fixtureID, matchupType, uint(managingTeamID))
	} else {
		// Use regular method for non-derby matches
		matchup, err = h.service.GetOrCreateMatchup(fixtureID, matchupType)
	}

	if err != nil {
		logAndError(w, "Failed to get or create matchup", err, http.StatusInternalServerError)
		return
	}

	// Add the player to the matchup (supports 3+ players for temporary over-assignment)
	err = h.service.AddPlayerToMatchup(matchup.ID, playerID, fixtureID)
	if err != nil {
		logAndError(w, "Failed to update matchup players", err, http.StatusInternalServerError)
		return
	}

	// For HTMX requests, return a success response
	if r.Header.Get("HX-Request") == "true" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Redirect back to team selection page for non-HTMX requests
	redirectURL := fmt.Sprintf("/admin/fixtures/%d/team-selection", fixtureID)
	if managingTeamParam != "" {
		redirectURL += fmt.Sprintf("?managingTeam=%s", managingTeamParam)
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// handleRemovePlayerFromMatchup handles removing a player from a matchup
func (h *FixturesHandler) handleRemovePlayerFromMatchup(w http.ResponseWriter, r *http.Request, fixtureID uint) {
	playerID := r.FormValue("player_id")
	matchupType := models.MatchupType(r.FormValue("matchup_type"))

	if playerID == "" {
		http.Error(w, "Player ID is required", http.StatusBadRequest)
		return
	}

	if matchupType == "" {
		http.Error(w, "Matchup type is required", http.StatusBadRequest)
		return
	}

	// Get or create the matchup
	var matchup *models.Matchup
	var err error

	// Check if this is for a specific managing team (derby match)
	managingTeamParam := r.FormValue("managing_team_id")
	if managingTeamParam != "" {
		managingTeamID, parseErr := strconv.ParseUint(managingTeamParam, 10, 32)
		if parseErr != nil {
			http.Error(w, "Invalid managing team ID", http.StatusBadRequest)
			return
		}
		// Use team-aware method for derby matches
		matchup, err = h.service.GetOrCreateMatchupWithTeam(fixtureID, matchupType, uint(managingTeamID))
	} else {
		// Use regular method for non-derby matches
		matchup, err = h.service.GetOrCreateMatchup(fixtureID, matchupType)
	}

	if err != nil {
		logAndError(w, "Failed to get matchup", err, http.StatusInternalServerError)
		return
	}

	// Remove the player from the matchup
	err = h.service.RemovePlayerFromMatchup(matchup.ID, playerID)
	if err != nil {
		logAndError(w, "Failed to update matchup players", err, http.StatusInternalServerError)
		return
	}

	// For HTMX requests, return a success response
	if r.Header.Get("HX-Request") == "true" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Redirect back to team selection page for non-HTMX requests
	redirectURL := fmt.Sprintf("/admin/fixtures/%d/team-selection", fixtureID)
	managingTeamParam = r.FormValue("managing_team_id")
	if managingTeamParam != "" {
		redirectURL += fmt.Sprintf("?managingTeam=%s", managingTeamParam)
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// Update the redirect targets for team selection actions
func (h *FixturesHandler) getTeamSelectionRedirectURL(r *http.Request, fixtureID uint) string {
	// If this is coming from the team selection page, redirect back to it
	if strings.Contains(r.Header.Get("Referer"), "/team-selection") {
		return fmt.Sprintf("/admin/fixtures/%d/team-selection", fixtureID)
	}
	// Otherwise redirect to fixture detail
	return fmt.Sprintf("/admin/fixtures/%d", fixtureID)
}

// renderTeamSelectionContainer renders just the team selection container for HTMX requests
func (h *FixturesHandler) renderTeamSelectionContainer(w http.ResponseWriter, r *http.Request, fixtureID uint) {
	// Check for managing team parameter (for derby matches)
	managingTeamParam := r.URL.Query().Get("managingTeam")
	if managingTeamParam == "" {
		managingTeamParam = r.FormValue("managing_team_id")
	}

	var fixtureDetail interface{}
	var err error
	var managingTeamID uint

	// Use team-aware or regular fixture detail based on managing team parameter
	if managingTeamParam != "" {
		managingTeamIDUint64, parseErr := strconv.ParseUint(managingTeamParam, 10, 32)
		if parseErr == nil {
			managingTeamID = uint(managingTeamIDUint64)
			fixtureDetail, err = h.service.GetFixtureDetailWithTeamContext(fixtureID, managingTeamID)
		} else {
			// Fall back to regular method if parsing fails
			fixtureDetail, err = h.service.GetFixtureDetail(fixtureID)
		}
	} else {
		fixtureDetail, err = h.service.GetFixtureDetail(fixtureID)
	}

	if err != nil {
		logAndError(w, "Fixture not found", err, http.StatusNotFound)
		return
	}

	// Get available players for this fixture with availability and eligibility status
	var managingTeamIDForEligibility uint
	if managingTeamParam != "" {
		if managingTeamIDUint64, parseErr := strconv.ParseUint(managingTeamParam, 10, 32); parseErr == nil {
			managingTeamIDForEligibility = uint(managingTeamIDUint64)
		}
	}

	teamPlayers, allStAnnPlayers, err := h.service.GetAvailablePlayersWithEligibilityForTeamSelection(fixtureID, managingTeamIDForEligibility)
	if err != nil {
		logAndError(w, "Failed to load available players", err, http.StatusInternalServerError)
		return
	}

	// Create a map of already selected player IDs for quick filtering
	selectedMap := make(map[string]bool)

	// Use reflection or type assertion to get selected players
	// This is a workaround since we're dealing with interface{} types
	if detail, ok := fixtureDetail.(*FixtureDetail); ok {
		for _, sp := range detail.SelectedPlayers {
			selectedMap[sp.PlayerID] = true
		}
	}

	// Filter out already selected players
	var availableTeamPlayers []PlayerWithEligibility
	for _, player := range teamPlayers {
		if !selectedMap[player.Player.ID] {
			availableTeamPlayers = append(availableTeamPlayers, player)
		}
	}

	var availableStAnnPlayers []PlayerWithEligibility
	for _, player := range allStAnnPlayers {
		if !selectedMap[player.Player.ID] {
			availableStAnnPlayers = append(availableStAnnPlayers, player)
		}
	}

	// Load the partial team selection container template for HTMX
	tmpl, err := parseTemplate(h.templateDir, "admin/fixture_team_selection_container.html")
	if err != nil {
		logAndError(w, "Failed to parse team selection container template", err, http.StatusInternalServerError)
		return
	}

	// Calculate selection percentage
	selectedCount := 0
	if detail, ok := fixtureDetail.(*FixtureDetail); ok {
		selectedCount = len(detail.SelectedPlayers)
	}
	selectionPercentage := 0
	if selectedCount > 0 {
		selectionPercentage = (selectedCount * 100) / 8
	}

	// Prepare template data
	templateData := map[string]interface{}{
		"FixtureDetail":       fixtureDetail,
		"TeamPlayers":         availableTeamPlayers,
		"AllStAnnPlayers":     availableStAnnPlayers,
		"SelectionPercentage": selectionPercentage,
	}

	// Include managing team ID if present
	if managingTeamParam != "" {
		templateData["ManagingTeamID"] = managingTeamID
	}

	// Execute the template with data
	if err := renderTemplate(w, tmpl, templateData); err != nil {
		logAndError(w, err.Error(), err, http.StatusInternalServerError)
	}
}

// handleUpdateFixtureNotes handles updating fixture notes
func (h *FixturesHandler) handleUpdateFixtureNotes(w http.ResponseWriter, r *http.Request) {
	// Get user from context
	_, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	// Extract fixture ID from URL path, removing the "/notes" suffix
	path := strings.TrimSuffix(r.URL.Path, "/notes")
	fixtureID, err := parseIDFromPath(path, "/admin/fixtures/")
	if err != nil {
		logAndError(w, "Invalid fixture ID", err, http.StatusBadRequest)
		return
	}

	// Only handle POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the notes from form data
	notes := r.FormValue("notes")

	// Validate notes length (max 1000 characters)
	if len(notes) > 1000 {
		http.Error(w, "Notes cannot exceed 1000 characters", http.StatusBadRequest)
		return
	}

	// Update the fixture notes
	err = h.service.UpdateFixtureNotes(fixtureID, notes)
	if err != nil {
		logAndError(w, "Failed to update fixture notes", err, http.StatusInternalServerError)
		return
	}

	// For HTMX requests, return a success response
	if r.Header.Get("HX-Request") == "true" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Notes updated successfully"))
		return
	}

	// For regular requests, redirect back to fixture detail
	http.Redirect(w, r, fmt.Sprintf("/admin/fixtures/%d", fixtureID), http.StatusSeeOther)
}

// handleSetDayCaptain handles setting the day captain for a fixture
func (h *FixturesHandler) handleSetDayCaptain(w http.ResponseWriter, r *http.Request, fixtureID uint) {
	// Get user from context
	_, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	// Only handle POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the player ID from form data
	playerID := r.FormValue("player_id")
	if playerID == "" {
		http.Error(w, "Player ID is required", http.StatusBadRequest)
		return
	}

	// Update the fixture day captain
	err = h.service.SetFixtureDayCaptain(fixtureID, playerID)
	if err != nil {
		logAndError(w, "Failed to set day captain", err, http.StatusInternalServerError)
		return
	}

	// For HTMX requests, return a success response
	if r.Header.Get("HX-Request") == "true" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Day captain updated successfully"))
		return
	}

	// For regular requests, redirect back to team selection
	managingTeamParam := r.FormValue("managing_team_id")
	redirectURL := fmt.Sprintf("/admin/fixtures/%d/team-selection", fixtureID)
	if managingTeamParam != "" {
		redirectURL += fmt.Sprintf("?managingTeam=%s", managingTeamParam)
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}
