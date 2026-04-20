// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"jim-dot-tennis/internal/models"
)

// PlanningMatrix is the week-at-a-glance decision surface. Columns are one
// team's perspective on a fixture, so a derby between two St Ann's teams
// produces two columns — one per team — each with its own selection state.
type PlanningMatrix struct {
	Week         *models.Week
	Columns      []*MatrixColumn
	ColumnGroups []*MatrixColumnGroup
	Rows         []*MatrixRow
	EmptyReason  string
}

// MatrixColumn is one column: a team's perspective on a fixture.
type MatrixColumn struct {
	Fixture           *models.Fixture
	PerspectiveTeamID uint
	TeamName          string
	OpponentName      string
	DivisionName      string
	IsHome            bool // is PerspectiveTeamID playing at home in Fixture?
	Key               string
}

// MatrixColumnGroup is the super-header entry for one team; ColumnCount is
// the colspan covering that team's consecutive fixture columns.
type MatrixColumnGroup struct {
	TeamID      uint
	TeamName    string
	ColumnCount int
}

// MatrixRow is one player row with cells keyed by MatrixColumn.Key.
type MatrixRow struct {
	Player      *models.Player
	Preferences *models.PlayerTennisPreferences
	Cells       map[string]MatrixCell
	// PrimaryDivisionLevel / PrimaryTeamName rank the row by the player's
	// highest team: 1st-team players sort before 2nd-team, and so on. This
	// mirrors the real picking order — you pick the 1st team first, then
	// cascade down. Lower division level = higher team.
	PrimaryDivisionLevel int
	PrimaryTeamName      string
	// CaptainNoteCount powers the in-row note icon. Zero means the icon
	// renders in a 'no notes yet' state; non-zero shows the count.
	CaptainNoteCount int
}

// MatrixCell is availability + selection state for one (player, column).
type MatrixCell struct {
	FixtureID       uint
	ColumnKey       string
	Status          models.AvailabilityStatus
	Reason          string
	InTeamSelection bool // whether this player is in the fixture_players pool for this column
}

