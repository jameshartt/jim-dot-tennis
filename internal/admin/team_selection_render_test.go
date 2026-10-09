// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/models"
)

// The HTMX response that replaces .team-selection-container after a change
// must be exactly the container the full page renders for the same state.
// Two hand-maintained copies drifted before: the swapped-in copy dropped the
// derby "Managing:" banner and carried its own stylesheet.
func TestTeamSelectionSwapMatchesFullPage(t *testing.T) {
	db := seedTeamSelectionFixture(t)
	h := NewFixturesHandler(NewService(db, "", 1, ""), findTemplatesPathAdmin(t), nil)
	pagePath := fmt.Sprintf("/admin/league/fixtures/%d/team-selection", selectionFixtureID)

	serve := func(r *http.Request) *httptest.ResponseRecorder {
		t.Helper()
		ctx := context.WithValue(r.Context(), auth.UserContextKey, models.User{Username: "captain", Role: "admin"})
		rec := httptest.NewRecorder()
		h.handleTeamSelection(rec, r.WithContext(ctx))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s: status %d\n%s", r.Method, r.URL, rec.Code, rec.Body.String())
		}
		return rec
	}

	form := url.Values{"action": {"add_player"}, "player_id": {"p-day"}, "is_home": {"true"}, "managing_team_id": {"2"}}
	post := httptest.NewRequest(http.MethodPost, pagePath, strings.NewReader(form.Encode()))
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	post.Header.Set("HX-Request", "true")
	swap := strings.TrimSpace(serve(post).Body.String())

	page := serve(httptest.NewRequest(http.MethodGet, pagePath+"?managingTeam=2", nil)).Body.String()

	if !strings.HasPrefix(swap, `<div class="team-selection-container">`) {
		t.Errorf("swap response should be just the container element, starts with:\n%.200s", swap)
	}
	if !strings.Contains(swap, "Managing: St Ann&#39;s B") {
		t.Errorf("swap response lost the derby managing-team banner")
	}
	if !strings.Contains(swap, "Weekday Wins") || !strings.Contains(page, "Weekday Wins") {
		t.Errorf("added player missing from the rendered selection")
	}
	if !strings.Contains(page, swap) {
		t.Errorf("swap response differs from the full page's container")
	}
}
