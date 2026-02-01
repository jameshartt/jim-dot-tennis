package admin

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/services"
)

// FixtureWithRelations represents a fixture with its related entities loaded
type FixtureWithRelations struct {
	models.Fixture
	HomeTeam           *models.Team     `json:"home_team,omitempty"`
	AwayTeam           *models.Team     `json:"away_team,omitempty"`
	Week               *models.Week     `json:"week,omitempty"`
	Division           *models.Division `json:"division,omitempty"`
	Season             *models.Season   `json:"season,omitempty"`
	IsStAnnsHome       bool             `json:"is_stanns_home"`
	IsStAnnsAway       bool             `json:"is_stanns_away"`
	IsDerby            bool             `json:"is_derby"`                       // Both teams are St Ann's
	DefaultTeamContext *models.Team     `json:"default_team_context,omitempty"` // Which team to manage by default
}

// FixtureDetail represents a fixture with comprehensive related data for detail view
type FixtureDetail struct {
	models.Fixture
	HomeTeam            *models.Team             `json:"home_team,omitempty"`
	AwayTeam            *models.Team             `json:"away_team,omitempty"`
	Week                *models.Week             `json:"week,omitempty"`
	Division            *models.Division         `json:"division,omitempty"`
	Season              *models.Season           `json:"season,omitempty"`
	DayCaptain          *models.Player           `json:"day_captain,omitempty"`
	Matchups            []MatchupWithPlayers     `json:"matchups,omitempty"`
	SelectedPlayers     []SelectedPlayerInfo     `json:"selected_players,omitempty"`
	DuplicateWarnings   []DuplicatePlayerWarning `json:"duplicate_warnings,omitempty"`
	VenueClub           *models.Club             `json:"venue_club,omitempty"`
	IsVenueOverridden   bool                     `json:"is_venue_overridden"`
	VenueOverrideReason string                   `json:"venue_override_reason,omitempty"`
}

// GetFixtures retrieves all fixtures for admin management
func (s *Service) GetFixtures() (interface{}, error) {
	// TODO: Implement fixture retrieval from database
	return nil, nil
}

// GetStAnnsFixtures retrieves upcoming fixtures for St. Ann's club with related data
func (s *Service) GetStAnnsFixtures() (*models.Club, []FixtureWithRelations, error) {
	ctx := context.Background()

	// Find St. Ann's club
	clubs, err := s.clubRepository.FindByNameLike(ctx, "St Ann")
	if err != nil {
		return nil, nil, err
	}
	if len(clubs) == 0 {
		return nil, nil, nil // No club found
	}
	stAnnsClub := &clubs[0]

	// Get all teams for St. Ann's club
	teams, err := s.teamRepository.FindByClub(ctx, stAnnsClub.ID)
	if err != nil {
		return stAnnsClub, nil, err
	}

	if len(teams) == 0 {
		return stAnnsClub, nil, nil // No teams found
	}

	// Get upcoming fixtures for all St. Ann's teams
	var allFixtures []models.Fixture
	fixtureMap := make(map[uint]models.Fixture) // Use map to deduplicate fixtures by ID

	for _, team := range teams {
		teamFixtures, err := s.fixtureRepository.FindByTeam(ctx, team.ID)
		if err != nil {
			continue // Skip this team if there's an error
		}
		// Add fixtures to map to automatically deduplicate
		for _, fixture := range teamFixtures {
			fixtureMap[fixture.ID] = fixture
		}
	}

	// Convert map back to slice
	for _, fixture := range fixtureMap {
		allFixtures = append(allFixtures, fixture)
	}

	// Filter for upcoming fixtures (scheduled or in progress) from tomorrow onwards
	var upcomingFixtures []models.Fixture
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrowStart := todayStart.Add(24 * time.Hour)
	for _, fixture := range allFixtures {
		if fixture.Status == models.Scheduled || fixture.Status == models.InProgress {
			// Upcoming list excludes today's fixtures; those are shown separately
			if !fixture.ScheduledDate.Before(tomorrowStart) {
				upcomingFixtures = append(upcomingFixtures, fixture)
			}
		}
	}

	// Build FixtureWithRelations by fetching related data
	fixturesWithRelations := s.buildFixturesWithRelations(ctx, upcomingFixtures, stAnnsClub)

	// Sort fixtures by scheduled date (nearest first), then by division (descending)
	sort.Slice(fixturesWithRelations, func(i, j int) bool {
		// First sort by date (ascending)
		if fixturesWithRelations[i].ScheduledDate.Before(fixturesWithRelations[j].ScheduledDate) {
			return true
		}
		if fixturesWithRelations[i].ScheduledDate.After(fixturesWithRelations[j].ScheduledDate) {
			return false
		}

		// If dates are equal, sort by division (descending - Division 4 before Division 3)
		divisionI := ""
		divisionJ := ""
		if fixturesWithRelations[i].Division != nil {
			divisionI = fixturesWithRelations[i].Division.Name
		}
		if fixturesWithRelations[j].Division != nil {
			divisionJ = fixturesWithRelations[j].Division.Name
		}

		// For descending order, return i > j
		return divisionI > divisionJ
	})

	return stAnnsClub, fixturesWithRelations, nil
}

