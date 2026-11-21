// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/models"
)

// SessionsHandler handles session-related requests
type SessionsHandler struct {
	service     *Service
	templateDir string
}

// NewSessionsHandler creates a new sessions handler
func NewSessionsHandler(service *Service, templateDir string) *SessionsHandler {
	return &SessionsHandler{
		service:     service,
		templateDir: templateDir,
	}
}

// HandleSessions handles session management routes
func (h *SessionsHandler) HandleSessions(w http.ResponseWriter, r *http.Request) {
	user, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	path := r.URL.Path

	// Route POST actions
	if r.Method == http.MethodPost {
		h.handleSessionAction(w, r, user, path)
		return
	}

	if r.Method == http.MethodGet {
		h.handleSessionsGet(w, r, user)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// handleSessionsGet renders the sessions management page
func (h *SessionsHandler) handleSessionsGet(w http.ResponseWriter, r *http.Request, user *models.User) {
	sessions, err := h.service.GetActiveSessions()
	if err != nil {
		logAndError(w, "Failed to load sessions", err, http.StatusInternalServerError)
		return
	}

	loginAttempts, err := h.service.GetRecentLoginAttempts(50)
	if err != nil {
		logAndError(w, "Failed to load login attempts", err, http.StatusInternalServerError)
		return
	}

	// The current session, to highlight it: its numeric id, as RequireAuth
	// put it in the context (nothing here reads the session cookie, so the
	// cookie's name and token never reach this page).
	currentSessionID := auth.SessionIDFromContext(r.Context())

	data := map[string]interface{}{
		"User":             user,
		"Sessions":         sessions,
		"LoginAttempts":    loginAttempts,
		"CurrentSessionID": currentSessionID,
		"SuccessMsg":       r.URL.Query().Get("success"),
		"ErrorMsg":         r.URL.Query().Get("error"),
	}

	tmpl, err := parseTemplate(h.templateDir, "admin/sessions.html")
	if err != nil {
		logAndError(w, "Failed to parse template", err, http.StatusInternalServerError)
		return
	}

	if err := renderTemplate(w, tmpl, data); err != nil {
		logAndError(w, "Failed to render template", err, http.StatusInternalServerError)
	}
}

// handleSessionAction routes POST actions for sessions
func (h *SessionsHandler) handleSessionAction(w http.ResponseWriter, r *http.Request, user *models.User, path string) {
	// POST /admin/league/sessions/cleanup
	if path == "/admin/league/sessions/cleanup" {
		if _, err := h.service.accounts.CleanupExpiredSessions(r.Context(), actorFor(user)); err != nil {
			log.Printf("Failed to cleanup sessions: %v", err)
			http.Redirect(w, r, "/admin/league/sessions?error=Failed+to+cleanup+sessions", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/admin/league/sessions?success=Expired+sessions+cleaned+up", http.StatusSeeOther)
		return
	}

	// POST /admin/league/sessions/invalidate-user/{userID}
	if strings.HasPrefix(path, "/admin/league/sessions/invalidate-user/") {
		userIDStr := strings.TrimPrefix(path, "/admin/league/sessions/invalidate-user/")
		userID, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid user ID", http.StatusBadRequest)
			return
		}
		if _, err := h.service.accounts.RevokeSessions(r.Context(), actorFor(user), userID); err != nil {
			log.Printf("Failed to invalidate user sessions: %v", err)
			http.Redirect(w, r, "/admin/league/sessions?error=Failed+to+invalidate+sessions", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/admin/league/sessions?success=All+sessions+for+user+invalidated", http.StatusSeeOther)
		return
	}

	// POST /admin/league/sessions/{id}/invalidate, id being the session's
	// numeric id (never its token)
	if strings.HasSuffix(path, "/invalidate") {
		idStr := strings.TrimSuffix(strings.TrimPrefix(path, "/admin/league/sessions/"), "/invalidate")
		sessionID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || sessionID <= 0 {
			http.Error(w, "Invalid session ID", http.StatusBadRequest)
			return
		}
		if err := h.service.accounts.RevokeSession(r.Context(), actorFor(user), sessionID); err != nil {
			log.Printf("Failed to invalidate session: %v", err)
			http.Redirect(w, r, "/admin/league/sessions?error=Failed+to+invalidate+session", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/admin/league/sessions?success=Session+invalidated", http.StatusSeeOther)
		return
	}

	http.Error(w, "Unknown action", http.StatusBadRequest)
}
