// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"path/filepath"
	"testing"
	"time"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"

	_ "github.com/mattn/go-sqlite3"
)

// newSeasonTestService boots a migrated SQLite DB with two seasons:
// 2025 (id 1, active) holding one division, two teams, three players (one
// inactive) and a captain; 2026 (id 2, inactive) empty.
func newSeasonTestService(t *testing.T) (*Service, *database.DB) {
	t.Helper()
	db, err := database.New(database.Config{Driver: "sqlite3", FilePath: filepath.Join(t.TempDir(), "seasons.db")})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.ExecuteMigrations(findMigrationsPathAdmin(t)); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	exec := func(q string, args ...interface{}) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("seed %q: %v", q, err)
		}
	}
	d := func(y int, m time.Month, day int) time.Time { return time.Date(y, m, day, 0, 0, 0, 0, time.UTC) }

	exec(`INSERT INTO seasons (id, name, year, start_date, end_date, is_active) VALUES (1, '2025 Season', 2025, ?, ?, 1)`, d(2025, 4, 1), d(2025, 9, 30))
	exec(`INSERT INTO seasons (id, name, year, start_date, end_date, is_active) VALUES (2, '2026 Season', 2026, ?, ?, 0)`, d(2026, 4, 1), d(2026, 9, 30))
	exec(`INSERT INTO leagues (id, name, type, year, region) VALUES (1, 'Parks League', 'Parks', 2025, 'Brighton')`)
	exec(`INSERT INTO divisions (id, name, level, play_day, league_id, season_id, max_teams_per_club) VALUES (1, 'Division 1', 1, 'Tuesday', 1, 1, 3)`)
	exec(`INSERT INTO clubs (id, name) VALUES (1, 'St Ann''s'), (2, 'Rivals')`)
	exec(`INSERT INTO teams (id, name, club_id, division_id, season_id) VALUES (1, 'St Ann''s A', 1, 1, 1), (2, 'Rivals A', 2, 1, 1)`)
	exec(`INSERT INTO players (id, first_name, last_name, club_id, is_active) VALUES
		('p-active', 'Ada', 'Active', 1, 1),
		('p-captain', 'Cat', 'Captain', 1, 1),
		('p-retired', 'Rex', 'Retired', 1, 0)`)
	exec(`INSERT INTO player_teams (player_id, team_id, season_id, is_active) VALUES
		('p-active', 1, 1, 1), ('p-captain', 1, 1, 1), ('p-retired', 1, 1, 1)`)
	exec(`INSERT INTO captains (player_id, team_id, role, season_id) VALUES ('p-captain', 1, 'Team', 1), ('p-retired', 1, 'Day', 1)`)

	return NewService(db, "", 1, ""), db
}

func count(t *testing.T, db *database.DB, q string, args ...interface{}) int {
	t.Helper()
	var n int
	if err := db.Get(&n, q, args...); err != nil {
		t.Fatalf("count %q: %v", q, err)
	}
	return n
}

func activeSeasonIDs(t *testing.T, db *database.DB) []uint {
	t.Helper()
	var ids []uint
	if err := db.Select(&ids, `SELECT id FROM seasons WHERE is_active = 1 ORDER BY id`); err != nil {
		t.Fatalf("active seasons: %v", err)
	}
	return ids
}

func TestSetActiveSeasonSwitchesExactlyOne(t *testing.T) {
	svc, db := newSeasonTestService(t)

	if err := svc.SetActiveSeason(2); err != nil {
		t.Fatalf("SetActiveSeason(2): %v", err)
	}
	if got := activeSeasonIDs(t, db); len(got) != 1 || got[0] != 2 {
		t.Fatalf("active seasons after switch: got %v, want [2]", got)
	}
}

// A failed activation must not leave the league with zero active seasons:
// the old code deactivated everything before discovering the target was bad.
func TestSetActiveSeasonUnknownIDKeepsCurrentActive(t *testing.T) {
	svc, db := newSeasonTestService(t)

	if err := svc.SetActiveSeason(999); err == nil {
		t.Fatal("SetActiveSeason(999): expected an error for an unknown season")
	}
	if got := activeSeasonIDs(t, db); len(got) != 1 || got[0] != 1 {
		t.Fatalf("active seasons after failed switch: got %v, want [1] (untouched)", got)
	}
}

func TestCreateSeasonWithWeeks(t *testing.T) {
	svc, db := newSeasonTestService(t)

	season := &models.Season{Name: "2027 Season", Year: 2027,
		StartDate: time.Date(2027, 4, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2027, 9, 30, 0, 0, 0, 0, time.UTC)}
	if err := svc.CreateSeasonWithWeeks(season, 18); err != nil {
		t.Fatalf("CreateSeasonWithWeeks: %v", err)
	}
	if season.ID == 0 {
		t.Fatal("season ID was not set")
	}
	if n := count(t, db, `SELECT COUNT(*) FROM weeks WHERE season_id = ?`, season.ID); n != 18 {
		t.Fatalf("weeks created: got %d, want 18", n)
	}
	var lastEnd time.Time
	if err := db.Get(&lastEnd, `SELECT end_date FROM weeks WHERE season_id = ? AND week_number = 18`, season.ID); err != nil {
		t.Fatalf("week 18: %v", err)
	}
	if !lastEnd.Equal(season.EndDate) {
		t.Fatalf("week 18 end: got %v, want season end %v", lastEnd, season.EndDate)
	}
}

