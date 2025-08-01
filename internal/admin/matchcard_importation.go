package admin

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/services"
)

// MatchCardImportationHandler handles match card importation requests
type MatchCardImportationHandler struct {
	service     *Service
	templateDir string
}

// NewMatchCardImportationHandler creates a new match card importation handler
func NewMatchCardImportationHandler(service *Service, templateDir string) *MatchCardImportationHandler {
	return &MatchCardImportationHandler{
		service:     service,
		templateDir: templateDir,
	}
}

// MatchCardImportationPageData contains data for the match card importation template
type MatchCardImportationPageData struct {
	User        *models.User
	WeekOptions []int
	DefaultWeek int
	DefaultYear int
	MaxYear     int
}

// ImportRequest represents the form data for match card import
type ImportRequest struct {
	Week          int    `json:"week"`
	Year          int    `json:"year"`
	ClubName      string `json:"club_name"`
	ClubID        int    `json:"club_id"`
	ClubCode      string `json:"club_code"`
	ClearExisting bool   `json:"clear_existing"`
	DryRun        bool   `json:"dry_run"`
}

// ImportResponse represents the results to display to the user
type ImportResponse struct {
	Success          bool                   `json:"success"`
	Message          string                 `json:"message"`
	Results          *services.ImportResult `json:"results,omitempty"`
	Error            string                 `json:"error,omitempty"`
	NonceExtracted   bool                   `json:"nonce_extracted"`
	ExtractedNonce   string                 `json:"extracted_nonce,omitempty"`
	ProcessingTimeMS int64                  `json:"processing_time_ms"`
	DryRun           bool                   `json:"dry_run"`
}

