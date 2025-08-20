package repository

import (
	"context"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
	"time"
)

// FixtureRepository defines the interface for fixture data access
type FixtureRepository interface {
	// Basic CRUD operations
	FindAll(ctx context.Context) ([]models.Fixture, error)
	FindByID(ctx context.Context, id uint) (*models.Fixture, error)
	Create(ctx context.Context, fixture *models.Fixture) error
	Update(ctx context.Context, fixture *models.Fixture) error
	Delete(ctx context.Context, id uint) error

	// Fixture-specific queries
	FindByDivision(ctx context.Context, divisionID uint) ([]models.Fixture, error)
	FindBySeason(ctx context.Context, seasonID uint) ([]models.Fixture, error)
	FindByDivisionAndSeason(ctx context.Context, divisionID, seasonID uint) ([]models.Fixture, error)
	FindByTeam(ctx context.Context, teamID uint) ([]models.Fixture, error)
	FindByHomeTeam(ctx context.Context, teamID uint) ([]models.Fixture, error)
	FindByAwayTeam(ctx context.Context, teamID uint) ([]models.Fixture, error)
	FindByStatus(ctx context.Context, status models.FixtureStatus) ([]models.Fixture, error)
	FindByDateRange(ctx context.Context, startDate, endDate time.Time) ([]models.Fixture, error)
	// Club-scoped queries
	FindByClubAndDateRange(ctx context.Context, clubID uint, startDate, endDate time.Time) ([]models.Fixture, error)
	FindByDayCaptain(ctx context.Context, dayCaptainID string) ([]models.Fixture, error)

	// Week-related queries
	FindByWeek(ctx context.Context, weekID uint) ([]models.Fixture, error)
	FindByWeekNumber(ctx context.Context, seasonID uint, weekNumber int) ([]models.Fixture, error)
	FindByWeekAndDivision(ctx context.Context, weekID, divisionID uint) ([]models.Fixture, error)

	// Fixture with relationships
	FindWithMatchups(ctx context.Context, id uint) (*models.Fixture, error)

	// Status management
	UpdateStatus(ctx context.Context, id uint, status models.FixtureStatus) error
	CompleteFixture(ctx context.Context, id uint, completedDate time.Time) error
	SetDayCaptain(ctx context.Context, id uint, dayCaptainID string) error
	RemoveDayCaptain(ctx context.Context, id uint) error

	// Date and scheduling queries
	FindUpcoming(ctx context.Context, limit int) ([]models.Fixture, error)
	FindToday(ctx context.Context) ([]models.Fixture, error)
	FindThisWeek(ctx context.Context) ([]models.Fixture, error)
	FindByWeekday(ctx context.Context, weekday time.Weekday) ([]models.Fixture, error)
	FindOverdue(ctx context.Context) ([]models.Fixture, error)

	// Statistics
	CountByStatus(ctx context.Context, status models.FixtureStatus) (int, error)
	CountByDivision(ctx context.Context, divisionID uint) (int, error)
	CountBySeason(ctx context.Context, seasonID uint) (int, error)
	CountByWeek(ctx context.Context, weekID uint) (int, error)

	// Fixture Player Selection methods
	FindSelectedPlayers(ctx context.Context, fixtureID uint) ([]models.FixturePlayer, error)
	FindSelectedPlayersByTeam(ctx context.Context, fixtureID, managingTeamID uint) ([]models.FixturePlayer, error)
	AddSelectedPlayer(ctx context.Context, fixturePlayer *models.FixturePlayer) error
	RemoveSelectedPlayer(ctx context.Context, fixtureID uint, playerID string) error
	RemoveSelectedPlayerByTeam(ctx context.Context, fixtureID, managingTeamID uint, playerID string) error
	UpdateSelectedPlayerPosition(ctx context.Context, fixtureID uint, playerID string, position int) error
	ClearSelectedPlayers(ctx context.Context, fixtureID uint) error
	ClearSelectedPlayersByTeam(ctx context.Context, fixtureID, managingTeamID uint) error

	// Player-specific fixture queries
	FindUpcomingFixturesForPlayer(ctx context.Context, playerID string) ([]models.Fixture, error)
}

