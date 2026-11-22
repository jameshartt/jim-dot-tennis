// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
)

// registerAdminRoutes runs Handler.RegisterRoutes the way the server does and
// returns the outer mux and the guarded /admin/league mux it built. A route
// registered without a declared role fails the test with the panic message,
// which names the route.
func registerAdminRoutes(t *testing.T, e settingsTestEnv, mw *auth.Middleware) (*http.ServeMux, *guardedMux) {
	t.Helper()
	var built *guardedMux
	observeAdminMux = func(g *guardedMux) { built = g }
	t.Cleanup(func() { observeAdminMux = nil })
	h := New(e.db, findTemplatesPathAdmin(t), "", e.holder, "")
	mux := http.NewServeMux()
	func() {
		defer func() {
			if p := recover(); p != nil {
				t.Fatalf("RegisterRoutes: %v", p)
			}
		}()
		h.RegisterRoutes(mux, mw)
	}()
	if built == nil {
		t.Fatal("RegisterRoutes did not build a guarded mux")
	}
	return mux, built
}

// TestEveryAdminRouteHasADeclaredRole: every route registered under
// /admin/league has a role in adminRouteRoles (registering one without fails
// here, naming it), and every declared pattern is really registered (no stale
// entries that would mislead canUse).
func TestEveryAdminRouteHasADeclaredRole(t *testing.T) {
	e := newSettingsTestEnv(t)
	mw := auth.NewMiddleware(auth.NewService(e.db, auth.DefaultConfig()), repository.NewPlayerRepository(e.db), repository.NewFantasyMixedDoublesRepository(e.db))
	_, g := registerAdminRoutes(t, e, mw)

	var stale []string
	for pattern := range adminRouteRoles {
		if !g.registered[pattern] {
			stale = append(stale, pattern)
		}
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Errorf("declared in adminRouteRoles but never registered (remove them or register the route): %v", stale)
	}
	for pattern, a := range adminRouteRoles {
		if !strings.HasPrefix(pattern, "/admin/league/") {
			t.Errorf("%q: admin routes live under /admin/league/", pattern)
		}
		if a.min != models.RoleCaptain && a.min != models.RoleAdmin {
			t.Errorf("%q: minimum role %q is neither captain nor admin", pattern, a.min)
		}
	}
}

func TestGuardedMuxRefusesUndeclaredRoute(t *testing.T) {
	defer func() {
		p := recover()
		if p == nil || !strings.Contains(fmt.Sprint(p), `"/admin/league/not-declared"`) {
			t.Fatalf("panic = %v, want one naming the route", p)
		}
	}()
	newGuardedMux().HandleFunc("/admin/league/not-declared", func(http.ResponseWriter, *http.Request) {})
}

// roleMatrixEnv serves the real admin routes (handlers stubbed) behind the
// real RequireAuth, with one signed-in session per role.
type roleMatrixEnv struct {
	mux     *http.ServeMux
	cookies map[models.Role]*http.Cookie
}

const reachedBody = "reached the handler"

func newRoleMatrixEnv(t *testing.T) roleMatrixEnv {
	t.Helper()
	e := newSettingsTestEnv(t)
	stubAdminHandlers = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(reachedBody))
	})
	t.Cleanup(func() { stubAdminHandlers = nil })
	authSvc := auth.NewService(e.db, auth.DefaultConfig())
	mw := auth.NewMiddleware(authSvc, repository.NewPlayerRepository(e.db), repository.NewFantasyMixedDoublesRepository(e.db))
	mux, _ := registerAdminRoutes(t, e, mw)

	// Accounts inserted directly, as rows written before WI-132 look: an
	// administrator from an earlier release needs no data change to keep
	// full access. A leftover player-role account must reach nothing.
	cookies := map[models.Role]*http.Cookie{}
	for _, role := range []models.Role{models.RoleAdmin, models.RoleCaptain, models.RoleTournamentsOnly} {
		res, err := e.db.Exec(`INSERT INTO users (username, password_hash, role, is_active, created_at, last_login_at) VALUES (?, NULL, ?, TRUE, ?, ?)`,
			"user-"+string(role), string(role), time.Now(), time.Now())
		if err != nil {
			t.Fatal(err)
		}
		id, _ := res.LastInsertId()
		rec := httptest.NewRecorder()
		if err := authSvc.StartSession(rec, httptest.NewRequest(http.MethodGet, "/login", nil), id); err != nil {
			t.Fatal(err)
		}
		for _, c := range rec.Result().Cookies() {
			if c.Name == auth.DefaultConfig().CookieName {
				cookies[role] = c
			}
		}
		if cookies[role] == nil {
			t.Fatalf("no session cookie for %s", role)
		}
	}
	return roleMatrixEnv{mux: mux, cookies: cookies}
}