// GetStAnnsPastFixtures retrieves past fixtures for St. Ann's club with related data
func (s *Service) GetStAnnsPastFixtures() (*models.Club, []FixtureWithRelations, error) {
	ctx := context.Background()

	// Get the active season
	activeSeason, err := s.seasonRepository.FindActive(ctx)
	if err != nil {
		return nil, nil, err
	}
	if activeSeason == nil {
		return nil, nil, nil // No active season
	}

	// Find St. Ann's club
	clubs, err := s.clubRepository.FindByNameLike(ctx, "St Ann")
	if err != nil {
		return nil, nil, err
	}
	if len(clubs) == 0 {
		return nil, nil, nil // No club found
	}
	stAnnsClub := &clubs[0]

	// Get all teams for St. Ann's club
	teams, err := s.teamRepository.FindByClub(ctx, stAnnsClub.ID)
	if err != nil {
		return stAnnsClub, nil, err
	}

	if len(teams) == 0 {
		return stAnnsClub, nil, nil // No teams found
	}

	// Get all fixtures for all St. Ann's teams
	var allFixtures []models.Fixture
	fixtureMap := make(map[uint]models.Fixture) // Use map to deduplicate fixtures by ID

	for _, team := range teams {
		teamFixtures, err := s.fixtureRepository.FindByTeam(ctx, team.ID)
		if err != nil {
			continue // Skip this team if there's an error
		}
		// Add fixtures to map to automatically deduplicate
		for _, fixture := range teamFixtures {
			fixtureMap[fixture.ID] = fixture
		}
	}

	// Convert map back to slice
	for _, fixture := range fixtureMap {
		allFixtures = append(allFixtures, fixture)
	}

	// Filter for past fixtures from the active season only (completed/cancelled/postponed or scheduled/in-progress before today)
	var pastFixtures []models.Fixture
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	for _, fixture := range allFixtures {
		// Only include fixtures from the active season
		if fixture.SeasonID != activeSeason.ID {
			continue
		}

		if fixture.Status == models.Completed || fixture.Status == models.Cancelled || fixture.Status == models.Postponed {
			pastFixtures = append(pastFixtures, fixture)
		} else if fixture.Status == models.Scheduled || fixture.Status == models.InProgress {
			if fixture.ScheduledDate.Before(todayStart) {
				pastFixtures = append(pastFixtures, fixture)
			}
		}
	}

	// Build FixtureWithRelations by fetching related data
	fixturesWithRelations := s.buildFixturesWithRelations(ctx, pastFixtures, stAnnsClub)

	// Sort fixtures by scheduled date (most recent first), then by division (ascending)
	sort.Slice(fixturesWithRelations, func(i, j int) bool {
		// First sort by date (descending - most recent first)
		if fixturesWithRelations[i].ScheduledDate.After(fixturesWithRelations[j].ScheduledDate) {
			return true
		}
		if fixturesWithRelations[i].ScheduledDate.Before(fixturesWithRelations[j].ScheduledDate) {
			return false
		}

		// If dates are equal, sort by division (ascending - Division 3 before Division 4)
		divisionI := ""
		divisionJ := ""
		if fixturesWithRelations[i].Division != nil {
			divisionI = fixturesWithRelations[i].Division.Name
		}
		if fixturesWithRelations[j].Division != nil {
			divisionJ = fixturesWithRelations[j].Division.Name
		}

		// For ascending order, return i < j
		return divisionI < divisionJ
	})

	return stAnnsClub, fixturesWithRelations, nil
}

