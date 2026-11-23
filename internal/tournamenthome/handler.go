// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

// Package tournamenthome is the tournament home page (WI-153): where an
// account with tournament access and no league role lands after signing in,
// with one thing to do, open tournament management.
//
// It lives outside /admin/league, so the league role table
// (internal/admin/roles.go) does not cover it. Its guard is its own and
// explicit: the person must be signed in (auth.Middleware.RequireAuth) and
// their effective tournament access (models.EffectiveTournamentAccess) must
// not be none.
package tournamenthome

import (
	"context"
	"errors"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/render"
	"jim-dot-tennis/internal/tournaments"
)

// ManagementPath is where the "Open tournament management" button goes, and
// where tournament management sends someone who needs to sign in: the route
// that hands the person over (WI-152). It is the one place this address is
// written here; the tournament management build names it too
// (VITE_EXTERNAL_LOGIN_URL in the TMX fork's .env.production).
const ManagementPath = auth.TournamentHomePath + "/open"

// tmxPath is tournament management itself. A hand-over lands on its
// single-use token route.
const tmxPath = "/tournaments/"

// Opener hands a person over to tournament management
// (internal/tournaments.Service).
type Opener interface {
	// Configured reports whether this deployment has the bundled tournament
	// stack. Without it, tournament management has its own sign-in.
	Configured() bool
	// Handover returns a single-use token that signs the account in to
	// tournament management, acting for the set of tournaments with the
	// given code ("" for the person's first).
	Handover(ctx context.Context, userID int64, provider string) (string, error)
	// ProvidersFor lists the sets of tournaments the account can manage.
	ProvidersFor(ctx context.Context, userID int64) ([]tournaments.ManagedProvider, error)
}

// leagueDashboardPath is the league dashboard, for captains and
// administrators who reach this page.
const leagueDashboardPath = "/admin/league/dashboard"

// FeatureSwitch says whether the club has Tournaments switched on
// (config.ClubSettings).
type FeatureSwitch interface {
	TournamentsEnabled() bool
}

// Handler serves the tournament home page.
type Handler struct {
	features    FeatureSwitch
	opener      Opener
	templateDir string
	templates   *render.Cache
}

// New returns the handler. templateDir is the template tree's root.
func New(features FeatureSwitch, opener Opener, templateDir string) *Handler {
	return &Handler{features: features, opener: opener, templateDir: templateDir, templates: render.NewCache()}
}

// RegisterRoutes adds GET auth.TournamentHomePath to mux, behind
// requireAuth (auth.Middleware.RequireAuth in the server).
func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler) {
	mux.Handle("GET "+auth.TournamentHomePath, requireAuth(http.HandlerFunc(h.serve)))
	mux.Handle("GET "+ManagementPath, requireAuth(http.HandlerFunc(h.open)))
}

// open hands the signed-in person over to tournament management. The guard
// is the same as the home page's: signed in, with tournament access, and
// Tournaments switched on. The single-use token travels in the address's
// fragment, which browsers never send to a server, so it reaches no log.
func (h *Handler) open(w http.ResponseWriter, r *http.Request) {
	user, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if user.TournamentAccessLevel() == models.TournamentAccessNone || !h.features.TournamentsEnabled() {
		http.Redirect(w, r, auth.TournamentHomePath, http.StatusSeeOther)
		return
	}
	if h.opener == nil || !h.opener.Configured() {
		// No bundled tournament stack: tournament management signs people
		// in itself.
		http.Redirect(w, r, tmxPath, http.StatusSeeOther)
		return
	}
	code := strings.TrimSpace(r.URL.Query().Get("provider"))
	if code == "" {
		// Someone with several sets of tournaments chooses on the home page:
		// a link that names none (tournament management's own "Log in", a
		// bookmark) must not pick one for them.
		if sets := h.sets(r.Context(), user); len(sets) > 1 {
			http.Redirect(w, r, auth.TournamentHomePath+"#choose", http.StatusSeeOther)
			return
		}
	}
	token, err := h.opener.Handover(r.Context(), user.ID, code)
	if errors.Is(err, tournaments.ErrNoAccess) {
		// A set of tournaments that is not one of theirs (or no longer is).
		http.Redirect(w, r, auth.TournamentHomePath, http.StatusSeeOther)
		return
	}
	if err != nil {
		log.Printf("tournament home: hand-over for account %d: %v", user.ID, err)
		data := pageData{Username: user.Username, Enabled: true, ManagementURL: ManagementPath, OpenFailed: true}
		if code != "" {
			data.ManagementURL = setURL(code)
		}
		if models.RoleAtLeast(user.Role, models.RoleCaptain) {
			data.LeaguePath = leagueDashboardPath
		}
		h.render(w, http.StatusBadGateway, data)
		return
	}
	http.Redirect(w, r, tmxPath+"#/sso?token="+url.QueryEscape(token), http.StatusSeeOther)
}

