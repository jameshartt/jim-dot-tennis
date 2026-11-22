// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package branding

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"jim-dot-tennis/internal/config"
	"jim-dot-tennis/internal/repository"
)

// Paths served by Handler.
const (
	ManifestPath       = "/static/manifest.json"
	PlayerManifestPath = "/api/manifest"
	ServiceWorkerPath  = "/static/service-worker.js"
)

// serviceWorkerPlaceholder is the line in static/service-worker.js replaced
// with the club's branding when the script is served.
const serviceWorkerPlaceholder = `var BRANDING = {"name":"Jim.Tennis","icon":"/static/icon-192.svg"};`

// Handler serves the uploaded logo, the manifests and the service worker.
// Names, logo and colours come from the request context (set by
// config.HomeClubMiddleware), so changes on the settings page apply at once.
type Handler struct {
	settings *config.ClubSettings
	static   fs.FS
}

// NewHandler returns a branding handler. static is the static file tree
// (embedded, or static/ on disk in development); it holds service-worker.js.
func NewHandler(settings *config.ClubSettings, static fs.FS) *Handler {
	return &Handler{settings: settings, static: static}
}

// RegisterRoutes registers the branding routes. The manifest and service
// worker paths are more specific than "/static/", so they win over the file server.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc(config.LogoURLPrefix, h.ServeLogo)
	mux.HandleFunc(ManifestPath, h.ServeManifest)
	mux.HandleFunc(PlayerManifestPath, h.ServeManifest)
	mux.HandleFunc(ServiceWorkerPath, h.ServeServiceWorker)
}

// ServeLogo serves the current uploaded logo, and nothing else, with headers
// that stop it being interpreted as anything but an image.
func (h *Handler) ServeLogo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, config.LogoURLPrefix)
	if !ValidLogoName(name) || name != h.settings.Stored(repository.SettingClubLogoFile) {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(filepath.Join(h.settings.Dir(), name))
	if err != nil {
		log.Printf("Club logo %s could not be opened: %v", name, err)
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	hdr := w.Header()
	hdr.Set("Content-Type", config.LogoMIMEType(name))
	hdr.Set("X-Content-Type-Options", "nosniff")
	hdr.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	hdr.Set("Cross-Origin-Resource-Policy", "same-origin")
	// The name changes with the content, so it can be cached for good.
	hdr.Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeContent(w, r, "", info.ModTime(), f)
}

type manifestIcon struct {
	Src     string `json:"src"`
	Sizes   string `json:"sizes"`
	Type    string `json:"type"`
	Purpose string `json:"purpose"`
}

type manifest struct {
	Name            string         `json:"name"`
	ShortName       string         `json:"short_name"`
	Description     string         `json:"description"`
	ID              string         `json:"id"`
	StartURL        string         `json:"start_url"`
	Scope           string         `json:"scope"`
	Display         string         `json:"display"`
	BackgroundColor string         `json:"background_color"`
	ThemeColor      string         `json:"theme_color"`
	Categories      []string       `json:"categories,omitempty"`
	PreferRelated   bool           `json:"prefer_related_applications"`
	Icons           []manifestIcon `json:"icons"`
}

// minManifestLogo is the smallest uploaded logo offered as an install icon.
const minManifestLogo = 192

// buildManifest returns the web app manifest for the given branding and start URL.
func buildManifest(b config.Branding, startURL, shortName string) manifest {
	icons := []manifestIcon{
		{Src: "/static/icon-192.svg", Sizes: "192x192", Type: "image/svg+xml", Purpose: "any"},
		{Src: "/static/icon-512.svg", Sizes: "512x512", Type: "image/svg+xml", Purpose: "any"},
		{Src: "/static/icon-192.svg", Sizes: "192x192", Type: "image/svg+xml", Purpose: "maskable"},
	}
	// A square uploaded logo that is big enough is the preferred install icon.
	if b.LogoType != "" && b.LogoWidth == b.LogoHeight && b.LogoWidth >= minManifestLogo {
		icons = append([]manifestIcon{{
			Src: b.LogoPath, Sizes: fmt.Sprintf("%dx%d", b.LogoWidth, b.LogoHeight), Type: b.LogoType, Purpose: "any",
		}}, icons...)
	}
	return manifest{
		Name:            b.Name,
		ShortName:       shortName,
		Description:     "Tennis league management and notifications",
		ID:              startURL,
		StartURL:        startURL,
		Scope:           "/",
		Display:         "standalone",
		BackgroundColor: "#ffffff",
		ThemeColor:      b.PrimaryColour,
		Categories:      []string{"sports", "lifestyle"},
		Icons:           icons,
	}
}

// safeStartURL accepts only a same-origin absolute path; anything else is "/".
func safeStartURL(s string) string {
	if s == "" || !strings.HasPrefix(s, "/") || strings.HasPrefix(s, "//") || strings.ContainsAny(s, "\\\r\n") {
		return "/"
	}
	return s
}

// ServeManifest serves the web app manifest. /api/manifest takes ?start= so
// a player's "Add to Home Screen" opens their own page.
func (h *Handler) ServeManifest(w http.ResponseWriter, r *http.Request) {
	startURL, shortName := "/", "Tennis"
	if r.URL.Path == PlayerManifestPath {
		startURL, shortName = safeStartURL(r.URL.Query().Get("start")), "My Tennis"
	}
	w.Header().Set("Content-Type", "application/manifest+json")
	w.Header().Set("Cache-Control", "no-cache")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(buildManifest(config.GetBranding(r.Context()), startURL, shortName)); err != nil {
		log.Printf("Failed to write manifest: %v", err)
	}
}

// ServeServiceWorker serves static/service-worker.js with the club's name and
// icon substituted for the placeholder (used for push notifications). A
// branding change changes the script bytes, so browsers pick it up on their
// next update check.
func (h *Handler) ServeServiceWorker(w http.ResponseWriter, r *http.Request) {
	src, err := fs.ReadFile(h.static, "service-worker.js")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	b := config.GetBranding(r.Context())
	icon := "/static/icon-192.svg"
	if b.LogoType != "" {
		icon = b.LogoPath
	}
	// encoding/json escapes <, >, &, U+2028 and U+2029, so this is safe inside a script.
	value, err := json.Marshal(map[string]string{"name": b.Name, "icon": icon})
	if err != nil {
		http.Error(w, "service worker unavailable", http.StatusInternalServerError)
		return
	}
	out := bytes.Replace(src, []byte(serviceWorkerPlaceholder), []byte("var BRANDING = "+string(value)+";"), 1)
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Service-Worker-Allowed", "/")
	w.Header().Set("Cache-Control", "no-cache")
	if _, err := w.Write(out); err != nil {
		log.Printf("Failed to write service worker: %v", err)
	}
}
