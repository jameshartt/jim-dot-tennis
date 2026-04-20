// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"log"
	"net/http"
	"strings"
)

// PlanningLinkHandler handles the one-time 'I am…' picker that writes
// users.player_id so the planning dashboard can personalise 'My Teams'.
type PlanningLinkHandler struct {
	service     *Service
	templateDir string
}

// NewPlanningLinkHandler creates a new planning-link handler.
func NewPlanningLinkHandler(service *Service, templateDir string) *PlanningLinkHandler {
	return &PlanningLinkHandler{service: service, templateDir: templateDir}
}

// HandleLink renders the picker (GET) or writes the chosen player id (POST).
// Both actions are admin-session authed upstream.
func (h *PlanningLinkHandler) HandleLink(w http.ResponseWriter, r *http.Request) {
	user, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.renderPicker(w, r)
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			logAndError(w, "bad form", err, http.StatusBadRequest)
			return
		}
		playerID := strings.TrimSpace(r.FormValue("player_id"))
		if playerID == "" {
			// POST with empty player_id unlinks
			if err := h.service.SetPlayerIDOnUser(r.Context(), user.ID, nil); err != nil {
				logAndError(w, "unlink failed", err, http.StatusInternalServerError)
				return
			}
		} else {
			if err := h.service.SetPlayerIDOnUser(r.Context(), user.ID, &playerID); err != nil {
				logAndError(w, "link failed", err, http.StatusInternalServerError)
				return
			}
		}
		// HTMX requests get a small acknowledgement header so the client can
		// reload the dashboard in place. Full-page POSTs get a redirect.
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("HX-Redirect", "/admin/league/planning")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.Redirect(w, r, "/admin/league/planning", http.StatusSeeOther)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// renderPicker renders the 'I am…' picker. Also used as the 'unlink/relink'
// affordance from the dashboard settings icon.
func (h *PlanningLinkHandler) renderPicker(w http.ResponseWriter, r *http.Request) {
	user, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	players, err := h.service.ListHomeClubPlayersForLink(r.Context())
	if err != nil {
		logAndError(w, "could not load players", err, http.StatusInternalServerError)
		return
	}

	tmpl, err := parseTemplate(h.templateDir, "admin/planning_link.html")
	if err != nil {
		logAndError(w, "template parse failed", err, http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"User":        user,
		"Players":     players,
		"LinkedID":    user.PlayerID,
		"ClubName":    homeClubNameFromContext(r),
		"IsForceShow": r.URL.Query().Get("force") == "1",
	}
	if err := renderTemplate(w, tmpl, data); err != nil {
		log.Printf("planning link render failed: %v", err)
	}
}
