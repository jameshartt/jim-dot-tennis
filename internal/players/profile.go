// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package players

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/models"
)

// ProfileHandler handles player profile requests
type ProfileHandler struct {
	service     *Service
	templateDir string
	myTennis    *MyTennisHandler
}

// NewProfileHandler creates a new profile handler
func NewProfileHandler(service *Service, templateDir string) *ProfileHandler {
	return &ProfileHandler{
		service:     service,
		templateDir: templateDir,
		myTennis:    NewMyTennisHandler(service, templateDir),
	}
}

// HandleProfile handles player profile routes
func (h *ProfileHandler) HandleProfile(w http.ResponseWriter, r *http.Request) {
	log.Printf("Player profile handler called with path: %s, method: %s", r.URL.Path, r.Method)

	// Extract auth token from URL path
	// Expected format: /my-profile/Sabalenka_Alcaraz_Guaff_Sinner[/action]
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/my-profile/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		http.Error(w, "Invalid profile URL - missing auth token", http.StatusBadRequest)
		return
	}
	authToken := pathParts[0]
	action := ""
	if len(pathParts) > 1 {
		action = pathParts[1]
	}

	// Get player from fantasy token context (set by RequireFantasyTokenAuth middleware)
	player, err := auth.GetPlayerFromContext(r.Context())
	if err != nil {
		logAndError(w, "Player not found in context", err, http.StatusUnauthorized)
		return
	}

	log.Printf("Authenticated player: %s %s (ID: %s) for auth token: %s, action: %s",
		player.FirstName, player.LastName, player.ID, authToken, action)

	// Route based on action and method.
	// GET /my-profile/{token}         → blank 'My Tennis' form (write-only; WI-095/WI-097)
	// POST /my-profile/{token}        → merge-save partial preferences (WI-097)
	// GET /my-profile/{token}/history → match history (initials-only; WI-093)
	switch {
	case action == "" && r.Method == http.MethodGet:
		h.myTennis.HandleGet(w, r, &player, authToken)
	case action == "" && r.Method == http.MethodPost:
		h.myTennis.HandlePost(w, r, &player, authToken)
	case action == "history" && r.Method == http.MethodGet:
		h.handleMatchHistory(w, r, &player, authToken)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleMatchHistory displays the match history page for a player
func (h *ProfileHandler) handleMatchHistory(w http.ResponseWriter, r *http.Request, player *models.Player, authToken string) {
	// Optional season filter
	var seasonID *uint
	seasonParam := r.URL.Query().Get("season")
	if seasonParam != "" {
		sid, err := strconv.ParseUint(seasonParam, 10, 32)
		if err == nil {
			s := uint(sid)
			seasonID = &s
		}
	}

	records, stats, err := h.service.GetPlayerMatchHistory(player.ID, seasonID)
	if err != nil {
		log.Printf("Failed to load match history for player %s: %v", player.ID, err)
		http.Error(w, "Failed to load match history", http.StatusInternalServerError)
		return
	}

	// Get all seasons for the selector
	allSeasons, _ := h.service.seasonRepository.FindAll(r.Context())

	// Get active season for default
	activeSeason, _ := h.service.seasonRepository.FindActive(r.Context())

	tmpl, err := parseTemplate(h.templateDir, "players/match_history.html")
	if err != nil {
		log.Printf("Error parsing match history template: %v", err)
		renderFallbackHTML(w, "Match History", "Match History",
			"Match history page - template error",
			fmt.Sprintf("/my-profile/%s", authToken))
		return
	}

	var selectedSeasonID uint
	if seasonID != nil {
		selectedSeasonID = *seasonID
	}

	if err := renderTemplate(w, tmpl, map[string]interface{}{
		"Player":           player,
		"Records":          records,
		"Stats":            stats,
		"AuthToken":        authToken,
		"Seasons":          allSeasons,
		"ActiveSeason":     activeSeason,
		"SelectedSeasonID": selectedSeasonID,
	}); err != nil {
		logAndError(w, err.Error(), err, http.StatusInternalServerError)
	}
}
