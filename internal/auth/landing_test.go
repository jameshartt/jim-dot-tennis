// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"jim-dot-tennis/internal/models"
)

func TestHomePathFor(t *testing.T) {
	const league = "/admin/league"
	for _, tc := range []struct {
		name   string
		user   models.User
		expect string
	}{
		{"administrator", models.User{Role: models.RoleAdmin, IsActive: true}, league},
		{"captain without tournaments", models.User{Role: models.RoleCaptain, IsActive: true}, league},
		{"captain with tournaments", models.User{Role: models.RoleCaptain, IsActive: true, TournamentAccess: models.TournamentAccessAll}, league},
		{"tournaments only, own", models.User{Role: models.RoleTournamentsOnly, IsActive: true, TournamentAccess: models.TournamentAccessOwn}, TournamentHomePath},
		{"tournaments only, all", models.User{Role: models.RoleTournamentsOnly, IsActive: true, TournamentAccess: models.TournamentAccessAll}, TournamentHomePath},
		{"old player account, no access", models.User{Role: models.RoleTournamentsOnly, IsActive: true, TournamentAccess: models.TournamentAccessNone}, league},
		{"disabled tournaments only", models.User{Role: models.RoleTournamentsOnly, TournamentAccess: models.TournamentAccessAll}, league},
	} {
		if got := HomePathFor(tc.user, league); got != tc.expect {
			t.Errorf("%s: HomePathFor = %q, want %q", tc.name, got, tc.expect)
		}
	}
}

// A Tournaments only account lands on the tournament home page after a
// password or passkey sign-in (unless it asked for a page); captains and
// administrators land on the league dashboard as before.
func TestSignInLandsTournamentsOnlyAccountsOnTheirHome(t *testing.T) {
	env := newPasskeyEnv(t)
	tid := env.addUser("tina", models.RoleTournamentsOnly)
	if _, err := env.svc.db.Exec(`UPDATE users SET tournament_access = 'own' WHERE id = ?`, tid); err != nil {
		t.Fatal(err)
	}
	env.addUser("cap", models.RoleCaptain)

	// Password sign-in: no passkey yet, so the passkey offer comes first,
	// carrying on to the right home.
	login := func(username, redirect string) string {
		t.Helper()
		form := url.Values{"username": {username}, "password": {testPassword}}
		if redirect != "" {
			form.Set("redirect", redirect)
		}
		rec := env.browser("198.51.100.7").postForm("/login", form)
		wantStatus(t, rec, http.StatusSeeOther)
		return rec.Header().Get("Location")
	}
	if loc := login("tina", ""); !strings.Contains(loc, url.QueryEscape(TournamentHomePath)) {
		t.Errorf("tina lands on %q, want the tournament home", loc)
	}
	if loc := login("cap", ""); !strings.Contains(loc, url.QueryEscape("/admin/league")) || strings.Contains(loc, "tournament") {
		t.Errorf("the captain lands on %q, want the league", loc)
	}
	if loc := login("tina", "/account/signin-methods"); !strings.Contains(loc, url.QueryEscape("/account/signin-methods")) {
		t.Errorf("an asked-for page is kept: %q", loc)
	}

	// Already signed in: the login page sends tina home too.
	b := env.browser("198.51.100.8")
	b.passwordSignIn("tina")
	rec := b.do(http.MethodGet, "/login", "", nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != TournamentHomePath {
		t.Errorf("GET /login when signed in: %d %q", rec.Code, rec.Header().Get("Location"))
	}

	// Passkey sign-in.
	a := newAuthenticator(t)
	cred := b.enrol(a, "tina's phone", true)
	fresh := env.browser("198.51.100.9")
	finish := fresh.passkeySignIn(a, cred)
	wantStatus(t, finish, http.StatusOK)
	var out map[string]string
	if err := json.Unmarshal(finish.Body.Bytes(), &out); err != nil || out["redirect"] != TournamentHomePath {
		t.Errorf("passkey sign-in redirect: %v %q", err, finish.Body.String())
	}
}

// The 403 page offers a Tournaments only account its tournament home.
func TestForbiddenPageOffersTheTournamentHome(t *testing.T) {
	render := func(user models.User) string {
		ctx := context.WithValue(context.Background(), UserContextKey, user)
		ctx = context.WithValue(ctx, RoleContextKey, user.Role)
		rec := httptest.NewRecorder()
		WriteForbidden(rec, httptest.NewRequest(http.MethodGet, "/admin/league/dashboard", nil).WithContext(ctx))
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status %d", rec.Code)
		}
		return rec.Body.String()
	}
	page := render(models.User{Role: models.RoleTournamentsOnly, IsActive: true, TournamentAccess: models.TournamentAccessOwn})
	if !strings.Contains(page, `href="`+TournamentHomePath+`"`) || !strings.Contains(page, "Your account is for tournaments") {
		t.Errorf("tournaments only 403 page:\n%s", page)
	}
	page = render(models.User{Role: models.RoleTournamentsOnly, IsActive: true})
	if strings.Contains(page, TournamentHomePath) {
		t.Error("an account with no tournament access is offered the tournament home")
	}
	page = render(models.User{Role: models.RoleCaptain, IsActive: true, TournamentAccess: models.TournamentAccessAll})
	if !strings.Contains(page, "forbidden-dashboard-link") || strings.Contains(page, "Your account is for tournaments") {
		t.Errorf("captain 403 page changed:\n%s", page)
	}
}
