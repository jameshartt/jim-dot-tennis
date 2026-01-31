package admin

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"jim-dot-tennis/internal/models"

	"github.com/google/uuid"
)

// PlayersHandler handles player-related requests
type PlayersHandler struct {
	service     *Service
	templateDir string
}

// NewPlayersHandler creates a new players handler
func NewPlayersHandler(service *Service, templateDir string) *PlayersHandler {
	return &PlayersHandler{
		service:     service,
		templateDir: templateDir,
	}
}

// HandlePlayers handles player management routes
func (h *PlayersHandler) HandlePlayers(w http.ResponseWriter, r *http.Request) {
	log.Printf("Admin players handler called with path: %s, method: %s", r.URL.Path, r.Method)

	// Check if this is a new player request
	if strings.Contains(r.URL.Path, "/new") {
		h.handlePlayerNew(w, r)
		return
	}

	// Check if this is an edit request
	if strings.Contains(r.URL.Path, "/edit") {
		h.handlePlayerEdit(w, r)
		return
	}

	// Check if this is an availability URL request
	if strings.Contains(r.URL.Path, "/generate-availability-url") {
		h.handleGenerateAvailabilityURL(w, r)
		return
	}

	if strings.Contains(r.URL.Path, "/availability-url") {
		h.handleGetAvailabilityURL(w, r)
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
		h.handlePlayersGet(w, r, user)
	case http.MethodPost:
		h.handlePlayersPost(w, r, user)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handlePlayersGet handles GET requests for player management
func (h *PlayersHandler) handlePlayersGet(w http.ResponseWriter, r *http.Request, user *models.User) {
	// Get filter parameters from URL query
	query := r.URL.Query().Get("q")             // search query
	activeFilter := r.URL.Query().Get("status") // "all", "active", "inactive"
	// Note: team_id and division_id are currently not handled in service; will be wired later

	// Default to showing all players if no filter specified
	if activeFilter == "" {
		activeFilter = "all"
	}

	// Get filtered players with availability information from the service
	// Parse team/division filters for service call (multi-select)
	var teamIDs []uint
	var divisionIDs []uint
	for _, v := range r.URL.Query()["team_id"] {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			teamIDs = append(teamIDs, uint(n))
		}
	}
	for _, v := range r.URL.Query()["division_id"] {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			divisionIDs = append(divisionIDs, uint(n))
		}
	}

	playersWithAvail, err := h.service.GetFilteredPlayersWithAvailability(query, activeFilter, 1, teamIDs, divisionIDs)
	if err != nil {
		logAndError(w, "Failed to load players", err, http.StatusInternalServerError)
		return
	}

	// Load St Ann's teams and all divisions for filter dropdowns
	_, stAnnsTeams, _ := h.service.GetStAnnsTeams()
	divisions, _ := h.service.GetAllDivisions()

	// Load the players template
	tmpl, err := parseTemplate(h.templateDir, "admin/players.html")
	if err != nil {
		log.Printf("Error parsing players template: %v", err)
		// Fallback to simple HTML response
		renderFallbackHTML(w, "Admin - Players", "Player Management",
			"Player management page - coming soon", "/admin/league")
		return
	}

	// Execute the template with data
	if err := renderTemplate(w, tmpl, map[string]interface{}{
		"User":                user,
		"Players":             playersWithAvail,
		"SearchQuery":         query,
		"ActiveFilter":        activeFilter,
		"Teams":               stAnnsTeams,
		"Divisions":           divisions,
		"SelectedTeamIDs":     teamIDs,
		"SelectedDivisionIDs": divisionIDs,
	}); err != nil {
		logAndError(w, err.Error(), err, http.StatusInternalServerError)
	}
}

// handlePlayersPost handles POST requests for player management
func (h *PlayersHandler) handlePlayersPost(w http.ResponseWriter, r *http.Request, user *models.User) {
	// TODO: Implement player creation/update/delete
	http.Error(w, "Player operations not yet implemented", http.StatusNotImplemented)
}

