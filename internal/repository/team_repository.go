package repository

import (
	"context"
	"fmt"
	"time"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
)

// TeamRepository handles database operations for Team entities
type TeamRepository struct {
	db *database.DB
}

// NewTeamRepository creates a new TeamRepository
func NewTeamRepository(db *database.DB) *TeamRepository {
	return &TeamRepository{db: db}
}

// Create inserts a new team into the database
func (r *TeamRepository) Create(ctx context.Context, team *models.Team) error {
	query := `
		INSERT INTO teams (
			name, club_id, division_id, season_id, created_at, updated_at
		)
		VALUES (
			:name, :club_id, :division_id, :season_id, :created_at, :updated_at
		)
		RETURNING id
	`

	now := time.Now()
	team.CreatedAt = now
	team.UpdatedAt = now

	stmt, err := r.db.PrepareNamedContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare query: %w", err)
	}
	defer stmt.Close()

	var id uint
	err = stmt.GetContext(ctx, &id, team)
	if err != nil {
		return fmt.Errorf("failed to insert team: %w", err)
	}

	team.ID = id
	return nil
}

// GetByID retrieves a team by ID
func (r *TeamRepository) GetByID(ctx context.Context, id uint) (*models.Team, error) {
	var team models.Team
	query := `SELECT * FROM teams WHERE id = $1`
	
	err := r.db.GetContext(ctx, &team, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get team by id: %w", err)
	}
	
	return &team, nil
}

// Update updates an existing team
func (r *TeamRepository) Update(ctx context.Context, team *models.Team) error {
	query := `
		UPDATE teams
		SET name = :name,
			club_id = :club_id,
			division_id = :division_id,
			season_id = :season_id,
			updated_at = :updated_at
		WHERE id = :id
	`

	team.UpdatedAt = time.Now()

	_, err := r.db.NamedExecContext(ctx, query, team)
	if err != nil {
		return fmt.Errorf("failed to update team: %w", err)
	}

	return nil
}

