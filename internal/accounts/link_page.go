// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package accounts

import (
	"errors"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/render"
	"jim-dot-tennis/internal/repository"
)

// Guessing at link addresses is slowed per client address: after
// linkFailureLimit requests for links that are not valid within
// linkFailureWindow, that address gets "too many attempts" for every link
// request until the window has passed. The count is by address only, never by
// link or by account, so nobody can use it to block someone else's link.
const (
	linkFailureLimit  = 10
	linkFailureWindow = 15 * time.Minute
)

// LinkPage serves invite and recovery links (auth.AccountLinkPathPrefix):
//
//	GET  /account/link/{token}                 who the link is for and what it will do; changes nothing
//	POST /account/link/{token}                 set a password, sign in
//	POST /account/link/{token}/passkey/begin   start enrolling a passkey (JSON)
//	POST /account/link/{token}/passkey/finish  enrol the passkey, sign in (JSON)
//
// None of them needs a session: holding the link is the authorisation. Every
// response is uncacheable and sends no referrer to any other site, and nothing here logs the
// request path or the token. A link that is wrong, expired, used, cancelled
// or for a disabled account gets one and the same answer.
type LinkPage struct {
	accounts    *Service
	auth        *auth.Service
	passkeys    *auth.Passkeys // nil: passkeys are never offered
	templateDir string
	afterPath   string
	failures    *auth.AttemptLimiter
	templates   *render.Cache
	now         func() time.Time
}

// NewLinkPage returns the page. afterPath is where the person lands once the
// link is used and they are signed in.
func NewLinkPage(accounts *Service, authService *auth.Service, passkeys *auth.Passkeys, templateDir, afterPath string) *LinkPage {
	return &LinkPage{
		accounts:    accounts,
		auth:        authService,
		passkeys:    passkeys,
		templateDir: templateDir,
		afterPath:   afterPath,
		failures:    auth.NewAttemptLimiter(linkFailureLimit, linkFailureWindow),
		templates:   render.NewCache(),
		now:         time.Now,
	}
}

// RegisterRoutes adds the link routes to mux. The whole subtree is taken, so
// a mistyped or over-long link address never falls through to a handler that
// logs the path.
func (p *LinkPage) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc(auth.AccountLinkPathPrefix, p.serve)
}

type linkPageData struct {
	Invalid   bool // the link cannot be used (one page for every reason)
	Throttled bool

	Path              string // this link's own address, for the form
	IsInvite          bool
	Username          string
	Role              string
	ExpiresAt         time.Time
	PasskeysAvailable bool
	// What the account has today (recovery links), to explain the choice.
	ExistingPasskeys int
	HasPassword      bool
	RemoveChecked    bool
	MinPassword      int
	MaxNameLength    int
	Error            string
	// PasswordAllowed: the page may offer a password (false for an
	// administrator while passkeys are required for administrators).
	PasswordAllowed bool
}

const (
	linkInvalidMessage   = "This link can't be used. It may be mistyped, out of date, already used or cancelled. Ask a club administrator for a new one."
	linkThrottledMessage = "Too many attempts. Wait a few minutes and try again."
)

