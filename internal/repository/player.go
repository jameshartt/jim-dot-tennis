package repository

import (
	"context"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
	"time"
)

// PlayerRepository defines the interface for player data access
type PlayerRepository interface {
	// Basic CRUD operations
	FindAll(ctx context.Context) ([]models.Player, error)
	FindByID(ctx context.Context, id string) (*models.Player, error)
	Create(ctx context.Context, player *models.Player) error
	Update(ctx context.Context, player *models.Player) error
	Delete(ctx context.Context, id string) error

	// Player-specific queries
	FindByClub(ctx context.Context, clubID uint) ([]models.Player, error)
	FindByEmail(ctx context.Context, email string) (*models.Player, error)
	FindByName(ctx context.Context, firstName, lastName string) ([]models.Player, error)
	FindByNameLike(ctx context.Context, name string) ([]models.Player, error)
	FindByPhone(ctx context.Context, phone string) (*models.Player, error)

	// Team relationships
	FindByTeam(ctx context.Context, teamID uint, seasonID uint) ([]models.Player, error)
	FindTeamsForPlayer(ctx context.Context, playerID string, seasonID uint) ([]models.PlayerTeam, error)
	FindAllTeamsForPlayer(ctx context.Context, playerID string) ([]models.PlayerTeam, error)
	IsPlayerInTeam(ctx context.Context, playerID string, teamID uint, seasonID uint) (bool, error)

	// Captain relationships
	FindCaptains(ctx context.Context) ([]models.Player, error)
	FindCaptainsByRole(ctx context.Context, role models.CaptainRole) ([]models.Player, error)
	FindCaptainsByTeam(ctx context.Context, teamID uint, seasonID uint) ([]models.Player, error)
	FindCaptainsByClub(ctx context.Context, clubID uint, seasonID uint) ([]models.Player, error)
	IsPlayerCaptain(ctx context.Context, playerID string, teamID uint, seasonID uint) (bool, error)
	FindCaptainRoles(ctx context.Context, playerID string, seasonID uint) ([]models.Captain, error)

	// Day captain specific
	FindDayCaptains(ctx context.Context) ([]models.Player, error)
	FindFixturesAsDayCaptain(ctx context.Context, playerID string) ([]models.Fixture, error)

	// Search and filtering
	SearchPlayers(ctx context.Context, query string) ([]models.Player, error)
	FindActivePlayersInSeason(ctx context.Context, seasonID uint) ([]models.Player, error)
	FindInactivePlayersInSeason(ctx context.Context, seasonID uint) ([]models.Player, error)

	// Fantasy match queries
	FindByFantasyMatchID(ctx context.Context, fantasyMatchID uint) (*models.Player, error)

	// Statistics
	CountByClub(ctx context.Context, clubID uint) (int, error)
	CountCaptains(ctx context.Context) (int, error)
	CountActiveInSeason(ctx context.Context, seasonID uint) (int, error)
	CountAll(ctx context.Context) (int, error)
}

// playerRepository implements PlayerRepository
type playerRepository struct {
	db *database.DB
}

// NewPlayerRepository creates a new player repository
func NewPlayerRepository(db *database.DB) PlayerRepository {
	return &playerRepository{
		db: db,
	}
}

// FindAll retrieves all players ordered by last name, first name
func (r *playerRepository) FindAll(ctx context.Context) ([]models.Player, error) {
	var players []models.Player
	err := r.db.SelectContext(ctx, &players, `
		SELECT id, first_name, last_name, email, phone, club_id, fantasy_match_id, created_at, updated_at
		FROM players 
		ORDER BY last_name ASC, first_name ASC
	`)
	return players, err
}

// FindByID retrieves a player by their ID (UUID)
func (r *playerRepository) FindByID(ctx context.Context, id string) (*models.Player, error) {
	var player models.Player
	err := r.db.GetContext(ctx, &player, `
		SELECT id, first_name, last_name, email, phone, club_id, fantasy_match_id, created_at, updated_at
		FROM players 
		WHERE id = ?
	`, id)
	if err != nil {
		return nil, err
	}
	return &player, nil
}

// Create inserts a new player record
func (r *playerRepository) Create(ctx context.Context, player *models.Player) error {
	now := time.Now()
	player.CreatedAt = now
	player.UpdatedAt = now

	_, err := r.db.NamedExecContext(ctx, `
		INSERT INTO players (id, first_name, last_name, email, phone, club_id, created_at, updated_at)
		VALUES (:id, :first_name, :last_name, :email, :phone, :club_id, :created_at, :updated_at)
	`, player)

	return err
}

