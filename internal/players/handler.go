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
	// Create players mux for all player routes
	playersMux := http.NewServeMux()

	// Fantasy mixed doubles availability routes
	playersMux.HandleFunc("/my-availability/", h.availability.HandleAvailability)

	// Register player routes with authentication middleware
	// Players can access their own availability (player role or higher)
	mux.Handle("/my-availability/", authMiddleware.RequireAuth(
		authMiddleware.RequireRole("player", "captain", "admin")(playersMux),
	))
}
