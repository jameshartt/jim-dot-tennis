// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package setup

import (
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/bhplta"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/render"
)

// Paths that stay reachable while the instance needs setup.
const (
	// Path is the setup entry page (step 1, the setup code). Later steps live
	// under Path + "/".
	Path = "/setup"
	// AdminPath is step 2: create the first administrator.
	AdminPath = Path + "/admin"
	// ClubPath is step 3: choose the club.
	ClubPath = Path + "/club"
	// DonePath is the confirmation page.
	DonePath = Path + "/done"
	// HealthPath is the liveness endpoint used by the container health check.
	HealthPath = "/healthz"
	// staticPrefix serves CSS, JS, images and the service worker.
	staticPrefix = "/static/"
	// brandingPrefix serves the uploaded club logo (internal/branding), so it
	// can show on the setup pages.
	brandingPrefix = "/branding/"
	// loginPath / logoutPath stay reachable once an administrator exists, so
	// that administrator can sign in to finish the club step.
	loginPath  = "/login"
	logoutPath = "/logout"
	// dashboardPath is where setup hands over.
	dashboardPath = "/admin/league/dashboard"
)

// ticketCookie carries the browser ticket from a redeemed setup code.
const ticketCookie = "setup_ticket"

// AllowedDuringSetup reports whether path is served normally while the
// instance needs setup: static assets, the uploaded logo, the health
// endpoint, and /setup with anything below it.
func AllowedDuringSetup(path string) bool {
	return path == HealthPath ||
		path == Path ||
		strings.HasPrefix(path, Path+"/") ||
		strings.HasPrefix(path, staticPrefix) ||
		strings.HasPrefix(path, brandingPrefix)
}

// Gate routes every request to the setup entry page while the instance needs
// setup, except for AllowedDuringSetup paths (and sign-in / sign-out once an
// administrator exists). Once a home club is in effect it is a pass-through
// costing one read lock.
func (s *State) Gate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if AllowedDuringSetup(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		status := s.Status(r.Context())
		if status == StatusReady ||
			(status == StatusNeedsClub && (r.URL.Path == loginPath || r.URL.Path == logoutPath)) {
			next.ServeHTTP(w, r)
			return
		}
		if r.Header.Get("HX-Request") == "true" {
			// HTMX swaps the body of a 3xx target in place; ask it to navigate instead.
			w.Header().Set("HX-Redirect", Path)
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Redirect(w, r, Path, http.StatusSeeOther)
	})
}

// Sessions is what the setup pages need from sign-in (auth.Service
// implements it).
type Sessions interface {
	// SignedInUser returns the active user behind the request's session.
	SignedInUser(r *http.Request) (*models.User, bool)
	// StartSession signs userID in with a fresh session, discarding any
	// session the request carried.
	StartSession(w http.ResponseWriter, r *http.Request, userID int64) error
	// CookieSecure reports whether cookies should be marked Secure.
	CookieSecure() bool
}

// Handler serves the setup pages.
type Handler struct {
	svc         *Service
	sessions    Sessions
	templateDir string
	templates   *render.Cache
}

// NewHandler returns the setup page handler.
func NewHandler(svc *Service, sessions Sessions, templateDir string) *Handler {
	return &Handler{svc: svc, sessions: sessions, templateDir: templateDir, templates: render.NewCache()}
}

// RegisterRoutes registers the setup pages.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc(Path, h.code)
	mux.HandleFunc(AdminPath, h.admin)
	mux.HandleFunc(ClubPath, h.club)
	mux.HandleFunc(DonePath, h.done)
	mux.HandleFunc(DemoPath, h.demo)
	mux.HandleFunc(DemoClearPath, h.demoClear)
}

// pageData is what every setup template renders.
type pageData struct {
	Step         int // 1-3 for the numbered steps, 0 otherwise
	Status       string
	Error        string
	Notice       string
	CodeUsed     bool // step 1: the current code has already been used
	SignIn       bool // entry page: an administrator exists; sign in to continue
	Username     string
	MinPassword  int
	Clubs        []ClubChoice
	Selected     string
	ClubName     string
	CreatedClub  bool
	DashboardURL string
	// Club step running in the background (reading BHPLTA).
	Working      bool
	ProgressText string
	Retry        bool // the last attempt failed; the form retries it
	// Import is what the BHPLTA bootstrap created (done page).
	Import *bhplta.Summary
	// DemoOffered: the club step offers "Explore with demo data".
	DemoOffered bool
}

