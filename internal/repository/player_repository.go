package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
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
	// If no ID is provided, generate a new UUID
	if player.ID == "" {
		player.ID = uuid.New().String()
	}

	query := `
		INSERT INTO players (id, first_name, last_name, email, phone, club_id, created_at, updated_at)
		VALUES (:id, :first_name, :last_name, :email, :phone, :club_id, :created_at, :updated_at)
	`

	now := time.Now()
	player.CreatedAt = now
	player.UpdatedAt = now

	_, err := r.db.NamedExecContext(ctx, query, player)
	if err != nil {
		return fmt.Errorf("failed to insert player: %w", err)
	}

	return nil
}

// GetByID retrieves a player by ID
func (r *PlayerRepository) GetByID(ctx context.Context, id string) (*models.Player, error) {
	var player models.Player
	query := `SELECT * FROM players WHERE id = $1`
	
	err := r.db.GetContext(ctx, &player, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get player by id: %w", err)
	}
	
	// Get teams for this player
	teamsQuery := `
		SELECT team_id FROM player_teams 
		WHERE player_id = $1 AND is_active = true
	`
	
	var teamIDs []uint
	err = r.db.SelectContext(ctx, &teamIDs, teamsQuery, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get player teams: %w", err)
	}
	
	player.Teams = teamIDs
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
			club_id = :club_id,
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
func (r *PlayerRepository) Delete(ctx context.Context, id string) error {
	// Start a transaction to delete all player relations
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	
	// Delete from player_teams
	_, err = tx.ExecContext(ctx, `DELETE FROM player_teams WHERE player_id = $1`, id)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete player teams: %w", err)
	}
	
	// Delete from captains
	_, err = tx.ExecContext(ctx, `DELETE FROM captains WHERE player_id = $1`, id)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete captain entries: %w", err)
	}
	
	// Delete from matchup_players
	_, err = tx.ExecContext(ctx, `DELETE FROM matchup_players WHERE player_id = $1`, id)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete matchup players: %w", err)
	}
	
	// Finally delete the player
	_, err = tx.ExecContext(ctx, `DELETE FROM players WHERE id = $1`, id)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete player: %w", err)
	}
	
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	
	return nil
}

// List retrieves all players, optionally filtered by club ID
func (r *PlayerRepository) List(ctx context.Context, clubID *uint) ([]models.Player, error) {
	var players []models.Player
	var query string
	var args []interface{}
	
	if clubID != nil {
		query = `SELECT * FROM players WHERE club_id = $1 ORDER BY last_name, first_name`
		args = append(args, *clubID)
	} else {
		query = `SELECT * FROM players ORDER BY last_name, first_name`
	}
	
	err := r.db.SelectContext(ctx, &players, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list players: %w", err)
	}
	
	// Get team associations for each player
	for i := range players {
		teamsQuery := `
			SELECT team_id FROM player_teams 
			WHERE player_id = $1 AND is_active = true
		`
		
		var teamIDs []uint
		err = r.db.SelectContext(ctx, &teamIDs, teamsQuery, players[i].ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get player teams: %w", err)
		}
		
		players[i].Teams = teamIDs
	}
	
	return players, nil
}

// AddToTeam adds a player to a team for a specific season
func (r *PlayerRepository) AddToTeam(ctx context.Context, playerID string, teamID uint, season string) error {
	query := `
		INSERT INTO player_teams (player_id, team_id, season, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, true, $4, $4)
		ON CONFLICT (player_id, team_id, season) 
		DO UPDATE SET is_active = true, updated_at = $4
	`
	
	now := time.Now()
	_, err := r.db.ExecContext(ctx, query, playerID, teamID, season, now)
	if err != nil {
		return fmt.Errorf("failed to add player to team: %w", err)
	}
	
	return nil
}

// RemoveFromTeam removes a player from a team for a specific season
func (r *PlayerRepository) RemoveFromTeam(ctx context.Context, playerID string, teamID uint, season string) error {
	query := `
		UPDATE player_teams 
		SET is_active = false, updated_at = $4
		WHERE player_id = $1 AND team_id = $2 AND season = $3
	`
	
	now := time.Now()
	_, err := r.db.ExecContext(ctx, query, playerID, teamID, season, now)
	if err != nil {
		return fmt.Errorf("failed to remove player from team: %w", err)
	}
	
	return nil
} 