// ResolveAvailabilityMatrix assembles the dashboard matrix for the scope + week.
//
// Performance note: for the typical home-club scale (5 teams × ~25 players,
// one week of fixtures) this runs fine as a straightforward query-per-player
// loop against the existing repos. If scope='all' on a larger club starts
// showing latency, the follow-up is a single bulk join.
func (s *Service) ResolveAvailabilityMatrix(ctx context.Context, scope *ResolvedScope, week *models.Week, filters MatrixFilters) (*PlanningMatrix, error) {
	if week == nil {
		return &PlanningMatrix{EmptyReason: "No week selected"}, nil
	}
	if scope == nil || len(scope.TeamIDs) == 0 {
		return &PlanningMatrix{Week: week, EmptyReason: "No teams in scope"}, nil
	}

	matrix := &PlanningMatrix{Week: week}

	// ---------- Columns: one per (team-in-scope, fixture). Derby fixtures
	// with both sides in scope emit two columns. ----------
	rawFixtures, err := s.fixtureRepository.FindByWeek(ctx, week.ID)
	if err != nil {
		return nil, err
	}
	teamInScope := make(map[uint]bool, len(scope.TeamIDs))
	for _, id := range scope.TeamIDs {
		teamInScope[id] = true
	}

	teamByID := map[uint]*models.Team{}
	divByID := map[uint]*models.Division{}

	for i := range rawFixtures {
		f := &rawFixtures[i]
		homeInScope := teamInScope[f.HomeTeamID]
		awayInScope := teamInScope[f.AwayTeamID]
		if !homeInScope && !awayInScope {
			continue
		}
		home, _ := loadTeam(ctx, s, f.HomeTeamID, teamByID)
		away, _ := loadTeam(ctx, s, f.AwayTeamID, teamByID)
		div, _ := loadDivision(ctx, s, f.DivisionID, divByID)

		homeName, awayName, divName := "", "", ""
		if home != nil {
			homeName = home.Name
		}
		if away != nil {
			awayName = away.Name
		}
		if div != nil {
			divName = div.Name
		}

		if homeInScope {
			matrix.Columns = append(matrix.Columns, &MatrixColumn{
				Fixture:           f,
				PerspectiveTeamID: f.HomeTeamID,
				TeamName:          homeName,
				OpponentName:      awayName,
				DivisionName:      divName,
				IsHome:            true,
				Key:               fmt.Sprintf("%d-%d", f.ID, f.HomeTeamID),
			})
		}
		if awayInScope {
			matrix.Columns = append(matrix.Columns, &MatrixColumn{
				Fixture:           f,
				PerspectiveTeamID: f.AwayTeamID,
				TeamName:          awayName,
				OpponentName:      homeName,
				DivisionName:      divName,
				IsHome:            false,
				Key:               fmt.Sprintf("%d-%d", f.ID, f.AwayTeamID),
			})
		}
	}
	// Team-first ordering: group by TeamName, then by date within each team.
	sort.Slice(matrix.Columns, func(i, j int) bool {
		a, b := matrix.Columns[i], matrix.Columns[j]
		if a.TeamName != b.TeamName {
			return a.TeamName < b.TeamName
		}
		return a.Fixture.ScheduledDate.Before(b.Fixture.ScheduledDate)
	})

	// Collapse consecutive same-team columns into groups for the super-header.
	for _, col := range matrix.Columns {
		if n := len(matrix.ColumnGroups); n > 0 && matrix.ColumnGroups[n-1].TeamID == col.PerspectiveTeamID {
			matrix.ColumnGroups[n-1].ColumnCount++
			continue
		}
		matrix.ColumnGroups = append(matrix.ColumnGroups, &MatrixColumnGroup{
			TeamID:      col.PerspectiveTeamID,
			TeamName:    col.TeamName,
			ColumnCount: 1,
		})
	}

	if len(matrix.Columns) == 0 {
		matrix.EmptyReason = "No fixtures in this week for the selected scope"
		return matrix, nil
	}

	// ---------- Selection state: prefetch fixture_players for all
	// in-scope fixtures so each cell knows whether its player is already in
	// that column's team-selection pool. ----------
	selectionByColumn := map[string]map[string]bool{}
	seenFixture := map[uint]bool{}
	for _, col := range matrix.Columns {
		if seenFixture[col.Fixture.ID] {
			continue
		}
		seenFixture[col.Fixture.ID] = true
		players, err := s.fixtureRepository.FindSelectedPlayers(ctx, col.Fixture.ID)
		if err != nil {
			continue
		}
		for i := range players {
			fp := &players[i]
			key := selectionColumnKey(col.Fixture, fp)
			if selectionByColumn[key] == nil {
				selectionByColumn[key] = map[string]bool{}
			}
			selectionByColumn[key][fp.PlayerID] = true
		}
	}

	// ---------- Rows: eligible players across in-scope teams ----------
	playerByID := map[string]*models.Player{}
	// primaryTeam[pid] is the player's highest-ranked in-scope team — lower
	// divisionLevel wins, with team name as the tiebreaker (A < B < C …).
	// Drives row sort so 1st-team players appear before 2nd-team and so on.
	type teamRank struct {
		divisionLevel int
		teamName      string
	}
	primaryTeam := map[string]teamRank{}

	for _, tID := range scope.TeamIDs {
		playerTeamRows, err := s.teamRepository.FindPlayersInTeam(ctx, tID, week.SeasonID)
		if err != nil {
			continue
		}
		team, _ := loadTeam(ctx, s, tID, teamByID)
		level := 9999
		name := ""
		if team != nil {
			name = team.Name
			if div, _ := loadDivision(ctx, s, team.DivisionID, divByID); div != nil {
				level = div.Level
			}
		}
		for _, pt := range playerTeamRows {
			if !pt.IsActive {
				continue
			}
			player, err := s.playerRepository.FindByID(ctx, pt.PlayerID)
			if err != nil || player == nil || !player.IsActive {
				continue
			}
			playerByID[player.ID] = player
			candidate := teamRank{divisionLevel: level, teamName: name}
			if existing, had := primaryTeam[player.ID]; !had ||
				candidate.divisionLevel < existing.divisionLevel ||
				(candidate.divisionLevel == existing.divisionLevel && candidate.teamName < existing.teamName) {
				primaryTeam[player.ID] = candidate
			}
		}
	}

	// Attach preferences + build cells.
	for pid, player := range playerByID {
		prefs, _ := s.tennisPreferenceRepository.FindByPlayerID(ctx, pid)

		if !matchFilters(prefs, filters) {
			continue
		}

		row := &MatrixRow{
			Player:               player,
			Preferences:          prefs,
			Cells:                map[string]MatrixCell{},
			PrimaryDivisionLevel: primaryTeam[pid].divisionLevel,
			PrimaryTeamName:      primaryTeam[pid].teamName,
		}

		// Resolve availability once per fixture even if multiple columns
		// (derby) reference the same fixture — the status is the same for
		// both perspectives.
		availByFixture := map[uint]MatrixCell{}
		for _, col := range matrix.Columns {
			base, ok := availByFixture[col.Fixture.ID]
			if !ok {
				base = resolveCell(ctx, s, pid, col.Fixture)
				availByFixture[col.Fixture.ID] = base
			}
			cell := base
			cell.ColumnKey = col.Key
			cell.InTeamSelection = selectionByColumn[col.Key][pid]
			row.Cells[col.Key] = cell
		}

		matrix.Rows = append(matrix.Rows, row)
	}

	// Attach captain-note counts — one query for the whole row set.
	if len(matrix.Rows) > 0 {
		pidList := make([]string, 0, len(matrix.Rows))
		for _, row := range matrix.Rows {
			pidList = append(pidList, row.Player.ID)
		}
		if counts, err := s.captainNoteRepository.CountsByPlayer(ctx, pidList); err == nil {
			for _, row := range matrix.Rows {
				row.CaptainNoteCount = counts[row.Player.ID]
			}
		}
	}

	// Row sort mirrors the pick order: 1st-team players first (lowest
	// division level), then cascading down through 2nd, 3rd, … teams. Names
	// break ties inside a team.
	sort.Slice(matrix.Rows, func(i, j int) bool {
		a, b := matrix.Rows[i], matrix.Rows[j]
		if a.PrimaryDivisionLevel != b.PrimaryDivisionLevel {
			return a.PrimaryDivisionLevel < b.PrimaryDivisionLevel
		}
		if a.PrimaryTeamName != b.PrimaryTeamName {
			return a.PrimaryTeamName < b.PrimaryTeamName
		}
		if a.Player.LastName != b.Player.LastName {
			return a.Player.LastName < b.Player.LastName
		}
		return a.Player.FirstName < b.Player.FirstName
	})

	return matrix, nil
}

