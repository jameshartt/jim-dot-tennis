// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"jim-dot-tennis/internal/accounts"
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
		// Through the account service, like the users page: it checks the
		// player exists and is not linked to another account, and records the
		// change in the audit log. An empty player_id removes the link.
		playerID := strings.TrimSpace(r.FormValue("player_id"))
		if _, err := h.service.accounts.SetPlayerLink(r.Context(), actorFor(user), user.ID, playerID); err != nil {
			code := "failed"
			switch {
			case errors.Is(err, accounts.ErrPlayerLinked):
				code = "taken"
			case errors.Is(err, accounts.ErrPlayerNotFound):
				code = "unknown"
			default:
				log.Printf("planning link: user %d: %v", user.ID, err)
			}
			target := "/admin/league/planning/link?force=1&error=" + code
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", target)
				w.WriteHeader(http.StatusNoContent)
				return
			}
			http.Redirect(w, r, target, http.StatusSeeOther)
			return
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

// planningLinkErrors are the messages for a refused link (?error=<code>).
var planningLinkErrors = map[string]string{
	"taken":   "That player is already linked to another account. If that is you, ask an administrator to sort it out on the users page.",
	"unknown": "That player could not be found. Pick one from the list.",
	"failed":  "Something went wrong. Nothing was changed; please try again.",
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
		"Error":       planningLinkErrors[r.URL.Query().Get("error")],
	}
	if err := renderTemplate(w, tmpl, data); err != nil {
		log.Printf("planning link render failed: %v", err)
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
	}
}
