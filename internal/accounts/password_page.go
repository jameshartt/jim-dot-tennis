// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package accounts

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"time"

	"jim-dot-tennis/internal/auth"
)

// Wrong current passwords typed on the password form are limited per client
// address (in memory), so a stolen session cannot be used to guess the
// password at leisure.
const (
	currentPasswordFailureLimit  = 5
	currentPasswordFailureWindow = 15 * time.Minute
)

// PasswordPage handles a signed-in user's own password. The form lives in
// the "Password" section of the sign-in methods page (auth.SignInMethodsPath);
// these endpoints act and send the browser back there with a result code:
//
//	GET  /account/password         redirect to the password section
//	POST /account/password         add or change the password
//	POST /account/password/remove  remove the password
//
// Adding or changing needs the current password or a passkey confirmation
// made in this session (auth.Passkeys.ConsumeReauthProof); a session alone is
// never enough. Changing signs the account out everywhere else and keeps this
// browser signed in with a new session.
type PasswordPage struct {
	accounts *Service
	auth     *auth.Service
	passkeys *auth.Passkeys // nil: no passkey confirmation
	failures *auth.AttemptLimiter
	now      func() time.Time
}

// NewPasswordPage returns the password endpoints. passkeys may be nil.
func NewPasswordPage(accounts *Service, authService *auth.Service, passkeys *auth.Passkeys) *PasswordPage {
	return &PasswordPage{
		accounts: accounts,
		auth:     authService,
		passkeys: passkeys,
		failures: auth.NewAttemptLimiter(currentPasswordFailureLimit, currentPasswordFailureWindow),
		now:      time.Now,
	}
}

// RegisterRoutes adds the endpoints to mux.
func (p *PasswordPage) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc(auth.PasswordPath, p.serve)
	mux.HandleFunc("POST "+auth.PasswordRemovePath, p.remove)
}

// back sends the browser to the password section with a result.
func back(w http.ResponseWriter, r *http.Request, key, code string) {
	http.Redirect(w, r, auth.SignInMethodsPath+"?"+key+"="+url.QueryEscape(code)+"#password", http.StatusSeeOther)
}

func (p *PasswordPage) signedIn(w http.ResponseWriter, r *http.Request) (int64, bool) {
	w.Header().Set("Cache-Control", "no-store")
	user, ok := p.auth.SignedInUser(r)
	if !ok {
		http.Redirect(w, r, "/login?redirect="+url.QueryEscape(auth.SignInMethodsPath), http.StatusSeeOther)
		return 0, false
	}
	return user.ID, true
}

func (p *PasswordPage) serve(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		// The form is on the sign-in methods page.
		http.Redirect(w, r, auth.SignInMethodsPath+"#password", http.StatusSeeOther)
	case http.MethodPost:
		p.set(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// set adds or changes the signed-in user's password.
func (p *PasswordPage) set(w http.ResponseWriter, r *http.Request) {
	userID, ok := p.signedIn(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	current := r.PostFormValue("current_password")
	next := r.PostFormValue("new_password")
	if next != r.PostFormValue("new_password_confirm") {
		back(w, r, "perror", auth.PasswordErrMismatch)
		return
	}
	if current != "" && p.failures.Blocked(r, p.now()) {
		back(w, r, "perror", auth.PasswordErrWrong)
		return
	}
	change := PasswordChange{Current: current, New: next}
	if current == "" && p.passkeys != nil {
		change.ConsumePasskeyProof = func() bool { return p.passkeys.ConsumeReauthProof(w, r, userID) }
	}
	res, err := p.accounts.SetOwnPassword(r.Context(), userID, change)
	if err != nil {
		code := auth.PasswordErrFailed
		var pe *auth.PasswordPolicyError
		switch {
		case errors.As(err, &pe):
			code = pe.Code
		case errors.Is(err, ErrWrongPassword):
			p.failures.Fail(r, p.now())
			code = auth.PasswordErrWrong
		case errors.Is(err, ErrReauthRequired):
			code = auth.PasswordErrReauth
		default:
			log.Printf("set password for user %d: %v", userID, err)
		}
		back(w, r, "perror", code)
		return
	}
	if res.Added {
		back(w, r, "done", "password-added")
		return
	}
	// Every session ended with the change; this browser carries on with a
	// fresh one, so only the other devices are signed out.
	if err := p.auth.StartSession(w, r, userID); err != nil {
		log.Printf("change password: start new session for user %d: %v", userID, err)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	back(w, r, "done", "password-changed")
}

// remove removes the signed-in user's password, never their last working
// way to sign in.
func (p *PasswordPage) remove(w http.ResponseWriter, r *http.Request) {
	userID, ok := p.signedIn(w, r)
	if !ok {
		return
	}
	err := p.accounts.RemoveOwnPassword(r.Context(), userID)
	switch {
	case err == nil:
		back(w, r, "done", "password-removed")
	case errors.Is(err, ErrLastSignInMethod):
		back(w, r, "perror", auth.PasswordErrLast)
	case errors.Is(err, ErrNoPassword):
		back(w, r, "perror", auth.PasswordErrNone)
	default:
		log.Printf("remove password for user %d: %v", userID, err)
		back(w, r, "perror", auth.PasswordErrFailed)
	}
}
