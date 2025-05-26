package repository

import (
	"context"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
	"time"
)

// LeagueRepository defines the interface for league data access
type LeagueRepository interface {
	// Basic CRUD operations
	FindAll(ctx context.Context) ([]models.League, error)
	FindByID(ctx context.Context, id uint) (*models.League, error)
	Create(ctx context.Context, league *models.League) error
	Update(ctx context.Context, league *models.League) error
	Delete(ctx context.Context, id uint) error

	// League-specific queries
	FindByType(ctx context.Context, leagueType models.LeagueType) ([]models.League, error)
	FindByYear(ctx context.Context, year int) ([]models.League, error)
	FindByRegion(ctx context.Context, region string) ([]models.League, error)
	FindByTypeAndYear(ctx context.Context, leagueType models.LeagueType, year int) ([]models.League, error)
	FindWithDivisions(ctx context.Context, id uint) (*models.League, error)
	FindWithSeasons(ctx context.Context, id uint) (*models.League, error)

	// Season association management
	AddSeason(ctx context.Context, leagueID, seasonID uint) error
	RemoveSeason(ctx context.Context, leagueID, seasonID uint) error
	FindSeasonsForLeague(ctx context.Context, leagueID uint) ([]models.Season, error)
}

// leagueRepository implements LeagueRepository
type leagueRepository struct {
	db *database.DB
}

// NewLeagueRepository creates a new league repository
func NewLeagueRepository(db *database.DB) LeagueRepository {
	return &leagueRepository{
		db: db,
	}
}

// FindAll retrieves all leagues ordered by year descending, then by name
func (r *leagueRepository) FindAll(ctx context.Context) ([]models.League, error) {
	var leagues []models.League
	err := r.db.SelectContext(ctx, &leagues, `
		SELECT id, name, type, year, region, created_at, updated_at
		FROM leagues 
		ORDER BY year DESC, name ASC
	`)
	return leagues, err
}

// FindByID retrieves a league by its ID
func (r *leagueRepository) FindByID(ctx context.Context, id uint) (*models.League, error) {
	var league models.League
	err := r.db.GetContext(ctx, &league, `
		SELECT id, name, type, year, region, created_at, updated_at
		FROM leagues 
		WHERE id = ?
	`, id)
	if err != nil {
		return nil, err
	}
	return &league, nil
}

// Create inserts a new league record
func (r *leagueRepository) Create(ctx context.Context, league *models.League) error {
	now := time.Now()
	league.CreatedAt = now
	league.UpdatedAt = now

	result, err := r.db.NamedExecContext(ctx, `
		INSERT INTO leagues (name, type, year, region, created_at, updated_at)
		VALUES (:name, :type, :year, :region, :created_at, :updated_at)
	`, league)

	if err != nil {
		return err
	}

	// Get the last inserted ID and set it on the league
	if id, err := result.LastInsertId(); err == nil {
		league.ID = uint(id)
	}

	return nil
}

// Update modifies an existing league record
func (r *leagueRepository) Update(ctx context.Context, league *models.League) error {
	league.UpdatedAt = time.Now()

	_, err := r.db.NamedExecContext(ctx, `
		UPDATE leagues 
		SET name = :name, type = :type, year = :year, region = :region, updated_at = :updated_at
		WHERE id = :id
	`, league)

	return err
}