// Delete removes a team from the database
func (r *TeamRepository) Delete(ctx context.Context, id uint) error {
	// Check if team has players
	var playerCount int
	err := r.db.GetContext(ctx, &playerCount, `SELECT COUNT(*) FROM player_teams WHERE team_id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to check team players: %w", err)
	}
	
	if playerCount > 0 {
		return fmt.Errorf("cannot delete team with %d player associations - please remove them first", playerCount)
	}
	
	// Check if team has captains
	var captainCount int
	err = r.db.GetContext(ctx, &captainCount, `SELECT COUNT(*) FROM captains WHERE team_id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to check team captains: %w", err)
	}
	
	if captainCount > 0 {
		return fmt.Errorf("cannot delete team with %d captains - please remove them first", captainCount)
	}
	
	// Check if team has fixtures (either home or away)
	var fixtureCount int
	err = r.db.GetContext(
		ctx, 
		&fixtureCount, 
		`SELECT COUNT(*) FROM fixtures WHERE home_team_id = $1 OR away_team_id = $1`, 
		id,
	)
	if err != nil {
		return fmt.Errorf("failed to check team fixtures: %w", err)
	}
	
	if fixtureCount > 0 {
		return fmt.Errorf("cannot delete team with %d fixtures - please remove those first", fixtureCount)
	}
	
	// Delete the team
	_, err = r.db.ExecContext(ctx, `DELETE FROM teams WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete team: %w", err)
	}
	
	return nil
}

// List retrieves all teams
func (r *TeamRepository) List(ctx context.Context) ([]models.Team, error) {
	var teams []models.Team
	query := `SELECT * FROM teams ORDER BY name`
	
	err := r.db.SelectContext(ctx, &teams, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list teams: %w", err)
	}
	
	return teams, nil
}

// GetByClub retrieves teams by club ID
func (r *TeamRepository) GetByClub(ctx context.Context, clubID uint) ([]models.Team, error) {
	var teams []models.Team
	query := `SELECT * FROM teams WHERE club_id = $1 ORDER BY name`
	
	err := r.db.SelectContext(ctx, &teams, query, clubID)
	if err != nil {
		return nil, fmt.Errorf("failed to get teams by club: %w", err)
	}
	
	return teams, nil
}

// GetByDivision retrieves teams by division ID
func (r *TeamRepository) GetByDivision(ctx context.Context, divisionID uint) ([]models.Team, error) {
	var teams []models.Team
	query := `SELECT * FROM teams WHERE division_id = $1 ORDER BY name`
	
	err := r.db.SelectContext(ctx, &teams, query, divisionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get teams by division: %w", err)
	}
	
	return teams, nil
}

// GetByDivisionAndSeason retrieves teams by division ID and season ID
func (r *TeamRepository) GetByDivisionAndSeason(ctx context.Context, divisionID, seasonID uint) ([]models.Team, error) {
	var teams []models.Team
	query := `SELECT * FROM teams WHERE division_id = $1 AND season_id = $2 ORDER BY name`
	
	err := r.db.SelectContext(ctx, &teams, query, divisionID, seasonID)
	if err != nil {
		return nil, fmt.Errorf("failed to get teams by division and season: %w", err)
	}
	
	return teams, nil
}

// GetByClubAndSeason retrieves teams by club ID and season ID
func (r *TeamRepository) GetByClubAndSeason(ctx context.Context, clubID, seasonID uint) ([]models.Team, error) {
	var teams []models.Team
	query := `SELECT * FROM teams WHERE club_id = $1 AND season_id = $2 ORDER BY name`
	
	err := r.db.SelectContext(ctx, &teams, query, clubID, seasonID)
	if err != nil {
		return nil, fmt.Errorf("failed to get teams by club and season: %w", err)
	}
	
	return teams, nil
}

// GetWithPlayers retrieves a team with its players for a specific season
func (r *TeamRepository) GetWithPlayers(ctx context.Context, id uint, seasonID uint) (*models.Team, error) {
	team, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	
	// Get player-team relationships for this team and season
	playersQuery := `
		SELECT pt.* 
		FROM player_teams pt
		WHERE pt.team_id = $1 AND pt.season_id = $2
		ORDER BY pt.created_at
	`
	
	var playerTeams []models.PlayerTeam
	err = r.db.SelectContext(ctx, &playerTeams, playersQuery, id, seasonID)
	if err != nil {
		return nil, fmt.Errorf("failed to get team players: %w", err)
	}
	
	team.Players = playerTeams
	return team, nil
}

// GetWithCaptains retrieves a team with its captains for a specific season
func (r *TeamRepository) GetWithCaptains(ctx context.Context, id uint, seasonID uint) (*models.Team, error) {
	team, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	
	// Get captains for this team and season
	captainsQuery := `
		SELECT c.* 
		FROM captains c
		WHERE c.team_id = $1 AND c.season_id = $2
		ORDER BY c.role
	`
	
	var captains []models.Captain
	err = r.db.SelectContext(ctx, &captains, captainsQuery, id, seasonID)
	if err != nil {
		return nil, fmt.Errorf("failed to get team captains: %w", err)
	}
	
	team.Captains = captains
	return team, nil
}

// AddPlayer adds a player to a team for a specific season
func (r *TeamRepository) AddPlayer(ctx context.Context, playerID string, teamID uint, seasonID uint) error {
	// Check if this player-team association already exists for this season
	var count int
	err := r.db.GetContext(
		ctx,
		&count,
		`SELECT COUNT(*) FROM player_teams WHERE player_id = $1 AND team_id = $2 AND season_id = $3`,
		playerID, teamID, seasonID,
	)
	if err != nil {
		return fmt.Errorf("failed to check player-team association: %w", err)
	}
	
	if count > 0 {
		return fmt.Errorf("player is already on this team for the specified season")
	}
	
	// Add the player to the team
	query := `
		INSERT INTO player_teams (
			player_id, team_id, season_id, is_active, created_at, updated_at
		)
		VALUES ($1, $2, $3, true, $4, $4)
	`
	
	now := time.Now()
	_, err = r.db.ExecContext(ctx, query, playerID, teamID, seasonID, now)
	if err != nil {
		return fmt.Errorf("failed to add player to team: %w", err)
	}
	
	return nil
}

// RemovePlayer removes a player from a team for a specific season
func (r *TeamRepository) RemovePlayer(ctx context.Context, playerID string, teamID uint, seasonID uint) error {
	query := `DELETE FROM player_teams WHERE player_id = $1 AND team_id = $2 AND season_id = $3`
	
	result, err := r.db.ExecContext(ctx, query, playerID, teamID, seasonID)
	if err != nil {
		return fmt.Errorf("failed to remove player from team: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("player was not on this team for the specified season")
	}
	
	return nil
}

// AddCaptain adds a captain to a team for a specific season
func (r *TeamRepository) AddCaptain(ctx context.Context, playerID string, teamID uint, role models.CaptainRole, seasonID uint) error {
	// Check if this player is already a captain for this team and season
	var count int
	err := r.db.GetContext(
		ctx,
		&count,
		`SELECT COUNT(*) FROM captains WHERE player_id = $1 AND team_id = $2 AND season_id = $3 AND role = $4`,
		playerID, teamID, seasonID, role,
	)
	if err != nil {
		return fmt.Errorf("failed to check captain association: %w", err)
	}
	
	if count > 0 {
		return fmt.Errorf("player is already a %s captain for this team in the specified season", role)
	}
	
	// Add the captain
	query := `
		INSERT INTO captains (
			player_id, team_id, role, season_id, is_active, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, true, $5, $5)
	`
	
	now := time.Now()
	_, err = r.db.ExecContext(ctx, query, playerID, teamID, role, seasonID, now)
	if err != nil {
		return fmt.Errorf("failed to add captain to team: %w", err)
	}
	
	return nil
}

// RemoveCaptain removes a captain from a team for a specific season
func (r *TeamRepository) RemoveCaptain(ctx context.Context, playerID string, teamID uint, role models.CaptainRole, seasonID uint) error {
	query := `DELETE FROM captains WHERE player_id = $1 AND team_id = $2 AND role = $3 AND season_id = $4`
	
	result, err := r.db.ExecContext(ctx, query, playerID, teamID, role, seasonID)
	if err != nil {
		return fmt.Errorf("failed to remove captain from team: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("player was not a %s captain for this team in the specified season", role)
	}
	
	return nil
} 