package admin

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
)

// Handler represents the admin handler
type Handler struct {
	service     *Service
	templateDir string
}

// New creates a new admin handler
func New(db *database.DB, templateDir string) *Handler {
	return &Handler{
		service:     NewService(db),
		templateDir: templateDir,
	}
}

// RegisterRoutes registers all admin routes with the given mux
func (h *Handler) RegisterRoutes(mux *http.ServeMux, authMiddleware *auth.Middleware) {
	// Create admin mux for all admin routes
	adminMux := http.NewServeMux()

	// Dashboard route with full path
	adminMux.HandleFunc("/admin/dashboard", h.handleDashboard)
	adminMux.HandleFunc("/admin/dashboard/", h.handleDashboard)

	// Admin area routes with full paths
	adminMux.HandleFunc("/admin/players", h.handlePlayers)
	adminMux.HandleFunc("/admin/players/", h.handlePlayers)
	adminMux.HandleFunc("/admin/players/filter", h.handlePlayersFilter)
	adminMux.HandleFunc("/admin/fixtures", h.handleFixtures)
	adminMux.HandleFunc("/admin/fixtures/", h.handleFixtures)
	adminMux.HandleFunc("/admin/users", h.handleUsers)
	adminMux.HandleFunc("/admin/users/", h.handleUsers)
	adminMux.HandleFunc("/admin/sessions", h.handleSessions)
	adminMux.HandleFunc("/admin/sessions/", h.handleSessions)
	adminMux.HandleFunc("/admin/teams", h.handleTeams)
	adminMux.HandleFunc("/admin/teams/", h.handleTeams)

	// Register admin routes with authentication middleware
	mux.Handle("/admin", authMiddleware.RequireAuth(
		authMiddleware.RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Redirect /admin to /admin/dashboard
			log.Printf("Redirecting to /admin/dashboard")
			http.Redirect(w, r, "/admin/dashboard", http.StatusFound)
		})),
	))
	mux.Handle("/admin/", authMiddleware.RequireAuth(
		authMiddleware.RequireRole("admin")(adminMux),
	))
}

