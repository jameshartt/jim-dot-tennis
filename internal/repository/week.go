package repository

import (
	"context"
	"time"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
)

// WeekRepository defines the interface for week data access
type WeekRepository interface {
	// Basic CRUD operations
	FindAll(ctx context.Context) ([]models.Week, error)
	FindByID(ctx context.Context, id uint) (*models.Week, error)
	Create(ctx context.Context, week *models.Week) error
	Update(ctx context.Context, week *models.Week) error
	Delete(ctx context.Context, id uint) error

	// Week-specific queries
	FindBySeason(ctx context.Context, seasonID uint) ([]models.Week, error)
	FindByWeekNumber(ctx context.Context, seasonID uint, weekNumber int) (*models.Week, error)
	FindActive(ctx context.Context, seasonID uint) (*models.Week, error)
	FindByDateRange(ctx context.Context, startDate, endDate time.Time) ([]models.Week, error)
	FindCurrentWeek(ctx context.Context, seasonID uint) (*models.Week, error)
	FindCurrentOrNextWeek(ctx context.Context, seasonID uint) (*models.Week, error)
	SetActive(ctx context.Context, id uint) error

	// Week with relationships
	FindWithFixtures(ctx context.Context, id uint) (*models.Week, error)

	// Statistics
	CountBySeason(ctx context.Context, seasonID uint) (int, error)
}

// weekRepository implements WeekRepository
type weekRepository struct {
	db *database.DB
}

// NewWeekRepository creates a new week repository
func NewWeekRepository(db *database.DB) WeekRepository {
	return &weekRepository{
		db: db,
	}
}

// FindAll retrieves all weeks ordered by season and week number
func (r *weekRepository) FindAll(ctx context.Context) ([]models.Week, error) {
	var weeks []models.Week
	err := r.db.SelectContext(ctx, &weeks, `
		SELECT id, week_number, season_id, start_date, end_date, name, is_active, created_at, updated_at
		FROM weeks 
		ORDER BY season_id DESC, week_number ASC
	`)
	return weeks, err
}

// FindByID retrieves a week by its ID
func (r *weekRepository) FindByID(ctx context.Context, id uint) (*models.Week, error) {
	var week models.Week
	err := r.db.GetContext(ctx, &week, `
		SELECT id, week_number, season_id, start_date, end_date, name, is_active, created_at, updated_at
		FROM weeks 
		WHERE id = ?
	`, id)
	if err != nil {
		return nil, err
	}
	return &week, nil
}

// Create inserts a new week record
func (r *weekRepository) Create(ctx context.Context, week *models.Week) error {
	now := time.Now()
	week.CreatedAt = now
	week.UpdatedAt = now

	result, err := r.db.NamedExecContext(ctx, `
		INSERT INTO weeks (week_number, season_id, start_date, end_date, name, is_active, created_at, updated_at)
		VALUES (:week_number, :season_id, :start_date, :end_date, :name, :is_active, :created_at, :updated_at)
	`, week)

	if err != nil {
		return err
	}

	// Get the last inserted ID and set it on the week
	if id, err := result.LastInsertId(); err == nil {
		week.ID = uint(id)
	}

	return nil
}

// Update modifies an existing week record
func (r *weekRepository) Update(ctx context.Context, week *models.Week) error {
	week.UpdatedAt = time.Now()

	_, err := r.db.NamedExecContext(ctx, `
		UPDATE weeks 
		SET week_number = :week_number, season_id = :season_id, start_date = :start_date, 
		    end_date = :end_date, name = :name, is_active = :is_active, updated_at = :updated_at
		WHERE id = :id
	`, week)

	return err
}

