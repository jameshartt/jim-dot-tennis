package admin

import (
	"log"
	"net/http"
	"strconv"
	"strings"
)

// SelectionOverviewHandler handles the selection overview dashboard
type SelectionOverviewHandler struct {
	service     *Service
	templateDir string
}

// NewSelectionOverviewHandler creates a new selection overview handler
func NewSelectionOverviewHandler(service *Service, templateDir string) *SelectionOverviewHandler {
	return &SelectionOverviewHandler{
		service:     service,
		templateDir: templateDir,
	}
}

// HandleSelectionOverview routes to the appropriate handler based on the path
func (h *SelectionOverviewHandler) HandleSelectionOverview(w http.ResponseWriter, r *http.Request) {
	// Route based on path
	path := r.URL.Path
	if strings.HasSuffix(path, "/refresh") {
		h.handleRefresh(w, r)
		return
	}
	h.handleOverviewPage(w, r)
}

// handleOverviewPage renders the full page
func (h *SelectionOverviewHandler) handleOverviewPage(w http.ResponseWriter, r *http.Request) {
	// Get active season
	season, err := h.service.GetActiveSeason()
	if err != nil {
		log.Printf("Error getting active season: %v", err)
		http.Error(w, "Failed to get active season", http.StatusInternalServerError)
		return
	}

	// Parse week ID from query param or use current/next week
	var weekID uint
	weekIDParam := r.URL.Query().Get("week")
	if weekIDParam != "" {
		parsedID, err := strconv.ParseUint(weekIDParam, 10, 32)
		if err != nil {
			log.Printf("Invalid week ID: %v", err)
			http.Error(w, "Invalid week ID", http.StatusBadRequest)
			return
		}
		weekID = uint(parsedID)
	} else {
		// Try to get current or next week
		week, err := h.service.GetCurrentOrNextWeek(season.ID)
		if err != nil {
			// If no current/next week (e.g., season hasn't started), get the first week
			log.Printf("No current/next week found, trying to get first week: %v", err)
			allWeeks, err := h.service.GetWeeksBySeason(season.ID)
			if err != nil || len(allWeeks) == 0 {
				log.Printf("Error getting weeks: %v", err)
				http.Error(w, "No weeks configured for this season", http.StatusNotFound)
				return
			}
			weekID = allWeeks[0].ID
		} else {
			weekID = week.ID
		}
	}

	// Parse division filter from query param
	var filteredDivisionIDs []uint
	divisionsParam := r.URL.Query().Get("divisions")
	if divisionsParam != "" {
		divStrs := strings.Split(divisionsParam, ",")
		for _, divStr := range divStrs {
			divID, err := strconv.ParseUint(strings.TrimSpace(divStr), 10, 32)
			if err == nil {
				filteredDivisionIDs = append(filteredDivisionIDs, uint(divID))
			}
		}
	}

	// Get overview data
	overview, err := h.service.GetWeekSelectionOverview(weekID, filteredDivisionIDs)
	if err != nil {
		log.Printf("Error getting selection overview: %v", err)
		http.Error(w, "Failed to get selection overview", http.StatusInternalServerError)
		return
	}

	// Prepare template data
	data := map[string]interface{}{
		"Overview":             overview,
		"FilteredDivisionIDs":  filteredDivisionIDs,
		"FilteredDivisionsStr": divisionsParam,
	}

	// Load and execute template
	tmpl, err := parseTemplate(h.templateDir, "admin/selection_overview.html")
	if err != nil {
		log.Printf("Error parsing template: %v", err)
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}
}

// handleRefresh handles HTMX partial refresh
func (h *SelectionOverviewHandler) handleRefresh(w http.ResponseWriter, r *http.Request) {
	// Parse week ID from query param (required for refresh)
	weekIDParam := r.URL.Query().Get("week")
	if weekIDParam == "" {
		http.Error(w, "Week ID required", http.StatusBadRequest)
		return
	}

	parsedID, err := strconv.ParseUint(weekIDParam, 10, 32)
	if err != nil {
		log.Printf("Invalid week ID: %v", err)
		http.Error(w, "Invalid week ID", http.StatusBadRequest)
		return
	}
	weekID := uint(parsedID)

	// Parse division filter from query param
	var filteredDivisionIDs []uint
	divisionsParam := r.URL.Query().Get("divisions")
	if divisionsParam != "" {
		divStrs := strings.Split(divisionsParam, ",")
		for _, divStr := range divStrs {
			divID, err := strconv.ParseUint(strings.TrimSpace(divStr), 10, 32)
			if err == nil {
				filteredDivisionIDs = append(filteredDivisionIDs, uint(divID))
			}
		}
	}

	// Get overview data
	overview, err := h.service.GetWeekSelectionOverview(weekID, filteredDivisionIDs)
	if err != nil {
		log.Printf("Error getting selection overview: %v", err)
		http.Error(w, "Failed to get selection overview", http.StatusInternalServerError)
		return
	}

	// Prepare template data
	data := map[string]interface{}{
		"Overview":             overview,
		"FilteredDivisionIDs":  filteredDivisionIDs,
		"FilteredDivisionsStr": divisionsParam,
		"WeekID":               weekID,
	}

	// Load and execute partial template
	tmpl, err := parseTemplate(h.templateDir, "admin/partials/selection_overview_content.html")
	if err != nil {
		log.Printf("Error parsing template: %v", err)
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}
}
