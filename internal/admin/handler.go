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
	dashboard         *DashboardHandler
	players           *PlayersHandler
	fixtures          *FixturesHandler
	teams             *TeamsHandler
	users             *UsersHandler
	sessions          *SessionsHandler
	matchCardImport   *MatchCardImportationHandler
	points            *PointsHandler
	clubWrapped       *ClubWrappedHandler
	seasons           *SeasonsHandler
	seasonSetup       *SeasonSetupHandler
	selectionOverview *SelectionOverviewHandler
}

// New creates a new admin handler
func New(db *database.DB, templateDir string) *Handler {
	service := NewService(db)

	return &Handler{
		service:           service,
		templateDir:       templateDir,
		dashboard:         NewDashboardHandler(service, templateDir),
		players:           NewPlayersHandler(service, templateDir),
		fixtures:          NewFixturesHandler(service, templateDir),
		teams:             NewTeamsHandler(service, templateDir),
		users:             NewUsersHandler(service, templateDir),
		sessions:          NewSessionsHandler(service, templateDir),
		matchCardImport:   NewMatchCardImportationHandler(service, templateDir),
		points:            NewPointsHandler(service, templateDir),
		clubWrapped:       NewClubWrappedHandler(service, templateDir),
		seasons:           NewSeasonsHandler(service, templateDir),
		seasonSetup:       NewSeasonSetupHandler(service, templateDir),
		selectionOverview: NewSelectionOverviewHandler(service, templateDir),
	}
}

// RegisterRoutes registers all admin routes with the given mux
func (h *Handler) RegisterRoutes(mux *http.ServeMux, authMiddleware *auth.Middleware) {
	// Create admin mux for all admin routes
	adminMux := http.NewServeMux()

	// Dashboard route with full path
	adminMux.HandleFunc("/admin/league/dashboard", h.dashboard.HandleDashboard)
	adminMux.HandleFunc("/admin/league/dashboard/", h.dashboard.HandleDashboard)

	// Admin area routes with full paths
	adminMux.HandleFunc("/admin/league/players", h.players.HandlePlayers)
	adminMux.HandleFunc("/admin/league/players/", h.players.HandlePlayers)
	adminMux.HandleFunc("/admin/league/players/filter", h.players.HandlePlayersFilter)
	adminMux.HandleFunc("/admin/league/fixtures", h.fixtures.HandleFixtures)
	adminMux.HandleFunc("/admin/league/fixtures/", h.fixtures.HandleFixtures)
	adminMux.HandleFunc("/admin/league/fixtures/week-overview", h.fixtures.HandleFixtures)
	adminMux.HandleFunc("/admin/league/users", h.users.HandleUsers)
	adminMux.HandleFunc("/admin/league/users/", h.users.HandleUsers)
	adminMux.HandleFunc("/admin/league/sessions", h.sessions.HandleSessions)
	adminMux.HandleFunc("/admin/league/sessions/", h.sessions.HandleSessions)
	adminMux.HandleFunc("/admin/league/teams", h.teams.HandleTeams)
	adminMux.HandleFunc("/admin/league/teams/", h.teams.HandleTeams)

	// Match card import routes
	adminMux.HandleFunc("/admin/league/match-card-import", h.matchCardImport.HandleMatchCardImportation)
	adminMux.HandleFunc("/admin/league/match-card-import/", h.matchCardImport.HandleMatchCardImportation)

	// Points table route
	adminMux.HandleFunc("/admin/league/points-table", h.points.HandlePointsTable)
	adminMux.HandleFunc("/admin/league/points-table/", h.points.HandlePointsTable)

	// Season wrapped routes
	adminMux.HandleFunc("/admin/league/wrapped", h.clubWrapped.HandleWrapped)
	adminMux.HandleFunc("/admin/league/wrapped/", h.clubWrapped.HandleWrapped)

	// Season management routes
	adminMux.HandleFunc("/admin/league/seasons", h.seasons.HandleSeasons)
	adminMux.HandleFunc("/admin/league/seasons/", h.seasons.HandleSeasons)
	adminMux.HandleFunc("/admin/league/seasons/set-active", h.seasons.HandleSetActiveSeason)
	adminMux.HandleFunc("/admin/league/seasons/setup", h.seasonSetup.HandleSeasonSetup)
	adminMux.HandleFunc("/admin/league/seasons/move-team", h.seasonSetup.HandleMoveTeam)
	adminMux.HandleFunc("/admin/league/seasons/copy-from-previous", h.seasonSetup.HandleCopyFromPreviousSeason)

	// Selection overview routes
	adminMux.HandleFunc("/admin/league/selection-overview", h.selectionOverview.HandleSelectionOverview)
	adminMux.HandleFunc("/admin/league/selection-overview/", h.selectionOverview.HandleSelectionOverview)
	adminMux.HandleFunc("/admin/league/selection-overview/refresh", h.selectionOverview.HandleSelectionOverview)

	// Preferred name approval routes
	adminMux.HandleFunc("/admin/league/preferred-names", h.service.HandlePreferredNameApprovals)
	adminMux.HandleFunc("/admin/league/preferred-names/", h.service.HandlePreferredNameApprovals)
	adminMux.HandleFunc("/admin/league/preferred-names/history", h.service.HandlePreferredNameHistory)
	adminMux.HandleFunc("/admin/league/preferred-names/approve/", h.service.HandleApprovePreferredName)
	adminMux.HandleFunc("/admin/league/preferred-names/reject/", h.service.HandleRejectPreferredName)

	// Register admin routes with authentication middleware
	mux.Handle("/admin/league", authMiddleware.RequireAuth(
		authMiddleware.RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Redirect /admin/league to /admin/league/dashboard
			log.Printf("Redirecting to /admin/league/dashboard")
			http.Redirect(w, r, "/admin/league/dashboard", http.StatusFound)
		})),
	))
	mux.Handle("/admin/league/", authMiddleware.RequireAuth(
		authMiddleware.RequireRole("admin")(adminMux),
	))
}

// RegisterPublicRoutes registers public-facing admin-related routes (no admin auth)
func (h *Handler) RegisterPublicRoutes(mux *http.ServeMux) {
	// Public Season Wrapped route protected by a lightweight access cookie
	mux.HandleFunc("/club/wrapped", h.clubWrapped.HandlePublicWrapped)
}
