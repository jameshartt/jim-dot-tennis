package admin

import (
	"log"
	"net/http"

	"jim-dot-tennis/internal/models"
)

// UsersHandler handles user-related requests
type UsersHandler struct {
	service     *Service
	templateDir string
}

// NewUsersHandler creates a new users handler
func NewUsersHandler(service *Service, templateDir string) *UsersHandler {
	return &UsersHandler{
		service:     service,
		templateDir: templateDir,
	}
}

// HandleUsers handles user management routes
func (h *UsersHandler) HandleUsers(w http.ResponseWriter, r *http.Request) {
	log.Printf("Admin users handler called with path: %s, method: %s", r.URL.Path, r.Method)

	// Get user from context
	user, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleUsersGet(w, r, user)
	case http.MethodPost:
		h.handleUsersPost(w, r, user)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleUsersGet handles GET requests for user management
func (h *UsersHandler) handleUsersGet(w http.ResponseWriter, r *http.Request, user *models.User) {
	// TODO: Implement users view
	// For now, render a simple placeholder page
	renderFallbackHTML(w, "Admin - Users", "User Management",
		"User management page - coming soon", "/admin")
}

// handleUsersPost handles POST requests for user management
func (h *UsersHandler) handleUsersPost(w http.ResponseWriter, r *http.Request, user *models.User) {
	// TODO: Implement user creation/update/delete
	http.Error(w, "User operations not yet implemented", http.StatusNotImplemented)
}
