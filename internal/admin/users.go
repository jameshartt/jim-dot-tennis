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
	"jim-dot-tennis/internal/auth"
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

// HandleUsers handles user management routes
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
		h.handleUsersGet(w, r, user)
	case http.MethodPost:
		h.handleUsersPost(w, r, user)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleUsersGet renders the user management page
func (h *UsersHandler) handleUsersGet(w http.ResponseWriter, r *http.Request, user *models.User) {
	users, err := h.service.accounts.List(r.Context())
	if err != nil {
		logAndError(w, "Failed to load users", err, http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"User":       user,
		"Users":      users,
		"Roles":      []string{string(models.RoleAdmin), string(models.RoleCaptain), string(models.RolePlayer)},
		"SuccessMsg": r.URL.Query().Get("success"),
		"ErrorMsg":   r.URL.Query().Get("error"),
	}

	tmpl, err := parseTemplate(h.templateDir, "admin/users.html")
	if err != nil {
		logAndError(w, "Failed to parse template", err, http.StatusInternalServerError)
		return
	}

	if err := renderTemplate(w, tmpl, data); err != nil {
		logAndError(w, "Failed to render template", err, http.StatusInternalServerError)
	}
}

// handleUsersPost creates a new user
func (h *UsersHandler) handleUsersPost(w http.ResponseWriter, r *http.Request, user *models.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	if username == "" || password == "" {
		http.Redirect(w, r, "/admin/league/users?error=Username+and+password+are+required", http.StatusSeeOther)
		return
	}

	role, err := accounts.ParseRole(r.FormValue("role"))
	if err != nil {
		http.Redirect(w, r, "/admin/league/users?error=Invalid+role", http.StatusSeeOther)
		return
	}

	// The administrator chose this password, so the new user is asked to
	// replace it at their first sign-in.
	if _, err := h.service.accounts.Create(r.Context(), actorFor(user), username, role, password, true); err != nil {
		log.Printf("Failed to create user: %v", err)
		redirectUsersError(w, r, err, "Failed to create user")
		return
	}

	http.Redirect(w, r, "/admin/league/users?success=User+created+successfully", http.StatusSeeOther)
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

	case "reset-password":
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}
		newPassword := r.FormValue("password")
		if newPassword == "" {
			http.Redirect(w, r, "/admin/league/users?error=Password+cannot+be+empty", http.StatusSeeOther)
			return
		}
		// Ends all of the user's sessions; unless it is the administrator's own
		// account the user must choose a new password at their next sign-in.
		if err := h.service.accounts.SetPassword(r.Context(), actorFor(currentUser), targetID, newPassword); err != nil {
			log.Printf("Failed to reset password: %v", err)
			redirectUsersError(w, r, err, "Failed to reset password")
			return
		}
		http.Redirect(w, r, "/admin/league/users?success=Password+reset+successfully", http.StatusSeeOther)

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
		errors.Is(err, accounts.ErrNotFound), errors.Is(err, accounts.ErrInvalidRole):
		msg = upperFirst(err.Error())
	case errors.As(err, &invalidName):
		msg = upperFirst(err.Error())
	case errors.Is(err, auth.ErrPasswordPolicy):
		msg = "Password: " + strings.TrimPrefix(err.Error(), auth.ErrPasswordPolicy.Error()+": ")
	}
	http.Redirect(w, r, "/admin/league/users?error="+url.QueryEscape(msg), http.StatusSeeOther)
}

func upperFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