// GetStAnnsTodaysFixtures retrieves today's fixtures for St. Ann's club with related data
func (s *Service) GetStAnnsTodaysFixtures() (*models.Club, []FixtureWithRelations, error) {
	ctx := context.Background()

	// Find St. Ann's club
	clubs, err := s.clubRepository.FindByNameLike(ctx, "St Ann")
	if err != nil {
		return nil, nil, err
	}
	if len(clubs) == 0 {
		return nil, nil, nil // No club found
	}
	stAnnsClub := &clubs[0]

	// Get all teams for St. Ann's club
	teams, err := s.teamRepository.FindByClub(ctx, stAnnsClub.ID)
	if err != nil {
		return stAnnsClub, nil, err
	}

	if len(teams) == 0 {
		return stAnnsClub, nil, nil // No teams found
	}

	// Collect fixtures for today across all teams (deduped)
	var allFixtures []models.Fixture
	fixtureMap := make(map[uint]models.Fixture)

	for _, team := range teams {
		teamFixtures, err := s.fixtureRepository.FindByTeam(ctx, team.ID)
		if err != nil {
			continue
		}
		for _, fixture := range teamFixtures {
			fixtureMap[fixture.ID] = fixture
		}
	}

	for _, fixture := range fixtureMap {
		allFixtures = append(allFixtures, fixture)
	}

	// Compute today's boundaries
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrowStart := todayStart.Add(24 * time.Hour)

	// Filter for today's fixtures
	var todaysFixtures []models.Fixture
	for _, fixture := range allFixtures {
		if fixture.Status == models.Scheduled || fixture.Status == models.InProgress {
			if !fixture.ScheduledDate.Before(todayStart) && fixture.ScheduledDate.Before(tomorrowStart) {
				todaysFixtures = append(todaysFixtures, fixture)
			}
		}
	}

	fixturesWithRelations := s.buildFixturesWithRelations(ctx, todaysFixtures, stAnnsClub)

	// Sort by time then division name desc
	sort.Slice(fixturesWithRelations, func(i, j int) bool {
		if fixturesWithRelations[i].ScheduledDate.Before(fixturesWithRelations[j].ScheduledDate) {
			return true
		}
		if fixturesWithRelations[i].ScheduledDate.After(fixturesWithRelations[j].ScheduledDate) {
			return false
		}
		divisionI := ""
		divisionJ := ""
		if fixturesWithRelations[i].Division != nil {
			divisionI = fixturesWithRelations[i].Division.Name
		}
		if fixturesWithRelations[j].Division != nil {
			divisionJ = fixturesWithRelations[j].Division.Name
		}
		return divisionI > divisionJ
	})

	return stAnnsClub, fixturesWithRelations, nil
}

// buildFixturesWithRelations is a helper method to build FixtureWithRelations from fixtures
func (s *Service) buildFixturesWithRelations(ctx context.Context, fixtures []models.Fixture, stAnnsClub *models.Club) []FixtureWithRelations {
	var fixturesWithRelations []FixtureWithRelations

	for _, fixture := range fixtures {
		fixtureWithRelations := FixtureWithRelations{
			Fixture: fixture,
		}

		// Declare team variables for later use
		var homeTeam, awayTeam *models.Team

		// Get home team
		if team, err := s.teamRepository.FindByID(ctx, fixture.HomeTeamID); err == nil {
			homeTeam = team
			fixtureWithRelations.HomeTeam = homeTeam
		}

		// Get away team
		if team, err := s.teamRepository.FindByID(ctx, fixture.AwayTeamID); err == nil {
			awayTeam = team
			fixtureWithRelations.AwayTeam = awayTeam
		}

		// Get week
		if week, err := s.weekRepository.FindByID(ctx, fixture.WeekID); err == nil {
			fixtureWithRelations.Week = week
		}

		// Get division
		if division, err := s.getDivisionByID(ctx, fixture.DivisionID); err == nil {
			fixtureWithRelations.Division = division
		}

		// Get season
		if season, err := s.getSeasonByID(ctx, fixture.SeasonID); err == nil {
			fixtureWithRelations.Season = season
		}

		// Determine if St. Ann's is home or away (only if teams were loaded successfully)
		if homeTeam != nil && homeTeam.ClubID == stAnnsClub.ID {
			fixtureWithRelations.IsStAnnsHome = true
		}
		if awayTeam != nil && awayTeam.ClubID == stAnnsClub.ID {
			fixtureWithRelations.IsStAnnsAway = true
		}

		// Determine if it's a derby match (both teams are St Ann's)
		if homeTeam != nil && awayTeam != nil &&
			homeTeam.ClubID == stAnnsClub.ID && awayTeam.ClubID == stAnnsClub.ID {

			// For derby matches, create TWO separate entries - one for each team's perspective

			// First entry: Home team perspective
			homeFixture := fixtureWithRelations
			homeFixture.IsDerby = true
			homeFixture.DefaultTeamContext = homeTeam
			fixturesWithRelations = append(fixturesWithRelations, homeFixture)

			// Second entry: Away team perspective
			awayFixture := fixtureWithRelations
			awayFixture.IsDerby = true
			awayFixture.DefaultTeamContext = awayTeam
			fixturesWithRelations = append(fixturesWithRelations, awayFixture)
		} else {
			// Regular match: only one entry
			fixturesWithRelations = append(fixturesWithRelations, fixtureWithRelations)
		}
	}

	return fixturesWithRelations
}

