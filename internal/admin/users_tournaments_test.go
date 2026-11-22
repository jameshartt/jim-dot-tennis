// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"jim-dot-tennis/internal/accounts"
	"jim-dot-tennis/internal/models"
)

// TestUsersPageTournamentAccess: an administrator creates a Tournaments only
// account (own or all) and changes any non-administrator's tournament access
// from the users page; the refusals come back as messages.
func TestUsersPageTournamentAccess(t *testing.T) {
	e := newUsersTestEnv(t, "")
	ctx := context.Background()

	page := e.do(http.MethodGet, "/admin/league/users", nil).Body.String()
	for _, want := range []string{`<option value="tournaments">Tournaments only</option>`, `name="tournament_access"`, "data-tournaments-help", "All tournaments (administrator)"} {
		if !strings.Contains(page, want) {
			t.Errorf("users page lacks %q", want)
		}
	}

	// Create a Tournaments only account with access to its own tournaments.
	rec := e.do(http.MethodPost, "/admin/league/users", url.Values{"username": {"tina"}, "role": {"tournaments"}, "tournament_access": {"own"}})
	issuedLink(t, rec)
	tina, err := e.svc.Find(ctx, "tina")
	if err != nil || tina.Role != models.RoleTournamentsOnly || tina.TournamentAccess != models.TournamentAccessOwn {
		t.Fatalf("tina: %+v %v", tina, err)
	}
	if !strings.Contains(rec.Body.String(), `<option value="" selected disabled>Tournaments only</option>`) {
		t.Error("tina is not shown as Tournaments only")
	}
	// Refused without access: nothing created, the reason shown.
	rec = e.do(http.MethodPost, "/admin/league/users", url.Values{"username": {"tom"}, "role": {"tournaments"}, "tournament_access": {"none"}})
	if loc := rec.Header().Get("Location"); rec.Code != http.StatusSeeOther || !strings.Contains(loc, "error=") || !strings.Contains(loc, "Tournaments+only") {
		t.Errorf("tournaments only with none: %d %q", rec.Code, loc)
	}
	if _, err := e.svc.Find(ctx, "tom"); err == nil {
		t.Error("tom was created")
	}
	// An administrator's access choice is ignored: they always have all.
	issuedLink(t, e.do(http.MethodPost, "/admin/league/users", url.Values{"username": {"ada"}, "role": {"admin"}, "tournament_access": {"own"}}))
	if ada, _ := e.svc.Find(ctx, "ada"); ada.TournamentAccess != models.TournamentAccessNone || ada.EffectiveTournamentAccess() != models.TournamentAccessAll {
		t.Errorf("ada: %+v", ada)
	}
	// A captain with access.
	issuedLink(t, e.do(http.MethodPost, "/admin/league/users", url.Values{"username": {"cap"}, "role": {"captain"}, "tournament_access": {"all"}}))
	capt, _ := e.svc.Find(ctx, "cap")
	if capt.TournamentAccess != models.TournamentAccessAll {
		t.Errorf("cap: %+v", capt)
	}

	set := func(id int64, value string) string {
		t.Helper()
		rec := e.do(http.MethodPost, "/admin/league/users/"+itoa(id)+"/tournaments", url.Values{"tournament_access": {value}})
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("set %d to %q: %d", id, value, rec.Code)
		}
		return rec.Header().Get("Location")
	}
	if loc := set(capt.ID, "none"); strings.Contains(loc, "error=") {
		t.Errorf("captain -> none: %q", loc)
	}
	if loc := set(tina.ID, "all"); strings.Contains(loc, "error=") {
		t.Errorf("tina -> all: %q", loc)
	}
	if loc := set(tina.ID, "none"); !strings.Contains(loc, "error=") || !strings.Contains(loc, "disable") {
		t.Errorf("tina -> none: %q", loc)
	}
	if loc := set(e.admin.ID, "own"); !strings.Contains(loc, "error=") || !strings.Contains(loc, "Administrators") {
		t.Errorf("admin -> own: %q", loc)
	}
	if loc := set(capt.ID, "bogus"); !strings.Contains(loc, "error=") {
		t.Errorf("bogus value: %q", loc)
	}
	if a, _ := e.svc.Get(ctx, tina.ID); a.TournamentAccess != models.TournamentAccessAll {
		t.Errorf("tina after changes: %q", a.TournamentAccess)
	}
	if a, _ := e.svc.Get(ctx, capt.ID); a.TournamentAccess != models.TournamentAccessNone {
		t.Errorf("cap after changes: %q", a.TournamentAccess)
	}

	// Recorded with the administrator as the actor.
	var rows []string
	if err := e.env.db.Select(&rows, `SELECT actor || ' ' || target_username || ' ' || detail FROM account_audit_log WHERE action = ? ORDER BY id`, accounts.ActionTournamentAccess); err != nil {
		t.Fatal(err)
	}
	if strings.Join(rows, "|") != "boss cap tournament_access all -> none|boss tina tournament_access own -> all" {
		t.Errorf("audit rows: %q", rows)
	}

	// The page: a Tournaments only row offers no "No tournaments".
	page = e.do(http.MethodGet, "/admin/league/users", nil).Body.String()
	row := page[strings.Index(page, `data-username="tina"`):]
	row = row[:strings.Index(row, "</tr>")]
	if strings.Contains(row, `value="none"`) || !strings.Contains(row, `<option value="all" selected>`) {
		t.Errorf("tina's row:\n%s", row)
	}
}
