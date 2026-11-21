// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package setup

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"jim-dot-tennis/internal/render"
)

// Paths that stay reachable while the instance needs setup.
const (
	// Path is the setup entry page. Setup steps live under Path + "/".
	Path = "/setup"
	// HealthPath is the liveness endpoint used by the container health check.
	HealthPath = "/healthz"
	// staticPrefix serves CSS, JS, images and the service worker.
	staticPrefix = "/static/"
)

// AllowedDuringSetup reports whether path is served normally while the
// instance needs setup: static assets, the health endpoint, and /setup with
// anything below it.
func AllowedDuringSetup(path string) bool {
	return path == HealthPath ||
		path == Path ||
		strings.HasPrefix(path, Path+"/") ||
		strings.HasPrefix(path, staticPrefix)
}

// Gate routes every request to the setup entry page while the instance needs
// setup, except for AllowedDuringSetup paths. Once a home club is in effect it
// is a pass-through costing one read lock.
func (s *State) Gate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if AllowedDuringSetup(r.URL.Path) || !s.NeedsSetup(r.Context()) {
			next.ServeHTTP(w, r)
			return
		}
		if r.Header.Get("HX-Request") == "true" {
			// HTMX swaps the body of a 3xx target in place; ask it to navigate instead.
			w.Header().Set("HX-Redirect", Path)
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Redirect(w, r, Path, http.StatusSeeOther)
	})
}

// Handler serves the setup pages.
type Handler struct {
	state       *State
	templateDir string
	templates   *render.Cache
}

// NewHandler returns the setup page handler.
func NewHandler(state *State, templateDir string) *Handler {
	return &Handler{state: state, templateDir: templateDir, templates: render.NewCache()}
}

// RegisterRoutes registers the setup entry page.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc(Path, h.entry)
}

// entryData is what templates/setup/welcome.html renders.
type entryData struct {
	Status    string
	NeedsClub bool
}

// entry is the setup entry page. It is a placeholder until the setup flow
// (WI-117) lands; once setup is complete it is not found.
func (h *Handler) entry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	status := h.state.Status(r.Context())
	if status == StatusReady {
		http.NotFound(w, r)
		return
	}

	tmpl, err := h.templates.Get("welcome", func() (*template.Template, error) {
		funcMap := template.FuncMap{"currentYear": func() int { return time.Now().Year() }}
		return template.New("").Funcs(funcMap).ParseFiles(
			filepath.Join(h.templateDir, "layout.html"),
			filepath.Join(h.templateDir, "setup", "welcome.html"),
		)
	})
	if err != nil {
		log.Printf("setup: parse templates: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	data := entryData{Status: status.String(), NeedsClub: status == StatusNeedsClub}
	if err := render.Execute(w, tmpl, "setup/welcome.html", data); err != nil {
		log.Printf("setup: render: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
