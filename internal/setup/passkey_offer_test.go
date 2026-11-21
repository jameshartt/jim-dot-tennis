// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package setup

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"jim-dot-tennis/internal/auth"
)

// The gate lets the first administrator reach passkey sign-in and their own
// sign-in methods page before a club is chosen, and nothing else new.
func TestGateAllowsPasskeyPagesOnlyOnceAnAdministratorExists(t *testing.T) {
	s, db := newTestState(t)
	reached := false
	gated := s.Gate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true }))
	paths := []string{"/login/passkey/begin", "/login/passkey/finish", auth.SignInMethodsPath,
		auth.PasskeysPathPrefix + "begin", auth.PasskeysPathPrefix + "3/delete"}

	// Needs setup: no administrator yet, so none of them are reachable.
	for _, path := range paths {
		reached = false
		gated.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, path, nil))
		if reached {
			t.Errorf("needs-setup %s reached the app", path)
		}
	}

	addUser(t, db, "first", "admin")
	for _, path := range paths {
		reached = false
		gated.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, path, nil))
		if !reached {
			t.Errorf("needs-club %s did not reach the app", path)
		}
	}
	for _, path := range []string{"/account/password", "/account/", "/account/signin-methods/x", "/loginx", "/admin/league/users"} {
		reached = false
		gated.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
		if reached {
			t.Errorf("needs-club %s reached the app", path)
		}
	}
}

// The "create your login" step offers a passkey only where passkeys work,
// keeps the password form as it was, and only goes on to passkey enrolment
// when asked to.
func TestAdminStepOffersPasskeyEnrolment(t *testing.T) {
	for _, tc := range []struct {
		name      string
		available bool
		then      string
		wantLoc   string
	}{
		{"offered and chosen", true, "passkey", auth.SignInMethodsPath + "?enrol=1&next=" + url.QueryEscape(ClubPath)},
		{"offered, not chosen", true, "", ClubPath},
		{"not available, field forged", false, "passkey", ClubPath},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, db := newTestService(t)
			code, err := s.IssueCode(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			cfg := auth.DefaultConfig()
			cfg.CookieSecure = false
			authSvc := auth.NewService(db, cfg)
			h := NewHandler(s, authSvc, findRepoDir(t, "templates"))
			h.OfferPasskeys(func(*http.Request) bool { return tc.available })
			mux := http.NewServeMux()
			h.RegisterRoutes(mux)
			srv := httptest.NewServer(s.state.Gate(mux))
			defer srv.Close()
			jar, _ := cookiejar.New(nil)
			client := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
			f := &flowClient{t: t, srv: srv, client: client}

			if st, _, _ := f.do("POST", Path, url.Values{"code": {code}}); st != http.StatusSeeOther {
				t.Fatalf("redeem code: %d", st)
			}
			st, _, page := f.do("GET", AdminPath, nil)
			if st != http.StatusOK {
				t.Fatalf("admin page: %d", st)
			}
			if got := strings.Contains(page, "data-passkey-setup"); got != tc.available {
				t.Errorf("passkey offer shown = %v, want %v", got, tc.available)
			}
			if got := strings.Contains(page, "/static/passkey.js"); got != tc.available {
				t.Errorf("passkey script loaded = %v, want %v", got, tc.available)
			}
			// The password form is unchanged: one submit button, same fields.
			if n := strings.Count(page, `type="submit"`); n != 1 {
				t.Errorf("%d submit buttons, want 1", n)
			}
			for _, field := range []string{`name="username"`, `name="password"`, `name="password_confirm"`} {
				if !strings.Contains(page, field) {
					t.Errorf("admin page lacks %s", field)
				}
			}

			form := url.Values{"username": {"first.admin"}, "password": {"a long enough passphrase"}, "password_confirm": {"a long enough passphrase"}}
			if tc.then != "" {
				form.Set("then", tc.then)
			}
			st, loc, _ := f.do("POST", AdminPath, form)
			if st != http.StatusSeeOther || loc != tc.wantLoc {
				t.Fatalf("create administrator: %d, Location %q, want %q", st, loc, tc.wantLoc)
			}
			if userCount(t, db) != 1 {
				t.Fatalf("users = %d", userCount(t, db))
			}
			// The step is gone afterwards, as before.
			if st, _, _ := f.do("GET", AdminPath, nil); st != http.StatusNotFound {
				t.Errorf("admin page after creation: %d, want 404", st)
			}
		})
	}
}
