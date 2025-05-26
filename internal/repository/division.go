package repository

import (
	"context"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
	"time"
)

// DivisionRepository defines the interface for division data access
type DivisionRepository interface {
	// Basic CRUD operations
	FindAll(ctx context.Context) ([]models.Division, error)
	FindByID(ctx context.Context, id uint) (*models.Division, error)
	Create(ctx context.Context, division *models.Division) error
	Update(ctx context.Context, division *models.Division) error
	Delete(ctx context.Context, id uint) error

	// Division-specific queries
	FindByLeague(ctx context.Context, leagueID uint) ([]models.Division, error)
	FindBySeason(ctx context.Context, seasonID uint) ([]models.Division, error)
	FindByLeagueAndSeason(ctx context.Context, leagueID, seasonID uint) ([]models.Division, error)
	FindByLevel(ctx context.Context, level int) ([]models.Division, error)
	FindByPlayDay(ctx context.Context, playDay string) ([]models.Division, error)
	FindWithTeams(ctx context.Context, id uint) (*models.Division, error)
	FindWithFixtures(ctx context.Context, id uint) (*models.Division, error)
	FindWithTeamsAndFixtures(ctx context.Context, id uint) (*models.Division, error)

	// Team management queries
	CountTeams(ctx context.Context, id uint) (int, error)
	CountTeamsByClub(ctx context.Context, divisionID, clubID uint) (int, error)
	CanAddTeamFromClub(ctx context.Context, divisionID, clubID uint) (bool, error)
	FindTeamsInDivision(ctx context.Context, id uint) ([]models.Team, error)
}

// divisionRepository implements DivisionRepository
type divisionRepository struct {
	db *database.DB
}

// NewDivisionRepository creates a new division repository
func NewDivisionRepository(db *database.DB) DivisionRepository {
	return &divisionRepository{
		db: db,
	}
}

// FindAll retrieves all divisions ordered by league, season, level, and name
func (r *divisionRepository) FindAll(ctx context.Context) ([]models.Division, error) {
	var divisions []models.Division
	err := r.db.SelectContext(ctx, &divisions, `
		SELECT id, name, level, play_day, league_id, season_id, max_teams_per_club, created_at, updated_at
		FROM divisions 
		ORDER BY league_id ASC, season_id DESC, level ASC, name ASC
	`)
	return divisions, err
}

// FindByID retrieves a division by its ID
func (r *divisionRepository) FindByID(ctx context.Context, id uint) (*models.Division, error) {
	var division models.Division
	err := r.db.GetContext(ctx, &division, `
		SELECT id, name, level, play_day, league_id, season_id, max_teams_per_club, created_at, updated_at
		FROM divisions 
		WHERE id = ?
	`, id)
	if err != nil {
		return nil, err
	}
	return &division, nil
}

// Create inserts a new division record
func (r *divisionRepository) Create(ctx context.Context, division *models.Division) error {
	now := time.Now()
	division.CreatedAt = now
	division.UpdatedAt = now

	result, err := r.db.NamedExecContext(ctx, `
		INSERT INTO divisions (name, level, play_day, league_id, season_id, max_teams_per_club, created_at, updated_at)
		VALUES (:name, :level, :play_day, :league_id, :season_id, :max_teams_per_club, :created_at, :updated_at)
	`, division)

	if err != nil {
		return err
	}

	// Get the last inserted ID and set it on the division
	if id, err := result.LastInsertId(); err == nil {
		division.ID = uint(id)
	}

	return nil
}

// Update modifies an existing division record
func (r *divisionRepository) Update(ctx context.Context, division *models.Division) error {
	division.UpdatedAt = time.Now()

	_, err := r.db.NamedExecContext(ctx, `
		UPDATE divisions 
		SET name = :name, level = :level, play_day = :play_day, league_id = :league_id, 
		    season_id = :season_id, max_teams_per_club = :max_teams_per_club, updated_at = :updated_at
		WHERE id = :id
	`, division)

	return err
}

