// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package setup

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/repository"
)

// demoReadyService returns a service with an administrator and no club.
func demoReadyService(t *testing.T) (*Service, *database.DB) {
	t.Helper()
	s, db := newTestService(t)
	if _, err := s.CreateAdmin(context.Background(), redeemedTicket(t, s), "demo.admin", goodPassword); err != nil {
		t.Fatal(err)
	}
	return s, db
}

func redeemedTicket(t *testing.T, s *Service) string {
	t.Helper()
	ctx := context.Background()
	code, err := s.IssueCode(ctx)
	if err != nil || code == "" {
		t.Fatalf("issue code: %q %v", code, err)
	}
	ticket, err := s.RedeemCode(ctx, code, "test")
	if err != nil {
		t.Fatal(err)
	}
	return ticket
}

func count(t *testing.T, db *database.DB, table string) int {
	t.Helper()
	var n int
	if err := db.Get(&n, `SELECT COUNT(*) FROM `+table); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestDemoTablesAreAllAccountedFor(t *testing.T) {
	_, db := newTestService(t)
	var tables []string
	if err := db.Select(&tables, `SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'schema_migrations%'`); err != nil {
		t.Fatal(err)
	}
	for _, name := range tables {
		inCleared, inKept := slices.Contains(clearedTables, name), slices.Contains(keptTables, name)
		if inCleared == inKept {
			t.Errorf("table %q must be in exactly one of clearedTables / keptTables (cleared %v, kept %v)", name, inCleared, inKept)
		}
	}
}

func TestLoadDemoOnEmptyInstance(t *testing.T) {
	ctx := context.Background()
	s, db := demoReadyService(t)
	if err := s.LoadDemo(ctx); err != nil {
		t.Fatalf("LoadDemo: %v", err)
	}
	if s.state.Status(ctx) != StatusReady || !s.DemoActive() {
		t.Fatalf("status %v demo %v", s.state.Status(ctx), s.DemoActive())
	}
	if home := s.state.HomeClub().Club(); home == nil || home.Name != "Sample Park Tennis Club" {
		t.Fatalf("home club %+v", home)
	}
	want := map[string]int{"clubs": 3, "leagues": 1, "seasons": 1, "weeks": demoWeeks, "divisions": 2, "teams": 6, "players": 24, "fixtures": 12, "captains": 2}
	for table, n := range want {
		if got := count(t, db, table); got != n {
			t.Errorf("%s: %d rows, want %d", table, got, n)
		}
	}
	var finished, completed, availability, selected int
	_ = db.Get(&finished, `SELECT COUNT(*) FROM matchups WHERE status = 'Finished'`)
	_ = db.Get(&completed, `SELECT COUNT(*) FROM fixtures WHERE status = 'Completed'`)
	_ = db.Get(&availability, `SELECT COUNT(*) FROM player_fixture_availability`)
	_ = db.Get(&selected, `SELECT COUNT(*) FROM fixture_players`)
	if finished != 24 || completed != 6 || availability == 0 || selected != 4 {
		t.Errorf("finished %d completed %d availability %d selected %d", finished, completed, availability, selected)
	}
	if err := s.LoadDemo(ctx); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("second LoadDemo: %v", err)
	}
}