// fixtureRepository implements FixtureRepository
type fixtureRepository struct {
	db *database.DB
}

// NewFixtureRepository creates a new fixture repository
func NewFixtureRepository(db *database.DB) FixtureRepository {
	return &fixtureRepository{
		db: db,
	}
}

// FindAll retrieves all fixtures ordered by scheduled date
func (r *fixtureRepository) FindAll(ctx context.Context) ([]models.Fixture, error) {
	var fixtures []models.Fixture
	err := r.db.SelectContext(ctx, &fixtures, `
		SELECT id, home_team_id, away_team_id, division_id, season_id, week_id, scheduled_date, 
		       venue_location, status, completed_date, day_captain_id, external_match_card_id, notes, created_at, updated_at
		FROM fixtures 
		ORDER BY scheduled_date ASC
	`)
	return fixtures, err
}

// FindByID retrieves a fixture by its ID
func (r *fixtureRepository) FindByID(ctx context.Context, id uint) (*models.Fixture, error) {
	var fixture models.Fixture
	err := r.db.GetContext(ctx, &fixture, `
		SELECT id, home_team_id, away_team_id, division_id, season_id, week_id, scheduled_date, 
		       venue_location, status, completed_date, day_captain_id, external_match_card_id, notes, created_at, updated_at
		FROM fixtures 
		WHERE id = ?
	`, id)
	if err != nil {
		return nil, err
	}
	return &fixture, nil
}

// Create inserts a new fixture record
func (r *fixtureRepository) Create(ctx context.Context, fixture *models.Fixture) error {
	now := time.Now()
	fixture.CreatedAt = now
	fixture.UpdatedAt = now

	result, err := r.db.NamedExecContext(ctx, `
		INSERT INTO fixtures (home_team_id, away_team_id, division_id, season_id, week_id, scheduled_date, 
		                     venue_location, status, completed_date, day_captain_id, external_match_card_id, notes, created_at, updated_at)
		VALUES (:home_team_id, :away_team_id, :division_id, :season_id, :week_id, :scheduled_date, 
		        :venue_location, :status, :completed_date, :day_captain_id, :external_match_card_id, :notes, :created_at, :updated_at)
	`, fixture)

	if err != nil {
		return err
	}

	// Get the last inserted ID and set it on the fixture
	if id, err := result.LastInsertId(); err == nil {
		fixture.ID = uint(id)
	}

	return nil
}

// Update modifies an existing fixture record
func (r *fixtureRepository) Update(ctx context.Context, fixture *models.Fixture) error {
	fixture.UpdatedAt = time.Now()

	_, err := r.db.NamedExecContext(ctx, `
		UPDATE fixtures 
		SET home_team_id = :home_team_id, away_team_id = :away_team_id, division_id = :division_id, 
		    season_id = :season_id, week_id = :week_id, scheduled_date = :scheduled_date, venue_location = :venue_location, 
		    status = :status, completed_date = :completed_date, day_captain_id = :day_captain_id, 
		    external_match_card_id = :external_match_card_id, notes = :notes, updated_at = :updated_at
		WHERE id = :id
	`, fixture)

	return err
}