// GetFixtureDetail retrieves comprehensive details for a specific fixture
func (s *Service) GetFixtureDetail(fixtureID uint) (*FixtureDetail, error) {
	ctx := context.Background()

	// Get the base fixture
	fixture, err := s.fixtureRepository.FindByID(ctx, fixtureID)
	if err != nil {
		return nil, err
	}

	// Create the detail struct
	detail := &FixtureDetail{
		Fixture: *fixture,
	}

	// Get home team
	if homeTeam, err := s.teamRepository.FindByID(ctx, fixture.HomeTeamID); err == nil {
		detail.HomeTeam = homeTeam
	}

	// Get away team
	if awayTeam, err := s.teamRepository.FindByID(ctx, fixture.AwayTeamID); err == nil {
		detail.AwayTeam = awayTeam
	}

	// Get week
	if week, err := s.weekRepository.FindByID(ctx, fixture.WeekID); err == nil {
		detail.Week = week
	}

	// Get division
	if division, err := s.getDivisionByID(ctx, fixture.DivisionID); err == nil {
		detail.Division = division
	}

	// Get season
	if season, err := s.getSeasonByID(ctx, fixture.SeasonID); err == nil {
		detail.Season = season
	}

	// Resolve venue
	venueResolver := services.NewVenueResolver(s.clubRepository, s.teamRepository, s.venueOverrideRepository)
	if resolution, err := venueResolver.ResolveFixtureVenue(ctx, fixture); err == nil {
		detail.VenueClub = resolution.Club
		detail.IsVenueOverridden = resolution.IsOverridden
		detail.VenueOverrideReason = resolution.OverrideReason
	}

	// Get day captain if assigned
	if fixture.DayCaptainID != nil {
		if dayCaptain, err := s.playerRepository.FindByID(ctx, *fixture.DayCaptainID); err == nil {
			detail.DayCaptain = dayCaptain
		}
	}

	// Get matchups with players for the fixture
	if matchups, err := s.matchupRepository.FindByFixture(ctx, fixtureID); err == nil {
		var matchupsWithPlayers []MatchupWithPlayers
		for _, matchup := range matchups {
			// Get players for this matchup
			matchupPlayers, err := s.matchupRepository.FindPlayersInMatchup(ctx, matchup.ID)
			if err != nil {
				// If we can't get players, still include the matchup with empty players
				matchupsWithPlayers = append(matchupsWithPlayers, MatchupWithPlayers{
					Matchup: matchup,
					Players: []MatchupPlayerWithInfo{},
				})
				continue
			}

			var playersWithInfo []MatchupPlayerWithInfo
			for _, mp := range matchupPlayers {
				if player, err := s.playerRepository.FindByID(ctx, mp.PlayerID); err == nil {
					playersWithInfo = append(playersWithInfo, MatchupPlayerWithInfo{
						MatchupPlayer: mp,
						Player:        *player,
					})
				}
			}

			matchupsWithPlayers = append(matchupsWithPlayers, MatchupWithPlayers{
				Matchup: matchup,
				Players: playersWithInfo,
			})
		}

		// Sort matchups in the desired order: 1st Mixed, 2nd Mixed, Mens, Womens
		sort.Slice(matchupsWithPlayers, func(i, j int) bool {
			return getMatchupOrder(matchupsWithPlayers[i].Matchup.Type) < getMatchupOrder(matchupsWithPlayers[j].Matchup.Type)
		})

		detail.Matchups = matchupsWithPlayers

		// Check for duplicate players across matchups
		detail.DuplicateWarnings = s.detectDuplicatePlayersInMatchups(matchupsWithPlayers)
	}

	// Get selected players for the fixture
	if selectedPlayers, err := s.fixtureRepository.FindSelectedPlayers(ctx, fixtureID); err == nil {
		var selectedPlayerInfos []SelectedPlayerInfo
		for _, sp := range selectedPlayers {
			if player, err := s.playerRepository.FindByID(ctx, sp.PlayerID); err == nil {
				// Get availability information for this player and fixture
				availability := s.determinePlayerAvailabilityForFixture(ctx, sp.PlayerID, fixtureID, fixture.ScheduledDate)

				selectedPlayerInfos = append(selectedPlayerInfos, SelectedPlayerInfo{
					FixturePlayer:      sp,
					Player:             *player,
					AvailabilityStatus: availability.Status,
					AvailabilityNotes:  availability.Notes,
				})
			}
		}
		detail.SelectedPlayers = selectedPlayerInfos
	}

	return detail, nil
}