// Update modifies an existing player record
func (r *playerRepository) Update(ctx context.Context, player *models.Player) error {
	player.UpdatedAt = time.Now()

	_, err := r.db.NamedExecContext(ctx, `
		UPDATE players 
		SET first_name = :first_name, last_name = :last_name, email = :email, 
		    phone = :phone, club_id = :club_id, fantasy_match_id = :fantasy_match_id, updated_at = :updated_at
		WHERE id = :id
	`, player)

	return err
}

// Delete removes a player record by ID
func (r *playerRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM players WHERE id = ?`, id)
	return err
}

// FindByClub retrieves all players for a specific club
func (r *playerRepository) FindByClub(ctx context.Context, clubID uint) ([]models.Player, error) {
	var players []models.Player
	err := r.db.SelectContext(ctx, &players, `
		SELECT id, first_name, last_name, email, phone, club_id, created_at, updated_at
		FROM players 
		WHERE club_id = ?
		ORDER BY last_name ASC, first_name ASC
	`, clubID)
	return players, err
}

// FindByEmail retrieves a player by their email address
func (r *playerRepository) FindByEmail(ctx context.Context, email string) (*models.Player, error) {
	var player models.Player
	err := r.db.GetContext(ctx, &player, `
		SELECT id, first_name, last_name, email, phone, club_id, created_at, updated_at
		FROM players 
		WHERE email = ?
	`, email)
	if err != nil {
		return nil, err
	}
	return &player, nil
}

// FindByName retrieves players with exact first and last name match
func (r *playerRepository) FindByName(ctx context.Context, firstName, lastName string) ([]models.Player, error) {
	var players []models.Player
	err := r.db.SelectContext(ctx, &players, `
		SELECT id, first_name, last_name, email, phone, club_id, created_at, updated_at
		FROM players 
		WHERE first_name = ? AND last_name = ?
		ORDER BY created_at ASC
	`, firstName, lastName)
	return players, err
}

// FindByNameLike retrieves players with names containing the search string
func (r *playerRepository) FindByNameLike(ctx context.Context, name string) ([]models.Player, error) {
	var players []models.Player
	searchPattern := "%" + name + "%"
	err := r.db.SelectContext(ctx, &players, `
		SELECT id, first_name, last_name, email, phone, club_id, created_at, updated_at
		FROM players 
		WHERE first_name LIKE ? OR last_name LIKE ? OR (first_name || ' ' || last_name) LIKE ?
		ORDER BY last_name ASC, first_name ASC
	`, searchPattern, searchPattern, searchPattern)
	return players, err
}

// FindByPhone retrieves a player by their phone number
func (r *playerRepository) FindByPhone(ctx context.Context, phone string) (*models.Player, error) {
	var player models.Player
	err := r.db.GetContext(ctx, &player, `
		SELECT id, first_name, last_name, email, phone, club_id, created_at, updated_at
		FROM players 
		WHERE phone = ?
	`, phone)
	if err != nil {
		return nil, err
	}
	return &player, nil
}

// FindByTeam retrieves all players in a specific team for a season
func (r *playerRepository) FindByTeam(ctx context.Context, teamID uint, seasonID uint) ([]models.Player, error) {
	var players []models.Player
	err := r.db.SelectContext(ctx, &players, `
		SELECT p.id, p.first_name, p.last_name, p.email, p.phone, p.club_id, p.created_at, p.updated_at
		FROM players p
		INNER JOIN player_teams pt ON p.id = pt.player_id
		WHERE pt.team_id = ? AND pt.season_id = ?
		ORDER BY p.last_name ASC, p.first_name ASC
	`, teamID, seasonID)
	return players, err
}

// FindTeamsForPlayer retrieves all teams a player is part of for a specific season
func (r *playerRepository) FindTeamsForPlayer(ctx context.Context, playerID string, seasonID uint) ([]models.PlayerTeam, error) {
	var playerTeams []models.PlayerTeam
	err := r.db.SelectContext(ctx, &playerTeams, `
		SELECT id, player_id, team_id, season_id, is_active, created_at, updated_at
		FROM player_teams
		WHERE player_id = ? AND season_id = ?
		ORDER BY created_at ASC
	`, playerID, seasonID)
	return playerTeams, err
}

// FindAllTeamsForPlayer retrieves all teams a player has ever been part of
func (r *playerRepository) FindAllTeamsForPlayer(ctx context.Context, playerID string) ([]models.PlayerTeam, error) {
	var playerTeams []models.PlayerTeam
	err := r.db.SelectContext(ctx, &playerTeams, `
		SELECT id, player_id, team_id, season_id, is_active, created_at, updated_at
		FROM player_teams
		WHERE player_id = ?
		ORDER BY season_id DESC, created_at ASC
	`, playerID)
	return playerTeams, err
}

// IsPlayerInTeam checks if a player is in a specific team for a season
func (r *playerRepository) IsPlayerInTeam(ctx context.Context, playerID string, teamID uint, seasonID uint) (bool, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM player_teams 
		WHERE player_id = ? AND team_id = ? AND season_id = ?
	`, playerID, teamID, seasonID)
	return count > 0, err
}