// Delete removes a fixture record by ID
func (r *fixtureRepository) Delete(ctx context.Context, id uint) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM fixtures WHERE id = ?`, id)
	return err
}

// FindByDivision retrieves all fixtures for a specific division
func (r *fixtureRepository) FindByDivision(ctx context.Context, divisionID uint) ([]models.Fixture, error) {
	var fixtures []models.Fixture
	err := r.db.SelectContext(ctx, &fixtures, `
		SELECT id, home_team_id, away_team_id, division_id, season_id, week_id, scheduled_date, 
		       venue_location, status, completed_date, day_captain_id, external_match_card_id, notes, created_at, updated_at
		FROM fixtures 
		WHERE division_id = ?
		ORDER BY scheduled_date ASC
	`, divisionID)
	return fixtures, err
}

// FindBySeason retrieves all fixtures for a specific season
func (r *fixtureRepository) FindBySeason(ctx context.Context, seasonID uint) ([]models.Fixture, error) {
	var fixtures []models.Fixture
	err := r.db.SelectContext(ctx, &fixtures, `
		SELECT id, home_team_id, away_team_id, division_id, season_id, week_id, scheduled_date, 
		       venue_location, status, completed_date, day_captain_id, notes, created_at, updated_at
		FROM fixtures 
		WHERE season_id = ?
		ORDER BY scheduled_date ASC
	`, seasonID)
	return fixtures, err
}

// FindByDivisionAndSeason retrieves fixtures for a specific division and season
func (r *fixtureRepository) FindByDivisionAndSeason(ctx context.Context, divisionID, seasonID uint) ([]models.Fixture, error) {
	var fixtures []models.Fixture
	err := r.db.SelectContext(ctx, &fixtures, `
		SELECT id, home_team_id, away_team_id, division_id, season_id, week_id, scheduled_date, 
		       venue_location, status, completed_date, day_captain_id, notes, created_at, updated_at
		FROM fixtures 
		WHERE division_id = ? AND season_id = ?
		ORDER BY scheduled_date ASC
	`, divisionID, seasonID)
	return fixtures, err
}

// FindByTeam retrieves all fixtures for a specific team (home or away)
func (r *fixtureRepository) FindByTeam(ctx context.Context, teamID uint) ([]models.Fixture, error) {
	var fixtures []models.Fixture
	err := r.db.SelectContext(ctx, &fixtures, `
		SELECT id, home_team_id, away_team_id, division_id, season_id, week_id, scheduled_date, 
		       venue_location, status, completed_date, day_captain_id, notes, created_at, updated_at
		FROM fixtures 
		WHERE home_team_id = ? OR away_team_id = ?
		ORDER BY scheduled_date ASC
	`, teamID, teamID)
	return fixtures, err
}

// FindByHomeTeam retrieves all fixtures where a team is playing at home
func (r *fixtureRepository) FindByHomeTeam(ctx context.Context, teamID uint) ([]models.Fixture, error) {
	var fixtures []models.Fixture
	err := r.db.SelectContext(ctx, &fixtures, `
		SELECT id, home_team_id, away_team_id, division_id, season_id, week_id, scheduled_date, 
		       venue_location, status, completed_date, day_captain_id, notes, created_at, updated_at
		FROM fixtures 
		WHERE home_team_id = ?
		ORDER BY scheduled_date ASC
	`, teamID)
	return fixtures, err
}

// FindByAwayTeam retrieves all fixtures where a team is playing away
func (r *fixtureRepository) FindByAwayTeam(ctx context.Context, teamID uint) ([]models.Fixture, error) {
	var fixtures []models.Fixture
	err := r.db.SelectContext(ctx, &fixtures, `
		SELECT id, home_team_id, away_team_id, division_id, season_id, week_id, scheduled_date, 
		       venue_location, status, completed_date, day_captain_id, notes, created_at, updated_at
		FROM fixtures 
		WHERE away_team_id = ?
		ORDER BY scheduled_date ASC
	`, teamID)
	return fixtures, err
}

// FindByStatus retrieves all fixtures with a specific status
func (r *fixtureRepository) FindByStatus(ctx context.Context, status models.FixtureStatus) ([]models.Fixture, error) {
	var fixtures []models.Fixture
	err := r.db.SelectContext(ctx, &fixtures, `
		SELECT id, home_team_id, away_team_id, division_id, season_id, week_id, scheduled_date, 
		       venue_location, status, completed_date, day_captain_id, notes, created_at, updated_at
		FROM fixtures 
		WHERE status = ?
		ORDER BY scheduled_date ASC
	`, string(status))
	return fixtures, err
}

// FindByDateRange retrieves fixtures within a specific date range
func (r *fixtureRepository) FindByDateRange(ctx context.Context, startDate, endDate time.Time) ([]models.Fixture, error) {
	var fixtures []models.Fixture
	err := r.db.SelectContext(ctx, &fixtures, `
		SELECT id, home_team_id, away_team_id, division_id, season_id, week_id, scheduled_date, 
		       venue_location, status, completed_date, day_captain_id, notes, created_at, updated_at
		FROM fixtures 
		WHERE scheduled_date >= ? AND scheduled_date <= ?
		ORDER BY scheduled_date ASC
	`, startDate, endDate)
	return fixtures, err
}

// FindByClubAndDateRange retrieves fixtures within a date range where either home or away team belongs to the given club
func (r *fixtureRepository) FindByClubAndDateRange(ctx context.Context, clubID uint, startDate, endDate time.Time) ([]models.Fixture, error) {
	var fixtures []models.Fixture
	err := r.db.SelectContext(ctx, &fixtures, `
        SELECT f.id, f.home_team_id, f.away_team_id, f.division_id, f.season_id, f.week_id,
               f.scheduled_date, f.venue_location, f.status, f.completed_date, f.day_captain_id,
               f.external_match_card_id, f.notes, f.created_at, f.updated_at
        FROM fixtures f
        INNER JOIN teams th ON th.id = f.home_team_id
        INNER JOIN teams ta ON ta.id = f.away_team_id
        WHERE (th.club_id = ? OR ta.club_id = ?)
          AND f.scheduled_date >= ? AND f.scheduled_date <= ?
          AND f.status IN (?, ?)
        ORDER BY f.scheduled_date ASC
    `, clubID, clubID, startDate, endDate, string(models.Scheduled), string(models.InProgress))
	return fixtures, err
}

// FindByDayCaptain retrieves all fixtures assigned to a specific day captain
func (r *fixtureRepository) FindByDayCaptain(ctx context.Context, dayCaptainID string) ([]models.Fixture, error) {
	var fixtures []models.Fixture
	err := r.db.SelectContext(ctx, &fixtures, `
		SELECT id, home_team_id, away_team_id, division_id, season_id, week_id, scheduled_date, 
		       venue_location, status, completed_date, day_captain_id, notes, created_at, updated_at
		FROM fixtures 
		WHERE day_captain_id = ?
		ORDER BY scheduled_date ASC
	`, dayCaptainID)
	return fixtures, err
}

// FindWithMatchups retrieves a fixture with its associated matchups
func (r *fixtureRepository) FindWithMatchups(ctx context.Context, id uint) (*models.Fixture, error) {
	// First get the fixture
	fixture, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Then get associated matchups
	var matchups []models.Matchup
	err = r.db.SelectContext(ctx, &matchups, `
		SELECT id, fixture_id, type, status, home_score, away_score, notes, managing_team_id, created_at, updated_at
		FROM matchups
		WHERE fixture_id = ?
		ORDER BY type ASC
	`, id)

	if err != nil {
		return nil, err
	}

	fixture.Matchups = matchups
	return fixture, nil
}

// UpdateStatus updates the status of a fixture
func (r *fixtureRepository) UpdateStatus(ctx context.Context, id uint, status models.FixtureStatus) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE fixtures 
		SET status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, string(status), id)
	return err
}

// CompleteFixture marks a fixture as completed with a completion date
func (r *fixtureRepository) CompleteFixture(ctx context.Context, id uint, completedDate time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE fixtures 
		SET status = ?, completed_date = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, string(models.Completed), completedDate, id)
	return err
}

// SetDayCaptain assigns a day captain to a fixture
func (r *fixtureRepository) SetDayCaptain(ctx context.Context, id uint, dayCaptainID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE fixtures 
		SET day_captain_id = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, dayCaptainID, id)
	return err
}

// RemoveDayCaptain removes the day captain assignment from a fixture
func (r *fixtureRepository) RemoveDayCaptain(ctx context.Context, id uint) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE fixtures 
		SET day_captain_id = NULL, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, id)
	return err
}

// FindUpcoming retrieves upcoming fixtures (scheduled or in progress) limited by count
func (r *fixtureRepository) FindUpcoming(ctx context.Context, limit int) ([]models.Fixture, error) {
	var fixtures []models.Fixture
	err := r.db.SelectContext(ctx, &fixtures, `
		SELECT id, home_team_id, away_team_id, division_id, season_id, week_id, scheduled_date, 
		       venue_location, status, completed_date, day_captain_id, notes, created_at, updated_at
		FROM fixtures 
		WHERE scheduled_date >= CURRENT_TIMESTAMP AND status IN (?, ?)
		ORDER BY scheduled_date ASC
		LIMIT ?
	`, string(models.Scheduled), string(models.InProgress), limit)
	return fixtures, err
}

// FindToday retrieves all fixtures scheduled for today
func (r *fixtureRepository) FindToday(ctx context.Context) ([]models.Fixture, error) {
	var fixtures []models.Fixture
	err := r.db.SelectContext(ctx, &fixtures, `
		SELECT id, home_team_id, away_team_id, division_id, season_id, week_id, scheduled_date, 
		       venue_location, status, completed_date, day_captain_id, notes, created_at, updated_at
		FROM fixtures 
		WHERE DATE(scheduled_date) = DATE(CURRENT_TIMESTAMP)
		ORDER BY scheduled_date ASC
	`)
	return fixtures, err
}

// FindThisWeek retrieves all fixtures scheduled for this week
func (r *fixtureRepository) FindThisWeek(ctx context.Context) ([]models.Fixture, error) {
	var fixtures []models.Fixture
	err := r.db.SelectContext(ctx, &fixtures, `
		SELECT id, home_team_id, away_team_id, division_id, season_id, week_id, scheduled_date, 
		       venue_location, status, completed_date, day_captain_id, notes, created_at, updated_at
		FROM fixtures 
		WHERE scheduled_date >= DATE('now', 'weekday 0', '-6 days') 
		  AND scheduled_date < DATE('now', 'weekday 0', '+1 day')
		ORDER BY scheduled_date ASC
	`)
	return fixtures, err
}

// FindByWeekday retrieves all fixtures scheduled for a specific weekday
func (r *fixtureRepository) FindByWeekday(ctx context.Context, weekday time.Weekday) ([]models.Fixture, error) {
	var fixtures []models.Fixture
	// SQLite uses 0=Sunday, 1=Monday, etc., same as Go's time.Weekday
	err := r.db.SelectContext(ctx, &fixtures, `
		SELECT id, home_team_id, away_team_id, division_id, season_id, week_id, scheduled_date, 
		       venue_location, status, completed_date, day_captain_id, notes, created_at, updated_at
		FROM fixtures 
		WHERE CAST(strftime('%w', scheduled_date) AS INTEGER) = ?
		ORDER BY scheduled_date ASC
	`, int(weekday))
	return fixtures, err
}

// FindOverdue retrieves fixtures that are scheduled but past their scheduled date
func (r *fixtureRepository) FindOverdue(ctx context.Context) ([]models.Fixture, error) {
	var fixtures []models.Fixture
	err := r.db.SelectContext(ctx, &fixtures, `
		SELECT id, home_team_id, away_team_id, division_id, season_id, week_id, scheduled_date, 
		       venue_location, status, completed_date, day_captain_id, notes, created_at, updated_at
		FROM fixtures 
		WHERE scheduled_date < CURRENT_TIMESTAMP AND status = ?
		ORDER BY scheduled_date ASC
	`, string(models.Scheduled))
	return fixtures, err
}

// CountByStatus returns the number of fixtures with a specific status
func (r *fixtureRepository) CountByStatus(ctx context.Context, status models.FixtureStatus) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM fixtures WHERE status = ?
	`, string(status))
	return count, err
}

