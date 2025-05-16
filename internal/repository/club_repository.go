package repository

import (
	"context"
	"fmt"
	"time"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
)

// ClubRepository handles database operations for Club entities
type ClubRepository struct {
	db *database.DB
}

// NewClubRepository creates a new ClubRepository
func NewClubRepository(db *database.DB) *ClubRepository {
	return &ClubRepository{db: db}
}

// Create inserts a new club into the database
func (r *ClubRepository) Create(ctx context.Context, club *models.Club) error {
	query := `
		INSERT INTO clubs (name, address, website, phone_number, created_at, updated_at)
		VALUES (:name, :address, :website, :phone_number, :created_at, :updated_at)
		RETURNING id
	`

	now := time.Now()
	club.CreatedAt = now
	club.UpdatedAt = now

	stmt, err := r.db.PrepareNamedContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare query: %w", err)
	}
	defer stmt.Close()

	var id uint
	err = stmt.GetContext(ctx, &id, club)
	if err != nil {
		return fmt.Errorf("failed to insert club: %w", err)
	}

	club.ID = id
	return nil
}

// GetByID retrieves a club by ID
func (r *ClubRepository) GetByID(ctx context.Context, id uint) (*models.Club, error) {
	var club models.Club
	query := `SELECT * FROM clubs WHERE id = $1`
	
	err := r.db.GetContext(ctx, &club, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get club by id: %w", err)
	}
	
	return &club, nil
}

// GetWithDetails retrieves a club by ID with its players and teams
func (r *ClubRepository) GetWithDetails(ctx context.Context, id uint) (*models.Club, error) {
	club, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	
	// Get players for this club
	playersQuery := `SELECT * FROM players WHERE club_id = $1 ORDER BY last_name, first_name`
	var players []models.Player
	
	err = r.db.SelectContext(ctx, &players, playersQuery, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get club players: %w", err)
	}
	
	// Get teams for this club
	teamsQuery := `SELECT * FROM teams WHERE club_id = $1 ORDER BY name`
	var teams []models.Team
	
	err = r.db.SelectContext(ctx, &teams, teamsQuery, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get club teams: %w", err)
	}
	
	club.Players = players
	club.Teams = teams
	
	return club, nil
}

// Update updates an existing club
func (r *ClubRepository) Update(ctx context.Context, club *models.Club) error {
	query := `
		UPDATE clubs
		SET name = :name,
			address = :address,
			website = :website,
			phone_number = :phone_number,
			updated_at = :updated_at
		WHERE id = :id
	`

	club.UpdatedAt = time.Now()

	_, err := r.db.NamedExecContext(ctx, query, club)
	if err != nil {
		return fmt.Errorf("failed to update club: %w", err)
	}

	return nil
}

// Delete removes a club from the database if it has no players or teams
func (r *ClubRepository) Delete(ctx context.Context, id uint) error {
	// Check if club has players
	var playerCount int
	err := r.db.GetContext(ctx, &playerCount, `SELECT COUNT(*) FROM players WHERE club_id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to check club players: %w", err)
	}
	
	if playerCount > 0 {
		return fmt.Errorf("cannot delete club with %d players - please transfer or delete them first", playerCount)
	}
	
	// Check if club has teams
	var teamCount int
	err = r.db.GetContext(ctx, &teamCount, `SELECT COUNT(*) FROM teams WHERE club_id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to check club teams: %w", err)
	}
	
	if teamCount > 0 {
		return fmt.Errorf("cannot delete club with %d teams - please transfer or delete them first", teamCount)
	}
	
	// Delete the club
	_, err = r.db.ExecContext(ctx, `DELETE FROM clubs WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete club: %w", err)
	}
	
	return nil
}

// List retrieves all clubs
func (r *ClubRepository) List(ctx context.Context) ([]models.Club, error) {
	var clubs []models.Club
	query := `SELECT * FROM clubs ORDER BY name`
	
	err := r.db.SelectContext(ctx, &clubs, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list clubs: %w", err)
	}
	
	return clubs, nil
} 