// resolveCell picks the most specific availability signal for (player, fixture):
//  1. A fixture-specific override, if any
//  2. A date-range exception covering the scheduled date
//  3. The player's general availability for that day of week
//
// Missing → Unknown.
func resolveCell(ctx context.Context, s *Service, playerID string, fixture *models.Fixture) MatrixCell {
	cell := MatrixCell{FixtureID: fixture.ID, Status: models.Unknown}

	if f, err := s.availabilityRepository.GetPlayerFixtureAvailability(ctx, playerID, fixture.ID); err == nil && f != nil {
		cell.Status = f.Status
		cell.Reason = f.Notes
		return cell
	}
	if e, err := s.availabilityRepository.GetPlayerAvailabilityByDate(ctx, playerID, fixture.ScheduledDate); err == nil && e != nil {
		cell.Status = e.Status
		cell.Reason = e.Reason
		return cell
	}
	general, err := s.availabilityRepository.GetPlayerGeneralAvailability(ctx, playerID, fixture.SeasonID)
	if err == nil {
		dayName := fixture.ScheduledDate.Weekday().String()
		for _, g := range general {
			if g.DayOfWeek == dayName {
				cell.Status = g.Status
				cell.Reason = g.Notes
				return cell
			}
		}
	}
	return cell
}

func matchFilters(prefs *models.PlayerTennisPreferences, f MatrixFilters) bool {
	if f.OpenToFillInOnly {
		if prefs == nil || prefs.OpenToFillIn == nil || !*prefs.OpenToFillIn {
			return false
		}
	}
	if len(f.Handedness) > 0 {
		if prefs == nil || prefs.Handedness == nil || !contains(f.Handedness, *prefs.Handedness) {
			return false
		}
	}
	if len(f.CourtSide) > 0 {
		if prefs == nil || prefs.PreferredCourtSide == nil || !contains(f.CourtSide, *prefs.PreferredCourtSide) {
			return false
		}
	}
	if len(f.MixedAppetite) > 0 {
		if prefs == nil || prefs.MixedDoublesAppetite == nil || !contains(f.MixedAppetite, *prefs.MixedDoublesAppetite) {
			return false
		}
	}
	if len(f.SameGenderAppetite) > 0 {
		if prefs == nil || prefs.SameGenderDoublesAppetite == nil || !contains(f.SameGenderAppetite, *prefs.SameGenderDoublesAppetite) {
			return false
		}
	}
	if f.MinCompetitiveness > 1 || f.MaxCompetitiveness < 5 {
		if prefs == nil || prefs.Competitiveness == nil {
			return false
		}
		c := *prefs.Competitiveness
		if c < f.MinCompetitiveness || c > f.MaxCompetitiveness {
			return false
		}
	}
	return true
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if strings.EqualFold(h, needle) {
			return true
		}
	}
	return false
}

// selectionColumnKey picks the right column for a fixture_players row: in
// derby matches the managing_team_id is set and points directly at the column,
// in regular fixtures we fall back to home/away team IDs on the fixture.
func selectionColumnKey(f *models.Fixture, fp *models.FixturePlayer) string {
	var teamID uint
	if fp.ManagingTeamID != nil && *fp.ManagingTeamID != 0 {
		teamID = *fp.ManagingTeamID
	} else if fp.IsHome {
		teamID = f.HomeTeamID
	} else {
		teamID = f.AwayTeamID
	}
	return fmt.Sprintf("%d-%d", f.ID, teamID)
}

func loadTeam(ctx context.Context, s *Service, id uint, cache map[uint]*models.Team) (*models.Team, error) {
	if t, ok := cache[id]; ok {
		return t, nil
	}
	t, err := s.teamRepository.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	cache[id] = t
	return t, nil
}

func loadDivision(ctx context.Context, s *Service, id uint, cache map[uint]*models.Division) (*models.Division, error) {
	if d, ok := cache[id]; ok {
		return d, nil
	}
	d, err := s.divisionRepository.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	cache[id] = d
	return d, nil
}
