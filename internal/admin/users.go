// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"jim-dot-tennis/internal/accounts"
	"jim-dot-tennis/internal/models"
)

// UsersHandler handles user-related requests
type UsersHandler struct {
	service     *Service
	templateDir string
}

// NewUsersHandler creates a new users handler
func NewUsersHandler(service *Service, templateDir string) *UsersHandler {
	return &UsersHandler{
		service:     service,
		templateDir: templateDir,
	}
}

// HandleUsers handles user management routes:
//
//	GET  /admin/league/users                       the page
//	POST /admin/league/users                       create an account and its invite link
//	POST /admin/league/users/{id}/toggle-active    enable or disable
//	POST /admin/league/users/{id}/role             change role (admin or captain)
//	POST /admin/league/users/{id}/player           link to a player record (empty: unlink)
//	POST /admin/league/users/{id}/invite           new invite link (users_links.go)
//	POST /admin/league/users/{id}/recover          recovery link (users_links.go)
//	POST /admin/league/users/links/{id}/revoke     cancel an unused link (users_links.go)
//
// Administrators never see or set another user's password: accounts are
// handed over, and recovered, with single-use links.
func (h *UsersHandler) HandleUsers(w http.ResponseWriter, r *http.Request) {
	log.Printf("Admin users handler called with path: %s, method: %s", r.URL.Path, r.Method)

	user, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	path := r.URL.Path

	// Route to sub-actions: /admin/league/users/{id}/{action}
	if strings.HasPrefix(path, "/admin/league/users/") && path != "/admin/league/users/" {
		h.handleUserAction(w, r, user)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.renderUsers(w, r, user, nil)
	case http.MethodPost:
		h.handleUsersPost(w, r, user)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// usersPageUser is the signed-in administrator as the users template sees
// them: no password hash.
type usersPageUser struct {
	ID       int64
	Username string
}

// playerOption is a player an account can be linked to.
type playerOption struct {
	ID   string
	Name string
}

// renderUsers renders the user management page. issued, when not nil, is a
// link that was just made: it is shown on this response only and never put
// in a redirect, so the token stays out of URLs, history and logs.
func (h *UsersHandler) renderUsers(w http.ResponseWriter, r *http.Request, user *models.User, issued *issuedLinkView) {
	users, err := h.service.accounts.List(r.Context())
	if err != nil {
		logAndError(w, "Failed to load users", err, http.StatusInternalServerError)
		return
	}
	pending, err := h.service.accounts.PendingLinks(r.Context())
	if err != nil {
		logAndError(w, "Failed to load links", err, http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"User":         usersPageUser{ID: user.ID, Username: user.Username},
		"Users":        users,
		"PendingLinks": pending,
		"Players":      h.linkablePlayers(r, users),
		"PlayerNames":  h.linkedPlayerNames(r, users),
		"Roles":        assignableRoles,
		"IssuedLink":   issued,
		"SuccessMsg":   r.URL.Query().Get("success"),
		"ErrorMsg":     r.URL.Query().Get("error"),
	}

	tmpl, err := parseTemplate(h.templateDir, "admin/users.html")
	if err != nil {
		logAndError(w, "Failed to parse template", err, http.StatusInternalServerError)
		return
	}

	// The page can carry a link that works once: never cache it.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if err := renderTemplate(w, tmpl, data); err != nil {
		logAndError(w, "Failed to render template", err, http.StatusInternalServerError)
	}
}

// linkablePlayers lists the home club's active players that no account is
// linked to yet. A failure only empties the list (linking is optional).
func (h *UsersHandler) linkablePlayers(r *http.Request, users []accounts.Account) []playerOption {
	players, err := h.service.playerRepository.FindAll(r.Context())
	if err != nil {
		log.Printf("Users page: could not list players: %v", err)
		return nil
	}
	linked := map[string]bool{}
	for _, u := range users {
		if u.PlayerID != nil {
			linked[*u.PlayerID] = true
		}
	}
	homeClub := h.service.homeClubID()
	var out []playerOption
	for _, p := range players {
		if linked[p.ID] || (homeClub != 0 && p.ClubID != homeClub) {
			continue
		}
		out = append(out, playerOption{ID: p.ID, Name: p.FirstName + " " + p.LastName})
	}
	return out
}

// assignableRoles are the roles the users page offers (the player role is
// retired: players use link tokens, not accounts).
var assignableRoles = []string{string(models.RoleCaptain), string(models.RoleAdmin)}

// linkedPlayerNames maps the player id of every linked account to the
// player's name, for the per-account "Linked player" control.
func (h *UsersHandler) linkedPlayerNames(r *http.Request, users []accounts.Account) map[string]string {
	names := map[string]string{}
	for _, u := range users {
		if u.PlayerID == nil || *u.PlayerID == "" {
			continue
		}
		p, err := h.service.playerRepository.FindByID(r.Context(), *u.PlayerID)
		if err != nil || p == nil {
			names[*u.PlayerID] = *u.PlayerID
			continue
		}
		names[*u.PlayerID] = p.FirstName + " " + p.LastName
	}
	return names
}

// handleUsersPost creates a new account with no password and shows its
// invite link once.
func (h *UsersHandler) handleUsersPost(w http.ResponseWriter, r *http.Request, user *models.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	if username == "" {
		http.Redirect(w, r, "/admin/league/users?error=Username+is+required", http.StatusSeeOther)
		return
	}

	role, err := accounts.ParseRole(r.FormValue("role"))
	if err != nil {
		http.Redirect(w, r, "/admin/league/users?error=Invalid+role", http.StatusSeeOther)
		return
	}

	var playerID *string
	if id := strings.TrimSpace(r.FormValue("player_id")); id != "" {
		playerID = &id
	}

	_, issued, err := h.service.accounts.Invite(r.Context(), actorFor(user), username, role, playerID)
	if err != nil {
		log.Printf("Failed to create user: %v", err)
		redirectUsersError(w, r, err, "Failed to create user")
		return
	}
	h.renderUsers(w, r, user, h.issuedLinkView(r, issued, true))
}

// handleUserAction routes POST actions for a specific user
func (h *UsersHandler) handleUserAction(w http.ResponseWriter, r *http.Request, currentUser *models.User) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse path: /admin/league/users/{id}/{action}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/admin/league/users/"), "/")
	if len(parts) < 2 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	// /admin/league/users/links/{id}/revoke
	if parts[0] == "links" {
		h.handleLinkAction(w, r, currentUser, parts[1:])
		return
	}

	targetID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	action := parts[1]

	switch action {
	case "toggle-active":
		// Prevent self-deactivation
		if targetID == currentUser.ID {
			http.Redirect(w, r, "/admin/league/users?error=Cannot+deactivate+your+own+account", http.StatusSeeOther)
			return
		}
		target, err := h.service.accounts.Get(r.Context(), targetID)
		if err != nil {
			redirectUsersError(w, r, err, "Failed to update user")
			return
		}
		if _, err := h.service.accounts.SetActive(r.Context(), actorFor(currentUser), targetID, !target.IsActive); err != nil {
			log.Printf("Failed to toggle user active: %v", err)
			redirectUsersError(w, r, err, "Failed to update user")
			return
		}
		http.Redirect(w, r, "/admin/league/users?success=User+status+updated", http.StatusSeeOther)

	case "role":
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}
		role, err := accounts.ParseRole(r.FormValue("role"))
		if err != nil {
			http.Redirect(w, r, "/admin/league/users?error=Invalid+role", http.StatusSeeOther)
			return
		}
		if _, err := h.service.accounts.SetRole(r.Context(), actorFor(currentUser), targetID, role); err != nil {
			log.Printf("Failed to update user role: %v", err)
			redirectUsersError(w, r, err, "Failed to update role")
			return
		}
		http.Redirect(w, r, "/admin/league/users?success=Role+updated+successfully", http.StatusSeeOther)

	case "player":
		// Link the account to a player record, change it, or remove it
		// (empty player_id). "My teams" on the planning dashboard uses it.
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}
		if _, err := h.service.accounts.SetPlayerLink(r.Context(), actorFor(currentUser), targetID, r.FormValue("player_id")); err != nil {
			log.Printf("Failed to update the linked player: %v", err)
			redirectUsersError(w, r, err, "Failed to update the linked player")
			return
		}
		http.Redirect(w, r, "/admin/league/users?success=Linked+player+updated", http.StatusSeeOther)

	case "invite":
		h.handleIssueLink(w, r, currentUser, targetID, accounts.LinkInvite)

	case "recover":
		h.handleIssueLink(w, r, currentUser, targetID, accounts.LinkRecovery)

	default:
		http.Error(w, "Unknown action", http.StatusBadRequest)
	}
}

// actorFor is the audit actor for an administrator using the web interface.
func actorFor(u *models.User) accounts.Actor {
	return accounts.UserActor(u.ID, u.Username)
}

// redirectUsersError sends the browser back to the users page with a message
// for err: the account rule that refused the change, or fallback.
func redirectUsersError(w http.ResponseWriter, r *http.Request, err error, fallback string) {
	msg := fallback
	var invalidName *accounts.InvalidUsernameError
	switch {
	case errors.Is(err, accounts.ErrLastAdmin), errors.Is(err, accounts.ErrUsernameTaken),
		errors.Is(err, accounts.ErrNotFound), errors.Is(err, accounts.ErrInvalidRole),
		errors.Is(err, accounts.ErrPlayerNotFound), errors.Is(err, accounts.ErrPlayerLinked),
		errors.Is(err, accounts.ErrInviteHasSignIn), errors.Is(err, accounts.ErrLinkNotPending):
		msg = upperFirst(err.Error())
	case errors.As(err, &invalidName):
		msg = upperFirst(err.Error())
	}
	http.Redirect(w, r, "/admin/league/users?error="+url.QueryEscape(msg), http.StatusSeeOther)
}

func upperFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
