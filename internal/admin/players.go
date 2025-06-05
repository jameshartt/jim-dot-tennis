package admin

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"jim-dot-tennis/internal/models"
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

	// Check if this is an edit request
	if strings.Contains(r.URL.Path, "/edit") {
		h.handlePlayerEdit(w, r)
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

	// Default to showing all players if no filter specified
	if activeFilter == "" {
		activeFilter = "all"
	}

	// Get filtered players from the service
	players, err := h.service.GetFilteredPlayers(query, activeFilter, 1) // Using season 1 for now
	if err != nil {
		logAndError(w, "Failed to load players", err, http.StatusInternalServerError)
		return
	}

	// Load the players template
	tmpl, err := parseTemplate(h.templateDir, "admin/players.html")
	if err != nil {
		log.Printf("Error parsing players template: %v", err)
		// Fallback to simple HTML response
		renderFallbackHTML(w, "Admin - Players", "Player Management",
			"Player management page - coming soon", "/admin")
		return
	}

	// Execute the template with data
	if err := renderTemplate(w, tmpl, map[string]interface{}{
		"User":         user,
		"Players":      players,
		"SearchQuery":  query,
		"ActiveFilter": activeFilter,
	}); err != nil {
		logAndError(w, err.Error(), err, http.StatusInternalServerError)
	}
}

// handlePlayersPost handles POST requests for player management
func (h *PlayersHandler) handlePlayersPost(w http.ResponseWriter, r *http.Request, user *models.User) {
	// TODO: Implement player creation/update/delete
	http.Error(w, "Player operations not yet implemented", http.StatusNotImplemented)
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
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/admin/players/"), "/")
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

	// Load the player edit template
	tmpl, err := parseTemplate(h.templateDir, "admin/player_edit.html")
	if err != nil {
		log.Printf("Error parsing player edit template: %v", err)
		// Fallback to simple HTML response
		renderFallbackHTML(w, "Edit Player", "Edit Player",
			"Player edit form - coming soon", "/admin/players")
		return
	}

	// Execute the template with data
	if err := renderTemplate(w, tmpl, map[string]interface{}{
		"User":   user,
		"Player": player,
		"Clubs":  clubs,
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

	// Get the existing player
	player, err := h.service.GetPlayerByID(playerID)
	if err != nil {
		logAndError(w, "Player not found", err, http.StatusNotFound)
		return
	}

	// Update player fields from form
	player.FirstName = strings.TrimSpace(r.FormValue("first_name"))
	player.LastName = strings.TrimSpace(r.FormValue("last_name"))
	player.Email = strings.TrimSpace(r.FormValue("email"))
	player.Phone = strings.TrimSpace(r.FormValue("phone"))

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

	// Update the player
	if err := h.service.UpdatePlayer(player); err != nil {
		logAndError(w, "Failed to update player", err, http.StatusInternalServerError)
		return
	}

	// Redirect back to players list
	http.Redirect(w, r, "/admin/players", http.StatusSeeOther)
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
	activeFilter := r.URL.Query().Get("status") // "all", "active", "inactive"

	// Default to showing all players if no filter specified
	if activeFilter == "" {
		activeFilter = "all"
	}

	// Get filtered players from the service
	players, err := h.service.GetFilteredPlayers(query, activeFilter, 1) // Using season 1 for now
	if err != nil {
		logAndError(w, "Failed to load players", err, http.StatusInternalServerError)
		return
	}

	// Return just the table body for HTMX to replace
	w.Header().Set("Content-Type", "text/html")

	// Generate table rows HTML
	if len(players) > 0 {
		for _, playerWithStatus := range players {
			player := playerWithStatus.Player
			// Use actual player status
			activeClass := "player-active"
			if !playerWithStatus.IsActive {
				activeClass = "player-inactive"
			}

			w.Write([]byte(fmt.Sprintf(`
				<tr data-player-id="%s" data-player-name="%s %s" class="%s">
					<td class="col-name">%s %s</td>
					<td class="col-email" title="%s">%s</td>
					<td class="col-phone" title="%s">%s</td>
				</tr>
			`, player.ID, player.FirstName, player.LastName, activeClass,
				player.FirstName, player.LastName,
				player.Email, player.Email,
				player.Phone, player.Phone)))
		}
	} else {
		w.Write([]byte(`
			<tr>
				<td colspan="3" style="text-align: center; padding: 2rem;">
					No players found matching your criteria.
				</td>
			</tr>
		`))
	}
}
