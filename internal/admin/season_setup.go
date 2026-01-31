package admin

import (
	"fmt"
	"net/http"
	"strconv"
)

// SeasonSetupHandler handles season setup requests
type SeasonSetupHandler struct {
	service     *Service
	templateDir string
}

// NewSeasonSetupHandler creates a new season setup handler
func NewSeasonSetupHandler(service *Service, templateDir string) *SeasonSetupHandler {
	return &SeasonSetupHandler{
		service:     service,
		templateDir: templateDir,
	}
}

// HandleSeasonSetup handles the season setup page
func (h *SeasonSetupHandler) HandleSeasonSetup(w http.ResponseWriter, r *http.Request) {
	user, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	// Get season ID from query params
	seasonIDStr := r.URL.Query().Get("id")
	if seasonIDStr == "" {
		http.Error(w, "Season ID required", http.StatusBadRequest)
		return
	}

	seasonID, err := strconv.ParseUint(seasonIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid season ID", http.StatusBadRequest)
		return
	}

	// Get season details
	season, err := h.service.GetSeasonByID(uint(seasonID))
	if err != nil {
		logAndError(w, "Failed to load season", err, http.StatusNotFound)
		return
	}

	// Get setup data
	setupData, err := h.service.GetSeasonSetupData(uint(seasonID))
	if err != nil {
		logAndError(w, "Failed to load season setup data", err, http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"User":         user,
		"Season":       season,
		"SetupData":    setupData,
		"PreviousYear": season.Year - 1,
	}

	// Load and render template
	tmpl, err := parseTemplate(h.templateDir, "admin/season_setup.html")
	if err != nil {
		logAndError(w, "Failed to parse template", err, http.StatusInternalServerError)
		return
	}

	if err := renderTemplate(w, tmpl, data); err != nil {
		logAndError(w, "Failed to render template", err, http.StatusInternalServerError)
	}
}

// HandleCopyFromPreviousSeason copies divisions and teams from the previous season
func (h *SeasonSetupHandler) HandleCopyFromPreviousSeason(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	targetSeasonID, _ := strconv.ParseUint(r.FormValue("season_id"), 10, 32)
	copyDivisions := r.FormValue("copy_divisions") == "on"
	copyTeams := r.FormValue("copy_teams") == "on"

	if targetSeasonID == 0 {
		http.Error(w, "Season ID required", http.StatusBadRequest)
		return
	}

	if !copyDivisions && !copyTeams {
		http.Error(w, "Select at least divisions or teams to copy", http.StatusBadRequest)
		return
	}

	// Copy from previous season
	err := h.service.CopyFromPreviousSeason(uint(targetSeasonID), copyDivisions, copyTeams)
	if err != nil {
		logAndError(w, "Failed to copy from previous season", err, http.StatusInternalServerError)
		return
	}

	// Redirect back to setup page with success message
	redirectURL := fmt.Sprintf("/admin/league/seasons/setup?id=%d&copied=true", targetSeasonID)
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// HandleMoveTeam handles promoting/demoting a team
func (h *SeasonSetupHandler) HandleMoveTeam(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	teamID, _ := strconv.ParseUint(r.FormValue("team_id"), 10, 32)
	targetDivisionID, _ := strconv.ParseUint(r.FormValue("target_division_id"), 10, 32)
	seasonID, _ := strconv.ParseUint(r.FormValue("season_id"), 10, 32)

	if teamID == 0 || targetDivisionID == 0 || seasonID == 0 {
		http.Error(w, "Missing required parameters", http.StatusBadRequest)
		return
	}

	if err := h.service.MoveTeamToDivision(uint(teamID), uint(targetDivisionID)); err != nil {
		logAndError(w, "Failed to move team", err, http.StatusInternalServerError)
		return
	}

	// Redirect back to setup page
	redirectURL := fmt.Sprintf("/admin/league/seasons/setup?id=%d&moved=true", seasonID)
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}
