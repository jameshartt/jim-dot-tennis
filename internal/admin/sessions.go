package admin

import (
	"log"
	"net/http"

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
	log.Printf("Admin sessions handler called with path: %s, method: %s", r.URL.Path, r.Method)

	// Get user from context
	user, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleSessionsGet(w, r, user)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSessionsGet handles GET requests for sessions management
func (h *SessionsHandler) handleSessionsGet(w http.ResponseWriter, r *http.Request, user *models.User) {
	// TODO: Implement sessions view
	// For now, render a simple placeholder page
	renderFallbackHTML(w, "Admin - Sessions", "Session Management",
		"Sessions management page - coming soon", "/admin/league")
}
