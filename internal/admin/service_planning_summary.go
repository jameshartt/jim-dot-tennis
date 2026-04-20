// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"context"
	"sort"
	"time"

	"jim-dot-tennis/internal/models"
)

// A parks-league fixture is 4 matchups: Mens (2M), Womens (2W), 1st Mixed
// (1M+1W), 2nd Mixed (1M+1W). Minimum bodies to field a fixture is therefore
// 4 men and 4 women. We flag 'short' below that threshold — no comfort band,
// captains can read 5/4 as 'one spare' without us spelling it out.
const (
	requiredMenPerFixture   = 4
	requiredWomenPerFixture = 4
)

// AvailabilitySummary lists, per team in scope, how many men and women have
// said they are Available for that team's week-N fixture. Used on the
// planning dashboard to answer 'do we have enough players this week?' at a
// glance, and to offer a direct 'Remind team' nudge when a team is short.
type AvailabilitySummary struct {
	Week  *models.Week
	Teams []TeamAvailabilityRow
}

// TeamAvailabilityRow is one team's week-level availability snapshot. A team
// with no fixture this week (bye) renders as HasFixture=false and is never
// flagged short.
type TeamAvailabilityRow struct {
	TeamID         uint
	TeamName       string
	DivisionLevel  int
	HasFixture     bool
	FixtureID      uint
	OpponentName   string
	ScheduledDate  time.Time
	IsHome         bool
	RequiredMen    int
	RequiredWomen  int
	AvailableMen   int
	AvailableWomen int
	// UnknownMen / UnknownWomen are the players who haven't set availability
	// yet — they're the ones the 'Remind' button can nudge.
	UnknownMen   int
	UnknownWomen int
	RosterMen    int
	RosterWomen  int
	ShortMen     bool
	ShortWomen   bool
}

// Short reports whether either gender is under the per-fixture requirement.
// Drives the amber banner + reminder CTA in the summary partial.
func (r TeamAvailabilityRow) Short() bool { return r.ShortMen || r.ShortWomen }

// BuildAvailabilitySummary tallies Available / Unknown counts per gender for
// each team in scope, resolved against that team's week-N fixture. Teams with
// no fixture this week are still reported (roster counts, HasFixture=false)
// so the list is stable across weeks.
func (s *Service) BuildAvailabilitySummary(ctx context.Context, scope *ResolvedScope, week *models.Week) (*AvailabilitySummary, error) {
	if scope == nil || week == nil {
		return &AvailabilitySummary{Week: week}, nil
	}

	summary := &AvailabilitySummary{Week: week}

	fixtures, err := s.fixtureRepository.FindByWeek(ctx, week.ID)
	if err != nil {
		return nil, err
	}
	fixtureByTeam := map[uint]*models.Fixture{}
	for i := range fixtures {
		f := &fixtures[i]
		fixtureByTeam[f.HomeTeamID] = f
		fixtureByTeam[f.AwayTeamID] = f
	}

	teamCache := map[uint]*models.Team{}
	divCache := map[uint]*models.Division{}

	for _, tID := range scope.TeamIDs {
		team, _ := loadTeam(ctx, s, tID, teamCache)
		if team == nil {
			continue
		}

		row := TeamAvailabilityRow{
			TeamID:        tID,
			TeamName:      team.Name,
			RequiredMen:   requiredMenPerFixture,
			RequiredWomen: requiredWomenPerFixture,
			DivisionLevel: 9999,
		}
		if div, _ := loadDivision(ctx, s, team.DivisionID, divCache); div != nil {
			row.DivisionLevel = div.Level
		}

		if f := fixtureByTeam[tID]; f != nil {
			row.HasFixture = true
			row.FixtureID = f.ID
			row.ScheduledDate = f.ScheduledDate
			row.IsHome = f.HomeTeamID == tID
			oppID := f.AwayTeamID
			if !row.IsHome {
				oppID = f.HomeTeamID
			}
			if opp, _ := loadTeam(ctx, s, oppID, teamCache); opp != nil {
				row.OpponentName = opp.Name
			}
		}

		roster, err := s.teamRepository.FindPlayersInTeam(ctx, tID, week.SeasonID)
		if err != nil {
			summary.Teams = append(summary.Teams, row)
			continue
		}
		for _, pt := range roster {
			if !pt.IsActive {
				continue
			}
			player, err := s.playerRepository.FindByID(ctx, pt.PlayerID)
			if err != nil || player == nil || !player.IsActive {
				continue
			}
			switch player.Gender {
			case models.PlayerGenderMen:
				row.RosterMen++
			case models.PlayerGenderWomen:
				row.RosterWomen++
			}
			if !row.HasFixture {
				continue
			}
			cell := resolveCell(ctx, s, player.ID, fixtureByTeam[tID])
			switch cell.Status {
			case models.Available:
				switch player.Gender {
				case models.PlayerGenderMen:
					row.AvailableMen++
				case models.PlayerGenderWomen:
					row.AvailableWomen++
				}
			case models.Unknown:
				switch player.Gender {
				case models.PlayerGenderMen:
					row.UnknownMen++
				case models.PlayerGenderWomen:
					row.UnknownWomen++
				}
			}
		}

		if row.HasFixture {
			row.ShortMen = row.AvailableMen < row.RequiredMen
			row.ShortWomen = row.AvailableWomen < row.RequiredWomen
		}

		summary.Teams = append(summary.Teams, row)
	}

	sort.Slice(summary.Teams, func(i, j int) bool {
		a, b := summary.Teams[i], summary.Teams[j]
		if a.DivisionLevel != b.DivisionLevel {
			return a.DivisionLevel < b.DivisionLevel
		}
		return a.TeamName < b.TeamName
	})

	return summary, nil
}