// CountByDivision returns the number of fixtures in a specific division
func (r *fixtureRepository) CountByDivision(ctx context.Context, divisionID uint) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM fixtures WHERE division_id = ?
	`, divisionID)
	return count, err
}

// CountBySeason returns the number of fixtures in a specific season
func (r *fixtureRepository) CountBySeason(ctx context.Context, seasonID uint) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM fixtures WHERE season_id = ?
	`, seasonID)
	return count, err
}

// FindByWeek retrieves fixtures for a specific week
func (r *fixtureRepository) FindByWeek(ctx context.Context, weekID uint) ([]models.Fixture, error) {
	var fixtures []models.Fixture
	err := r.db.SelectContext(ctx, &fixtures, `
		SELECT id, home_team_id, away_team_id, division_id, season_id, week_id, scheduled_date, 
		       venue_location, status, completed_date, day_captain_id, notes, created_at, updated_at
		FROM fixtures 
		WHERE week_id = ?
		ORDER BY scheduled_date ASC
	`, weekID)
	return fixtures, err
}

// FindByWeekNumber retrieves fixtures for a specific week number in a season
func (r *fixtureRepository) FindByWeekNumber(ctx context.Context, seasonID uint, weekNumber int) ([]models.Fixture, error) {
	var fixtures []models.Fixture
	err := r.db.SelectContext(ctx, &fixtures, `
		SELECT f.id, f.home_team_id, f.away_team_id, f.division_id, f.season_id, f.week_id, f.scheduled_date, 
		       f.venue_location, f.status, f.completed_date, f.day_captain_id, f.notes, f.created_at, f.updated_at
		FROM fixtures f
		INNER JOIN weeks w ON f.week_id = w.id
		WHERE f.season_id = ? AND w.week_number = ?
		ORDER BY f.scheduled_date ASC
	`, seasonID, weekNumber)
	return fixtures, err
}

