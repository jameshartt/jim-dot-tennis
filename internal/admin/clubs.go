package admin

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"jim-dot-tennis/internal/models"
)

// ClubsHandler handles club-related requests
type ClubsHandler struct {
	service     *Service
	templateDir string
}

// NewClubsHandler creates a new clubs handler
func NewClubsHandler(service *Service, templateDir string) *ClubsHandler {
	return &ClubsHandler{
		service:     service,
		templateDir: templateDir,
	}
}

// HandleClubs handles club management routes
func (h *ClubsHandler) HandleClubs(w http.ResponseWriter, r *http.Request) {
	log.Printf("Admin clubs handler called with path: %s, method: %s", r.URL.Path, r.Method)

	// Check if this is a specific club detail request
	if strings.Contains(r.URL.Path, "/clubs/") && r.URL.Path != "/admin/league/clubs/" {
		h.handleClubDetail(w, r)
		return
	}

	// List all clubs
	switch r.Method {
	case http.MethodGet:
		h.handleClubsList(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// ClubListItem represents a club in the list view with data completeness info
type ClubListItem struct {
	models.Club
	DataCompleteness     int    // percentage 0-100
	DataCompletenessText string // "Low", "Medium", "High"
}

// calculateDataCompleteness returns the percentage of venue fields that are filled
func calculateDataCompleteness(club models.Club) (int, string) {
	total := 10
	filled := 0

	if club.Postcode != nil && *club.Postcode != "" {
		filled++
	}
	if club.AddressLine1 != nil && *club.AddressLine1 != "" {
		filled++
	}
	if club.City != nil && *club.City != "" {
		filled++
	}
	if club.CourtSurface != nil && *club.CourtSurface != "" {
		filled++
	}
	if club.CourtCount != nil && *club.CourtCount > 0 {
		filled++
	}
	if club.Latitude != nil {
		filled++
	}
	if club.Longitude != nil {
		filled++
	}
	if club.ParkingInfo != nil && *club.ParkingInfo != "" {
		filled++
	}
	if club.TransportInfo != nil && *club.TransportInfo != "" {
		filled++
	}
	if club.GoogleMapsURL != nil && *club.GoogleMapsURL != "" {
		filled++
	}

	pct := (filled * 100) / total
	text := "Low"
	if pct >= 70 {
		text = "High"
	} else if pct >= 40 {
		text = "Medium"
	}
	return pct, text
}

// handleClubsList handles GET requests for the clubs list page
func (h *ClubsHandler) handleClubsList(w http.ResponseWriter, r *http.Request) {
	// Get user from context
	user, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	// Get all clubs
	clubs, err := h.service.GetAllClubs()
	if err != nil {
		logAndError(w, "Failed to load clubs", err, http.StatusInternalServerError)
		return
	}

	// Build list items with completeness data
	var clubItems []ClubListItem
	for _, club := range clubs {
		pct, text := calculateDataCompleteness(club)
		clubItems = append(clubItems, ClubListItem{
			Club:                 club,
			DataCompleteness:     pct,
			DataCompletenessText: text,
		})
	}

	// Load the clubs template
	tmpl, err := parseTemplate(h.templateDir, "admin/clubs.html")
	if err != nil {
		log.Printf("Error parsing clubs template: %v", err)
		renderFallbackHTML(w, "Admin - Clubs", "Club Management",
			"Club management page - coming soon", "/admin/league")
		return
	}

	// Execute the template with data
	if err := renderTemplate(w, tmpl, map[string]interface{}{
		"User":  user,
		"Clubs": clubItems,
	}); err != nil {
		logAndError(w, err.Error(), err, http.StatusInternalServerError)
	}
}

// handleClubDetail handles requests for individual club details
func (h *ClubsHandler) handleClubDetail(w http.ResponseWriter, r *http.Request) {
	log.Printf("Admin club detail handler called with path: %s, method: %s", r.URL.Path, r.Method)

	// Get user from context
	user, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	// Extract club ID from URL path
	clubID, err := parseIDFromPath(r.URL.Path, "/admin/league/clubs/")
	if err != nil {
		logAndError(w, "Invalid club ID", err, http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleClubDetailGet(w, r, user, clubID)
	case http.MethodPost:
		h.handleClubUpdate(w, r, user, clubID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleClubDetailGet handles GET requests for the club detail/edit page
func (h *ClubsHandler) handleClubDetailGet(w http.ResponseWriter, r *http.Request, user *models.User, clubID uint) {
	club, err := h.service.GetClubDetail(clubID)
	if err != nil {
		logAndError(w, "Club not found", err, http.StatusNotFound)
		return
	}

	pct, pctText := calculateDataCompleteness(*club)

	// Check for success message
	successMsg := ""
	if r.URL.Query().Get("success") == "updated" {
		successMsg = "Club updated successfully."
	}

	// Load the club detail template
	tmpl, err := parseTemplate(h.templateDir, "admin/club_detail.html")
	if err != nil {
		log.Printf("Error parsing club detail template: %v", err)
		renderFallbackHTML(w, "Club Detail", "Club Detail",
			"Club detail page - coming soon", "/admin/league/clubs")
		return
	}

	// Execute the template with data
	if err := renderTemplate(w, tmpl, map[string]interface{}{
		"User":                 user,
		"Club":                 club,
		"DataCompleteness":     pct,
		"DataCompletenessText": pctText,
		"Success":              successMsg,
	}); err != nil {
		logAndError(w, err.Error(), err, http.StatusInternalServerError)
	}
}

// handleClubUpdate handles POST requests to update a club
func (h *ClubsHandler) handleClubUpdate(w http.ResponseWriter, r *http.Request, user *models.User, clubID uint) {
	// Parse form data
	if err := r.ParseForm(); err != nil {
		logAndError(w, "Failed to parse form data", err, http.StatusBadRequest)
		return
	}

	// Get the existing club
	club, err := h.service.GetClubDetail(clubID)
	if err != nil {
		logAndError(w, "Club not found", err, http.StatusNotFound)
		return
	}

	// Update fields from form
	club.Name = strings.TrimSpace(r.FormValue("name"))
	club.Address = strings.TrimSpace(r.FormValue("address"))
	club.Website = strings.TrimSpace(r.FormValue("website"))
	club.PhoneNumber = strings.TrimSpace(r.FormValue("phone_number"))

	// Handle optional string pointer fields
	club.Postcode = stringPtrFromForm(r, "postcode")
	club.AddressLine1 = stringPtrFromForm(r, "address_line_1")
	club.AddressLine2 = stringPtrFromForm(r, "address_line_2")
	club.City = stringPtrFromForm(r, "city")
	club.CourtSurface = stringPtrFromForm(r, "court_surface")
	club.ParkingInfo = stringPtrFromForm(r, "parking_info")
	club.TransportInfo = stringPtrFromForm(r, "transport_info")
	club.Tips = stringPtrFromForm(r, "tips")
	club.GoogleMapsURL = stringPtrFromForm(r, "google_maps_url")

	// Handle latitude
	if latStr := strings.TrimSpace(r.FormValue("latitude")); latStr != "" {
		if lat, err := strconv.ParseFloat(latStr, 64); err == nil {
			club.Latitude = &lat
		}
	} else {
		club.Latitude = nil
	}

	// Handle longitude
	if lonStr := strings.TrimSpace(r.FormValue("longitude")); lonStr != "" {
		if lon, err := strconv.ParseFloat(lonStr, 64); err == nil {
			club.Longitude = &lon
		}
	} else {
		club.Longitude = nil
	}

	// Handle court count
	if countStr := strings.TrimSpace(r.FormValue("court_count")); countStr != "" {
		if count, err := strconv.Atoi(countStr); err == nil {
			club.CourtCount = &count
		}
	} else {
		club.CourtCount = nil
	}

	// Save the club
	if err := h.service.UpdateClub(club); err != nil {
		logAndError(w, "Failed to update club", err, http.StatusInternalServerError)
		return
	}

	// Redirect back to club detail with success message
	http.Redirect(w, r, fmt.Sprintf("/admin/league/clubs/%d?success=updated", clubID), http.StatusSeeOther)
}

// stringPtrFromForm gets a string pointer from a form value. Returns nil for empty strings.
func stringPtrFromForm(r *http.Request, key string) *string {
	val := strings.TrimSpace(r.FormValue(key))
	if val == "" {
		return nil
	}
	return &val
}

// Service methods for clubs

// GetAllClubs retrieves all clubs
func (s *Service) GetAllClubs() ([]models.Club, error) {
	ctx := context.Background()
	return s.clubRepository.FindAll(ctx)
}

// GetClubDetail retrieves a single club by ID
func (s *Service) GetClubDetail(id uint) (*models.Club, error) {
	ctx := context.Background()
	return s.clubRepository.FindByID(ctx, id)
}

// UpdateClub updates a club record
func (s *Service) UpdateClub(club *models.Club) error {
	ctx := context.Background()
	return s.clubRepository.Update(ctx, club)
}
