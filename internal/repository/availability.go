package repository

import (
	"context"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
	"time"
)

// AvailabilityRepository defines the interface for player availability data access
type AvailabilityRepository interface {
	// Player date-specific availability
	GetPlayerAvailabilityByDate(ctx context.Context, playerID string, date time.Time) (*models.PlayerAvailabilityException, error)
	GetPlayerAvailabilityByDateRange(ctx context.Context, playerID string, startDate, endDate time.Time) ([]models.PlayerAvailabilityException, error)
	UpsertPlayerAvailability(ctx context.Context, playerID string, date time.Time, status models.AvailabilityStatus, reason string) error
	DeletePlayerAvailability(ctx context.Context, playerID string, date time.Time) error

	// Player general availability (day of week patterns)
	GetPlayerGeneralAvailability(ctx context.Context, playerID string, seasonID uint) ([]models.PlayerGeneralAvailability, error)
	UpsertPlayerGeneralAvailability(ctx context.Context, playerID string, seasonID uint, dayOfWeek string, status models.AvailabilityStatus, notes string) error

	// Player fixture-specific availability
	GetPlayerFixtureAvailability(ctx context.Context, playerID string, fixtureID uint) (*models.PlayerFixtureAvailability, error)
	UpsertPlayerFixtureAvailability(ctx context.Context, playerID string, fixtureID uint, status models.AvailabilityStatus, notes string) error

	// Batch operations
	BatchUpsertPlayerAvailability(ctx context.Context, playerID string, availabilities []AvailabilityUpdate) error
}

// AvailabilityUpdate represents a single availability update
type AvailabilityUpdate struct {
	Date   time.Time
	Status models.AvailabilityStatus
	Reason string
}

// availabilityRepository implements AvailabilityRepository
type availabilityRepository struct {
	db *database.DB
}

// NewAvailabilityRepository creates a new availability repository
func NewAvailabilityRepository(db *database.DB) AvailabilityRepository {
	return &availabilityRepository{
		db: db,
	}
}

// GetPlayerAvailabilityByDate retrieves a player's availability for a specific date
func (r *availabilityRepository) GetPlayerAvailabilityByDate(ctx context.Context, playerID string, date time.Time) (*models.PlayerAvailabilityException, error) {
	var availability models.PlayerAvailabilityException
	err := r.db.GetContext(ctx, &availability, `
		SELECT id, player_id, status, start_date, end_date, reason, created_at, updated_at
		FROM player_availability_exceptions
		WHERE player_id = ? AND start_date <= ? AND end_date >= ?
		ORDER BY created_at DESC
		LIMIT 1
	`, playerID, date, date)

	if err != nil {
		return nil, err
	}
	return &availability, nil
}

// GetPlayerAvailabilityByDateRange retrieves a player's availability for a date range
func (r *availabilityRepository) GetPlayerAvailabilityByDateRange(ctx context.Context, playerID string, startDate, endDate time.Time) ([]models.PlayerAvailabilityException, error) {
	var availabilities []models.PlayerAvailabilityException
	err := r.db.SelectContext(ctx, &availabilities, `
		SELECT id, player_id, status, start_date, end_date, reason, created_at, updated_at
		FROM player_availability_exceptions
		WHERE player_id = ? AND (
			(start_date >= ? AND start_date <= ?) OR
			(end_date >= ? AND end_date <= ?) OR
			(start_date <= ? AND end_date >= ?)
		)
		ORDER BY start_date ASC
	`, playerID, startDate, endDate, startDate, endDate, startDate, endDate)

	return availabilities, err
}

// UpsertPlayerAvailability creates or updates a player's availability for a specific date
func (r *availabilityRepository) UpsertPlayerAvailability(ctx context.Context, playerID string, date time.Time, status models.AvailabilityStatus, reason string) error {
	// For simplicity, we'll store each date as a single-day exception
	// In a production system, you might want to merge adjacent dates with the same status

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete any existing availability for this exact date
	_, err = tx.ExecContext(ctx, `
		DELETE FROM player_availability_exceptions 
		WHERE player_id = ? AND start_date = ? AND end_date = ?
	`, playerID, date, date)
	if err != nil {
		return err
	}

	// Insert new availability (unless it's 'clear' status)
	if status != "clear" {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO player_availability_exceptions (player_id, status, start_date, end_date, reason)
			VALUES (?, ?, ?, ?, ?)
		`, playerID, string(status), date, date, reason)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// DeletePlayerAvailability removes a player's availability for a specific date
func (r *availabilityRepository) DeletePlayerAvailability(ctx context.Context, playerID string, date time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM player_availability_exceptions 
		WHERE player_id = ? AND start_date = ? AND end_date = ?
	`, playerID, date, date)
	return err
}