// setURL is the hand-over address for the set of tournaments with this code.
func setURL(code string) string {
	return ManagementPath + "?provider=" + url.QueryEscape(code)
}

// SetChoice is one set of tournaments a person can open: what the home page
// shows when they have more than one.
type SetChoice struct {
	Name string // what people call it, set by an administrator
	Code string // its code on the tournament server, shown small
	URL  string // opens tournament management for it
}

// sets lists the sets of tournaments user can open, or nil when this
// deployment has no bundled tournament stack or the list cannot be read (the
// page then offers the single button, which still works).
func (h *Handler) sets(ctx context.Context, user models.User) []SetChoice {
	if h.opener == nil || !h.opener.Configured() {
		return nil
	}
	providers, err := h.opener.ProvidersFor(ctx, user.ID)
	if err != nil {
		log.Printf("tournament home: reading the sets of tournaments of account %d: %v", user.ID, err)
		return nil
	}
	out := make([]SetChoice, 0, len(providers))
	for _, p := range providers {
		name := p.Name
		if name == "" {
			name = p.Abbr
		}
		out = append(out, SetChoice{Name: name, Code: p.Abbr, URL: setURL(p.Abbr)})
	}
	return out
}

// pageData is what the template sees.
type pageData struct {
	Username      string
	AccessLabel   string
	AllAccess     bool
	Enabled       bool   // Tournaments is switched on for the club
	ManagementURL string // the button's target
	LeaguePath    string // the league dashboard, for captains and administrators; "" otherwise
	Refused       bool   // the account has no tournament access
	OpenFailed    bool   // tournament management could not be opened just now
	// Sets is filled only when the person has more than one set of
	// tournaments: the page then asks which to manage.
	Sets []SetChoice
}

func (h *Handler) serve(w http.ResponseWriter, r *http.Request) {
	user, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		// RequireAuth always puts the user there; refuse rather than guess.
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	data := pageData{Username: user.Username}
	if models.RoleAtLeast(user.Role, models.RoleCaptain) {
		data.LeaguePath = leagueDashboardPath
	}
	status := http.StatusOK
	access := user.TournamentAccessLevel()
	if access == models.TournamentAccessNone {
		data.Refused = true
		status = http.StatusForbidden
	} else {
		data.AccessLabel = access.Label()
		data.AllAccess = access == models.TournamentAccessAll
		data.Enabled = h.features.TournamentsEnabled()
		if data.Enabled {
			data.ManagementURL = ManagementPath
			if sets := h.sets(r.Context(), user); len(sets) > 1 {
				data.Sets = sets
			}
		}
	}
	h.render(w, status, data)
}

func (h *Handler) render(w http.ResponseWriter, status int, data pageData) {
	tmpl, err := h.templates.Get("tournament_home", func() (*template.Template, error) {
		funcMap := template.FuncMap{"currentYear": func() int { return time.Now().Year() }}
		return render.ParseFiles(template.New("").Funcs(funcMap),
			filepath.Join(h.templateDir, "layout.html"),
			filepath.Join(h.templateDir, "tournament", "home.html"),
		)
	})
	if err != nil {
		log.Printf("tournament home: parse templates: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := render.Execute(w, tmpl, "tournament/home.html", data); err != nil {
		log.Printf("tournament home: %v", err)
	}
}
