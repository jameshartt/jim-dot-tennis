// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strings"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/version"
)

// Role requirements for every /admin/league route (WI-132), in one place.
//
// RegisterRoutes registers routes on a guardedMux, which refuses (panics at
// startup, and so fails TestEveryAdminRouteHasADeclaredRole) any pattern that
// has no entry in adminRouteRoles. Each registered handler is wrapped so the
// request's role is checked against the entry before the handler runs. The
// same table drives navigation: templates call canUse (common.go funcMap) to
// decide whether to show a link or a form.
//
// Captains can do everything a captain does for every team (the club's
// whole-team ethos: no per-team isolation). Administrators can do all of that
// and run the instance. To add a route: add its pattern here with
// captainRoute, adminRoute or one of the mixed rules, then register it in
// handler.go. Anything that is not clearly captain work is adminRoute.
var adminRouteRoles = map[string]routeAccess{
	// The landing page after sign-in, for both roles.
	"/admin/league/dashboard":  captainRoute,
	"/admin/league/dashboard/": captainRoute,

	// Players and player links (availability links, reminders).
	"/admin/league/players":        captainRoute,
	"/admin/league/players/":       captainRoute,
	"/admin/league/players/filter": captainRoute,

	// Fixtures, rescheduling, selection, results, notes, push notify/remind.
	"/admin/league/fixtures":               captainRoute,
	"/admin/league/fixtures/":              captainRoute,
	"/admin/league/fixtures/week-overview": captainRoute,

	// Accounts and sessions.
	"/admin/league/users":     adminRoute,
	"/admin/league/users/":    adminRoute,
	"/admin/league/sessions":  adminRoute,
	"/admin/league/sessions/": adminRoute,
	"/admin/league/audit-log": adminRoute,

	// Teams: captains see every team and add players to a roster; creating
	// teams and assigning team captains is administration.
	"/admin/league/teams":  captainExcept(teamsAdminActions),
	"/admin/league/teams/": captainExcept(teamsAdminActions),
	// Opponents' teams and clubs: captains read them, administrators change them.
	"/admin/league/teams/away":           captainReadsAdminWrites,
	"/admin/league/teams/away/":          captainReadsAdminWrites,
	"/admin/league/teams/toggle-active/": adminRoute,
	"/admin/league/clubs":                captainReadsAdminWrites,
	"/admin/league/clubs/":               captainReadsAdminWrites,

	// Results: match-card import is captain work.
	"/admin/league/match-card-import":  captainRoute,
	"/admin/league/match-card-import/": captainRoute,

	// Club data import scrapes and rewrites every club's details from BHPLTA.
	"/admin/league/club-data-import":  adminRoute,
	"/admin/league/club-data-import/": adminRoute,

	// Points and standings.
	"/admin/league/points-table":  captainRoute,
	"/admin/league/points-table/": captainRoute,

	// Season Wrapped (club review page): not named for captains.
	"/admin/league/wrapped":  adminRoute,
	"/admin/league/wrapped/": adminRoute,

	// Seasons: creation, activation, deletion, setup, copy, BHPLTA import.
	"/admin/league/seasons":                    adminRoute,
	"/admin/league/seasons/":                   adminRoute,
	"/admin/league/seasons/set-active":         adminRoute,
	"/admin/league/seasons/delete":             adminRoute,
	"/admin/league/seasons/setup":              adminRoute,
	"/admin/league/seasons/move-team":          adminRoute,
	"/admin/league/seasons/copy-from-previous": adminRoute,
	"/admin/league/seasons/copy":               adminRoute,
	"/admin/league/seasons/from-bhplta":        adminRoute,
	"/admin/league/seasons/review-away-teams":  adminRoute,

	// Divisions (league structure).
	"/admin/league/divisions/":       adminRoute,
	"/admin/league/divisions/review": adminRoute,

	// Tournaments (CourtHive providers and sync): not named for captains.
	"/admin/league/tournaments":                    adminRoute,
	"/admin/league/tournaments/providers":          adminRoute,
	"/admin/league/tournaments/providers/":         adminRoute,
	"/admin/league/tournaments/edit/":              adminRoute,
	"/admin/league/tournaments/toggle-visibility/": adminRoute,
	"/admin/league/tournaments/sync/":              adminRoute,

	// Captain planning dashboard, its matrix, and linking yourself to your
	// own player record ("I am...").
	"/admin/league/planning":        captainRoute,
	"/admin/league/planning/":       captainRoute,
	"/admin/league/planning/matrix": captainRoute,
	"/admin/league/planning/cell":   captainRoute,
	"/admin/league/planning/link":   captainRoute,

	// Captain private notes.
	"/admin/league/captain-notes":  captainRoute,
	"/admin/league/captain-notes/": captainRoute,

	// Selection overview.
	"/admin/league/selection-overview":         captainRoute,
	"/admin/league/selection-overview/":        captainRoute,
	"/admin/league/selection-overview/refresh": captainRoute,

	// Players' preferred-name requests (player records).
	"/admin/league/preferred-names":          captainRoute,
	"/admin/league/preferred-names/":         captainRoute,
	"/admin/league/preferred-names/history":  captainRoute,
	"/admin/league/preferred-names/approve/": captainRoute,
	"/admin/league/preferred-names/reject/":  captainRoute,

	// Club settings and feature switches.
	settingsPath:       adminRoute,
	settingsPath + "/": adminRoute,
}

// adminRootPath redirects to the dashboard; anyone who can use the dashboard
// can follow it.
const adminRootPath = "/admin/league"

// signedInRoles are the roles that may enter /admin/league at all; the
// per-route table then narrows it. Player-role accounts are not among them.
var signedInRoles = []models.Role{models.RoleAdmin, models.RoleCaptain}