// Delete removes a week record by ID
func (r *weekRepository) Delete(ctx context.Context, id uint) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM weeks WHERE id = ?`, id)
	return err
}

// FindBySeason retrieves all weeks for a specific season
func (r *weekRepository) FindBySeason(ctx context.Context, seasonID uint) ([]models.Week, error) {
	var weeks []models.Week
	err := r.db.SelectContext(ctx, &weeks, `
		SELECT id, week_number, season_id, start_date, end_date, name, is_active, created_at, updated_at
		FROM weeks 
		WHERE season_id = ?
		ORDER BY week_number ASC
	`, seasonID)
	return weeks, err
}

// FindByWeekNumber retrieves a specific week by season and week number
func (r *weekRepository) FindByWeekNumber(ctx context.Context, seasonID uint, weekNumber int) (*models.Week, error) {
	var week models.Week
	err := r.db.GetContext(ctx, &week, `
		SELECT id, week_number, season_id, start_date, end_date, name, is_active, created_at, updated_at
		FROM weeks 
		WHERE season_id = ? AND week_number = ?
	`, seasonID, weekNumber)
	if err != nil {
		return nil, err
	}
	return &week, nil
}

// FindActive retrieves the currently active week for a season
func (r *weekRepository) FindActive(ctx context.Context, seasonID uint) (*models.Week, error) {
	var week models.Week
	err := r.db.GetContext(ctx, &week, `
		SELECT id, week_number, season_id, start_date, end_date, name, is_active, created_at, updated_at
		FROM weeks 
		WHERE season_id = ? AND is_active = TRUE
		LIMIT 1
	`, seasonID)
	if err != nil {
		return nil, err
	}
	return &week, nil
}

// FindByDateRange retrieves weeks that fall within a specific date range
func (r *weekRepository) FindByDateRange(ctx context.Context, startDate, endDate time.Time) ([]models.Week, error) {
	var weeks []models.Week
	err := r.db.SelectContext(ctx, &weeks, `
		SELECT id, week_number, season_id, start_date, end_date, name, is_active, created_at, updated_at
		FROM weeks 
		WHERE start_date <= ? AND end_date >= ?
		ORDER BY season_id DESC, week_number ASC
	`, endDate, startDate)
	return weeks, err
}

// FindCurrentWeek retrieves the week that contains the current date for a season
func (r *weekRepository) FindCurrentWeek(ctx context.Context, seasonID uint) (*models.Week, error) {
	var week models.Week
	now := time.Now()
	err := r.db.GetContext(ctx, &week, `
		SELECT id, week_number, season_id, start_date, end_date, name, is_active, created_at, updated_at
		FROM weeks 
		WHERE season_id = ? AND start_date <= ? AND end_date >= ?
		LIMIT 1
	`, seasonID, now, now)
	if err != nil {
		return nil, err
	}
	return &week, nil
}

// SetActive sets a week as active (this will automatically deactivate others due to the database trigger)
func (r *weekRepository) SetActive(ctx context.Context, id uint) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE weeks 
		SET is_active = TRUE, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, id)
	return err
}

// FindWithFixtures retrieves a week with its associated fixtures
func (r *weekRepository) FindWithFixtures(ctx context.Context, id uint) (*models.Week, error) {
	// First get the week
	week, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Then get associated fixtures
	var fixtures []models.Fixture
	err = r.db.SelectContext(ctx, &fixtures, `
		SELECT id, home_team_id, away_team_id, division_id, season_id, week_id, scheduled_date, 
		       venue_location, status, completed_date, day_captain_id, notes, created_at, updated_at
		FROM fixtures
		WHERE week_id = ?
		ORDER BY scheduled_date ASC
	`, id)

	if err != nil {
		return nil, err
	}

	week.Fixtures = fixtures
	return week, nil
}

// CountBySeason returns the number of weeks in a specific season
func (r *weekRepository) CountBySeason(ctx context.Context, seasonID uint) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM weeks WHERE season_id = ?
	`, seasonID)
	return count, err
}

// FindCurrentOrNextWeek gets the current week or next upcoming week
func (r *weekRepository) FindCurrentOrNextWeek(ctx context.Context, seasonID uint) (*models.Week, error) {
	var week models.Week
	now := time.Now()
	err := r.db.GetContext(ctx, &week, `
		SELECT id, week_number, season_id, start_date, end_date, name, is_active, created_at, updated_at
		FROM weeks
		WHERE season_id = ?
		  AND end_date >= ?
		ORDER BY start_date ASC
		LIMIT 1
	`, seasonID, now)
	if err != nil {
		// Fallback to latest week in season
		err = r.db.GetContext(ctx, &week, `
			SELECT id, week_number, season_id, start_date, end_date, name, is_active, created_at, updated_at
			FROM weeks
			WHERE season_id = ?
			ORDER BY week_number DESC
			LIMIT 1
		`, seasonID)
		if err != nil {
			return nil, err
		}
	}
	return &week, nil
}
