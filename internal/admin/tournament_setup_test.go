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
	"jim-dot-tennis/internal/repository"
	"jim-dot-tennis/internal/tournaments"
)

// withTournamentStack gives the settings env's service a tournament client
// pointed at a fake stack (password "" for a deployment without one).
func withTournamentStack(t *testing.T, e settingsTestEnv, url, password string) {
	t.Helper()
	c, err := courthive.New(courthive.Config{ServerURL: url, IdentityURL: url, ServicePassword: password})
	if err != nil {
		t.Fatal(err)
	}
	e.handler.service.tournamentSetup = tournaments.NewService(e.db, c)
	e.handler.service.courthiveAPIURL = url
}

// switchTournaments saves the settings form with Tournaments on or off and
// returns the redirect target.
func switchTournaments(t *testing.T, e settingsTestEnv, on bool, extra map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	fields := validSettingsFields()
	fields["feature_tournaments"] = "off"
	if on {
		fields["feature_tournaments"] = "on"
	}
	for k, v := range extra {
		fields[k] = v
	}
	return e.do(settingsForm(t, fields), models.RoleAdmin)
}

func getPage(e settingsTestEnv, target string) string {
	return e.do(httptest.NewRequest(http.MethodGet, target, nil), models.RoleAdmin).Body.String()
}