// IsStAnnsHomeInFixture determines whether St Ann's is the home team in a fixture
// Uses the exact same logic as buildFixturesWithRelations to ensure consistency
func (s *Service) IsStAnnsHomeInFixture(fixtureID uint) bool {
	ctx := context.Background()

	// Get the fixture
	fixture, err := s.fixtureRepository.FindByID(ctx, fixtureID)
	if err != nil {
		return false // Default to away if we can't determine
	}

	// Find St. Ann's club (using same logic as buildFixturesWithRelations)
	clubs, err := s.clubRepository.FindByNameLike(ctx, "St Ann")
	if err != nil || len(clubs) == 0 {
		return false // Default to away if we can't find St Ann's
	}
	stAnnsClub := &clubs[0]

	// Get home team (using same logic as buildFixturesWithRelations)
	homeTeam, err := s.teamRepository.FindByID(ctx, fixture.HomeTeamID)
	if err != nil {
		return false // Default to away if we can't get home team
	}

	// Use exact same logic as buildFixturesWithRelations:
	// "Determine if St. Ann's is home or away (only if teams were loaded successfully)"
	if homeTeam != nil && homeTeam.ClubID == stAnnsClub.ID {
		return true // IsStAnnsHome = true
	}

	return false // IsStAnnsHome = false (either away or not St Ann's fixture)
}

