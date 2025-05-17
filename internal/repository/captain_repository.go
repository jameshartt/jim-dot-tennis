package repository

import (
	"context"
	"fmt"
	"time"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
)

// CaptainRepository handles database operations for Captain entities
type CaptainRepository struct {
	db *database.DB
}

// NewCaptainRepository creates a new CaptainRepository
func NewCaptainRepository(db *database.DB) *CaptainRepository {
	return &CaptainRepository{db: db}
}

// Create inserts a new captain into the database
func (r *CaptainRepository) Create(ctx context.Context, captain *models.Captain) error {
	query := `
		INSERT INTO captains (
			player_id, team_id, role, season_id, is_active, created_at, updated_at
		)
		VALUES (
			:player_id, :team_id, :role, :season_id, :is_active, :created_at, :updated_at
		)
		RETURNING id
	`

	now := time.Now()
	captain.CreatedAt = now
	captain.UpdatedAt = now

	stmt, err := r.db.PrepareNamedContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare query: %w", err)
	}
	defer stmt.Close()

	var id uint
	err = stmt.GetContext(ctx, &id, captain)
	if err != nil {
		return fmt.Errorf("failed to insert captain: %w", err)
	}

	captain.ID = id
	return nil
}

// GetByID retrieves a captain by ID
func (r *CaptainRepository) GetByID(ctx context.Context, id uint) (*models.Captain, error) {
	var captain models.Captain
	query := `SELECT * FROM captains WHERE id = $1`
	
	err := r.db.GetContext(ctx, &captain, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get captain by id: %w", err)
	}
	
	return &captain, nil
}

// Update updates an existing captain
func (r *CaptainRepository) Update(ctx context.Context, captain *models.Captain) error {
	query := `
		UPDATE captains
		SET player_id = :player_id,
			team_id = :team_id,
			role = :role,
			season_id = :season_id,
			is_active = :is_active,
			updated_at = :updated_at
		WHERE id = :id
	`

	captain.UpdatedAt = time.Now()

	_, err := r.db.NamedExecContext(ctx, query, captain)
	if err != nil {
		return fmt.Errorf("failed to update captain: %w", err)
	}

	return nil
}

// Delete removes a captain from the database
func (r *CaptainRepository) Delete(ctx context.Context, id uint) error {
	// Check if captain is a day captain for any fixtures
	var fixtureCount int
	err := r.db.GetContext(
		ctx, 
		&fixtureCount,
		`SELECT COUNT(*) FROM fixtures f 
		 JOIN captains c ON f.day_captain_id = c.player_id 
		 WHERE c.id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("failed to check captain fixtures: %w", err)
	}
	
	if fixtureCount > 0 {
		return fmt.Errorf("cannot delete captain associated with %d fixtures - please reassign those first", fixtureCount)
	}
	
	// Delete the captain
	_, err = r.db.ExecContext(ctx, `DELETE FROM captains WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete captain: %w", err)
	}
	
	return nil
}

// GetByTeam retrieves captains by team ID
func (r *CaptainRepository) GetByTeam(ctx context.Context, teamID uint) ([]models.Captain, error) {
	var captains []models.Captain
	query := `SELECT * FROM captains WHERE team_id = $1 ORDER BY role`
	
	err := r.db.SelectContext(ctx, &captains, query, teamID)
	if err != nil {
		return nil, fmt.Errorf("failed to get captains by team: %w", err)
	}
	
	return captains, nil
}

// GetByTeamAndSeason retrieves captains by team ID and season ID
func (r *CaptainRepository) GetByTeamAndSeason(ctx context.Context, teamID, seasonID uint) ([]models.Captain, error) {
	var captains []models.Captain
	query := `SELECT * FROM captains WHERE team_id = $1 AND season_id = $2 ORDER BY role`
	
	err := r.db.SelectContext(ctx, &captains, query, teamID, seasonID)
	if err != nil {
		return nil, fmt.Errorf("failed to get captains by team and season: %w", err)
	}
	
	return captains, nil
}

// GetByPlayer retrieves captaincies by player ID
func (r *CaptainRepository) GetByPlayer(ctx context.Context, playerID string) ([]models.Captain, error) {
	var captains []models.Captain
	query := `SELECT * FROM captains WHERE player_id = $1 ORDER BY season_id DESC, team_id`
	
	err := r.db.SelectContext(ctx, &captains, query, playerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get captaincies by player: %w", err)
	}
	
	return captains, nil
}

// GetActiveByPlayer retrieves active captaincies by player ID
func (r *CaptainRepository) GetActiveByPlayer(ctx context.Context, playerID string) ([]models.Captain, error) {
	var captains []models.Captain
	query := `
		SELECT c.* 
		FROM captains c
		JOIN seasons s ON c.season_id = s.id
		WHERE c.player_id = $1 AND c.is_active = true AND s.is_active = true
		ORDER BY c.team_id, c.role
	`
	
	err := r.db.SelectContext(ctx, &captains, query, playerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active captaincies by player: %w", err)
	}
	
	return captains, nil
}

// GetByRole retrieves captains by role
func (r *CaptainRepository) GetByRole(ctx context.Context, role models.CaptainRole) ([]models.Captain, error) {
	var captains []models.Captain
	query := `SELECT * FROM captains WHERE role = $1 ORDER BY team_id`
	
	err := r.db.SelectContext(ctx, &captains, query, role)
	if err != nil {
		return nil, fmt.Errorf("failed to get captains by role: %w", err)
	}
	
	return captains, nil
}

// SetActive sets a captain as active or inactive
func (r *CaptainRepository) SetActive(ctx context.Context, id uint, isActive bool) error {
	query := `
		UPDATE captains
		SET is_active = $1, updated_at = $2
		WHERE id = $3
	`
	
	_, err := r.db.ExecContext(ctx, query, isActive, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update captain active status: %w", err)
	}
	
	return nil
}

// SetTeamCaptain assigns a player as team captain
func (r *CaptainRepository) SetTeamCaptain(ctx context.Context, playerID string, teamID, seasonID uint) error {
	// Check if player is on the team for this season
	var playerTeamCount int
	err := r.db.GetContext(
		ctx,
		&playerTeamCount,
		`SELECT COUNT(*) FROM player_teams 
		 WHERE player_id = $1 AND team_id = $2 AND season_id = $3`,
		playerID, teamID, seasonID,
	)
	if err != nil {
		return fmt.Errorf("failed to check player team membership: %w", err)
	}
	
	if playerTeamCount == 0 {
		return fmt.Errorf("player must be a member of the team to be assigned as captain")
	}
	
	// Check if team already has a team captain for this season
	var existingCaptainID uint
	err = r.db.GetContext(
		ctx,
		&existingCaptainID,
		`SELECT id FROM captains 
		 WHERE team_id = $1 AND season_id = $2 AND role = $3`,
		teamID, seasonID, models.TeamCaptain,
	)
	
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()
	
	now := time.Now()
	
	// If there's an existing captain, update them
	if err == nil {
		// Existing captain found
		_, err = tx.ExecContext(
			ctx,
			`UPDATE captains SET is_active = false, updated_at = $1 WHERE id = $2`,
			now, existingCaptainID,
		)
		if err != nil {
			return fmt.Errorf("failed to deactivate existing captain: %w", err)
		}
	}
	
	// Insert the new captain
	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO captains 
		(player_id, team_id, role, season_id, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, true, $5, $5)`,
		playerID, teamID, models.TeamCaptain, seasonID, now,
	)
	if err != nil {
		return fmt.Errorf("failed to insert new captain: %w", err)
	}
	
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	
	return nil
} 