// Delete removes a league record by ID
func (r *leagueRepository) Delete(ctx context.Context, id uint) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM leagues WHERE id = ?`, id)
	return err
}

// FindByType retrieves all leagues of a specific type
func (r *leagueRepository) FindByType(ctx context.Context, leagueType models.LeagueType) ([]models.League, error) {
	var leagues []models.League
	err := r.db.SelectContext(ctx, &leagues, `
		SELECT id, name, type, year, region, created_at, updated_at
		FROM leagues 
		WHERE type = ?
		ORDER BY year DESC, name ASC
	`, string(leagueType))
	return leagues, err
}

// FindByYear retrieves all leagues for a specific year
func (r *leagueRepository) FindByYear(ctx context.Context, year int) ([]models.League, error) {
	var leagues []models.League
	err := r.db.SelectContext(ctx, &leagues, `
		SELECT id, name, type, year, region, created_at, updated_at
		FROM leagues 
		WHERE year = ?
		ORDER BY name ASC
	`, year)
	return leagues, err
}

// FindByRegion retrieves all leagues in a specific region
func (r *leagueRepository) FindByRegion(ctx context.Context, region string) ([]models.League, error) {
	var leagues []models.League
	err := r.db.SelectContext(ctx, &leagues, `
		SELECT id, name, type, year, region, created_at, updated_at
		FROM leagues 
		WHERE region = ?
		ORDER BY year DESC, name ASC
	`, region)
	return leagues, err
}

// FindByTypeAndYear retrieves leagues by type and year
func (r *leagueRepository) FindByTypeAndYear(ctx context.Context, leagueType models.LeagueType, year int) ([]models.League, error) {
	var leagues []models.League
	err := r.db.SelectContext(ctx, &leagues, `
		SELECT id, name, type, year, region, created_at, updated_at
		FROM leagues 
		WHERE type = ? AND year = ?
		ORDER BY name ASC
	`, string(leagueType), year)
	return leagues, err
}

// FindWithDivisions retrieves a league with its associated divisions
func (r *leagueRepository) FindWithDivisions(ctx context.Context, id uint) (*models.League, error) {
	// First get the league
	league, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Then get associated divisions
	var divisions []models.Division
	err = r.db.SelectContext(ctx, &divisions, `
		SELECT id, name, level, play_day, league_id, season_id, max_teams_per_club, created_at, updated_at
		FROM divisions
		WHERE league_id = ?
		ORDER BY level ASC, name ASC
	`, id)

	if err != nil {
		return nil, err
	}

	league.Divisions = divisions
	return league, nil
}

// FindWithSeasons retrieves a league with its associated seasons
func (r *leagueRepository) FindWithSeasons(ctx context.Context, id uint) (*models.League, error) {
	// First get the league
	league, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Then get associated seasons through the league_seasons join table
	var seasons []models.Season
	err = r.db.SelectContext(ctx, &seasons, `
		SELECT s.id, s.name, s.year, s.start_date, s.end_date, s.is_active, s.created_at, s.updated_at
		FROM seasons s
		INNER JOIN league_seasons ls ON s.id = ls.season_id
		WHERE ls.league_id = ?
		ORDER BY s.year DESC, s.start_date DESC
	`, id)

	if err != nil {
		return nil, err
	}

	league.Seasons = seasons
	return league, nil
}

// AddSeason creates an association between a league and a season
func (r *leagueRepository) AddSeason(ctx context.Context, leagueID, seasonID uint) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO league_seasons (league_id, season_id, created_at, updated_at)
		VALUES (?, ?, ?, ?)
	`, leagueID, seasonID, now, now)
	return err
}

// RemoveSeason removes the association between a league and a season
func (r *leagueRepository) RemoveSeason(ctx context.Context, leagueID, seasonID uint) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM league_seasons 
		WHERE league_id = ? AND season_id = ?
	`, leagueID, seasonID)
	return err
}

// FindSeasonsForLeague retrieves all seasons associated with a specific league
func (r *leagueRepository) FindSeasonsForLeague(ctx context.Context, leagueID uint) ([]models.Season, error) {
	var seasons []models.Season
	err := r.db.SelectContext(ctx, &seasons, `
		SELECT s.id, s.name, s.year, s.start_date, s.end_date, s.is_active, s.created_at, s.updated_at
		FROM seasons s
		INNER JOIN league_seasons ls ON s.id = ls.season_id
		WHERE ls.league_id = ?
		ORDER BY s.year DESC, s.start_date DESC
	`, leagueID)
	return seasons, err
}
