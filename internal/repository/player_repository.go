package repository

import (
	"context"
	"fmt"
	"time"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
)

// PlayerRepository handles database operations for Player entities
type PlayerRepository struct {
	db *database.DB
}

// NewPlayerRepository creates a new PlayerRepository
func NewPlayerRepository(db *database.DB) *PlayerRepository {
	return &PlayerRepository{db: db}
}

// Create inserts a new player into the database
func (r *PlayerRepository) Create(ctx context.Context, player *models.Player) error {
	query := `
		INSERT INTO players (first_name, last_name, email, phone, team_id, created_at, updated_at)
		VALUES (:first_name, :last_name, :email, :phone, :team_id, :created_at, :updated_at)
		RETURNING id
	`

	now := time.Now()
	player.CreatedAt = now
	player.UpdatedAt = now

	stmt, err := r.db.PrepareNamedContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare query: %w", err)
	}
	defer stmt.Close()

	var id uint
	err = stmt.GetContext(ctx, &id, player)
	if err != nil {
		return fmt.Errorf("failed to insert player: %w", err)
	}

	player.ID = id
	return nil
}

// GetByID retrieves a player by ID
func (r *PlayerRepository) GetByID(ctx context.Context, id uint) (*models.Player, error) {
	var player models.Player
	query := `SELECT * FROM players WHERE id = $1`
	
	err := r.db.GetContext(ctx, &player, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get player by id: %w", err)
	}
	
	return &player, nil
}

// Update updates an existing player
func (r *PlayerRepository) Update(ctx context.Context, player *models.Player) error {
	query := `
		UPDATE players
		SET first_name = :first_name,
			last_name = :last_name,
			email = :email,
			phone = :phone,
			team_id = :team_id,
			updated_at = :updated_at
		WHERE id = :id
	`

	player.UpdatedAt = time.Now()

	_, err := r.db.NamedExecContext(ctx, query, player)
	if err != nil {
		return fmt.Errorf("failed to update player: %w", err)
	}

	return nil
}

// Delete removes a player from the database
func (r *PlayerRepository) Delete(ctx context.Context, id uint) error {
	query := `DELETE FROM players WHERE id = $1`
	
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete player: %w", err)
	}
	
	return nil
}

// List retrieves all players, optionally filtered by team ID
func (r *PlayerRepository) List(ctx context.Context, teamID *uint) ([]models.Player, error) {
	var players []models.Player
	var query string
	var args []interface{}
	
	if teamID != nil {
		query = `SELECT * FROM players WHERE team_id = $1 ORDER BY last_name, first_name`
		args = append(args, *teamID)
	} else {
		query = `SELECT * FROM players ORDER BY last_name, first_name`
	}
	
	err := r.db.SelectContext(ctx, &players, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list players: %w", err)
	}
	
	return players, nil
} 