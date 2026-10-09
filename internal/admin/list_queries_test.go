// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"jim-dot-tennis/internal/database"

	"github.com/jmoiron/sqlx"
	"github.com/mattn/go-sqlite3"
)

// countingConnector opens sqlite connections whose statements are counted.
// The wrapped conn exposes only Prepare, so database/sql routes every query
// and exec through it.
type countingConnector struct {
	dsn     string
	queries *atomic.Int64
}

type countingConn struct {
	driver.Conn
	queries *atomic.Int64
}

func (c countingConnector) Connect(context.Context) (driver.Conn, error) {
	conn, err := c.Driver().Open(c.dsn)
	if err != nil {
		return nil, err
	}
	return countingConn{Conn: conn, queries: c.queries}, nil
}

func (countingConnector) Driver() driver.Driver { return &sqlite3.SQLiteDriver{} }

func (c countingConn) Prepare(query string) (driver.Stmt, error) {
	c.queries.Add(1)
	return c.Conn.Prepare(query)
}

// countingDB opens a second handle on an existing sqlite file that counts
// every statement it runs.
func countingDB(t *testing.T, path string) (*database.DB, *atomic.Int64) {
	t.Helper()
	var n atomic.Int64
	db := sqlx.NewDb(sql.OpenDB(countingConnector{dsn: "file:" + path + "?_foreign_keys=on", queries: &n}), "sqlite3")
	t.Cleanup(func() { db.Close() })
	return &database.DB{DB: db}, &n
}

// seedPlayedSeason builds an active season in which St Ann's A has played
// `played` completed home fixtures against Rivals A (two scored matchups each)
// and has `upcoming` more scheduled. Returns the database file path.
func seedPlayedSeason(t *testing.T, played, upcoming int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "season.db")
	db, err := database.New(database.Config{Driver: "sqlite3", FilePath: path})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := db.ExecuteMigrations(findMigrationsPathAdmin(t)); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	exec := func(q string, args ...interface{}) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("seed %q: %v", q, err)
		}
	}

	weeks := played + upcoming
	start := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -7*played)
	exec(`INSERT INTO seasons (id, name, year, start_date, end_date, is_active) VALUES (1, 'Now', 2026, ?, ?, 1)`,
		start, start.AddDate(0, 0, 7*weeks))
	exec(`INSERT INTO leagues (id, name, type, year, region) VALUES (1, 'Parks', 'Parks', 2026, 'Brighton')`)
	exec(`INSERT INTO divisions (id, name, level, play_day, league_id, season_id) VALUES (1, 'Division 1', 1, 'Tuesday', 1, 1)`)
	exec(`INSERT INTO clubs (id, name, address, website, phone_number) VALUES (1, 'St Ann''s', '', '', ''), (2, 'Rivals', '', '', '')`)
	exec(`INSERT INTO teams (id, name, club_id, division_id, season_id) VALUES (1, 'St Ann''s A', 1, 1, 1), (2, 'Rivals A', 2, 1, 1)`)
	exec(`INSERT INTO players (id, first_name, last_name, gender, club_id) VALUES
		('h1', 'Home', 'One', 'Men', 1), ('h2', 'Home', 'Two', 'Men', 1), ('h3', 'Home', 'Three', 'Men', 1), ('h4', 'Home', 'Four', 'Men', 1),
		('a1', 'Away', 'One', 'Men', 2), ('a2', 'Away', 'Two', 'Men', 2), ('a3', 'Away', 'Three', 'Men', 2), ('a4', 'Away', 'Four', 'Men', 2)`)
	exec(`INSERT INTO player_teams (player_id, team_id, season_id, is_active) VALUES
		('h1', 1, 1, 1), ('h2', 1, 1, 1), ('h3', 1, 1, 1), ('h4', 1, 1, 1), ('a1', 2, 1, 1), ('a2', 2, 1, 1), ('a3', 2, 1, 1), ('a4', 2, 1, 1)`)

	for w := 1; w <= weeks; w++ {
		date := start.AddDate(0, 0, 7*(w-1))
		exec(`INSERT INTO weeks (id, week_number, season_id, start_date, end_date, name) VALUES (?, ?, 1, ?, ?, ?)`,
			w, w, date, date.AddDate(0, 0, 6), fmt.Sprintf("Week %d", w))
		status := "Scheduled"
		if w <= played {
			status = "Completed"
		}
		exec(`INSERT INTO fixtures (id, home_team_id, away_team_id, division_id, season_id, week_id, scheduled_date, venue_location, status, notes, completed_date)
			VALUES (?, 1, 2, 1, 1, ?, ?, '', ?, '', ?)`, w, w, date, status, date)
		if w > played {
			continue
		}
		// Pair 1 (h1+h2) wins 6-0 6-0 over a1+a2; pair 2 (h3+h4) loses 0-6 0-6.
		for m, pair := range [][4]string{{"h1", "h2", "a1", "a2"}, {"h3", "h4", "a3", "a4"}} {
			matchupType := []string{"Mens", "1st Mixed"}[m]
			id := w*10 + m
			hs, as := 6, 0
			if m == 1 {
				hs, as = 0, 6
			}
			exec(`INSERT INTO matchups (id, fixture_id, type, status, home_score, away_score, home_set1, away_set1, home_set2, away_set2, notes)
				VALUES (?, ?, ?, 'Finished', ?, ?, ?, ?, ?, ?, '')`, id, w, matchupType, 2*hs/6, 2*as/6, hs, as, hs, as)
			for i, p := range pair {
				exec(`INSERT INTO matchup_players (matchup_id, player_id, is_home) VALUES (?, ?, ?)`, id, p, i < 2)
			}
		}
	}
	return path
}

