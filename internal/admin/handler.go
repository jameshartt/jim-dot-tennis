package admin

import (
	"log"
	"net/http"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/database"
)

// Handler represents the admin handler
type Handler struct {
	service     *Service
	templateDir string

	// Sub-handlers for different domains
	dashboard *DashboardHandler
	players   *PlayersHandler
	fixtures  *FixturesHandler
	teams     *TeamsHandler
	users     *UsersHandler
	sessions  *SessionsHandler
}

// New creates a new admin handler
func New(db *database.DB, templateDir string) *Handler {
	service := NewService(db)

	return &Handler{
		service:     service,
		templateDir: templateDir,
		dashboard:   NewDashboardHandler(service, templateDir),
		players:     NewPlayersHandler(service, templateDir),
		fixtures:    NewFixturesHandler(service, templateDir),
		teams:       NewTeamsHandler(service, templateDir),
		users:       NewUsersHandler(service, templateDir),
		sessions:    NewSessionsHandler(service, templateDir),
	}
}

// RegisterRoutes registers all admin routes with the given mux
func (h *Handler) RegisterRoutes(mux *http.ServeMux, authMiddleware *auth.Middleware) {
	// Create admin mux for all admin routes
	adminMux := http.NewServeMux()

	// Dashboard route with full path
	adminMux.HandleFunc("/admin/dashboard", h.dashboard.HandleDashboard)
	adminMux.HandleFunc("/admin/dashboard/", h.dashboard.HandleDashboard)

	// Admin area routes with full paths
	adminMux.HandleFunc("/admin/players", h.players.HandlePlayers)
	adminMux.HandleFunc("/admin/players/", h.players.HandlePlayers)
	adminMux.HandleFunc("/admin/players/filter", h.players.HandlePlayersFilter)
	adminMux.HandleFunc("/admin/fixtures", h.fixtures.HandleFixtures)
	adminMux.HandleFunc("/admin/fixtures/", h.fixtures.HandleFixtures)
	adminMux.HandleFunc("/admin/users", h.users.HandleUsers)
	adminMux.HandleFunc("/admin/users/", h.users.HandleUsers)
	adminMux.HandleFunc("/admin/sessions", h.sessions.HandleSessions)
	adminMux.HandleFunc("/admin/sessions/", h.sessions.HandleSessions)
	adminMux.HandleFunc("/admin/teams", h.teams.HandleTeams)
	adminMux.HandleFunc("/admin/teams/", h.teams.HandleTeams)

	// Preferred name approval routes
	adminMux.HandleFunc("/admin/preferred-names", h.service.HandlePreferredNameApprovals)
	adminMux.HandleFunc("/admin/preferred-names/", h.service.HandlePreferredNameApprovals)
	adminMux.HandleFunc("/admin/preferred-names/history", h.service.HandlePreferredNameHistory)
	adminMux.HandleFunc("/admin/preferred-names/approve/", h.service.HandleApprovePreferredName)
	adminMux.HandleFunc("/admin/preferred-names/reject/", h.service.HandleRejectPreferredName)

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