// FindCaptains retrieves all players who are captains
func (r *playerRepository) FindCaptains(ctx context.Context) ([]models.Player, error) {
	var players []models.Player
	err := r.db.SelectContext(ctx, &players, `
		SELECT DISTINCT p.id, p.first_name, p.last_name, p.email, p.phone, p.club_id, p.created_at, p.updated_at
		FROM players p
		INNER JOIN captains c ON p.id = c.player_id
		WHERE c.is_active = TRUE
		ORDER BY p.last_name ASC, p.first_name ASC
	`)
	return players, err
}

// FindCaptainsByRole retrieves all players who are captains with a specific role
func (r *playerRepository) FindCaptainsByRole(ctx context.Context, role models.CaptainRole) ([]models.Player, error) {
	var players []models.Player
	err := r.db.SelectContext(ctx, &players, `
		SELECT DISTINCT p.id, p.first_name, p.last_name, p.email, p.phone, p.club_id, p.created_at, p.updated_at
		FROM players p
		INNER JOIN captains c ON p.id = c.player_id
		WHERE c.role = ? AND c.is_active = TRUE
		ORDER BY p.last_name ASC, p.first_name ASC
	`, string(role))
	return players, err
}

// FindCaptainsByTeam retrieves all captains for a specific team and season
func (r *playerRepository) FindCaptainsByTeam(ctx context.Context, teamID uint, seasonID uint) ([]models.Player, error) {
	var players []models.Player
	err := r.db.SelectContext(ctx, &players, `
		SELECT p.id, p.first_name, p.last_name, p.email, p.phone, p.club_id, p.created_at, p.updated_at
		FROM players p
		INNER JOIN captains c ON p.id = c.player_id
		WHERE c.team_id = ? AND c.season_id = ? AND c.is_active = TRUE
		ORDER BY c.role ASC, p.last_name ASC, p.first_name ASC
	`, teamID, seasonID)
	return players, err
}

// FindCaptainsByClub retrieves all captains from a specific club for a season
func (r *playerRepository) FindCaptainsByClub(ctx context.Context, clubID uint, seasonID uint) ([]models.Player, error) {
	var players []models.Player
	err := r.db.SelectContext(ctx, &players, `
		SELECT DISTINCT p.id, p.first_name, p.last_name, p.email, p.phone, p.club_id, p.created_at, p.updated_at
		FROM players p
		INNER JOIN captains c ON p.id = c.player_id
		WHERE p.club_id = ? AND c.season_id = ? AND c.is_active = TRUE
		ORDER BY p.last_name ASC, p.first_name ASC
	`, clubID, seasonID)
	return players, err
}

