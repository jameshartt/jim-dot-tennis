// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"context"
	"fmt"
	"log"
	"strings"

	"jim-dot-tennis/internal/fixturefile"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
	"jim-dot-tennis/internal/services"
)

// SeasonStats holds statistics for a season
type SeasonStats struct {
	SeasonID       uint
	TeamCount      int
	PlayerCount    int
	DivisionCount  int
	FixtureCount   int
	CompletedCount int
}

// DivisionWithTeams holds a division and its teams
type DivisionWithTeams struct {
	Division  models.Division
	Teams     []TeamWithPlayers
	TeamCount int
}

// TeamWithPlayers holds a team with its players and captains
type TeamWithPlayers struct {
	Team         models.Team
	Club         models.Club
	Players      []models.Player
	Captains     []models.Captain
	PlayerCount  int
	CaptainCount int
}

// SeasonSetupData holds all data needed for the season setup page
type SeasonSetupData struct {
	Divisions []DivisionWithTeams
}

// GetAllSeasons retrieves all seasons ordered by year descending
func (s *Service) GetAllSeasons() ([]models.Season, error) {
	ctx := context.Background()
	return s.seasonRepository.FindAll(ctx)
}

// GetActiveSeason retrieves the currently active season
func (s *Service) GetActiveSeason() (*models.Season, error) {
	ctx := context.Background()
	return s.seasonRepository.FindActive(ctx)
}

// CreateSeasonWithWeeks creates a season and automatically generates weeks for
// it. The season and its weeks are written in one transaction.
func (s *Service) CreateSeasonWithWeeks(season *models.Season, numWeeks int) error {
	ctx := context.Background()

	// Calculate the duration of each week
	totalDays := season.EndDate.Sub(season.StartDate).Hours() / 24
	daysPerWeek := totalDays / float64(numWeeks)

	weeks := make([]models.Week, 0, numWeeks)
	for i := 1; i <= numWeeks; i++ {
		weekStart := season.StartDate.AddDate(0, 0, int(float64(i-1)*daysPerWeek))
		weekEnd := season.StartDate.AddDate(0, 0, int(float64(i)*daysPerWeek)-1)

		// For the last week, use the season end date
		if i == numWeeks {
			weekEnd = season.EndDate
		}

		weeks = append(weeks, models.Week{
			WeekNumber: i,
			StartDate:  weekStart,
			EndDate:    weekEnd,
			Name:       fmt.Sprintf("Week %d", i),
			IsActive:   false,
		})
	}

	if err := s.seasonRepository.CreateWithWeeks(ctx, season, weeks); err != nil {
		return err
	}

	log.Printf("Successfully created season '%s' with %d weeks", season.Name, numWeeks)
	return nil
}

// SetActiveSeason sets a season as active and deactivates all others
// (atomically — a failure leaves the previous active season in place).
func (s *Service) SetActiveSeason(seasonID uint) error {
	return s.seasonRepository.SetActive(context.Background(), seasonID)
}

// GetWeeksBySeason retrieves weeks for a specific season
func (s *Service) GetWeeksBySeason(seasonID uint) ([]models.Week, error) {
	ctx := context.Background()
	return s.weekRepository.FindBySeason(ctx, seasonID)
}

// GetWeekCountForSeason returns the number of weeks in a season
func (s *Service) GetWeekCountForSeason(seasonID uint) (int, error) {
	ctx := context.Background()
	return s.weekRepository.CountBySeason(ctx, seasonID)
}

