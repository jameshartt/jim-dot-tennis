package repository

import (
	"context"
	"fmt"
	"time"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
)

// MatchupRepository handles database operations for Matchup entities
type MatchupRepository struct {
	db *database.DB
}

// NewMatchupRepository creates a new MatchupRepository
func NewMatchupRepository(db *database.DB) *MatchupRepository {
	return &MatchupRepository{db: db}
}

// Create inserts a new matchup into the database
func (r *MatchupRepository) Create(ctx context.Context, matchup *models.Matchup) error {
	query := `
		INSERT INTO matchups (
			fixture_id, type, status, home_score, away_score, 
			notes, created_at, updated_at
		)
		VALUES (
			:fixture_id, :type, :status, :home_score, :away_score, 
			:notes, :created_at, :updated_at
		)
		RETURNING id
	`

	now := time.Now()
	matchup.CreatedAt = now
	matchup.UpdatedAt = now

	stmt, err := r.db.PrepareNamedContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare query: %w", err)
	}
	defer stmt.Close()

	var id uint
	err = stmt.GetContext(ctx, &id, matchup)
	if err != nil {
		return fmt.Errorf("failed to insert matchup: %w", err)
	}

	matchup.ID = id
	return nil
}

// GetByID retrieves a matchup by ID
func (r *MatchupRepository) GetByID(ctx context.Context, id uint) (*models.Matchup, error) {
	var matchup models.Matchup
	query := `SELECT * FROM matchups WHERE id = $1`
	
	err := r.db.GetContext(ctx, &matchup, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get matchup by id: %w", err)
	}
	
	return &matchup, nil
}

// Update updates an existing matchup
func (r *MatchupRepository) Update(ctx context.Context, matchup *models.Matchup) error {
	query := `
		UPDATE matchups
		SET fixture_id = :fixture_id,
			type = :type,
			status = :status,
			home_score = :home_score,
			away_score = :away_score,
			notes = :notes,
			updated_at = :updated_at
		WHERE id = :id
	`

	matchup.UpdatedAt = time.Now()

	_, err := r.db.NamedExecContext(ctx, query, matchup)
	if err != nil {
		return fmt.Errorf("failed to update matchup: %w", err)
	}

	return nil
}

// Delete removes a matchup from the database
func (r *MatchupRepository) Delete(ctx context.Context, id uint) error {
	// First delete any matchup players
	_, err := r.db.ExecContext(ctx, `DELETE FROM matchup_players WHERE matchup_id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete matchup players: %w", err)
	}
	
	// Then delete the matchup
	_, err = r.db.ExecContext(ctx, `DELETE FROM matchups WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete matchup: %w", err)
	}
	
	return nil
}

// GetByFixture retrieves matchups by fixture ID
func (r *MatchupRepository) GetByFixture(ctx context.Context, fixtureID uint) ([]models.Matchup, error) {
	var matchups []models.Matchup
	query := `SELECT * FROM matchups WHERE fixture_id = $1 ORDER BY type`
	
	err := r.db.SelectContext(ctx, &matchups, query, fixtureID)
	if err != nil {
		return nil, fmt.Errorf("failed to get matchups by fixture: %w", err)
	}
	
	return matchups, nil
}

// GetWithPlayers retrieves a matchup with its players
func (r *MatchupRepository) GetWithPlayers(ctx context.Context, id uint) (*models.Matchup, error) {
	matchup, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	
	// Get players for this matchup
	query := `
		SELECT mp.* 
		FROM matchup_players mp
		WHERE mp.matchup_id = $1
	`
	
	var matchupPlayers []models.MatchupPlayer
	err = r.db.SelectContext(ctx, &matchupPlayers, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get matchup players: %w", err)
	}
	
	// We could return the matchupPlayers here, but they would need to be processed
	// by the service layer to get the actual player details
	
	return matchup, nil
}

// UpdateScore updates the score of a matchup
func (r *MatchupRepository) UpdateScore(ctx context.Context, id uint, homeScore, awayScore int) error {
	query := `
		UPDATE matchups
		SET home_score = $1, 
			away_score = $2, 
			updated_at = $3
		WHERE id = $4
	`
	
	_, err := r.db.ExecContext(ctx, query, homeScore, awayScore, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update matchup score: %w", err)
	}
	
	return nil
}

// UpdateStatus updates the status of a matchup
func (r *MatchupRepository) UpdateStatus(ctx context.Context, id uint, status models.MatchupStatus) error {
	query := `
		UPDATE matchups
		SET status = $1, updated_at = $2
		WHERE id = $3
	`
	
	_, err := r.db.ExecContext(ctx, query, status, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update matchup status: %w", err)
	}
	
	return nil
}

// AddPlayer adds a player to a matchup
func (r *MatchupRepository) AddPlayer(ctx context.Context, matchupID uint, playerID string, isHome bool) error {
	// Check if player already exists in this matchup
	var count int
	err := r.db.GetContext(
		ctx, 
		&count, 
		`SELECT COUNT(*) FROM matchup_players WHERE matchup_id = $1 AND player_id = $2`,
		matchupID, playerID,
	)
	if err != nil {
		return fmt.Errorf("failed to check if player exists in matchup: %w", err)
	}
	
	if count > 0 {
		return fmt.Errorf("player already exists in this matchup")
	}
	
	// Add the player to the matchup
	query := `
		INSERT INTO matchup_players (
			matchup_id, player_id, is_home, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $4)
	`
	
	now := time.Now()
	_, err = r.db.ExecContext(ctx, query, matchupID, playerID, isHome, now)
	if err != nil {
		return fmt.Errorf("failed to add player to matchup: %w", err)
	}
	
	return nil
}

// RemovePlayer removes a player from a matchup
func (r *MatchupRepository) RemovePlayer(ctx context.Context, matchupID uint, playerID string) error {
	query := `DELETE FROM matchup_players WHERE matchup_id = $1 AND player_id = $2`
	
	result, err := r.db.ExecContext(ctx, query, matchupID, playerID)
	if err != nil {
		return fmt.Errorf("failed to remove player from matchup: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("player was not in this matchup")
	}
	
	return nil
}

// GetPlayersByMatchup retrieves players by matchup ID
func (r *MatchupRepository) GetPlayersByMatchup(ctx context.Context, matchupID uint) ([]models.MatchupPlayer, error) {
	var matchupPlayers []models.MatchupPlayer
	query := `
		SELECT mp.* 
		FROM matchup_players mp
		WHERE mp.matchup_id = $1
	`
	
	err := r.db.SelectContext(ctx, &matchupPlayers, query, matchupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get players by matchup: %w", err)
	}
	
	return matchupPlayers, nil
} 