package repository

import (
	"context"
	"time"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
)

// MatchupRepository defines the interface for matchup data access
type MatchupRepository interface {
	// Basic CRUD operations
	FindAll(ctx context.Context) ([]models.Matchup, error)
	FindByID(ctx context.Context, id uint) (*models.Matchup, error)
	Create(ctx context.Context, matchup *models.Matchup) error
	Update(ctx context.Context, matchup *models.Matchup) error
	Delete(ctx context.Context, id uint) error

	// Matchup-specific queries
	FindByFixture(ctx context.Context, fixtureID uint) ([]models.Matchup, error)
	FindByType(ctx context.Context, matchupType models.MatchupType) ([]models.Matchup, error)
	FindByStatus(ctx context.Context, status models.MatchupStatus) ([]models.Matchup, error)
	FindByFixtureAndType(ctx context.Context, fixtureID uint, matchupType models.MatchupType) (*models.Matchup, error)
	FindByFixtureTypeAndTeam(ctx context.Context, fixtureID uint, matchupType models.MatchupType, managingTeamID uint) (*models.Matchup, error)

	// Matchup with relationships
	FindWithPlayers(ctx context.Context, id uint) (*models.Matchup, error)

	// Matchup Player management
	AddPlayer(ctx context.Context, matchupID uint, playerID string, isHome bool) error
	RemovePlayer(ctx context.Context, matchupID uint, playerID string) error
	FindPlayersInMatchup(ctx context.Context, matchupID uint) ([]models.MatchupPlayer, error)
	ClearPlayers(ctx context.Context, matchupID uint) error

	// Status management
	UpdateStatus(ctx context.Context, id uint, status models.MatchupStatus) error
	UpdateScore(ctx context.Context, id uint, homeScore, awayScore int) error

	// Statistics
	CountByFixture(ctx context.Context, fixtureID uint) (int, error)
	CountByStatus(ctx context.Context, status models.MatchupStatus) (int, error)
}

// matchupRepository implements MatchupRepository
type matchupRepository struct {
	db *database.DB
}

// NewMatchupRepository creates a new matchup repository
func NewMatchupRepository(db *database.DB) MatchupRepository {
	return &matchupRepository{
		db: db,
	}
}

// FindAll retrieves all matchups ordered by fixture and type
func (r *matchupRepository) FindAll(ctx context.Context) ([]models.Matchup, error) {
	var matchups []models.Matchup
	err := r.db.SelectContext(ctx, &matchups, `
		SELECT id, fixture_id, type, status, home_score, away_score, 
		       home_set1, away_set1, home_set2, away_set2, home_set3, away_set3,
		       notes, managing_team_id, conceded_by, created_at, updated_at
		FROM matchups 
		ORDER BY fixture_id ASC, type ASC
	`)
	return matchups, err
}

// FindByID retrieves a matchup by its ID
func (r *matchupRepository) FindByID(ctx context.Context, id uint) (*models.Matchup, error) {
	var matchup models.Matchup
	err := r.db.GetContext(ctx, &matchup, `
		SELECT id, fixture_id, type, status, home_score, away_score,
		       home_set1, away_set1, home_set2, away_set2, home_set3, away_set3,
		       notes, managing_team_id, conceded_by, created_at, updated_at
		FROM matchups 
		WHERE id = ?
	`, id)
	if err != nil {
		return nil, err
	}
	return &matchup, nil
}

// Create inserts a new matchup record
func (r *matchupRepository) Create(ctx context.Context, matchup *models.Matchup) error {
	now := time.Now()
	matchup.CreatedAt = now
	matchup.UpdatedAt = now

	result, err := r.db.NamedExecContext(ctx, `
		INSERT INTO matchups (fixture_id, type, status, home_score, away_score, 
		                     home_set1, away_set1, home_set2, away_set2, home_set3, away_set3,
		                     notes, managing_team_id, conceded_by, created_at, updated_at)
		VALUES (:fixture_id, :type, :status, :home_score, :away_score,
		        :home_set1, :away_set1, :home_set2, :away_set2, :home_set3, :away_set3,
		        :notes, :managing_team_id, :conceded_by, :created_at, :updated_at)
	`, matchup)

	if err != nil {
		return err
	}

	// Get the last inserted ID and set it on the matchup
	if id, err := result.LastInsertId(); err == nil {
		matchup.ID = uint(id)
	}

	return nil
}