// Delete removes a division record by ID
func (r *divisionRepository) Delete(ctx context.Context, id uint) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM divisions WHERE id = ?`, id)
	return err
}

// FindByLeague retrieves all divisions for a specific league
func (r *divisionRepository) FindByLeague(ctx context.Context, leagueID uint) ([]models.Division, error) {
	var divisions []models.Division
	err := r.db.SelectContext(ctx, &divisions, `
		SELECT id, name, level, play_day, league_id, season_id, max_teams_per_club, created_at, updated_at
		FROM divisions 
		WHERE league_id = ?
		ORDER BY season_id DESC, level ASC, name ASC
	`, leagueID)
	return divisions, err
}

// FindBySeason retrieves all divisions for a specific season
func (r *divisionRepository) FindBySeason(ctx context.Context, seasonID uint) ([]models.Division, error) {
	var divisions []models.Division
	err := r.db.SelectContext(ctx, &divisions, `
		SELECT id, name, level, play_day, league_id, season_id, max_teams_per_club, created_at, updated_at
		FROM divisions 
		WHERE season_id = ?
		ORDER BY league_id ASC, level ASC, name ASC
	`, seasonID)
	return divisions, err
}

// FindByLeagueAndSeason retrieves divisions for a specific league and season
func (r *divisionRepository) FindByLeagueAndSeason(ctx context.Context, leagueID, seasonID uint) ([]models.Division, error) {
	var divisions []models.Division
	err := r.db.SelectContext(ctx, &divisions, `
		SELECT id, name, level, play_day, league_id, season_id, max_teams_per_club, created_at, updated_at
		FROM divisions 
		WHERE league_id = ? AND season_id = ?
		ORDER BY level ASC, name ASC
	`, leagueID, seasonID)
	return divisions, err
}

// FindByLevel retrieves all divisions at a specific level
func (r *divisionRepository) FindByLevel(ctx context.Context, level int) ([]models.Division, error) {
	var divisions []models.Division
	err := r.db.SelectContext(ctx, &divisions, `
		SELECT id, name, level, play_day, league_id, season_id, max_teams_per_club, created_at, updated_at
		FROM divisions 
		WHERE level = ?
		ORDER BY league_id ASC, season_id DESC, name ASC
	`, level)
	return divisions, err
}

// FindByPlayDay retrieves all divisions that play on a specific day
func (r *divisionRepository) FindByPlayDay(ctx context.Context, playDay string) ([]models.Division, error) {
	var divisions []models.Division
	err := r.db.SelectContext(ctx, &divisions, `
		SELECT id, name, level, play_day, league_id, season_id, max_teams_per_club, created_at, updated_at
		FROM divisions 
		WHERE play_day = ?
		ORDER BY league_id ASC, season_id DESC, level ASC, name ASC
	`, playDay)
	return divisions, err
}

// FindWithTeams retrieves a division with its associated teams
func (r *divisionRepository) FindWithTeams(ctx context.Context, id uint) (*models.Division, error) {
	// First get the division
	division, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Then get associated teams
	var teams []models.Team
	err = r.db.SelectContext(ctx, &teams, `
		SELECT id, name, club_id, division_id, season_id, created_at, updated_at
		FROM teams
		WHERE division_id = ?
		ORDER BY name ASC
	`, id)

	if err != nil {
		return nil, err
	}

	division.Teams = teams
	return division, nil
}

// FindWithFixtures retrieves a division with its associated fixtures
func (r *divisionRepository) FindWithFixtures(ctx context.Context, id uint) (*models.Division, error) {
	// First get the division
	division, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Then get associated fixtures
	var fixtures []models.Fixture
	err = r.db.SelectContext(ctx, &fixtures, `
		SELECT id, home_team_id, away_team_id, division_id, season_id, scheduled_date, 
		       venue_location, status, completed_date, day_captain_id, notes, created_at, updated_at
		FROM fixtures
		WHERE division_id = ?
		ORDER BY scheduled_date ASC
	`, id)

	if err != nil {
		return nil, err
	}

	division.Fixtures = fixtures
	return division, nil
}

// FindWithTeamsAndFixtures retrieves a division with both its teams and fixtures
func (r *divisionRepository) FindWithTeamsAndFixtures(ctx context.Context, id uint) (*models.Division, error) {
	// First get the division
	division, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Get associated teams
	var teams []models.Team
	err = r.db.SelectContext(ctx, &teams, `
		SELECT id, name, club_id, division_id, season_id, created_at, updated_at
		FROM teams
		WHERE division_id = ?
		ORDER BY name ASC
	`, id)

	if err != nil {
		return nil, err
	}

	// Get associated fixtures
	var fixtures []models.Fixture
	err = r.db.SelectContext(ctx, &fixtures, `
		SELECT id, home_team_id, away_team_id, division_id, season_id, scheduled_date, 
		       venue_location, status, completed_date, day_captain_id, notes, created_at, updated_at
		FROM fixtures
		WHERE division_id = ?
		ORDER BY scheduled_date ASC
	`, id)

	if err != nil {
		return nil, err
	}

	division.Teams = teams
	division.Fixtures = fixtures
	return division, nil
}

// CountTeams returns the total number of teams in a division
func (r *divisionRepository) CountTeams(ctx context.Context, id uint) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM teams WHERE division_id = ?
	`, id)
	return count, err
}

// CountTeamsByClub returns the number of teams from a specific club in a division
func (r *divisionRepository) CountTeamsByClub(ctx context.Context, divisionID, clubID uint) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM teams WHERE division_id = ? AND club_id = ?
	`, divisionID, clubID)
	return count, err
}

// CanAddTeamFromClub checks if a club can add another team to the division (respects max_teams_per_club)
func (r *divisionRepository) CanAddTeamFromClub(ctx context.Context, divisionID, clubID uint) (bool, error) {
	// Get the division to check max_teams_per_club
	division, err := r.FindByID(ctx, divisionID)
	if err != nil {
		return false, err
	}

	// Count current teams from this club
	currentCount, err := r.CountTeamsByClub(ctx, divisionID, clubID)
	if err != nil {
		return false, err
	}

	return currentCount < division.MaxTeamsPerClub, nil
}

// FindTeamsInDivision retrieves all teams in a division (alias for convenience)
func (r *divisionRepository) FindTeamsInDivision(ctx context.Context, id uint) ([]models.Team, error) {
	var teams []models.Team
	err := r.db.SelectContext(ctx, &teams, `
		SELECT id, name, club_id, division_id, season_id, created_at, updated_at
		FROM teams
		WHERE division_id = ?
		ORDER BY name ASC
	`, id)
	return teams, err
}