func TestLoadDemoRefusedWithRealData(t *testing.T) {
	for name, stmt := range map[string]string{
		"club":   `INSERT INTO clubs (name, address, website, phone_number) VALUES ('Real', '', '', '')`,
		"league": `INSERT INTO leagues (name, type, year, region) VALUES ('Real', 'Parks', 2026, 'x')`,
		"season": `INSERT INTO seasons (name, year, start_date, end_date) VALUES ('Real', 2026, '2026-01-01', '2026-02-01')`,
		"player": `INSERT INTO clubs (name, address, website, phone_number) VALUES ('X', '', '', ''); INSERT INTO players (id, first_name, last_name, club_id) VALUES ('p', 'A', 'B', 1)`,
	} {
		t.Run(name, func(t *testing.T) {
			s, db := demoReadyService(t)
			if _, err := db.Exec(stmt); err != nil {
				t.Fatal(err)
			}
			before := count(t, db, "clubs")
			if err := s.LoadDemo(context.Background()); !errors.Is(err, ErrDemoHasData) {
				t.Fatalf("LoadDemo: %v", err)
			}
			if s.DemoActive() || count(t, db, "clubs") != before || count(t, db, "fixtures") != 0 {
				t.Fatal("refused load left changes behind")
			}
		})
	}
	// A real club set up already: no demo either (status is ready).
	s, _ := demoReadyService(t)
	if _, err := s.ChooseClub(context.Background(), "hove-park-tennis-club"); err != nil {
		t.Fatal(err)
	}
	if err := s.LoadDemo(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("LoadDemo after real setup: %v", err)
	}
}

func TestLoadDemoRefusedWithEnvOverride(t *testing.T) {
	s, db := demoReadyService(t)
	t.Setenv("HOME_CLUB_NAME", "Anything")
	if err := s.LoadDemo(context.Background()); !errors.Is(err, ErrDemoEnvOverride) {
		t.Fatalf("LoadDemo: %v", err)
	}
	if count(t, db, "clubs") != 0 {
		t.Fatal("clubs written")
	}
}

func TestClearDemoKeepsAdminAndReturnsToClubStep(t *testing.T) {
	ctx := context.Background()
	s, db := demoReadyService(t)
	if err := s.LoadDemo(ctx); err != nil {
		t.Fatal(err)
	}
	// An account linked to a demo player keeps existing, unlinked.
	if _, err := db.Exec(`UPDATE users SET player_id = (SELECT id FROM players LIMIT 1)`); err != nil {
		t.Fatal(err)
	}
	if err := s.ClearDemo(ctx); err != nil {
		t.Fatalf("ClearDemo: %v", err)
	}
	for _, table := range clearedTables {
		if n := count(t, db, table); n != 0 {
			t.Errorf("%s still has %d rows", table, n)
		}
	}
	if userCount(t, db) != 1 {
		t.Fatalf("users: %d", userCount(t, db))
	}
	var linked int
	_ = db.Get(&linked, `SELECT COUNT(*) FROM users WHERE player_id IS NOT NULL`)
	settings := repository.NewSettingsRepository(db)
	for _, key := range []string{repository.SettingHomeClubID, repository.SettingSetupCompletedAt, repository.SettingDemoData} {
		if _, ok, _ := settings.GetString(ctx, key); ok {
			t.Errorf("setting %s still stored", key)
		}
	}
	if linked != 0 || s.DemoActive() || s.state.Status(ctx) != StatusNeedsClub || s.state.HomeClub().Resolved() {
		t.Fatalf("linked %d demo %v status %v resolved %v", linked, s.DemoActive(), s.state.Status(ctx), s.state.HomeClub().Resolved())
	}
	if n := count(t, db, "sqlite_sequence WHERE name = 'clubs'"); n != 0 {
		t.Fatalf("clubs numbering not reset")
	}

	// Real setup works afterwards, and demo data can be loaded again.
	res, err := s.ChooseClub(ctx, "hove-park-tennis-club")
	if err != nil {
		t.Fatalf("ChooseClub after clearing: %v", err)
	}
	if s.state.Status(ctx) != StatusReady || s.state.HomeClub().ID() != res.ClubID || s.DemoActive() {
		t.Fatal("real setup did not take effect")
	}
	if err := s.ClearDemo(ctx); !errors.Is(err, ErrNotDemo) {
		t.Fatalf("ClearDemo on a real instance: %v", err)
	}
	if count(t, db, "clubs") != 1 || s.state.Status(ctx) != StatusReady {
		t.Fatal("clear on a real instance changed data")
	}
}

