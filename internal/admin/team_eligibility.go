package admin

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"jim-dot-tennis/internal/models"
)

// PlayerEligibilityInfo contains information about a player's eligibility for a team
type PlayerEligibilityInfo struct {
	Player                   models.Player
	CanPlay                  bool
	RemainingHigherTeamPlays int    // How many times they can still play for higher teams before lock-in (-1 if not applicable)
	IsLockedToHigherTeam     bool   // Whether they're locked to a higher team
	LockedToTeamName         string // Name of the team they're locked to
	PlayedThisWeek           bool   // Whether they've already played this week
	PlayedThisWeekTeam       string // Which team they played for this week
	NeedsWarning             bool   // Whether to show a warning (2 or fewer remaining)
	IsTopTeam                bool   // Whether this is the top-ranked team (no higher teams)
	IsLocked                 bool   // Whether player is locked to this team or higher
	CanPlayLower             bool   // Whether player can play for lower teams
}

// TeamRank represents the ranking of a team based on alphabetical order
type TeamRank struct {
	Team models.Team
	Rank int // Lower number = higher team (1 is highest)
}

// TeamEligibilityService handles team eligibility rules
type TeamEligibilityService struct {
	service *Service
}

// NewTeamEligibilityService creates a new team eligibility service
func NewTeamEligibilityService(service *Service) *TeamEligibilityService {
	return &TeamEligibilityService{
		service: service,
	}
}

// GetTeamRanking returns teams ranked by alphabetical order (St Ann's, St Ann's A, St Ann's B, etc.)
func (s *TeamEligibilityService) GetTeamRanking(ctx context.Context, clubID uint, seasonID uint) ([]TeamRank, error) {
	// Get all teams for the club and season
	teams, err := s.service.teamRepository.FindByClubAndSeason(ctx, clubID, seasonID)
	if err != nil {
		return nil, err
	}

	// Sort teams alphabetically by name
	sort.Slice(teams, func(i, j int) bool {
		return teams[i].Name < teams[j].Name
	})

	// Assign ranks (1-based, where 1 is the highest team)
	var rankings []TeamRank
	for i, team := range teams {
		rankings = append(rankings, TeamRank{
			Team: team,
			Rank: i + 1,
		})
	}

	return rankings, nil
}

