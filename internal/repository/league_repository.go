package repository

import (
	"context"
	"fmt"
	"time"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
)

// LeagueRepository handles database operations for League entities
type LeagueRepository struct {
	db *database.DB
}

// NewLeagueRepository creates a new LeagueRepository
func NewLeagueRepository(db *database.DB) *LeagueRepository {
	return &LeagueRepository{db: db}
}

// Create inserts a new league into the database
func (r *LeagueRepository) Create(ctx context.Context, league *models.League) error {
	query := `
		INSERT INTO leagues (name, type, year, region, created_at, updated_at)
		VALUES (:name, :type, :year, :region, :created_at, :updated_at)
		RETURNING id
	`

	now := time.Now()
	league.CreatedAt = now
	league.UpdatedAt = now

	stmt, err := r.db.PrepareNamedContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare query: %w", err)
	}
	defer stmt.Close()

	var id uint
	err = stmt.GetContext(ctx, &id, league)
	if err != nil {
		return fmt.Errorf("failed to insert league: %w", err)
	}

	league.ID = id
	return nil
}

// GetByID retrieves a league by ID
func (r *LeagueRepository) GetByID(ctx context.Context, id uint) (*models.League, error) {
	var league models.League
	query := `SELECT * FROM leagues WHERE id = $1`
	
	err := r.db.GetContext(ctx, &league, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get league by id: %w", err)
	}
	
	return &league, nil
}

// Update updates an existing league
func (r *LeagueRepository) Update(ctx context.Context, league *models.League) error {
	query := `
		UPDATE leagues
		SET name = :name,
			type = :type,
			year = :year,
			region = :region,
			updated_at = :updated_at
		WHERE id = :id
	`

	league.UpdatedAt = time.Now()

	_, err := r.db.NamedExecContext(ctx, query, league)
	if err != nil {
		return fmt.Errorf("failed to update league: %w", err)
	}

	return nil
}

// Delete removes a league from the database
func (r *LeagueRepository) Delete(ctx context.Context, id uint) error {
	// Check if league has divisions
	var divisionCount int
	err := r.db.GetContext(ctx, &divisionCount, `SELECT COUNT(*) FROM divisions WHERE league_id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to check league divisions: %w", err)
	}
	
	if divisionCount > 0 {
		return fmt.Errorf("cannot delete league with %d divisions - please remove them first", divisionCount)
	}
	
	// Check if league has seasons
	var seasonCount int
	err = r.db.GetContext(ctx, &seasonCount, `SELECT COUNT(*) FROM league_seasons WHERE league_id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to check league seasons: %w", err)
	}
	
	if seasonCount > 0 {
		return fmt.Errorf("cannot delete league with %d season associations - please remove those first", seasonCount)
	}
	
	// Delete the league
	_, err = r.db.ExecContext(ctx, `DELETE FROM leagues WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete league: %w", err)
	}
	
	return nil
}

// List retrieves all leagues
func (r *LeagueRepository) List(ctx context.Context) ([]models.League, error) {
	var leagues []models.League
	query := `SELECT * FROM leagues ORDER BY name`
	
	err := r.db.SelectContext(ctx, &leagues, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list leagues: %w", err)
	}
	
	return leagues, nil
}

// GetByType retrieves leagues by type
func (r *LeagueRepository) GetByType(ctx context.Context, leagueType models.LeagueType) ([]models.League, error) {
	var leagues []models.League
	query := `SELECT * FROM leagues WHERE type = $1 ORDER BY name`
	
	err := r.db.SelectContext(ctx, &leagues, query, leagueType)
	if err != nil {
		return nil, fmt.Errorf("failed to get leagues by type: %w", err)
	}
	
	return leagues, nil
}

// GetWithDetails retrieves a league with its divisions and seasons
func (r *LeagueRepository) GetWithDetails(ctx context.Context, id uint) (*models.League, error) {
	league, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	
	// Get divisions for this league
	divisionsQuery := `SELECT * FROM divisions WHERE league_id = $1 ORDER BY level, name`
	var divisions []models.Division
	
	err = r.db.SelectContext(ctx, &divisions, divisionsQuery, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get league divisions: %w", err)
	}
	
	// Get seasons for this league through the many-to-many relationship
	seasonsQuery := `
		SELECT s.* 
		FROM seasons s
		JOIN league_seasons ls ON s.id = ls.season_id
		WHERE ls.league_id = $1
		ORDER BY s.year DESC, s.start_date DESC
	`
	
	var seasons []models.Season
	err = r.db.SelectContext(ctx, &seasons, seasonsQuery, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get league seasons: %w", err)
	}
	
	league.Divisions = divisions
	league.Seasons = seasons
	
	return league, nil
}

// AddSeason associates a season with a league
func (r *LeagueRepository) AddSeason(ctx context.Context, leagueID, seasonID uint) error {
	query := `
		INSERT INTO league_seasons (league_id, season_id, created_at, updated_at)
		VALUES ($1, $2, $3, $3)
		ON CONFLICT (league_id, season_id) DO NOTHING
	`
	
	now := time.Now()
	_, err := r.db.ExecContext(ctx, query, leagueID, seasonID, now)
	if err != nil {
		return fmt.Errorf("failed to associate season with league: %w", err)
	}
	
	return nil
}

// RemoveSeason removes a season association from a league
func (r *LeagueRepository) RemoveSeason(ctx context.Context, leagueID, seasonID uint) error {
	query := `DELETE FROM league_seasons WHERE league_id = $1 AND season_id = $2`
	
	_, err := r.db.ExecContext(ctx, query, leagueID, seasonID)
	if err != nil {
		return fmt.Errorf("failed to remove season from league: %w", err)
	}
	
	return nil
} 