// FindByWeekAndDivision retrieves fixtures for a specific week and division
func (r *fixtureRepository) FindByWeekAndDivision(ctx context.Context, weekID, divisionID uint) ([]models.Fixture, error) {
	var fixtures []models.Fixture
	err := r.db.SelectContext(ctx, &fixtures, `
		SELECT id, home_team_id, away_team_id, division_id, season_id, week_id, scheduled_date, 
		       venue_location, status, completed_date, day_captain_id, notes, created_at, updated_at
		FROM fixtures 
		WHERE week_id = ? AND division_id = ?
		ORDER BY scheduled_date ASC
	`, weekID, divisionID)
	return fixtures, err
}

// CountByWeek returns the number of fixtures in a specific week
func (r *fixtureRepository) CountByWeek(ctx context.Context, weekID uint) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM fixtures WHERE week_id = ?
	`, weekID)
	return count, err
}

// FindSelectedPlayers retrieves all players selected for a specific fixture
func (r *fixtureRepository) FindSelectedPlayers(ctx context.Context, fixtureID uint) ([]models.FixturePlayer, error) {
	var players []models.FixturePlayer
	err := r.db.SelectContext(ctx, &players, `
		SELECT id, fixture_id, player_id, is_home, position, managing_team_id, created_at, updated_at
		FROM fixture_players 
		WHERE fixture_id = ?
		ORDER BY position ASC, created_at ASC
	`, fixtureID)
	return players, err
}

// AddSelectedPlayer adds a player to the fixture selection
func (r *fixtureRepository) AddSelectedPlayer(ctx context.Context, fixturePlayer *models.FixturePlayer) error {
	now := time.Now()
	fixturePlayer.CreatedAt = now
	fixturePlayer.UpdatedAt = now

	result, err := r.db.NamedExecContext(ctx, `
		INSERT INTO fixture_players (fixture_id, player_id, is_home, position, managing_team_id, created_at, updated_at)
		VALUES (:fixture_id, :player_id, :is_home, :position, :managing_team_id, :created_at, :updated_at)
	`, fixturePlayer)

	if err != nil {
		return err
	}

	// Get the last inserted ID and set it on the fixture player
	if id, err := result.LastInsertId(); err == nil {
		fixturePlayer.ID = uint(id)
	}

	return nil
}

// RemoveSelectedPlayer removes a player from the fixture selection
func (r *fixtureRepository) RemoveSelectedPlayer(ctx context.Context, fixtureID uint, playerID string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM fixture_players 
		WHERE fixture_id = ? AND player_id = ?
	`, fixtureID, playerID)
	return err
}