// GetPlayerEligibilityForTeam checks if a player is eligible to play for a specific team
func (s *TeamEligibilityService) GetPlayerEligibilityForTeam(ctx context.Context, playerID string, teamID uint, fixtureID uint) (*PlayerEligibilityInfo, error) {
	// Get the fixture to determine the scheduled date and season
	fixture, err := s.service.fixtureRepository.FindByID(ctx, fixtureID)
	if err != nil {
		return nil, err
	}

	// Determine the playing calendar week by date to handle reschedules properly
	playingWeek, err := s.findWeekForDate(ctx, fixture.SeasonID, fixture.ScheduledDate)
	if err != nil {
		return nil, err
	}
	if playingWeek == nil {
		// Fallback to the stored week if date lookup fails
		playingWeek, err = s.service.weekRepository.FindByID(ctx, fixture.WeekID)
		if err != nil {
			return nil, err
		}
	}

	// Get the player
	player, err := s.service.playerRepository.FindByID(ctx, playerID)
	if err != nil {
		return nil, err
	}

	// Get the target team
	targetTeam, err := s.service.teamRepository.FindByID(ctx, teamID)
	if err != nil {
		return nil, err
	}

	eligibility := &PlayerEligibilityInfo{
		Player:                   *player,
		CanPlay:                  true,
		RemainingHigherTeamPlays: -1, // -1 means not applicable (not in second half or no higher teams)
		IsLockedToHigherTeam:     false,
		NeedsWarning:             false,
		IsTopTeam:                false,
		IsLocked:                 false,
		CanPlayLower:             false,
	}

	// Compute Monday 00:00 to Saturday 00:00 window for the fixture's week
	weekStart, weekEndExclusive := s.mondayToSaturdayWindow(fixture.ScheduledDate)

	// Rule 1: No player shall be allowed to play or be scheduled to play in more than one team in the Monday–Friday window
	hasPlayedThisWeek, playedTeam, err := s.hasPlayerPlayedInCalendarWeek(ctx, playerID, player.ClubID, weekStart, weekEndExclusive, fixture.ID)
	if err != nil {
		return nil, err
	}

	eligibility.PlayedThisWeek = hasPlayedThisWeek
	eligibility.PlayedThisWeekTeam = playedTeam

	if hasPlayedThisWeek {
		eligibility.CanPlay = false
		return eligibility, nil
	}

	// Rule 2 applies starting in the second half of the season
	secondHalfStart, hasSecondHalf, err := s.getSecondHalfStartDate(ctx, fixture.SeasonID)
	if err != nil {
		return nil, err
	}

	isSecondHalfByDate := hasSecondHalf && (fixture.ScheduledDate.Equal(secondHalfStart) || fixture.ScheduledDate.After(secondHalfStart))
	isSecondHalfByWeek := playingWeek.WeekNumber >= 10

	if isSecondHalfByDate || isSecondHalfByWeek {
		// Get team rankings to determine which teams are "higher"
		rankings, err := s.GetTeamRanking(ctx, targetTeam.ClubID, fixture.SeasonID)
		if err != nil {
			return nil, err
		}

		targetTeamRank := s.findTeamRank(rankings, teamID)
		if targetTeamRank == -1 {
			return nil, fmt.Errorf("target team not found in rankings")
		}

		// Check if this is the top team (rank 1)
		isTopTeam := targetTeamRank == 1
		eligibility.IsTopTeam = isTopTeam

		// Count matches played in higher teams during second half up to this fixture date (by scheduled date)
		startDate := secondHalfStart
		if !hasSecondHalf {
			startDate = playingWeek.StartDate
		}
		endDate := fixture.ScheduledDate

		higherTeamMatches, lockedTeam, err := s.countHigherTeamMatchesInDateRange(ctx, playerID, targetTeamRank, rankings, fixture.SeasonID, startDate, endDate)
		if err != nil {
			return nil, err
		}

		// Check if player is locked to a higher team (4 or more matches in higher team)
		if higherTeamMatches >= 4 {
			eligibility.CanPlay = false
			eligibility.IsLockedToHigherTeam = true
			eligibility.LockedToTeamName = lockedTeam
			return eligibility, nil
		}

		// For top team, no restrictions on higher team play since there are no higher teams
		if isTopTeam {
			// For top team, count matches in this team up to this fixture date
			currentTeamMatches, err := s.countCurrentTeamMatchesInDateRange(ctx, playerID, teamID, fixture.SeasonID, startDate, endDate)
			if err != nil {
				return nil, err
			}

			eligibility.RemainingHigherTeamPlays = 4 - currentTeamMatches // Show how many more before locked to this team
			eligibility.IsLocked = currentTeamMatches >= 4
			eligibility.CanPlayLower = true // Top team can always play lower teams
		} else {
			// For non-top teams, count matches in current team AND higher teams
			currentTeamMatches, err := s.countCurrentTeamMatchesInDateRange(ctx, playerID, teamID, fixture.SeasonID, startDate, endDate)
			if err != nil {
				return nil, err
			}

			// Add matches from higher teams to matches from current team
			totalMatches := higherTeamMatches + currentTeamMatches
			eligibility.RemainingHigherTeamPlays = 4 - totalMatches

			// Check if this is the lowest division team (players can't get locked to lowest team)
			isLowestTeam := s.isLowestRankedTeam(rankings, teamID)

			if !isLowestTeam {
				// Show warning if they're getting close to being locked (2 or fewer remaining)
				if eligibility.RemainingHigherTeamPlays <= 2 && eligibility.RemainingHigherTeamPlays >= 0 {
					eligibility.NeedsWarning = true
				}

				// Set lock status based on total matches, but only if not locked to higher team
				// IsLocked should only be true if they're locked to THIS team or lower
				// If they have 4+ matches in higher teams, they're blocked, not locked to this team
				if totalMatches >= 4 && higherTeamMatches < 4 {
					eligibility.IsLocked = true
				}

				// Can play lower teams if not locked to a higher team
				eligibility.CanPlayLower = !eligibility.IsLockedToHigherTeam
			} else {
				// For the lowest team, we don't track remaining plays since they can't get locked
				eligibility.RemainingHigherTeamPlays = -1
				eligibility.IsLocked = false
				eligibility.CanPlayLower = false // Lowest team can't play lower
			}
		}
	} else {
		// Before the second half, no restrictions apply
		eligibility.CanPlayLower = true
	}

	return eligibility, nil
}

