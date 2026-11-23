// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package setup

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/bhplta"
)

// Between seasons set-up creates no season. The done page then says what the
// club can do meanwhile and how to bring the season in later; with a season
// it says neither.
func TestDonePageBetweenSeasons(t *testing.T) {
	s, db := newTestService(t)
	h := NewHandler(s, auth.NewService(db, auth.DefaultConfig()), findRepoDir(t, "templates"))

	for _, tc := range []struct {
		name string
		sum  *bhplta.Summary
		want bool
	}{
		{"off season", &bhplta.Summary{LeagueName: "Parks League", NoFixtures: true, ClubsCreated: 12,
			Notes: []string{"BHPLTA has not published any upcoming fixtures yet, so no season was created."}}, true},
		{"season created", &bhplta.Summary{LeagueName: "Parks League", SeasonID: 1, SeasonName: "2027 Season",
			Divisions: 4, Teams: 41, Weeks: 18, Fixtures: 228, ClubsCreated: 12}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.render(rec, http.StatusOK, "done", pageData{Status: StatusReady.String(), DashboardURL: dashboardPath, Import: tc.sum})
			page := rec.Body.String()
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d", rec.Code)
			}
			for _, s := range []string{"data-setup-off-season", "add your players", "Start a new season from BHPLTA"} {
				if got := strings.Contains(page, s); got != tc.want {
					t.Errorf("page contains %q = %v, want %v", s, got, tc.want)
				}
			}
			if got := strings.Contains(page, `data-count="fixtures"`); got == tc.want {
				t.Errorf("fixtures count shown = %v", got)
			}
		})
	}
}