// IsPlayerCaptain checks if a player is a captain for a specific team and season
func (r *playerRepository) IsPlayerCaptain(ctx context.Context, playerID string, teamID uint, seasonID uint) (bool, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM captains 
		WHERE player_id = ? AND team_id = ? AND season_id = ? AND is_active = TRUE
	`, playerID, teamID, seasonID)
	return count > 0, err
}

// FindCaptainRoles retrieves all captain roles for a player in a specific season
func (r *playerRepository) FindCaptainRoles(ctx context.Context, playerID string, seasonID uint) ([]models.Captain, error) {
	var captains []models.Captain
	err := r.db.SelectContext(ctx, &captains, `
		SELECT id, player_id, team_id, role, season_id, is_active, created_at, updated_at
		FROM captains
		WHERE player_id = ? AND season_id = ? AND is_active = TRUE
		ORDER BY role ASC, created_at ASC
	`, playerID, seasonID)
	return captains, err
}

// FindDayCaptains retrieves all players who are day captains
func (r *playerRepository) FindDayCaptains(ctx context.Context) ([]models.Player, error) {
	return r.FindCaptainsByRole(ctx, models.DayCaptain)
}

// FindFixturesAsDayCaptain retrieves all fixtures where a player is the day captain
func (r *playerRepository) FindFixturesAsDayCaptain(ctx context.Context, playerID string) ([]models.Fixture, error) {
	var fixtures []models.Fixture
	err := r.db.SelectContext(ctx, &fixtures, `
		SELECT id, home_team_id, away_team_id, division_id, season_id, scheduled_date, 
		       venue_location, status, completed_date, day_captain_id, notes, created_at, updated_at
		FROM fixtures
		WHERE day_captain_id = ?
		ORDER BY scheduled_date ASC
	`, playerID)
	return fixtures, err
}

// SearchPlayers performs a comprehensive search across player fields
func (r *playerRepository) SearchPlayers(ctx context.Context, query string) ([]models.Player, error) {
	var players []models.Player
	searchPattern := "%" + query + "%"
	err := r.db.SelectContext(ctx, &players, `
		SELECT id, first_name, last_name, email, phone, club_id, created_at, updated_at
		FROM players 
		WHERE first_name LIKE ? OR last_name LIKE ? OR email LIKE ? OR phone LIKE ?
		   OR (first_name || ' ' || last_name) LIKE ?
		ORDER BY last_name ASC, first_name ASC
	`, searchPattern, searchPattern, searchPattern, searchPattern, searchPattern)
	return players, err
}

// FindActivePlayersInSeason retrieves all players who are active in a specific season
func (r *playerRepository) FindActivePlayersInSeason(ctx context.Context, seasonID uint) ([]models.Player, error) {
	var players []models.Player
	err := r.db.SelectContext(ctx, &players, `
		SELECT DISTINCT p.id, p.first_name, p.last_name, p.email, p.phone, p.club_id, p.created_at, p.updated_at
		FROM players p
		INNER JOIN player_teams pt ON p.id = pt.player_id
		WHERE pt.season_id = ? AND pt.is_active = TRUE
		ORDER BY p.last_name ASC, p.first_name ASC
	`, seasonID)
	return players, err
}

// FindInactivePlayersInSeason retrieves all players who are not active in a specific season
func (r *playerRepository) FindInactivePlayersInSeason(ctx context.Context, seasonID uint) ([]models.Player, error) {
	var players []models.Player
	err := r.db.SelectContext(ctx, &players, `
		SELECT p.id, p.first_name, p.last_name, p.email, p.phone, p.club_id, p.created_at, p.updated_at
		FROM players p
		WHERE p.id NOT IN (
			SELECT DISTINCT pt.player_id 
			FROM player_teams pt 
			WHERE pt.season_id = ? AND pt.is_active = TRUE
		)
		ORDER BY p.last_name ASC, p.first_name ASC
	`, seasonID)
	return players, err
}

// CountByClub returns the number of players in a specific club
func (r *playerRepository) CountByClub(ctx context.Context, clubID uint) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM players WHERE club_id = ?
	`, clubID)
	return count, err
}

// CountCaptains returns the total number of active captains
func (r *playerRepository) CountCaptains(ctx context.Context) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(DISTINCT player_id) FROM captains WHERE is_active = TRUE
	`)
	return count, err
}

// CountActiveInSeason returns the number of active players in a specific season
func (r *playerRepository) CountActiveInSeason(ctx context.Context, seasonID uint) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(DISTINCT player_id) FROM player_teams 
		WHERE season_id = ? AND is_active = TRUE
	`, seasonID)
	return count, err
}

// CountAll returns the total number of players in the database
func (r *playerRepository) CountAll(ctx context.Context) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM players
	`)
	return count, err
}

// FindByFantasyMatchID retrieves a player by their fantasy match ID
func (r *playerRepository) FindByFantasyMatchID(ctx context.Context, fantasyMatchID uint) (*models.Player, error) {
	var player models.Player
	err := r.db.GetContext(ctx, &player, `
		SELECT id, first_name, last_name, email, phone, club_id, fantasy_match_id, created_at, updated_at
		FROM players 
		WHERE fantasy_match_id = ?
	`, fantasyMatchID)
	if err != nil {
		return nil, err
	}
	return &player, nil
}
