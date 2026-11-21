// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"jim-dot-tennis/internal/models"
)

// TestUsersPageRolesAndPlayerLink: the users page offers only captain and
// admin, shows a leftover player-role account as having no access, and lets
// an administrator set, change and remove an existing account's linked
// player.
func TestUsersPageRolesAndPlayerLink(t *testing.T) {
	e := newUsersTestEnv(t, "")
	ctx := context.Background()
	if _, err := e.env.db.Exec(`INSERT INTO players (id, first_name, last_name, club_id) VALUES ('p1', 'Pat', 'Smith', 1), ('p2', 'Sam', 'Jones', 1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := e.env.db.Exec(`INSERT INTO users (username, role, is_active, created_at, last_login_at) VALUES ('oldplayer', 'player', TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}

	// Create a captain with no linked player.
	rec := e.do(http.MethodPost, "/admin/league/users", url.Values{"username": {"carol"}, "role": {"captain"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("create captain: %d %s", rec.Code, rec.Body.String())
	}
	carol, err := e.svc.Find(ctx, "carol")
	if err != nil || carol.Role != models.RoleCaptain || carol.PlayerID != nil {
		t.Fatalf("carol: %+v %v", carol, err)
	}
	// The player role can no longer be given.
	if rec := e.do(http.MethodPost, "/admin/league/users", url.Values{"username": {"pete"}, "role": {"player"}}); rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "error=") {
		t.Errorf("create as player: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if _, err := e.svc.Find(ctx, "pete"); err == nil {
		t.Error("a player-role account was created")
	}

	page := e.do(http.MethodGet, "/admin/league/users", nil).Body.String()
	if strings.Contains(page, `<option value="player"`) {
		t.Error("the users page still offers the player role")
	}
	if !strings.Contains(page, "player (no access)") {
		t.Error("the leftover player-role account is not shown as having no access")
	}
	if !strings.Contains(page, `action="/admin/league/users/`+itoa(carol.ID)+`/player"`) || !strings.Contains(page, "data-user-player-select") {
		t.Fatal("no linked-player control for carol")
	}

	link := func(id int64, playerID string) {
		t.Helper()
		rec := e.do(http.MethodPost, "/admin/league/users/"+itoa(id)+"/player", url.Values{"player_id": {playerID}})
		if rec.Code != http.StatusSeeOther || strings.Contains(rec.Header().Get("Location"), "error=") {
			t.Fatalf("link %d to %q: %d %q", id, playerID, rec.Code, rec.Header().Get("Location"))
		}
	}
	link(carol.ID, "p1")
	if a, _ := e.svc.Get(ctx, carol.ID); a.PlayerID == nil || *a.PlayerID != "p1" {
		t.Fatalf("carol not linked to p1: %+v", a)
	}
	page = e.do(http.MethodGet, "/admin/league/users", nil).Body.String()
	if !strings.Contains(page, `<option value="p1" selected>Pat Smith</option>`) {
		t.Error("carol's linked player is not shown selected")
	}
	// p1 is taken: another account cannot have it.
	rec = e.do(http.MethodPost, "/admin/league/users/"+itoa(e.admin.ID)+"/player", url.Values{"player_id": {"p1"}})
	if !strings.Contains(rec.Header().Get("Location"), "error=") {
		t.Errorf("linking a taken player: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	link(carol.ID, "p2")
	link(carol.ID, "")
	if a, _ := e.svc.Get(ctx, carol.ID); a.PlayerID != nil {
		t.Fatalf("carol still linked: %+v", a)
	}
}
