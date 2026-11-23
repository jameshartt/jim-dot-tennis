// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package bhplta

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"

	"jim-dot-tennis/internal/database"
)

// The saved BHPLTA pages live with the e2e fixtures so the Playwright
// fresh-install suite serves the same files (see the README there).
const fixturesDir = "tests/e2e/fixtures/bhplta"

func repoPath(t *testing.T, rel string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		candidate := filepath.Join(dir, rel)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		dir = filepath.Dir(dir)
	}
	t.Fatalf("could not locate %s", rel)
	return ""
}

func newDB(t *testing.T) *database.DB {
	t.Helper()
	db, err := database.New(database.Config{Driver: "sqlite3", FilePath: filepath.Join(t.TempDir(), "test.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.ExecuteMigrations(repoPath(t, "migrations")); err != nil {
		t.Fatal(err)
	}
	return db
}

// savedSite serves the saved BHPLTA pages. overrides maps a path to a
// handler used instead of the saved file.
func savedSite(t *testing.T, overrides map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	files := http.FileServer(http.Dir(repoPath(t, fixturesDir)))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h, ok := overrides[r.URL.Path]; ok {
			h(w, r)
			return
		}
		files.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func serveFile(t *testing.T, rel string) http.HandlerFunc {
	path := repoPath(t, filepath.Join(fixturesDir, rel))
	return func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, path) }
}

func bootstrapper(db *database.DB, baseURL string) *Bootstrapper {
	return &Bootstrapper{DB: db, Source: &Source{BaseURL: baseURL, Client: &http.Client{Timeout: 5 * time.Second}}}
}

var tables = []string{"leagues", "league_seasons", "seasons", "weeks", "divisions", "clubs", "teams", "fixtures"}

func counts(t *testing.T, db *database.DB) map[string]int {
	t.Helper()
	out := map[string]int{}
	for _, table := range tables {
		var n int
		if err := db.Get(&n, `SELECT COUNT(*) FROM `+table); err != nil {
			t.Fatal(err)
		}
		out[table] = n
	}
	return out
}

func assertEmpty(t *testing.T, db *database.DB) {
	t.Helper()
	for table, n := range counts(t, db) {
		if n != 0 {
			t.Errorf("%s has %d rows, want none", table, n)
		}
	}
}

// The saved fixtures page is BHPLTA's page from late May 2024: four
// divisions, the remaining fixtures from week 7 (divisions 3 and 4 from
// week 8) to week 18. Its 15 byes are not fixtures.
const (
	savedDivisions = 4
	savedTeams     = 41
	savedFixtures  = 228
	knownClubs     = 12
)

func TestRunImportsSeasonFromSavedPages(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	srv := savedSite(t, nil)

	sum, err := bootstrapper(db, srv.URL).Run(ctx, Options{HomeSlug: "st-anns"}, nil, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := counts(t, db)
	want := map[string]int{"leagues": 1, "league_seasons": 1, "seasons": 1, "weeks": DefaultWeeks,
		"divisions": savedDivisions, "clubs": knownClubs, "teams": savedTeams, "fixtures": savedFixtures}
	for table, n := range want {
		if c[table] != n {
			t.Errorf("%s: %d rows, want %d", table, c[table], n)
		}
	}
	if sum.Divisions != savedDivisions || sum.Teams != savedTeams || sum.Fixtures != savedFixtures ||
		sum.Weeks != DefaultWeeks || sum.ClubsCreated != knownClubs || !sum.LeagueCreated || !sum.SeasonCreated {
		t.Errorf("summary %+v", sum)
	}
	if sum.SeasonName != "2024 Season" || !sum.SeasonActivated || sum.FirstWeek != 7 || sum.LastWeek != 18 {
		t.Errorf("season in summary: %+v", sum)
	}
	if len(sum.Unmatched) != 0 {
		t.Errorf("unmatched: %v", sum.Unmatched)
	}
	if !strings.Contains(strings.Join(sum.Notes, " "), "from week 7") {
		t.Errorf("notes do not explain the missing early weeks: %v", sum.Notes)
	}

	// The home club is St Ann's, with its venue from its club page, and
	// owns the four St Ann's teams.
	if sum.HomeClubName != "St Ann's Tennis" || !sum.HomeClubCreated || sum.HomeTeams != 4 {
		t.Errorf("home club in summary: %q created=%v teams=%d", sum.HomeClubName, sum.HomeClubCreated, sum.HomeTeams)
	}
	var home struct {
		Postcode *string  `db:"postcode"`
		Lat      *float64 `db:"latitude"`
		Teams    int      `db:"teams"`
	}
	if err := db.Get(&home, `SELECT postcode, latitude, (SELECT COUNT(*) FROM teams WHERE club_id = clubs.id) AS teams FROM clubs WHERE id = ?`, sum.HomeClubID); err != nil {
		t.Fatal(err)
	}
	if home.Postcode == nil || *home.Postcode == "" || home.Lat == nil || home.Teams != 4 {
		t.Errorf("home club row: postcode %v lat %v teams %d", home.Postcode, home.Lat, home.Teams)
	}

	// Every club has its venue, and every team a club with a page.
	var noVenue int
	if err := db.Get(&noVenue, `SELECT COUNT(*) FROM clubs WHERE latitude IS NULL OR address = ''`); err != nil || noVenue != 0 {
		t.Errorf("%d clubs without venue details (err %v)", noVenue, err)
	}
	var queens int
	if err := db.Get(&queens, `SELECT COUNT(*) FROM teams t JOIN clubs c ON c.id = t.club_id WHERE c.name = 'Queens Park Tennis Club'`); err != nil || queens != 6 {
		t.Errorf("Queens Park teams: %d (err %v), want 6", queens, err)
	}

	// Fixtures hang off weeks by week number, and the week windows hold
	// their fixtures; divisions get their play day.
	var misplaced int
	if err := db.Get(&misplaced, `
		SELECT COUNT(*) FROM fixtures f JOIN weeks w ON w.id = f.week_id
		WHERE date(f.scheduled_date) < date(w.start_date) OR date(f.scheduled_date) > date(w.end_date)`); err != nil || misplaced != 0 {
		t.Errorf("%d fixtures outside their week's dates (err %v)", misplaced, err)
	}
	var playDay string
	if err := db.Get(&playDay, `SELECT play_day FROM divisions WHERE name = 'Division 1'`); err != nil || playDay != "Thursday" {
		t.Errorf("Division 1 play day %q (err %v), want Thursday", playDay, err)
	}
	var level int
	if err := db.Get(&level, `SELECT level FROM divisions WHERE name = 'Division 3'`); err != nil || level != 3 {
		t.Errorf("Division 3 level %d (err %v)", level, err)
	}
}

func TestRunIsRepeatableWithoutDuplicates(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	srv := savedSite(t, nil)
	b := bootstrapper(db, srv.URL)
	if _, err := b.Run(ctx, Options{HomeSlug: "st-anns"}, nil, nil); err != nil {
		t.Fatal(err)
	}
	before := counts(t, db)

	// The same season again is refused, and changes nothing.
	_, err := b.Run(ctx, Options{}, nil, nil)
	var exists *SeasonExistsError
	if !errors.As(err, &exists) || !strings.Contains(Describe(err), "2024 Season already has divisions") {
		t.Fatalf("second run: %v", err)
	}
	if after := counts(t, db); !equal(before, after) {
		t.Fatalf("refused run changed rows: %v -> %v", before, after)
	}

	// Setup-style (keep the existing season): clubs are matched, not duplicated.
	sum, err := b.Run(ctx, Options{HomeSlug: "hove-park-tennis-club", KeepExistingSeason: true}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !sum.SeasonKept || sum.ClubsCreated != 0 || sum.ClubsMatched != knownClubs {
		t.Fatalf("keep-season run: %+v", sum)
	}
	if after := counts(t, db); !equal(before, after) {
		t.Fatalf("keep-season run changed rows: %v -> %v", before, after)
	}

	// A later season reuses the league and clubs and adds its own teams.
	snap, err := b.Source.Fetch(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := range snap.Divisions {
		for j := range snap.Divisions[i].Fixtures {
			f := &snap.Divisions[i].Fixtures[j]
			f.Date = f.Date.AddDate(1, 0, 0)
		}
	}
	sum, err = b.Apply(ctx, snap, Options{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sum.SeasonName != "2025 Season" || sum.SeasonActivated || sum.ClubsCreated != 0 || sum.LeagueCreated {
		t.Fatalf("later season: %+v", sum)
	}
	after := counts(t, db)
	if after["clubs"] != knownClubs || after["leagues"] != 1 || after["seasons"] != 2 ||
		after["teams"] != 2*savedTeams || after["fixtures"] != 2*savedFixtures || after["league_seasons"] != 2 {
		t.Fatalf("after a later season: %v", after)
	}
}

func equal(a, b map[string]int) bool {
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return len(a) == len(b)
}

// A failure part-way through the writes rolls everything back.
func TestRunRollsBackOnFailureMidway(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	srv := savedSite(t, nil)
	if _, err := db.Exec(`
		CREATE TRIGGER fail_midway BEFORE INSERT ON fixtures
		WHEN (SELECT COUNT(*) FROM fixtures) >= 100
		BEGIN SELECT RAISE(ABORT, 'injected failure'); END`); err != nil {
		t.Fatal(err)
	}
	_, err := bootstrapper(db, srv.URL).Run(ctx, Options{HomeSlug: "st-anns"}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "injected failure") {
		t.Fatalf("Run: %v, want the injected failure", err)
	}
	if msg := Describe(err); !strings.Contains(msg, "could not be saved") {
		t.Errorf("Describe: %q", msg)
	}
	assertEmpty(t, db)

	// A failure in the caller's step before commit rolls back too.
	if _, err := db.Exec(`DROP TRIGGER fail_midway`); err != nil {
		t.Fatal(err)
	}
	_, err = bootstrapper(db, srv.URL).Run(ctx, Options{HomeSlug: "st-anns"}, nil, func(context.Context, *sqlx.Tx, *Summary) error {
		return errors.New("completion failed")
	})
	if err == nil {
		t.Fatal("want the before-commit error")
	}
	assertEmpty(t, db)
}

func TestFetchFailuresAreReadableAndWriteNothing(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name      string
		overrides map[string]http.HandlerFunc
		closed    bool
		want      string
		also      string // a second phrase the message must contain
		notWant   string // a phrase the message must not contain
	}{
		{
			name: "fixtures page error status",
			overrides: map[string]http.HandlerFunc{"/bhplta_tables/fixtures/": func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "down", http.StatusServiceUnavailable)
			}},
			want: "the BHPLTA fixtures page failed: the BHPLTA website answered with HTTP status 503",
		},
		{
			name: "fixtures page markup changed",
			overrides: map[string]http.HandlerFunc{"/bhplta_tables/fixtures/": func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`<html><body><table class="new-layout"><tr><td>Dyke v Hove</td></tr></table></body></html>`))
			}},
			want: "The BHPLTA fixtures page did not look as expected",
		},
		{
			name:   "site unreachable",
			closed: true,
			want:   "Could not reach the BHPLTA fixtures page",
			also:   "check that it can connect to the internet",
		},
		{
			name: "fixtures page refused",
			overrides: map[string]http.HandlerFunc{"/bhplta_tables/fixtures/": func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "Forbidden", http.StatusForbidden)
			}},
			want:    "The BHPLTA website refused the request for the BHPLTA fixtures page (HTTP status 403)",
			also:    "its internet connection is working",
			notWant: "check that",
		},
		{
			name: "fixtures page reset",
			overrides: map[string]http.HandlerFunc{"/bhplta_tables/fixtures/": func(w http.ResponseWriter, r *http.Request) {
				conn, _, err := w.(http.Hijacker).Hijack()
				if err == nil {
					_ = conn.Close()
				}
			}},
			want:    "The BHPLTA website refused the request for the BHPLTA fixtures page (it reset or closed the connection",
			notWant: "check that",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := newDB(t)
			srv := savedSite(t, tc.overrides)
			if tc.closed {
				srv.Close()
			}
			_, err := bootstrapper(db, srv.URL).Run(ctx, Options{HomeSlug: "st-anns"}, nil, nil)
			var fe *FetchError
			if !errors.As(err, &fe) {
				t.Fatalf("Run: %v, want a *FetchError", err)
			}
			msg := Describe(err)
			if !strings.Contains(msg, tc.want) || !strings.Contains(msg, tc.also) {
				t.Errorf("Describe: %q, want it to contain %q and %q", msg, tc.want, tc.also)
			}
			if tc.notWant != "" && strings.Contains(strings.ToLower(msg), tc.notWant) {
				t.Errorf("Describe: %q, must not contain %q", msg, tc.notWant)
			}
			assertEmpty(t, db)
		})
	}
}