// GetSeasonStats retrieves statistics for a season
func (s *Service) GetSeasonStats(seasonID uint) (*SeasonStats, error) {
	ctx := context.Background()

	stats := &SeasonStats{
		SeasonID: seasonID,
	}

	// Count teams in this season
	teams, err := s.teamRepository.FindBySeason(ctx, seasonID)
	if err == nil {
		stats.TeamCount = len(teams)
	}

	// Count divisions in this season
	divisions, err := s.divisionRepository.FindBySeason(ctx, seasonID)
	if err == nil {
		stats.DivisionCount = len(divisions)
	}

	// Count fixtures in this season
	fixtures, err := s.fixtureRepository.FindBySeason(ctx, seasonID)
	if err == nil {
		stats.FixtureCount = len(fixtures)
		// Count completed fixtures
		for _, f := range fixtures {
			if f.Status == models.Completed {
				stats.CompletedCount++
			}
		}
	}

	// Count unique players in this season (via player_teams table)
	// For now, we'll just set it to 0 or implement if needed
	stats.PlayerCount = 0

	return stats, nil
}

// GetSeasonByID retrieves a season by its ID
func (s *Service) GetSeasonByID(seasonID uint) (*models.Season, error) {
	ctx := context.Background()
	return s.seasonRepository.FindByID(ctx, seasonID)
}

// GetSeasonSetupData retrieves all data needed for season setup page
func (s *Service) GetSeasonSetupData(seasonID uint) (*SeasonSetupData, error) {
	ctx := context.Background()

	setupData := &SeasonSetupData{
		Divisions: []DivisionWithTeams{},
	}

	// Get all divisions for this season
	divisions, err := s.divisionRepository.FindBySeason(ctx, seasonID)
	if err != nil {
		return nil, err
	}

	// For each division, get its teams with details
	for _, division := range divisions {
		divisionData := DivisionWithTeams{
			Division:  division,
			Teams:     []TeamWithPlayers{},
			TeamCount: 0,
		}

		// Get teams in this division
		teams, err := s.teamRepository.FindByDivisionAndSeason(ctx, division.ID, seasonID)
		if err != nil {
			continue
		}

		divisionData.TeamCount = len(teams)

		// For each team, get its players and captains
		for _, team := range teams {
			teamData := TeamWithPlayers{
				Team:         team,
				Players:      []models.Player{},
				Captains:     []models.Captain{},
				PlayerCount:  0,
				CaptainCount: 0,
			}

			// Get club for this team
			club, err := s.clubRepository.FindByID(ctx, team.ClubID)
			if err == nil {
				teamData.Club = *club
			}

			// Get player count for this team
			playerCount, err := s.teamRepository.CountPlayers(ctx, team.ID, seasonID)
			if err == nil {
				teamData.PlayerCount = playerCount
			}

			// Get captains for this team
			captains, err := s.teamRepository.FindCaptainsInTeam(ctx, team.ID, seasonID)
			if err == nil {
				teamData.Captains = captains
				teamData.CaptainCount = len(captains)
			}

			divisionData.Teams = append(divisionData.Teams, teamData)
		}

		setupData.Divisions = append(setupData.Divisions, divisionData)
	}

	return setupData, nil
}

// DeleteSeason deletes a season and all dependent data after safety checks
func (s *Service) DeleteSeason(seasonID uint) (*repository.SeasonDeletionStats, error) {
	ctx := context.Background()

	// Get the season
	season, err := s.seasonRepository.FindByID(ctx, seasonID)
	if err != nil {
		return nil, fmt.Errorf("season not found: %w", err)
	}

	// Safety: cannot delete the active season
	if season.IsActive {
		return nil, fmt.Errorf("cannot delete the active season — deactivate it first")
	}

	// Safety: cannot delete if there are completed fixtures
	fixtures, err := s.fixtureRepository.FindBySeason(ctx, seasonID)
	if err != nil {
		return nil, fmt.Errorf("failed to check fixtures: %w", err)
	}
	for _, f := range fixtures {
		if f.Status == models.Completed {
			return nil, fmt.Errorf("cannot delete season with completed fixtures — this season has match results that would be lost")
		}
	}

	// Perform cascading delete
	stats, err := s.seasonRepository.DeleteCascade(ctx, seasonID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete season: %w", err)
	}

	log.Printf("Deleted season '%s' (ID %d): %d fixtures, %d teams, %d divisions, %d weeks, %d player assignments, %d captains",
		season.Name, seasonID, stats.Fixtures, stats.Teams, stats.Divisions, stats.Weeks, stats.Players, stats.Captains)

	return stats, nil
}

