package admin

import (
	"log"
	"net/http"
)

// DashboardHandler handles dashboard-related requests
type DashboardHandler struct {
	service     *Service
	templateDir string
}

// NewDashboardHandler creates a new dashboard handler
func NewDashboardHandler(service *Service, templateDir string) *DashboardHandler {
	return &DashboardHandler{
		service:     service,
		templateDir: templateDir,
	}
}

// HandleDashboard serves the main admin dashboard
func (h *DashboardHandler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	log.Printf("Admin dashboard handler called with path: %s", r.URL.Path)

	// Only handle dashboard path
	if r.URL.Path != "/admin/dashboard" {
		log.Printf("Dashboard handler: not found for path: %s", r.URL.Path)
		http.NotFound(w, r)
		return
	}

	// Get user from context
	user, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}
	log.Printf("Admin dashboard requested by user: %s (role: %s)", user.Username, user.Role)

	// Get dashboard data
	dashboardData, err := h.service.GetDashboardData(user)
	if err != nil {
		logAndError(w, "Internal server error", err, http.StatusInternalServerError)
		return
	}

	// Load and parse the template
	tmpl, err := parseTemplate(h.templateDir, "admin_standalone.html")
	if err != nil {
		logAndError(w, "Internal server error", err, http.StatusInternalServerError)
		return
	}

	// Prepare template data
	templateData := map[string]interface{}{
		"User":          user,
		"Stats":         dashboardData.Stats,
		"LoginAttempts": dashboardData.LoginAttempts,
	}

	// Execute the template
	if err := renderTemplate(w, tmpl, templateData); err != nil {
		logAndError(w, err.Error(), err, http.StatusInternalServerError)
	}
}
