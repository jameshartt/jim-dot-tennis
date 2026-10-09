// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"

	_ "github.com/mattn/go-sqlite3"
)

var cardDate = time.Date(2026, 6, 2, 18, 0, 0, 0, time.UTC)

// newImportTestService seeds St Ann's (club 1) teams A (1) and B (3), Rivals
// A (2, club 2), and two fixtures on cardDate:
//   - 500: St Ann's A vs Rivals A (regular)
//   - 501: St Ann's A vs St Ann's B (derby)
//
// Players "Hal Home*" play home, "Ava Away*" play away.
func newImportTestService(t *testing.T) (*MatchCardService, *database.DB) {
	t.Helper()
	db, err := database.New(database.Config{Driver: "sqlite3", FilePath: filepath.Join(t.TempDir(), "import.db")})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.ExecuteMigrations(findMigrationsPath(t)); err != nil {
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
	exec(`INSERT INTO clubs (id, name, address, website, phone_number) VALUES (1, 'St Ann''s', '', '', ''), (2, 'Rivals', '', '', '')`)
	exec(`INSERT INTO teams (id, name, club_id, division_id, season_id) VALUES
		(1, 'St Ann''s A', 1, 1, 1), (2, 'Rivals A', 2, 1, 1), (3, 'St Ann''s B', 1, 1, 1)`)
	for _, f := range []struct{ id, home, away int }{{500, 1, 2}, {501, 1, 3}} {
		exec(`INSERT INTO fixtures (id, home_team_id, away_team_id, division_id, season_id, week_id, scheduled_date, venue_location, status, notes)
			VALUES (?, ?, ?, 1, 1, 1, ?, '', 'Scheduled', '')`, f.id, f.home, f.away, cardDate)
	}
	for _, name := range []string{"Hal Homeone", "Hana Hometwo", "Ava Awayone", "Abe Awaytwo"} {
		parts := strings.Fields(name)
		exec(`INSERT INTO players (id, first_name, last_name, club_id) VALUES (?, ?, ?, 1)`, "p-"+strings.ToLower(parts[1]), parts[0], parts[1])
	}

	svc := NewMatchCardService(
		repository.NewFixtureRepository(db), repository.NewMatchupRepository(db), repository.NewTeamRepository(db),
		repository.NewClubRepository(db), repository.NewPlayerRepository(db), repository.NewDivisionRepository(db),
		repository.NewSeasonRepository(db), 1,
	)
	return svc, db
}

func findMigrationsPath(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 6; i++ {
		if info, err := os.Stat(filepath.Join(dir, "migrations")); err == nil && info.IsDir() {
			return filepath.Join(dir, "migrations")
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("could not locate migrations directory")
	return ""
}

// card builds a four-rubber match card covering each scoring rule: a straight
// win, a concession, a halved match with split sets, and a retirement.
func card(id int, home, away string, mensSplit bool) MatchCardData {
	mensSet2 := [2]int{6, 4}
	if mensSplit {
		mensSet2 = [2]int{3, 6}
	}
	hp := []string{"Hal Homeone", "Hana Hometwo"}
	ap := []string{"Ava Awayone", "Abe Awaytwo"}
	return MatchCardData{
		ExternalID: id, HomeTeam: home, AwayTeam: away, EventDate: cardDate,
		Matchups: []MatchupData{
			{Type: "Men's", HomePlayers: hp, AwayPlayers: ap, HomeScores: []int{6, mensSet2[0]}, AwayScores: []int{4, mensSet2[1]}},
			{Type: "Ladies'", HomePlayers: hp, AwayPlayers: ap, ConcededBy: "Away"},
			{Type: "First mixed", HomePlayers: hp, AwayPlayers: ap, HomeScores: []int{6, 4}, AwayScores: []int{4, 6}, Halved: true},
			{Type: "Second mixed", HomePlayers: hp, AwayPlayers: ap, HomeScores: []int{3}, AwayScores: []int{2}, RetiredBy: "Home"},
		},
	}
}

type rubber struct {
	Type      models.MatchupType
	Status    models.MatchupStatus
	Home      int
	Away      int
	Conceded  string
	Retired   string
	Set1      string
	Players   string // "player_id:is_home,..." sorted
	ManagedBy uint
}

func loadRubbers(t *testing.T, db *database.DB, fixtureID int) map[string]rubber {
	t.Helper()
	var rows []struct {
		ID        uint   `db:"id"`
		Type      string `db:"type"`
		Status    string `db:"status"`
		Home      int    `db:"home_score"`
		Away      int    `db:"away_score"`
		Conceded  string `db:"conceded"`
		Retired   string `db:"retired"`
		Set1      string `db:"set1"`
		ManagedBy uint   `db:"managing_team_id"`
		Players   string `db:"players"`
	}
	if err := db.Select(&rows, `
		SELECT m.id, m.type, m.status, m.home_score, m.away_score,
			COALESCE(m.conceded_by, '') AS conceded, COALESCE(m.retired_by, '') AS retired,
			COALESCE(m.home_set1 || '-' || m.away_set1, '') AS set1, m.managing_team_id,
			COALESCE((SELECT GROUP_CONCAT(player_id || ':' || is_home, ',')
				FROM (SELECT player_id, is_home FROM matchup_players WHERE matchup_id = m.id ORDER BY player_id)), '') AS players
		FROM matchups m WHERE m.fixture_id = ?`, fixtureID); err != nil {
		t.Fatalf("load matchups: %v", err)
	}
	out := map[string]rubber{}
	for _, r := range rows {
		key := fmt.Sprintf("%s@%d", r.Type, r.ManagedBy)
		if _, dup := out[key]; dup {
			t.Fatalf("duplicate matchup %s", key)
		}
		out[key] = rubber{models.MatchupType(r.Type), models.MatchupStatus(r.Status), r.Home, r.Away, r.Conceded, r.Retired, r.Set1, r.Players, r.ManagedBy}
	}
	return out
}

func rosters(t *testing.T, db *database.DB) []string {
	t.Helper()
	var rows []string
	if err := db.Select(&rows, `SELECT player_id || '@' || team_id FROM player_teams WHERE season_id = 1 ORDER BY 1`); err != nil {
		t.Fatalf("rosters: %v", err)
	}
	return rows
}

const (
	homePlayers = "p-homeone:1,p-hometwo:1"
	awayPlayers = "p-awayone:0,p-awaytwo:0"
	allPlayers  = "p-awayone:0,p-awaytwo:0,p-homeone:1,p-hometwo:1"
)

// checkScores asserts the scoring the four-rubber card must produce, shared by
// the regular and derby paths.
func checkScores(t *testing.T, got map[string]rubber, teamID uint, mensSplit bool) {
	t.Helper()
	want := []rubber{
		{Type: models.Mens, Status: models.Finished, Home: 2, Away: 0, Set1: "6-4"},
		{Type: models.Womens, Status: models.Defaulted, Home: 2, Away: 0, Conceded: "Away"},
		{Type: models.FirstMixed, Status: models.Finished, Home: 1, Away: 1, Set1: "6-4"},
		{Type: models.SecondMixed, Status: models.Finished, Home: 0, Away: 2, Retired: "Home", Set1: "3-2"},
	}
	if mensSplit {
		want[0].Home, want[0].Away = 1, 1
	}
	for _, w := range want {
		g, ok := got[fmt.Sprintf("%s@%d", w.Type, teamID)]
		if !ok {
			t.Errorf("team %d: missing %s matchup", teamID, w.Type)
			continue
		}
		if g.Status != w.Status || g.Home != w.Home || g.Away != w.Away || g.Conceded != w.Conceded || g.Retired != w.Retired || g.Set1 != w.Set1 {
			t.Errorf("team %d %s: got %s %d-%d conceded=%q retired=%q set1=%q, want %s %d-%d conceded=%q retired=%q set1=%q",
				teamID, w.Type, g.Status, g.Home, g.Away, g.Conceded, g.Retired, g.Set1,
				w.Status, w.Home, w.Away, w.Conceded, w.Retired, w.Set1)
		}
	}
}

func importCard(t *testing.T, svc *MatchCardService, c MatchCardData, clear bool) *ImportResult {
	t.Helper()
	res, err := svc.processMatchCard(context.Background(), ImportConfig{ClearExistingMatchups: clear}, c)
	if err != nil {
		t.Fatalf("import card %d: %v", c.ExternalID, err)
	}
	if len(res.Errors) > 0 || len(res.UnmatchedPlayers) > 0 {
		t.Fatalf("import card %d: errors=%v unmatched=%v", c.ExternalID, res.Errors, res.UnmatchedPlayers)
	}
	return res
}

// Behaviour lock: a regular fixture gets one slate, managed by the home-club
// team, with both sides' players; only home-club players join a roster.
func TestImportRegularMatchCard(t *testing.T) {
	svc, db := newImportTestService(t)

	for _, clear := range []bool{false, true} {
		res := importCard(t, svc, card(7000, "St Ann's A", "Rivals A", false), clear)
		if res.MatchedPlayers != 16 {
			t.Errorf("clear=%v: matched %d players, want 16", clear, res.MatchedPlayers)
		}
		got := loadRubbers(t, db, 500)
		if len(got) != 4 {
			t.Fatalf("clear=%v: got %d matchups, want 4: %v", clear, len(got), got)
		}
		checkScores(t, got, 1, false)
		for k, r := range got {
			if r.Players != allPlayers {
				t.Errorf("clear=%v %s: players %q, want %q", clear, k, r.Players, allPlayers)
			}
		}
	}
	// Re-import without clearing updates in place.
	importCard(t, svc, card(7000, "St Ann's A", "Rivals A", true), false)
	checkScores(t, loadRubbers(t, db, 500), 1, true)

	if got := rosters(t, db); strings.Join(got, ",") != "p-homeone@1,p-hometwo@1" {
		t.Errorf("rosters: got %v, want only home players on team 1", got)
	}
}

// Behaviour lock: a derby gets one slate per St Ann's team, each with only
// that team's players, and each side joins its own team's roster.
func TestImportDerbyMatchCard(t *testing.T) {
	svc, db := newImportTestService(t)

	importCard(t, svc, card(7001, "St Ann's A", "St Ann's B", false), true)
	got := loadRubbers(t, db, 501)
	if len(got) != 8 {
		t.Fatalf("got %d matchups, want 8 (4 per team): %v", len(got), got)
	}
	checkScores(t, got, 1, false)
	checkScores(t, got, 3, false)
	for k, r := range got {
		want := homePlayers
		if r.ManagedBy == 3 {
			want = awayPlayers
		}
		if r.Players != want {
			t.Errorf("%s: players %q, want %q", k, r.Players, want)
		}
	}
	if got := rosters(t, db); strings.Join(got, ",") != "p-awayone@3,p-awaytwo@3,p-homeone@1,p-hometwo@1" {
		t.Errorf("rosters: got %v", got)
	}
}

// Re-importing a derby card without "clear existing" must update both slates
// like a regular card does, not fail on the existing matchups.
func TestImportDerbyMatchCardReimportUpdates(t *testing.T) {
	svc, db := newImportTestService(t)

	importCard(t, svc, card(7001, "St Ann's A", "St Ann's B", false), false)
	importCard(t, svc, card(7001, "St Ann's A", "St Ann's B", true), false)
	got := loadRubbers(t, db, 501)
	if len(got) != 8 {
		t.Fatalf("got %d matchups, want 8: %v", len(got), got)
	}
	checkScores(t, got, 1, true)
	checkScores(t, got, 3, true)
}