// teamsAdminActions are the administrator-only requests under the teams
// patterns: creating a team (POST to the list) and assigning or removing a
// team's captains.
func teamsAdminActions(method, path string) bool {
	p := strings.TrimSuffix(path, "/")
	if p == "/admin/league/teams" {
		return isWrite(method)
	}
	return strings.HasSuffix(p, "/add-captain") || strings.HasSuffix(p, "/remove-captain")
}

// routeAccess is the role a route needs.
type routeAccess struct {
	min models.Role
	// adminWhen, if set, marks the requests to a captain route that still
	// need an administrator (patterns that mix captain and admin actions).
	adminWhen func(method, path string) bool
}

var (
	captainRoute = routeAccess{min: models.RoleCaptain}
	adminRoute   = routeAccess{min: models.RoleAdmin}
	// captainReadsAdminWrites: captains may look (GET/HEAD); any change needs
	// an administrator.
	captainReadsAdminWrites = captainExcept(func(method, _ string) bool { return isWrite(method) })
)

// captainExcept is a captain route whose requests matching admin need an
// administrator.
func captainExcept(admin func(method, path string) bool) routeAccess {
	return routeAccess{min: models.RoleCaptain, adminWhen: admin}
}

func isWrite(method string) bool {
	return method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions
}

// required is the role this request needs.
func (a routeAccess) required(method, path string) models.Role {
	if a.adminWhen != nil && a.adminWhen(method, path) {
		return models.RoleAdmin
	}
	return a.min
}

// requireAccess runs next only when the signed-in role meets a; otherwise
// it answers 403 (RequireAuth has already sent anonymous requests to log in).
func requireAccess(a routeAccess, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role, _ := r.Context().Value(auth.RoleContextKey).(models.Role)
		if !models.RoleAtLeast(role, a.required(r.Method, r.URL.Path)) {
			auth.WriteForbidden(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// guardedMux is the /admin/league mux: it only accepts patterns that have a
// declared role, and wraps each handler with that role's check.
type guardedMux struct {
	mux        *http.ServeMux
	roles      map[string]routeAccess
	registered map[string]bool
}

// Test hooks, nil in the server. stubAdminHandlers replaces every registered
// handler so the route and role checks can be exercised without the pages
// behind them; observeAdminMux sees the mux RegisterRoutes builds.
var (
	stubAdminHandlers http.Handler
	observeAdminMux   func(*guardedMux)
)

func newGuardedMux() *guardedMux {
	g := &guardedMux{mux: http.NewServeMux(), roles: adminRouteRoles, registered: map[string]bool{}}
	if observeAdminMux != nil {
		observeAdminMux(g)
	}
	return g
}

// undeclaredRouteMessage explains how to fix a route registered without a role.
func undeclaredRouteMessage(pattern string) string {
	return fmt.Sprintf("admin route %q has no declared role: add it to adminRouteRoles in internal/admin/roles.go (captainRoute or adminRoute)", pattern)
}

// Handle registers h for pattern, guarded by the pattern's declared role.
func (g *guardedMux) Handle(pattern string, h http.Handler) {
	a, ok := g.roles[pattern]
	if !ok {
		panic(undeclaredRouteMessage(pattern))
	}
	if stubAdminHandlers != nil {
		h = stubAdminHandlers
	}
	g.registered[pattern] = true
	g.mux.Handle(pattern, requireAccess(a, h))
}

// HandleFunc registers f for pattern, guarded by the pattern's declared role.
func (g *guardedMux) HandleFunc(pattern string, f func(http.ResponseWriter, *http.Request)) {
	g.Handle(pattern, http.HandlerFunc(f))
}

func (g *guardedMux) ServeHTTP(w http.ResponseWriter, r *http.Request) { g.mux.ServeHTTP(w, r) }

// accessMux matches a path to its pattern in adminRouteRoles the way the
// real mux does, for canUse.
var accessMux = func() *http.ServeMux {
	m := http.NewServeMux()
	patterns := make([]string, 0, len(adminRouteRoles))
	for p := range adminRouteRoles {
		patterns = append(patterns, p)
	}
	sort.Strings(patterns)
	for _, p := range patterns {
		m.Handle(p, http.NotFoundHandler())
	}
	return m
}()

// requiredRoleFor is the role a request with method to path needs, or false
// when no admin route serves the path.
func requiredRoleFor(method, path string) (models.Role, bool) {
	req, err := http.NewRequest(method, path, nil)
	if err != nil {
		return "", false
	}
	_, pattern := accessMux.Handler(req)
	a, ok := adminRouteRoles[pattern]
	if !ok {
		return "", false
	}
	return a.required(method, req.URL.Path), true
}

// NavTemplateFuncs are the role helpers for any template set that parses an
// admin page (the server's top-level template set parses templates/*.html,
// which includes the dashboard).
func NavTemplateFuncs() template.FuncMap {
	return template.FuncMap{"canUse": canUse, "appVersion": version.Short}
}

// canUse is the template helper for navigation: whether user (the page's
// .User) may use target, written "/admin/league/..." (a GET) or
// "POST /admin/league/...". Unknown users and paths are false, so a link
// is hidden rather than shown to someone who would get a 403. The server
// enforces the same table regardless.
func canUse(user interface{}, target string) bool {
	var role models.Role
	switch u := user.(type) {
	case *models.User:
		if u == nil {
			return false
		}
		role = u.Role
	case models.User:
		role = u.Role
	default:
		return false
	}
	method, path := http.MethodGet, strings.TrimSpace(target)
	if m, p, ok := strings.Cut(path, " "); ok {
		method, path = strings.ToUpper(m), strings.TrimSpace(p)
	}
	need, ok := requiredRoleFor(method, path)
	return ok && models.RoleAtLeast(role, need)
}