// UpdateSelectedPlayerPosition updates the position of a selected player
func (r *fixtureRepository) UpdateSelectedPlayerPosition(ctx context.Context, fixtureID uint, playerID string, position int) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE fixture_players 
		SET position = ?, updated_at = CURRENT_TIMESTAMP
		WHERE fixture_id = ? AND player_id = ?
	`, position, fixtureID, playerID)
	return err
}

// FindSelectedPlayersByTeam retrieves all players selected for a specific fixture by managing team
func (r *fixtureRepository) FindSelectedPlayersByTeam(ctx context.Context, fixtureID, managingTeamID uint) ([]models.FixturePlayer, error) {
	var players []models.FixturePlayer
	err := r.db.SelectContext(ctx, &players, `
		SELECT id, fixture_id, player_id, is_home, position, managing_team_id, created_at, updated_at
		FROM fixture_players 
		WHERE fixture_id = ? AND managing_team_id = ?
		ORDER BY position ASC, created_at ASC
	`, fixtureID, managingTeamID)
	return players, err
}

// RemoveSelectedPlayerByTeam removes a player from the fixture selection for a specific team
func (r *fixtureRepository) RemoveSelectedPlayerByTeam(ctx context.Context, fixtureID, managingTeamID uint, playerID string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM fixture_players 
		WHERE fixture_id = ? AND managing_team_id = ? AND player_id = ?
	`, fixtureID, managingTeamID, playerID)
	return err
}