// GetPlayerGeneralAvailability retrieves a player's general availability patterns
func (r *availabilityRepository) GetPlayerGeneralAvailability(ctx context.Context, playerID string, seasonID uint) ([]models.PlayerGeneralAvailability, error) {
	var availabilities []models.PlayerGeneralAvailability
	err := r.db.SelectContext(ctx, &availabilities, `
		SELECT id, player_id, day_of_week, status, season_id, notes, created_at, updated_at
		FROM player_general_availability
		WHERE player_id = ? AND season_id = ?
		ORDER BY 
			CASE day_of_week 
				WHEN 'Monday' THEN 1
				WHEN 'Tuesday' THEN 2
				WHEN 'Wednesday' THEN 3
				WHEN 'Thursday' THEN 4
				WHEN 'Friday' THEN 5
				WHEN 'Saturday' THEN 6
				WHEN 'Sunday' THEN 7
				ELSE 8
			END
	`, playerID, seasonID)

	return availabilities, err
}

// UpsertPlayerGeneralAvailability creates or updates a player's general availability
func (r *availabilityRepository) UpsertPlayerGeneralAvailability(ctx context.Context, playerID string, seasonID uint, dayOfWeek string, status models.AvailabilityStatus, notes string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO player_general_availability (player_id, day_of_week, status, season_id, notes)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(player_id, day_of_week, season_id) DO UPDATE SET
			status = excluded.status,
			notes = excluded.notes,
			updated_at = CURRENT_TIMESTAMP
	`, playerID, dayOfWeek, string(status), seasonID, notes)

	return err
}

// GetPlayerFixtureAvailability retrieves a player's availability for a specific fixture
func (r *availabilityRepository) GetPlayerFixtureAvailability(ctx context.Context, playerID string, fixtureID uint) (*models.PlayerFixtureAvailability, error) {
	var availability models.PlayerFixtureAvailability
	err := r.db.GetContext(ctx, &availability, `
		SELECT id, player_id, fixture_id, status, notes, created_at, updated_at
		FROM player_fixture_availability
		WHERE player_id = ? AND fixture_id = ?
	`, playerID, fixtureID)

	if err != nil {
		return nil, err
	}
	return &availability, nil
}

// UpsertPlayerFixtureAvailability creates or updates a player's fixture availability
func (r *availabilityRepository) UpsertPlayerFixtureAvailability(ctx context.Context, playerID string, fixtureID uint, status models.AvailabilityStatus, notes string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO player_fixture_availability (player_id, fixture_id, status, notes)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(player_id, fixture_id) DO UPDATE SET
			status = excluded.status,
			notes = excluded.notes,
			updated_at = CURRENT_TIMESTAMP
	`, playerID, fixtureID, string(status), notes)

	return err
}

// BatchUpsertPlayerAvailability performs multiple availability updates in a single transaction
func (r *availabilityRepository) BatchUpsertPlayerAvailability(ctx context.Context, playerID string, availabilities []AvailabilityUpdate) error {
	if len(availabilities) == 0 {
		return nil
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, avail := range availabilities {
		// Delete existing availability for this date
		_, err = tx.ExecContext(ctx, `
			DELETE FROM player_availability_exceptions 
			WHERE player_id = ? AND start_date = ? AND end_date = ?
		`, playerID, avail.Date, avail.Date)
		if err != nil {
			return err
		}

		// Insert new availability (unless it's 'clear' status)
		if avail.Status != "clear" {
			_, err = tx.ExecContext(ctx, `
				INSERT INTO player_availability_exceptions (player_id, status, start_date, end_date, reason)
				VALUES (?, ?, ?, ?, ?)
			`, playerID, string(avail.Status), avail.Date, avail.Date, avail.Reason)
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}