// do sends method path as role ("" = anonymous).
func (m roleMatrixEnv) do(method, path string, role models.Role, header ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if role != "" {
		req.AddCookie(m.cookies[role])
	}
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	rec := httptest.NewRecorder()
	m.mux.ServeHTTP(rec, req)
	return rec
}

// samplePath is a concrete path served by pattern ("/x/" becomes "/x/1").
func samplePath(pattern string) string {
	if strings.HasSuffix(pattern, "/") {
		return pattern + "1"
	}
	return pattern
}

// TestAdminRouteRoleMatrix: every registered route x {anonymous, player,
// captain, admin} x {GET, POST}. Anonymous requests are sent to log in,
// player-role accounts get 403 everywhere, captains get exactly what the
// table gives them, and administrators reach every handler.
func TestAdminRouteRoleMatrix(t *testing.T) {
	m := newRoleMatrixEnv(t)
	patterns := make([]string, 0, len(adminRouteRoles))
	for p := range adminRouteRoles {
		patterns = append(patterns, p)
	}
	sort.Strings(patterns)

	for _, pattern := range patterns {
		path := samplePath(pattern)
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			name := method + " " + path
			if rec := m.do(method, path, ""); rec.Code != http.StatusSeeOther || !strings.HasPrefix(rec.Header().Get("Location"), "/login?redirect=") {
				t.Errorf("%s anonymous: %d %q, want 303 to /login", name, rec.Code, rec.Header().Get("Location"))
			}
			if rec := m.do(method, path, models.RoleTournamentsOnly); rec.Code != http.StatusForbidden {
				t.Errorf("%s player: %d, want 403", name, rec.Code)
			}
			if rec := m.do(method, path, models.RoleAdmin); rec.Code != http.StatusOK || rec.Body.String() != reachedBody {
				t.Errorf("%s admin: %d, want the handler", name, rec.Code)
			}
			want := http.StatusOK
			if adminRouteRoles[pattern].required(method, path) == models.RoleAdmin {
				want = http.StatusForbidden
			}
			if rec := m.do(method, path, models.RoleCaptain); rec.Code != want {
				t.Errorf("%s captain: %d, want %d", name, rec.Code, want)
			}
		}
	}

	// /admin/league itself redirects anyone who can use the dashboard.
	for role, want := range map[models.Role]int{"": http.StatusSeeOther, models.RoleTournamentsOnly: http.StatusForbidden, models.RoleCaptain: http.StatusFound, models.RoleAdmin: http.StatusFound} {
		rec := m.do(http.MethodGet, adminRootPath, role)
		if rec.Code != want {
			t.Errorf("GET %s as %q: %d, want %d", adminRootPath, role, rec.Code, want)
		}
	}
}

