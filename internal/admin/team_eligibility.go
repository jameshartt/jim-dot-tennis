package admin

import (
	"context"
	"fmt"
	"sort"
	"strings"

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
	// Get the fixture to determine the week and season
	fixture, err := s.service.fixtureRepository.FindByID(ctx, fixtureID)
	if err != nil {
		return nil, err
	}

	// Get the week to check if we're in second half of season (Week 10+)
	week, err := s.service.weekRepository.FindByID(ctx, fixture.WeekID)
	if err != nil {
		return nil, err
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

	// Check if player has already played this week
	hasPlayedThisWeek, playedTeam, err := s.hasPlayerPlayedThisWeek(ctx, playerID, week.ID)
	if err != nil {
		return nil, err
	}

	eligibility.PlayedThisWeek = hasPlayedThisWeek
	eligibility.PlayedThisWeekTeam = playedTeam

	// Rule 1: No player shall be allowed to play in more than one team in any week
	if hasPlayedThisWeek {
		eligibility.CanPlay = false
		return eligibility, nil
	}

	// Rule 2 only applies from Week 10 onwards (second half of season)
	if week.WeekNumber >= 10 {
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

		// Count matches played in higher teams during second half of season
		higherTeamMatches, lockedTeam, err := s.countHigherTeamMatches(ctx, playerID, targetTeamRank, rankings, fixture.SeasonID)
		if err != nil {
			return nil, err
		}

		// Check if player is locked to a higher team (more than 4 matches in higher team)
		if higherTeamMatches > 4 {
			eligibility.CanPlay = false
			eligibility.IsLockedToHigherTeam = true
			eligibility.LockedToTeamName = lockedTeam
			return eligibility, nil
		}

		// For top team, no restrictions on higher team play since there are no higher teams
		if isTopTeam {
			// For top team, count matches played in this team to show lock-in progress
			currentTeamMatches, err := s.countCurrentTeamMatches(ctx, playerID, teamID, fixture.SeasonID)
			if err != nil {
				return nil, err
			}

			eligibility.RemainingHigherTeamPlays = 4 - currentTeamMatches // Show how many more before locked to this team
			eligibility.IsLocked = currentTeamMatches >= 4
			eligibility.CanPlayLower = true // Top team can always play lower teams
		} else {
			// For non-top teams, count matches in current team AND higher teams
			currentTeamMatches, err := s.countCurrentTeamMatches(ctx, playerID, teamID, fixture.SeasonID)
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

				// Set lock status based on total matches (current + higher teams)
				if totalMatches >= 4 {
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
		// Before Week 10, no restrictions apply
		eligibility.CanPlayLower = true
	}

	return eligibility, nil
}

// hasPlayerPlayedThisWeek checks if a player has already played in any fixture this week
func (s *TeamEligibilityService) hasPlayerPlayedThisWeek(ctx context.Context, playerID string, weekID uint) (bool, string, error) {
	// Query for any matchup players in fixtures for this week
	query := `
		SELECT DISTINCT t.name
		FROM matchup_players mp
		INNER JOIN matchups m ON mp.matchup_id = m.id
		INNER JOIN fixtures f ON m.fixture_id = f.id
		INNER JOIN teams t ON (f.home_team_id = t.id OR f.away_team_id = t.id)
		WHERE mp.player_id = ? AND f.week_id = ?
		LIMIT 1
	`

	var teamName string
	err := s.service.db.GetContext(ctx, &teamName, query, playerID, weekID)
	if err != nil {
		// If no rows found, player hasn't played this week
		return false, "", nil
	}

	return true, teamName, nil
}

// countHigherTeamMatches counts how many matches a player has played in higher-ranked teams
// during the second half of the season (from Week 10 onwards)
func (s *TeamEligibilityService) countHigherTeamMatches(ctx context.Context, playerID string, targetTeamRank int, rankings []TeamRank, seasonID uint) (int, string, error) {
	// Get all teams ranked higher than the target team
	var higherTeamIDs []uint
	var higherTeamNames []string
	for _, ranking := range rankings {
		if ranking.Rank < targetTeamRank {
			higherTeamIDs = append(higherTeamIDs, ranking.Team.ID)
			higherTeamNames = append(higherTeamNames, ranking.Team.Name)
		}
	}

	if len(higherTeamIDs) == 0 {
		return 0, "", nil // No higher teams
	}

	// Build the query to count matches in higher teams from Week 10 onwards
	query := `
		SELECT COUNT(DISTINCT m.id),
		       COALESCE(GROUP_CONCAT(DISTINCT t.name), '') as team_names
		FROM matchup_players mp
		INNER JOIN matchups m ON mp.matchup_id = m.id
		INNER JOIN fixtures f ON m.fixture_id = f.id
		INNER JOIN weeks w ON f.week_id = w.id
		INNER JOIN teams t ON (
			(f.home_team_id IN (` + s.createPlaceholders(len(higherTeamIDs)) + `) AND mp.is_home = 1) OR
			(f.away_team_id IN (` + s.createPlaceholders(len(higherTeamIDs)) + `) AND mp.is_home = 0)
		)
		WHERE mp.player_id = ?
		  AND w.week_number >= 10
		  AND w.season_id = ?
		  AND (f.home_team_id IN (` + s.createPlaceholders(len(higherTeamIDs)) + `) OR f.away_team_id IN (` + s.createPlaceholders(len(higherTeamIDs)) + `))
	`

	// Build args slice
	args := make([]interface{}, 0)
	// First set of team IDs for the team filter in SELECT
	for _, teamID := range higherTeamIDs {
		args = append(args, teamID)
	}
	// Second set for the team filter in SELECT
	for _, teamID := range higherTeamIDs {
		args = append(args, teamID)
	}
	// Player ID and season ID
	args = append(args, playerID, seasonID)
	// Third set for WHERE clause home team
	for _, teamID := range higherTeamIDs {
		args = append(args, teamID)
	}
	// Fourth set for WHERE clause away team
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

// countCurrentTeamMatches counts how many matches a player has played in the current team
func (s *TeamEligibilityService) countCurrentTeamMatches(ctx context.Context, playerID string, teamID uint, seasonID uint) (int, error) {
	query := `
		SELECT COUNT(DISTINCT m.id)
		FROM matchup_players mp
		INNER JOIN matchups m ON mp.matchup_id = m.id
		INNER JOIN fixtures f ON m.fixture_id = f.id
		INNER JOIN weeks w ON f.week_id = w.id
		WHERE mp.player_id = ? 
		  AND w.week_number >= 10 
		  AND w.season_id = ?
		  AND (
		    (f.home_team_id = ? AND mp.is_home = 1) OR
		    (f.away_team_id = ? AND mp.is_home = 0)
		  )
	`

	var count int
	err := s.service.db.GetContext(ctx, &count, query, playerID, seasonID, teamID, teamID)
	if err != nil {
		return 0, err
	}

	return count, nil
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

// GetPlayersEligibilityForTeamSelection gets eligibility info for all players for team selection
func (s *TeamEligibilityService) GetPlayersEligibilityForTeamSelection(ctx context.Context, players []models.Player, teamID uint, fixtureID uint) (map[string]*PlayerEligibilityInfo, error) {
	eligibilityMap := make(map[string]*PlayerEligibilityInfo)

	for _, player := range players {
		eligibility, err := s.GetPlayerEligibilityForTeam(ctx, player.ID, teamID, fixtureID)
		if err != nil {
			// Log error but continue with other players
			eligibility = &PlayerEligibilityInfo{
				Player:  player,
				CanPlay: true, // Default to allowing play if we can't determine eligibility
			}
		}
		eligibilityMap[player.ID] = eligibility
	}

	return eligibilityMap, nil
}
