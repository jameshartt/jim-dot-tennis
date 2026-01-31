package repository

import (
	"context"
	"database/sql"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
	"time"
)

// TeamRepository defines the interface for team data access
type TeamRepository interface {
	// Basic CRUD operations
	FindAll(ctx context.Context) ([]models.Team, error)
	FindByID(ctx context.Context, id uint) (*models.Team, error)
	Create(ctx context.Context, team *models.Team) error
	Update(ctx context.Context, team *models.Team) error
	Delete(ctx context.Context, id uint) error

	// Team-specific queries
	FindByClub(ctx context.Context, clubID uint) ([]models.Team, error)
	FindByDivision(ctx context.Context, divisionID uint) ([]models.Team, error)
	FindBySeason(ctx context.Context, seasonID uint) ([]models.Team, error)
	FindByClubAndSeason(ctx context.Context, clubID, seasonID uint) ([]models.Team, error)
	FindByDivisionAndSeason(ctx context.Context, divisionID, seasonID uint) ([]models.Team, error)
	FindByName(ctx context.Context, name string) ([]models.Team, error)
	FindByNameLike(ctx context.Context, name string) ([]models.Team, error)

	// Team with relationships
	FindWithPlayers(ctx context.Context, id uint) (*models.Team, error)
	FindWithCaptains(ctx context.Context, id uint) (*models.Team, error)
	FindWithPlayersAndCaptains(ctx context.Context, id uint) (*models.Team, error)

	// Player management
	AddPlayer(ctx context.Context, teamID uint, playerID string, seasonID uint) error
	RemovePlayer(ctx context.Context, teamID uint, playerID string, seasonID uint) error
	FindPlayersInTeam(ctx context.Context, teamID, seasonID uint) ([]models.PlayerTeam, error)
	IsPlayerInTeam(ctx context.Context, teamID uint, playerID string, seasonID uint) (bool, error)

	// Captain management
	AddCaptain(ctx context.Context, teamID uint, playerID string, role models.CaptainRole, seasonID uint) error
	RemoveCaptain(ctx context.Context, teamID uint, playerID string, seasonID uint) error
	FindCaptainsInTeam(ctx context.Context, teamID, seasonID uint) ([]models.Captain, error)
	FindTeamCaptain(ctx context.Context, teamID, seasonID uint) (*models.Captain, error)

	// Statistics
	CountPlayers(ctx context.Context, teamID, seasonID uint) (int, error)
	CountCaptains(ctx context.Context, teamID, seasonID uint) (int, error)

	// Season management
	UpdateDivision(ctx context.Context, teamID uint, newDivisionID uint) error
	FindOrCreateByNameAndClubAndSeason(ctx context.Context, name string, clubID, seasonID, defaultDivisionID uint) (*models.Team, bool, error)
	FindByNameAndSeason(ctx context.Context, name string, seasonID uint) ([]models.Team, error)
}

// teamRepository implements TeamRepository
type teamRepository struct {
	db *database.DB
}

// NewTeamRepository creates a new team repository
func NewTeamRepository(db *database.DB) TeamRepository {
	return &teamRepository{
		db: db,
	}
}

// FindAll retrieves all teams ordered by club, division, and name
func (r *teamRepository) FindAll(ctx context.Context) ([]models.Team, error) {
	var teams []models.Team
	err := r.db.SelectContext(ctx, &teams, `
		SELECT id, name, club_id, division_id, season_id, created_at, updated_at
		FROM teams 
		ORDER BY club_id ASC, division_id ASC, name ASC
	`)
	return teams, err
}

// FindByID retrieves a team by its ID
func (r *teamRepository) FindByID(ctx context.Context, id uint) (*models.Team, error) {
	var team models.Team
	err := r.db.GetContext(ctx, &team, `
		SELECT id, name, club_id, division_id, season_id, created_at, updated_at
		FROM teams 
		WHERE id = ?
	`, id)
	if err != nil {
		return nil, err
	}
	return &team, nil
}

