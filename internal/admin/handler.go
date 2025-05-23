package admin

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"

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

	// Dashboard route
	adminMux.HandleFunc("/", h.handleDashboard)

	// Admin area routes
	adminMux.HandleFunc("/players", h.handlePlayers)
	adminMux.HandleFunc("/players/", h.handlePlayers)
	adminMux.HandleFunc("/fixtures", h.handleFixtures)
	adminMux.HandleFunc("/fixtures/", h.handleFixtures)
	adminMux.HandleFunc("/users", h.handleUsers)
	adminMux.HandleFunc("/users/", h.handleUsers)
	adminMux.HandleFunc("/sessions", h.handleSessions)
	adminMux.HandleFunc("/sessions/", h.handleSessions)

	// Register admin routes with authentication middleware
	mux.Handle("/admin", authMiddleware.RequireAuth(
		authMiddleware.RequireRole("admin")(adminMux),
	))
	mux.Handle("/admin/", authMiddleware.RequireAuth(
		authMiddleware.RequireRole("admin")(adminMux),
	))
}

// handleDashboard serves the main admin dashboard
func (h *Handler) handleDashboard(w http.ResponseWriter, r *http.Request) {
	log.Printf("Admin dashboard handler called with path: %s", r.URL.Path)

	// Only handle root admin paths
	if r.URL.Path != "/admin/" && r.URL.Path != "/admin" {
		log.Printf("Not found for path: %s", r.URL.Path)
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

// handlePlayersGet handles GET requests for player management
func (h *Handler) handlePlayersGet(w http.ResponseWriter, r *http.Request, user *models.User) {
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
		"User": user,
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
	// TODO: Implement fixtures view
	// For now, render a simple placeholder page
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