// MoveTeamToDivision promotes/demotes a team to a different division
func (s *Service) MoveTeamToDivision(teamID uint, targetDivisionID uint) error {
	ctx := context.Background()
	return s.teamRepository.UpdateDivision(ctx, teamID, targetDivisionID)
}

// CopyFromPreviousSeason copies divisions and/or teams from the previous season
// to the target season. The copy is all-or-nothing: any failure (including a
// player or captain insert) is returned and nothing is written.
func (s *Service) CopyFromPreviousSeason(targetSeasonID uint, copyDivisions, copyTeams bool) error {
	ctx := context.Background()

	// Get the target season
	targetSeason, err := s.seasonRepository.FindByID(ctx, targetSeasonID)
	if err != nil {
		return fmt.Errorf("failed to find target season: %w", err)
	}

	// Find the previous season (by year)
	previousYear := targetSeason.Year - 1
	previousSeasons, err := s.seasonRepository.FindByYear(ctx, previousYear)
	if err != nil || len(previousSeasons) == 0 {
		return fmt.Errorf("no season found for year %d", previousYear)
	}

	return s.seasonRepository.CopyStructure(ctx, previousSeasons[0].ID, targetSeasonID, copyDivisions, copyTeams)
}

// ImportSummary holds counts of what was created during a season import
type ImportSummary struct {
	DivisionsCreated int
	TeamsCreated     int
	ClubsCreated     int
	FixturesCreated  int
	PlayersCopied    int
	CaptainsCopied   int
	Errors           []string
}

// CopyFromPreviousSeasonWithFixtures creates divisions and teams based on scraped BHPLTA fixture data,
// copies players/captains from the previous season's matching teams, and creates all fixture records.
func (s *Service) CopyFromPreviousSeasonWithFixtures(
	targetSeasonID uint,
	fixturesURL string,
	copyPlayers bool,
) (ImportSummary, error) {
	scrapedDivisions, err := services.ScrapeFixtures(fixturesURL)
	if err != nil {
		return ImportSummary{}, fmt.Errorf("failed to scrape fixtures: %w", err)
	}
	return s.importSeasonDivisions(targetSeasonID, scrapedDivisions, seasonImportOptions{
		copyPlayers:           copyPlayers,
		requirePreviousSeason: true,
	})
}

// ImportFixtureFile loads a fixture file written by the fixture generator
// into a season: its divisions, teams and every fixture. Clubs the file names
// that this instance has never seen are created, so a club new to jim.tennis
// can start from the file alone.
func (s *Service) ImportFixtureFile(targetSeasonID uint, file *fixturefile.File, copyPlayers bool) (ImportSummary, error) {
	ctx := context.Background()
	season, err := s.seasonRepository.FindByID(ctx, targetSeasonID)
	if err != nil {
		return ImportSummary{}, fmt.Errorf("failed to find target season: %w", err)
	}
	if season.Year != file.Season.Year {
		return ImportSummary{}, fmt.Errorf("the fixture file is for the %d season, but %s is %d",
			file.Season.Year, season.Name, season.Year)
	}
	weekCount, err := s.weekRepository.CountBySeason(ctx, targetSeasonID)
	if err != nil {
		return ImportSummary{}, fmt.Errorf("failed to count weeks: %w", err)
	}
	if weekCount < file.Season.Weeks {
		return ImportSummary{}, fmt.Errorf("the fixture file has %d weeks, but %s has %d",
			file.Season.Weeks, season.Name, weekCount)
	}
	existing, err := s.divisionRepository.FindBySeason(ctx, targetSeasonID)
	if err != nil {
		return ImportSummary{}, fmt.Errorf("failed to check existing divisions: %w", err)
	}
	if len(existing) > 0 {
		return ImportSummary{}, fmt.Errorf("%s already has divisions; fixture files load into an empty season", season.Name)
	}

	divisions := make([]services.ScrapedDivision, 0, len(file.Divisions))
	config := make(map[string]divisionConfig, len(file.Divisions))
	for _, d := range file.Divisions {
		sd := services.ScrapedDivision{Name: d.Name}
		for _, t := range d.Teams {
			sd.Teams = append(sd.Teams, t.Name)
		}
		for _, f := range d.Fixtures {
			sd.Fixtures = append(sd.Fixtures, services.ScrapedFixture{
				Division: d.Name,
				Week:     f.Week,
				Date:     f.Time(),
				HomeTeam: f.Home,
				AwayTeam: f.Away,
			})
		}
		divisions = append(divisions, sd)
		config[d.Name] = divisionConfig{level: d.Level, playDay: d.PlayDay}
	}
	clubs := make(map[string]string)
	for _, d := range file.Divisions {
		for _, t := range d.Teams {
			clubs[t.Name] = t.Club
		}
	}
	// Nothing below runs in one transaction, so make sure the clubs can be
	// read before the first row is written.
	if _, err := s.clubRepository.FindAll(ctx); err != nil {
		return ImportSummary{}, fmt.Errorf("failed to read clubs: %w", err)
	}

	return s.importSeasonDivisions(targetSeasonID, divisions, seasonImportOptions{
		copyPlayers:    copyPlayers,
		divisionConfig: config,
		teamClubs:      clubs,
		createClubs:    true,
		leagueName:     file.League.Name,
	})
}