// TestCaptainAccessSpec states the agreed access model independently of the
// table: what a captain must reach and what must be refused.
func TestCaptainAccessSpec(t *testing.T) {
	m := newRoleMatrixEnv(t)
	captainCan := []string{
		"GET /admin/league/dashboard",
		"GET /admin/league/planning",
		"GET /admin/league/planning/matrix",
		"POST /admin/league/planning/cell",
		"POST /admin/league/planning/link",
		"GET /admin/league/players",
		"POST /admin/league/players/p1/edit",
		"POST /admin/league/players/p1/generate-availability-url",
		"POST /admin/league/players/remind-all",
		"GET /admin/league/fixtures",
		"GET /admin/league/fixtures/week-overview",
		"POST /admin/league/fixtures/1/edit",
		"POST /admin/league/fixtures/1/results",
		"POST /admin/league/fixtures/1/team-selection",
		"POST /admin/league/fixtures/1/notify-selected",
		"POST /admin/league/fixtures/1/remind-availability",
		"POST /admin/league/match-card-import",
		"GET /admin/league/points-table",
		"GET /admin/league/selection-overview",
		"POST /admin/league/captain-notes",
		"GET /admin/league/teams",
		"GET /admin/league/teams/1",
		"POST /admin/league/teams/1/add-players",
		"GET /admin/league/teams/away",
		"GET /admin/league/teams/away/1",
		"GET /admin/league/clubs",
		"GET /admin/league/clubs/1",
		"POST /admin/league/preferred-names/approve/1",
	}
	adminOnly := []string{
		"GET /admin/league/users",
		"POST /admin/league/users",
		"POST /admin/league/users/1/role",
		"POST /admin/league/users/1/player",
		"POST /admin/league/users/1/recover",
		"POST /admin/league/users/links/1/revoke",
		"GET /admin/league/sessions",
		"POST /admin/league/sessions/invalidate-user/1",
		"GET /admin/league/backups",
		"GET /admin/league/settings",
		"POST /admin/league/settings",
		"POST /admin/league/settings/lookup",
		"GET /admin/league/seasons",
		"POST /admin/league/seasons",
		"POST /admin/league/seasons/set-active",
		"POST /admin/league/seasons/from-bhplta",
		"POST /admin/league/seasons/copy",
		"GET /admin/league/seasons/setup",
		"POST /admin/league/divisions/1/edit",
		"POST /admin/league/divisions/review",
		"POST /admin/league/club-data-import",
		"POST /admin/league/teams",
		"POST /admin/league/teams/",
		"POST /admin/league/teams/1/add-captain",
		"POST /admin/league/teams/1/remove-captain",
		"POST /admin/league/teams/toggle-active/1",
		"POST /admin/league/teams/away",
		"POST /admin/league/teams/away/1",
		"POST /admin/league/teams/away/1/delete",
		"POST /admin/league/clubs/",
		"POST /admin/league/clubs/1",
		"POST /admin/league/clubs/1/delete",
		"GET /admin/league/tournaments",
		"POST /admin/league/tournaments/sync/1",
		"POST /admin/league/tournaments/setup",
		"GET /admin/league/wrapped",
	}
	for _, target := range captainCan {
		method, path, _ := strings.Cut(target, " ")
		if rec := m.do(method, path, models.RoleCaptain); rec.Code != http.StatusOK {
			t.Errorf("captain %s: %d, want the handler", target, rec.Code)
		}
		if !canUse(&models.User{Role: models.RoleCaptain}, target) {
			t.Errorf("canUse(captain, %q) = false", target)
		}
	}
	for _, target := range adminOnly {
		method, path, _ := strings.Cut(target, " ")
		rec := m.do(method, path, models.RoleCaptain)
		if rec.Code != http.StatusForbidden {
			t.Errorf("captain %s: %d, want 403", target, rec.Code)
		}
		if canUse(models.User{Role: models.RoleCaptain}, target) {
			t.Errorf("canUse(captain, %q) = true", target)
		}
		if rec := m.do(method, path, models.RoleAdmin); rec.Code != http.StatusOK {
			t.Errorf("admin %s: %d, want the handler", target, rec.Code)
		}
		if !canUse(&models.User{Role: models.RoleAdmin}, target) {
			t.Errorf("canUse(admin, %q) = false", target)
		}
	}
}

// TestForbiddenResponses: a normal request gets the friendly page; an HTMX
// request gets a short message and no swap.
func TestForbiddenResponses(t *testing.T) {
	m := newRoleMatrixEnv(t)
	rec := m.do(http.MethodGet, "/admin/league/users", models.RoleCaptain)
	body := rec.Body.String()
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Header().Get("Content-Type"), "text/html") ||
		!strings.Contains(body, `data-testid="forbidden-page"`) || !strings.Contains(body, `href="/admin/league/dashboard"`) {
		t.Errorf("captain page 403: %d %q\n%s", rec.Code, rec.Header().Get("Content-Type"), body)
	}
	if strings.Contains(body, reachedBody) {
		t.Error("the handler ran for a forbidden request")
	}

	rec = m.do(http.MethodPost, "/admin/league/teams/1/add-captain", models.RoleCaptain, "HX-Request", "true")
	if rec.Code != http.StatusForbidden || rec.Header().Get("HX-Reswap") != "none" || rec.Body.String() != auth.ForbiddenMessage {
		t.Errorf("captain HTMX 403: %d reswap=%q %q", rec.Code, rec.Header().Get("HX-Reswap"), rec.Body.String())
	}

	// A player-role account is not offered the dashboard.
	rec = m.do(http.MethodGet, "/admin/league/dashboard", models.RoleTournamentsOnly)
	if rec.Code != http.StatusForbidden || strings.Contains(rec.Body.String(), `href="/admin/league/dashboard"`) {
		t.Errorf("player 403: %d\n%s", rec.Code, rec.Body.String())
	}
}