// GetUpcomingFixturesForTeam retrieves upcoming fixtures for a specific team
// Limited to a specific count and includes today's fixtures
func (s *Service) GetUpcomingFixturesForTeam(teamID uint, limit int) ([]FixtureWithRelations, error) {
	ctx := context.Background()

	// Get all fixtures for the team
	teamFixtures, err := s.fixtureRepository.FindByTeam(ctx, teamID)
	if err != nil {
		return nil, err
	}

	// Filter for upcoming fixtures (today or later) that are scheduled or in progress
	var upcomingFixtures []models.Fixture
	now := time.Now()
	today := now.Truncate(24 * time.Hour)

	for _, fixture := range teamFixtures {
		// Include fixtures that are today or in the future, and are scheduled or in progress
		if (fixture.ScheduledDate.After(now) || fixture.ScheduledDate.After(today)) &&
			(fixture.Status == models.Scheduled || fixture.Status == models.InProgress) {
			upcomingFixtures = append(upcomingFixtures, fixture)
		}
	}

	// Sort by scheduled date (earliest first)
	// Note: Go's slice sorting would be better, but we'll keep it simple for now
	// since the repository should already return them in order

	// Limit the results
	if limit > 0 && len(upcomingFixtures) > limit {
		upcomingFixtures = upcomingFixtures[:limit]
	}

	// Build FixtureWithRelations by fetching related data
	var fixturesWithRelations []FixtureWithRelations
	for _, fixture := range upcomingFixtures {
		fixtureWithRelations := FixtureWithRelations{
			Fixture: fixture,
		}

		// Declare team variables for later use
		var homeTeam, awayTeam *models.Team

		// Get home team
		if homeTeamResult, err := s.teamRepository.FindByID(ctx, fixture.HomeTeamID); err == nil {
			homeTeam = homeTeamResult
			fixtureWithRelations.HomeTeam = homeTeam
		}

		// Get away team
		if awayTeamResult, err := s.teamRepository.FindByID(ctx, fixture.AwayTeamID); err == nil {
			awayTeam = awayTeamResult
			fixtureWithRelations.AwayTeam = awayTeam
		}

		// Get week
		if week, err := s.weekRepository.FindByID(ctx, fixture.WeekID); err == nil {
			fixtureWithRelations.Week = week
		}

		// Get division
		if division, err := s.getDivisionByID(ctx, fixture.DivisionID); err == nil {
			fixtureWithRelations.Division = division
		}

		// Get season
		if season, err := s.getSeasonByID(ctx, fixture.SeasonID); err == nil {
			fixtureWithRelations.Season = season
		}

		// Determine if the requesting team is home or away (only if teams were loaded successfully)
		if homeTeam != nil && homeTeam.ID == teamID {
			fixtureWithRelations.IsStAnnsHome = true
		}
		if awayTeam != nil && awayTeam.ID == teamID {
			fixtureWithRelations.IsStAnnsAway = true
		}

		// Determine if it's a derby match (both teams are from the same club)
		if homeTeam != nil && awayTeam != nil && homeTeam.ClubID == awayTeam.ClubID {
			fixtureWithRelations.IsDerby = true

			// For derby matches, set the default team context to the requesting team
			if homeTeam.ID == teamID {
				fixtureWithRelations.DefaultTeamContext = homeTeam
			} else if awayTeam.ID == teamID {
				fixtureWithRelations.DefaultTeamContext = awayTeam
			}
		}

		fixturesWithRelations = append(fixturesWithRelations, fixtureWithRelations)
	}

	return fixturesWithRelations, nil
}

// GetFixtureDetailWithTeamContext gets fixture details filtered for a specific managing team (for derby matches)
func (s *Service) GetFixtureDetailWithTeamContext(fixtureID uint, managingTeamID uint) (*FixtureDetail, error) {
	ctx := context.Background()

	// Get the base fixture
	fixture, err := s.fixtureRepository.FindByID(ctx, fixtureID)
	if err != nil {
		return nil, err
	}

	// Create the detail struct
	detail := &FixtureDetail{
		Fixture: *fixture,
	}

	// Get home team
	if homeTeam, err := s.teamRepository.FindByID(ctx, fixture.HomeTeamID); err == nil {
		detail.HomeTeam = homeTeam
	}

	// Get away team
	if awayTeam, err := s.teamRepository.FindByID(ctx, fixture.AwayTeamID); err == nil {
		detail.AwayTeam = awayTeam
	}

	// Get week
	if week, err := s.weekRepository.FindByID(ctx, fixture.WeekID); err == nil {
		detail.Week = week
	}

	// Get division
	if division, err := s.getDivisionByID(ctx, fixture.DivisionID); err == nil {
		detail.Division = division
	}

	// Get season
	if season, err := s.getSeasonByID(ctx, fixture.SeasonID); err == nil {
		detail.Season = season
	}

	// Resolve venue
	venueResolver := services.NewVenueResolver(s.clubRepository, s.teamRepository, s.venueOverrideRepository)
	if resolution, err := venueResolver.ResolveFixtureVenue(ctx, fixture); err == nil {
		detail.VenueClub = resolution.Club
		detail.IsVenueOverridden = resolution.IsOverridden
		detail.VenueOverrideReason = resolution.OverrideReason
	}

	// Get day captain if assigned
	if fixture.DayCaptainID != nil {
		if dayCaptain, err := s.playerRepository.FindByID(ctx, *fixture.DayCaptainID); err == nil {
			detail.DayCaptain = dayCaptain
		}
	}

	// Get matchups with players for the fixture, filtered by managing team
	if matchups, err := s.getMatchupsForTeam(ctx, fixtureID, managingTeamID); err == nil {
		var matchupsWithPlayers []MatchupWithPlayers
		for _, matchup := range matchups {
			// Get players for this matchup
			matchupPlayers, err := s.matchupRepository.FindPlayersInMatchup(ctx, matchup.ID)
			if err != nil {
				// If we can't get players, still include the matchup with empty players
				matchupsWithPlayers = append(matchupsWithPlayers, MatchupWithPlayers{
					Matchup: matchup,
					Players: []MatchupPlayerWithInfo{},
				})
				continue
			}

			var playersWithInfo []MatchupPlayerWithInfo
			for _, mp := range matchupPlayers {
				if player, err := s.playerRepository.FindByID(ctx, mp.PlayerID); err == nil {
					playersWithInfo = append(playersWithInfo, MatchupPlayerWithInfo{
						MatchupPlayer: mp,
						Player:        *player,
					})
				}
			}

			matchupsWithPlayers = append(matchupsWithPlayers, MatchupWithPlayers{
				Matchup: matchup,
				Players: playersWithInfo,
			})
		}

		// Sort matchups in the desired order: 1st Mixed, 2nd Mixed, Mens, Womens
		sort.Slice(matchupsWithPlayers, func(i, j int) bool {
			return getMatchupOrder(matchupsWithPlayers[i].Matchup.Type) < getMatchupOrder(matchupsWithPlayers[j].Matchup.Type)
		})

		detail.Matchups = matchupsWithPlayers

		// Check for duplicate players across matchups
		detail.DuplicateWarnings = s.detectDuplicatePlayersInMatchups(matchupsWithPlayers)
	}

	// Get selected players for the fixture, filtered by managing team
	if selectedPlayers, err := s.fixtureRepository.FindSelectedPlayersByTeam(ctx, fixtureID, managingTeamID); err == nil {
		var selectedPlayerInfos []SelectedPlayerInfo
		for _, sp := range selectedPlayers {
			if player, err := s.playerRepository.FindByID(ctx, sp.PlayerID); err == nil {
				// Get availability information for this player and fixture
				availability := s.determinePlayerAvailabilityForFixture(ctx, sp.PlayerID, fixtureID, fixture.ScheduledDate)

				selectedPlayerInfos = append(selectedPlayerInfos, SelectedPlayerInfo{
					FixturePlayer:      sp,
					Player:             *player,
					AvailabilityStatus: availability.Status,
					AvailabilityNotes:  availability.Notes,
				})
			}
		}
		detail.SelectedPlayers = selectedPlayerInfos
	}

	return detail, nil
}

