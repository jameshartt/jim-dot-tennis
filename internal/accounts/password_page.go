// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package accounts

import (
	"errors"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/render"
)

// PasswordPage serves auth.ChangePasswordPath, where a signed-in user who
// has a password chooses a new one (linked from the sign-in methods page).
// Nobody is forced here: passwords are only ever chosen by their owner.
type PasswordPage struct {
	accounts    *Service
	auth        *auth.Service
	templateDir string
	afterPath   string
}

// NewPasswordPage returns the page. afterPath is where the user goes once
// the password is changed.
func NewPasswordPage(accounts *Service, authService *auth.Service, templateDir, afterPath string) *PasswordPage {
	return &PasswordPage{accounts: accounts, auth: authService, templateDir: templateDir, afterPath: afterPath}
}

// RegisterRoutes adds the page to mux.
func (p *PasswordPage) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc(auth.ChangePasswordPath, p.serve)
}

type passwordPageData struct {
	Username    string
	Error       string
	MinPassword int
}

var passwordTemplates = render.NewCache()

func (p *PasswordPage) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	user, ok := p.auth.SignedInUser(r)
	if !ok {
		http.Redirect(w, r, "/login?redirect="+url.QueryEscape(auth.ChangePasswordPath), http.StatusSeeOther)
		return
	}
	if user.PasswordHash == "" {
		// Nothing to change: the account signs in with passkeys.
		http.Redirect(w, r, auth.SignInMethodsPath, http.StatusSeeOther)
		return
	}
	data := passwordPageData{Username: user.Username, MinPassword: auth.MinPasswordLength}
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		p.render(w, http.StatusOK, data)
	case http.MethodPost:
		current := r.PostFormValue("current_password")
		next := r.PostFormValue("new_password")
		if next != r.PostFormValue("new_password_confirm") {
			data.Error = "The two new passwords are not the same. Type them again."
			p.render(w, http.StatusUnprocessableEntity, data)
			return
		}
		err := p.accounts.ChangeOwnPassword(r.Context(), user.ID, current, next)
		switch {
		case err == nil:
		case errors.Is(err, ErrNoPassword):
			http.Redirect(w, r, auth.SignInMethodsPath, http.StatusSeeOther)
			return
		case errors.Is(err, ErrWrongPassword):
			data.Error = "Your current password is not right."
			p.render(w, http.StatusUnprocessableEntity, data)
			return
		case errors.Is(err, auth.ErrPasswordPolicy):
			data.Error = strings.TrimPrefix(err.Error(), auth.ErrPasswordPolicy.Error()+": ")
			p.render(w, http.StatusUnprocessableEntity, data)
			return
		default:
			log.Printf("change password for user %d: %v", user.ID, err)
			http.Error(w, "Could not change the password. Try again.", http.StatusInternalServerError)
			return
		}
		// Every session was ended with the old password; sign this browser
		// in again with a new session.
		if err := p.auth.StartSession(w, r, user.ID); err != nil {
			log.Printf("change password: start new session for user %d: %v", user.ID, err)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, p.afterPath, http.StatusSeeOther)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (p *PasswordPage) render(w http.ResponseWriter, status int, data passwordPageData) {
	tmpl, err := passwordTemplates.Get(p.templateDir, func() (*template.Template, error) {
		funcMap := template.FuncMap{"currentYear": func() int { return time.Now().Year() }}
		return template.New("").Funcs(funcMap).ParseFiles(
			filepath.Join(p.templateDir, "layout.html"),
			filepath.Join(p.templateDir, "account_password.html"),
		)
	})
	if err != nil {
		log.Printf("change password page: parse templates: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := render.Execute(w, tmpl, "account_password.html", data); err != nil {
		log.Printf("change password page: %v", err)
	}
}
