// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"jim-dot-tennis/internal/accounts"
	"jim-dot-tennis/internal/config"
	"jim-dot-tennis/internal/models"
)

// Invite and recovery links on the users page. The rules live in
// internal/accounts/links.go; this file only issues, shows and revokes.

// issuedLinkView is a link that has just been made, as the users page shows
// it (once).
type issuedLinkView struct {
	URL        string // the full address to hand over
	Username   string
	IsInvite   bool
	NewAccount bool // the account was created by this request
	ExpiresAt  time.Time
	Lifetime   string // "48 hours", "1 hour"
	Superseded int64  // earlier unused links this one cancelled
	// FromRequest: no public URL is configured, so the address was built
	// from this request's host (development only).
	FromRequest bool
}

// linkBaseURL is the address links are built from: the configured public
// URL. Only when none is set (development; production refuses to start
// without one) does it fall back to the address this request was made to.
func (s *Service) linkBaseURL(r *http.Request) (base string, fromRequest bool) {
	if s.clubSettings != nil {
		if p, ok, _ := s.clubSettings.ResolvePublicURL(); ok {
			return p.Base, false
		}
	}
	return config.RequestBaseURL(r), true
}

func (h *UsersHandler) issuedLinkView(r *http.Request, issued accounts.IssuedLink, newAccount bool) *issuedLinkView {
	base, fromRequest := h.service.linkBaseURL(r)
	lifetime := "1 hour"
	if issued.Kind == accounts.LinkInvite {
		lifetime = "48 hours"
	}
	return &issuedLinkView{
		URL:         base + issued.Path(),
		Username:    issued.Username,
		IsInvite:    issued.Kind == accounts.LinkInvite,
		NewAccount:  newAccount,
		ExpiresAt:   issued.ExpiresAt,
		Lifetime:    lifetime,
		Superseded:  issued.Superseded,
		FromRequest: fromRequest,
	}
}

// handleIssueLink makes an invite or recovery link for an existing account
// and shows it once. The account is not changed: it keeps its sign-in
// methods and sessions until the link is used.
func (h *UsersHandler) handleIssueLink(w http.ResponseWriter, r *http.Request, currentUser *models.User, targetID int64, kind accounts.LinkKind) {
	issued, err := h.service.accounts.IssueLink(r.Context(), actorFor(currentUser), targetID, kind)
	if err != nil {
		log.Printf("Failed to issue %s link for user %d: %v", kind, targetID, err)
		redirectUsersError(w, r, err, "Failed to create the link")
		return
	}
	h.renderUsers(w, r, currentUser, h.issuedLinkView(r, issued, false))
}

// handleLinkAction serves /admin/league/users/links/{id}/revoke.
func (h *UsersHandler) handleLinkAction(w http.ResponseWriter, r *http.Request, currentUser *models.User, parts []string) {
	if len(parts) != 2 || parts[1] != "revoke" {
		http.Error(w, "Unknown action", http.StatusBadRequest)
		return
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		http.Error(w, "Invalid link ID", http.StatusBadRequest)
		return
	}
	if _, err := h.service.accounts.RevokeLink(r.Context(), actorFor(currentUser), id); err != nil {
		log.Printf("Failed to revoke link %d: %v", id, err)
		redirectUsersError(w, r, err, "Failed to cancel the link")
		return
	}
	http.Redirect(w, r, "/admin/league/users?success=Link+cancelled", http.StatusSeeOther)
}