// handlePlayerNew handles GET/POST requests for creating a new player
func (h *PlayersHandler) handlePlayerNew(w http.ResponseWriter, r *http.Request) {
	log.Printf("Admin player new handler called with path: %s, method: %s", r.URL.Path, r.Method)

	// Get user from context
	user, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handlePlayerNewGet(w, r, user)
	case http.MethodPost:
		h.handlePlayerNewPost(w, r, user)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handlePlayerNewGet handles GET requests to show the new player form
func (h *PlayersHandler) handlePlayerNewGet(w http.ResponseWriter, r *http.Request, user *models.User) {
	// Load the player new template
	tmpl, err := parseTemplate(h.templateDir, "admin/player_new.html")
	if err != nil {
		log.Printf("Error parsing player new template: %v", err)
		// Fallback to simple HTML response
		renderFallbackHTML(w, "Add New Player", "Add New Player",
			"Add new player form - coming soon", "/admin/league/players")
		return
	}

	// Execute the template with data
	if err := renderTemplate(w, tmpl, map[string]interface{}{
		"User": user,
	}); err != nil {
		logAndError(w, err.Error(), err, http.StatusInternalServerError)
	}
}

// handlePlayerNewPost handles POST requests to create a new player
func (h *PlayersHandler) handlePlayerNewPost(w http.ResponseWriter, r *http.Request, user *models.User) {
	// Parse form data
	if err := r.ParseForm(); err != nil {
		logAndError(w, "Invalid form data", err, http.StatusBadRequest)
		return
	}

	// Get player fields from form
	firstName := strings.TrimSpace(r.FormValue("first_name"))
	lastName := strings.TrimSpace(r.FormValue("last_name"))
	gender := strings.TrimSpace(r.FormValue("gender"))

	// Validate required fields
	if firstName == "" || lastName == "" || gender == "" {
		logAndError(w, "First name, last name, and gender are required", fmt.Errorf("missing required fields"), http.StatusBadRequest)
		return
	}

	// Validate gender value
	if gender != "Men" && gender != "Women" && gender != "Unknown" {
		logAndError(w, "Invalid gender value", fmt.Errorf("gender must be Men, Women, or Unknown"), http.StatusBadRequest)
		return
	}

	// Get St. Ann's club ID automatically
	stAnnsClubs, err := h.service.GetClubsByName("St Ann")
	if err != nil {
		logAndError(w, "Failed to find St. Ann's club", err, http.StatusInternalServerError)
		return
	}
	if len(stAnnsClubs) == 0 {
		logAndError(w, "St. Ann's club not found", fmt.Errorf("club not found"), http.StatusInternalServerError)
		return
	}
	stAnnsClubID := stAnnsClubs[0].ID

	// Create new player with generated UUID and auto-assigned to St. Ann's
	player := &models.Player{
		ID:               uuid.New().String(),
		FirstName:        firstName,
		LastName:         lastName,
		Gender:           models.PlayerGender(gender),
		ReportingPrivacy: models.PlayerReportingVisible, // Default to visible
		ClubID:           stAnnsClubID,                  // Auto-assign to St. Ann's instead of 0
	}

	// Create the player
	if err := h.service.CreatePlayer(player); err != nil {
		logAndError(w, "Failed to create player", err, http.StatusInternalServerError)
		return
	}

	log.Printf("Successfully created new player: %s %s (ID: %s, Club: St. Ann's)", firstName, lastName, player.ID)

	// Redirect to the player edit page to allow setting additional details
	http.Redirect(w, r, fmt.Sprintf("/admin/league/players/%s/edit", player.ID), http.StatusSeeOther)
}

// handlePlayerEdit handles GET/POST requests for editing a player
func (h *PlayersHandler) handlePlayerEdit(w http.ResponseWriter, r *http.Request) {
	log.Printf("Admin player edit handler called with path: %s, method: %s", r.URL.Path, r.Method)

	// Get user from context
	user, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	// Extract player ID from URL path
	// Path format: /admin/players/{id}/edit
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/admin/league/players/"), "/")
	if len(pathParts) < 2 || pathParts[0] == "" || pathParts[1] != "edit" {
		http.Error(w, "Invalid player edit URL", http.StatusBadRequest)
		return
	}
	playerID := pathParts[0]

	switch r.Method {
	case http.MethodGet:
		h.handlePlayerEditGet(w, r, user, playerID)
	case http.MethodPost:
		h.handlePlayerEditPost(w, r, user, playerID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handlePlayerEditGet handles GET requests to show the player edit form
func (h *PlayersHandler) handlePlayerEditGet(w http.ResponseWriter, r *http.Request, user *models.User, playerID string) {
	// Get the player by ID
	player, err := h.service.GetPlayerByID(playerID)
	if err != nil {
		logAndError(w, "Player not found", err, http.StatusNotFound)
		return
	}

	// Get all clubs for the dropdown
	clubs, err := h.service.GetClubs()
	if err != nil {
		logAndError(w, "Failed to load clubs", err, http.StatusInternalServerError)
		return
	}

	// Get unassigned fantasy doubles pairings for the dropdown (excluding already assigned ones)
	fantasyPairings, err := h.service.GetUnassignedFantasyDoubles(playerID)
	if err != nil {
		log.Printf("Failed to load fantasy doubles pairings: %v", err)
		fantasyPairings = []models.FantasyMixedDoubles{} // Default to empty slice
	}

	// Get ATP and WTA players for creating new pairings
	atpPlayers, err := h.service.GetATPPlayers()
	if err != nil {
		log.Printf("Failed to load ATP players: %v", err)
		atpPlayers = []models.ProTennisPlayer{} // Default to empty slice
	}

	wtaPlayers, err := h.service.GetWTAPlayers()
	if err != nil {
		log.Printf("Failed to load WTA players: %v", err)
		wtaPlayers = []models.ProTennisPlayer{} // Default to empty slice
	}

	// Get current fantasy pairing details if assigned
	var currentFantasyDetail *FantasyDoublesDetail
	if player.FantasyMatchID != nil {
		currentFantasyDetail, err = h.service.GetFantasyDoublesDetailByID(*player.FantasyMatchID)
		if err != nil {
			log.Printf("Failed to load current fantasy details: %v", err)
		}
	}

	// Load the player edit template
	tmpl, err := parseTemplate(h.templateDir, "admin/player_edit.html")
	if err != nil {
		log.Printf("Error parsing player edit template: %v", err)
		// Fallback to simple HTML response
		renderFallbackHTML(w, "Edit Player", "Edit Player",
			"Player edit form - coming soon", "/admin/league/players")
		return
	}

	// Helper for template - provide the dereferenced fantasy match ID for comparison
	var currentFantasyMatchID uint = 0
	if player.FantasyMatchID != nil {
		currentFantasyMatchID = *player.FantasyMatchID
	}

	// Execute the template with data
	if err := renderTemplate(w, tmpl, map[string]interface{}{
		"User":                  user,
		"Player":                player,
		"Clubs":                 clubs,
		"FantasyPairings":       fantasyPairings,
		"ATPPlayers":            atpPlayers,
		"WTAPlayers":            wtaPlayers,
		"CurrentFantasyDetail":  currentFantasyDetail,
		"CurrentFantasyMatchID": currentFantasyMatchID,
	}); err != nil {
		logAndError(w, err.Error(), err, http.StatusInternalServerError)
	}
}

// handlePlayerEditPost handles POST requests to update a player
func (h *PlayersHandler) handlePlayerEditPost(w http.ResponseWriter, r *http.Request, user *models.User, playerID string) {
	// Parse form data
	if err := r.ParseForm(); err != nil {
		logAndError(w, "Invalid form data", err, http.StatusBadRequest)
		return
	}

	// Check if this is a fantasy doubles creation request
	if r.FormValue("action") == "create_fantasy" {
		h.handleCreateFantasyDoubles(w, r, user, playerID)
		return
	}

	// Check if this is a random fantasy generation request
	if r.FormValue("action") == "generate_random_fantasy" {
		h.handleGenerateRandomFantasyDoubles(w, r, user, playerID)
		return
	}

	// Get the existing player
	player, err := h.service.GetPlayerByID(playerID)
	if err != nil {
		logAndError(w, "Player not found", err, http.StatusNotFound)
		return
	}

	// Update player fields from form
	player.FirstName = strings.TrimSpace(r.FormValue("first_name"))
	player.LastName = strings.TrimSpace(r.FormValue("last_name"))

	// Handle gender field
	gender := strings.TrimSpace(r.FormValue("gender"))
	if gender == "" {
		logAndError(w, "Gender is required", fmt.Errorf("missing gender field"), http.StatusBadRequest)
		return
	}

	// Validate gender value
	if gender != "Men" && gender != "Women" && gender != "Unknown" {
		logAndError(w, "Invalid gender value", fmt.Errorf("gender must be Men, Women, or Unknown"), http.StatusBadRequest)
		return
	}

	player.Gender = models.PlayerGender(gender)

	// Handle reporting privacy field
	reportingPrivacy := strings.TrimSpace(r.FormValue("reporting_privacy"))
	if reportingPrivacy == "" {
		logAndError(w, "Reporting privacy is required", fmt.Errorf("missing reporting_privacy field"), http.StatusBadRequest)
		return
	}

	// Validate reporting privacy value
	if reportingPrivacy != "visible" && reportingPrivacy != "hidden" {
		logAndError(w, "Invalid reporting privacy value", fmt.Errorf("reporting_privacy must be visible or hidden"), http.StatusBadRequest)
		return
	}

	player.ReportingPrivacy = models.PlayerReportingPrivacy(reportingPrivacy)

	// Handle club ID (convert from string to uint)
	clubIDStr := r.FormValue("club_id")
	if clubIDStr != "" {
		clubID, err := strconv.ParseUint(clubIDStr, 10, 32)
		if err != nil {
			logAndError(w, "Invalid club ID", err, http.StatusBadRequest)
			return
		}
		player.ClubID = uint(clubID)
	}

	// Handle fantasy match assignment
	fantasyMatchIDStr := r.FormValue("fantasy_match_id")
	if fantasyMatchIDStr == "" {
		player.FantasyMatchID = nil
	} else {
		fantasyMatchID, err := strconv.ParseUint(fantasyMatchIDStr, 10, 32)
		if err != nil {
			logAndError(w, "Invalid fantasy match ID", err, http.StatusBadRequest)
			return
		}
		fantasyMatchIDUint := uint(fantasyMatchID)
		player.FantasyMatchID = &fantasyMatchIDUint
	}

	// Update the player
	if err := h.service.UpdatePlayer(player); err != nil {
		logAndError(w, "Failed to update player", err, http.StatusInternalServerError)
		return
	}

	// Redirect back to players list
	http.Redirect(w, r, "/admin/league/players", http.StatusSeeOther)
}

// handleCreateFantasyDoubles handles the creation of a new fantasy doubles pairing
func (h *PlayersHandler) handleCreateFantasyDoubles(w http.ResponseWriter, r *http.Request, user *models.User, playerID string) {
	// Parse the tennis player IDs from the form
	teamAWomanIDStr := r.FormValue("team_a_woman_id")
	teamAManIDStr := r.FormValue("team_a_man_id")
	teamBWomanIDStr := r.FormValue("team_b_woman_id")
	teamBManIDStr := r.FormValue("team_b_man_id")

	// Validate that all IDs are provided
	if teamAWomanIDStr == "" || teamAManIDStr == "" || teamBWomanIDStr == "" || teamBManIDStr == "" {
		logAndError(w, "All four tennis players must be selected", fmt.Errorf("missing tennis player selections"), http.StatusBadRequest)
		return
	}

	// Convert string IDs to integers
	teamAWomanID, err := strconv.Atoi(teamAWomanIDStr)
	if err != nil {
		logAndError(w, "Invalid Team A woman ID", err, http.StatusBadRequest)
		return
	}

	teamAManID, err := strconv.Atoi(teamAManIDStr)
	if err != nil {
		logAndError(w, "Invalid Team A man ID", err, http.StatusBadRequest)
		return
	}

	teamBWomanID, err := strconv.Atoi(teamBWomanIDStr)
	if err != nil {
		logAndError(w, "Invalid Team B woman ID", err, http.StatusBadRequest)
		return
	}

	teamBManID, err := strconv.Atoi(teamBManIDStr)
	if err != nil {
		logAndError(w, "Invalid Team B man ID", err, http.StatusBadRequest)
		return
	}

	// Create the fantasy doubles pairing
	fantasyMatch, err := h.service.CreateFantasyDoubles(teamAWomanID, teamAManID, teamBWomanID, teamBManID)
	if err != nil {
		logAndError(w, "Failed to create fantasy doubles pairing", err, http.StatusInternalServerError)
		return
	}

	log.Printf("Created fantasy doubles pairing with ID: %d, AuthToken: %s", fantasyMatch.ID, fantasyMatch.AuthToken)

	// Assign the newly created pairing to the player
	err = h.service.UpdatePlayerFantasyMatch(playerID, &fantasyMatch.ID)
	if err != nil {
		logAndError(w, "Failed to assign fantasy pairing to player", err, http.StatusInternalServerError)
		return
	}

	log.Printf("Successfully assigned fantasy pairing %d to player %s", fantasyMatch.ID, playerID)

	// Redirect back to the player edit page to show the new assignment
	http.Redirect(w, r, fmt.Sprintf("/admin/league/players/%s/edit", playerID), http.StatusSeeOther)
}

// handleGenerateRandomFantasyDoubles handles the generation of a random fantasy doubles pairing
func (h *PlayersHandler) handleGenerateRandomFantasyDoubles(w http.ResponseWriter, r *http.Request, user *models.User, playerID string) {
	// Generate and assign a random fantasy doubles pairing
	fantasyDetail, err := h.service.GenerateAndAssignRandomFantasyMatch(playerID)
	if err != nil {
		logAndError(w, "Failed to generate and assign random fantasy pairing", err, http.StatusInternalServerError)
		return
	}

	log.Printf("Generated and assigned random fantasy pairing %s to player %s", fantasyDetail.Match.AuthToken, playerID)

	// Redirect back to the player edit page to show the new assignment
	http.Redirect(w, r, fmt.Sprintf("/admin/league/players/%s/edit", playerID), http.StatusSeeOther)
}

// HandlePlayersFilter handles HTMX requests for filtering players
func (h *PlayersHandler) HandlePlayersFilter(w http.ResponseWriter, r *http.Request) {
	// Get user from context for authentication
	_, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	// Only handle GET requests for filtering
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get filter parameters from URL query
	query := r.URL.Query().Get("q")             // search query
	activeFilter := r.URL.Query().Get("status") // filter key
	// Note: team_id and division_id are currently not handled in service; will be wired later

	// Default to showing all players if no filter specified
	if activeFilter == "" {
		activeFilter = "all"
	}

	// Get filtered players with availability information from the service
	// Parse team/division filters for service call (multi-select)
	var teamIDs []uint
	var divisionIDs []uint
	for _, v := range r.URL.Query()["team_id"] {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			teamIDs = append(teamIDs, uint(n))
		}
	}
	for _, v := range r.URL.Query()["division_id"] {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			divisionIDs = append(divisionIDs, uint(n))
		}
	}

	playersWithAvail, err := h.service.GetFilteredPlayersWithAvailability(query, activeFilter, 1, teamIDs, divisionIDs)
	if err != nil {
		logAndError(w, "Failed to load players", err, http.StatusInternalServerError)
		return
	}

	// We now return the FULL table (thead + tbody) so headers can change with dynamic columns
	w.Header().Set("Content-Type", "text/html")

	// Build helper maps for header labels
	teamNameByID := make(map[uint]string)
	if _, teams, err := h.service.GetStAnnsTeams(); err == nil {
		for _, twr := range teams {
			teamNameByID[twr.Team.ID] = twr.Team.Name
		}
	}
	divisionNameByID := make(map[uint]string)
	if divs, err := h.service.GetAllDivisions(); err == nil {
		for _, d := range divs {
			divisionNameByID[d.ID] = d.Name
		}
	}

	// Start table and header
	w.Write([]byte(`<table id="players-table" class="players-table">`))
	w.Write([]byte(`<thead><tr>`))
	w.Write([]byte(`<th class="col-name">Name</th>`))
	w.Write([]byte(`<th class="col-gender">Gender</th>`))
	w.Write([]byte(`<th class="col-availability">Availability Set For Next Week</th>`))
	// Dynamic team columns
	for _, tID := range teamIDs {
		label := teamNameByID[tID]
		if label == "" {
			label = fmt.Sprintf("Team %d", tID)
		}
		w.Write([]byte(fmt.Sprintf(`<th class="col-availability">%s Appearances</th>`, label)))
	}
	// Dynamic division columns
	for _, dID := range divisionIDs {
		label := divisionNameByID[dID]
		if label == "" {
			label = fmt.Sprintf("Division %d", dID)
		}
		w.Write([]byte(fmt.Sprintf(`<th class="col-availability">%s Appearances</th>`, label)))
	}
	w.Write([]byte(`<th class="col-action">Action</th>`))
	w.Write([]byte(`</tr></thead>`))

	// Body
	w.Write([]byte(`<tbody id="players-tbody">`))
	if len(playersWithAvail) > 0 {
		for _, p := range playersWithAvail {
			activeClass := "player-active"
			availStatusIcon := "❌"
			if p.HasSetNextWeekAvail {
				availStatusIcon = "✅"
			}
			actionButton := ""
			if p.HasAvailabilityURL {
				actionButton = fmt.Sprintf(`<button class="btn-copy-url" onclick="copyAvailabilityURL('%s', this)">📋 Copy</button>`, p.Player.ID)
			} else {
				actionButton = fmt.Sprintf(`<button class="btn-generate-url" onclick="generateAvailabilityURL('%s', this)">🔗 Generate</button>`, p.Player.ID)
			}

			w.Write([]byte(fmt.Sprintf(`
				<tr data-player-id="%s" data-player-name="%s %s" class="%s">
					<td class="col-name">
						<a href="/admin/league/players/%s/edit" class="row-link">%s %s</a>
					</td>
					<td class="col-gender">%s</td>
					<td class="col-availability">%s</td>
			`, p.Player.ID, p.Player.FirstName, p.Player.LastName, activeClass,
				p.Player.ID, p.Player.FirstName, p.Player.LastName,
				p.Player.Gender, availStatusIcon)))

			// Team count cells in same order as headers
			for _, tID := range teamIDs {
				count := 0
				if p.TeamAppearanceCounts != nil {
					if c, ok := p.TeamAppearanceCounts[tID]; ok {
						count = c
					}
				}
				w.Write([]byte(fmt.Sprintf(`<td class="col-availability">%d</td>`, count)))
			}
			// Division count cells
			for _, dID := range divisionIDs {
				count := 0
				if p.DivisionAppearanceCounts != nil {
					if c, ok := p.DivisionAppearanceCounts[dID]; ok {
						count = c
					}
				}
				w.Write([]byte(fmt.Sprintf(`<td class="col-availability">%d</td>`, count)))
			}

			w.Write([]byte(fmt.Sprintf(`<td class="col-action">%s</td></tr>`, actionButton)))
		}
	} else {
		colspan := 4 + len(teamIDs) + len(divisionIDs)
		w.Write([]byte(fmt.Sprintf(`<tr><td colspan="%d" style="text-align: center; padding: 2rem;">No players found matching your criteria.</td></tr>`, colspan)))
	}
	w.Write([]byte(`</tbody></table>`))
}

// handleGenerateAvailabilityURL handles POST requests to generate availability URLs
func (h *PlayersHandler) handleGenerateAvailabilityURL(w http.ResponseWriter, r *http.Request) {
	// Get user from context for authentication
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

	// Extract player ID from URL path
	// Path format: /admin/players/{id}/generate-availability-url
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/admin/league/players/"), "/")
	if len(pathParts) < 2 || pathParts[0] == "" {
		http.Error(w, "Invalid player URL", http.StatusBadRequest)
		return
	}
	playerID := pathParts[0]

	// Generate and assign a random fantasy doubles pairing
	fantasyDetail, err := h.service.GenerateAndAssignRandomFantasyMatch(playerID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Construct the availability URL
	availabilityURL := fmt.Sprintf("%s/my-availability/%s", getBaseURL(r), fantasyDetail.Match.AuthToken)

	// Return the URL as JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"url": availabilityURL})
}

// handleGetAvailabilityURL handles GET requests to retrieve existing availability URLs
func (h *PlayersHandler) handleGetAvailabilityURL(w http.ResponseWriter, r *http.Request) {
	// Get user from context for authentication
	_, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	// Only handle GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract player ID from URL path
	// Path format: /admin/players/{id}/availability-url
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/admin/league/players/"), "/")
	if len(pathParts) < 2 || pathParts[0] == "" {
		http.Error(w, "Invalid player URL", http.StatusBadRequest)
		return
	}
	playerID := pathParts[0]

	// Get the player
	player, err := h.service.GetPlayerByID(playerID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Player not found"})
		return
	}

	// Check if player has a fantasy match assigned
	if player.FantasyMatchID == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "No availability URL generated for this player"})
		return
	}

	// Get the fantasy match details
	fantasyDetail, err := h.service.GetFantasyDoublesDetailByID(*player.FantasyMatchID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get fantasy match details"})
		return
	}

	// Construct the availability URL
	availabilityURL := fmt.Sprintf("%s/my-availability/%s", getBaseURL(r), fantasyDetail.Match.AuthToken)

	// Return the URL as JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"url": availabilityURL})
}

// getBaseURL extracts the base URL from the request
func getBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, r.Host)
}
