package repository

import (
	"context"
	"fmt"
	"time"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
)

// PlayerTeamRepository handles database operations for PlayerTeam entities
type PlayerTeamRepository struct {
	db *database.DB
}

// NewPlayerTeamRepository creates a new PlayerTeamRepository
func NewPlayerTeamRepository(db *database.DB) *PlayerTeamRepository {
	return &PlayerTeamRepository{db: db}
}

// Create inserts a new player-team association into the database
func (r *PlayerTeamRepository) Create(ctx context.Context, playerTeam *models.PlayerTeam) error {
	query := `
		INSERT INTO player_teams (
			player_id, team_id, season_id, is_active, created_at, updated_at
		)
		VALUES (
			:player_id, :team_id, :season_id, :is_active, :created_at, :updated_at
		)
		RETURNING id
	`

	now := time.Now()
	playerTeam.CreatedAt = now
	playerTeam.UpdatedAt = now

	stmt, err := r.db.PrepareNamedContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare query: %w", err)
	}
	defer stmt.Close()

	var id uint
	err = stmt.GetContext(ctx, &id, playerTeam)
	if err != nil {
		return fmt.Errorf("failed to insert player-team association: %w", err)
	}

	playerTeam.ID = id
	return nil
}

// GetByID retrieves a player-team association by ID
func (r *PlayerTeamRepository) GetByID(ctx context.Context, id uint) (*models.PlayerTeam, error) {
	var playerTeam models.PlayerTeam
	query := `SELECT * FROM player_teams WHERE id = $1`
	
	err := r.db.GetContext(ctx, &playerTeam, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get player-team association by id: %w", err)
	}
	
	return &playerTeam, nil
}

// Update updates an existing player-team association
func (r *PlayerTeamRepository) Update(ctx context.Context, playerTeam *models.PlayerTeam) error {
	query := `
		UPDATE player_teams
		SET player_id = :player_id,
			team_id = :team_id,
			season_id = :season_id,
			is_active = :is_active,
			updated_at = :updated_at
		WHERE id = :id
	`

	playerTeam.UpdatedAt = time.Now()

	_, err := r.db.NamedExecContext(ctx, query, playerTeam)
	if err != nil {
		return fmt.Errorf("failed to update player-team association: %w", err)
	}

	return nil
}

// Delete removes a player-team association from the database
func (r *PlayerTeamRepository) Delete(ctx context.Context, id uint) error {
	// Check if the player is a captain of this team
	var captainCount int
	err := r.db.GetContext(
		ctx, 
		&captainCount, 
		`SELECT COUNT(*) FROM captains c 
		 JOIN player_teams pt ON c.player_id = pt.player_id AND c.team_id = pt.team_id 
		 WHERE pt.id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("failed to check if player is a captain: %w", err)
	}
	
	if captainCount > 0 {
		return fmt.Errorf("cannot remove player-team association - player is a captain of this team")
	}
	
	// Delete the player-team association
	_, err = r.db.ExecContext(ctx, `DELETE FROM player_teams WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete player-team association: %w", err)
	}
	
	return nil
}

// GetByPlayer retrieves team associations by player ID
func (r *PlayerTeamRepository) GetByPlayer(ctx context.Context, playerID string) ([]models.PlayerTeam, error) {
	var playerTeams []models.PlayerTeam
	query := `SELECT * FROM player_teams WHERE player_id = $1 ORDER BY season_id DESC, team_id`
	
	err := r.db.SelectContext(ctx, &playerTeams, query, playerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get team associations by player: %w", err)
	}
	
	return playerTeams, nil
}

// GetByTeam retrieves player associations by team ID
func (r *PlayerTeamRepository) GetByTeam(ctx context.Context, teamID uint) ([]models.PlayerTeam, error) {
	var playerTeams []models.PlayerTeam
	query := `SELECT * FROM player_teams WHERE team_id = $1 ORDER BY created_at`
	
	err := r.db.SelectContext(ctx, &playerTeams, query, teamID)
	if err != nil {
		return nil, fmt.Errorf("failed to get player associations by team: %w", err)
	}
	
	return playerTeams, nil
}

// GetByPlayerAndSeason retrieves team associations by player ID and season ID
func (r *PlayerTeamRepository) GetByPlayerAndSeason(ctx context.Context, playerID string, seasonID uint) ([]models.PlayerTeam, error) {
	var playerTeams []models.PlayerTeam
	query := `SELECT * FROM player_teams WHERE player_id = $1 AND season_id = $2 ORDER BY team_id`
	
	err := r.db.SelectContext(ctx, &playerTeams, query, playerID, seasonID)
	if err != nil {
		return nil, fmt.Errorf("failed to get team associations by player and season: %w", err)
	}
	
	return playerTeams, nil
}

// GetByTeamAndSeason retrieves player associations by team ID and season ID
func (r *PlayerTeamRepository) GetByTeamAndSeason(ctx context.Context, teamID uint, seasonID uint) ([]models.PlayerTeam, error) {
	var playerTeams []models.PlayerTeam
	query := `SELECT * FROM player_teams WHERE team_id = $1 AND season_id = $2 ORDER BY created_at`
	
	err := r.db.SelectContext(ctx, &playerTeams, query, teamID, seasonID)
	if err != nil {
		return nil, fmt.Errorf("failed to get player associations by team and season: %w", err)
	}
	
	return playerTeams, nil
}

// GetActiveByPlayer retrieves active team associations by player ID
func (r *PlayerTeamRepository) GetActiveByPlayer(ctx context.Context, playerID string) ([]models.PlayerTeam, error) {
	var playerTeams []models.PlayerTeam
	query := `
		SELECT pt.* 
		FROM player_teams pt
		JOIN seasons s ON pt.season_id = s.id
		WHERE pt.player_id = $1 AND pt.is_active = true AND s.is_active = true
		ORDER BY pt.team_id
	`
	
	err := r.db.SelectContext(ctx, &playerTeams, query, playerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active team associations by player: %w", err)
	}
	
	return playerTeams, nil
}

// SetActive sets a player-team association as active or inactive
func (r *PlayerTeamRepository) SetActive(ctx context.Context, id uint, isActive bool) error {
	query := `
		UPDATE player_teams
		SET is_active = $1, updated_at = $2
		WHERE id = $3
	`
	
	_, err := r.db.ExecContext(ctx, query, isActive, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update player-team active status: %w", err)
	}
	
	return nil
}

// GetByUniqueKey retrieves a player-team association by player ID, team ID, and season ID
func (r *PlayerTeamRepository) GetByUniqueKey(ctx context.Context, playerID string, teamID, seasonID uint) (*models.PlayerTeam, error) {
	var playerTeam models.PlayerTeam
	query := `
		SELECT * 
		FROM player_teams 
		WHERE player_id = $1 AND team_id = $2 AND season_id = $3
	`
	
	err := r.db.GetContext(ctx, &playerTeam, query, playerID, teamID, seasonID)
	if err != nil {
		return nil, fmt.Errorf("failed to get player-team association by unique key: %w", err)
	}
	
	return &playerTeam, nil
}

// CountByTeamAndSeason counts the number of players on a team for a specific season
func (r *PlayerTeamRepository) CountByTeamAndSeason(ctx context.Context, teamID, seasonID uint) (int, error) {
	var count int
	query := `
		SELECT COUNT(*) 
		FROM player_teams 
		WHERE team_id = $1 AND season_id = $2
	`
	
	err := r.db.GetContext(ctx, &count, query, teamID, seasonID)
	if err != nil {
		return 0, fmt.Errorf("failed to count players by team and season: %w", err)
	}
	
	return count, nil
} 