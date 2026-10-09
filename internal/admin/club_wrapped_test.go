// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"jim-dot-tennis/internal/config"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"

	_ "github.com/mattn/go-sqlite3"
)

// newWrappedTestHandler seeds two seasons of St Ann's results:
//   - 2025 (inactive): Olive Old + Oscar Old play three finished matchups
//     across two completed fixtures (so they would top any all-time list).
//   - 2026 (active): Nina Now + Ned Now play nine finished matchups across
//     nine completed fixtures.
func newWrappedTestHandler(t *testing.T) (*ClubWrappedHandler, context.Context) {
	t.Helper()
	db, err := database.New(database.Config{Driver: "sqlite3", FilePath: filepath.Join(t.TempDir(), "wrapped.db")})
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
	d := func(y int, m time.Month, day int) time.Time { return time.Date(y, m, day, 18, 0, 0, 0, time.UTC) }

	exec(`INSERT INTO clubs (id, name) VALUES (1, 'St Ann''s'), (2, 'Rivals')`)
	exec(`INSERT INTO leagues (id, name, type, year, region) VALUES (1, 'Parks', 'Parks', 2025, 'Brighton')`)
	exec(`INSERT INTO players (id, first_name, last_name, club_id) VALUES
		('p-old1', 'Olive', 'Old', 1), ('p-old2', 'Oscar', 'Old', 1),
		('p-now1', 'Nina', 'Now', 1), ('p-now2', 'Ned', 'Now', 1)`)

	for _, s := range []struct {
		id, year int
		active   bool
	}{{1, 2025, false}, {2, 2026, true}} {
		exec(`INSERT INTO seasons (id, name, year, start_date, end_date, is_active) VALUES (?, ?, ?, ?, ?, ?)`,
			s.id, "Season", s.year, d(s.year, 4, 1), d(s.year, 9, 30), s.active)
		exec(`INSERT INTO weeks (id, week_number, season_id, start_date, end_date) VALUES (?, 1, ?, ?, ?)`,
			s.id, s.id, d(s.year, 4, 1), d(s.year, 9, 30))
		exec(`INSERT INTO divisions (id, name, level, play_day, league_id, season_id) VALUES (?, 'Division 1', 1, 'Tuesday', 1, ?)`, s.id, s.id)
		exec(`INSERT INTO teams (id, name, club_id, division_id, season_id) VALUES (?, 'St Ann''s A', 1, ?, ?), (?, 'Rivals A', 2, ?, ?)`,
			s.id*10+1, s.id, s.id, s.id*10+2, s.id, s.id)
	}

	fixture := func(id, season, year int) {
		exec(`INSERT INTO fixtures (id, home_team_id, away_team_id, division_id, season_id, week_id, scheduled_date, venue_location, status)
			VALUES (?, ?, ?, ?, ?, ?, ?, 'Away Park', 'Completed')`, id, season*10+1, season*10+2, season, season, season, d(year, 6, 1))
	}
	matchup := func(id, fixtureID int, mtype string, players ...string) {
		exec(`INSERT INTO matchups (id, fixture_id, type, status, home_score, away_score, home_set1, away_set1, home_set2, away_set2, notes)
			VALUES (?, ?, ?, 'Finished', 2, 0, 6, 1, 6, 2, '')`, id, fixtureID, mtype)
		for _, p := range players {
			exec(`INSERT INTO matchup_players (matchup_id, player_id, is_home) VALUES (?, ?, 1)`, id, p)
		}
	}
	fixture(100, 1, 2025)
	fixture(101, 1, 2025)
	matchup(1001, 100, "Mens", "p-old1", "p-old2")
	matchup(1002, 100, "1st Mixed", "p-old1", "p-old2")
	matchup(1011, 101, "Mens", "p-old1", "p-old2")
	// Nine 2026 straight-set wins, enough to clear every leaderboard threshold
	// that a single pairing can reach (style, grinders, win %, pairings,
	// butterflies, dominating winners).
	for i := 0; i < 9; i++ {
		fixture(200+i, 2, 2026)
		matchup(2001+i, 200+i, "Mens", "p-now1", "p-now2")
	}

	ctx := config.WithHomeClub(context.Background(), 1, &models.Club{ID: 1, Name: "St Ann's"}, "")
	return NewClubWrappedHandler(NewService(db, "", 1, ""), ""), ctx
}

// Wrapped is a per-season recap: once a second season has results, stats
// must cover the active season only, not every season ever played.
func TestClubWrappedScopedToActiveSeason(t *testing.T) {
	h, ctx := newWrappedTestHandler(t)

	data, err := h.generateClubWrappedData(ctx)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if data.SeasonYear != 2026 {
		t.Errorf("SeasonYear: got %d, want 2026 (the active season)", data.SeasonYear)
	}
	o := data.OverallStats
	if o.TotalMatchups != 9 || o.TotalFixtures != 9 || o.PlayersUsed != 2 {
		t.Errorf("overall stats: got matchups=%d fixtures=%d players=%d, want 9/9/2",
			o.TotalMatchups, o.TotalFixtures, o.PlayersUsed)
	}
	if o.MostActivePlayer.MatchesCount != 9 || !strings.HasSuffix(o.MostActivePlayer.Name, "Now") {
		t.Errorf("most active player: got %+v, want a 2026 player with 9 matches", o.MostActivePlayer)
	}
	if b := data.FixtureBreakdown; b.TotalFixtures != 9 || b.Perfect_8_0 != 0 {
		t.Errorf("fixture breakdown: got total=%d, want 9", b.TotalFixtures)
	}

	// Each leaderboard the 2026 pair qualifies for must be populated: an empty
	// list here means its query failed (e.g. a season argument out of place).
	boards := map[string]int{
		"MensOnlyPlayers":   len(data.PlayingStylePlayers.MensOnlyPlayers),
		"GameGrinders":      len(data.GameGrinders),
		"TopWinPercentage":  len(data.TopWinPercentage),
		"TopPairings":       len(data.TopPairings),
		"SocialButterflies": len(data.SocialButterflies),
		"DominatingWinners": len(data.DominatingWinners),
	}
	for name, n := range boards {
		if n != 2 && !(name == "TopPairings" && n == 1) {
			t.Errorf("%s: got %d entries, want the 2026 pair", name, n)
		}
	}
	if data.AvailabilityEngagement.PlayersWhoPlayed != 2 {
		t.Errorf("availability engagement: got %d players who played, want 2", data.AvailabilityEngagement.PlayersWhoPlayed)
	}

	blob, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, name := range []string{"Olive", "Oscar"} {
		if strings.Contains(string(blob), name) {
			t.Errorf("2025-only player %q appears in the 2026 wrapped", name)
		}
	}
}

func TestPersonalWrappedScopedToActiveSeason(t *testing.T) {
	h, ctx := newWrappedTestHandler(t)

	now := h.getPersonalWrappedData(ctx, 2, "p-now1")
	if now == nil || now.MatchesPlayed != 9 || now.FixturesPlayed != 9 || now.LongestWinStreak != 9 || now.UniquePartners != 1 {
		t.Fatalf("2026 player: got %+v, want 9 matches in 9 fixtures, 9-win streak, 1 partner", now)
	}
	old := h.getPersonalWrappedData(ctx, 2, "p-old1")
	if old != nil && (old.MatchesPlayed != 0 || old.FixturesPlayed != 0) {
		t.Fatalf("2025-only player: got %d matches in %d fixtures this season, want 0",
			old.MatchesPlayed, old.FixturesPlayed)
	}
}
