package auth

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"time"
)

// Handler provides HTTP handlers for auth-related routes
type Handler struct {
	service      *Service
	templateDir  string
	redirectPath string
}

// NewHandler creates a new auth handler
func NewHandler(service *Service, templateDir string, redirectPath string) *Handler {
	return &Handler{
		service:      service,
		templateDir:  templateDir,
		redirectPath: redirectPath,
	}
}

// LoginHandler handles the login page and form submission
func (h *Handler) LoginHandler() http.HandlerFunc {
	type loginData struct {
		Error    string
		Username string
	}

	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("LoginHandler called with method: %s, URL: %s", r.Method, r.URL.Path)

		// If already logged in, redirect
		cookie, err := r.Cookie(h.service.config.CookieName)
		if err == nil {
			log.Printf("Found existing cookie: %s", cookie.Value)
			// Validate the session
			_, err := h.service.ValidateSession(cookie.Value, r)
			if err == nil {
				log.Printf("Valid session found, redirecting to: %s", h.redirectPath)
				// Valid session, redirect to the target page
				http.Redirect(w, r, h.redirectPath, http.StatusSeeOther)
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
			// Create template with functions
			funcMap := template.FuncMap{
				"currentYear": func() int {
					return time.Now().Year()
				},
			}

			tmpl, err := template.New("").Funcs(funcMap).ParseFiles(
				filepath.Join(h.templateDir, "layout.html"),
				filepath.Join(h.templateDir, "login.html"),
			)
			if err != nil {
				log.Printf("Error parsing template: %v", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			data := loginData{}
			if err := tmpl.ExecuteTemplate(w, "login.html", data); err != nil {
				log.Printf("Error executing template: %v", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
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
				}

				// Create template with functions
				funcMap := template.FuncMap{
					"currentYear": func() int {
						return time.Now().Year()
					},
				}

				tmpl, err := template.New("").Funcs(funcMap).ParseFiles(
					filepath.Join(h.templateDir, "layout.html"),
					filepath.Join(h.templateDir, "login.html"),
				)
				if err != nil {
					log.Printf("Error parsing template: %v", err)
					http.Error(w, "Internal server error", http.StatusInternalServerError)
					return
				}

				if err := tmpl.ExecuteTemplate(w, "login.html", data); err != nil {
					log.Printf("Error executing template: %v", err)
					http.Error(w, "Internal server error", http.StatusInternalServerError)
				}
				return
			}

			log.Printf("Login successful for user: %s, setting session cookie and redirecting to: %s", username, h.redirectPath)
			// Set session cookie
			h.service.SetSessionCookie(w, session)

			// Ensure cookie is actually set before redirecting
			log.Printf("Session cookie set with value: %s, expires: %v", session.ID, session.ExpiresAt)

			// Check for redirect parameter from URL
			redirectTo := r.URL.Query().Get("redirect")
			if redirectTo == "" {
				redirectTo = h.redirectPath
			}
			if redirectTo == "" {
				log.Printf("WARNING: redirectPath is empty, defaulting to /admin/league")
				redirectTo = "/admin/league"
			}

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