// workingQuery marks the club page while it waits for the background club
// step; once setup completes it hands over to the done page.
const workingQuery = "working"

func allowMethods(w http.ResponseWriter, r *http.Request, methods ...string) bool {
	for _, m := range methods {
		if r.Method == m || (m == http.MethodGet && r.Method == http.MethodHead) {
			return true
		}
	}
	w.Header().Set("Allow", strings.Join(methods, ", "))
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	return false
}

// signedInAdmin returns the signed-in administrator, if any.
func (h *Handler) signedInAdmin(r *http.Request) (*models.User, bool) {
	u, ok := h.sessions.SignedInUser(r)
	if !ok || u.Role != models.RoleAdmin {
		return nil, false
	}
	return u, true
}

func (h *Handler) ticket(r *http.Request) string {
	c, err := r.Cookie(ticketCookie)
	if err != nil {
		return ""
	}
	return c.Value
}

func (h *Handler) setTicket(w http.ResponseWriter, ticket string) {
	http.SetCookie(w, &http.Cookie{
		Name:     ticketCookie,
		Value:    ticket,
		Path:     Path,
		MaxAge:   int(ticketLifetime / time.Second),
		HttpOnly: true,
		Secure:   h.sessions.CookieSecure(),
		SameSite: http.SameSiteStrictMode,
	})
}

func (h *Handler) clearTicket(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     ticketCookie,
		Value:    "",
		Path:     Path,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.sessions.CookieSecure(),
		SameSite: http.SameSiteStrictMode,
	})
}

// code is step 1 (GET shows the form or resumes; POST checks the code). It is
// also the page the gate sends everyone to, so with an administrator but no
// club it points the administrator at sign-in or the club step instead.
func (h *Handler) code(w http.ResponseWriter, r *http.Request) {
	if !allowMethods(w, r, http.MethodGet, http.MethodPost) {
		return
	}
	ctx := r.Context()
	status := h.svc.State().Status(ctx)
	switch status {
	case StatusReady:
		http.NotFound(w, r)
		return
	case StatusNeedsClub:
		if r.Method == http.MethodPost {
			// Code entry is gone for good once an administrator exists.
			http.NotFound(w, r)
			return
		}
		if _, ok := h.signedInAdmin(r); ok {
			http.Redirect(w, r, ClubPath, http.StatusSeeOther)
			return
		}
		h.render(w, http.StatusOK, "signin", pageData{SignIn: true, Status: status.String()})
		return
	}

	if r.Method != http.MethodPost {
		if h.svc.TicketValid(ctx, h.ticket(r)) {
			http.Redirect(w, r, AdminPath, http.StatusSeeOther)
			return
		}
		h.render(w, http.StatusOK, "code", pageData{Step: 1, Status: status.String(), CodeUsed: h.svc.CodeWasRedeemed(ctx)})
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}
	ticket, err := h.svc.RedeemCode(ctx, r.PostForm.Get("code"), auth.ClientIP(r))
	if err == nil {
		h.setTicket(w, ticket)
		http.Redirect(w, r, AdminPath, http.StatusSeeOther)
		return
	}
	data := pageData{Step: 1, Status: status.String()}
	code := http.StatusOK
	switch {
	case errors.Is(err, ErrUnavailable):
		http.NotFound(w, r)
		return
	case errors.Is(err, ErrThrottled):
		code = http.StatusTooManyRequests
		data.Error = "Too many wrong codes have been tried. Wait 15 minutes and try again, or restart the app to get a new code."
	case errors.Is(err, ErrWrongCode):
		code = http.StatusUnprocessableEntity
		data.Error = "That code doesn't match. Check you copied the latest code from the server log: each restart prints a new one."
	case errors.Is(err, ErrNoActiveCode):
		data.CodeUsed = h.svc.CodeWasRedeemed(ctx)
		data.Error = "There is no setup code waiting to be used. Restart the app to get a new code."
	default:
		log.Printf("setup: redeem code: %v", err)
		code = http.StatusInternalServerError
		data.Error = "Something went wrong checking the code. Please try again."
	}
	h.render(w, code, "code", data)
}