// If any week insert fails, the season must not be left behind half-built.
func TestCreateSeasonWithWeeksIsAtomic(t *testing.T) {
	svc, db := newSeasonTestService(t)
	if _, err := db.Exec(`CREATE TRIGGER test_fail_week BEFORE INSERT ON weeks WHEN NEW.week_number = 5
		BEGIN SELECT RAISE(ABORT, 'injected week failure'); END`); err != nil {
		t.Fatalf("install trigger: %v", err)
	}

	season := &models.Season{Name: "Broken Season", Year: 2027,
		StartDate: time.Date(2027, 4, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2027, 9, 30, 0, 0, 0, 0, time.UTC)}
	if err := svc.CreateSeasonWithWeeks(season, 18); err == nil {
		t.Fatal("expected an error when a week insert fails")
	}
	if n := count(t, db, `SELECT COUNT(*) FROM seasons WHERE name = 'Broken Season'`); n != 0 {
		t.Fatalf("orphan season rows left behind: %d", n)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM weeks WHERE season_id NOT IN (SELECT id FROM seasons)`); n != 0 {
		t.Fatalf("orphan week rows left behind: %d", n)
	}
}

// Behaviour lock for the yearly copy: divisions + teams copied, inactive
// players and captains skipped.
func TestCopyFromPreviousSeasonCopiesStructure(t *testing.T) {
	svc, db := newSeasonTestService(t)

	if err := svc.CopyFromPreviousSeason(2, true, true); err != nil {
		t.Fatalf("CopyFromPreviousSeason: %v", err)
	}

	var div models.Division
	if err := db.Get(&div, `SELECT id, name, level, play_day, league_id, season_id, max_teams_per_club, created_at, updated_at FROM divisions WHERE season_id = 2`); err != nil {
		t.Fatalf("copied division: %v", err)
	}
	if div.Name != "Division 1" || div.Level != 1 || div.PlayDay != "Tuesday" || div.LeagueID != 1 || div.MaxTeamsPerClub != 3 {
		t.Fatalf("copied division fields: %+v", div)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM teams WHERE season_id = 2 AND division_id = ? AND active = 1`, div.ID); n != 2 {
		t.Fatalf("copied teams: got %d, want 2", n)
	}

	var players []string
	if err := db.Select(&players, `SELECT pt.player_id FROM player_teams pt JOIN teams t ON t.id = pt.team_id
		WHERE pt.season_id = 2 AND t.name = 'St Ann''s A' ORDER BY pt.player_id`); err != nil {
		t.Fatalf("copied players: %v", err)
	}
	if len(players) != 2 || players[0] != "p-active" || players[1] != "p-captain" {
		t.Fatalf("copied players: got %v, want [p-active p-captain] (inactive skipped)", players)
	}

	var captains []string
	if err := db.Select(&captains, `SELECT player_id || ':' || role FROM captains WHERE season_id = 2`); err != nil {
		t.Fatalf("copied captains: %v", err)
	}
	if len(captains) != 1 || captains[0] != "p-captain:Team" {
		t.Fatalf("copied captains: got %v, want [p-captain:Team] (inactive skipped)", captains)
	}

	// Teams-only copy into a season whose divisions already exist maps by name.
	if _, err := db.Exec(`DELETE FROM captains WHERE season_id = 2; DELETE FROM player_teams WHERE season_id = 2; DELETE FROM teams WHERE season_id = 2`); err != nil {
		t.Fatalf("reset teams: %v", err)
	}
	if err := svc.CopyFromPreviousSeason(2, false, true); err != nil {
		t.Fatalf("teams-only copy: %v", err)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM teams WHERE season_id = 2 AND division_id = ?`, div.ID); n != 2 {
		t.Fatalf("teams-only copy: got %d teams in existing division, want 2", n)
	}
}

// A failure mid-copy (here: a captain insert) must surface as an error and
// leave the target season empty, not half-populated with players missing.
func TestCopyFromPreviousSeasonIsAtomic(t *testing.T) {
	svc, db := newSeasonTestService(t)
	if _, err := db.Exec(`CREATE TRIGGER test_fail_captain BEFORE INSERT ON captains WHEN NEW.season_id = 2
		BEGIN SELECT RAISE(ABORT, 'injected captain failure'); END`); err != nil {
		t.Fatalf("install trigger: %v", err)
	}

	if err := svc.CopyFromPreviousSeason(2, true, true); err == nil {
		t.Fatal("expected the captain failure to be reported, got nil")
	}
	for _, table := range []string{"divisions", "teams", "player_teams", "captains"} {
		if n := count(t, db, `SELECT COUNT(*) FROM `+table+` WHERE season_id = 2`); n != 0 {
			t.Errorf("%s rows left in target season after failed copy: %d", table, n)
		}
	}
}
