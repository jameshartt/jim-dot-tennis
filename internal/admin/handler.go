// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"log"
	"net/http"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/config"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/webpush"
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
	auditLog          *AuditLogHandler
	matchCardImport   *MatchCardImportationHandler
	points            *PointsHandler
	clubWrapped       *ClubWrappedHandler
	clubs             *ClubsHandler
	clubImport        *ClubImportHandler
	seasons           *SeasonsHandler
	seasonSetup       *SeasonSetupHandler
	selectionOverview *SelectionOverviewHandler
	divisions         *DivisionsHandler
	tournaments       *TournamentsHandler
	planning          *PlanningHandler
	planningLink      *PlanningLinkHandler
	captainNotes      *CaptainNotesHandler
	settings          *SettingsHandler
}

// New creates a new admin handler
func New(db *database.DB, templateDir string, courthiveAPIURL string, homeClub config.HomeClubSource, bhpltaClubCode string, pushService ...*webpush.Service) *Handler {
	service := NewService(db, courthiveAPIURL, homeClub, bhpltaClubCode, pushService...)
	service.templateDir = templateDir

	var ps *webpush.Service
	if len(pushService) > 0 {
		ps = pushService[0]
	}

	return &Handler{
		service:           service,
		templateDir:       templateDir,
		dashboard:         NewDashboardHandler(service, templateDir),
		players:           NewPlayersHandler(service, templateDir),
		fixtures:          NewFixturesHandler(service, templateDir, ps),
		teams:             NewTeamsHandler(service, templateDir),
		users:             NewUsersHandler(service, templateDir),
		sessions:          NewSessionsHandler(service, templateDir),
		auditLog:          NewAuditLogHandler(service, templateDir),
		matchCardImport:   NewMatchCardImportationHandler(service, templateDir),
		points:            NewPointsHandler(service, templateDir),
		clubWrapped:       NewClubWrappedHandler(service, templateDir),
		clubs:             NewClubsHandler(service, templateDir),
		clubImport:        NewClubImportHandler(service, templateDir),
		seasons:           NewSeasonsHandler(service, templateDir),
		seasonSetup:       NewSeasonSetupHandler(service, templateDir),
		selectionOverview: NewSelectionOverviewHandler(service, templateDir),
		divisions:         NewDivisionsHandler(service, templateDir),
		tournaments:       NewTournamentsHandler(service, templateDir),
		planning:          NewPlanningHandler(service, templateDir),
		planningLink:      NewPlanningLinkHandler(service, templateDir),
		captainNotes:      NewCaptainNotesHandler(service, templateDir),
		settings:          NewSettingsHandler(service, templateDir),
	}
}

