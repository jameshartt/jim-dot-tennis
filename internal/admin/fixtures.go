package admin

import (
	"fmt"
	"log"
	"net/http"
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
		// Check if this is a team selection request
		if strings.HasSuffix(r.URL.Path, "/team-selection") {
			h.handleTeamSelection(w, r)
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
	// Get St. Ann's fixtures with related data
	club, fixtures, err := h.service.GetStAnnsFixtures()
	if err != nil {
		logAndError(w, "Failed to load fixtures", err, http.StatusInternalServerError)
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
		"User":     user,
		"Club":     club,
		"Fixtures": fixtures,
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
	// Get the fixture with full details
	fixtureDetail, err := h.service.GetFixtureDetail(fixtureID)
	if err != nil {
		logAndError(w, "Fixture not found", err, http.StatusNotFound)
		return
	}

	// Get available players for matchup creation
	availablePlayers, err := h.service.GetAvailablePlayersForMatchup(fixtureID)
	if err != nil {
		logAndError(w, "Failed to load available players for matchup", err, http.StatusInternalServerError)
		return
	}

	// Capture navigation context from query parameters
	navigationContext := map[string]string{
		"from":     r.URL.Query().Get("from"),
		"teamId":   r.URL.Query().Get("teamId"),
		"teamName": r.URL.Query().Get("teamName"),
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
	default:
		http.Error(w, "Unknown action", http.StatusBadRequest)
	}
}

// handleAddPlayerToFixture handles adding a player to the fixture selection
func (h *FixturesHandler) handleAddPlayerToFixture(w http.ResponseWriter, r *http.Request, fixtureID uint) {
	playerID := r.FormValue("player_id")
	isHome := r.FormValue("is_home") == "true"

	if playerID == "" {
		http.Error(w, "Player ID is required", http.StatusBadRequest)
		return
	}

	err := h.service.AddPlayerToFixture(fixtureID, playerID, isHome)
	if err != nil {
		logAndError(w, "Failed to add player to fixture", err, http.StatusInternalServerError)
		return
	}

	// Redirect back to appropriate page
	redirectURL := h.getTeamSelectionRedirectURL(r, fixtureID)
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// handleRemovePlayerFromFixture handles removing a player from the fixture selection
func (h *FixturesHandler) handleRemovePlayerFromFixture(w http.ResponseWriter, r *http.Request, fixtureID uint) {
	playerID := r.FormValue("player_id")

	if playerID == "" {
		http.Error(w, "Player ID is required", http.StatusBadRequest)
		return
	}

	err := h.service.RemovePlayerFromFixture(fixtureID, playerID)
	if err != nil {
		logAndError(w, "Failed to remove player from fixture", err, http.StatusInternalServerError)
		return
	}

	// Redirect back to appropriate page
	redirectURL := h.getTeamSelectionRedirectURL(r, fixtureID)
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// handleClearFixturePlayers handles clearing all players from the fixture selection
func (h *FixturesHandler) handleClearFixturePlayers(w http.ResponseWriter, r *http.Request, fixtureID uint) {
	err := h.service.ClearFixturePlayerSelection(fixtureID)
	if err != nil {
		logAndError(w, "Failed to clear fixture players", err, http.StatusInternalServerError)
		return
	}

	// Redirect back to appropriate page
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

	// Render inline player selection template
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`
		<div class="player-selection-form">
			<h4>Add Players to Selection</h4>
			
			` + renderPlayerGroup("Team Players", availableTeamPlayers, fixtureID, true) + `
			
			` + renderPlayerGroup("All St Ann Players", availableStAnnPlayers, fixtureID, true) + `
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
	// Get the fixture with full details
	fixtureDetail, err := h.service.GetFixtureDetail(fixtureID)
	if err != nil {
		logAndError(w, "Fixture not found", err, http.StatusNotFound)
		return
	}

	// Get available players for this fixture
	teamPlayers, allStAnnPlayers, err := h.service.GetAvailablePlayersForFixture(fixtureID)
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
	if err := renderTemplate(w, tmpl, map[string]interface{}{
		"FixtureDetail":       fixtureDetail,
		"TeamPlayers":         availableTeamPlayers,
		"AllStAnnPlayers":     availableStAnnPlayers,
		"SelectionPercentage": selectionPercentage,
	}); err != nil {
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
	default:
		http.Error(w, "Unknown action", http.StatusBadRequest)
	}
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
