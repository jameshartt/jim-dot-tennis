package admin

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"

	"jim-dot-tennis/internal/models"
)

// PreferredNameApprovalData contains data for the preferred name approval page
type PreferredNameApprovalData struct {
	Title           string
	User            *models.User
	PendingRequests []PreferredNameRequestWithPlayer
	Stats           PreferredNameStats
}

// PreferredNameRequestWithPlayer contains a request with associated player data
type PreferredNameRequestWithPlayer struct {
	models.PreferredNameRequest
	Player models.Player
}

// PreferredNameStats contains statistics about preferred name requests
type PreferredNameStats struct {
	PendingCount  int
	ApprovedCount int
	RejectedCount int
}

// HandlePreferredNameApprovals shows the preferred name approval management page
func (s *Service) HandlePreferredNameApprovals(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	// Get pending requests with player data
	pendingRequests, err := s.playerRepository.FindPreferredNameRequestsByStatus(ctx, models.PreferredNamePending)
	if err != nil {
		log.Printf("Error fetching pending preferred name requests: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Enrich with player data
	var requestsWithPlayers []PreferredNameRequestWithPlayer
	for _, request := range pendingRequests {
		player, err := s.playerRepository.FindByID(ctx, request.PlayerID)
		if err != nil {
			log.Printf("Error fetching player %s: %v", request.PlayerID, err)
			continue
		}
		requestsWithPlayers = append(requestsWithPlayers, PreferredNameRequestWithPlayer{
			PreferredNameRequest: request,
			Player:               *player,
		})
	}

	// Get stats
	pendingCount, _ := s.playerRepository.CountPendingPreferredNameRequests(ctx)

	// Count approved and rejected (we'll implement these counts if needed)
	approvedRequests, _ := s.playerRepository.FindPreferredNameRequestsByStatus(ctx, models.PreferredNameApproved)
	rejectedRequests, _ := s.playerRepository.FindPreferredNameRequestsByStatus(ctx, models.PreferredNameRejected)

	// Get user from context
	user, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	data := PreferredNameApprovalData{
		Title:           "Preferred Name Approvals",
		User:            user,
		PendingRequests: requestsWithPlayers,
		Stats: PreferredNameStats{
			PendingCount:  pendingCount,
			ApprovedCount: len(approvedRequests),
			RejectedCount: len(rejectedRequests),
		},
	}

	// Execute template using admin's parseTemplate function
	tmpl, err := parseTemplate("templates", "admin/preferred_name_approvals.html")
	if err != nil {
		log.Printf("Error parsing preferred name approvals template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("Error rendering preferred name approvals template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// HandleApprovePreferredName handles approval of a preferred name request
func (s *Service) HandleApprovePreferredName(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := context.Background()
	user, err := getUserFromContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get request ID from URL path - parse it from the path manually
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 5 || parts[4] == "" {
		http.Error(w, "Invalid request ID", http.StatusBadRequest)
		return
	}

	requestID, err := strconv.ParseUint(parts[4], 10, 32)
	if err != nil {
		http.Error(w, "Invalid request ID", http.StatusBadRequest)
		return
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	adminNotes := strings.TrimSpace(r.FormValue("admin_notes"))
	var adminNotesPtr *string
	if adminNotes != "" {
		adminNotesPtr = &adminNotes
	}

	// Approve the request
	err = s.playerRepository.ApprovePreferredNameRequest(ctx, uint(requestID), user.Username, adminNotesPtr)
	if err != nil {
		log.Printf("Error approving preferred name request %d: %v", requestID, err)
		if strings.Contains(err.Error(), "no longer pending") {
			http.Error(w, "Request is no longer pending", http.StatusConflict)
			return
		}
		http.Error(w, "Error approving request", http.StatusInternalServerError)
		return
	}

	// Redirect back to approval page
	http.Redirect(w, r, "/admin/preferred-names", http.StatusSeeOther)
}

// HandleRejectPreferredName handles rejection of a preferred name request
func (s *Service) HandleRejectPreferredName(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := context.Background()
	user, err := getUserFromContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get request ID from URL path - parse it from the path manually
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 5 || parts[4] == "" {
		http.Error(w, "Invalid request ID", http.StatusBadRequest)
		return
	}

	requestID, err := strconv.ParseUint(parts[4], 10, 32)
	if err != nil {
		http.Error(w, "Invalid request ID", http.StatusBadRequest)
		return
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	adminNotes := strings.TrimSpace(r.FormValue("admin_notes"))
	var adminNotesPtr *string
	if adminNotes != "" {
		adminNotesPtr = &adminNotes
	}

	// Reject the request
	err = s.playerRepository.RejectPreferredNameRequest(ctx, uint(requestID), user.Username, adminNotesPtr)
	if err != nil {
		log.Printf("Error rejecting preferred name request %d: %v", requestID, err)
		http.Error(w, "Error rejecting request", http.StatusInternalServerError)
		return
	}

	// Redirect back to approval page
	http.Redirect(w, r, "/admin/preferred-names", http.StatusSeeOther)
}

// HandlePreferredNameHistory shows the history of all preferred name requests
func (s *Service) HandlePreferredNameHistory(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	// Get all approved and rejected requests
	approvedRequests, err := s.playerRepository.FindPreferredNameRequestsByStatus(ctx, models.PreferredNameApproved)
	if err != nil {
		log.Printf("Error fetching approved requests: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	rejectedRequests, err := s.playerRepository.FindPreferredNameRequestsByStatus(ctx, models.PreferredNameRejected)
	if err != nil {
		log.Printf("Error fetching rejected requests: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Combine and sort by processed_at descending
	allRequests := append(approvedRequests, rejectedRequests...)

	// Simple sort by processed_at (most recent first)
	for i := 0; i < len(allRequests)-1; i++ {
		for j := i + 1; j < len(allRequests); j++ {
			if allRequests[i].ProcessedAt == nil ||
				(allRequests[j].ProcessedAt != nil && allRequests[j].ProcessedAt.After(*allRequests[i].ProcessedAt)) {
				allRequests[i], allRequests[j] = allRequests[j], allRequests[i]
			}
		}
	}

	// Enrich with player data
	var requestsWithPlayers []PreferredNameRequestWithPlayer
	for _, request := range allRequests {
		player, err := s.playerRepository.FindByID(ctx, request.PlayerID)
		if err != nil {
			log.Printf("Error fetching player %s: %v", request.PlayerID, err)
			continue
		}
		requestsWithPlayers = append(requestsWithPlayers, PreferredNameRequestWithPlayer{
			PreferredNameRequest: request,
			Player:               *player,
		})
	}

	// Get user from context
	user, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	data := struct {
		Title             string
		User              *models.User
		ProcessedRequests []PreferredNameRequestWithPlayer
	}{
		Title:             "Preferred Name History",
		User:              user,
		ProcessedRequests: requestsWithPlayers,
	}

	// Execute template using admin's parseTemplate function
	tmpl, err := parseTemplate("templates", "admin/preferred_name_history.html")
	if err != nil {
		log.Printf("Error parsing preferred name history template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("Error rendering preferred name history template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
