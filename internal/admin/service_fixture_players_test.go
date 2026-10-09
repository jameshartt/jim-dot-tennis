// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"

	_ "github.com/mattn/go-sqlite3"
)

// Behaviour lock for the team-selection player lists: availability priority
// (fixture-specific > date exception > weekday pattern > unknown) and the
// per-player eligibility rules, as seen through the page's service call.
func TestTeamSelectionAvailabilityAndEligibility(t *testing.T) {
	db := seedTeamSelectionFixture(t)
	svc := NewService(db, "", 1, "")
	team, others, err := svc.GetAvailablePlayersWithEligibilityForTeamSelection(selectionFixtureID, 0)
	if err != nil {
		t.Fatalf("load selection: %v", err)
	}
	got := map[string]PlayerWithEligibility{}
	for _, p := range team {
		got["team:"+p.Player.ID] = p
	}
	for _, p := range others {
		got["other:"+p.Player.ID] = p
	}
	if len(got) != 6 {
		t.Fatalf("want 3 team + 3 other home-club players, got %v", keys(got))
	}

	for key, want := range map[string]PlayerAvailabilityInfo{
		"team:p-fix":   {Status: models.Available, Notes: "fixture note"},
		"team:p-date":  {Status: models.IfNeeded, Notes: "newer"},
		"other:p-day":  {Status: models.Unavailable, Notes: "busy"},
		"other:p-none": {Status: models.Unknown},
	} {
		p, ok := got[key]
		if !ok {
			t.Errorf("%s missing from lists %v", key, keys(got))
			continue
		}
		if p.AvailabilityStatus != want.Status || p.AvailabilityNotes != want.Notes {
			t.Errorf("%s availability: got %s %q, want %s %q", key, p.AvailabilityStatus, p.AvailabilityNotes, want.Status, want.Notes)
		}
	}

	for key, want := range map[string]PlayerEligibilityInfo{
		"team:p-fix":   {CanPlay: true, RemainingHigherTeamPlays: 4},
		"team:p-week":  {CanPlay: false, PlayedThisWeek: true, PlayedThisWeekTeam: "St Ann's A", RemainingHigherTeamPlays: -1},
		"other:p-up":   {CanPlay: false, IsLockedToHigherTeam: true, LockedToTeamName: "St Ann's A", RemainingHigherTeamPlays: -1},
		"other:p-none": {CanPlay: true, RemainingHigherTeamPlays: 4},
	} {
		e := got[key].Eligibility
		if e == nil {
			t.Errorf("%s: no eligibility", key)
			continue
		}
		if e.CanPlay != want.CanPlay || e.PlayedThisWeek != want.PlayedThisWeek || e.PlayedThisWeekTeam != want.PlayedThisWeekTeam ||
			e.IsLockedToHigherTeam != want.IsLockedToHigherTeam || e.LockedToTeamName != want.LockedToTeamName ||
			e.RemainingHigherTeamPlays != want.RemainingHigherTeamPlays {
			t.Errorf("%s eligibility: got %+v", key, *e)
		}
		if e.Player.ID != got[key].Player.ID {
			t.Errorf("%s eligibility is for player %q", key, e.Player.ID)
		}
	}
}

// selectionFixtureID is St Ann's B (rank 2 of 3) at home on Tuesday of league
// week 15 in the seedTeamSelectionFixture database.
const selectionFixtureID = 500

