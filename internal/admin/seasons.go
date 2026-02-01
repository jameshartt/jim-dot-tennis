package admin

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"jim-dot-tennis/internal/models"
)

// SeasonsHandler handles season-related requests
type SeasonsHandler struct {
	service     *Service
	templateDir string
}

// NewSeasonsHandler creates a new seasons handler
func NewSeasonsHandler(service *Service, templateDir string) *SeasonsHandler {
	return &SeasonsHandler{
		service:     service,
		templateDir: templateDir,
	}
}

// HandleSeasons handles season management routes
func (h *SeasonsHandler) HandleSeasons(w http.ResponseWriter, r *http.Request) {
	log.Printf("Admin seasons handler called with path: %s, method: %s", r.URL.Path, r.Method)

	switch r.Method {
	case http.MethodGet:
		h.handleSeasonsList(w, r)
	case http.MethodPost:
		h.handleCreateSeason(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleSetActiveSeason handles setting the active season
func (h *SeasonsHandler) HandleSetActiveSeason(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	seasonIDStr := r.URL.Query().Get("id")
	seasonID, err := strconv.ParseUint(seasonIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid season ID", http.StatusBadRequest)
		return
	}

	if err := h.service.SetActiveSeason(uint(seasonID)); err != nil {
		logAndError(w, "Failed to set active season", err, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/league/seasons", http.StatusSeeOther)
}

// SeasonWithStats combines a season with its statistics
type SeasonWithStats struct {
	Season    models.Season
	Stats     *SeasonStats
	WeekCount int
}

// handleSeasonsList displays the list of seasons
func (h *SeasonsHandler) handleSeasonsList(w http.ResponseWriter, r *http.Request) {
	// Get user from context
	user, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	seasons, err := h.service.GetAllSeasons()
	if err != nil {
		logAndError(w, "Failed to load seasons", err, http.StatusInternalServerError)
		return
	}

	activeSeason, _ := h.service.GetActiveSeason()

	// Build seasons with stats
	var seasonsWithStats []SeasonWithStats
	for _, season := range seasons {
		stats, _ := h.service.GetSeasonStats(season.ID)
		weekCount, _ := h.service.GetWeekCountForSeason(season.ID)

		seasonsWithStats = append(seasonsWithStats, SeasonWithStats{
			Season:    season,
			Stats:     stats,
			WeekCount: weekCount,
		})
	}

	// Load the template
	tmpl, err := parseTemplate(h.templateDir, "admin/season_list.html")
	if err != nil {
		log.Printf("Error parsing season list template: %v", err)
		// Fallback to simple HTML response
		renderFallbackHTML(w, "Admin - Seasons", "Season Management",
			"Season management page - coming soon", "/admin/league")
		return
	}

	// Execute the template with data
	if err := renderTemplate(w, tmpl, map[string]interface{}{
		"User":         user,
		"Seasons":      seasonsWithStats,
		"ActiveSeason": activeSeason,
		"CurrentYear":  time.Now().Year(),
	}); err != nil {
		logAndError(w, err.Error(), err, http.StatusInternalServerError)
	}
}

// handleCreateSeason handles POST request to create a new season
func (h *SeasonsHandler) handleCreateSeason(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	yearStr := r.FormValue("year")
	startDateStr := r.FormValue("start_date")
	endDateStr := r.FormValue("end_date")
	numWeeksStr := r.FormValue("num_weeks")

	year, err := strconv.Atoi(yearStr)
	if err != nil {
		http.Error(w, "Invalid year", http.StatusBadRequest)
		return
	}

	numWeeks, err := strconv.Atoi(numWeeksStr)
	if err != nil || numWeeks < 1 || numWeeks > 52 {
		http.Error(w, "Invalid number of weeks (must be between 1 and 52)", http.StatusBadRequest)
		return
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		http.Error(w, "Invalid start date", http.StatusBadRequest)
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		http.Error(w, "Invalid end date", http.StatusBadRequest)
		return
	}

	season := &models.Season{
		Name:      name,
		Year:      year,
		StartDate: startDate,
		EndDate:   endDate,
		IsActive:  false,
	}

	if err := h.service.CreateSeasonWithWeeks(season, numWeeks); err != nil {
		logAndError(w, "Failed to create season", err, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/league/seasons", http.StatusSeeOther)
}