func TestCanUse(t *testing.T) {
	captain := &models.User{Role: models.RoleCaptain}
	admin := models.User{Role: models.RoleAdmin}
	player := &models.User{Role: models.RoleTournamentsOnly}
	var nobody *models.User
	cases := []struct {
		user   interface{}
		target string
		want   bool
	}{
		{captain, "/admin/league/fixtures", true},
		{captain, "/admin/league/users", false},
		{captain, "/admin/league/clubs", true},
		{captain, "POST /admin/league/clubs", false},
		{captain, "post /admin/league/clubs/3", false},
		{admin, "/admin/league/users", true},
		{admin, "POST /admin/league/clubs/3/delete", true},
		{player, "/admin/league/fixtures", false},
		{nobody, "/admin/league/fixtures", false},
		{nil, "/admin/league/fixtures", false},
		{"admin", "/admin/league/fixtures", false},
		{admin, "/somewhere/else", false},
	}
	for _, c := range cases {
		if got := canUse(c.user, c.target); got != c.want {
			t.Errorf("canUse(%v, %q) = %v, want %v", c.user, c.target, got, c.want)
		}
	}
}

// TestDashboardNavigationByRole: the dashboard shows each role only what it
// can use (the logout link stays for everyone).
func TestDashboardNavigationByRole(t *testing.T) {
	tmpl, err := parseTemplate(findTemplatesPathAdmin(t), "admin_standalone.html")
	if err != nil {
		t.Fatal(err)
	}
	render := func(role models.Role) string {
		var buf bytes.Buffer
		err := tmpl.Execute(&buf, map[string]interface{}{
			"User":              &models.User{Username: "someone", Role: role},
			"Stats":             Stats{},
			"SignInMethodsPath": auth.SignInMethodsPath,
			"Features":          struct{ Tournaments, ClubWrapped, MyTennis bool }{true, true, true},
		})
		if err != nil {
			t.Fatalf("render as %s: %v", role, err)
		}
		return buf.String()
	}
	adminOnly := []string{"nav-users", "nav-sessions", "nav-settings", "nav-seasons", "nav-tournaments", "nav-wrapped", "nav-club-data-import", "nav-system"}
	both := []string{"nav-players", "nav-teams", "nav-fixtures", "nav-clubs", "nav-match-card-import", "nav-points-table", "nav-selection-overview", "nav-planning", "nav-preferred-names"}

	captainPage := render(models.RoleCaptain)
	for _, id := range adminOnly {
		if strings.Contains(captainPage, `data-testid="`+id+`"`) {
			t.Errorf("captain dashboard shows %s", id)
		}
	}
	for _, href := range []string{`href="/admin/league/users"`, `href="/admin/league/sessions"`, `href="/admin/league/settings"`, `href="/admin/league/seasons"`, `href="/admin/league/audit-log"`} {
		if strings.Contains(captainPage, href) {
			t.Errorf("captain dashboard links %s", href)
		}
	}
	if !strings.Contains(captainPage, "Captain Dashboard") || !strings.Contains(captainPage, `action="/logout"`) {
		t.Error("captain dashboard: title or logout link missing")
	}

	adminPage := render(models.RoleAdmin)
	for _, id := range append(adminOnly, both...) {
		if !strings.Contains(adminPage, `data-testid="`+id+`"`) {
			t.Errorf("admin dashboard lacks %s", id)
		}
	}
	for _, id := range both {
		if !strings.Contains(captainPage, `data-testid="`+id+`"`) {
			t.Errorf("captain dashboard lacks %s", id)
		}
	}
	if !strings.Contains(adminPage, "Admin Dashboard") {
		t.Error("admin dashboard title changed")
	}
}