// hasPlayerPlayedInCalendarWeek checks if a player has already played or is scheduled to play in the Monday–Friday window
func (s *TeamEligibilityService) hasPlayerPlayedInCalendarWeek(ctx context.Context, playerID string, clubID uint, weekStart time.Time, weekEndExclusive time.Time, excludeFixtureID uint) (bool, string, error) {
	query := `
		SELECT DISTINCT CASE WHEN COALESCE(mp.is_home, fp.is_home) = 1 THEN th.name ELSE ta.name END AS team_name
		FROM fixtures f
		INNER JOIN teams th ON th.id = f.home_team_id
		INNER JOIN teams ta ON ta.id = f.away_team_id
		LEFT JOIN matchups m ON m.fixture_id = f.id
		LEFT JOIN matchup_players mp ON mp.matchup_id = m.id AND mp.player_id = ?
		LEFT JOIN fixture_players fp ON fp.fixture_id = f.id AND fp.player_id = ?
		WHERE f.scheduled_date >= ? AND f.scheduled_date < ?
		  AND f.id != ?
		  AND f.status IN ('Scheduled', 'InProgress', 'Completed')
		  AND (mp.player_id IS NOT NULL OR fp.player_id IS NOT NULL)
		  AND (
		    (COALESCE(mp.is_home, fp.is_home) = 1 AND th.club_id = ?) OR
		    (COALESCE(mp.is_home, fp.is_home) = 0 AND ta.club_id = ?)
		  )
		LIMIT 1
	`

	var teamName string
	err := s.service.db.GetContext(ctx, &teamName, query, playerID, playerID, weekStart, weekEndExclusive, excludeFixtureID, clubID, clubID)
	if err != nil {
		return false, "", nil
	}
	return true, teamName, nil
}

// countHigherTeamMatchesInDateRange counts how many matches a player has played in higher-ranked teams within a date window
func (s *TeamEligibilityService) countHigherTeamMatchesInDateRange(ctx context.Context, playerID string, targetTeamRank int, rankings []TeamRank, seasonID uint, startDate time.Time, endDate time.Time) (int, string, error) {
	// Get all teams ranked higher than the target team
	var higherTeamIDs []uint
	for _, ranking := range rankings {
		if ranking.Rank < targetTeamRank {
			higherTeamIDs = append(higherTeamIDs, ranking.Team.ID)
		}
	}

	if len(higherTeamIDs) == 0 {
		return 0, "", nil // No higher teams
	}

	// Build the query to count matches in higher teams within the date range
	// Get the name of the St. Ann's team the player was actually playing FOR
	query := `
		SELECT COUNT(DISTINCT m.id),
		       COALESCE(GROUP_CONCAT(DISTINCT t.name), '') as team_names
		FROM matchup_players mp
		INNER JOIN matchups m ON mp.matchup_id = m.id
		INNER JOIN fixtures f ON m.fixture_id = f.id
		INNER JOIN teams t ON (
			(mp.is_home = 1 AND f.home_team_id = t.id) OR
			(mp.is_home = 0 AND f.away_team_id = t.id)
		)
		WHERE mp.player_id = ?
		  AND f.season_id = ?
		  AND f.scheduled_date >= ? AND f.scheduled_date <= ?
		  AND t.id IN (` + s.createPlaceholders(len(higherTeamIDs)) + `)
	`

	// Build args slice
	args := make([]interface{}, 0)
	// Player ID, season ID, and date window
	args = append(args, playerID, seasonID, startDate, endDate)
	// Team IDs for WHERE clause to filter higher teams
	for _, teamID := range higherTeamIDs {
		args = append(args, teamID)
	}

	var count int
	var teamNamesStr string
	err := s.service.db.QueryRowxContext(ctx, query, args...).Scan(&count, &teamNamesStr)
	if err != nil {
		return 0, "", err
	}

	// Return the first team name as the "locked to" team
	var lockedTeam string
	if teamNamesStr != "" {
		teamNames := strings.Split(teamNamesStr, ",")
		lockedTeam = strings.TrimSpace(teamNames[0])
	}

	return count, lockedTeam, nil
}

