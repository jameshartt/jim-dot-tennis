package repository

import (
	"context"
	"fmt"
	"time"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
)

// DivisionRepository handles database operations for Division entities
type DivisionRepository struct {
	db *database.DB
}

// NewDivisionRepository creates a new DivisionRepository
func NewDivisionRepository(db *database.DB) *DivisionRepository {
	return &DivisionRepository{db: db}
}

// Create inserts a new division into the database
func (r *DivisionRepository) Create(ctx context.Context, division *models.Division) error {
	query := `
		INSERT INTO divisions (
			name, level, play_day, league_id, season_id, max_teams_per_club, 
			created_at, updated_at
		)
		VALUES (
			:name, :level, :play_day, :league_id, :season_id, :max_teams_per_club, 
			:created_at, :updated_at
		)
		RETURNING id
	`

	now := time.Now()
	division.CreatedAt = now
	division.UpdatedAt = now

	stmt, err := r.db.PrepareNamedContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare query: %w", err)
	}
	defer stmt.Close()

	var id uint
	err = stmt.GetContext(ctx, &id, division)
	if err != nil {
		return fmt.Errorf("failed to insert division: %w", err)
	}

	division.ID = id
	return nil
}

// GetByID retrieves a division by ID
func (r *DivisionRepository) GetByID(ctx context.Context, id uint) (*models.Division, error) {
	var division models.Division
	query := `SELECT * FROM divisions WHERE id = $1`
	
	err := r.db.GetContext(ctx, &division, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get division by id: %w", err)
	}
	
	return &division, nil
}

// Update updates an existing division
func (r *DivisionRepository) Update(ctx context.Context, division *models.Division) error {
	query := `
		UPDATE divisions
		SET name = :name,
			level = :level,
			play_day = :play_day,
			league_id = :league_id,
			season_id = :season_id,
			max_teams_per_club = :max_teams_per_club,
			updated_at = :updated_at
		WHERE id = :id
	`

	division.UpdatedAt = time.Now()

	_, err := r.db.NamedExecContext(ctx, query, division)
	if err != nil {
		return fmt.Errorf("failed to update division: %w", err)
	}

	return nil
}

// Delete removes a division from the database
func (r *DivisionRepository) Delete(ctx context.Context, id uint) error {
	// Check if division has teams
	var teamCount int
	err := r.db.GetContext(ctx, &teamCount, `SELECT COUNT(*) FROM teams WHERE division_id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to check division teams: %w", err)
	}
	
	if teamCount > 0 {
		return fmt.Errorf("cannot delete division with %d teams - please remove or transfer them first", teamCount)
	}
	
	// Check if division has fixtures
	var fixtureCount int
	err = r.db.GetContext(ctx, &fixtureCount, `SELECT COUNT(*) FROM fixtures WHERE division_id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to check division fixtures: %w", err)
	}
	
	if fixtureCount > 0 {
		return fmt.Errorf("cannot delete division with %d fixtures - please remove them first", fixtureCount)
	}
	
	// Delete the division
	_, err = r.db.ExecContext(ctx, `DELETE FROM divisions WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete division: %w", err)
	}
	
	return nil
}

// List retrieves all divisions
func (r *DivisionRepository) List(ctx context.Context) ([]models.Division, error) {
	var divisions []models.Division
	query := `SELECT * FROM divisions ORDER BY level, name`
	
	err := r.db.SelectContext(ctx, &divisions, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list divisions: %w", err)
	}
	
	return divisions, nil
}

// GetByLeagueAndSeason retrieves divisions by league ID and season ID
func (r *DivisionRepository) GetByLeagueAndSeason(ctx context.Context, leagueID, seasonID uint) ([]models.Division, error) {
	var divisions []models.Division
	query := `
		SELECT * 
		FROM divisions 
		WHERE league_id = $1 AND season_id = $2 
		ORDER BY level, name
	`
	
	err := r.db.SelectContext(ctx, &divisions, query, leagueID, seasonID)
	if err != nil {
		return nil, fmt.Errorf("failed to get divisions by league and season: %w", err)
	}
	
	return divisions, nil
}

// ListByLeagueAndSeason retrieves all divisions for a given league and season
func (r *DivisionRepository) ListByLeagueAndSeason(ctx context.Context, leagueID, seasonID uint) ([]models.Division, error) {
	var divisions []models.Division
	query := `
		SELECT * 
		FROM divisions 
		WHERE league_id = $1 AND season_id = $2 
		ORDER BY level, name
	`
	
	err := r.db.SelectContext(ctx, &divisions, query, leagueID, seasonID)
	if err != nil {
		return nil, fmt.Errorf("failed to list divisions by league and season: %w", err)
	}
	
	return divisions, nil
}

// GetWithTeams retrieves a division with its teams
func (r *DivisionRepository) GetWithTeams(ctx context.Context, id uint) (*models.Division, error) {
	division, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	
	// Get teams for this division
	teamsQuery := `
		SELECT t.* 
		FROM teams t
		WHERE t.division_id = $1
		ORDER BY t.name
	`
	
	var teams []models.Team
	err = r.db.SelectContext(ctx, &teams, teamsQuery, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get division teams: %w", err)
	}
	
	division.Teams = teams
	return division, nil
}

// GetWithFixtures retrieves a division with its fixtures
func (r *DivisionRepository) GetWithFixtures(ctx context.Context, id uint) (*models.Division, error) {
	division, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	
	// Get fixtures for this division
	fixturesQuery := `
		SELECT f.* 
		FROM fixtures f
		WHERE f.division_id = $1
		ORDER BY f.scheduled_date
	`
	
	var fixtures []models.Fixture
	err = r.db.SelectContext(ctx, &fixtures, fixturesQuery, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get division fixtures: %w", err)
	}
	
	division.Fixtures = fixtures
	return division, nil
}

// CountTeamsByClub counts the number of teams from a specific club in this division
func (r *DivisionRepository) CountTeamsByClub(ctx context.Context, divisionID, clubID uint) (int, error) {
	var count int
	query := `
		SELECT COUNT(*) 
		FROM teams 
		WHERE division_id = $1 AND club_id = $2
	`
	
	err := r.db.GetContext(ctx, &count, query, divisionID, clubID)
	if err != nil {
		return 0, fmt.Errorf("failed to count teams by club: %w", err)
	}
	
	return count, nil
} 