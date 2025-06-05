package admin

import (
	"log"
	"net/http"
	"strings"

	"jim-dot-tennis/internal/models"
)

// FixturesHandler handles fixture-related requests
type FixturesHandler struct {
	service     *Service
	templateDir string
}

// NewFixturesHandler creates a new fixtures handler
func NewFixturesHandler(service *Service, templateDir string) *FixturesHandler {
	return &FixturesHandler{
		service:     service,
		templateDir: templateDir,
	}
}

// HandleFixtures handles fixture management routes
func (h *FixturesHandler) HandleFixtures(w http.ResponseWriter, r *http.Request) {
	log.Printf("Admin fixtures handler called with path: %s, method: %s", r.URL.Path, r.Method)

	// Check if this is a specific fixture detail request
	if strings.Contains(r.URL.Path, "/fixtures/") && r.URL.Path != "/admin/fixtures/" {
		h.handleFixtureDetail(w, r)
		return
	}

	// Get user from context
	user, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleFixturesGet(w, r, user)
	case http.MethodPost:
		h.handleFixturesPost(w, r, user)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleFixturesGet handles GET requests for fixture management
func (h *FixturesHandler) handleFixturesGet(w http.ResponseWriter, r *http.Request, user *models.User) {
	// Get St. Ann's fixtures with related data
	club, fixtures, err := h.service.GetStAnnsFixtures()
	if err != nil {
		logAndError(w, "Failed to load fixtures", err, http.StatusInternalServerError)
		return
	}

	// Load the fixtures template
	tmpl, err := parseTemplate(h.templateDir, "admin/fixtures.html")
	if err != nil {
		log.Printf("Error parsing fixtures template: %v", err)
		// Fallback to simple HTML response
		renderFallbackHTML(w, "Admin - Fixtures", "Fixture Management",
			"Fixture management page - coming soon", "/admin")
		return
	}

	// Execute the template with data
	if err := renderTemplate(w, tmpl, map[string]interface{}{
		"User":     user,
		"Club":     club,
		"Fixtures": fixtures,
	}); err != nil {
		logAndError(w, err.Error(), err, http.StatusInternalServerError)
	}
}

// handleFixturesPost handles POST requests for fixture management
func (h *FixturesHandler) handleFixturesPost(w http.ResponseWriter, r *http.Request, user *models.User) {
	// TODO: Implement fixture creation/update/delete
	http.Error(w, "Fixture operations not yet implemented", http.StatusNotImplemented)
}

// handleFixtureDetail handles requests for individual fixture details
func (h *FixturesHandler) handleFixtureDetail(w http.ResponseWriter, r *http.Request) {
	log.Printf("Admin fixture detail handler called with path: %s, method: %s", r.URL.Path, r.Method)

	// Get user from context
	user, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	// Extract fixture ID from URL path
	fixtureID, err := parseIDFromPath(r.URL.Path, "/admin/fixtures/")
	if err != nil {
		logAndError(w, "Invalid fixture ID", err, http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleFixtureDetailGet(w, r, user, fixtureID)
	case http.MethodPost:
		h.handleFixtureDetailPost(w, r, user, fixtureID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleFixtureDetailGet handles GET requests to show the fixture detail page
func (h *FixturesHandler) handleFixtureDetailGet(w http.ResponseWriter, r *http.Request, user *models.User, fixtureID uint) {
	// Get the fixture with full details
	fixtureDetail, err := h.service.GetFixtureDetail(fixtureID)
	if err != nil {
		logAndError(w, "Fixture not found", err, http.StatusNotFound)
		return
	}

	// Load the fixture detail template
	tmpl, err := parseTemplate(h.templateDir, "admin/fixture_detail.html")
	if err != nil {
		log.Printf("Error parsing fixture detail template: %v", err)
		// Fallback to simple HTML response
		renderFallbackHTML(w, "Fixture Detail", "Fixture Detail",
			"Fixture detail page - coming soon", "/admin/fixtures")
		return
	}

	// Execute the template with data
	if err := renderTemplate(w, tmpl, map[string]interface{}{
		"User":          user,
		"FixtureDetail": fixtureDetail,
	}); err != nil {
		logAndError(w, err.Error(), err, http.StatusInternalServerError)
	}
}

// handleFixtureDetailPost handles POST requests to update fixture details
func (h *FixturesHandler) handleFixtureDetailPost(w http.ResponseWriter, r *http.Request, user *models.User, fixtureID uint) {
	// TODO: Implement fixture detail updates
	http.Error(w, "Fixture detail updates not yet implemented", http.StatusNotImplemented)
}
