package repository

import (
	"context"
	"fmt"
	"time"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
)

// FixtureRepository handles database operations for Fixture entities
type FixtureRepository struct {
	db *database.DB
}

// NewFixtureRepository creates a new FixtureRepository
func NewFixtureRepository(db *database.DB) *FixtureRepository {
	return &FixtureRepository{db: db}
}

// Create inserts a new fixture into the database
func (r *FixtureRepository) Create(ctx context.Context, fixture *models.Fixture) error {
	query := `
		INSERT INTO fixtures (
			home_team_id, away_team_id, division_id, season_id, scheduled_date,
			venue_location, status, completed_date, day_captain_id, notes,
			created_at, updated_at
		)
		VALUES (
			:home_team_id, :away_team_id, :division_id, :season_id, :scheduled_date,
			:venue_location, :status, :completed_date, :day_captain_id, :notes,
			:created_at, :updated_at
		)
		RETURNING id
	`

	now := time.Now()
	fixture.CreatedAt = now
	fixture.UpdatedAt = now

	stmt, err := r.db.PrepareNamedContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare query: %w", err)
	}
	defer stmt.Close()

	var id uint
	err = stmt.GetContext(ctx, &id, fixture)
	if err != nil {
		return fmt.Errorf("failed to insert fixture: %w", err)
	}

	fixture.ID = id
	return nil
}

// GetByID retrieves a fixture by ID
func (r *FixtureRepository) GetByID(ctx context.Context, id uint) (*models.Fixture, error) {
	var fixture models.Fixture
	query := `SELECT * FROM fixtures WHERE id = $1`
	
	err := r.db.GetContext(ctx, &fixture, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get fixture by id: %w", err)
	}
	
	return &fixture, nil
}

// Update updates an existing fixture
func (r *FixtureRepository) Update(ctx context.Context, fixture *models.Fixture) error {
	query := `
		UPDATE fixtures
		SET home_team_id = :home_team_id,
			away_team_id = :away_team_id,
			division_id = :division_id,
			season_id = :season_id,
			scheduled_date = :scheduled_date,
			venue_location = :venue_location,
			status = :status,
			completed_date = :completed_date,
			day_captain_id = :day_captain_id,
			notes = :notes,
			updated_at = :updated_at
		WHERE id = :id
	`

	fixture.UpdatedAt = time.Now()

	_, err := r.db.NamedExecContext(ctx, query, fixture)
	if err != nil {
		return fmt.Errorf("failed to update fixture: %w", err)
	}

	return nil
}

// Delete removes a fixture from the database
func (r *FixtureRepository) Delete(ctx context.Context, id uint) error {
	// Check if fixture has matchups
	var matchupCount int
	err := r.db.GetContext(ctx, &matchupCount, `SELECT COUNT(*) FROM matchups WHERE fixture_id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to check fixture matchups: %w", err)
	}
	
	if matchupCount > 0 {
		return fmt.Errorf("cannot delete fixture with %d matchups - please remove them first", matchupCount)
	}
	
	// Delete the fixture
	_, err = r.db.ExecContext(ctx, `DELETE FROM fixtures WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete fixture: %w", err)
	}
	
	return nil
}

// List retrieves all fixtures
func (r *FixtureRepository) List(ctx context.Context) ([]models.Fixture, error) {
	var fixtures []models.Fixture
	query := `SELECT * FROM fixtures ORDER BY scheduled_date`
	
	err := r.db.SelectContext(ctx, &fixtures, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list fixtures: %w", err)
	}
	
	return fixtures, nil
}

// GetByDivision retrieves fixtures by division ID
func (r *FixtureRepository) GetByDivision(ctx context.Context, divisionID uint) ([]models.Fixture, error) {
	var fixtures []models.Fixture
	query := `SELECT * FROM fixtures WHERE division_id = $1 ORDER BY scheduled_date`
	
	err := r.db.SelectContext(ctx, &fixtures, query, divisionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get fixtures by division: %w", err)
	}
	
	return fixtures, nil
}

// GetByTeam retrieves fixtures by team ID (either home or away)
func (r *FixtureRepository) GetByTeam(ctx context.Context, teamID uint) ([]models.Fixture, error) {
	var fixtures []models.Fixture
	query := `
		SELECT * 
		FROM fixtures 
		WHERE home_team_id = $1 OR away_team_id = $1 
		ORDER BY scheduled_date
	`
	
	err := r.db.SelectContext(ctx, &fixtures, query, teamID)
	if err != nil {
		return nil, fmt.Errorf("failed to get fixtures by team: %w", err)
	}
	
	return fixtures, nil
}

