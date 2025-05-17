package repository

import (
	"context"
	"fmt"
	"time"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
)

// SeasonRepository handles database operations for Season entities
type SeasonRepository struct {
	db *database.DB
}

// NewSeasonRepository creates a new SeasonRepository
func NewSeasonRepository(db *database.DB) *SeasonRepository {
	return &SeasonRepository{db: db}
}

// Create inserts a new season into the database
func (r *SeasonRepository) Create(ctx context.Context, season *models.Season) error {
	query := `
		INSERT INTO seasons (name, year, start_date, end_date, is_active, created_at, updated_at)
		VALUES (:name, :year, :start_date, :end_date, :is_active, :created_at, :updated_at)
		RETURNING id
	`

	now := time.Now()
	season.CreatedAt = now
	season.UpdatedAt = now

	stmt, err := r.db.PrepareNamedContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare query: %w", err)
	}
	defer stmt.Close()

	var id uint
	err = stmt.GetContext(ctx, &id, season)
	if err != nil {
		return fmt.Errorf("failed to insert season: %w", err)
	}

	season.ID = id
	return nil
}

// GetByID retrieves a season by ID
func (r *SeasonRepository) GetByID(ctx context.Context, id uint) (*models.Season, error) {
	var season models.Season
	query := `SELECT * FROM seasons WHERE id = $1`
	
	err := r.db.GetContext(ctx, &season, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get season by id: %w", err)
	}
	
	return &season, nil
}

// GetActive retrieves the currently active season
func (r *SeasonRepository) GetActive(ctx context.Context) (*models.Season, error) {
	var season models.Season
	query := `SELECT * FROM seasons WHERE is_active = true ORDER BY start_date DESC LIMIT 1`
	
	err := r.db.GetContext(ctx, &season, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get active season: %w", err)
	}
	
	return &season, nil
}

// Update updates an existing season
func (r *SeasonRepository) Update(ctx context.Context, season *models.Season) error {
	query := `
		UPDATE seasons
		SET name = :name,
			year = :year,
			start_date = :start_date,
			end_date = :end_date,
			is_active = :is_active,
			updated_at = :updated_at
		WHERE id = :id
	`

	season.UpdatedAt = time.Now()

	_, err := r.db.NamedExecContext(ctx, query, season)
	if err != nil {
		return fmt.Errorf("failed to update season: %w", err)
	}

	return nil
}

// SetActive sets a season as active and marks all others as inactive
func (r *SeasonRepository) SetActive(ctx context.Context, id uint) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()
	
	// Set all seasons to inactive
	_, err = tx.ExecContext(ctx, `UPDATE seasons SET is_active = false, updated_at = $1 WHERE is_active = true`, time.Now())
	if err != nil {
		return fmt.Errorf("failed to deactivate seasons: %w", err)
	}
	
	// Set the specified season to active
	_, err = tx.ExecContext(ctx, `UPDATE seasons SET is_active = true, updated_at = $1 WHERE id = $2`, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to activate season: %w", err)
	}
	
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	
	return nil
}

// Delete removes a season from the database
func (r *SeasonRepository) Delete(ctx context.Context, id uint) error {
	// Check if season has leagues associated with it
	var leagueCount int
	err := r.db.GetContext(ctx, &leagueCount, `SELECT COUNT(*) FROM league_seasons WHERE season_id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to check season leagues: %w", err)
	}
	
	if leagueCount > 0 {
		return fmt.Errorf("cannot delete season with %d league associations - please remove those first", leagueCount)
	}
	
	// Check if season has teams
	var teamCount int
	err = r.db.GetContext(ctx, &teamCount, `SELECT COUNT(*) FROM teams WHERE season_id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to check season teams: %w", err)
	}
	
	if teamCount > 0 {
		return fmt.Errorf("cannot delete season with %d teams - please remove them first", teamCount)
	}
	
	// Delete the season
	_, err = r.db.ExecContext(ctx, `DELETE FROM seasons WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete season: %w", err)
	}
	
	return nil
}

// List retrieves all seasons
func (r *SeasonRepository) List(ctx context.Context) ([]models.Season, error) {
	var seasons []models.Season
	query := `SELECT * FROM seasons ORDER BY year DESC, start_date DESC`
	
	err := r.db.SelectContext(ctx, &seasons, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list seasons: %w", err)
	}
	
	return seasons, nil
}

// GetWithLeagues retrieves a season with its associated leagues
func (r *SeasonRepository) GetWithLeagues(ctx context.Context, id uint) (*models.Season, error) {
	season, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	
	// Get leagues for this season through the many-to-many relationship
	leaguesQuery := `
		SELECT l.* 
		FROM leagues l
		JOIN league_seasons ls ON l.id = ls.league_id
		WHERE ls.season_id = $1
		ORDER BY l.name
	`
	
	var leagues []models.League
	err = r.db.SelectContext(ctx, &leagues, leaguesQuery, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get season leagues: %w", err)
	}
	
	season.Leagues = leagues
	return season, nil
} 