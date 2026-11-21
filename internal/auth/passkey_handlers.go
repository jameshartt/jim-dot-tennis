// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"time"

	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/render"
	"jim-dot-tennis/internal/repository"
)

// Passkey routes. All state-changing ones are POST, so the app-wide
// CrossOriginGuard covers them; the JSON ones additionally require a JSON
// content type, which a cross-site form cannot send.
const (
	// SignInMethodsPath is the signed-in user's "Sign-in methods" page.
	SignInMethodsPath = "/account/signin-methods"
	// PasskeysPathPrefix is the prefix of the signed-in passkey endpoints.
	PasskeysPathPrefix = "/account/passkeys/"

	passkeyLoginBeginPath     = "/login/passkey/begin"
	passkeyLoginFinishPath    = "/login/passkey/finish"
	passkeyRegisterBeginPath  = PasskeysPathPrefix + "begin"
	passkeyRegisterFinishPath = PasskeysPathPrefix + "finish"
)

// PasskeyHandler serves passkey sign-in, enrolment and the sign-in methods page.
type PasskeyHandler struct {
	passkeys     *Passkeys
	service      *Service
	templateDir  string
	redirectPath string
	templates    *render.Cache
}

// NewPasskeyHandler returns the passkey handler. redirectPath is where a
// passkey sign-in lands when the login page carried no redirect.
func NewPasskeyHandler(passkeys *Passkeys, templateDir, redirectPath string) *PasskeyHandler {
	return &PasskeyHandler{
		passkeys:     passkeys,
		service:      passkeys.service,
		templateDir:  templateDir,
		redirectPath: redirectPath,
		templates:    render.NewCache(),
	}
}

// RegisterRoutes registers the passkey routes. Sign-in is public; everything
// under /account/ needs a signed-in session and acts on that user only.
func (h *PasskeyHandler) RegisterRoutes(mux *http.ServeMux, m *Middleware) {
	mux.HandleFunc("POST "+passkeyLoginBeginPath, h.loginBegin)
	mux.HandleFunc("POST "+passkeyLoginFinishPath, h.loginFinish)

	signedIn := func(fn http.HandlerFunc) http.Handler { return m.RequireAuth(fn) }
	mux.Handle("GET "+SignInMethodsPath, signedIn(h.signInMethods))
	mux.Handle("POST "+passkeyRegisterBeginPath, signedIn(h.registerBegin))
	mux.Handle("POST "+passkeyRegisterFinishPath, signedIn(h.registerFinish))
	mux.Handle("POST "+PasskeysPathPrefix+"{id}/rename", signedIn(h.rename))
	mux.Handle("POST "+PasskeysPathPrefix+"{id}/delete", signedIn(h.remove))
	mux.Handle("POST "+PasskeysPathPrefix+"{id}/clear-warning",
		m.RequireAuth(m.RequireRole(models.RoleAdmin)(http.HandlerFunc(h.clearWarning))))
}

// passkeyError is the JSON body of a failed passkey request.
type passkeyError struct {
	Error string `json:"error"`
}

func writePasskeyJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("passkey: write response: %v", err)
	}
}

// writePasskeyFailure maps a passkey error to a status and a message for the
// person. Verification failures all get the same message.
func writePasskeyFailure(w http.ResponseWriter, err error) {
	status, msg := http.StatusInternalServerError, "Something went wrong. Please try again."
	switch {
	case errors.Is(err, ErrPasskeysUnavailable):
		status, msg = http.StatusServiceUnavailable, "Passkeys can't be used at this web address."
	case errors.Is(err, ErrPasskeyThrottled):
		status, msg = http.StatusTooManyRequests, "Too many attempts. Wait a few minutes and try again."
	case errors.Is(err, ErrPasskeyCeremony):
		status, msg = http.StatusBadRequest, "That passkey request has expired. Please try again."
	case errors.Is(err, ErrPasskeyRejected):
		status, msg = http.StatusUnauthorized, "That passkey wasn't accepted. Try again, or sign in another way."
	case errors.Is(err, ErrPasskeyBlocked):
		status, msg = http.StatusForbidden, "This passkey has been blocked for a security check. Sign in another way, or ask an administrator."
	case errors.Is(err, ErrPasskeyLimit):
		status, msg = http.StatusConflict, "This account already has the most passkeys it can hold. Remove one first."
	case errors.Is(err, repository.ErrPasskeyExists):
		status, msg = http.StatusConflict, "That passkey is already registered."
	default:
		log.Printf("passkey: %v", err)
	}
	writePasskeyJSON(w, status, passkeyError{Error: msg})
}

// requireJSON refuses requests whose body is not declared as JSON. Browsers
// cannot send that content type cross-site without a CORS preflight, which
// this app never answers.
func requireJSON(w http.ResponseWriter, r *http.Request) bool {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mt != "application/json" {
		writePasskeyJSON(w, http.StatusUnsupportedMediaType, passkeyError{Error: "Expected a JSON request."})
		return false
	}
	return true
}