// RegisterRoutes registers all admin routes with the given mux
// SetPasskeysAvailable tells the settings page how to find out whether
// passkeys work at a request's address (auth.Passkeys.Available).
func (h *Handler) SetPasskeysAvailable(available func(*http.Request) bool) {
	h.service.passkeysAvailable = available
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, authMiddleware *auth.Middleware) {
	// Create admin mux for all admin routes. Every pattern must have a role
	// declared in adminRouteRoles (roles.go); the mux enforces it per request.
	adminMux := newGuardedMux()

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
	adminMux.HandleFunc(auditLogPath, h.auditLog.HandleAuditLog) // administrators only (also checked in the handler)
	adminMux.HandleFunc("/admin/league/teams", h.teams.HandleTeams)
	adminMux.HandleFunc("/admin/league/teams/", h.teams.HandleTeams)
	adminMux.HandleFunc("/admin/league/teams/away", h.teams.HandleAwayTeams)
	adminMux.HandleFunc("/admin/league/teams/away/", h.teams.HandleAwayTeams)
	adminMux.HandleFunc("/admin/league/teams/toggle-active/", h.teams.HandleToggleActive)
	adminMux.HandleFunc("/admin/league/clubs", h.clubs.HandleClubs)
	adminMux.HandleFunc("/admin/league/clubs/", h.clubs.HandleClubs)

	// Match card import routes
	adminMux.HandleFunc("/admin/league/match-card-import", h.matchCardImport.HandleMatchCardImportation)
	adminMux.HandleFunc("/admin/league/match-card-import/", h.matchCardImport.HandleMatchCardImportation)

	// Club data import routes
	adminMux.HandleFunc("/admin/league/club-data-import", h.clubImport.HandleClubImport)
	adminMux.HandleFunc("/admin/league/club-data-import/", h.clubImport.HandleClubImport)

	// Points table route
	adminMux.HandleFunc("/admin/league/points-table", h.points.HandlePointsTable)
	adminMux.HandleFunc("/admin/league/points-table/", h.points.HandlePointsTable)

	// Season wrapped routes
	adminMux.HandleFunc("/admin/league/wrapped", h.wrappedOnly(h.clubWrapped.HandleWrapped))
	adminMux.HandleFunc("/admin/league/wrapped/", h.wrappedOnly(h.clubWrapped.HandleWrapped))

	// Season management routes
	adminMux.HandleFunc("/admin/league/seasons", h.seasons.HandleSeasons)
	adminMux.HandleFunc("/admin/league/seasons/", h.seasons.HandleSeasons)
	adminMux.HandleFunc("/admin/league/seasons/set-active", h.seasons.HandleSetActiveSeason)
	adminMux.HandleFunc("/admin/league/seasons/delete", h.seasons.HandleDeleteSeason)
	adminMux.HandleFunc("/admin/league/seasons/setup", h.seasonSetup.HandleSeasonSetup)
	adminMux.HandleFunc("/admin/league/seasons/move-team", h.seasonSetup.HandleMoveTeam)
	adminMux.HandleFunc("/admin/league/seasons/copy-from-previous", h.seasonSetup.HandleCopyFromPreviousSeason)
	adminMux.HandleFunc("/admin/league/seasons/copy", h.seasonSetup.HandleSeasonCopy)
	adminMux.HandleFunc("/admin/league/seasons/from-bhplta", h.seasonSetup.HandleStartFromBHPLTA)
	adminMux.HandleFunc("/admin/league/seasons/review-away-teams", h.seasonSetup.HandleReviewAwayTeams)

	// Division management routes
	adminMux.HandleFunc("/admin/league/divisions/", h.divisions.HandleDivisionEdit)
	adminMux.HandleFunc("/admin/league/divisions/review", h.teams.HandleDivisionReview)

	// Tournament management routes
	adminMux.HandleFunc("/admin/league/tournaments", h.tournamentsOnly(h.tournaments.HandleTournaments))
	adminMux.HandleFunc("/admin/league/tournaments/providers", h.tournamentsOnly(h.tournaments.HandleProviders))
	adminMux.HandleFunc("/admin/league/tournaments/providers/", h.tournamentsOnly(h.tournaments.HandleProviderEdit))
	adminMux.HandleFunc("/admin/league/tournaments/edit/", h.tournamentsOnly(h.tournaments.HandleTournamentEdit))
	adminMux.HandleFunc("/admin/league/tournaments/toggle-visibility/", h.tournamentsOnly(h.tournaments.HandleToggleVisibility))
	adminMux.HandleFunc("/admin/league/tournaments/sync/", h.tournamentsOnly(h.tournaments.HandleSync))

	// Captain planning dashboard (Sprint 017)
	adminMux.HandleFunc("/admin/league/planning", h.planning.HandleDashboard)
	adminMux.HandleFunc("/admin/league/planning/", h.planning.HandleDashboard)
	adminMux.HandleFunc("/admin/league/planning/matrix", h.planning.HandleDashboard)
	adminMux.HandleFunc("/admin/league/planning/cell", h.planning.HandleCellToggle)
	adminMux.HandleFunc("/admin/league/planning/link", h.planningLink.HandleLink)

	// Captain private notes (Sprint 017 WI-105). Admin-session only; this
	// surface is explicitly forbidden from any token-authenticated mount.
	adminMux.HandleFunc("/admin/league/captain-notes", h.captainNotes.HandleNotes)
	adminMux.HandleFunc("/admin/league/captain-notes/", h.captainNotes.HandleNotes)

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

	// Club settings (admin role, declared in roles.go and checked again in the handler)
	adminMux.HandleFunc(settingsPath, h.settings.HandleSettings)
	adminMux.HandleFunc(settingsPath+"/", h.settings.HandleSettings)

	// Register admin routes with authentication middleware. Captains and
	// administrators may enter; adminMux then checks each route's role.
	mux.Handle(adminRootPath, authMiddleware.RequireAuth(
		authMiddleware.RequireRole(signedInRoles...)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Redirect /admin/league to /admin/league/dashboard
			log.Printf("Redirecting to /admin/league/dashboard")
			http.Redirect(w, r, "/admin/league/dashboard", http.StatusFound)
		})),
	))
	mux.Handle("/admin/league/", authMiddleware.RequireAuth(
		authMiddleware.RequireRole(signedInRoles...)(adminMux),
	))
}

// RegisterPublicRoutes registers public-facing admin-related routes (no admin auth)
func (h *Handler) RegisterPublicRoutes(mux *http.ServeMux) {
	// Public Season Wrapped route protected by a lightweight access cookie
	mux.HandleFunc("/club/wrapped", h.wrappedOnly(h.clubWrapped.HandlePublicWrapped))
}

// tournamentsOnly serves next only while the tournaments switch is on; with it
// off the route does not exist (404).
func (h *Handler) tournamentsOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.service.clubSettings.TournamentsEnabled() {
			http.NotFound(w, r)
			return
		}
		next(w, r)
	}
}

// wrappedOnly serves next only while the Club Wrapped switch is on (404 otherwise).
func (h *Handler) wrappedOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.service.clubSettings.ClubWrappedEnabled() {
			http.NotFound(w, r)
			return
		}
		next(w, r)
	}
}