func TestClearDemoRefusedWithoutFlag(t *testing.T) {
	ctx := context.Background()
	s, db := demoReadyService(t)
	if _, err := db.Exec(`INSERT INTO clubs (name, address, website, phone_number) VALUES ('Real', '', '', '')`); err != nil {
		t.Fatal(err)
	}
	if err := s.ClearDemo(ctx); !errors.Is(err, ErrNotDemo) {
		t.Fatalf("ClearDemo: %v", err)
	}
	if count(t, db, "clubs") != 1 {
		t.Fatal("clubs deleted without the demo flag")
	}
}

func TestDemoBanner(t *testing.T) {
	ctx := context.Background()
	s, _ := demoReadyService(t)
	page := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"a":"<body>"}`)
		case "/fragment":
			_, _ = io.WriteString(w, `<tr><td>row</td></tr>`)
		case "/sniffed":
			_, _ = io.WriteString(w, "<!DOCTYPE html><html><BODY class=x><p>hi</p></BODY></html>")
		default:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Content-Length", "47")
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `<html><body class="a"><p>page</p></body></html>`)
		}
	})
	h := s.DemoBanner(page)
	get := func(path string) (int, string) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		return rec.Code, rec.Body.String()
	}

	if _, body := get("/"); strings.Contains(body, "demo-banner") {
		t.Fatal("banner shown before demo data is loaded")
	}
	if err := s.LoadDemo(ctx); err != nil {
		t.Fatal(err)
	}
	code, body := get("/")
	if code != http.StatusCreated || !strings.Contains(body, `<body class="a"><div id="demo-banner"`) || !strings.Contains(body, "Clear demo data") || !strings.HasSuffix(body, "</html>") {
		t.Fatalf("page: %d %s", code, body)
	}
	if _, body := get("/sniffed"); !strings.Contains(body, `<BODY class=x><div id="demo-banner"`) {
		t.Fatalf("sniffed page: %s", body)
	}
	for _, p := range []string{"/json", "/fragment"} {
		if _, body := get(p); strings.Contains(body, "demo-banner") {
			t.Fatalf("%s got a banner", p)
		}
	}
	if err := s.ClearDemo(ctx); err != nil {
		t.Fatal(err)
	}
	if _, body := get("/"); strings.Contains(body, "demo-banner") {
		t.Fatal("banner shown after clearing")
	}
}

func TestDemoPagesFlow(t *testing.T) {
	ctx := context.Background()
	s, db := newTestService(t)
	code, _ := s.IssueCode(ctx)
	b := newFlow(t, s, db)
	b.do("POST", Path, url.Values{"code": {code}})
	b.do("POST", AdminPath, url.Values{"username": {"flow.admin"}, "password": {goodPassword}, "password_confirm": {goodPassword}})

	if st, _, body := b.do("GET", ClubPath, nil); st != 200 || !strings.Contains(body, "Explore with demo data") {
		t.Fatalf("club page: %d", st)
	}
	if st, _, _ := b.do("GET", DemoClearPath, nil); st != http.StatusNotFound {
		t.Fatalf("clear page without demo data: %d", st)
	}
	if st, loc, _ := b.do("POST", DemoPath, url.Values{}); st != http.StatusSeeOther || loc != dashboardPath {
		t.Fatalf("load: %d %q", st, loc)
	}
	if st, _, body := b.do("GET", DemoClearPath, nil); st != 200 || !strings.Contains(body, "Clear demo data?") {
		t.Fatalf("clear page: %d", st)
	}
	if st, loc, _ := b.do("POST", DemoClearPath, url.Values{}); st != http.StatusSeeOther || loc != ClubPath {
		t.Fatalf("clear: %d %q", st, loc)
	}
	if st, _, body := b.do("GET", ClubPath, nil); st != 200 || !strings.Contains(body, "Choose your club") {
		t.Fatalf("back at club step: %d", st)
	}
	// Not signed in: neither route works.
	anon := newFlow(t, s, db)
	if st, _, _ := anon.do("POST", DemoPath, url.Values{}); st != http.StatusForbidden {
		t.Fatalf("anonymous load: %d", st)
	}
}