// handleDashboard serves the main admin dashboard
func (h *Handler) handleDashboard(w http.ResponseWriter, r *http.Request) {
	log.Printf("Admin dashboard handler called with path: %s", r.URL.Path)

	// Only handle dashboard path
	if r.URL.Path != "/admin/dashboard" {
		log.Printf("Dashboard handler: not found for path: %s", r.URL.Path)
		http.NotFound(w, r)
		return
	}

	// Get user from context
	user, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		log.Printf("Failed to get user from context: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	log.Printf("Admin dashboard requested by user: %s (role: %s)", user.Username, user.Role)

	// Get dashboard data
	dashboardData, err := h.service.GetDashboardData(&user)
	if err != nil {
		log.Printf("Failed to get dashboard data: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Load the standalone admin template
	adminTemplatePath := filepath.Join(h.templateDir, "admin_standalone.html")
	tmpl, err := template.ParseFiles(adminTemplatePath)
	if err != nil {
		log.Printf("Error parsing standalone admin template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Prepare template data
	templateData := map[string]interface{}{
		"User":          user,
		"Stats":         dashboardData.Stats,
		"LoginAttempts": dashboardData.LoginAttempts,
	}

	// Execute the template
	if err := tmpl.Execute(w, templateData); err != nil {
		log.Printf("Error executing admin dashboard template: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handlePlayers handles player management routes
func (h *Handler) handlePlayers(w http.ResponseWriter, r *http.Request) {
	log.Printf("Admin players handler called with path: %s, method: %s", r.URL.Path, r.Method)

	// Check if this is an edit request
	if strings.Contains(r.URL.Path, "/edit") {
		h.handlePlayerEdit(w, r)
		return
	}

	// Get user from context
	user, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		log.Printf("Failed to get user from context: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handlePlayersGet(w, r, &user)
	case http.MethodPost:
		h.handlePlayersPost(w, r, &user)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleFixtures handles fixture management routes
func (h *Handler) handleFixtures(w http.ResponseWriter, r *http.Request) {
	log.Printf("Admin fixtures handler called with path: %s, method: %s", r.URL.Path, r.Method)

	// Check if this is a specific fixture detail request
	if strings.Contains(r.URL.Path, "/fixtures/") && r.URL.Path != "/admin/fixtures/" {
		h.handleFixtureDetail(w, r)
		return
	}

	// Get user from context
	user, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		log.Printf("Failed to get user from context: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleFixturesGet(w, r, &user)
	case http.MethodPost:
		h.handleFixturesPost(w, r, &user)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleUsers handles user management routes
func (h *Handler) handleUsers(w http.ResponseWriter, r *http.Request) {
	log.Printf("Admin users handler called with path: %s, method: %s", r.URL.Path, r.Method)

	// Get user from context
	user, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		log.Printf("Failed to get user from context: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleUsersGet(w, r, &user)
	case http.MethodPost:
		h.handleUsersPost(w, r, &user)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSessions handles session management routes
func (h *Handler) handleSessions(w http.ResponseWriter, r *http.Request) {
	log.Printf("Admin sessions handler called with path: %s, method: %s", r.URL.Path, r.Method)

	// Get user from context
	user, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		log.Printf("Failed to get user from context: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleSessionsGet(w, r, &user)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSessionsGet handles GET requests for sessions management
func (h *Handler) handleSessionsGet(w http.ResponseWriter, r *http.Request, user *models.User) {
	// TODO: Implement sessions view
	// For now, render a simple placeholder page
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`
	<!DOCTYPE html>
	<html>
	<head><title>Admin - Sessions</title></head>
	<body>
		<h1>Session Management</h1>
		<p>Sessions management page - coming soon</p>
		<a href="/admin">Back to Dashboard</a>
	</body>
	</html>
	`))
}

// handleTeams handles team management routes
func (h *Handler) handleTeams(w http.ResponseWriter, r *http.Request) {
	log.Printf("Admin teams handler called with path: %s, method: %s", r.URL.Path, r.Method)

	// Get user from context
	user, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		log.Printf("Failed to get user from context: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleTeamsGet(w, r, &user)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleTeamsGet handles GET requests for team management
func (h *Handler) handleTeamsGet(w http.ResponseWriter, r *http.Request, user *models.User) {
	// TODO: Implement teams view
	// For now, render a simple placeholder page
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`
	<!DOCTYPE html>
	<html>
	<head><title>Admin - Teams</title></head>
	<body>
		<h1>Team Management</h1>
		<p>Team management page - coming soon</p>
		<a href="/admin">Back to Dashboard</a>
	</body>
	</html>
	`))
}

// handlePlayersGet handles GET requests for player management
func (h *Handler) handlePlayersGet(w http.ResponseWriter, r *http.Request, user *models.User) {
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
		log.Printf("Failed to get filtered players: %v", err)
		http.Error(w, "Failed to load players", http.StatusInternalServerError)
		return
	}

	// Load the players template
	playersTemplatePath := filepath.Join(h.templateDir, "admin", "players.html")
	tmpl, err := template.ParseFiles(playersTemplatePath)
	if err != nil {
		log.Printf("Error parsing players template: %v", err)
		// Fallback to simple HTML response
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`
		<!DOCTYPE html>
		<html>
		<head><title>Admin - Players</title></head>
		<body>
			<h1>Player Management</h1>
			<p>Player management page - coming soon</p>
			<a href="/admin">Back to Dashboard</a>
		</body>
		</html>
		`))
		return
	}

	// Execute the template with data
	if err := tmpl.Execute(w, map[string]interface{}{
		"User":         user,
		"Players":      players,
		"SearchQuery":  query,
		"ActiveFilter": activeFilter,
	}); err != nil {
		log.Printf("Error executing players template: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handlePlayersPost handles POST requests for player management
func (h *Handler) handlePlayersPost(w http.ResponseWriter, r *http.Request, user *models.User) {
	// TODO: Implement player creation/update/delete
	http.Error(w, "Player operations not yet implemented", http.StatusNotImplemented)
}

// handleFixturesGet handles GET requests for fixture management
func (h *Handler) handleFixturesGet(w http.ResponseWriter, r *http.Request, user *models.User) {
	// Get St. Ann's fixtures with related data
	club, fixtures, err := h.service.GetStAnnsFixtures()
	if err != nil {
		log.Printf("Failed to get St. Ann's fixtures: %v", err)
		http.Error(w, "Failed to load fixtures", http.StatusInternalServerError)
		return
	}

	// Load the fixtures template
	fixturesTemplatePath := filepath.Join(h.templateDir, "admin", "fixtures.html")
	tmpl, err := template.ParseFiles(fixturesTemplatePath)
	if err != nil {
		log.Printf("Error parsing fixtures template: %v", err)
		// Fallback to simple HTML response
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`
		<!DOCTYPE html>
		<html>
		<head><title>Admin - Fixtures</title></head>
		<body>
			<h1>Fixture Management</h1>
			<p>Fixture management page - coming soon</p>
			<a href="/admin">Back to Dashboard</a>
		</body>
		</html>
		`))
		return
	}

	// Execute the template with data
	if err := tmpl.Execute(w, map[string]interface{}{
		"User":     user,
		"Club":     club,
		"Fixtures": fixtures,
	}); err != nil {
		log.Printf("Error executing fixtures template: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleFixturesPost handles POST requests for fixture management
func (h *Handler) handleFixturesPost(w http.ResponseWriter, r *http.Request, user *models.User) {
	// TODO: Implement fixture creation/update/delete
	http.Error(w, "Fixture operations not yet implemented", http.StatusNotImplemented)
}

// handleUsersGet handles GET requests for user management
func (h *Handler) handleUsersGet(w http.ResponseWriter, r *http.Request, user *models.User) {
	// TODO: Implement users view
	// For now, render a simple placeholder page
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`
	<!DOCTYPE html>
	<html>
	<head><title>Admin - Users</title></head>
	<body>
		<h1>User Management</h1>
		<p>User management page - coming soon</p>
		<a href="/admin">Back to Dashboard</a>
	</body>
	</html>
	`))
}

// handleUsersPost handles POST requests for user management
func (h *Handler) handleUsersPost(w http.ResponseWriter, r *http.Request, user *models.User) {
	// TODO: Implement user creation/update/delete
	http.Error(w, "User operations not yet implemented", http.StatusNotImplemented)
}

// handlePlayerEdit handles GET requests for editing a player
func (h *Handler) handlePlayerEdit(w http.ResponseWriter, r *http.Request) {
	log.Printf("Admin player edit handler called with path: %s, method: %s", r.URL.Path, r.Method)

	// Get user from context
	user, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		log.Printf("Failed to get user from context: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
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
		h.handlePlayerEditGet(w, r, &user, playerID)
	case http.MethodPost:
		h.handlePlayerEditPost(w, r, &user, playerID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handlePlayerEditGet handles GET requests to show the player edit form
func (h *Handler) handlePlayerEditGet(w http.ResponseWriter, r *http.Request, user *models.User, playerID string) {
	// Get the player by ID
	player, err := h.service.GetPlayerByID(playerID)
	if err != nil {
		log.Printf("Failed to get player by ID %s: %v", playerID, err)
		http.Error(w, "Player not found", http.StatusNotFound)
		return
	}

	// Get all clubs for the dropdown
	clubs, err := h.service.GetClubs()
	if err != nil {
		log.Printf("Failed to get clubs: %v", err)
		http.Error(w, "Failed to load clubs", http.StatusInternalServerError)
		return
	}

	// Load the player edit template
	editTemplatePath := filepath.Join(h.templateDir, "admin", "player_edit.html")
	tmpl, err := template.ParseFiles(editTemplatePath)
	if err != nil {
		log.Printf("Error parsing player edit template: %v", err)
		// Fallback to simple HTML response
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`
		<!DOCTYPE html>
		<html>
		<head><title>Edit Player</title></head>
		<body>
			<h1>Edit Player</h1>
			<p>Player edit form - coming soon</p>
			<a href="/admin/players">Back to Players</a>
		</body>
		</html>
		`))
		return
	}

	// Execute the template with data
	if err := tmpl.Execute(w, map[string]interface{}{
		"User":   user,
		"Player": player,
		"Clubs":  clubs,
	}); err != nil {
		log.Printf("Error executing player edit template: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handlePlayerEditPost handles POST requests to update a player
func (h *Handler) handlePlayerEditPost(w http.ResponseWriter, r *http.Request, user *models.User, playerID string) {
	// Parse form data
	if err := r.ParseForm(); err != nil {
		log.Printf("Failed to parse form data: %v", err)
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	// Get the existing player
	player, err := h.service.GetPlayerByID(playerID)
	if err != nil {
		log.Printf("Failed to get player by ID %s: %v", playerID, err)
		http.Error(w, "Player not found", http.StatusNotFound)
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
			log.Printf("Invalid club ID: %s", clubIDStr)
			http.Error(w, "Invalid club ID", http.StatusBadRequest)
			return
		}
		player.ClubID = uint(clubID)
	}

	// Update the player
	if err := h.service.UpdatePlayer(player); err != nil {
		log.Printf("Failed to update player: %v", err)
		http.Error(w, "Failed to update player", http.StatusInternalServerError)
		return
	}

	// Redirect back to players list
	http.Redirect(w, r, "/admin/players", http.StatusSeeOther)
}

// handlePlayersFilter handles HTMX requests for filtering players
func (h *Handler) handlePlayersFilter(w http.ResponseWriter, r *http.Request) {
	// Get user from context for authentication
	_, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		log.Printf("Failed to get user from context: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
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
		log.Printf("Failed to get filtered players: %v", err)
		http.Error(w, "Failed to load players", http.StatusInternalServerError)
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

// handleFixtureDetail handles requests for individual fixture details
func (h *Handler) handleFixtureDetail(w http.ResponseWriter, r *http.Request) {
	log.Printf("Admin fixture detail handler called with path: %s, method: %s", r.URL.Path, r.Method)

	// Get user from context
	user, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		log.Printf("Failed to get user from context: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Extract fixture ID from URL path
	// Path format: /admin/fixtures/{id}
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/admin/fixtures/"), "/")
	if len(pathParts) < 1 || pathParts[0] == "" {
		http.Error(w, "Invalid fixture URL", http.StatusBadRequest)
		return
	}
	fixtureIDStr := pathParts[0]

	// Convert fixture ID to uint
	fixtureID, err := strconv.ParseUint(fixtureIDStr, 10, 32)
	if err != nil {
		log.Printf("Invalid fixture ID: %s", fixtureIDStr)
		http.Error(w, "Invalid fixture ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleFixtureDetailGet(w, r, &user, uint(fixtureID))
	case http.MethodPost:
		h.handleFixtureDetailPost(w, r, &user, uint(fixtureID))
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleFixtureDetailGet handles GET requests to show the fixture detail page
func (h *Handler) handleFixtureDetailGet(w http.ResponseWriter, r *http.Request, user *models.User, fixtureID uint) {
	// Get the fixture with full details
	fixtureDetail, err := h.service.GetFixtureDetail(fixtureID)
	if err != nil {
		log.Printf("Failed to get fixture detail for ID %d: %v", fixtureID, err)
		http.Error(w, "Fixture not found", http.StatusNotFound)
		return
	}

	// Load the fixture detail template
	detailTemplatePath := filepath.Join(h.templateDir, "admin", "fixture_detail.html")
	tmpl, err := template.ParseFiles(detailTemplatePath)
	if err != nil {
		log.Printf("Error parsing fixture detail template: %v", err)
		// Fallback to simple HTML response
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`
		<!DOCTYPE html>
		<html>
		<head><title>Fixture Detail</title></head>
		<body>
			<h1>Fixture Detail</h1>
			<p>Fixture detail page - coming soon</p>
			<a href="/admin/fixtures">Back to Fixtures</a>
		</body>
		</html>
		`))
		return
	}

	// Execute the template with data
	if err := tmpl.Execute(w, map[string]interface{}{
		"User":          user,
		"FixtureDetail": fixtureDetail,
	}); err != nil {
		log.Printf("Error executing fixture detail template: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleFixtureDetailPost handles POST requests to update fixture details
func (h *Handler) handleFixtureDetailPost(w http.ResponseWriter, r *http.Request, user *models.User, fixtureID uint) {
	// TODO: Implement fixture detail updates
	http.Error(w, "Fixture detail updates not yet implemented", http.StatusNotImplemented)
}
