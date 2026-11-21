// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"log"
	"net/http"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
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
	if r.URL.Path != "/admin/league/dashboard" {
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

	// Fixtures awaiting reschedule (window passed, no result) — surfaced on the
	// dashboard as a prominent nudge for captains to set a new date.
	_, awaitingReschedule, err := h.service.GetHomeClubAwaitingRescheduleFixtures()
	if err != nil {
		log.Printf("Failed to load fixtures awaiting reschedule for dashboard: %v", err)
		awaitingReschedule = nil // Non-fatal: dashboard still renders without the nudge
	}

	// Passkeys blocked because their sign counter went backwards (a possible
	// copy): administrators are told here and review them on the sign-in
	// methods page.
	passkeyWarnings := 0
	if user.Role == models.RoleAdmin {
		warnings, err := repository.NewWebAuthnCredentialRepository(h.service.db).CloneWarnings(r.Context())
		if err != nil {
			log.Printf("Failed to load passkey warnings for dashboard: %v", err)
		}
		passkeyWarnings = len(warnings)
	}

	// Prepare template data
	templateData := map[string]interface{}{
		"PasskeyWarnings":    passkeyWarnings,
		"SignInMethodsPath":  auth.SignInMethodsPath,
		"User":               user,
		"Stats":              dashboardData.Stats,
		"LoginAttempts":      dashboardData.LoginAttempts,
		"AwaitingReschedule": awaitingReschedule,
		"Features":           h.service.clubSettings.Features(),
	}

	// Execute the template
	if err := renderTemplate(w, tmpl, templateData); err != nil {
		logAndError(w, err.Error(), err, http.StatusInternalServerError)
	}
}