func (p *LinkPage) serve(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("Cache-Control", "no-store")
	// same-origin: the link's address (which is the secret) is never sent
	// to another site in a Referer header, and the page links nowhere else
	// anyway. Not no-referrer: with that policy a browser posts the page's
	// own form with "Origin: null", and where it sends no Sec-Fetch-Site
	// header (plain http to a host other than localhost) the cross-origin
	// guard would refuse the form, so the link could not be used.
	h.Set("Referrer-Policy", "same-origin")
	h.Set("X-Robots-Tag", "noindex, nofollow")

	token, action, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, auth.AccountLinkPathPrefix), "/")
	isJSON := action != ""
	switch {
	case action == "" && (r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodPost):
	case (action == "passkey/begin" || action == "passkey/finish") && r.Method == http.MethodPost:
	case action == "" || action == "passkey/begin" || action == "passkey/finish":
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	default:
		p.invalid(w, r, false)
		return
	}

	now := p.now()
	if p.failures.Blocked(r, now) {
		if isJSON {
			auth.WritePasskeyJSON(w, http.StatusTooManyRequests, map[string]string{"error": linkThrottledMessage})
			return
		}
		p.render(w, http.StatusTooManyRequests, linkPageData{Throttled: true})
		return
	}
	link, err := p.accounts.LookupLink(r.Context(), token)
	if err != nil {
		if !errors.Is(err, ErrLinkInvalid) {
			log.Printf("account link: look up: %v", err)
			http.Error(w, "Something went wrong. Please try again.", http.StatusInternalServerError)
			return
		}
		p.invalid(w, r, isJSON)
		return
	}

	switch {
	case action == "passkey/begin":
		p.passkeyBegin(w, r, link, token)
	case action == "passkey/finish":
		p.passkeyFinish(w, r, link, token)
	case r.Method == http.MethodPost:
		p.setPassword(w, r, link, token)
	default:
		p.render(w, http.StatusOK, p.pageData(r, link, token))
	}
}

// invalid answers a request for a link that cannot be used and counts it
// against the client address.
func (p *LinkPage) invalid(w http.ResponseWriter, r *http.Request, asJSON bool) {
	p.failures.Fail(r, p.now())
	if asJSON {
		auth.WritePasskeyJSON(w, http.StatusNotFound, map[string]string{"error": linkInvalidMessage})
		return
	}
	p.render(w, http.StatusNotFound, linkPageData{Invalid: true})
}

func (p *LinkPage) passkeysAvailable(r *http.Request) bool {
	return p.passkeys != nil && p.passkeys.Available(r)
}

func (p *LinkPage) pageData(r *http.Request, link Link, token string) linkPageData {
	data := linkPageData{
		Path:              LinkPath(token),
		IsInvite:          link.Kind == LinkInvite,
		Username:          link.Username,
		Role:              string(link.Role),
		ExpiresAt:         link.ExpiresAt,
		PasskeysAvailable: p.passkeysAvailable(r),
		MinPassword:       auth.MinPasswordLength,
		MaxNameLength:     auth.MaxPasskeyNameLength,
	}
	if account, err := p.accounts.Get(r.Context(), link.UserID); err == nil {
		data.ExistingPasskeys = account.Passkeys
		data.HasPassword = account.HasPassword
	}
	data.PasswordAllowed = true
	if link.Role == models.RoleAdmin && data.PasskeysAvailable {
		if required, err := repository.AdminPasskeysRequired(r.Context(), p.accounts.db); err != nil || required {
			data.PasswordAllowed = false
		}
	}
	return data
}

// linkUser is the account behind link in the form the passkey code takes.
func linkUser(link Link) *models.User {
	return &models.User{ID: link.UserID, Username: link.Username, Role: link.Role, IsActive: link.UserActive}
}

// passkeyBegin starts enrolling a passkey for the link's account. The
// ceremony is tied to this browser (ceremony cookie), this account and this
// link (LinkBinding); the link is not touched.
func (p *LinkPage) passkeyBegin(w http.ResponseWriter, r *http.Request, link Link, token string) {
	if !auth.RequireJSON(w, r) {
		return
	}
	if p.passkeys == nil {
		auth.WritePasskeyFailure(w, auth.ErrPasskeysUnavailable)
		return
	}
	options, err := p.passkeys.BeginLinkRegistration(w, r, linkUser(link), LinkBinding(token))
	if err != nil {
		auth.WritePasskeyFailure(w, err)
		return
	}
	auth.WritePasskeyJSON(w, http.StatusOK, options)
}