// Create inserts a new team record
func (r *teamRepository) Create(ctx context.Context, team *models.Team) error {
	now := time.Now()
	team.CreatedAt = now
	team.UpdatedAt = now

	result, err := r.db.NamedExecContext(ctx, `
		INSERT INTO teams (name, club_id, division_id, season_id, created_at, updated_at)
		VALUES (:name, :club_id, :division_id, :season_id, :created_at, :updated_at)
	`, team)

	if err != nil {
		return err
	}

	// Get the last inserted ID and set it on the team
	if id, err := result.LastInsertId(); err == nil {
		team.ID = uint(id)
	}

	return nil
}

// Update modifies an existing team record
func (r *teamRepository) Update(ctx context.Context, team *models.Team) error {
	team.UpdatedAt = time.Now()

	_, err := r.db.NamedExecContext(ctx, `
		UPDATE teams 
		SET name = :name, club_id = :club_id, division_id = :division_id, 
		    season_id = :season_id, updated_at = :updated_at
		WHERE id = :id
	`, team)

	return err
}

// Delete removes a team record by ID
func (r *teamRepository) Delete(ctx context.Context, id uint) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM teams WHERE id = ?`, id)
	return err
}

// FindByClub retrieves all teams for a specific club
func (r *teamRepository) FindByClub(ctx context.Context, clubID uint) ([]models.Team, error) {
	var teams []models.Team
	err := r.db.SelectContext(ctx, &teams, `
		SELECT id, name, club_id, division_id, season_id, created_at, updated_at
		FROM teams 
		WHERE club_id = ?
		ORDER BY season_id DESC, division_id ASC, name ASC
	`, clubID)
	return teams, err
}

// FindByDivision retrieves all teams in a specific division
func (r *teamRepository) FindByDivision(ctx context.Context, divisionID uint) ([]models.Team, error) {
	var teams []models.Team
	err := r.db.SelectContext(ctx, &teams, `
		SELECT id, name, club_id, division_id, season_id, created_at, updated_at
		FROM teams 
		WHERE division_id = ?
		ORDER BY name ASC
	`, divisionID)
	return teams, err
}

// FindBySeason retrieves all teams for a specific season
func (r *teamRepository) FindBySeason(ctx context.Context, seasonID uint) ([]models.Team, error) {
	var teams []models.Team
	err := r.db.SelectContext(ctx, &teams, `
		SELECT id, name, club_id, division_id, season_id, created_at, updated_at
		FROM teams 
		WHERE season_id = ?
		ORDER BY club_id ASC, division_id ASC, name ASC
	`, seasonID)
	return teams, err
}

// FindByClubAndSeason retrieves teams for a specific club and season
func (r *teamRepository) FindByClubAndSeason(ctx context.Context, clubID, seasonID uint) ([]models.Team, error) {
	var teams []models.Team
	err := r.db.SelectContext(ctx, &teams, `
		SELECT id, name, club_id, division_id, season_id, created_at, updated_at
		FROM teams 
		WHERE club_id = ? AND season_id = ?
		ORDER BY division_id ASC, name ASC
	`, clubID, seasonID)
	return teams, err
}

// FindByDivisionAndSeason retrieves teams for a specific division and season
func (r *teamRepository) FindByDivisionAndSeason(ctx context.Context, divisionID, seasonID uint) ([]models.Team, error) {
	var teams []models.Team
	err := r.db.SelectContext(ctx, &teams, `
		SELECT id, name, club_id, division_id, season_id, created_at, updated_at
		FROM teams 
		WHERE division_id = ? AND season_id = ?
		ORDER BY name ASC
	`, divisionID, seasonID)
	return teams, err
}

// FindByName retrieves teams with an exact name match
func (r *teamRepository) FindByName(ctx context.Context, name string) ([]models.Team, error) {
	var teams []models.Team
	err := r.db.SelectContext(ctx, &teams, `
		SELECT id, name, club_id, division_id, season_id, created_at, updated_at
		FROM teams 
		WHERE name = ?
		ORDER BY season_id DESC, club_id ASC, division_id ASC
	`, name)
	return teams, err
}

// FindByNameLike retrieves teams with names containing the search string
func (r *teamRepository) FindByNameLike(ctx context.Context, name string) ([]models.Team, error) {
	var teams []models.Team
	searchPattern := "%" + name + "%"
	err := r.db.SelectContext(ctx, &teams, `
		SELECT id, name, club_id, division_id, season_id, created_at, updated_at
		FROM teams 
		WHERE name LIKE ?
		ORDER BY name ASC
	`, searchPattern)
	return teams, err
}

// FindWithPlayers retrieves a team with its associated players
func (r *teamRepository) FindWithPlayers(ctx context.Context, id uint) (*models.Team, error) {
	// First get the team
	team, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Then get associated players through the player_teams join table
	var playerTeams []models.PlayerTeam
	err = r.db.SelectContext(ctx, &playerTeams, `
		SELECT id, player_id, team_id, season_id, is_active, created_at, updated_at
		FROM player_teams
		WHERE team_id = ? AND season_id = ?
		ORDER BY created_at ASC
	`, id, team.SeasonID)

	if err != nil {
		return nil, err
	}

	team.Players = playerTeams
	return team, nil
}

// FindWithCaptains retrieves a team with its associated captains
func (r *teamRepository) FindWithCaptains(ctx context.Context, id uint) (*models.Team, error) {
	// First get the team
	team, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Then get associated captains
	var captains []models.Captain
	err = r.db.SelectContext(ctx, &captains, `
		SELECT id, player_id, team_id, role, season_id, is_active, created_at, updated_at
		FROM captains
		WHERE team_id = ? AND season_id = ?
		ORDER BY role ASC, created_at ASC
	`, id, team.SeasonID)

	if err != nil {
		return nil, err
	}

	team.Captains = captains
	return team, nil
}

// FindWithPlayersAndCaptains retrieves a team with both its players and captains
func (r *teamRepository) FindWithPlayersAndCaptains(ctx context.Context, id uint) (*models.Team, error) {
	// First get the team
	team, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Get associated players
	var playerTeams []models.PlayerTeam
	err = r.db.SelectContext(ctx, &playerTeams, `
		SELECT id, player_id, team_id, season_id, is_active, created_at, updated_at
		FROM player_teams
		WHERE team_id = ? AND season_id = ?
		ORDER BY created_at ASC
	`, id, team.SeasonID)

	if err != nil {
		return nil, err
	}

	// Get associated captains
	var captains []models.Captain
	err = r.db.SelectContext(ctx, &captains, `
		SELECT id, player_id, team_id, role, season_id, is_active, created_at, updated_at
		FROM captains
		WHERE team_id = ? AND season_id = ?
		ORDER BY role ASC, created_at ASC
	`, id, team.SeasonID)

	if err != nil {
		return nil, err
	}

	team.Players = playerTeams
	team.Captains = captains
	return team, nil
}

// AddPlayer adds a player to a team for a specific season
func (r *teamRepository) AddPlayer(ctx context.Context, teamID uint, playerID string, seasonID uint) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO player_teams (player_id, team_id, season_id, is_active, created_at, updated_at)
		VALUES (?, ?, ?, TRUE, ?, ?)
	`, playerID, teamID, seasonID, now, now)
	return err
}