// admin is step 2: create the first administrator.
func (h *Handler) admin(w http.ResponseWriter, r *http.Request) {
	if !allowMethods(w, r, http.MethodGet, http.MethodPost) {
		return
	}
	ctx := r.Context()
	status := h.svc.State().Status(ctx)
	if !status.NeedsFirstAdmin() {
		// An administrator exists: this step is gone.
		http.NotFound(w, r)
		return
	}
	data := pageData{Step: 2, Status: status.String(), MinPassword: auth.MinPasswordLength}
	if r.Method != http.MethodPost {
		if !h.svc.TicketValid(ctx, h.ticket(r)) {
			http.Redirect(w, r, Path, http.StatusSeeOther)
			return
		}
		h.render(w, http.StatusOK, "admin", data)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}
	username := r.PostForm.Get("username")
	password := r.PostForm.Get("password")
	data.Username = strings.TrimSpace(username)
	if password != r.PostForm.Get("password_confirm") {
		data.Error = "The two passwords don't match. Type the same password in both boxes."
		h.render(w, http.StatusUnprocessableEntity, "admin", data)
		return
	}
	userID, err := h.svc.CreateAdmin(ctx, h.ticket(r), username, password)
	var inputErr *InputError
	switch {
	case err == nil:
	case errors.Is(err, ErrUnavailable):
		http.NotFound(w, r)
		return
	case errors.Is(err, ErrTicketInvalid):
		h.clearTicket(w)
		http.Redirect(w, r, Path, http.StatusSeeOther)
		return
	case errors.As(err, &inputErr):
		data.Error = inputErr.Message
		h.render(w, http.StatusUnprocessableEntity, "admin", data)
		return
	default:
		log.Printf("setup: create administrator: %v", err)
		data.Error = "Something went wrong creating your login. Please try again."
		h.render(w, http.StatusInternalServerError, "admin", data)
		return
	}

	log.Printf("setup: first administrator created (user ID %d)", userID)
	h.clearTicket(w)
	if err := h.sessions.StartSession(w, r, userID); err != nil {
		log.Printf("setup: sign in new administrator: %v", err)
		http.Redirect(w, r, loginPath, http.StatusSeeOther)
		return
	}
	if h.svc.State().Status(ctx) == StatusReady {
		// A home club was already in effect: there is no club to choose.
		http.Redirect(w, r, dashboardPath, http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, ClubPath, http.StatusSeeOther)
}

// club is step 3: choose the club. Only a signed-in administrator can reach it.
func (h *Handler) club(w http.ResponseWriter, r *http.Request) {
	if !allowMethods(w, r, http.MethodGet, http.MethodPost) {
		return
	}
	ctx := r.Context()
	switch h.svc.State().Status(ctx) {
	case StatusReady:
		// The page waiting for the background club step hands over to the
		// done page; otherwise the step is gone.
		if r.Method != http.MethodPost && r.URL.Query().Get(workingQuery) != "" && h.svc.RecentlyCompleted(ctx) {
			if _, ok := h.signedInAdmin(r); ok {
				http.Redirect(w, r, DonePath, http.StatusSeeOther)
				return
			}
		}
		http.NotFound(w, r)
		return
	case StatusNeedsSetup, StatusNeedsAdmin:
		if r.Method == http.MethodPost {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, Path, http.StatusSeeOther)
		return
	}
	if _, ok := h.signedInAdmin(r); !ok {
		if r.Method == http.MethodPost {
			http.Error(w, "Sign in as an administrator to choose the club.", http.StatusForbidden)
			return
		}
		http.Redirect(w, r, Path, http.StatusSeeOther)
		return
	}

	data := pageData{Step: 3, Status: StatusNeedsClub.String(), Clubs: h.svc.Clubs(), DemoOffered: true}
	if r.Method != http.MethodPost {
		if job, ok := h.svc.ClubJob(); ok {
			data.Selected = job.Slug
			switch {
			case job.Running, job.Err == nil && job.Result != nil:
				// A job that has just finished successfully while the status
				// above still read needs-club (this request raced the
				// completion): keep waiting; the next refresh hands over to
				// the done page.
				data.Working = true
				data.ClubName = job.ClubName
				data.ProgressText = progressText(job)
			case job.Err != nil:
				data.Retry = true
				data.Error = clubErrorMessage(job.Err)
			}
		}
		h.render(w, http.StatusOK, "club", data)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}
	data.Selected = r.PostForm.Get("club")
	err := h.svc.StartChooseClub(ctx, data.Selected)
	switch {
	case err == nil, errors.Is(err, ErrClubJobRunning):
		http.Redirect(w, r, ClubPath+"?"+workingQuery+"=1", http.StatusSeeOther)
	case errors.Is(err, ErrUnavailable):
		http.NotFound(w, r)
	case errors.Is(err, ErrUnknownClub):
		data.Error = "Pick your club from the list."
		h.render(w, http.StatusUnprocessableEntity, "club", data)
	default:
		log.Printf("setup: choose club: %v", err)
		data.Error = "Something went wrong setting up your club. Nothing was changed; please try again."
		h.render(w, http.StatusInternalServerError, "club", data)
	}
}

// progressText describes how far the background club step has got.
func progressText(job ClubJob) string {
	if job.Total == 0 || job.What == "" {
		return "Starting..."
	}
	return fmt.Sprintf("Reading %s from the BHPLTA website (%d of %d pages)...", job.What, job.Step, job.Total)
}

// clubErrorMessage explains a failed club step.
func clubErrorMessage(err error) string {
	var pe *ProvisionError
	if errors.As(err, &pe) {
		return pe.Message + " Nothing was changed, so you can try again."
	}
	return "Something went wrong setting up your club. Nothing was changed; please try again."
}

// done is the confirmation page, shown to a signed-in administrator shortly
// after setup completes; otherwise it is not found.
func (h *Handler) done(w http.ResponseWriter, r *http.Request) {
	if !allowMethods(w, r, http.MethodGet) {
		return
	}
	ctx := r.Context()
	if !h.svc.RecentlyCompleted(ctx) {
		http.NotFound(w, r)
		return
	}
	if _, ok := h.signedInAdmin(r); !ok {
		http.NotFound(w, r)
		return
	}
	club := h.svc.State().HomeClub().Club()
	data := pageData{Status: StatusReady.String(), DashboardURL: dashboardPath}
	if club != nil {
		data.ClubName = club.Name
	}
	if job, ok := h.svc.ClubJob(); ok && job.Result != nil {
		data.Import = job.Result.Import
	}
	h.render(w, http.StatusOK, "done", data)
}

// render renders templates/setup/<page>.html inside the app layout.
func (h *Handler) render(w http.ResponseWriter, status int, page string, data pageData) {
	tmpl, err := h.templates.Get(page, func() (*template.Template, error) {
		funcMap := template.FuncMap{"currentYear": func() int { return time.Now().Year() }}
		return template.New("").Funcs(funcMap).ParseFiles(
			filepath.Join(h.templateDir, "layout.html"),
			filepath.Join(h.templateDir, "setup", "parts.html"),
			filepath.Join(h.templateDir, "setup", page+".html"),
		)
	})
	if err != nil {
		log.Printf("setup: parse templates: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	// same-origin, not no-referrer: with no-referrer a browser sends "Origin:
	// null" on form posts, and on plain http to a LAN address (no Sec-Fetch-Site
	// header there) the cross-origin guard would then refuse the setup forms.
	// Nothing is sent to other sites either way.
	w.Header().Set("Referrer-Policy", "same-origin")
	rec := &statusWriter{ResponseWriter: w, status: status}
	if err := render.Execute(rec, tmpl, "setup/"+page+".html", data); err != nil {
		log.Printf("setup: render %s: %v", page, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// statusWriter writes status before the first body byte, so render.Execute
// (which only writes once rendering succeeded) can still fall back to 500.
type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (s *statusWriter) Write(b []byte) (int, error) {
	if !s.wroteHeader {
		s.wroteHeader = true
		s.ResponseWriter.Header().Set("Content-Type", "text/html; charset=utf-8")
		s.ResponseWriter.WriteHeader(s.status)
	}
	return s.ResponseWriter.Write(b)
}