// divisionConfig carries a division's settings when there is no previous
// season to copy them from.
type divisionConfig struct {
	level   int
	playDay string
}

// seasonImportOptions tunes importSeasonDivisions for its two sources: the
// BHPLTA fixtures page and a fixture file.
type seasonImportOptions struct {
	copyPlayers           bool
	requirePreviousSeason bool
	divisionConfig        map[string]divisionConfig // by division name, used when the previous season has no match
	teamClubs             map[string]string         // team name → club name, when the source states it
	createClubs           bool                      // create clubs this instance does not know
	leagueName            string                    // league to create if none exists
}

// importSeasonDivisions creates divisions, teams and fixtures for a season,
// copying division settings, players and captains from the previous season's
// matching divisions and teams where they exist.
func (s *Service) importSeasonDivisions(
	targetSeasonID uint,
	scrapedDivisions []services.ScrapedDivision,
	opts seasonImportOptions,
) (ImportSummary, error) {
	ctx := context.Background()
	summary := ImportSummary{}
	copyPlayers := opts.copyPlayers

	// 2. Get target season
	targetSeason, err := s.seasonRepository.FindByID(ctx, targetSeasonID)
	if err != nil {
		return summary, fmt.Errorf("failed to find target season: %w", err)
	}

	// 3. Get previous season (by year - 1) for copying config and players
	previousYear := targetSeason.Year - 1
	var previousSeason models.Season
	var prevDivisions []models.Division
	previousSeasons, err := s.seasonRepository.FindByYear(ctx, previousYear)
	hasPrevious := err == nil && len(previousSeasons) > 0
	if !hasPrevious && opts.requirePreviousSeason {
		return summary, fmt.Errorf("no season found for year %d", previousYear)
	}
	if hasPrevious {
		previousSeason = previousSeasons[0]

		// 4. Get previous season's divisions to copy config (league_id, play_day, level, max_teams_per_club)
		prevDivisions, err = s.divisionRepository.FindBySeason(ctx, previousSeason.ID)
		if err != nil {
			return summary, fmt.Errorf("failed to find previous season divisions: %w", err)
		}
	}
	prevDivByName := make(map[string]models.Division)
	for _, d := range prevDivisions {
		prevDivByName[d.Name] = d
	}

	// A division needs a league. Reuse last season's, else any, else create one.
	var fallbackLeagueID uint
	if len(prevDivisions) > 0 {
		fallbackLeagueID = prevDivisions[0].LeagueID
	} else if len(opts.divisionConfig) > 0 {
		fallbackLeagueID, err = s.findOrCreateLeague(ctx, opts.leagueName, targetSeason.Year)
		if err != nil {
			return summary, err
		}
	}

	// 5. Create divisions for target season based on scraped data
	newDivByName := make(map[string]uint)
	for _, sd := range scrapedDivisions {
		newDiv := &models.Division{
			Name:     sd.Name,
			SeasonID: targetSeasonID,
		}

		// Copy config from previous season's matching division
		if prevDiv, ok := prevDivByName[sd.Name]; ok {
			newDiv.Level = prevDiv.Level
			newDiv.PlayDay = prevDiv.PlayDay
			newDiv.LeagueID = prevDiv.LeagueID
			newDiv.MaxTeamsPerClub = prevDiv.MaxTeamsPerClub
		} else if cfg, ok := opts.divisionConfig[sd.Name]; ok {
			newDiv.Level = cfg.level
			newDiv.PlayDay = cfg.playDay
			newDiv.LeagueID = fallbackLeagueID
			newDiv.MaxTeamsPerClub = 2
		} else {
			// Fallback: try to infer level from name
			summary.Errors = append(summary.Errors,
				fmt.Sprintf("no previous division config found for %q, using defaults", sd.Name))
			newDiv.LeagueID = fallbackLeagueID
		}

		if err := s.divisionRepository.Create(ctx, newDiv); err != nil {
			return summary, fmt.Errorf("failed to create division %s: %w", sd.Name, err)
		}
		newDivByName[sd.Name] = newDiv.ID
		summary.DivisionsCreated++
	}

	// 6. Create teams for target season based on scraped data
	// Build a map of team name -> new team ID for fixture creation
	teamIDMap := make(map[string]uint) // team name -> new team ID

	// Get previous season teams for player/captain copying
	prevTeamByName := make(map[string]models.Team)
	if hasPrevious {
		prevTeams, _ := s.teamRepository.FindBySeason(ctx, previousSeason.ID)
		for _, t := range prevTeams {
			prevTeamByName[t.Name] = t
		}
	}

	for _, sd := range scrapedDivisions {
		divisionID, ok := newDivByName[sd.Name]
		if !ok {
			continue
		}

		for _, teamName := range sd.Teams {
			// The source may state the club; otherwise derive it from the team name
			clubName := opts.teamClubs[teamName]
			if clubName == "" {
				clubName = repository.ExtractClubNameFromTeamName(teamName)
			}

			// Find the club — truncate at apostrophe for LIKE search to handle encoding variants
			// e.g. "St Ann's" -> search for "St Ann" which matches both "St Ann's" and "St Ann's"
			searchName := clubName
			if idx := strings.IndexAny(searchName, "'\u2019"); idx > 0 {
				searchName = searchName[:idx]
			}
			clubs, err := s.clubRepository.FindByNameLike(ctx, searchName)
			if err != nil {
				// A failed lookup is not an absent club: never create on error.
				summary.Errors = append(summary.Errors,
					fmt.Sprintf("club lookup failed for team %q (looked for %q): %v", teamName, clubName, err))
				continue
			}
			if len(clubs) == 0 && opts.createClubs {
				created := &models.Club{Name: clubName}
				if cErr := s.clubRepository.Create(ctx, created); cErr == nil {
					clubs = []models.Club{*created}
					summary.ClubsCreated++
				}
			}
			if len(clubs) == 0 {
				summary.Errors = append(summary.Errors,
					fmt.Sprintf("club not found for team %q (looked for %q)", teamName, clubName))
				continue
			}
			club := clubs[0]

			// Create new team
			newTeam := &models.Team{
				Name:       teamName,
				ClubID:     club.ID,
				DivisionID: divisionID,
				SeasonID:   targetSeasonID,
			}

			if err := s.teamRepository.Create(ctx, newTeam); err != nil {
				summary.Errors = append(summary.Errors,
					fmt.Sprintf("failed to create team %q: %v", teamName, err))
				continue
			}

			teamIDMap[teamName] = newTeam.ID
			summary.TeamsCreated++

			// Copy players and captains from previous season's matching team (skip inactive players)
			if copyPlayers {
				if prevTeam, ok := prevTeamByName[teamName]; ok {
					// Copy players
					oldPlayers, err := s.teamRepository.FindPlayersInTeam(ctx, prevTeam.ID, previousSeason.ID)
					if err == nil {
						for _, pt := range oldPlayers {
							player, pErr := s.playerRepository.FindByID(ctx, pt.PlayerID)
							if pErr != nil || !player.IsActive {
								continue // Skip inactive or missing players
							}
							if err := s.teamRepository.AddPlayer(ctx, newTeam.ID, pt.PlayerID, targetSeasonID); err == nil {
								summary.PlayersCopied++
							}
						}
					}

					// Copy captains
					oldCaptains, err := s.teamRepository.FindCaptainsInTeam(ctx, prevTeam.ID, previousSeason.ID)
					if err == nil {
						for _, c := range oldCaptains {
							player, pErr := s.playerRepository.FindByID(ctx, c.PlayerID)
							if pErr != nil || !player.IsActive {
								continue // Skip inactive or missing players
							}
							if err := s.teamRepository.AddCaptain(ctx, newTeam.ID, c.PlayerID, c.Role, targetSeasonID); err == nil {
								summary.CaptainsCopied++
							}
						}
					}
				}
			}
		}
	}

	// 7. Create fixtures
	for _, sd := range scrapedDivisions {
		divisionID, ok := newDivByName[sd.Name]
		if !ok {
			continue
		}

		for _, sf := range sd.Fixtures {
			homeTeamID, homeOk := teamIDMap[sf.HomeTeam]
			awayTeamID, awayOk := teamIDMap[sf.AwayTeam]
			if !homeOk || !awayOk {
				summary.Errors = append(summary.Errors,
					fmt.Sprintf("skipping fixture %s v %s (week %d): team not found",
						sf.HomeTeam, sf.AwayTeam, sf.Week))
				continue
			}

			// Look up week by number
			week, err := s.weekRepository.FindByWeekNumber(ctx, targetSeasonID, sf.Week)
			if err != nil {
				summary.Errors = append(summary.Errors,
					fmt.Sprintf("week %d not found for season %d: %v", sf.Week, targetSeasonID, err))
				continue
			}

			fixture := &models.Fixture{
				HomeTeamID:    homeTeamID,
				AwayTeamID:    awayTeamID,
				DivisionID:    divisionID,
				SeasonID:      targetSeasonID,
				WeekID:        week.ID,
				ScheduledDate: sf.Date,
				VenueLocation: "TBD",
				Status:        models.Scheduled,
			}

			if err := s.fixtureRepository.Create(ctx, fixture); err != nil {
				summary.Errors = append(summary.Errors,
					fmt.Sprintf("failed to create fixture %s v %s: %v",
						sf.HomeTeam, sf.AwayTeam, err))
				continue
			}
			summary.FixturesCreated++
		}
	}

	log.Printf("Season import complete: %d divisions, %d teams, %d fixtures, %d players, %d captains, %d errors",
		summary.DivisionsCreated, summary.TeamsCreated, summary.FixturesCreated,
		summary.PlayersCopied, summary.CaptainsCopied, len(summary.Errors))

	return summary, nil
}

// findOrCreateLeague returns a league for new divisions to belong to.
func (s *Service) findOrCreateLeague(ctx context.Context, name string, year int) (uint, error) {
	leagues, err := s.leagueRepository.FindAll(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to list leagues: %w", err)
	}
	if len(leagues) > 0 {
		return leagues[0].ID, nil
	}
	if name == "" {
		name = "Parks League"
	}
	league := &models.League{Name: name, Type: models.ParksLeague, Year: year}
	if err := s.leagueRepository.Create(ctx, league); err != nil {
		return 0, fmt.Errorf("failed to create league: %w", err)
	}
	return league.ID, nil
}