// RemovePlayer removes a player from a team for a specific season
func (r *teamRepository) RemovePlayer(ctx context.Context, teamID uint, playerID string, seasonID uint) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM player_teams 
		WHERE team_id = ? AND player_id = ? AND season_id = ?
	`, teamID, playerID, seasonID)
	return err
}

// FindPlayersInTeam retrieves all players in a team for a specific season
func (r *teamRepository) FindPlayersInTeam(ctx context.Context, teamID, seasonID uint) ([]models.PlayerTeam, error) {
	var playerTeams []models.PlayerTeam
	err := r.db.SelectContext(ctx, &playerTeams, `
		SELECT id, player_id, team_id, season_id, is_active, created_at, updated_at
		FROM player_teams
		WHERE team_id = ? AND season_id = ?
		ORDER BY created_at ASC
	`, teamID, seasonID)
	return playerTeams, err
}

// IsPlayerInTeam checks if a player is in a team for a specific season
func (r *teamRepository) IsPlayerInTeam(ctx context.Context, teamID uint, playerID string, seasonID uint) (bool, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM player_teams 
		WHERE team_id = ? AND player_id = ? AND season_id = ?
	`, teamID, playerID, seasonID)
	return count > 0, err
}

// AddCaptain adds a captain to a team for a specific season
func (r *teamRepository) AddCaptain(ctx context.Context, teamID uint, playerID string, role models.CaptainRole, seasonID uint) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO captains (player_id, team_id, role, season_id, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, TRUE, ?, ?)
	`, playerID, teamID, string(role), seasonID, now, now)
	return err
}

// RemoveCaptain removes a captain from a team for a specific season
func (r *teamRepository) RemoveCaptain(ctx context.Context, teamID uint, playerID string, seasonID uint) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM captains 
		WHERE team_id = ? AND player_id = ? AND season_id = ?
	`, teamID, playerID, seasonID)
	return err
}