// ClearSelectedPlayers removes all selected players from a fixture
func (r *fixtureRepository) ClearSelectedPlayers(ctx context.Context, fixtureID uint) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM fixture_players WHERE fixture_id = ?
	`, fixtureID)
	return err
}

// ClearSelectedPlayersByTeam removes all selected players from a fixture for a specific team
func (r *fixtureRepository) ClearSelectedPlayersByTeam(ctx context.Context, fixtureID, managingTeamID uint) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM fixture_players WHERE fixture_id = ? AND managing_team_id = ?
	`, fixtureID, managingTeamID)
	return err
}

// FindUpcomingFixturesForPlayer retrieves upcoming fixtures where a specific player has been selected
func (r *fixtureRepository) FindUpcomingFixturesForPlayer(ctx context.Context, playerID string) ([]models.Fixture, error) {
	var fixtures []models.Fixture

	// Execute the main query

	err := r.db.SelectContext(ctx, &fixtures, `
		SELECT DISTINCT f.id, f.home_team_id, f.away_team_id, f.division_id, f.season_id, f.week_id, 
		       f.scheduled_date, f.venue_location, f.status, f.completed_date, f.day_captain_id, 
		       f.external_match_card_id, f.notes, f.created_at, f.updated_at
		FROM fixtures f
		INNER JOIN fixture_players fp ON f.id = fp.fixture_id
		WHERE fp.player_id = ? 
		  AND date(f.scheduled_date) >= date('now')
		  AND f.status IN ('Scheduled', 'InProgress')
				ORDER BY f.scheduled_date ASC
	`, playerID)

	return fixtures, err
}