// A single club page that cannot be read is reported, not fatal.
func TestMissingClubPageIsReported(t *testing.T) {
	db := newDB(t)
	srv := savedSite(t, map[string]http.HandlerFunc{"/clubs/rookery-tennis-club/": http.NotFound})
	sum, err := bootstrapper(db, srv.URL).Run(context.Background(), Options{HomeSlug: "st-anns"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(sum.Unmatched) != 1 || !strings.Contains(sum.Unmatched[0], "Rookery") {
		t.Fatalf("unmatched: %v", sum.Unmatched)
	}
	var lat *float64
	if err := db.Get(&lat, `SELECT latitude FROM clubs WHERE name = 'Rookery Tennis Club'`); err != nil || lat != nil {
		t.Fatalf("Rookery: lat %v err %v, want a club without venue details", lat, err)
	}
}

// Between seasons BHPLTA lists no fixtures: the league and clubs are still
// imported and the home club chosen, with a note about adding the season.
func TestOffSeasonImportsClubsOnly(t *testing.T) {
	db := newDB(t)
	srv := savedSite(t, map[string]http.HandlerFunc{"/bhplta_tables/fixtures/": serveFile(t, "off-season/bhplta_tables/fixtures/index.html")})
	var steps []string
	sum, err := bootstrapper(db, srv.URL).Run(context.Background(), Options{HomeSlug: "queens-park"}, func(step, total int, what string) {
		steps = append(steps, what)
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !sum.NoFixtures || sum.SeasonID != 0 || sum.HomeClubName != "Queens Park Tennis Club" || len(sum.Notes) == 0 {
		t.Fatalf("summary %+v", sum)
	}
	c := counts(t, db)
	if c["leagues"] != 1 || c["clubs"] != knownClubs || c["seasons"] != 0 || c["fixtures"] != 0 {
		t.Fatalf("rows: %v", c)
	}
	if len(steps) != 1+knownClubs || steps[0] != "the fixtures page" {
		t.Fatalf("progress steps: %v", steps)
	}
}

// Clubs already in the database (named differently) are matched, and only
// their missing venue details are filled in.
func TestExistingClubsAreMatched(t *testing.T) {
	db := newDB(t)
	if _, err := db.Exec(`INSERT INTO clubs (name, address, website, phone_number) VALUES
		('St Ann''s Tennis Club', '10 Egremont Place', '', ''), ('Hove', '', '', '')`); err != nil {
		t.Fatal(err)
	}
	srv := savedSite(t, nil)
	sum, err := bootstrapper(db, srv.URL).Run(context.Background(), Options{HomeSlug: "st-anns"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sum.ClubsMatched != 2 || sum.ClubsCreated != knownClubs-2 || sum.HomeClubID != 1 || sum.HomeClubCreated {
		t.Fatalf("summary %+v", sum)
	}
	var addr string
	if err := db.Get(&addr, `SELECT address FROM clubs WHERE id = 1`); err != nil || addr != "10 Egremont Place" {
		t.Fatalf("existing address overwritten: %q (err %v)", addr, err)
	}
	var hoveTeams int
	if err := db.Get(&hoveTeams, `SELECT COUNT(*) FROM teams WHERE club_id = 2`); err != nil || hoveTeams != 7 {
		t.Fatalf("Hove teams on the existing club: %d (err %v), want 7", hoveTeams, err)
	}
}

func TestClubKey(t *testing.T) {
	for _, group := range [][]string{
		{"St Ann's Tennis", "St Ann’s", "St Anns", "St Ann's Tennis Club", "St Ann&#039;s"},
		{"Hove Park Tennis Club", "Hove Park", "Hove"},
		{"King Alfred Lawn Tennis Club", "King Alfred Tennis Club", "King Alfred"},
		{"Queens Park Tennis Club", "Queens"},
		{"Park Avenue Tennis Club", "Park Avenue"},
	} {
		for _, name := range group[1:] {
			if clubKey(name) != clubKey(group[0]) {
				t.Errorf("clubKey(%q) = %q, clubKey(%q) = %q", name, clubKey(name), group[0], clubKey(group[0]))
			}
		}
	}
	if clubKey("Preston Park") == clubKey("Park Avenue") {
		t.Error("Preston Park and Park Avenue must differ")
	}
}