func (h *PasskeyHandler) setCeremonyCookie(w http.ResponseWriter, ticket string) {
	h.passkeys.setCeremonyCookie(w, ticket)
}

// takeCeremonyCookie returns the browser's ceremony ticket and clears the cookie.
func (h *PasskeyHandler) takeCeremonyCookie(w http.ResponseWriter, r *http.Request) string {
	return h.passkeys.takeCeremonyCookie(w, r)
}

// loginBegin starts a passkey sign-in. The optional JSON body carries a
// username for the username-first path.
func (h *PasskeyHandler) loginBegin(w http.ResponseWriter, r *http.Request) {
	if !requireJSON(w, r) {
		return
	}
	var req struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writePasskeyJSON(w, http.StatusBadRequest, passkeyError{Error: "The request could not be read."})
		return
	}
	if len(req.Username) > 256 {
		req.Username = req.Username[:256]
	}
	options, ticket, err := h.passkeys.BeginLogin(r, req.Username)
	if err != nil {
		writePasskeyFailure(w, err)
		return
	}
	h.setCeremonyCookie(w, ticket)
	writePasskeyJSON(w, http.StatusOK, options)
}

// loginFinish verifies the passkey response and signs the user in with the
// same session a password sign-in creates.
func (h *PasskeyHandler) loginFinish(w http.ResponseWriter, r *http.Request) {
	if !requireJSON(w, r) {
		return
	}
	user, err := h.passkeys.FinishLogin(r, h.takeCeremonyCookie(w, r), r.Body)
	if err != nil {
		writePasskeyFailure(w, err)
		return
	}
	session, err := h.service.completeLogin(user, r)
	if err != nil {
		writePasskeyFailure(w, err)
		return
	}
	h.service.SetSessionCookie(w, session)
	target := safeRedirectPath(r.URL.Query().Get("redirect"))
	if target == "" {
		target = h.redirectPath
	}
	writePasskeyJSON(w, http.StatusOK, map[string]string{"redirect": target})
}

// contextUser returns the signed-in user RequireAuth put in the context.
func contextUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
	user, err := GetUserFromContext(r.Context())
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return nil, false
	}
	return &user, true
}

// registerBegin starts enrolling a passkey for the signed-in user.
func (h *PasskeyHandler) registerBegin(w http.ResponseWriter, r *http.Request) {
	user, ok := contextUser(w, r)
	if !ok || !requireJSON(w, r) {
		return
	}
	options, ticket, err := h.passkeys.BeginRegistration(r, user, h.service.sessionFingerprint(r))
	if err != nil {
		writePasskeyFailure(w, err)
		return
	}
	h.setCeremonyCookie(w, ticket)
	writePasskeyJSON(w, http.StatusOK, options)
}

// registerFinish stores the new passkey. Its name comes from the "name"
// query parameter (the body is the browser's credential).
func (h *PasskeyHandler) registerFinish(w http.ResponseWriter, r *http.Request) {
	user, ok := contextUser(w, r)
	if !ok || !requireJSON(w, r) {
		return
	}
	cred, err := h.passkeys.FinishRegistration(r, user, h.service.sessionFingerprint(r),
		h.takeCeremonyCookie(w, r), r.Body, r.URL.Query().Get("name"))
	if err != nil {
		writePasskeyFailure(w, err)
		return
	}
	writePasskeyJSON(w, http.StatusOK, map[string]any{"id": cred.ID, "name": cred.Name})
}

// backToSignInMethods redirects to the page with a result code it turns into
// a message.
func backToSignInMethods(w http.ResponseWriter, r *http.Request, key, code string) {
	http.Redirect(w, r, SignInMethodsPath+"?"+key+"="+url.QueryEscape(code), http.StatusSeeOther)
}

// passkeyID reads the {id} path value.
func passkeyID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

// rename renames one of the signed-in user's own passkeys.
func (h *PasskeyHandler) rename(w http.ResponseWriter, r *http.Request) {
	user, ok := contextUser(w, r)
	if !ok {
		return
	}
	id, ok := passkeyID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	err := h.passkeys.Rename(r.Context(), user, id, r.PostFormValue("name"))
	switch {
	case err == nil:
		backToSignInMethods(w, r, "done", "renamed")
	case errors.Is(err, repository.ErrPasskeyNotFound):
		http.NotFound(w, r)
	default:
		log.Printf("passkey: rename %d for user %d: %v", id, user.ID, err)
		backToSignInMethods(w, r, "error", "failed")
	}
}

// remove deletes one of the signed-in user's own passkeys, never their last
// sign-in method.
func (h *PasskeyHandler) remove(w http.ResponseWriter, r *http.Request) {
	user, ok := contextUser(w, r)
	if !ok {
		return
	}
	id, ok := passkeyID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	err := h.passkeys.Remove(r.Context(), user, id)
	switch {
	case err == nil:
		backToSignInMethods(w, r, "done", "removed")
	case errors.Is(err, repository.ErrLastSignInMethod):
		backToSignInMethods(w, r, "error", "last")
	case errors.Is(err, repository.ErrPasskeyNotFound):
		http.NotFound(w, r)
	default:
		log.Printf("passkey: remove %d for user %d: %v", id, user.ID, err)
		backToSignInMethods(w, r, "error", "failed")
	}
}