// The points table's statement count must not grow with the number of
// matchups played (it ran one players query per matchup).
func TestPointsTableQueriesDoNotScaleWithMatchups(t *testing.T) {
	const played = 12
	db, queries := countingDB(t, seedPlayedSeason(t, played, 0))
	h := NewPointsHandler(NewService(db, "", 1, ""), "")

	men, _, err := h.calculatePlayerPoints()
	if err != nil {
		t.Fatalf("calculate: %v", err)
	}
	if n := queries.Load(); n > 10 {
		t.Errorf("points table ran %d statements for %d matchups", n, 2*played)
	}

	if len(men) != 8 {
		t.Fatalf("want all 8 players ranked, got %d", len(men))
	}
	for _, p := range men {
		if p.MatchesPlayed != played {
			t.Errorf("%s played %d, want %d", p.ID, p.MatchesPlayed, played)
		}
	}
	winners := map[string]bool{"h1": true, "h2": true, "a3": true, "a4": true}
	for i, p := range men {
		if (i < 4) != winners[p.ID] {
			t.Errorf("rank %d is %s; winners must take the top four places: %+v", i+1, p.ID, men)
			break
		}
	}
}

// The fixture lists' statement count must not grow with the number of
// fixtures shown (each row ran its own team, week, division and season
// lookups).
func TestFixtureListQueriesDoNotScaleWithFixtures(t *testing.T) {
	const played, upcoming = 12, 6
	db, queries := countingDB(t, seedPlayedSeason(t, played, upcoming))
	svc := NewService(db, "", 1, "")

	for name, list := range map[string]func() (int, error){
		"past": func() (int, error) {
			_, fixtures, err := svc.GetHomeClubPastFixtures()
			return checkFixtureRelations(t, fixtures), err
		},
		"upcoming": func() (int, error) {
			_, fixtures, err := svc.GetHomeClubFixtures()
			return checkFixtureRelations(t, fixtures), err
		},
	} {
		queries.Store(0)
		n, err := list()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if n == 0 {
			t.Errorf("%s: no fixtures listed", name)
		}
		if q := queries.Load(); q > 15 {
			t.Errorf("%s fixtures ran %d statements for %d fixtures", name, q, n)
		}
	}
}

func checkFixtureRelations(t *testing.T, fixtures []FixtureWithRelations) int {
	t.Helper()
	for _, f := range fixtures {
		if f.HomeTeam == nil || f.HomeTeam.Name != "St Ann's A" || f.AwayTeam == nil || f.AwayTeam.Name != "Rivals A" ||
			f.Week == nil || f.Week.ID != f.WeekID || f.Division == nil || f.Season == nil || !f.IsHomeClub || f.IsAwayClub {
			t.Errorf("fixture %d relations not loaded: %+v", f.ID, f)
		}
	}
	return len(fixtures)
}