// Update modifies an existing matchup record
func (r *matchupRepository) Update(ctx context.Context, matchup *models.Matchup) error {
	matchup.UpdatedAt = time.Now()

	_, err := r.db.NamedExecContext(ctx, `
		UPDATE matchups 
		SET fixture_id = :fixture_id, type = :type, status = :status, 
		    home_score = :home_score, away_score = :away_score,
		    home_set1 = :home_set1, away_set1 = :away_set1,
		    home_set2 = :home_set2, away_set2 = :away_set2,
		    home_set3 = :home_set3, away_set3 = :away_set3,
		    notes = :notes, managing_team_id = :managing_team_id, conceded_by = :conceded_by, updated_at = :updated_at
		WHERE id = :id
	`, matchup)

	return err
}

// Delete removes a matchup record by ID
func (r *matchupRepository) Delete(ctx context.Context, id uint) error {
	// First delete all associated matchup players
	_, err := r.db.ExecContext(ctx, `DELETE FROM matchup_players WHERE matchup_id = ?`, id)
	if err != nil {
		return err
	}

	// Then delete the matchup
	_, err = r.db.ExecContext(ctx, `DELETE FROM matchups WHERE id = ?`, id)
	return err
}

// FindByFixture retrieves all matchups for a specific fixture
func (r *matchupRepository) FindByFixture(ctx context.Context, fixtureID uint) ([]models.Matchup, error) {
	var matchups []models.Matchup
	err := r.db.SelectContext(ctx, &matchups, `
		SELECT id, fixture_id, type, status, home_score, away_score,
		       home_set1, away_set1, home_set2, away_set2, home_set3, away_set3,
		       notes, managing_team_id, conceded_by, created_at, updated_at
		FROM matchups 
		WHERE fixture_id = ?
		ORDER BY type ASC
	`, fixtureID)
	return matchups, err
}

// FindByType retrieves all matchups of a specific type
func (r *matchupRepository) FindByType(ctx context.Context, matchupType models.MatchupType) ([]models.Matchup, error) {
	var matchups []models.Matchup
	err := r.db.SelectContext(ctx, &matchups, `
		SELECT id, fixture_id, type, status, home_score, away_score,
		       home_set1, away_set1, home_set2, away_set2, home_set3, away_set3,
		       notes, managing_team_id, conceded_by, created_at, updated_at
		FROM matchups 
		WHERE type = ?
		ORDER BY fixture_id ASC
	`, string(matchupType))
	return matchups, err
}

// FindByStatus retrieves all matchups with a specific status
func (r *matchupRepository) FindByStatus(ctx context.Context, status models.MatchupStatus) ([]models.Matchup, error) {
	var matchups []models.Matchup
	err := r.db.SelectContext(ctx, &matchups, `
		SELECT id, fixture_id, type, status, home_score, away_score,
		       home_set1, away_set1, home_set2, away_set2, home_set3, away_set3,
		       notes, managing_team_id, conceded_by, created_at, updated_at
		FROM matchups 
		WHERE status = ?
		ORDER BY fixture_id ASC, type ASC
	`, string(status))
	return matchups, err
}

// FindByFixtureAndType retrieves a specific matchup by fixture and type
func (r *matchupRepository) FindByFixtureAndType(ctx context.Context, fixtureID uint, matchupType models.MatchupType) (*models.Matchup, error) {
	var matchup models.Matchup
	err := r.db.GetContext(ctx, &matchup, `
		SELECT id, fixture_id, type, status, home_score, away_score,
		       home_set1, away_set1, home_set2, away_set2, home_set3, away_set3,
		       notes, managing_team_id, conceded_by, created_at, updated_at
		FROM matchups 
		WHERE fixture_id = ? AND type = ?
	`, fixtureID, string(matchupType))
	if err != nil {
		return nil, err
	}
	return &matchup, nil
}

