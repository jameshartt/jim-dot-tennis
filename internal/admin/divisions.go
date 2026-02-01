package admin

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"jim-dot-tennis/internal/models"
)

// DivisionsHandler handles division management requests
type DivisionsHandler struct {
	service     *Service
	templateDir string
}

// NewDivisionsHandler creates a new divisions handler
func NewDivisionsHandler(service *Service, templateDir string) *DivisionsHandler {
	return &DivisionsHandler{
		service:     service,
		templateDir: templateDir,
	}
}

// HandleDivisionEdit handles GET and POST for editing a division
func (h *DivisionsHandler) HandleDivisionEdit(w http.ResponseWriter, r *http.Request) {
	log.Printf("Division edit handler called with path: %s, method: %s", r.URL.Path, r.Method)

	_, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	// Extract division ID from path: /admin/league/divisions/{id}/edit
	divisionID, err := parseDivisionIDFromPath(r.URL.Path)
	if err != nil {
		http.Error(w, "Invalid division ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleEditGet(w, r, divisionID)
	case http.MethodPost:
		h.handleEditPost(w, r, divisionID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *DivisionsHandler) handleEditGet(w http.ResponseWriter, r *http.Request, divisionID uint) {
	division, err := h.service.GetDivisionByID(divisionID)
	if err != nil {
		logAndError(w, "Division not found", err, http.StatusNotFound)
		return
	}

	data := map[string]interface{}{
		"Division": division,
		"PlayDays": []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"},
	}

	tmpl, err := parseTemplate(h.templateDir, "admin/division_edit.html")
	if err != nil {
		logAndError(w, "Failed to parse template", err, http.StatusInternalServerError)
		return
	}

	if err := renderTemplate(w, tmpl, data); err != nil {
		logAndError(w, "Failed to render template", err, http.StatusInternalServerError)
	}
}

func (h *DivisionsHandler) handleEditPost(w http.ResponseWriter, r *http.Request, divisionID uint) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	// Get current division
	division, err := h.service.GetDivisionByID(divisionID)
	if err != nil {
		logAndError(w, "Division not found", err, http.StatusNotFound)
		return
	}

	// Validate and apply form values
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "Division name is required", http.StatusBadRequest)
		return
	}
	division.Name = name

	playDay := r.FormValue("play_day")
	if !isValidPlayDay(playDay) {
		http.Error(w, "Invalid play day", http.StatusBadRequest)
		return
	}
	division.PlayDay = playDay

	levelStr := r.FormValue("level")
	level, err := strconv.Atoi(levelStr)
	if err != nil || level < 1 {
		http.Error(w, "Invalid level", http.StatusBadRequest)
		return
	}
	division.Level = level

	maxTeamsStr := r.FormValue("max_teams_per_club")
	maxTeams, err := strconv.Atoi(maxTeamsStr)
	if err != nil || maxTeams < 1 {
		http.Error(w, "Invalid max teams per club", http.StatusBadRequest)
		return
	}
	division.MaxTeamsPerClub = maxTeams

	// Save
	if err := h.service.UpdateDivision(division); err != nil {
		logAndError(w, "Failed to update division", err, http.StatusInternalServerError)
		return
	}

	// Redirect back to season setup
	redirectURL := fmt.Sprintf("/admin/league/seasons/setup?id=%d", division.SeasonID)
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

func parseDivisionIDFromPath(path string) (uint, error) {
	// Path format: /admin/league/divisions/{id}/edit
	parts := strings.Split(strings.TrimPrefix(path, "/admin/league/divisions/"), "/")
	if len(parts) < 1 || parts[0] == "" {
		return 0, fmt.Errorf("missing division ID")
	}
	id, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return 0, err
	}
	return uint(id), nil
}

func isValidPlayDay(day string) bool {
	validDays := map[string]bool{
		"Monday": true, "Tuesday": true, "Wednesday": true,
		"Thursday": true, "Friday": true, "Saturday": true, "Sunday": true,
	}
	return validDays[day]
}

// GetDivisionByID retrieves a single division by ID
func (s *Service) GetDivisionByID(id uint) (*models.Division, error) {
	ctx := context.Background()
	return s.divisionRepository.FindByID(ctx, id)
}

// UpdateDivision updates an existing division
func (s *Service) UpdateDivision(division *models.Division) error {
	ctx := context.Background()
	return s.divisionRepository.Update(ctx, division)
}
