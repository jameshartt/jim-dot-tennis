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

// ClubTeamInfo represents a team with its related info for the club detail view
type ClubTeamInfo struct {
	models.Team
	DivisionName string
	SeasonName   string
	PlayerCount  int
	IsStAnns     bool
}

// ClubTeamsBySeason groups teams by season for display
type ClubTeamsBySeason struct {
	Season *models.Season
	Teams  []ClubTeamInfo
}

// ClubDependencies holds information about a club's dependent records
type ClubDependencies struct {
	TeamCount    int
	FixtureCount int
	Teams        []models.Team
}

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
	case http.MethodPost:
		h.handleClubCreate(w, r)
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

	// Check for success message
	successMsg := ""
	switch r.URL.Query().Get("success") {
	case "deleted":
		successMsg = "Club deleted successfully."
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
		"User":    user,
		"Clubs":   clubItems,
		"Success": successMsg,
	}); err != nil {
		logAndError(w, err.Error(), err, http.StatusInternalServerError)
	}
}

// handleClubDetail handles requests for individual club details
func (h *ClubsHandler) handleClubDetail(w http.ResponseWriter, r *http.Request) {
	log.Printf("Admin club detail handler called with path: %s, method: %s", r.URL.Path, r.Method)

	// Check for delete action
	if strings.HasSuffix(r.URL.Path, "/delete") {
		h.handleClubDelete(w, r)
		return
	}

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
	switch r.URL.Query().Get("success") {
	case "updated":
		successMsg = "Club updated successfully."
	case "team_created":
		successMsg = "Team created successfully."
	case "created":
		successMsg = "Club created successfully."
	}

	// Get club dependencies for delete confirmation
	deps, err := h.service.GetClubDependencies(clubID)
	if err != nil {
		log.Printf("Failed to get club dependencies: %v", err)
		deps = &ClubDependencies{}
	}

	// Fetch teams for this club grouped by season
	clubTeams, err := h.service.GetClubTeams(clubID)
	if err != nil {
		log.Printf("Failed to load club teams: %v", err)
	}

	// Fetch active season and divisions for the "Add Team" form
	activeSeason, _ := h.service.GetActiveSeason()
	var divisions []models.Division
	if activeSeason != nil {
		divisions, _ = h.service.GetDivisionsBySeason(activeSeason.ID)
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
		"Dependencies":         deps,
		"ClubTeams":            clubTeams,
		"ActiveSeason":         activeSeason,
		"Divisions":            divisions,
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

// handleClubCreate handles POST requests to create a new club
func (h *ClubsHandler) handleClubCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		logAndError(w, "Failed to parse form data", err, http.StatusBadRequest)
		return
	}

	action := r.FormValue("action")
	if action != "create" {
		http.Error(w, "Unknown action", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "Club name is required", http.StatusBadRequest)
		return
	}

	club := &models.Club{
		Name: name,
	}

	if err := h.service.CreateClub(club); err != nil {
		logAndError(w, "Failed to create club", err, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/admin/league/clubs/%d?success=created", club.ID), http.StatusSeeOther)
}

// handleClubDelete handles POST requests to delete a club
func (h *ClubsHandler) handleClubDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract club ID from URL path (path is like /admin/league/clubs/123/delete)
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/admin/league/clubs/"), "/")
	if len(pathParts) < 2 || pathParts[0] == "" || pathParts[1] != "delete" {
		http.Error(w, "Invalid delete URL", http.StatusBadRequest)
		return
	}

	clubID, err := strconv.ParseUint(pathParts[0], 10, 32)
	if err != nil {
		logAndError(w, "Invalid club ID", err, http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		logAndError(w, "Failed to parse form data", err, http.StatusBadRequest)
		return
	}

	// Check for dependencies
	deps, err := h.service.GetClubDependencies(uint(clubID))
	if err != nil {
		logAndError(w, "Failed to check club dependencies", err, http.StatusInternalServerError)
		return
	}

	// If there are fixtures and no force param, block the delete
	force := r.FormValue("force") == "true"
	if deps.FixtureCount > 0 && !force {
		http.Redirect(w, r, fmt.Sprintf("/admin/league/clubs/%d?error=Club+has+%d+fixtures+across+%d+teams.+Use+force+delete+to+remove.", clubID, deps.FixtureCount, deps.TeamCount), http.StatusSeeOther)
		return
	}

	if err := h.service.DeleteClub(uint(clubID)); err != nil {
		logAndError(w, "Failed to delete club", err, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/league/clubs?success=deleted", http.StatusSeeOther)
}

// CreateClub creates a new club record
func (s *Service) CreateClub(club *models.Club) error {
	ctx := context.Background()
	return s.clubRepository.Create(ctx, club)
}

// GetClubDependencies checks for teams and fixtures associated with a club
func (s *Service) GetClubDependencies(clubID uint) (*ClubDependencies, error) {
	ctx := context.Background()
	deps := &ClubDependencies{}

	teams, err := s.teamRepository.FindByClub(ctx, clubID)
	if err != nil {
		return nil, err
	}
	deps.Teams = teams
	deps.TeamCount = len(teams)

	for _, team := range teams {
		fixtures, err := s.fixtureRepository.FindByTeam(ctx, team.ID)
		if err == nil {
			deps.FixtureCount += len(fixtures)
		}
	}

	return deps, nil
}

// DeleteClub deletes a club and cascades to its teams
func (s *Service) DeleteClub(clubID uint) error {
	ctx := context.Background()
	teams, err := s.teamRepository.FindByClub(ctx, clubID)
	if err != nil {
		return err
	}
	for _, team := range teams {
		if err := s.teamRepository.Delete(ctx, team.ID); err != nil {
			return err
		}
	}
	return s.clubRepository.Delete(ctx, clubID)
}

// GetClubTeams retrieves all teams for a club, grouped by season with active season first
func (s *Service) GetClubTeams(clubID uint) ([]ClubTeamsBySeason, error) {
	ctx := context.Background()

	teams, err := s.teamRepository.FindByClub(ctx, clubID)
	if err != nil {
		return nil, err
	}

	// Get St Ann's club ID for comparison
	stAnnsClubs, _ := s.clubRepository.FindByNameLike(ctx, "St Ann")
	stAnnsID := uint(0)
	if len(stAnnsClubs) > 0 {
		stAnnsID = stAnnsClubs[0].ID
	}

	activeSeason, _ := s.seasonRepository.FindActive(ctx)

	seasonMap := make(map[uint]*ClubTeamsBySeason)
	seasonOrder := []uint{}

	for _, team := range teams {
		if _, exists := seasonMap[team.SeasonID]; !exists {
			season, _ := s.seasonRepository.FindByID(ctx, team.SeasonID)
			seasonMap[team.SeasonID] = &ClubTeamsBySeason{Season: season}
			seasonOrder = append(seasonOrder, team.SeasonID)
		}

		info := ClubTeamInfo{
			Team:     team,
			IsStAnns: team.ClubID == stAnnsID,
		}

		if div, err := s.divisionRepository.FindByID(ctx, team.DivisionID); err == nil {
			info.DivisionName = div.Name
		}
		if season, err := s.seasonRepository.FindByID(ctx, team.SeasonID); err == nil {
			info.SeasonName = season.Name
		}
		if info.IsStAnns {
			if count, err := s.teamRepository.CountPlayers(ctx, team.ID, team.SeasonID); err == nil {
				info.PlayerCount = count
			}
		}

		seasonMap[team.SeasonID].Teams = append(seasonMap[team.SeasonID].Teams, info)
	}

	var result []ClubTeamsBySeason
	if activeSeason != nil {
		if st, ok := seasonMap[activeSeason.ID]; ok {
			result = append(result, *st)
		}
	}
	for _, sid := range seasonOrder {
		if activeSeason != nil && sid == activeSeason.ID {
			continue
		}
		result = append(result, *seasonMap[sid])
	}

	return result, nil
}
