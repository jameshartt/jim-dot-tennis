package admin

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"jim-dot-tennis/internal/models"
)

// SeasonsHandler handles season-related requests
type SeasonsHandler struct {
	service     *Service
	templateDir string
}

// NewSeasonsHandler creates a new seasons handler
func NewSeasonsHandler(service *Service, templateDir string) *SeasonsHandler {
	return &SeasonsHandler{
		service:     service,
		templateDir: templateDir,
	}
}

// HandleSeasons handles season management routes
func (h *SeasonsHandler) HandleSeasons(w http.ResponseWriter, r *http.Request) {
	log.Printf("Admin seasons handler called with path: %s, method: %s", r.URL.Path, r.Method)

	switch r.Method {
	case http.MethodGet:
		h.handleSeasonsList(w, r)
	case http.MethodPost:
		h.handleCreateSeason(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleSetActiveSeason handles setting the active season
func (h *SeasonsHandler) HandleSetActiveSeason(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	seasonIDStr := r.URL.Query().Get("id")
	seasonID, err := strconv.ParseUint(seasonIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid season ID", http.StatusBadRequest)
		return
	}

	if err := h.service.SetActiveSeason(uint(seasonID)); err != nil {
		logAndError(w, "Failed to set active season", err, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/league/seasons", http.StatusSeeOther)
}

// handleSeasonsList displays the list of seasons
func (h *SeasonsHandler) handleSeasonsList(w http.ResponseWriter, r *http.Request) {
	seasons, err := h.service.GetAllSeasons()
	if err != nil {
		logAndError(w, "Failed to load seasons", err, http.StatusInternalServerError)
		return
	}

	activeSeason, _ := h.service.GetActiveSeason()

	// Simple HTML response
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`
<!DOCTYPE html>
<html>
<head>
    <title>Season Management</title>
    <link rel="stylesheet" href="/static/css/main.css">
    <style>
        body { font-family: Arial, sans-serif; max-width: 1200px; margin: 0 auto; padding: 20px; }
        h1 { color: #2c5530; }
        .seasons-table { width: 100%; border-collapse: collapse; margin: 20px 0; }
        .seasons-table th, .seasons-table td { border: 1px solid #ddd; padding: 12px; text-align: left; }
        .seasons-table th { background-color: #4a7c59; color: white; }
        .seasons-table tr:nth-child(even) { background-color: #f8f9fa; }
        .active-badge { background: #28a745; color: white; padding: 4px 8px; border-radius: 4px; font-size: 12px; }
        .btn { padding: 8px 16px; border: none; border-radius: 4px; cursor: pointer; text-decoration: none; display: inline-block; }
        .btn-primary { background: #4a7c59; color: white; }
        .btn-secondary { background: #6c757d; color: white; }
        .btn:hover { opacity: 0.9; }
        .create-form { background: #f8f9fa; padding: 20px; border-radius: 8px; margin: 20px 0; }
        .form-group { margin-bottom: 15px; }
        .form-group label { display: block; margin-bottom: 5px; font-weight: bold; }
        .form-group input, .form-group select { width: 100%; padding: 8px; border: 1px solid #ddd; border-radius: 4px; }
    </style>
</head>
<body>
    <h1>Season Management</h1>
    <p><a href="/admin/league/dashboard" class="btn btn-secondary">← Back to Dashboard</a></p>

    <div class="create-form">
        <h2>Create New Season</h2>
        <form method="POST" action="/admin/league/seasons">
            <div class="form-group">
                <label for="name">Season Name:</label>
                <input type="text" id="name" name="name" required placeholder="e.g., 2026 Season">
            </div>
            <div class="form-group">
                <label for="year">Year:</label>
                <input type="number" id="year" name="year" required value="2026">
            </div>
            <div class="form-group">
                <label for="start_date">Start Date:</label>
                <input type="date" id="start_date" name="start_date" required>
                <small style="color: #666; font-size: 12px;">Date picker will display in your local format</small>
            </div>
            <div class="form-group">
                <label for="end_date">End Date:</label>
                <input type="date" id="end_date" name="end_date" required>
                <small style="color: #666; font-size: 12px;">Date picker will display in your local format</small>
            </div>
            <button type="submit" class="btn btn-primary">Create Season</button>
        </form>
    </div>

    <h2>Existing Seasons</h2>
    <table class="seasons-table">
        <thead>
            <tr>
                <th>ID</th>
                <th>Name</th>
                <th>Year</th>
                <th>Start Date</th>
                <th>End Date</th>
                <th>Status</th>
                <th>Actions</th>
            </tr>
        </thead>
        <tbody>
`))

	for _, season := range seasons {
		isActive := activeSeason != nil && season.ID == activeSeason.ID
		statusBadge := ""
		if isActive {
			statusBadge = `<span class="active-badge">ACTIVE</span>`
		}

		activeButton := ""
		if !isActive {
			activeButton = `<form method="POST" action="/admin/league/seasons/set-active?id=` + strconv.Itoa(int(season.ID)) + `" style="display: inline;">
                <button type="submit" class="btn btn-secondary">Set Active</button>
            </form>`
		}

		w.Write([]byte(`
            <tr>
                <td>` + strconv.Itoa(int(season.ID)) + `</td>
                <td>` + season.Name + `</td>
                <td>` + strconv.Itoa(season.Year) + `</td>
                <td class="date-cell" data-date="` + season.StartDate.Format("2006-01-02") + `">` + season.StartDate.Format("02 Jan 2006") + `</td>
                <td class="date-cell" data-date="` + season.EndDate.Format("2006-01-02") + `">` + season.EndDate.Format("02 Jan 2006") + `</td>
                <td>` + statusBadge + `</td>
                <td>` + activeButton + `</td>
            </tr>
        `))
	}

	w.Write([]byte(`
        </tbody>
    </table>

    <script>
        // Format dates based on user's locale
        document.addEventListener('DOMContentLoaded', function() {
            const dateCells = document.querySelectorAll('.date-cell');
            dateCells.forEach(cell => {
                const isoDate = cell.getAttribute('data-date');
                if (isoDate) {
                    const date = new Date(isoDate);
                    // Format: e.g., "31/01/2026" for en-GB or "1/31/2026" for en-US
                    cell.textContent = date.toLocaleDateString();
                }
            });
        });
    </script>
</body>
</html>
`))
}

// handleCreateSeason handles POST request to create a new season
func (h *SeasonsHandler) handleCreateSeason(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	yearStr := r.FormValue("year")
	startDateStr := r.FormValue("start_date")
	endDateStr := r.FormValue("end_date")

	year, err := strconv.Atoi(yearStr)
	if err != nil {
		http.Error(w, "Invalid year", http.StatusBadRequest)
		return
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		http.Error(w, "Invalid start date", http.StatusBadRequest)
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		http.Error(w, "Invalid end date", http.StatusBadRequest)
		return
	}

	season := &models.Season{
		Name:      name,
		Year:      year,
		StartDate: startDate,
		EndDate:   endDate,
		IsActive:  false,
	}

	if err := h.service.CreateSeason(season); err != nil {
		logAndError(w, "Failed to create season", err, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/league/seasons", http.StatusSeeOther)
}