// seedTeamSelectionFixture builds a migrated database with one fixture whose
// home-club players cover every availability source and eligibility rule.
func seedTeamSelectionFixture(t *testing.T) *database.DB {
	t.Helper()
	db, err := database.New(database.Config{Driver: "sqlite3", FilePath: filepath.Join(t.TempDir(), "selection.db")})
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

	// 18 one-week league weeks from Tuesday 14 April; week N is played on
	// the Tuesday N-1 weeks later.
	start := time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC)
	played := func(week int) time.Time { return start.AddDate(0, 0, (week-1)*7) }
	exec(`INSERT INTO seasons (id, name, year, start_date, end_date, is_active) VALUES (1, '2026', 2026, ?, ?, 1), (2, '2025', 2025, ?, ?, 0)`,
		start, played(18).AddDate(0, 0, 6), start.AddDate(-1, 0, 0), start.AddDate(-1, 5, 0))
	for w := 1; w <= 18; w++ {
		exec(`INSERT INTO weeks (id, week_number, season_id, start_date, end_date, name) VALUES (?, ?, 1, ?, ?, ?)`,
			w, w, played(w), played(w).AddDate(0, 0, 6), fmt.Sprintf("Week %d", w))
	}
	exec(`INSERT INTO leagues (id, name, type, year, region) VALUES (1, 'Parks', 'Parks', 2026, 'Brighton')`)
	exec(`INSERT INTO divisions (id, name, level, play_day, league_id, season_id) VALUES (1, 'Division 1', 1, 'Tuesday', 1, 1)`)
	exec(`INSERT INTO clubs (id, name) VALUES (1, 'St Ann''s'), (2, 'Rivals')`)
	exec(`INSERT INTO teams (id, name, club_id, division_id, season_id) VALUES
		(1, 'St Ann''s A', 1, 1, 1), (2, 'St Ann''s B', 1, 1, 1), (3, 'St Ann''s C', 1, 1, 1), (4, 'Rivals A', 2, 1, 1)`)
	exec(`INSERT INTO players (id, first_name, last_name, club_id) VALUES
		('p-fix', 'Fixture', 'Wins', 1), ('p-date', 'Date', 'Wins', 1), ('p-day', 'Weekday', 'Wins', 1),
		('p-none', 'No', 'Answer', 1), ('p-week', 'Played', 'Monday', 1), ('p-up', 'Played', 'Up', 1),
		('p-rival', 'Other', 'Club', 2)`)
	exec(`INSERT INTO player_teams (player_id, team_id, season_id, is_active) VALUES
		('p-fix', 2, 1, 1), ('p-date', 2, 1, 1), ('p-week', 2, 1, 1)`)

	const fixtureID = selectionFixtureID
	fixture := func(id, home, away, week int, date time.Time, status string) {
		exec(`INSERT INTO fixtures (id, home_team_id, away_team_id, division_id, season_id, week_id, scheduled_date, venue_location, status, notes)
			VALUES (?, ?, ?, 1, 1, ?, ?, '', ?, '')`, id, home, away, week, date, status)
	}
	fixture(fixtureID, 2, 4, 15, played(15), "Scheduled")

	// p-fix: fixture answer beats a date exception and a weekday pattern.
	exec(`INSERT INTO player_fixture_availability (player_id, fixture_id, status, notes) VALUES ('p-fix', ?, 'Available', 'fixture note')`, fixtureID)
	exec(`INSERT INTO player_availability_exceptions (player_id, status, start_date, end_date, reason) VALUES ('p-fix', 'Unavailable', ?, ?, 'away')`,
		played(15).AddDate(0, 0, -1), played(15).AddDate(0, 0, 1))
	exec(`INSERT INTO player_general_availability (player_id, day_of_week, status, season_id, notes) VALUES ('p-fix', 'Tuesday', 'Unavailable', 1, '')`)

	// p-date: the most recently created covering exception wins over the
	// weekday pattern; exceptions not covering the date are ignored.
	exec(`INSERT INTO player_availability_exceptions (player_id, status, start_date, end_date, reason, created_at) VALUES
		('p-date', 'Unavailable', ?, ?, 'older', '2026-05-01 10:00:00'),
		('p-date', 'IfNeeded', ?, ?, 'newer', '2026-06-01 10:00:00'),
		('p-date', 'Unavailable', ?, ?, 'next week', '2026-07-01 10:00:00')`,
		played(14), played(16), played(15), played(15), played(16), played(16))
	exec(`INSERT INTO player_general_availability (player_id, day_of_week, status, season_id, notes) VALUES ('p-date', 'Tuesday', 'Available', 1, '')`)

	// p-day: only the fixture's weekday in the fixture's season counts.
	exec(`INSERT INTO player_general_availability (player_id, day_of_week, status, season_id, notes) VALUES
		('p-day', 'Monday', 'Available', 1, ''), ('p-day', 'Tuesday', 'Unavailable', 1, 'busy'), ('p-day', 'Wednesday', 'Available', 1, ''),
		('p-none', 'Tuesday', 'Available', 2, '')`)

	// p-week: already playing for St Ann's A on the Monday of the same week.
	fixture(600, 1, 4, 15, played(15).AddDate(0, 0, -1), "Scheduled")
	exec(`INSERT INTO fixture_players (fixture_id, player_id, is_home, position, managing_team_id) VALUES (600, 'p-week', 1, 1, 1)`)

	// p-up: five completed second-half matches for St Ann's A (Rule 16 lock).
	for w := 10; w <= 14; w++ {
		fixture(700+w, 1, 4, w, played(w), "Completed")
		exec(`INSERT INTO matchups (id, fixture_id, type, status) VALUES (?, ?, 'Mens', 'Finished')`, 700+w, 700+w)
		exec(`INSERT INTO matchup_players (matchup_id, player_id, is_home) VALUES (?, 'p-up', 1)`, 700+w)
	}

	return db
}

func keys(m map[string]PlayerWithEligibility) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