// FindByFixtureTypeAndTeam retrieves a specific matchup by fixture, type, and managing team (for derby matches)
func (r *matchupRepository) FindByFixtureTypeAndTeam(ctx context.Context, fixtureID uint, matchupType models.MatchupType, managingTeamID uint) (*models.Matchup, error) {
	var matchup models.Matchup
	err := r.db.GetContext(ctx, &matchup, `
		SELECT id, fixture_id, type, status, home_score, away_score,
		       home_set1, away_set1, home_set2, away_set2, home_set3, away_set3,
		       notes, managing_team_id, conceded_by, created_at, updated_at
		FROM matchups 
		WHERE fixture_id = ? AND type = ? AND managing_team_id = ?
	`, fixtureID, string(matchupType), managingTeamID)
	if err != nil {
		return nil, err
	}
	return &matchup, nil
}

// FindWithPlayers retrieves a matchup with its associated players
func (r *matchupRepository) FindWithPlayers(ctx context.Context, id uint) (*models.Matchup, error) {
	// First get the matchup
	matchup, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Then get associated players
	players, err := r.FindPlayersInMatchup(ctx, id)
	if err != nil {
		return nil, err
	}

	// Note: We would need to add a Players field to the Matchup model to store this
	// For now, this function structure is prepared for when that field is added
	_ = players // Use players variable to avoid compiler warning

	return matchup, nil
}

// AddPlayer adds a player to a matchup
func (r *matchupRepository) AddPlayer(ctx context.Context, matchupID uint, playerID string, isHome bool) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO matchup_players (matchup_id, player_id, is_home, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`, matchupID, playerID, isHome, now, now)
	return err
}

// RemovePlayer removes a player from a matchup
func (r *matchupRepository) RemovePlayer(ctx context.Context, matchupID uint, playerID string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM matchup_players 
		WHERE matchup_id = ? AND player_id = ?
	`, matchupID, playerID)
	return err
}

// FindPlayersInMatchup retrieves all players in a specific matchup
func (r *matchupRepository) FindPlayersInMatchup(ctx context.Context, matchupID uint) ([]models.MatchupPlayer, error) {
	var players []models.MatchupPlayer
	err := r.db.SelectContext(ctx, &players, `
		SELECT id, matchup_id, player_id, is_home, created_at, updated_at
		FROM matchup_players
		WHERE matchup_id = ?
		ORDER BY is_home DESC, created_at ASC
	`, matchupID)
	return players, err
}

// ClearPlayers removes all players from a matchup
func (r *matchupRepository) ClearPlayers(ctx context.Context, matchupID uint) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM matchup_players WHERE matchup_id = ?
	`, matchupID)
	return err
}

// UpdateStatus updates the status of a matchup
func (r *matchupRepository) UpdateStatus(ctx context.Context, id uint, status models.MatchupStatus) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE matchups 
		SET status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, string(status), id)
	return err
}

// UpdateScore updates the scores of a matchup
func (r *matchupRepository) UpdateScore(ctx context.Context, id uint, homeScore, awayScore int) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE matchups 
		SET home_score = ?, away_score = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, homeScore, awayScore, id)
	return err
}

// CountByFixture returns the number of matchups in a specific fixture
func (r *matchupRepository) CountByFixture(ctx context.Context, fixtureID uint) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM matchups WHERE fixture_id = ?
	`, fixtureID)
	return count, err
}

// CountByStatus returns the number of matchups with a specific status
func (r *matchupRepository) CountByStatus(ctx context.Context, status models.MatchupStatus) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM matchups WHERE status = ?
	`, string(status))
	return count, err
}