// HandleMatchCardImportation handles both GET (show form) and POST (process import) requests
func (h *MatchCardImportationHandler) HandleMatchCardImportation(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.showImportForm(w, r)
	case http.MethodPost:
		h.processImport(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// showImportForm displays the match card importation page
func (h *MatchCardImportationHandler) showImportForm(w http.ResponseWriter, r *http.Request) {
	// Get user from context
	user, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	currentYear := time.Now().Year()

	// Generate week options (1-18)
	weekOptions := make([]int, 18)
	for i := 0; i < 18; i++ {
		weekOptions[i] = i + 1
	}

	// Determine current week (rough estimate based on time of year)
	now := time.Now()
	weekOfYear := int(now.YearDay()/7) + 1
	defaultWeek := 1
	if weekOfYear >= 10 && weekOfYear <= 27 { // Roughly March to July
		defaultWeek = min(weekOfYear-9, 18)
	}

	data := MatchCardImportationPageData{
		User:        user,
		WeekOptions: weekOptions,
		DefaultWeek: defaultWeek,
		DefaultYear: currentYear,
		MaxYear:     currentYear + 1, // Allow next year
	}

	// Load and parse the template
	tmpl, err := parseTemplate(h.templateDir, "admin/match_card_importation.html")
	if err != nil {
		logAndError(w, "Failed to parse template", err, http.StatusInternalServerError)
		return
	}

	// Execute the template
	if err := renderTemplate(w, tmpl, data); err != nil {
		logAndError(w, "Failed to render template", err, http.StatusInternalServerError)
		return
	}
}

// processImport processes the match card import request
func (h *MatchCardImportationHandler) processImport(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	// Parse form data
	if err := r.ParseForm(); err != nil {
		h.renderImportError(w, "Failed to parse form data", err)
		return
	}

	// Extract and validate form parameters
	req, err := h.parseImportRequest(r)
	if err != nil {
		h.renderImportError(w, "Invalid form data", err)
		return
	}

	// Validate parameters
	if err := h.validateImportRequest(req); err != nil {
		h.renderImportError(w, "Validation failed", err)
		return
	}

	// Create match card service
	matchCardService := services.NewMatchCardService(
		h.service.fixtureRepository,
		h.service.matchupRepository,
		h.service.teamRepository,
		h.service.clubRepository,
		h.service.playerRepository,
	)

	// Create import configuration
	config := services.ImportConfig{
		ClubName:              req.ClubName,
		ClubID:                req.ClubID,
		Year:                  req.Year,
		Nonce:                 "", // Will be auto-extracted
		ClubCode:              req.ClubCode,
		BaseURL:               "https://www.bhplta.co.uk/wp-admin/admin-ajax.php",
		RateLimit:             time.Second * 2, // 2 second rate limit
		DryRun:                req.DryRun,
		Verbose:               true, // Always verbose for web interface
		ClearExistingMatchups: req.ClearExisting,
	}

	// Run the import with auto-nonce extraction
	ctx := context.Background()
	result, err := matchCardService.ImportWeekMatchCardsWithAutoNonce(ctx, config, req.Week)

	processingTime := time.Since(startTime).Milliseconds()

	if err != nil {
		h.renderImportError(w, "Import failed", err)
		return
	}

	// Render success response
	response := ImportResponse{
		Success:          true,
		Message:          "Import completed successfully",
		Results:          result,
		NonceExtracted:   true,
		ProcessingTimeMS: processingTime,
		DryRun:           req.DryRun, // Track dry run status separately
	}

	h.renderImportSuccess(w, response)
}

// parseImportRequest extracts and parses form data into ImportRequest
func (h *MatchCardImportationHandler) parseImportRequest(r *http.Request) (*ImportRequest, error) {
	week, err := strconv.Atoi(r.FormValue("week"))
	if err != nil {
		return nil, fmt.Errorf("invalid week: %v", err)
	}

	year, err := strconv.Atoi(r.FormValue("year"))
	if err != nil {
		return nil, fmt.Errorf("invalid year: %v", err)
	}

	clubID, err := strconv.Atoi(r.FormValue("club_id"))
	if err != nil {
		return nil, fmt.Errorf("invalid club ID: %v", err)
	}

	return &ImportRequest{
		Week:          week,
		Year:          year,
		ClubName:      r.FormValue("club_name"),
		ClubID:        clubID,
		ClubCode:      r.FormValue("club_code"),
		ClearExisting: r.FormValue("clear_existing") == "on",
		DryRun:        r.FormValue("dry_run") == "on",
	}, nil
}

// validateImportRequest validates the import request parameters
func (h *MatchCardImportationHandler) validateImportRequest(req *ImportRequest) error {
	currentYear := time.Now().Year()

	if req.Week < 1 || req.Week > 18 {
		return fmt.Errorf("week must be between 1 and 18, got %d", req.Week)
	}

	if req.Year < currentYear {
		return fmt.Errorf("year cannot be before %d, got %d", currentYear, req.Year)
	}

	if req.Year > currentYear+1 {
		return fmt.Errorf("year cannot be more than one year in the future, got %d", req.Year)
	}

	if req.ClubName == "" {
		return fmt.Errorf("club name is required")
	}

	if req.ClubID <= 0 {
		return fmt.Errorf("club ID must be positive, got %d", req.ClubID)
	}

	if req.ClubCode == "" {
		return fmt.Errorf("club code/password is required")
	}

	return nil
}

// renderImportError renders an error response for the import
func (h *MatchCardImportationHandler) renderImportError(w http.ResponseWriter, message string, err error) {
	w.Header().Set("Content-Type", "text/html")

	errorHTML := fmt.Sprintf(`
		<div class="import-error">
			<h4>❌ %s</h4>
			<p><strong>Error:</strong> %v</p>
			<small class="text-muted">Please check your parameters and try again.</small>
		</div>
	`, message, err)

	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(errorHTML))
}

// renderImportSuccess renders a success response for the import
func (h *MatchCardImportationHandler) renderImportSuccess(w http.ResponseWriter, response ImportResponse) {
	w.Header().Set("Content-Type", "text/html")

	// Determine status message
	statusMessage := "✅ Import completed successfully"
	if response.DryRun {
		statusMessage = "🔍 Dry run completed successfully"
	}

	// Generate statistics HTML
	statsHTML := h.generateStatsHTML(response.Results)

	// Generate errors HTML if any
	errorsHTML := ""
	if len(response.Results.Errors) > 0 {
		errorsHTML = fmt.Sprintf(`
			<div class="mt-3">
				<h5>⚠️ Warnings/Errors (%d)</h5>
				<div class="alert alert-warning">
					<ul class="mb-0">
		`, len(response.Results.Errors))

		for _, err := range response.Results.Errors {
			errorsHTML += fmt.Sprintf("<li>%s</li>", err)
		}

		errorsHTML += `
					</ul>
				</div>
			</div>
		`
	}

	// Build complete response HTML
	successHTML := fmt.Sprintf(`
		<div class="import-success">
			<h4>%s</h4>
			<p><strong>Processing time:</strong> %dms</p>
			%s
		</div>
		%s
		<div class="mt-3">
			<small class="text-muted">
				<strong>Auto-n-once:</strong> ✅ Automatic n-once extraction successful<br>
				<strong>Database:</strong> %s<br>
				<strong>Timestamp:</strong> %s
			</small>
		</div>
	`,
		statusMessage,
		response.ProcessingTimeMS,
		statsHTML,
		errorsHTML,
		func() string {
			if response.DryRun {
				return "No changes made (dry run mode)"
			}
			return "Changes saved to database"
		}(),
		time.Now().Format("2006-01-02 15:04:05"),
	)

	w.Write([]byte(successHTML))
}

// generateStatsHTML creates HTML for displaying import statistics
func (h *MatchCardImportationHandler) generateStatsHTML(results *services.ImportResult) string {
	return fmt.Sprintf(`
		<div class="stats-grid">
			<div class="stat-item">
				<div class="stat-value">%d</div>
				<div class="stat-label">Matches Processed</div>
			</div>
			<div class="stat-item">
				<div class="stat-value">%d</div>
				<div class="stat-label">Fixtures Updated</div>
			</div>
			<div class="stat-item">
				<div class="stat-value">%d</div>
				<div class="stat-label">Matchups Created</div>
			</div>
			<div class="stat-item">
				<div class="stat-value">%d</div>
				<div class="stat-label">Matchups Updated</div>
			</div>
			<div class="stat-item">
				<div class="stat-value">%d</div>
				<div class="stat-label">Players Matched</div>
			</div>
			<div class="stat-item">
				<div class="stat-value">%d</div>
				<div class="stat-label">Total Errors</div>
			</div>
		</div>
	`,
		results.ProcessedMatches,
		results.UpdatedFixtures,
		results.CreatedMatchups,
		results.UpdatedMatchups,
		results.MatchedPlayers,
		len(results.Errors),
	)
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
