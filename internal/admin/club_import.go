package admin

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"jim-dot-tennis/internal/services"
)

// ClubImportHandler handles club data import requests
type ClubImportHandler struct {
	service     *Service
	templateDir string
}

// NewClubImportHandler creates a new club import handler
func NewClubImportHandler(service *Service, templateDir string) *ClubImportHandler {
	return &ClubImportHandler{
		service:     service,
		templateDir: templateDir,
	}
}

// HandleClubImport handles both GET (show form) and POST (process import) requests
func (h *ClubImportHandler) HandleClubImport(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.showImportForm(w, r)
	case http.MethodPost:
		h.processImport(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// ClubImportPageData contains data for the club import template
type ClubImportPageData struct {
	User      interface{}
	ClubSlugs []services.ClubSlugMapping
}

// showImportForm displays the club data import page
func (h *ClubImportHandler) showImportForm(w http.ResponseWriter, r *http.Request) {
	user, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	data := ClubImportPageData{
		User:      user,
		ClubSlugs: services.KnownClubSlugs,
	}

	tmpl, err := parseTemplate(h.templateDir, "admin/club_import.html")
	if err != nil {
		logAndError(w, "Failed to parse template", err, http.StatusInternalServerError)
		return
	}

	if err := renderTemplate(w, tmpl, data); err != nil {
		logAndError(w, "Failed to render template", err, http.StatusInternalServerError)
	}
}

// processImport processes the club data import request
func (h *ClubImportHandler) processImport(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	if err := r.ParseForm(); err != nil {
		h.renderImportError(w, "Failed to parse form data", err)
		return
	}

	slug := r.FormValue("slug")
	dryRun := r.FormValue("dry_run") == "on"

	scraper := services.NewClubScraper(h.service.db, h.service.clubRepository, dryRun, true)
	ctx := context.Background()

	var summary *services.ClubScraperSummary
	var err error

	if slug != "" {
		summary, err = scraper.ScrapeClub(ctx, slug)
	} else {
		summary, err = scraper.ScrapeAll(ctx)
	}

	processingTime := time.Since(startTime).Milliseconds()

	if err != nil {
		h.renderImportError(w, "Import failed", err)
		return
	}

	h.renderImportSuccess(w, summary, dryRun, processingTime)
}

// renderImportError renders an error response for the import
func (h *ClubImportHandler) renderImportError(w http.ResponseWriter, message string, err error) {
	w.Header().Set("Content-Type", "text/html")

	errorHTML := fmt.Sprintf(`
		<div class="import-error" style="padding: 1rem; background: #f8d7da; border: 1px solid #f5c6cb; border-radius: 8px; color: #721c24;">
			<h4>Import Error</h4>
			<p><strong>Error:</strong> %v</p>
			<small>Please check your parameters and try again.</small>
		</div>
	`, err)

	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(errorHTML))
}

// renderImportSuccess renders a success response for the import
func (h *ClubImportHandler) renderImportSuccess(w http.ResponseWriter, summary *services.ClubScraperSummary, dryRun bool, processingTimeMS int64) {
	w.Header().Set("Content-Type", "text/html")

	statusMessage := "Import completed successfully"
	if dryRun {
		statusMessage = "Dry run completed successfully"
	}

	errorsHTML := ""
	if len(summary.Errors) > 0 {
		errorsHTML = fmt.Sprintf(`
			<div style="margin-top: 1rem;">
				<h5>Warnings/Errors (%d)</h5>
				<div style="padding: 0.75rem; background: #fff3cd; border: 1px solid #ffc107; border-radius: 4px;">
					<ul style="margin: 0; padding-left: 1.5rem;">
		`, len(summary.Errors))

		for _, e := range summary.Errors {
			errorsHTML += fmt.Sprintf("<li>%s</li>", e)
		}

		errorsHTML += `
					</ul>
				</div>
			</div>
		`
	}

	successHTML := fmt.Sprintf(`
		<div class="import-success" style="padding: 1rem; background: #d4edda; border: 1px solid #c3e6cb; border-radius: 8px; color: #155724;">
			<h4>%s</h4>
			<p><strong>Processing time:</strong> %dms</p>
			<div class="stats-grid" style="display: grid; grid-template-columns: repeat(auto-fit, minmax(120px, 1fr)); gap: 0.75rem; margin-top: 1rem;">
				<div style="text-align: center; padding: 0.75rem; background: white; border-radius: 4px;">
					<div style="font-size: 1.5rem; font-weight: bold;">%d</div>
					<div style="font-size: 0.875rem; color: #666;">Matched</div>
				</div>
				<div style="text-align: center; padding: 0.75rem; background: white; border-radius: 4px;">
					<div style="font-size: 1.5rem; font-weight: bold;">%d</div>
					<div style="font-size: 0.875rem; color: #666;">Created</div>
				</div>
				<div style="text-align: center; padding: 0.75rem; background: white; border-radius: 4px;">
					<div style="font-size: 1.5rem; font-weight: bold;">%d</div>
					<div style="font-size: 0.875rem; color: #666;">Updated</div>
				</div>
				<div style="text-align: center; padding: 0.75rem; background: white; border-radius: 4px;">
					<div style="font-size: 1.5rem; font-weight: bold;">%d</div>
					<div style="font-size: 0.875rem; color: #666;">Skipped</div>
				</div>
			</div>
		</div>
		%s
		<div style="margin-top: 1rem;">
			<small style="color: #666;">
				<strong>Database:</strong> %s<br>
				<strong>Timestamp:</strong> %s
			</small>
		</div>
	`,
		statusMessage,
		processingTimeMS,
		summary.Matched,
		summary.Created,
		summary.Updated,
		summary.Skipped,
		errorsHTML,
		func() string {
			if dryRun {
				return "No changes made (dry run mode)"
			}
			return "Changes saved to database"
		}(),
		time.Now().Format("2006-01-02 15:04:05"),
	)

	w.Write([]byte(successHTML))
}
