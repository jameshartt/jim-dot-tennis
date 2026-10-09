// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"

	_ "github.com/mattn/go-sqlite3"
)

// newResultsTestService seeds two fixtures:
//   - 900: St Ann's A (team 1) vs Rivals A (team 2), matchups 1 (Mens) and 2 (Womens)
//   - 901: derby St Ann's A (team 1) vs St Ann's B (team 3) with dual slates —
//     matchups 11/12 managed by team 1 and 21/22 managed by team 3.
//
// Matchup 99 (Mens, fixture 902) belongs to an unrelated fixture.
func newResultsTestService(t *testing.T) (*Service, *database.DB) {
	t.Helper()
	db, err := database.New(database.Config{Driver: "sqlite3", FilePath: filepath.Join(t.TempDir(), "results.db")})
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
	start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	exec(`INSERT INTO seasons (id, name, year, start_date, end_date, is_active) VALUES (1, '2026', 2026, ?, ?, 1)`, start, end)
	exec(`INSERT INTO weeks (id, week_number, season_id, start_date, end_date) VALUES (1, 1, 1, ?, ?)`, start, end)
	exec(`INSERT INTO leagues (id, name, type, year, region) VALUES (1, 'Parks', 'Parks', 2026, 'Brighton')`)
	exec(`INSERT INTO divisions (id, name, level, play_day, league_id, season_id) VALUES (1, 'Division 1', 1, 'Tuesday', 1, 1)`)
	exec(`INSERT INTO clubs (id, name) VALUES (1, 'St Ann''s'), (2, 'Rivals')`)
	exec(`INSERT INTO teams (id, name, club_id, division_id, season_id) VALUES
		(1, 'St Ann''s A', 1, 1, 1), (2, 'Rivals A', 2, 1, 1), (3, 'St Ann''s B', 1, 1, 1)`)
	for _, f := range []struct{ id, home, away int }{{900, 1, 2}, {901, 1, 3}, {902, 2, 3}} {
		exec(`INSERT INTO fixtures (id, home_team_id, away_team_id, division_id, season_id, week_id, scheduled_date, venue_location, status)
			VALUES (?, ?, ?, 1, 1, 1, ?, '', 'Scheduled')`, f.id, f.home, f.away, start)
	}
	exec(`INSERT INTO matchups (id, fixture_id, type, status, managing_team_id, notes) VALUES
		(1, 900, 'Mens', 'Pending', NULL, ''), (2, 900, 'Womens', 'Pending', NULL, ''),
		(11, 901, 'Mens', 'Pending', 1, ''), (12, 901, 'Womens', 'Pending', 1, ''),
		(21, 901, 'Mens', 'Pending', 3, ''), (22, 901, 'Womens', 'Pending', 3, ''),
		(99, 902, 'Mens', 'Pending', NULL, '')`)

	return NewService(db, "", 1, ""), db
}

// saveResults is the save path the results handler drives.
func saveResults(svc *Service, fixtureID, managingTeamID uint, isDerby bool, entries []MatchupScoreEntry) error {
	return svc.SaveFixtureResults(fixtureID, managingTeamID, isDerby, entries)
}

func loadMatchup(t *testing.T, svc *Service, id uint) *models.Matchup {
	t.Helper()
	m, err := svc.matchupRepository.FindByID(context.Background(), id)
	if err != nil {
		t.Fatalf("load matchup %d: %v", id, err)
	}
	return m
}

// Behaviour lock for the scoring rules applied on save.
func TestSaveFixtureResultsScoring(t *testing.T) {
	svc, _ := newResultsTestService(t)

	entries := []MatchupScoreEntry{
		{MatchupID: 1, HomeSet1: ip(6), AwaySet1: ip(4), HomeSet2: ip(3), AwaySet2: ip(6), HomeSet3: ip(10), AwaySet3: ip(8)},
		{MatchupID: 2, Conceded: true, ConcededBy: models.ConcededAway},
	}
	if err := saveResults(svc, 900, 0, false, entries); err != nil {
		t.Fatalf("save: %v", err)
	}

	m := loadMatchup(t, svc, 1)
	if m.Status != models.Finished || m.HomeScore != 2 || m.AwayScore != 0 || m.HomeSet3 == nil || *m.HomeSet3 != 10 {
		t.Fatalf("three-set home win: got status=%s %d-%d set3=%v", m.Status, m.HomeScore, m.AwayScore, m.HomeSet3)
	}
	m = loadMatchup(t, svc, 2)
	if m.Status != models.Defaulted || m.HomeScore != 2 || m.AwayScore != 0 || m.HomeSet1 != nil ||
		m.ConcededBy == nil || *m.ConcededBy != models.ConcededAway {
		t.Fatalf("away concession: got status=%s %d-%d set1=%v conceded=%v", m.Status, m.HomeScore, m.AwayScore, m.HomeSet1, m.ConcededBy)
	}

	// Re-saving as a retirement replaces the concession.
	entries = []MatchupScoreEntry{{MatchupID: 2, Retired: true, RetiredBy: models.RetiredHome, HomeSet1: ip(6), AwaySet1: ip(2)}}
	if err := saveResults(svc, 900, 0, false, entries); err != nil {
		t.Fatalf("re-save: %v", err)
	}
	m = loadMatchup(t, svc, 2)
	if m.Status != models.Finished || m.HomeScore != 0 || m.AwayScore != 2 || m.ConcededBy != nil ||
		m.RetiredBy == nil || *m.RetiredBy != models.RetiredHome || m.HomeSet1 == nil || *m.HomeSet1 != 6 {
		t.Fatalf("home retirement: got status=%s %d-%d conceded=%v retired=%v", m.Status, m.HomeScore, m.AwayScore, m.ConcededBy, m.RetiredBy)
	}
}