// clearWarning (administrators only) lets a passkey with a clone warning
// sign in again.
func (h *PasskeyHandler) clearWarning(w http.ResponseWriter, r *http.Request) {
	admin, ok := contextUser(w, r)
	if !ok {
		return
	}
	id, ok := passkeyID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	err := h.passkeys.ClearCloneWarning(r.Context(), admin, id)
	switch {
	case err == nil:
		backToSignInMethods(w, r, "done", "cleared")
	case errors.Is(err, repository.ErrPasskeyNotFound):
		http.NotFound(w, r)
	default:
		log.Printf("passkey: clear warning %d: %v", id, err)
		backToSignInMethods(w, r, "error", "failed")
	}
}

// passkeyRow is one passkey on the sign-in methods page.
type passkeyRow struct {
	ID         int64
	Name       string
	CreatedAt  time.Time
	LastUsedAt *time.Time
	Synced     bool // backed up by its provider (iCloud Keychain, Google Password Manager...)
	Blocked    bool // has a clone warning
}

// signInMethodsData is what templates/account/signin_methods.html renders.
// WI-130 extends the page with adding and removing a password.
type signInMethodsData struct {
	Username string
	Passkeys []passkeyRow
	// HasPassword: the account has a password (an invited or recovered
	// account may have passkeys only).
	HasPassword bool
	// CanRemove: the account has more than one sign-in method, so one can go.
	CanRemove bool
	// PasskeysAvailable: passkeys can be enrolled at this web address.
	PasskeysAvailable bool
	AtLimit           bool
	MaxNameLength     int
	Notice            string
	Error             string
	// Next is a same-site path to continue to ("" for none), e.g. back to
	// first-run setup.
	Next string
	// Enrol asks the page to draw attention to adding a passkey.
	Enrol    bool
	IsAdmin  bool
	Warnings []models.PasskeyWarning // administrators only
	HomePath string
}

var signInMethodsNotices = map[string]string{
	"added":   "Passkey added. You can use it the next time you sign in.",
	"renamed": "Passkey renamed.",
	"removed": "Passkey removed. Also delete it from your device or password manager, so it is not offered again.",
	"cleared": "Warning cleared. That passkey can sign in again.",
}

var signInMethodsErrors = map[string]string{
	"last":   "That is your only way to sign in, so it can't be removed. Add another sign-in method first.",
	"failed": "Something went wrong. Nothing was changed; please try again.",
}

// signInMethods shows the signed-in user's passkeys and password.
func (h *PasskeyHandler) signInMethods(w http.ResponseWriter, r *http.Request) {
	user, ok := contextUser(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	creds, err := h.passkeys.List(ctx, user.ID)
	if err != nil {
		log.Printf("sign-in methods: list passkeys for user %d: %v", user.ID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	methods, err := h.service.CountSignInMethods(ctx, user.ID)
	if err != nil {
		log.Printf("sign-in methods: count for user %d: %v", user.ID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	q := r.URL.Query()
	data := signInMethodsData{
		Username:          user.Username,
		HasPassword:       user.PasswordHash != "",
		CanRemove:         methods > 1,
		PasskeysAvailable: h.passkeys.Available(r),
		AtLimit:           len(creds) >= MaxPasskeysPerUser,
		MaxNameLength:     MaxPasskeyNameLength,
		Notice:            signInMethodsNotices[q.Get("done")],
		Error:             signInMethodsErrors[q.Get("error")],
		Next:              safeRedirectPath(q.Get("next")),
		Enrol:             q.Get("enrol") != "",
		IsAdmin:           user.Role == models.RoleAdmin,
		HomePath:          h.redirectPath,
	}
	for _, c := range creds {
		data.Passkeys = append(data.Passkeys, passkeyRow{
			ID: c.ID, Name: c.Name, CreatedAt: c.CreatedAt, LastUsedAt: c.LastUsedAt,
			Synced: c.BackupState, Blocked: c.CloneWarningAt != nil,
		})
	}
	if data.IsAdmin {
		if data.Warnings, err = h.passkeys.CloneWarnings(ctx); err != nil {
			log.Printf("sign-in methods: list clone warnings: %v", err)
		}
	}

	tmpl, err := h.templates.Get("signin_methods", func() (*template.Template, error) {
		funcMap := template.FuncMap{"currentYear": func() int { return time.Now().Year() }}
		return template.New("").Funcs(funcMap).ParseFiles(
			filepath.Join(h.templateDir, "layout.html"),
			filepath.Join(h.templateDir, "account", "signin_methods.html"),
		)
	})
	if err != nil {
		log.Printf("sign-in methods: parse templates: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if err := render.Execute(w, tmpl, "account/signin_methods.html", data); err != nil {
		log.Printf("sign-in methods: render: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
