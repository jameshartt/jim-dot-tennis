package repository

import (
	"context"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
	"time"
)

// SeasonRepository defines the interface for season data access
type SeasonRepository interface {
	// Basic CRUD operations
	FindAll(ctx context.Context) ([]models.Season, error)
	FindByID(ctx context.Context, id uint) (*models.Season, error)
	Create(ctx context.Context, season *models.Season) error
	Update(ctx context.Context, season *models.Season) error
	Delete(ctx context.Context, id uint) error

	// Season-specific queries
	FindActive(ctx context.Context) (*models.Season, error)
	FindByYear(ctx context.Context, year int) ([]models.Season, error)
	SetActive(ctx context.Context, id uint) error
	FindWithLeagues(ctx context.Context, id uint) (*models.Season, error)
}

// seasonRepository implements SeasonRepository
type seasonRepository struct {
	db *database.DB
}

// NewSeasonRepository creates a new season repository
func NewSeasonRepository(db *database.DB) SeasonRepository {
	return &seasonRepository{
		db: db,
	}
}

// FindAll retrieves all seasons ordered by year descending
func (r *seasonRepository) FindAll(ctx context.Context) ([]models.Season, error) {
	var seasons []models.Season
	err := r.db.SelectContext(ctx, &seasons, `
		SELECT id, name, year, start_date, end_date, is_active, created_at, updated_at
		FROM seasons 
		ORDER BY year DESC, start_date DESC
	`)
	return seasons, err
}

// FindByID retrieves a season by its ID
func (r *seasonRepository) FindByID(ctx context.Context, id uint) (*models.Season, error) {
	var season models.Season
	err := r.db.GetContext(ctx, &season, `
		SELECT id, name, year, start_date, end_date, is_active, created_at, updated_at
		FROM seasons 
		WHERE id = ?
	`, id)
	if err != nil {
		return nil, err
	}
	return &season, nil
}

// Create inserts a new season record
func (r *seasonRepository) Create(ctx context.Context, season *models.Season) error {
	now := time.Now()
	season.CreatedAt = now
	season.UpdatedAt = now

	result, err := r.db.NamedExecContext(ctx, `
		INSERT INTO seasons (name, year, start_date, end_date, is_active, created_at, updated_at)
		VALUES (:name, :year, :start_date, :end_date, :is_active, :created_at, :updated_at)
	`, season)

	if err != nil {
		return err
	}

	// Get the last inserted ID and set it on the season
	if id, err := result.LastInsertId(); err == nil {
		season.ID = uint(id)
	}

	return nil
}

// Update modifies an existing season record
func (r *seasonRepository) Update(ctx context.Context, season *models.Season) error {
	season.UpdatedAt = time.Now()

	_, err := r.db.NamedExecContext(ctx, `
		UPDATE seasons 
		SET name = :name, year = :year, start_date = :start_date, 
		    end_date = :end_date, is_active = :is_active, updated_at = :updated_at
		WHERE id = :id
	`, season)

	return err
}

// Delete removes a season record by ID
func (r *seasonRepository) Delete(ctx context.Context, id uint) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM seasons WHERE id = ?`, id)
	return err
}

// FindActive retrieves the currently active season
func (r *seasonRepository) FindActive(ctx context.Context) (*models.Season, error) {
	var season models.Season
	err := r.db.GetContext(ctx, &season, `
		SELECT id, name, year, start_date, end_date, is_active, created_at, updated_at
		FROM seasons 
		WHERE is_active = TRUE
		LIMIT 1
	`)
	if err != nil {
		return nil, err
	}
	return &season, nil
}

// FindByYear retrieves all seasons for a specific year
func (r *seasonRepository) FindByYear(ctx context.Context, year int) ([]models.Season, error) {
	var seasons []models.Season
	err := r.db.SelectContext(ctx, &seasons, `
		SELECT id, name, year, start_date, end_date, is_active, created_at, updated_at
		FROM seasons 
		WHERE year = ?
		ORDER BY start_date DESC
	`, year)
	return seasons, err
}

// SetActive sets a season as active (this will automatically deactivate others due to the database trigger)
func (r *seasonRepository) SetActive(ctx context.Context, id uint) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE seasons 
		SET is_active = TRUE, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, id)
	return err
}

// FindWithLeagues retrieves a season with its associated leagues
func (r *seasonRepository) FindWithLeagues(ctx context.Context, id uint) (*models.Season, error) {
	// First get the season
	season, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Then get associated leagues through the league_seasons join table
	var leagues []models.League
	err = r.db.SelectContext(ctx, &leagues, `
		SELECT l.id, l.name, l.type, l.year, l.region, l.created_at, l.updated_at
		FROM leagues l
		INNER JOIN league_seasons ls ON l.id = ls.league_id
		WHERE ls.season_id = ?
		ORDER BY l.name
	`, id)

	if err != nil {
		return nil, err
	}

	season.Leagues = leagues
	return season, nil
}