// UpdateFixtureNotes updates the notes field of a fixture
func (s *Service) UpdateFixtureNotes(fixtureID uint, notes string) error {
	ctx := context.Background()

	// Get the current fixture
	fixture, err := s.fixtureRepository.FindByID(ctx, fixtureID)
	if err != nil {
		return err
	}

	// Update the notes
	fixture.Notes = notes

	// Save the updated fixture
	return s.fixtureRepository.Update(ctx, fixture)
}

// SetFixtureDayCaptain sets the day captain for a fixture
func (s *Service) SetFixtureDayCaptain(fixtureID uint, playerID string) error {
	ctx := context.Background()

	// Get the current fixture
	fixture, err := s.fixtureRepository.FindByID(ctx, fixtureID)
	if err != nil {
		return err
	}

	// Update the day captain
	fixture.DayCaptainID = &playerID

	// Save the updated fixture
	return s.fixtureRepository.Update(ctx, fixture)
}

// GetStAnnsNextWeekFixturesByDivision retrieves St Ann's fixtures for the next week organized by division
func (s *Service) GetStAnnsNextWeekFixturesByDivision() (map[string][]FixtureWithRelations, error) {
	ctx := context.Background()

	// Find St. Ann's club
	clubs, err := s.clubRepository.FindByNameLike(ctx, "St Ann")
	if err != nil {
		return nil, err
	}
	if len(clubs) == 0 {
		return make(map[string][]FixtureWithRelations), nil // No club found
	}
	stAnnsClub := &clubs[0]

	// Get all teams for St. Ann's club
	teams, err := s.teamRepository.FindByClub(ctx, stAnnsClub.ID)
	if err != nil {
		return nil, err
	}

	if len(teams) == 0 {
		return make(map[string][]FixtureWithRelations), nil // No teams found
	}

	// Get next week date range
	weekStart, weekEnd := s.getNextWeekDateRange()

	// Get all fixtures for all St. Ann's teams within the next week
	var allFixtures []models.Fixture
	fixtureMap := make(map[uint]models.Fixture) // Use map to deduplicate fixtures by ID

	for _, team := range teams {
		teamFixtures, err := s.fixtureRepository.FindByTeam(ctx, team.ID)
		if err != nil {
			continue // Skip this team if there's an error
		}
		// Filter fixtures for next week and add to map to automatically deduplicate
		for _, fixture := range teamFixtures {
			if fixture.ScheduledDate.After(weekStart) && fixture.ScheduledDate.Before(weekEnd) {
				if fixture.Status == models.Scheduled || fixture.Status == models.InProgress {
					fixtureMap[fixture.ID] = fixture
				}
			}
		}
	}

	// Convert map back to slice
	for _, fixture := range fixtureMap {
		allFixtures = append(allFixtures, fixture)
	}

	// Build FixtureWithRelations by fetching related data
	fixturesWithRelations := s.buildFixturesWithRelations(ctx, allFixtures, stAnnsClub)

	// Organize fixtures by division
	fixturesByDivision := make(map[string][]FixtureWithRelations)

	// Initialize division groups in the order we want (1, 2, 3, 4)
	fixturesByDivision["Division 1"] = []FixtureWithRelations{}
	fixturesByDivision["Division 2"] = []FixtureWithRelations{}
	fixturesByDivision["Division 3"] = []FixtureWithRelations{}
	fixturesByDivision["Division 4"] = []FixtureWithRelations{}

	for _, fixture := range fixturesWithRelations {
		if fixture.Division != nil {
			divisionName := fixture.Division.Name
			fixturesByDivision[divisionName] = append(fixturesByDivision[divisionName], fixture)
		} else {
			// If no division, put in a "Other" category
			if fixturesByDivision["Other"] == nil {
				fixturesByDivision["Other"] = []FixtureWithRelations{}
			}
			fixturesByDivision["Other"] = append(fixturesByDivision["Other"], fixture)
		}
	}

	return fixturesByDivision, nil
}

