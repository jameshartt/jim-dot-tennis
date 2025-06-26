package repository

import (
	"context"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
	"time"
)

// ClubRepository defines the interface for club data access
type ClubRepository interface {
	// Basic CRUD operations
	FindAll(ctx context.Context) ([]models.Club, error)
	FindByID(ctx context.Context, id uint) (*models.Club, error)
	Create(ctx context.Context, club *models.Club) error
	Update(ctx context.Context, club *models.Club) error
	Delete(ctx context.Context, id uint) error

	// Club-specific queries
	FindByName(ctx context.Context, name string) ([]models.Club, error)
	FindByNameLike(ctx context.Context, name string) ([]models.Club, error)
	FindWithPlayers(ctx context.Context, id uint) (*models.Club, error)
	FindWithTeams(ctx context.Context, id uint) (*models.Club, error)
	FindWithPlayersAndTeams(ctx context.Context, id uint) (*models.Club, error)
	FindByPlayerID(ctx context.Context, playerID string) (*models.Club, error)
	CountPlayers(ctx context.Context, id uint) (int, error)
	CountTeams(ctx context.Context, id uint) (int, error)

	// New queries
	GetPlayersByClub(ctx context.Context, clubID uint) ([]models.Player, error)
	GetAllPlayersWithClubs(ctx context.Context) ([]models.Player, error)
}

// clubRepository implements ClubRepository
type clubRepository struct {
	db *database.DB
}

// NewClubRepository creates a new club repository
func NewClubRepository(db *database.DB) ClubRepository {
	return &clubRepository{
		db: db,
	}
}

// FindAll retrieves all clubs ordered by name
func (r *clubRepository) FindAll(ctx context.Context) ([]models.Club, error) {
	var clubs []models.Club
	err := r.db.SelectContext(ctx, &clubs, `
		SELECT id, name, address, website, phone_number, created_at, updated_at
		FROM clubs 
		ORDER BY name ASC
	`)
	return clubs, err
}

// FindByID retrieves a club by its ID
func (r *clubRepository) FindByID(ctx context.Context, id uint) (*models.Club, error) {
	var club models.Club
	err := r.db.GetContext(ctx, &club, `
		SELECT id, name, address, website, phone_number, created_at, updated_at
		FROM clubs 
		WHERE id = ?
	`, id)
	if err != nil {
		return nil, err
	}
	return &club, nil
}

// Create inserts a new club record
func (r *clubRepository) Create(ctx context.Context, club *models.Club) error {
	now := time.Now()
	club.CreatedAt = now
	club.UpdatedAt = now

	result, err := r.db.NamedExecContext(ctx, `
		INSERT INTO clubs (name, address, website, phone_number, created_at, updated_at)
		VALUES (:name, :address, :website, :phone_number, :created_at, :updated_at)
	`, club)

	if err != nil {
		return err
	}

	// Get the last inserted ID and set it on the club
	if id, err := result.LastInsertId(); err == nil {
		club.ID = uint(id)
	}

	return nil
}

// Update modifies an existing club record
func (r *clubRepository) Update(ctx context.Context, club *models.Club) error {
	club.UpdatedAt = time.Now()

	_, err := r.db.NamedExecContext(ctx, `
		UPDATE clubs 
		SET name = :name, address = :address, website = :website, 
		    phone_number = :phone_number, updated_at = :updated_at
		WHERE id = :id
	`, club)

	return err
}

