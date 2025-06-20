package players

import (
	"net/http"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/database"
)

// Handler represents the players handler
type Handler struct {
	service     *Service
	templateDir string

	// Sub-handlers for different domains
	availability *AvailabilityHandler
}

// New creates a new players handler
func New(db *database.DB, templateDir string) *Handler {
	service := NewService(db)

	return &Handler{
		service:      service,
		templateDir:  templateDir,
		availability: NewAvailabilityHandler(service, templateDir),
	}
}

// RegisterRoutes registers all player routes with the given mux
func (h *Handler) RegisterRoutes(mux *http.ServeMux, authMiddleware *auth.Middleware) {
	// Fantasy mixed doubles availability routes with token-based authentication
	// The auth token is embedded in the URL path: /my-availability/Sabalenka_Djokovic_Gauff_Sinner
	mux.Handle("/my-availability/", authMiddleware.RequireFantasyTokenAuth(
		http.HandlerFunc(h.availability.HandleAvailability),
	))
}