// UpdateFixtureSchedule updates a fixture's scheduled date and adds the previous date to history
func (s *Service) UpdateFixtureSchedule(fixtureID uint, newScheduledDate time.Time, rescheduleReason models.RescheduledReason, notes string) error {
	ctx := context.Background()

	// Get the current fixture to retrieve the current scheduled date
	currentFixture, err := s.fixtureRepository.FindByID(ctx, fixtureID)
	if err != nil {
		return fmt.Errorf("failed to get current fixture: %w", err)
	}

	// Check if fixture is completed
	if currentFixture.Status == models.Completed {
		return fmt.Errorf("cannot reschedule completed fixture")
	}

	// Prepare the previous dates array
	var previousDates []time.Time

	// Parse existing previous dates from JSON
	if len(currentFixture.PreviousDates) > 0 {
		// Note: PreviousDates is already a []time.Time slice from the model
		previousDates = currentFixture.PreviousDates
	}

	// Add the current scheduled date to previous dates if it's different from the new date
	if !currentFixture.ScheduledDate.Equal(newScheduledDate) {
		// Check if this date is already in the previous dates to avoid duplicates
		dateExists := false
		for _, prevDate := range previousDates {
			if prevDate.Equal(currentFixture.ScheduledDate) {
				dateExists = true
				break
			}
		}

		if !dateExists {
			previousDates = append(previousDates, currentFixture.ScheduledDate)
		}
	}

	// Update the fixture with new data
	updatedFixture := *currentFixture
	updatedFixture.ScheduledDate = newScheduledDate
	updatedFixture.PreviousDates = previousDates
	updatedFixture.RescheduledReason = &rescheduleReason
	if notes != "" {
		updatedFixture.Notes = notes
	}
	updatedFixture.UpdatedAt = time.Now()

	// Save the updated fixture
	err = s.fixtureRepository.Update(ctx, &updatedFixture)
	if err != nil {
		return fmt.Errorf("failed to update fixture: %w", err)
	}

	log.Printf("Fixture %d rescheduled from %v to %v for reason: %s",
		fixtureID, currentFixture.ScheduledDate, newScheduledDate, rescheduleReason)

	return nil
}

// CreateFixture creates a new fixture
func (s *Service) CreateFixture(fixture *models.Fixture) error {
	ctx := context.Background()
	return s.fixtureRepository.Create(ctx, fixture)
}