// passkeyFinish verifies the new passkey, then uses the link: the passkey is
// stored in the transaction that consumes the link. A passkey that does not
// verify leaves the link unused, so the person can try again.
func (p *LinkPage) passkeyFinish(w http.ResponseWriter, r *http.Request, link Link, token string) {
	if !auth.RequireJSON(w, r) {
		return
	}
	if p.passkeys == nil {
		auth.WritePasskeyFailure(w, auth.ErrPasskeysUnavailable)
		return
	}
	q := r.URL.Query()
	cred, err := p.passkeys.VerifyLinkRegistration(w, r, linkUser(link), LinkBinding(token), q.Get("name"))
	if err != nil {
		auth.WritePasskeyFailure(w, err)
		return
	}
	_, err = p.accounts.RedeemLink(r.Context(), token, Redemption{Passkey: cred, RemoveOtherMethods: q.Get("remove") == "1"})
	switch {
	case err == nil:
	case errors.Is(err, ErrLinkInvalid):
		p.invalid(w, r, true)
		return
	default:
		auth.WritePasskeyFailure(w, err)
		return
	}
	auth.WritePasskeyJSON(w, http.StatusOK, map[string]string{"redirect": p.signIn(w, r, link.UserID)})
}

// setPassword sets the password typed on the page and uses the link.
func (p *LinkPage) setPassword(w http.ResponseWriter, r *http.Request, link Link, token string) {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	password := r.PostFormValue("new_password")
	remove := r.PostFormValue("remove_other_methods") != ""
	retry := func(msg string) {
		data := p.pageData(r, link, token)
		data.Error = msg
		data.RemoveChecked = remove
		p.render(w, http.StatusUnprocessableEntity, data)
	}
	if password == "" {
		retry("Type a password, or use a passkey.")
		return
	}
	if password != r.PostFormValue("new_password_confirm") {
		retry("The two passwords are not the same. Type them again.")
		return
	}
	_, err := p.accounts.RedeemLink(r.Context(), token, Redemption{Password: password, RemoveOtherMethods: remove,
		PasskeysUnavailable: !p.passkeysAvailable(r)})
	switch {
	case err == nil:
	case errors.Is(err, ErrLinkInvalid):
		p.invalid(w, r, false)
		return
	case errors.Is(err, ErrAdminPasskeyRequired):
		retry("The club requires administrators to sign in with a passkey, so set up a passkey above. A password can't be used for this account.")
		return
	case errors.Is(err, auth.ErrPasswordPolicy):
		msg, _ := auth.PolicyMessage(err)
		retry(msg)
		return
	default:
		log.Printf("account link: set password for user %d: %v", link.UserID, err)
		http.Error(w, "Something went wrong. Nothing was changed; please try again.", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, p.signIn(w, r, link.UserID), http.StatusSeeOther)
}

// signIn starts a session for the account whose link was just used and
// returns where to send the browser. The link is already spent, so if the
// session cannot be started the person signs in with what they just set up.
func (p *LinkPage) signIn(w http.ResponseWriter, r *http.Request, userID int64) string {
	if err := p.auth.StartSession(w, r, userID); err != nil {
		log.Printf("account link: start session for user %d: %v", userID, err)
		return "/login"
	}
	return p.auth.HomePath(r.Context(), userID, p.afterPath)
}

func (p *LinkPage) render(w http.ResponseWriter, status int, data linkPageData) {
	tmpl, err := p.templates.Get("accept_link", func() (*template.Template, error) {
		funcMap := template.FuncMap{"currentYear": func() int { return time.Now().Year() }}
		return render.ParseFiles(template.New("").Funcs(funcMap),
			filepath.Join(p.templateDir, "layout.html"),
			filepath.Join(p.templateDir, "account", "accept_link.html"),
		)
	})
	if err != nil {
		log.Printf("account link page: parse templates: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := render.Execute(w, tmpl, "account/accept_link.html", data); err != nil {
		log.Printf("account link page: %v", err)
	}
}