// tournamentsPage renders the tournaments page for an administrator.
func tournamentsPage(t *testing.T, e settingsTestEnv, query string) string {
	t.Helper()
	h := NewTournamentsHandler(e.handler.service, findTemplatesPathAdmin(t))
	req := httptest.NewRequest(http.MethodGet, "/admin/league/tournaments"+query, nil)
	ctx := config.WithHomeClub(req.Context(), e.holder.ID(), e.holder.Club(), e.holder.LogoPath())
	ctx = context.WithValue(ctx, auth.UserContextKey, models.User{ID: 1, Username: "someone", Role: models.RoleAdmin})
	rec := httptest.NewRecorder()
	h.HandleTournaments(rec, req.WithContext(ctx))
	if rec.Code != http.StatusOK {
		t.Fatalf("tournaments page: %d %s", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

func postSetup(t *testing.T, e settingsTestEnv, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	h := NewTournamentsHandler(e.handler.service, findTemplatesPathAdmin(t))
	req := httptest.NewRequest(http.MethodPost, tournamentSetupPath, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx := config.WithHomeClub(req.Context(), e.holder.ID(), e.holder.Club(), e.holder.LogoPath())
	ctx = context.WithValue(ctx, auth.UserContextKey, models.User{ID: 1, Username: "someone", Role: models.RoleAdmin})
	rec := httptest.NewRecorder()
	h.HandleSetup(rec, req.WithContext(ctx))
	return rec
}

func TestSwitchingTournamentsOnSetsUpTheClub(t *testing.T) {
	e := newSettingsTestEnv(t)
	fake := courthivetest.New(t)
	withTournamentStack(t, e, fake.URL, courthivetest.Password)

	if rec := switchTournaments(t, e, false, nil); rec.Code != http.StatusSeeOther || strings.Contains(rec.Header().Get("Location"), "tournament_setup") {
		t.Fatalf("switch off: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	// Off: the page offers the code, made from the club's name.
	body := getPage(e, settingsPath)
	if !strings.Contains(body, `name="tournament_abbreviation" value="ALPHA"`) {
		t.Fatal("the settings page does not offer the tournament code")
	}

	// A bad code: nothing saved, nothing set up.
	rec := switchTournaments(t, e, true, map[string]string{"tournament_abbreviation": "AL PHA"})
	if rec.Code != http.StatusBadRequest || e.holder.ClubSettings().TournamentsEnabled() || fake.Calls(courthivetest.CallVersion) != 0 {
		t.Fatalf("bad code: %d on=%v", rec.Code, e.holder.ClubSettings().TournamentsEnabled())
	}

	rec = switchTournaments(t, e, true, map[string]string{"tournament_abbreviation": "alphatc"})
	loc := rec.Header().Get("Location")
	if rec.Code != http.StatusSeeOther || !strings.Contains(loc, "tournament_setup=done") {
		t.Fatalf("switch on: %d %q", rec.Code, loc)
	}
	if p := fake.Providers(); len(p) != 1 || p[0].Abbreviation != "ALPHATC" || p[0].Name != "Alpha Tennis Club" {
		t.Fatalf("providers = %+v", p)
	}
	rows, _ := repository.NewTournamentProviderRepository(e.db).FindAll(context.Background())
	if len(rows) != 1 || rows[0].ProviderAbbr != "ALPHATC" {
		t.Fatalf("provider rows = %+v", rows)
	}
	body = getPage(e, strings.SplitN(loc, "#", 2)[0])
	for _, want := range []string{`data-tournament-setup-outcome="done"`, "Tournaments are ready", "ALPHATC", "Permanent tournament administrator"} {
		if !strings.Contains(body, want) {
			t.Errorf("settings page after set-up lacks %q", want)
		}
	}
	if strings.Contains(body, `name="tournament_abbreviation"`) {
		t.Error("the code is still editable after set-up")
	}

	// Calendar sync works at once, and the empty calendar points at
	// tournament management.
	if res, err := e.handler.service.SyncFromCourtHive(rows[0].ID); err != nil || res.New != 0 {
		t.Fatalf("sync: %+v %v", res, err)
	}
	page := tournamentsPage(t, e, "")
	for _, want := range []string{`data-tournament-setup="set-up"`, "data-first-tournament", `href="/tournament-home/open"`, "Check the set-up again"} {
		if !strings.Contains(page, want) {
			t.Errorf("tournaments page lacks %q", want)
		}
	}

	// Switching off and on again runs set-up again and changes nothing.
	switchTournaments(t, e, false, nil)
	rec = switchTournaments(t, e, true, nil)
	if !strings.Contains(rec.Header().Get("Location"), "tournament_setup=done") || len(fake.Providers()) != 1 {
		t.Fatalf("second switch on: %q", rec.Header().Get("Location"))
	}
	if body := getPage(e, strings.SplitN(rec.Header().Get("Location"), "#", 2)[0]); !strings.Contains(body, "nothing was changed") {
		t.Error("the repeat does not say nothing changed")
	}
	for _, tok := range append(fake.Tokens(), courthivetest.Password) {
		if strings.Contains(body+page, tok) {
			t.Fatal("a page shows a token or the password")
		}
	}
}

func TestSwitchingTournamentsOnWithoutTheBundledStack(t *testing.T) {
	e := newSettingsTestEnv(t)
	switchTournaments(t, e, false, nil)
	if strings.Contains(getPage(e, settingsPath), `name="tournament_abbreviation"`) {
		t.Fatal("the code is offered without a tournament stack")
	}
	rec := switchTournaments(t, e, true, nil)
	loc := rec.Header().Get("Location")
	if rec.Code != http.StatusSeeOther || !strings.Contains(loc, "tournament_setup=external") || !e.holder.ClubSettings().TournamentsEnabled() {
		t.Fatalf("switch on: %d %q", rec.Code, loc)
	}
	body := getPage(e, strings.SplitN(loc, "#", 2)[0])
	if !strings.Contains(body, "not switched on in this deployment") || !strings.Contains(body, "docs/install.md") {
		t.Fatal("no note about the bundled stack")
	}
	// The legacy path: a provider added by hand still syncs.
	if err := e.handler.service.CreateTournamentProvider(&models.TournamentProvider{Name: "Hand made", ProviderAbbr: "HAND"}); err != nil {
		t.Fatal(err)
	}
	page := tournamentsPage(t, e, "")
	if !strings.Contains(page, `data-tournament-setup="not-configured"`) || !strings.Contains(page, "Sync from CourtHive") || strings.Contains(page, "data-setup-run") {
		t.Fatal("tournaments page in the not-configured state is wrong")
	}
	// The set-up button's route says the same, and changes nothing.
	rec = postSetup(t, e, url.Values{"abbreviation": {"ALPHA"}})
	if !strings.Contains(rec.Header().Get("Location"), "setup=external") {
		t.Fatalf("setup without a stack: %q", rec.Header().Get("Location"))
	}
}

func TestTournamentSetupPanelFailureRetryAndUnmanagedAccounts(t *testing.T) {
	e := newSettingsTestEnv(t)
	fake := courthivetest.New(t)
	withTournamentStack(t, e, fake.URL, courthivetest.Password)
	id := fake.AddProvider("ALPHA", "Alpha", false)
	fake.AddAccount(id, "organiser@example.org", courthive.RoleProviderAdmin, "", "")

	page := tournamentsPage(t, e, "")
	if !strings.Contains(page, `data-tournament-setup="not-set-up"`) || !strings.Contains(page, `name="abbreviation" value="ALPHA"`) {
		t.Fatal("panel before set-up")
	}

	// The tournament server is not ready.
	fake.FailNext(courthivetest.CallVersion, http.StatusServiceUnavailable)
	rec := postSetup(t, e, url.Values{"abbreviation": {"ALPHA"}})
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "setup=failed") {
		t.Fatalf("failed run: %q", loc)
	}
	page = tournamentsPage(t, e, "?setup=failed")
	for _, want := range []string{"data-setup-failed", "Reach the tournament server", "Try again", "wait a minute"} {
		if !strings.Contains(page, want) {
			t.Errorf("failure panel lacks %q", want)
		}
	}

	// Try again: adopts the provider made by hand and lists the person.
	rec = postSetup(t, e, url.Values{})
	if !strings.Contains(rec.Header().Get("Location"), "setup=done") {
		t.Fatalf("retry: %q", rec.Header().Get("Location"))
	}
	page = tournamentsPage(t, e, "?setup=done")
	for _, want := range []string{"data-setup-unmanaged", "organiser@example.org", "Tournament administrator", "Nothing has been changed for them", "Tournaments only account", `href="/admin/league/users"`} {
		if !strings.Contains(page, want) {
			t.Errorf("panel after set-up lacks %q", want)
		}
	}

	// Back to the settings page when asked.
	rec = postSetup(t, e, url.Values{"return": {"settings"}})
	if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, settingsPath+"?tournament_setup=done") {
		t.Fatalf("return to settings: %q", loc)
	}
	// The code is fixed now.
	rec = postSetup(t, e, url.Values{"abbreviation": {"OTHER"}})
	if !strings.Contains(rec.Header().Get("Location"), "setup=failed") || !strings.Contains(rec.Header().Get("Location"), "cannot+be+changed") {
		t.Fatalf("changing the code: %q", rec.Header().Get("Location"))
	}
	if len(fake.Providers()) != 1 {
		t.Fatal("a second provider")
	}
}

func TestSyncMovesATournamentToTheSetItWasMovedTo(t *testing.T) {
	e := newSettingsTestEnv(t)
	fake := courthivetest.New(t)
	withTournamentStack(t, e, fake.URL, courthivetest.Password)
	ctx := context.Background()
	svc := e.handler.service
	providers := repository.NewTournamentProviderRepository(e.db)
	tournamentsRepo := repository.NewTournamentRepository(e.db)

	fake.AddProvider("SETA", "Set A", false)
	fake.AddProvider("SETB", "Set B", false)
	a := &models.TournamentProvider{Name: "Set A", ProviderAbbr: "SETA"}
	b := &models.TournamentProvider{Name: "Set B", ProviderAbbr: "SETB"}
	for _, p := range []*models.TournamentProvider{a, b} {
		if err := providers.Create(ctx, p); err != nil {
			t.Fatal(err)
		}
	}

	cup := courthivetest.CalendarEntry{ID: "cup-1", Name: "Cup", StartDate: "2026-06-01", EndDate: "2026-06-02"}
	fake.SetCalendar("SETA", cup)
	if res, err := svc.SyncFromCourtHive(a.ID); err != nil || res.New != 1 {
		t.Fatalf("first sync: %+v %v", res, err)
	}
	row, err := tournamentsRepo.FindByCourthiveTournamentID(ctx, "cup-1")
	if err != nil {
		t.Fatal(err)
	}
	row.IsVisible, row.DisplayOrder = true, 7
	if err := tournamentsRepo.Update(ctx, row); err != nil {
		t.Fatal(err)
	}

	// The tournament moves to set B on the tournament software.
	fake.SetCalendar("SETA")
	fake.SetCalendar("SETB", cup)
	res, err := svc.SyncFromCourtHive(b.ID)
	if err != nil || res.New != 0 || res.Updated != 1 {
		t.Fatalf("sync of B: %+v %v", res, err)
	}
	check := func(when string) {
		t.Helper()
		all, err := tournamentsRepo.FindAll(ctx)
		if err != nil || len(all) != 1 {
			t.Fatalf("%s: rows = %+v %v", when, all, err)
		}
		got := all[0]
		if got.ID != row.ID || got.ProviderID != b.ID || got.ProviderName != "Set B" || !got.IsVisible || got.DisplayOrder != 7 {
			t.Fatalf("%s: row = %+v", when, got)
		}
	}
	check("after syncing B")

	// Syncing A afterwards changes nothing.
	if res, err := svc.SyncFromCourtHive(a.ID); err != nil || res.New+res.Updated+res.Unchanged != 0 {
		t.Fatalf("sync of A: %+v %v", res, err)
	}
	check("after syncing A")
	if res, err := svc.SyncFromCourtHive(b.ID); err != nil || res.Unchanged != 1 {
		t.Fatalf("second sync of B: %+v %v", res, err)
	}
}