// FindCaptainsInTeam retrieves all captains in a team for a specific season
func (r *teamRepository) FindCaptainsInTeam(ctx context.Context, teamID, seasonID uint) ([]models.Captain, error) {
	var captains []models.Captain
	err := r.db.SelectContext(ctx, &captains, `
		SELECT id, player_id, team_id, role, season_id, is_active, created_at, updated_at
		FROM captains
		WHERE team_id = ? AND season_id = ?
		ORDER BY role ASC, created_at ASC
	`, teamID, seasonID)
	return captains, err
}

// FindTeamCaptain retrieves the team captain (not day captain) for a specific season
func (r *teamRepository) FindTeamCaptain(ctx context.Context, teamID, seasonID uint) (*models.Captain, error) {
	var captain models.Captain
	err := r.db.GetContext(ctx, &captain, `
		SELECT id, player_id, team_id, role, season_id, is_active, created_at, updated_at
		FROM captains
		WHERE team_id = ? AND season_id = ? AND role = ?
		LIMIT 1
	`, teamID, seasonID, string(models.TeamCaptain))
	if err != nil {
		return nil, err
	}
	return &captain, nil
}

// CountPlayers returns the number of players in a team for a specific season
func (r *teamRepository) CountPlayers(ctx context.Context, teamID, seasonID uint) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM player_teams 
		WHERE team_id = ? AND season_id = ?
	`, teamID, seasonID)
	return count, err
}

// CountCaptains returns the number of captains in a team for a specific season
func (r *teamRepository) CountCaptains(ctx context.Context, teamID, seasonID uint) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM captains
		WHERE team_id = ? AND season_id = ?
	`, teamID, seasonID)
	return count, err
}

// UpdateDivision changes a team's division (for promotion/demotion)
func (r *teamRepository) UpdateDivision(ctx context.Context, teamID uint, newDivisionID uint) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE teams
		SET division_id = ?, updated_at = ?
		WHERE id = ?
	`, newDivisionID, time.Now(), teamID)
	return err
}

// FindOrCreateByNameAndClubAndSeason finds a team by name, club, and season or creates it if not found
func (r *teamRepository) FindOrCreateByNameAndClubAndSeason(
	ctx context.Context,
	name string,
	clubID, seasonID, defaultDivisionID uint,
) (*models.Team, bool, error) {
	// Try to find existing team
	var team models.Team
	err := r.db.GetContext(ctx, &team, `
		SELECT id, name, club_id, division_id, season_id, created_at, updated_at
		FROM teams
		WHERE name = ? AND club_id = ? AND season_id = ?
	`, name, clubID, seasonID)

	if err == nil {
		// Team found
		return &team, false, nil
	}

	if err != sql.ErrNoRows {
		// Some other error occurred
		return nil, false, err
	}

	// Team not found, create it
	newTeam := &models.Team{
		Name:       name,
		ClubID:     clubID,
		DivisionID: defaultDivisionID,
		SeasonID:   seasonID,
	}

	if err := r.Create(ctx, newTeam); err != nil {
		return nil, false, err
	}

	return newTeam, true, nil
}

// FindByNameAndSeason retrieves teams with an exact name match in a specific season
func (r *teamRepository) FindByNameAndSeason(ctx context.Context, name string, seasonID uint) ([]models.Team, error) {
	var teams []models.Team
	err := r.db.SelectContext(ctx, &teams, `
		SELECT id, name, club_id, division_id, season_id, created_at, updated_at
		FROM teams
		WHERE name = ? AND season_id = ?
		ORDER BY name ASC
	`, name, seasonID)
	return teams, err
}
