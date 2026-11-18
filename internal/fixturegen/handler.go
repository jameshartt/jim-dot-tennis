// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package fixturegen

import (
	"context"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"jim-dot-tennis/internal/render"
)

const (
	tablesTTL      = 30 * time.Minute
	refreshMinimum = time.Minute // floor between upstream fetches, however often refresh is asked for
)

// Handler serves the fixture generator page and its league-tables feed.
type Handler struct {
	templates *template.Template
	client    *http.Client
	tablesURL string

	mu      sync.Mutex
	cached  *Tables
	fetched time.Time
	now     func() time.Time
}

// New creates the handler. The tables URL comes from LEAGUE_TABLES_URL and
// defaults to the BHPLTA league tables page.
func New(templates *template.Template) *Handler {
	url := os.Getenv("LEAGUE_TABLES_URL")
	if url == "" {
		url = DefaultTablesURL
	}
	return &Handler{
		templates: templates,
		client:    &http.Client{Timeout: 12 * time.Second},
		tablesURL: url,
		now:       time.Now,
	}
}

// RegisterRoutes registers the public routes. No authentication: the page
// reads public league tables and writes nothing.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/fixture-generator", h.handlePage)
	mux.HandleFunc("/fixture-generator/api/tables", h.handleTables)
}

func (h *Handler) handlePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data := map[string]interface{}{
		"Config": map[string]string{
			"tablesURL":   "/fixture-generator/api/tables",
			"snapshotURL": "/static/fixture-generator/bhplta-2026.json",
		},
	}
	if err := render.Execute(w, h.templates, "fixture_generator.html", data); err != nil {
		log.Printf("fixture generator: render failed: %v", err)
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
	}
}

func (h *Handler) handleTables(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tables, err := h.tables(r.Context(), r.URL.Query().Get("refresh") != "")
	if err != nil {
		log.Printf("fixture generator: league tables unavailable: %v", err)
		http.Error(w, "League tables are unavailable", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(tables); err != nil {
		log.Printf("fixture generator: failed to write tables: %v", err)
	}
}

// tables returns the league tables, fetching them when the cached copy is
// stale. If the league website is down, a stale copy beats none.
func (h *Handler) tables(ctx context.Context, refresh bool) (*Tables, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	age := h.now().Sub(h.fetched)
	if h.cached != nil && age < tablesTTL && (!refresh || age < refreshMinimum) {
		return h.cached, nil
	}
	tables, err := FetchTables(ctx, h.client, h.tablesURL)
	if err != nil {
		if h.cached != nil {
			log.Printf("fixture generator: serving cached league tables: %v", err)
			return h.cached, nil
		}
		return nil, err
	}
	h.cached, h.fetched = tables, h.now()
	return tables, nil
}
