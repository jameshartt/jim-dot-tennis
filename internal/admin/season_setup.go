package admin

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"jim-dot-tennis/internal/models"
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

	// If teams were copied, redirect to away team review; otherwise back to setup
	if copyTeams {
		redirectURL := fmt.Sprintf("/admin/league/seasons/review-away-teams?id=%d&copied=true", targetSeasonID)
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
	} else {
		redirectURL := fmt.Sprintf("/admin/league/seasons/setup?id=%d&copied=true", targetSeasonID)
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
	}
}

// HandleReviewAwayTeams handles the post-copy away team review page
func (h *SeasonSetupHandler) HandleReviewAwayTeams(w http.ResponseWriter, r *http.Request) {
	user, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleReviewAwayTeamsGet(w, r, user)
	case http.MethodPost:
		h.handleReviewAwayTeamsPost(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleReviewAwayTeamsGet shows the away team review page
func (h *SeasonSetupHandler) handleReviewAwayTeamsGet(w http.ResponseWriter, r *http.Request, user *models.User) {
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

	season, err := h.service.GetSeasonByID(uint(seasonID))
	if err != nil {
		logAndError(w, "Failed to load season", err, http.StatusNotFound)
		return
	}

	reviewData, err := h.service.GetAwayTeamReviewData(uint(seasonID))
	if err != nil {
		logAndError(w, "Failed to load review data", err, http.StatusInternalServerError)
		return
	}

	divisions, _ := h.service.GetDivisionsBySeason(uint(seasonID))
	clubs, _ := h.service.GetAllClubs()

	successMsg := ""
	if r.URL.Query().Get("success") == "updated" {
		successMsg = "Away team review changes saved successfully."
	}
	if r.URL.Query().Get("copied") == "true" {
		successMsg = "Season copied successfully. Review the away teams below and make any adjustments."
	}

	tmpl, err := parseTemplate(h.templateDir, "admin/season_away_team_review.html")
	if err != nil {
		logAndError(w, "Failed to parse template", err, http.StatusInternalServerError)
		return
	}

	if err := renderTemplate(w, tmpl, map[string]interface{}{
		"User":       user,
		"Season":     season,
		"ReviewData": reviewData,
		"Divisions":  divisions,
		"Clubs":      clubs,
		"Success":    successMsg,
	}); err != nil {
		logAndError(w, err.Error(), err, http.StatusInternalServerError)
	}
}

// handleReviewAwayTeamsPost processes bulk changes from the review page
func (h *SeasonSetupHandler) handleReviewAwayTeamsPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		logAndError(w, "Failed to parse form data", err, http.StatusBadRequest)
		return
	}

	seasonIDStr := r.FormValue("season_id")
	seasonID, err := strconv.ParseUint(seasonIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid season ID", http.StatusBadRequest)
		return
	}

	// Process division changes and active status for existing teams
	for key, values := range r.Form {
		if len(values) == 0 {
			continue
		}

		// Handle division changes: team_123_division = "5"
		if strings.HasPrefix(key, "team_") && strings.HasSuffix(key, "_division") {
			teamIDStr := strings.TrimPrefix(key, "team_")
			teamIDStr = strings.TrimSuffix(teamIDStr, "_division")
			teamID, err := strconv.ParseUint(teamIDStr, 10, 32)
			if err != nil {
				continue
			}

			newDivID, err := strconv.ParseUint(values[0], 10, 32)
			if err != nil {
				continue
			}

			team, err := h.service.GetTeamByID(uint(teamID))
			if err != nil {
				continue
			}
			if team.DivisionID != uint(newDivID) {
				h.service.MoveTeamToDivision(uint(teamID), uint(newDivID))
			}
		}

		// Handle active/inactive: team_123_active = "on" (checked) or absent (unchecked)
		if strings.HasPrefix(key, "team_") && strings.HasSuffix(key, "_active") {
			teamIDStr := strings.TrimPrefix(key, "team_")
			teamIDStr = strings.TrimSuffix(teamIDStr, "_active")
			teamID, err := strconv.ParseUint(teamIDStr, 10, 32)
			if err != nil {
				continue
			}

			team, err := h.service.GetTeamByID(uint(teamID))
			if err != nil {
				continue
			}
			if !team.Active {
				team.Active = true
				h.service.UpdateTeam(team)
			}
		}
	}

	// Handle deactivations: teams whose active checkbox was NOT checked
	// We need to compare against all away team IDs in the form
	teamIDs := r.Form["review_team_ids"]
	activeTeamIDs := make(map[string]bool)
	for key := range r.Form {
		if strings.HasPrefix(key, "team_") && strings.HasSuffix(key, "_active") {
			teamIDStr := strings.TrimPrefix(key, "team_")
			teamIDStr = strings.TrimSuffix(teamIDStr, "_active")
			activeTeamIDs[teamIDStr] = true
		}
	}
	for _, tidStr := range teamIDs {
		if activeTeamIDs[tidStr] {
			continue // already handled above
		}
		teamID, err := strconv.ParseUint(tidStr, 10, 32)
		if err != nil {
			continue
		}
		team, err := h.service.GetTeamByID(uint(teamID))
		if err != nil {
			continue
		}
		if team.Active {
			team.Active = false
			h.service.UpdateTeam(team)
		}
	}

	// Handle new team creation
	newTeamNames := r.Form["new_team_name"]
	newTeamClubs := r.Form["new_team_club"]
	newTeamDivisions := r.Form["new_team_division"]

	for i := 0; i < len(newTeamNames); i++ {
		name := strings.TrimSpace(newTeamNames[i])
		if name == "" {
			continue
		}
		var clubID, divisionID uint64
		if i < len(newTeamClubs) {
			clubID, _ = strconv.ParseUint(newTeamClubs[i], 10, 32)
		}
		if i < len(newTeamDivisions) {
			divisionID, _ = strconv.ParseUint(newTeamDivisions[i], 10, 32)
		}
		if clubID == 0 || divisionID == 0 {
			continue
		}

		team := &models.Team{
			Name:       name,
			ClubID:     uint(clubID),
			DivisionID: uint(divisionID),
			SeasonID:   uint(seasonID),
		}
		h.service.CreateTeam(team)
	}

	http.Redirect(w, r, fmt.Sprintf("/admin/league/seasons/review-away-teams?id=%d&success=updated", seasonID), http.StatusSeeOther)
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