// GetByStatus retrieves fixtures by status
func (r *FixtureRepository) GetByStatus(ctx context.Context, status models.FixtureStatus) ([]models.Fixture, error) {
	var fixtures []models.Fixture
	query := `SELECT * FROM fixtures WHERE status = $1 ORDER BY scheduled_date`
	
	err := r.db.SelectContext(ctx, &fixtures, query, status)
	if err != nil {
		return nil, fmt.Errorf("failed to get fixtures by status: %w", err)
	}
	
	return fixtures, nil
}

// GetUpcoming retrieves upcoming fixtures
func (r *FixtureRepository) GetUpcoming(ctx context.Context, days int) ([]models.Fixture, error) {
	var fixtures []models.Fixture
	query := `
		SELECT * 
		FROM fixtures 
		WHERE scheduled_date BETWEEN NOW() AND NOW() + INTERVAL '$1 days'
		ORDER BY scheduled_date
	`
	
	err := r.db.SelectContext(ctx, &fixtures, query, days)
	if err != nil {
		return nil, fmt.Errorf("failed to get upcoming fixtures: %w", err)
	}
	
	return fixtures, nil
}

// GetBySeason retrieves fixtures by season ID
func (r *FixtureRepository) GetBySeason(ctx context.Context, seasonID uint) ([]models.Fixture, error) {
	var fixtures []models.Fixture
	query := `SELECT * FROM fixtures WHERE season_id = $1 ORDER BY scheduled_date`
	
	err := r.db.SelectContext(ctx, &fixtures, query, seasonID)
	if err != nil {
		return nil, fmt.Errorf("failed to get fixtures by season: %w", err)
	}
	
	return fixtures, nil
}

// GetWithMatchups retrieves a fixture with its matchups
func (r *FixtureRepository) GetWithMatchups(ctx context.Context, id uint) (*models.Fixture, error) {
	fixture, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	
	// Get matchups for this fixture
	matchupsQuery := `
		SELECT * 
		FROM matchups 
		WHERE fixture_id = $1
		ORDER BY id
	`
	
	var matchups []models.Matchup
	err = r.db.SelectContext(ctx, &matchups, matchupsQuery, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get fixture matchups: %w", err)
	}
	
	fixture.Matchups = matchups
	return fixture, nil
}

// UpdateStatus updates the status of a fixture
func (r *FixtureRepository) UpdateStatus(ctx context.Context, id uint, status models.FixtureStatus) error {
	query := `
		UPDATE fixtures
		SET status = $1, updated_at = $2
		WHERE id = $3
	`
	
	_, err := r.db.ExecContext(ctx, query, status, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update fixture status: %w", err)
	}
	
	// If status is Completed, set the completed date
	if status == models.Completed {
		completedDate := time.Now()
		_, err = r.db.ExecContext(
			ctx,
			`UPDATE fixtures SET completed_date = $1 WHERE id = $2`,
			completedDate, id,
		)
		if err != nil {
			return fmt.Errorf("failed to set fixture completed date: %w", err)
		}
	}
	
	return nil
}

// CreateMatchups creates the standard set of matchups for a fixture
func (r *FixtureRepository) CreateMatchups(ctx context.Context, fixtureID uint) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()
	
	now := time.Now()
	
	// Create the four standard matchups
	matchups := []struct {
		Type models.MatchupType
	}{
		{Type: models.Mens},
		{Type: models.Womens},
		{Type: models.FirstMixed},
		{Type: models.SecondMixed},
	}
	
	for _, m := range matchups {
		_, err = tx.ExecContext(
			ctx,
			`INSERT INTO matchups 
			(fixture_id, type, status, home_score, away_score, notes, created_at, updated_at)
			VALUES ($1, $2, $3, 0, 0, '', $4, $4)`,
			fixtureID, m.Type, models.Pending, now,
		)
		
		if err != nil {
			return fmt.Errorf("failed to create matchup: %w", err)
		}
	}
	
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	
	return nil
}

// AssignDayCaptain assigns a day captain to a fixture
func (r *FixtureRepository) AssignDayCaptain(ctx context.Context, fixtureID uint, playerID string) error {
	query := `
		UPDATE fixtures
		SET day_captain_id = $1, updated_at = $2
		WHERE id = $3
	`
	
	_, err := r.db.ExecContext(ctx, query, playerID, time.Now(), fixtureID)
	if err != nil {
		return fmt.Errorf("failed to assign day captain: %w", err)
	}
	
	return nil
} 