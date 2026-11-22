// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package tournamenthome

import (
	"context"
	"errors"
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
	return servePath(t, sw, nil, user, auth.TournamentHomePath)
}

// fakeOpener stands in for the tournament service.
type fakeOpener struct {
	configured bool
	token      string
	err        error
	asked      []int64
}

func (f *fakeOpener) Configured() bool { return f.configured }
func (f *fakeOpener) Handover(_ context.Context, userID int64) (string, error) {
	f.asked = append(f.asked, userID)
	return f.token, f.err
}

func servePath(t *testing.T, sw *fakeSwitch, opener Opener, user *models.User, path string) *httptest.ResponseRecorder {
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
	New(sw, opener, templatesRoot(t)).RegisterRoutes(mux, requireAuth)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestOpenHandsOverOnlyWithAccess(t *testing.T) {
	on, off := &fakeSwitch{on: true}, &fakeSwitch{on: false}
	tina := models.User{ID: 41, Username: "tina", Role: models.RoleTournamentsOnly, IsActive: true, TournamentAccess: models.TournamentAccessOwn}
	cap := models.User{ID: 42, Username: "cap", Role: models.RoleCaptain, IsActive: true}
	gone := models.User{ID: 43, Username: "gone", Role: models.RoleAdmin, IsActive: false}

	// Not signed in: to the sign-in page, and nothing is asked of the tournament server.
	op := &fakeOpener{configured: true, token: "one-time"}
	if rec := servePath(t, on, op, nil, ManagementPath); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
		t.Errorf("anonymous: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	// No access, a disabled account, or Tournaments switched off: back to the home page.
	for name, c := range map[string]struct {
		sw *fakeSwitch
		u  models.User
	}{"captain without access": {on, cap}, "disabled administrator": {on, gone}, "switched off": {off, tina}} {
		rec := servePath(t, c.sw, op, &c.u, ManagementPath)
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != auth.TournamentHomePath {
			t.Errorf("%s: %d %q", name, rec.Code, rec.Header().Get("Location"))
		}
	}
	if len(op.asked) != 0 {
		t.Fatalf("a hand-over was requested for someone without access: %v", op.asked)
	}

	// With access: the token goes in the fragment, and the answer must not be stored or leak a referrer.
	rec := servePath(t, on, op, &tina, ManagementPath)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/tournaments/#/sso?token=one-time" {
		t.Fatalf("tina: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if rec.Header().Get("Cache-Control") != "no-store" || rec.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Error("the hand-over answer may be cached or leak a referrer")
	}
	if len(op.asked) != 1 || op.asked[0] != 41 {
		t.Errorf("asked = %v", op.asked)
	}
}

func TestOpenWithoutBundledStackGoesStraightToTournamentManagement(t *testing.T) {
	tina := models.User{ID: 41, Username: "tina", Role: models.RoleTournamentsOnly, IsActive: true, TournamentAccess: models.TournamentAccessAll}
	for name, op := range map[string]Opener{"none": nil, "not configured": &fakeOpener{}} {
		rec := servePath(t, &fakeSwitch{on: true}, op, &tina, ManagementPath)
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/tournaments/" {
			t.Errorf("%s: %d %q", name, rec.Code, rec.Header().Get("Location"))
		}
	}
}

func TestOpenFailureSaysSoWithoutDetails(t *testing.T) {
	tina := models.User{ID: 41, Username: "tina", Role: models.RoleTournamentsOnly, IsActive: true, TournamentAccess: models.TournamentAccessAll}
	op := &fakeOpener{configured: true, err: errors.New("courthive: secret-looking-detail HTTP 500")}
	rec := servePath(t, &fakeSwitch{on: true}, op, &tina, ManagementPath)
	body := rec.Body.String()
	if rec.Code != http.StatusBadGateway || !strings.Contains(body, "tournament-open-failed") || !strings.Contains(body, "Try again") {
		t.Fatalf("%d\n%s", rec.Code, body)
	}
	if strings.Contains(body, "secret-looking-detail") {
		t.Error("the page shows the internal error")
	}
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