// countCurrentTeamMatchesInDateRange counts matches for the specific team within a date window
func (s *TeamEligibilityService) countCurrentTeamMatchesInDateRange(ctx context.Context, playerID string, teamID uint, seasonID uint, startDate time.Time, endDate time.Time) (int, error) {
	query := `
		SELECT COUNT(DISTINCT m.id)
		FROM matchup_players mp
		INNER JOIN matchups m ON mp.matchup_id = m.id
		INNER JOIN fixtures f ON m.fixture_id = f.id
		WHERE mp.player_id = ? 
		  AND f.season_id = ?
		  AND f.scheduled_date >= ? AND f.scheduled_date <= ?
		  AND (
			(f.home_team_id = ? AND mp.is_home = 1) OR
			(f.away_team_id = ? AND mp.is_home = 0)
		  )
	`

	var count int
	err := s.service.db.GetContext(ctx, &count, query, playerID, seasonID, startDate, endDate, teamID, teamID)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// findWeekForDate finds the week record that contains the given date for a season
func (s *TeamEligibilityService) findWeekForDate(ctx context.Context, seasonID uint, date time.Time) (*models.Week, error) {
	weeks, err := s.service.weekRepository.FindBySeason(ctx, seasonID)
	if err != nil {
		return nil, err
	}
	for i := range weeks {
		w := weeks[i]
		if (date.Equal(w.StartDate) || date.After(w.StartDate)) && (date.Equal(w.EndDate) || date.Before(w.EndDate)) {
			return &w, nil
		}
	}
	return nil, nil
}

// getSecondHalfStartDate returns the start date of Week 10 (second half) if present
func (s *TeamEligibilityService) getSecondHalfStartDate(ctx context.Context, seasonID uint) (time.Time, bool, error) {
	week10, err := s.service.weekRepository.FindByWeekNumber(ctx, seasonID, 10)
	if err != nil {
		// If not found, return false flag; upstream will fallback
		return time.Time{}, false, nil
	}
	return week10.StartDate, true, nil
}

// mondayToSaturdayWindow returns Monday 00:00 (inclusive) to Saturday 00:00 (exclusive) for the week of the given date
func (s *TeamEligibilityService) mondayToSaturdayWindow(date time.Time) (time.Time, time.Time) {
	// Normalize to same location
	loc := date.Location()
	weekday := int(date.Weekday()) // 0=Sunday, 1=Monday, ...
	// Days since Monday: (weekday + 6) % 7
	daysSinceMonday := (weekday + 6) % 7
	monday := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -daysSinceMonday)
	saturday := monday.AddDate(0, 0, 5) // Saturday 00:00 exclusive
	return monday, saturday
}

// createPlaceholders creates a string of SQL placeholders for IN clauses
func (s *TeamEligibilityService) createPlaceholders(count int) string {
	if count == 0 {
		return ""
	}
	placeholders := make([]string, count)
	for i := range placeholders {
		placeholders[i] = "?"
	}
	return strings.Join(placeholders, ",")
}

// findTeamRank finds the rank of a team in the rankings
func (s *TeamEligibilityService) findTeamRank(rankings []TeamRank, teamID uint) int {
	for _, ranking := range rankings {
		if ranking.Team.ID == teamID {
			return ranking.Rank
		}
	}
	return -1 // Not found
}

// isLowestRankedTeam checks if a team is the lowest ranked team (can't get locked to lowest team)
func (s *TeamEligibilityService) isLowestRankedTeam(rankings []TeamRank, teamID uint) bool {
	if len(rankings) == 0 {
		return false
	}

	// Find the highest rank number (lowest ranked team)
	maxRank := 0
	for _, ranking := range rankings {
		if ranking.Rank > maxRank {
			maxRank = ranking.Rank
		}
	}

	// Check if this team has the highest rank number
	for _, ranking := range rankings {
		if ranking.Team.ID == teamID {
			return ranking.Rank == maxRank
		}
	}

	return false
}