// Behaviour lock: a derby save mirrors results onto the other team's slate.
func TestSaveFixtureResultsMirrorsDerby(t *testing.T) {
	svc, _ := newResultsTestService(t)

	entries := []MatchupScoreEntry{
		{MatchupID: 11, HomeSet1: ip(6), AwaySet1: ip(4), HomeSet2: ip(4), AwaySet2: ip(6)},
		{MatchupID: 12, HomeSet1: ip(1), AwaySet1: ip(6), HomeSet2: ip(2), AwaySet2: ip(6)},
	}
	if err := saveResults(svc, 901, 1, true, entries); err != nil {
		t.Fatalf("save: %v", err)
	}

	for _, pair := range [][2]uint{{11, 21}, {12, 22}} {
		src, mir := loadMatchup(t, svc, pair[0]), loadMatchup(t, svc, pair[1])
		if mir.Status != src.Status || mir.HomeScore != src.HomeScore || mir.AwayScore != src.AwayScore ||
			mir.HomeSet2 == nil || *mir.HomeSet2 != *src.HomeSet2 {
			t.Fatalf("mirror %d→%d: source %s %d-%d, mirror %s %d-%d", pair[0], pair[1],
				src.Status, src.HomeScore, src.AwayScore, mir.Status, mir.HomeScore, mir.AwayScore)
		}
		if mir.ManagingTeamID == nil || *mir.ManagingTeamID != 3 {
			t.Fatalf("mirror %d lost its managing team", pair[1])
		}
	}
	if m := loadMatchup(t, svc, 11); m.HomeScore != 1 || m.AwayScore != 1 {
		t.Fatalf("split sets should halve: got %d-%d", m.HomeScore, m.AwayScore)
	}
}

// A failure partway through (here: writing the derby mirror) must leave every
// matchup untouched — not one slate scored and the other stale.
func TestSaveFixtureResultsIsAtomic(t *testing.T) {
	svc, db := newResultsTestService(t)
	if _, err := db.Exec(`CREATE TRIGGER test_fail_mirror BEFORE UPDATE ON matchups WHEN OLD.id = 22
		BEGIN SELECT RAISE(ABORT, 'injected mirror failure'); END`); err != nil {
		t.Fatalf("install trigger: %v", err)
	}

	entries := []MatchupScoreEntry{
		{MatchupID: 11, HomeSet1: ip(6), AwaySet1: ip(0), HomeSet2: ip(6), AwaySet2: ip(0)},
		{MatchupID: 12, HomeSet1: ip(6), AwaySet1: ip(0), HomeSet2: ip(6), AwaySet2: ip(0)},
	}
	if err := saveResults(svc, 901, 1, true, entries); err == nil {
		t.Fatal("expected the mirror failure to be reported, got nil")
	}
	for _, id := range []uint{11, 12, 21, 22} {
		if m := loadMatchup(t, svc, id); m.Status != models.Pending || m.HomeScore != 0 {
			t.Errorf("matchup %d changed despite failed save: status=%s %d-%d", id, m.Status, m.HomeScore, m.AwayScore)
		}
	}
}

// A posted matchup ID from another fixture must be rejected, not scored.
func TestSaveFixtureResultsRejectsForeignMatchup(t *testing.T) {
	svc, _ := newResultsTestService(t)

	entries := []MatchupScoreEntry{
		{MatchupID: 1, HomeSet1: ip(6), AwaySet1: ip(0), HomeSet2: ip(6), AwaySet2: ip(0)},
		{MatchupID: 99, HomeSet1: ip(6), AwaySet1: ip(0), HomeSet2: ip(6), AwaySet2: ip(0)},
	}
	if err := saveResults(svc, 900, 0, false, entries); err == nil {
		t.Fatal("expected an error for a matchup from another fixture")
	}
	for _, id := range []uint{1, 99} {
		if m := loadMatchup(t, svc, id); m.Status != models.Pending {
			t.Errorf("matchup %d changed: status=%s", id, m.Status)
		}
	}
}
