// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/config"
	"jim-dot-tennis/internal/courthive"
	"jim-dot-tennis/internal/courthive/courthivetest"
	"jim-dot-tennis/internal/models"
)

func postTournaments(t *testing.T, e settingsTestEnv, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	h := NewTournamentsHandler(e.handler.service, findTemplatesPathAdmin(t))
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx := config.WithHomeClub(req.Context(), e.holder.ID(), e.holder.Club(), e.holder.LogoPath())
	ctx = context.WithValue(ctx, auth.UserContextKey, models.User{ID: 1, Username: "someone", Role: models.RoleAdmin})
	rec := httptest.NewRecorder()
	if path == tournamentAddSetPath {
		h.HandleAddSet(rec, req.WithContext(ctx))
	} else {
		h.HandleRenameSet(rec, req.WithContext(ctx))
	}
	return rec
}

// An administrator adds a second set of tournaments from the tournaments
// page (taking on one made by hand), renames it, and cannot delete it or
// change its code; the page words it for people, not as "providers".
func TestAddingAndRenamingASetOfTournaments(t *testing.T) {
	e := newSettingsTestEnv(t)
	fake := courthivetest.New(t)
	withTournamentStack(t, e, fake.URL, courthivetest.Password)
	ctx := context.Background()

	// Before the club is set up: refused with what to do.
	rec := postTournaments(t, e, tournamentAddSetPath, url.Values{"code": {"CUP"}, "name": {"Parks Cup"}})
	if loc := rec.Header().Get("Location"); rec.Code != http.StatusSeeOther || !strings.Contains(loc, "setup=failed") || !strings.Contains(loc, "Set+the+club+up+first") {
		t.Fatalf("before set-up: %d %q", rec.Code, loc)
	}
	if rec := postSetup(t, e, url.Values{"abbreviation": {"ALPHA"}}); !strings.Contains(rec.Header().Get("Location"), "setup=done") {
		t.Fatalf("set-up: %q", rec.Header().Get("Location"))
	}

	old := fake.AddProvider("CUP", "Made by hand", false)
	fake.AddAccount(old, "jim@example.org", courthive.RoleProviderAdmin, "", "")
	body := tournamentsPage(t, e, "")
	for _, want := range []string{"data-setup-sets", "Add another set of tournaments", `data-found-code="CUP"`, "what people see when they choose"} {
		if !strings.Contains(body, want) {
			t.Errorf("the tournaments page lacks %q", want)
		}
	}

	rec = postTournaments(t, e, tournamentAddSetPath, url.Values{"code": {"cup"}, "name": {"Parks Cup"}})
	if loc := rec.Header().Get("Location"); rec.Code != http.StatusSeeOther || !strings.Contains(loc, "setup=done") {
		t.Fatalf("add: %d %q", rec.Code, loc)
	}
	sets, err := e.handler.service.tournamentSetup.ManagedProviders(ctx)
	if err != nil || len(sets) != 2 || sets[1].Abbr != "CUP" || sets[1].Name != "Parks Cup" || sets[1].CourthiveID != old {
		t.Fatalf("sets = %+v, %v", sets, err)
	}
	body = tournamentsPage(t, e, "?setup=done")
	for _, want := range []string{"Parks Cup is ready", `data-setup-set="CUP"`, `data-setup-set="ALPHA"`, "jim@example.org"} {
		if !strings.Contains(body, want) {
			t.Errorf("after adding, the page lacks %q", want)
		}
	}
	if strings.Contains(body, `data-found-code="CUP"`) {
		t.Error("the set taken on is still listed as not taken on")
	}

	// Rename: the name people see changes, the code does not.
	rec = postTournaments(t, e, tournamentRenameSetPath, url.Values{"id": {itoa(int64(sets[1].RowID))}, "name": {"  Parks League Cup "}})
	if rec.Code != http.StatusSeeOther || strings.Contains(rec.Header().Get("Location"), "error=") {
		t.Fatalf("rename: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	sets, _ = e.handler.service.tournamentSetup.ManagedProviders(ctx)
	if sets[1].Name != "Parks League Cup" || sets[1].Abbr != "CUP" {
		t.Fatalf("after rename: %+v", sets[1])
	}
	if rec := postTournaments(t, e, tournamentRenameSetPath, url.Values{"id": {itoa(int64(sets[1].RowID))}, "name": {" "}}); !strings.Contains(rec.Header().Get("Location"), "error=") {
		t.Error("an empty name was accepted")
	}

	// A managed set cannot be deleted, nor its code changed, on the providers page.
	if err := e.handler.service.DeleteTournamentProvider(sets[1].RowID); err == nil || !strings.Contains(err.Error(), "cannot be deleted") {
		t.Errorf("delete: %v", err)
	}
	row, _ := e.handler.service.GetTournamentProviderByID(sets[1].RowID)
	row.ProviderAbbr = "OTHER"
	if err := e.handler.service.UpdateTournamentProvider(row); err == nil {
		t.Error("the code of a managed set was changed")
	}
}

// With more than one set of tournaments the users page offers them by name
// for each person with tournament access, and stores the choice.
func TestUsersPageChoosesSetsOfTournaments(t *testing.T) {
	e := newUsersTestEnv(t, "")
	ctx := context.Background()
	db := e.env.db
	mustExec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatal(err)
		}
	}
	// One set: nothing to choose.
	mustExec(`INSERT INTO tournament_providers (id, name, provider_abbr, courthive_provider_id) VALUES (1, 'St Ann''s club tournaments', 'STANNS', 'uuid-own')`)
	mustExec(`INSERT INTO app_settings (key, value) VALUES ('tournament_provider_id', 'uuid-own'), ('tournament_provider_abbr', 'STANNS')`)
	issuedLink(t, e.do(http.MethodPost, "/admin/league/users", url.Values{"username": {"tina"}, "role": {"tournaments"}, "tournament_access": {"own"}}))
	page := e.do(http.MethodGet, "/admin/league/users", nil).Body.String()
	if strings.Contains(page, "Can manage tournaments for") {
		t.Fatal("with one set of tournaments the users page offers a choice")
	}

	// Two: offered by name; tina (nothing stored) has the club's own ticked.
	mustExec(`INSERT INTO tournament_providers (id, name, provider_abbr, courthive_provider_id) VALUES (2, 'Parks Cup', 'BHPLTA', 'uuid-cup')`)
	page = e.do(http.MethodGet, "/admin/league/users", nil).Body.String()
	row := page[strings.Index(page, `data-username="tina"`):]
	row = row[:strings.Index(row, "</tr>")]
	for _, want := range []string{"Can manage tournaments for", "St Ann&#39;s club tournaments", "Parks Cup",
		`value="1" data-set-code="STANNS" checked`, `value="2" data-set-code="BHPLTA" >`} {
		if !strings.Contains(row, want) {
			t.Errorf("tina's row lacks %q:\n%s", want, row)
		}
	}
	if strings.Contains(strings.ToLower(row), "provider") {
		t.Errorf("tina's row says \"provider\":\n%s", row)
	}
	if !strings.Contains(page, "in every set of tournaments the club runs") {
		t.Error("the administrator's row does not say they have every set")
	}

	tina, _ := e.svc.Find(ctx, "tina")
	post := func(id int64, sets ...string) string {
		t.Helper()
		rec := e.do(http.MethodPost, "/admin/league/users/"+itoa(id)+"/tournament-sets", url.Values{"tournament_set": sets})
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("choose sets: %d", rec.Code)
		}
		return rec.Header().Get("Location")
	}
	if loc := post(tina.ID, "2"); strings.Contains(loc, "error=") {
		t.Fatalf("choose the cup: %q", loc)
	}
	var picked []uint
	if err := db.Select(&picked, `SELECT tournament_provider_id FROM user_tournament_providers WHERE user_id = ?`, tina.ID); err != nil || len(picked) != 1 || picked[0] != 2 {
		t.Fatalf("stored = %v, %v", picked, err)
	}
	page = e.do(http.MethodGet, "/admin/league/users", nil).Body.String()
	row = page[strings.Index(page, `data-username="tina"`):]
	row = row[:strings.Index(row, "</tr>")]
	if !strings.Contains(row, `value="2" data-set-code="BHPLTA" checked`) || strings.Contains(row, `data-set-code="STANNS" checked`) {
		t.Errorf("tina's row after choosing the cup:\n%s", row)
	}
	// None ticked, a set that does not exist, an administrator: refused with a reason.
	if loc := post(tina.ID); !strings.Contains(loc, "error=") || !strings.Contains(loc, "at+least+one") {
		t.Errorf("none ticked: %q", loc)
	}
	if loc := post(tina.ID, "77"); !strings.Contains(loc, "error=") {
		t.Errorf("unknown set: %q", loc)
	}
	if loc := post(e.admin.ID, "2"); !strings.Contains(loc, "error=") || !strings.Contains(loc, "Administrators") {
		t.Errorf("administrator: %q", loc)
	}
	// Chosen at creation.
	issuedLink(t, e.do(http.MethodPost, "/admin/league/users", url.Values{"username": {"cup.only"}, "role": {"tournaments"}, "tournament_access": {"all"}, "tournament_set": {"2"}}))
	cupOnly, _ := e.svc.Find(ctx, "cup.only")
	if err := db.Select(&picked, `SELECT tournament_provider_id FROM user_tournament_providers WHERE user_id = ?`, cupOnly.ID); err != nil || len(picked) != 1 || picked[0] != 2 {
		t.Fatalf("stored at creation = %v, %v", picked, err)
	}
}