// Delete removes a club record by ID
func (r *clubRepository) Delete(ctx context.Context, id uint) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM clubs WHERE id = ?`, id)
	return err
}

// FindByName retrieves clubs with an exact name match
func (r *clubRepository) FindByName(ctx context.Context, name string) ([]models.Club, error) {
	var clubs []models.Club
	err := r.db.SelectContext(ctx, &clubs, `
		SELECT id, name, address, website, phone_number, created_at, updated_at
		FROM clubs 
		WHERE name = ?
		ORDER BY name ASC
	`, name)
	return clubs, err
}

// FindByNameLike retrieves clubs with names containing the search string
func (r *clubRepository) FindByNameLike(ctx context.Context, name string) ([]models.Club, error) {
	var clubs []models.Club
	searchPattern := "%" + name + "%"
	err := r.db.SelectContext(ctx, &clubs, `
		SELECT id, name, address, website, phone_number, created_at, updated_at
		FROM clubs 
		WHERE name LIKE ?
		ORDER BY name ASC
	`, searchPattern)
	return clubs, err
}

// FindWithPlayers retrieves a club with its associated players
func (r *clubRepository) FindWithPlayers(ctx context.Context, id uint) (*models.Club, error) {
	// First get the club
	club, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Then get associated players
	var players []models.Player
	err = r.db.SelectContext(ctx, &players, `
		SELECT id, first_name, last_name, club_id, created_at, updated_at
		FROM players
		WHERE club_id = ?
		ORDER BY last_name ASC, first_name ASC
	`, id)

	if err != nil {
		return nil, err
	}

	club.Players = players
	return club, nil
}

// FindWithTeams retrieves a club with its associated teams
func (r *clubRepository) FindWithTeams(ctx context.Context, id uint) (*models.Club, error) {
	// First get the club
	club, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Then get associated teams
	var teams []models.Team
	err = r.db.SelectContext(ctx, &teams, `
		SELECT id, name, club_id, division_id, season_id, created_at, updated_at
		FROM teams
		WHERE club_id = ?
		ORDER BY name ASC
	`, id)

	if err != nil {
		return nil, err
	}

	club.Teams = teams
	return club, nil
}

// FindWithPlayersAndTeams retrieves a club with both its players and teams
func (r *clubRepository) FindWithPlayersAndTeams(ctx context.Context, id uint) (*models.Club, error) {
	// First get the club
	club, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Get associated players
	var players []models.Player
	err = r.db.SelectContext(ctx, &players, `
		SELECT id, first_name, last_name, club_id, created_at, updated_at
		FROM players
		WHERE club_id = ?
		ORDER BY last_name ASC, first_name ASC
	`, id)

	if err != nil {
		return nil, err
	}

	// Get associated teams
	var teams []models.Team
	err = r.db.SelectContext(ctx, &teams, `
		SELECT id, name, club_id, division_id, season_id, created_at, updated_at
		FROM teams
		WHERE club_id = ?
		ORDER BY name ASC
	`, id)

	if err != nil {
		return nil, err
	}

	club.Players = players
	club.Teams = teams
	return club, nil
}

// FindByPlayerID retrieves the club that a specific player belongs to
func (r *clubRepository) FindByPlayerID(ctx context.Context, playerID string) (*models.Club, error) {
	var club models.Club
	err := r.db.GetContext(ctx, &club, `
		SELECT c.id, c.name, c.address, c.website, c.phone_number, c.created_at, c.updated_at
		FROM clubs c
		INNER JOIN players p ON c.id = p.club_id
		WHERE p.id = ?
	`, playerID)
	if err != nil {
		return nil, err
	}
	return &club, nil
}

// CountPlayers returns the number of players in a club
func (r *clubRepository) CountPlayers(ctx context.Context, id uint) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM players WHERE club_id = ?
	`, id)
	return count, err
}

// CountTeams returns the number of teams in a club
func (r *clubRepository) CountTeams(ctx context.Context, id uint) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM teams WHERE club_id = ?
	`, id)
	return count, err
}

// GetPlayersByClub retrieves all players for a specific club
func (r *clubRepository) GetPlayersByClub(ctx context.Context, clubID uint) ([]models.Player, error) {
	var players []models.Player
	err := r.db.SelectContext(ctx, &players, `
		SELECT id, first_name, last_name, club_id, created_at, updated_at
		FROM players 
		WHERE club_id = ?
		ORDER BY last_name ASC, first_name ASC
	`, clubID)
	return players, err
}

// GetAllPlayersWithClubs retrieves all players with their club information
func (r *clubRepository) GetAllPlayersWithClubs(ctx context.Context) ([]models.Player, error) {
	var players []models.Player
	err := r.db.SelectContext(ctx, &players, `
		SELECT id, first_name, last_name, club_id, created_at, updated_at
		FROM players 
		ORDER BY last_name ASC, first_name ASC
	`)
	return players, err
}
