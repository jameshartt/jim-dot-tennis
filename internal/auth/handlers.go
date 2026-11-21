// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"html/template"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"time"

	"jim-dot-tennis/internal/render"
)

// Handler provides HTTP handlers for auth-related routes
type Handler struct {
	service      *Service
	templateDir  string
	redirectPath string
	passkeys     *Passkeys
}

// SetPasskeys makes the login page lead with passkey sign-in wherever
// passkeys are available.
func (h *Handler) SetPasskeys(p *Passkeys) { h.passkeys = p }

// NewHandler creates a new auth handler
func NewHandler(service *Service, templateDir string, redirectPath string) *Handler {
	return &Handler{
		service:      service,
		templateDir:  templateDir,
		redirectPath: redirectPath,
	}
}

// loginData is what the login form renders.
type loginData struct {
	Error    string
	Username string
	// Redirect is the validated same-site path to return to after signing in
	// ("" when there is none); the form carries it as a hidden field.
	Redirect string
	// PasskeysAvailable: passkey sign-in can be offered at this web address.
	PasskeysAvailable bool
	// AdminPasskeysRequired: administrators must use a passkey here (shown
	// to everyone; never per username).
	AdminPasskeysRequired bool
}

// loginTemplates caches the parsed login page.
var loginTemplates = render.NewCache()

// renderLogin renders the login form, parsing its templates on first use.
func (h *Handler) renderLogin(w http.ResponseWriter, r *http.Request, data loginData) {
	data.PasskeysAvailable = h.passkeys != nil && h.passkeys.Available(r)
	data.AdminPasskeysRequired = data.PasskeysAvailable && h.service.AdminPasskeysRequired(r.Context())
	tmpl, err := loginTemplates.Get(h.templateDir, func() (*template.Template, error) {
		funcMap := template.FuncMap{
			"currentYear": func() int {
				return time.Now().Year()
			},
		}
		return template.New("").Funcs(funcMap).ParseFiles(
			filepath.Join(h.templateDir, "layout.html"),
			filepath.Join(h.templateDir, "login.html"),
		)
	})
	if err != nil {
		log.Printf("Error parsing template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if err := render.Execute(w, tmpl, "login.html", data); err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// LoginHandler handles the login page and form submission
func (h *Handler) LoginHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("LoginHandler called with method: %s, URL: %s", r.Method, r.URL.Path)

		// If already logged in, redirect
		cookie, err := r.Cookie(h.service.config.CookieName)
		if err == nil {
			log.Printf("Found existing cookie: %s", redactToken(cookie.Value))
			// Validate the session
			_, err := h.service.ValidateSession(cookie.Value, r)
			if err == nil {
				target := safeRedirectPath(r.URL.Query().Get("redirect"))
				if target == "" {
					target = h.redirectPath
				}
				log.Printf("Valid session found, redirecting to: %s", target)
				// Valid session, redirect to the target page
				http.Redirect(w, r, target, http.StatusSeeOther)
				return
			}
			log.Printf("Invalid session: %v, clearing cookie", err)
			// Invalid session, clear the cookie
			h.service.ClearSessionCookie(w)
		} else {
			log.Printf("No session cookie found: %v", err)
		}

		// GET request - show login form
		if r.Method == http.MethodGet {
			log.Printf("Processing GET request for login page")
			h.renderLogin(w, r, loginData{Redirect: safeRedirectPath(r.URL.Query().Get("redirect"))})
			return
		}

		// POST request - process login
		if r.Method == http.MethodPost {
			log.Printf("Processing POST request for login form")
			if err := r.ParseForm(); err != nil {
				log.Printf("Error parsing form: %v", err)
				http.Error(w, "Invalid form data", http.StatusBadRequest)
				return
			}

			username := r.Form.Get("username")
			password := r.Form.Get("password")
			log.Printf("Form submitted with username: %s", username)

			session, err := h.service.Login(username, password, r)
			if err != nil {
				log.Printf("Login failed: %v", err)

				data := loginData{
					Error:    err.Error(),
					Username: username,
					Redirect: safeRedirectPath(r.FormValue("redirect")),
				}

				h.renderLogin(w, r, data)
				return
			}

			log.Printf("Login successful for user: %s, setting session cookie and redirecting to: %s", username, h.redirectPath)
			// Set session cookie
			h.service.SetSessionCookie(w, session)

			// Ensure cookie is actually set before redirecting
			log.Printf("Session cookie set with value: %s, expires: %v", redactToken(session.ID), session.ExpiresAt)

			// The form posts the redirect as a hidden field; the URL query is
			// accepted too. Only same-site paths survive safeRedirectPath.
			redirectTo := safeRedirectPath(r.PostFormValue("redirect"))
			if redirectTo == "" {
				redirectTo = safeRedirectPath(r.URL.Query().Get("redirect"))
			}
			if redirectTo == "" {
				redirectTo = h.redirectPath
			}
			if redirectTo == "" {
				log.Printf("WARNING: redirectPath is empty, defaulting to /admin/league")
				redirectTo = "/admin/league"
			}
			// An account with no passkey is offered one first (never forced).
			redirectTo = h.passkeyPromptTarget(r, session.UserID, redirectTo)

			log.Printf("Login successful for user: %s, redirecting to: %s", username, redirectTo)

			// Explicitly set appropriate headers for redirect
			w.Header().Set("Location", redirectTo)
			w.WriteHeader(http.StatusSeeOther)
			return
		}

		// Other methods not allowed
		log.Printf("Method not allowed: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// safeRedirectPath returns target if it is a same-origin path ("/admin/...",
// optionally with a query), otherwise "". Absolute URLs, scheme-relative
// "//host" and "/\host" forms, and anything with control characters are
// rejected so a login link cannot send someone to another site.
func safeRedirectPath(target string) string {
	if target == "" || target[0] != '/' || len(target) > 2048 {
		return ""
	}
	if len(target) > 1 && (target[1] == '/' || target[1] == '\\') {
		return ""
	}
	for _, c := range target {
		if c < 0x20 || c == 0x7f || c == '\\' {
			return ""
		}
	}
	u, err := url.Parse(target)
	if err != nil || u.Scheme != "" || u.Host != "" || u.User != nil {
		return ""
	}
	return target
}

// LogoutHandler handles user logout
func (h *Handler) LogoutHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get session cookie
		cookie, err := r.Cookie(h.service.config.CookieName)
		if err == nil {
			// Invalidate the session
			h.service.InvalidateSession(cookie.Value)
		}

		// Clear the cookie
		h.service.ClearSessionCookie(w)

		// Redirect to login page
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}

// RegisterRoutes registers auth routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/login", h.LoginHandler())
	mux.HandleFunc("/logout", h.LogoutHandler())
}
