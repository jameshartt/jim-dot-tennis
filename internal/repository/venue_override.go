package repository

import (
	"context"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
	"time"
)

// VenueOverrideRepository defines the interface for venue override data access
type VenueOverrideRepository interface {
	FindAll(ctx context.Context) ([]models.VenueOverride, error)
	FindByID(ctx context.Context, id uint) (*models.VenueOverride, error)
	Create(ctx context.Context, override *models.VenueOverride) error
	Update(ctx context.Context, override *models.VenueOverride) error
	Delete(ctx context.Context, id uint) error
	FindByClub(ctx context.Context, clubID uint) ([]models.VenueOverride, error)
	FindActiveForClubOnDate(ctx context.Context, clubID uint, date time.Time) (*models.VenueOverride, error)
}

type venueOverrideRepository struct {
	db *database.DB
}

// NewVenueOverrideRepository creates a new venue override repository
func NewVenueOverrideRepository(db *database.DB) VenueOverrideRepository {
	return &venueOverrideRepository{db: db}
}

const venueOverrideColumns = `id, club_id, venue_club_id, start_date, end_date, reason, created_at, updated_at`

func (r *venueOverrideRepository) FindAll(ctx context.Context) ([]models.VenueOverride, error) {
	var overrides []models.VenueOverride
	err := r.db.SelectContext(ctx, &overrides, `
		SELECT `+venueOverrideColumns+`
		FROM venue_overrides
		ORDER BY start_date DESC
	`)
	return overrides, err
}

func (r *venueOverrideRepository) FindByID(ctx context.Context, id uint) (*models.VenueOverride, error) {
	var override models.VenueOverride
	err := r.db.GetContext(ctx, &override, `
		SELECT `+venueOverrideColumns+`
		FROM venue_overrides
		WHERE id = ?
	`, id)
	if err != nil {
		return nil, err
	}
	return &override, nil
}

func (r *venueOverrideRepository) Create(ctx context.Context, override *models.VenueOverride) error {
	now := time.Now()
	override.CreatedAt = now
	override.UpdatedAt = now

	result, err := r.db.NamedExecContext(ctx, `
		INSERT INTO venue_overrides (club_id, venue_club_id, start_date, end_date, reason, created_at, updated_at)
		VALUES (:club_id, :venue_club_id, :start_date, :end_date, :reason, :created_at, :updated_at)
	`, override)
	if err != nil {
		return err
	}

	if id, err := result.LastInsertId(); err == nil {
		override.ID = uint(id)
	}
	return nil
}

func (r *venueOverrideRepository) Update(ctx context.Context, override *models.VenueOverride) error {
	override.UpdatedAt = time.Now()

	_, err := r.db.NamedExecContext(ctx, `
		UPDATE venue_overrides
		SET club_id = :club_id, venue_club_id = :venue_club_id,
		    start_date = :start_date, end_date = :end_date,
		    reason = :reason, updated_at = :updated_at
		WHERE id = :id
	`, override)
	return err
}

func (r *venueOverrideRepository) Delete(ctx context.Context, id uint) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM venue_overrides WHERE id = ?`, id)
	return err
}

func (r *venueOverrideRepository) FindByClub(ctx context.Context, clubID uint) ([]models.VenueOverride, error) {
	var overrides []models.VenueOverride
	err := r.db.SelectContext(ctx, &overrides, `
		SELECT `+venueOverrideColumns+`
		FROM venue_overrides
		WHERE club_id = ?
		ORDER BY start_date DESC
	`, clubID)
	return overrides, err
}

// FindActiveForClubOnDate finds an active venue override for a club on a specific date
func (r *venueOverrideRepository) FindActiveForClubOnDate(ctx context.Context, clubID uint, date time.Time) (*models.VenueOverride, error) {
	var override models.VenueOverride
	err := r.db.GetContext(ctx, &override, `
		SELECT `+venueOverrideColumns+`
		FROM venue_overrides
		WHERE club_id = ?
		  AND date(start_date) <= date(?)
		  AND date(end_date) >= date(?)
		ORDER BY created_at DESC
		LIMIT 1
	`, clubID, date, date)
	if err != nil {
		return nil, err
	}
	return &override, nil
}
