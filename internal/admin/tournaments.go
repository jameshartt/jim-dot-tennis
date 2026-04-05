// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"jim-dot-tennis/internal/models"
)

// TournamentsHandler handles tournament and provider management requests
type TournamentsHandler struct {
	service     *Service
	templateDir string
}

// NewTournamentsHandler creates a new tournaments handler
func NewTournamentsHandler(service *Service, templateDir string) *TournamentsHandler {
	return &TournamentsHandler{
		service:     service,
		templateDir: templateDir,
	}
}

// --- Provider routes ---

// HandleProviders handles GET (list) and POST (create) for providers
func (h *TournamentsHandler) HandleProviders(w http.ResponseWriter, r *http.Request) {
	_, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleProvidersList(w, r)
	case http.MethodPost:
		h.handleProviderCreate(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *TournamentsHandler) handleProvidersList(w http.ResponseWriter, r *http.Request) {
	providers, err := h.service.GetAllTournamentProviders()
	if err != nil {
		logAndError(w, "Failed to load providers", err, http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Providers":    providers,
		"Success":      r.URL.Query().Get("success"),
		"Error":        r.URL.Query().Get("error"),
		"HomeClubName": homeClubNameFromContext(r),
	}

	tmpl, err := parseTemplate(h.templateDir, "admin/tournament_providers.html")
	if err != nil {
		logAndError(w, "Failed to parse template", err, http.StatusInternalServerError)
		return
	}
	if err := renderTemplate(w, tmpl, data); err != nil {
		logAndError(w, "Failed to render template", err, http.StatusInternalServerError)
	}
}

func (h *TournamentsHandler) handleProviderCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	abbr := strings.TrimSpace(r.FormValue("provider_abbr"))

	if name == "" || abbr == "" {
		http.Redirect(w, r, "/admin/league/tournaments/providers?error=Name+and+abbreviation+are+required", http.StatusSeeOther)
		return
	}

	provider := &models.TournamentProvider{
		Name:         name,
		ProviderAbbr: abbr,
	}

	if err := h.service.CreateTournamentProvider(provider); err != nil {
		log.Printf("Failed to create provider: %v", err)
		http.Redirect(w, r, "/admin/league/tournaments/providers?error=Failed+to+create+provider", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/league/tournaments/providers?success=Provider+created", http.StatusSeeOther)
}

// HandleProviderEdit handles GET (edit form) and POST (update/delete) for a single provider
func (h *TournamentsHandler) HandleProviderEdit(w http.ResponseWriter, r *http.Request) {
	_, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	providerID, err := parseIDFromPath(r.URL.Path, "/admin/league/tournaments/providers/")
	if err != nil {
		http.Error(w, "Invalid provider ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleProviderEditGet(w, r, providerID)
	case http.MethodPost:
		h.handleProviderEditPost(w, r, providerID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *TournamentsHandler) handleProviderEditGet(w http.ResponseWriter, r *http.Request, providerID uint) {
	provider, err := h.service.GetTournamentProviderByID(providerID)
	if err != nil {
		logAndError(w, "Provider not found", err, http.StatusNotFound)
		return
	}

	data := map[string]interface{}{
		"Provider": provider,
	}

	tmpl, err := parseTemplate(h.templateDir, "admin/tournament_provider_form.html")
	if err != nil {
		logAndError(w, "Failed to parse template", err, http.StatusInternalServerError)
		return
	}
	if err := renderTemplate(w, tmpl, data); err != nil {
		logAndError(w, "Failed to render template", err, http.StatusInternalServerError)
	}
}

func (h *TournamentsHandler) handleProviderEditPost(w http.ResponseWriter, r *http.Request, providerID uint) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	action := r.FormValue("action")

	if action == "delete" {
		if err := h.service.DeleteTournamentProvider(providerID); err != nil {
			log.Printf("Failed to delete provider: %v", err)
			http.Redirect(w, r, "/admin/league/tournaments/providers?error="+err.Error(), http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/admin/league/tournaments/providers?success=Provider+deleted", http.StatusSeeOther)
		return
	}

	provider, err := h.service.GetTournamentProviderByID(providerID)
	if err != nil {
		logAndError(w, "Provider not found", err, http.StatusNotFound)
		return
	}

	provider.Name = strings.TrimSpace(r.FormValue("name"))
	provider.ProviderAbbr = strings.TrimSpace(r.FormValue("provider_abbr"))

	if provider.Name == "" || provider.ProviderAbbr == "" {
		http.Error(w, "Name and abbreviation are required", http.StatusBadRequest)
		return
	}

	if err := h.service.UpdateTournamentProvider(provider); err != nil {
		logAndError(w, "Failed to update provider", err, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/league/tournaments/providers?success=Provider+updated", http.StatusSeeOther)
}

// --- Tournament routes ---

// HandleTournaments handles GET (list) and POST (create) for tournaments
func (h *TournamentsHandler) HandleTournaments(w http.ResponseWriter, r *http.Request) {
	_, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleTournamentsList(w, r)
	case http.MethodPost:
		h.handleTournamentCreate(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *TournamentsHandler) handleTournamentsList(w http.ResponseWriter, r *http.Request) {
	tournaments, err := h.service.GetAllTournaments()
	if err != nil {
		logAndError(w, "Failed to load tournaments", err, http.StatusInternalServerError)
		return
	}

	providers, err := h.service.GetAllTournamentProviders()
	if err != nil {
		logAndError(w, "Failed to load providers", err, http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Tournaments": tournaments,
		"Providers":   providers,
		"Success":     r.URL.Query().Get("success"),
		"Error":       r.URL.Query().Get("error"),
	}

	tmpl, err := parseTemplate(h.templateDir, "admin/tournaments.html")
	if err != nil {
		logAndError(w, "Failed to parse template", err, http.StatusInternalServerError)
		return
	}
	if err := renderTemplate(w, tmpl, data); err != nil {
		logAndError(w, "Failed to render template", err, http.StatusInternalServerError)
	}
}

func (h *TournamentsHandler) handleTournamentCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	providerIDStr := r.FormValue("provider_id")
	providerID, err := strconv.ParseUint(providerIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid provider ID", http.StatusBadRequest)
		return
	}

	displayOrder, _ := strconv.Atoi(r.FormValue("display_order"))

	tournament := &models.Tournament{
		Name:                  strings.TrimSpace(r.FormValue("name")),
		Description:           strings.TrimSpace(r.FormValue("description")),
		CourthiveTournamentID: strings.TrimSpace(r.FormValue("courthive_tournament_id")),
		ProviderID:            uint(providerID),
		StartDate:             r.FormValue("start_date"),
		EndDate:               r.FormValue("end_date"),
		IsVisible:             r.FormValue("is_visible") == "on",
		DisplayOrder:          displayOrder,
	}

	if tournament.Name == "" {
		http.Redirect(w, r, "/admin/league/tournaments?error=Tournament+name+is+required", http.StatusSeeOther)
		return
	}

	if err := h.service.CreateTournament(tournament); err != nil {
		log.Printf("Failed to create tournament: %v", err)
		http.Redirect(w, r, "/admin/league/tournaments?error=Failed+to+create+tournament", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/league/tournaments?success=Tournament+created", http.StatusSeeOther)
}

// HandleTournamentEdit handles GET (edit form) and POST (update/delete) for a single tournament
func (h *TournamentsHandler) HandleTournamentEdit(w http.ResponseWriter, r *http.Request) {
	_, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	tournamentID, err := parseIDFromPath(r.URL.Path, "/admin/league/tournaments/edit/")
	if err != nil {
		http.Error(w, "Invalid tournament ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleTournamentEditGet(w, r, tournamentID)
	case http.MethodPost:
		h.handleTournamentEditPost(w, r, tournamentID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *TournamentsHandler) handleTournamentEditGet(w http.ResponseWriter, r *http.Request, tournamentID uint) {
	tournament, err := h.service.GetTournamentByID(tournamentID)
	if err != nil {
		logAndError(w, "Tournament not found", err, http.StatusNotFound)
		return
	}

	providers, err := h.service.GetAllTournamentProviders()
	if err != nil {
		logAndError(w, "Failed to load providers", err, http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Tournament": tournament,
		"Providers":  providers,
	}

	tmpl, err := parseTemplate(h.templateDir, "admin/tournament_form.html")
	if err != nil {
		logAndError(w, "Failed to parse template", err, http.StatusInternalServerError)
		return
	}
	if err := renderTemplate(w, tmpl, data); err != nil {
		logAndError(w, "Failed to render template", err, http.StatusInternalServerError)
	}
}

func (h *TournamentsHandler) handleTournamentEditPost(w http.ResponseWriter, r *http.Request, tournamentID uint) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	action := r.FormValue("action")

	if action == "delete" {
		if err := h.service.DeleteTournament(tournamentID); err != nil {
			log.Printf("Failed to delete tournament: %v", err)
			http.Redirect(w, r, "/admin/league/tournaments?error=Failed+to+delete+tournament", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/admin/league/tournaments?success=Tournament+deleted", http.StatusSeeOther)
		return
	}

	tournament, err := h.service.GetTournamentByID(tournamentID)
	if err != nil {
		logAndError(w, "Tournament not found", err, http.StatusNotFound)
		return
	}

	providerID, _ := strconv.ParseUint(r.FormValue("provider_id"), 10, 32)
	displayOrder, _ := strconv.Atoi(r.FormValue("display_order"))

	tournament.Name = strings.TrimSpace(r.FormValue("name"))
	tournament.Description = strings.TrimSpace(r.FormValue("description"))
	tournament.CourthiveTournamentID = strings.TrimSpace(r.FormValue("courthive_tournament_id"))
	tournament.ProviderID = uint(providerID)
	tournament.StartDate = r.FormValue("start_date")
	tournament.EndDate = r.FormValue("end_date")
	tournament.IsVisible = r.FormValue("is_visible") == "on"
	tournament.DisplayOrder = displayOrder

	if err := h.service.UpdateTournament(tournament); err != nil {
		logAndError(w, "Failed to update tournament", err, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/league/tournaments?success=Tournament+updated", http.StatusSeeOther)
}

// HandleToggleVisibility toggles a tournament's visibility via HTMX
func (h *TournamentsHandler) HandleToggleVisibility(w http.ResponseWriter, r *http.Request) {
	_, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tournamentID, err := parseIDFromPath(r.URL.Path, "/admin/league/tournaments/toggle-visibility/")
	if err != nil {
		http.Error(w, "Invalid tournament ID", http.StatusBadRequest)
		return
	}

	tournament, err := h.service.ToggleTournamentVisibility(tournamentID)
	if err != nil {
		logAndError(w, "Failed to toggle visibility", err, http.StatusInternalServerError)
		return
	}

	// Return just the toggle button HTML for HTMX swap
	visibleText := "Hidden"
	visibleClass := "badge-hidden"
	if tournament.IsVisible {
		visibleText = "Visible"
		visibleClass = "badge-visible"
	}

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<button class="visibility-toggle %s" hx-post="/admin/league/tournaments/toggle-visibility/%d" hx-swap="outerHTML">%s</button>`,
		visibleClass, tournament.ID, visibleText)
}

// HandleSync syncs tournaments from CourtHive for a given provider
func (h *TournamentsHandler) HandleSync(w http.ResponseWriter, r *http.Request) {
	_, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	providerID, err := parseIDFromPath(r.URL.Path, "/admin/league/tournaments/sync/")
	if err != nil {
		http.Error(w, "Invalid provider ID", http.StatusBadRequest)
		return
	}

	result, err := h.service.SyncFromCourtHive(providerID)
	if err != nil {
		log.Printf("CourtHive sync failed for provider %d: %v", providerID, err)
		http.Redirect(w, r, fmt.Sprintf("/admin/league/tournaments?error=Sync+failed:+%s", err.Error()), http.StatusSeeOther)
		return
	}

	msg := fmt.Sprintf("Sync+complete:+%d+new,+%d+updated,+%d+unchanged", result.New, result.Updated, result.Unchanged)
	http.Redirect(w, r, "/admin/league/tournaments?success="+msg, http.StatusSeeOther)
}
