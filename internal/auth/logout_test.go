// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jim-dot-tennis/internal/models"
)

// Administrators' sign-ins reach the audit hook with their method; other
// roles' do not.
func TestSignInAuditForAdministratorsOnly(t *testing.T) {
	env := newPasskeyEnv(t)
	var got []string
	env.svc.SetSignInAudit(func(_ context.Context, user *models.User, method, address string) {
		got = append(got, user.Username+" "+method+" "+address)
	})
	env.addUser("ann", models.RoleAdmin)
	captainID := env.addUser("cap", models.RoleCaptain)
	env.browser("198.51.100.1").passwordSignIn("ann")
	env.browser("198.51.100.2").passwordSignIn("cap")
	r := httptest.NewRequest(http.MethodPost, "/account/link/x", nil)
	r.RemoteAddr = "198.51.100.3:1"
	if err := env.svc.StartSession(httptest.NewRecorder(), r, captainID); err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ";") != "ann password 198.51.100.1" {
		t.Errorf("sign-ins recorded: %q", got)
	}
}

// Logout is POST only: a GET (a link, an image, a prefetch) changes nothing,
// and a POST from another site is refused by the cross-origin guard.
func TestLogoutRequiresPOST(t *testing.T) {
	env := newPasskeyEnv(t)
	env.addUser("alice", models.RoleAdmin)
	b := env.browser("198.51.100.1")
	b.passwordSignIn("alice")
	token := b.cookies["session_token"]
	valid := func() bool {
		_, err := env.svc.ValidateSession(token, httptest.NewRequest(http.MethodGet, "/", nil))
		return err == nil
	}

	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPut} {
		rec := b.do(method, "/logout", "", nil)
		wantStatus(t, rec, http.StatusMethodNotAllowed)
		if rec.Header().Get("Allow") != http.MethodPost {
			t.Errorf("%s /logout: Allow %q", method, rec.Header().Get("Allow"))
		}
		if !valid() || !b.signedIn() {
			t.Fatalf("%s /logout signed the user out", method)
		}
	}

	// A cross-site form post is blocked before it reaches the handler.
	b.headers["Sec-Fetch-Site"] = "cross-site"
	wantStatus(t, b.do(http.MethodPost, "/logout", "application/x-www-form-urlencoded", nil), http.StatusForbidden)
	if !valid() {
		t.Fatal("a cross-site POST signed the user out")
	}

	// The page's own Sign out button works.
	b.headers["Sec-Fetch-Site"] = "same-origin"
	rec := b.do(http.MethodPost, "/logout", "application/x-www-form-urlencoded", nil)
	wantStatus(t, rec, http.StatusSeeOther)
	if rec.Header().Get("Location") != "/login" {
		t.Errorf("logout redirects to %q", rec.Header().Get("Location"))
	}
	if valid() || b.signedIn() {
		t.Fatal("still signed in after POST /logout")
	}
}
