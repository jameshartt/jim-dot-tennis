// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package setup

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"jim-dot-tennis/internal/config"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/repository"
)

// newEnvClubService returns a setup service whose home club comes from
// HOME_CLUB_NAME (a club inserted directly), with no user accounts.
func newEnvClubService(t *testing.T) (*Service, *database.DB) {
	t.Helper()
	_, db := newTestState(t)
	if _, err := db.Exec(`INSERT INTO clubs (id, name) VALUES (4, 'Env Club')`); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME_CLUB_NAME", "Env Club")
	holder, err := config.NewHomeClubHolder(context.Background(), repository.NewClubRepository(db), repository.NewSettingsRepository(db))
	if err != nil {
		t.Fatal(err)
	}
	if holder.ID() != 4 {
		t.Fatalf("home club %d, want 4 from HOME_CLUB_NAME", holder.ID())
	}
	return NewService(NewState(db, holder), nil), db
}

func addUser(t *testing.T, db *database.DB, username, role string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO users (username, password_hash, role) VALUES (?, 'x', ?)`, username, role); err != nil {
		t.Fatal(err)
	}
}

// Every combination of home club x administrator x other accounts.
func TestStatusForEveryState(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name  string
		club  bool
		admin bool
		other bool
		want  Status
	}{
		{"no club, no users", false, false, false, StatusNeedsSetup},
		{"no club, captain only", false, false, true, StatusNeedsSetup},
		{"no club, admin", false, true, false, StatusNeedsClub},
		{"no club, admin and captain", false, true, true, StatusNeedsClub},
		{"club, no users", true, false, false, StatusNeedsAdmin},
		{"club, captain only", true, false, true, StatusNeedsAdmin},
		{"club, admin", true, true, false, StatusReady},
		{"club, admin and captain", true, true, true, StatusReady},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var s *Service
			var db *database.DB
			if c.club {
				s, db = newEnvClubService(t)
			} else {
				s, db = newTestService(t)
			}
			if c.admin {
				addUser(t, db, "admin1", "admin")
			}
			if c.other {
				addUser(t, db, "captain1", "captain")
				addUser(t, db, "player1", "player")
			}
			if got := s.state.Status(ctx); got != c.want {
				t.Fatalf("status %v, want %v", got, c.want)
			}
			code, err := s.IssueCode(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if wantCode := c.want.NeedsFirstAdmin(); (code != "") != wantCode {
				t.Fatalf("IssueCode %q, want a code: %v", code, wantCode)
			}
		})
	}
}

// Once ready, the ready path no longer queries for administrators.
func TestReadyLatches(t *testing.T) {
	ctx := context.Background()
	s, db := newEnvClubService(t)
	addUser(t, db, "admin1", "admin")
	if s.state.Status(ctx) != StatusReady {
		t.Fatal("not ready")
	}
	if _, err := db.Exec(`DELETE FROM users`); err != nil {
		t.Fatal(err)
	}
	if s.state.Status(ctx) != StatusReady {
		t.Fatal("ready did not latch")
	}
}

// A home club from the environment but no administrator: the setup code
// creates the first administrator, who lands on the dashboard; there is no
// club step, and setup is gone afterwards.
func TestNeedsAdminFlowLandsOnDashboard(t *testing.T) {
	for _, withCaptain := range []bool{false, true} {
		t.Run(map[bool]string{false: "no users", true: "captain only"}[withCaptain], func(t *testing.T) {
			ctx := context.Background()
			s, db := newEnvClubService(t)
			if withCaptain {
				addUser(t, db, "captain1", "captain")
			}
			code, err := s.IssueCode(ctx)
			if err != nil || code == "" {
				t.Fatalf("IssueCode: %q %v", code, err)
			}
			browser := newFlow(t, s, db)

			// Everything, sign-in included, leads to the code page.
			for _, p := range []string{"/admin/league/dashboard", "/login"} {
				if st, loc, _ := browser.do("GET", p, nil); st != http.StatusSeeOther || loc != Path {
					t.Fatalf("gate %s: %d %q", p, st, loc)
				}
			}
			st, _, body := browser.do("GET", Path, nil)
			if st != http.StatusOK || !strings.Contains(body, `data-setup-status="needs-admin"`) || strings.Contains(body, "3. Your club") {
				t.Fatalf("code page: %d", st)
			}
			if st, loc, _ := browser.do("GET", ClubPath, nil); st != http.StatusSeeOther || loc != Path {
				t.Fatalf("club step: %d %q", st, loc)
			}
			// Without the code, administrator creation is not reachable.
			other := newFlow(t, s, db)
			if st, loc, _ := other.do("POST", AdminPath, url.Values{"username": {"intruder"}, "password": {goodPassword}, "password_confirm": {goodPassword}}); st != http.StatusSeeOther || loc != Path {
				t.Fatalf("create admin without code: %d %q", st, loc)
			}

			if st, loc, _ := browser.do("POST", Path, url.Values{"code": {code}}); st != http.StatusSeeOther || loc != AdminPath {
				t.Fatalf("right code: %d %q", st, loc)
			}
			if st, loc, _ := browser.do("POST", AdminPath, url.Values{"username": {"first.admin"}, "password": {goodPassword}, "password_confirm": {goodPassword}}); st != http.StatusSeeOther || loc != dashboardPath {
				t.Fatalf("create admin: %d %q, want dashboard", st, loc)
			}
			if st, _, body := browser.do("GET", dashboardPath, nil); st != http.StatusOK || body != "dashboard" {
				t.Fatalf("dashboard: %d %q", st, body)
			}
			if s.state.Status(ctx) != StatusReady {
				t.Fatalf("status %v, want ready", s.state.Status(ctx))
			}
			for _, f := range []*flowClient{browser, other} {
				for _, p := range []string{Path, AdminPath, ClubPath} {
					for _, m := range []string{"GET", "POST"} {
						if st, _, _ := f.do(m, p, url.Values{}); st != http.StatusNotFound {
							t.Errorf("%s %s after admin: %d, want 404", m, p, st)
						}
					}
				}
			}
			if c, _ := s.IssueCode(ctx); c != "" {
				t.Fatal("IssueCode issued a code with an administrator present")
			}
			// The club was in effect already: setup did not record a club choice.
			if _, ok, _ := repository.NewSettingsRepository(db).GetUint(ctx, repository.SettingHomeClubID); ok {
				t.Fatal("home_club_id stored by the administrator step")
			}
		})
	}
}
