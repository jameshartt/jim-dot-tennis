// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package tournamenthome

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/models"
)

type fakeSwitch struct{ on bool }

func (f *fakeSwitch) TournamentsEnabled() bool { return f.on }

// templatesRoot finds the repository's templates directory.
func templatesRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		if info, err := os.Stat(filepath.Join(dir, "templates")); err == nil && info.IsDir() {
			return filepath.Join(dir, "templates")
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("could not find templates")
	return ""
}

// serve requests the page as user (nil: not signed in) through a stand-in
// for RequireAuth that sends anonymous requests to /login.
func serve(t *testing.T, sw *fakeSwitch, user *models.User) *httptest.ResponseRecorder {
	t.Helper()
	t.Setenv("TEMPLATE_RELOAD", "")
	mux := http.NewServeMux()
	requireAuth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if user == nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), auth.UserContextKey, *user)))
		})
	}
	New(sw, templatesRoot(t)).RegisterRoutes(mux, requireAuth)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, auth.TournamentHomePath, nil))
	return rec
}

func TestTournamentHomeGuard(t *testing.T) {
	on := &fakeSwitch{on: true}
	if rec := serve(t, on, nil); rec.Code != http.StatusSeeOther {
		t.Errorf("anonymous: %d", rec.Code)
	}
	for name, u := range map[string]models.User{
		"captain without access":        {Username: "cap", Role: models.RoleCaptain, IsActive: true},
		"old player account, no access": {Username: "old", Role: models.RoleTournamentsOnly, IsActive: true},
	} {
		rec := serve(t, on, &u)
		if rec.Code != http.StatusForbidden || strings.Contains(rec.Body.String(), "tournament-management-link") {
			t.Errorf("%s: %d\n%s", name, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "tournament-home-refused") {
			t.Errorf("%s: no explanation", name)
		}
	}
}

func TestTournamentHomeLinksToManagement(t *testing.T) {
	on := &fakeSwitch{on: true}
	tina := models.User{Username: "tina", Role: models.RoleTournamentsOnly, IsActive: true, TournamentAccess: models.TournamentAccessOwn}
	rec := serve(t, on, &tina)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `href="`+ManagementPath+`"`) || !strings.Contains(body, "Open tournament management") {
		t.Fatalf("tina: %d\n%s", rec.Code, body)
	}
	if strings.Contains(body, "tournament-home-league-link") {
		t.Error("a Tournaments only account is offered the league dashboard")
	}
	if !strings.Contains(body, "your own tournaments") || !strings.Contains(body, `data-testid="logout-button"`) {
		t.Error("no access wording or sign-out button")
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("the page may be cached")
	}

	// Administrators (always all) and captains with access get a way back
	// to the league.
	for _, u := range []models.User{
		{Username: "ada", Role: models.RoleAdmin, IsActive: true},
		{Username: "cap", Role: models.RoleCaptain, IsActive: true, TournamentAccess: models.TournamentAccessAll},
	} {
		rec := serve(t, on, &u)
		body := rec.Body.String()
		if rec.Code != http.StatusOK || !strings.Contains(body, "tournament-home-league-link") || !strings.Contains(body, "every tournament") {
			t.Errorf("%s: %d\n%s", u.Username, rec.Code, body)
		}
	}

	// Switched off: said plainly, no button.
	rec = serve(t, &fakeSwitch{}, &tina)
	body = rec.Body.String()
	if rec.Code != http.StatusOK || strings.Contains(body, "tournament-management-link") || !strings.Contains(body, "Tournaments are not switched on for this club") {
		t.Errorf("switched off: %d\n%s", rec.Code, body)
	}
}
