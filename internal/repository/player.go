package repository

import (
	"context"
	"fmt"
	"time"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
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
	FindByName(ctx context.Context, firstName, lastName string) ([]models.Player, error)
	FindByNameLike(ctx context.Context, name string) ([]models.Player, error)

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

	// Appearances
	FindPlayersWhoPlayedForClubInSeason(ctx context.Context, clubID uint, seasonID uint) ([]models.Player, error)
	FindPlayersWhoPlayedForTeamInSeason(ctx context.Context, teamID uint, seasonID uint) ([]models.Player, error)
	FindPlayersWhoPlayedForClubDivisionInSeason(ctx context.Context, clubID uint, divisionID uint, seasonID uint) ([]models.Player, error)

	// Appearance counts
	CountPlayerAppearancesForTeamInSeason(ctx context.Context, playerID string, teamID uint, seasonID uint) (int, error)
	CountPlayerAppearancesForClubDivisionInSeason(ctx context.Context, playerID string, clubID uint, divisionID uint, seasonID uint) (int, error)

	// Fantasy match queries
	FindByFantasyMatchID(ctx context.Context, fantasyMatchID uint) (*models.Player, error)

	// Preferred name operations
	CreatePreferredNameRequest(ctx context.Context, request *models.PreferredNameRequest) error
	FindPreferredNameRequestsByStatus(ctx context.Context, status models.PreferredNameRequestStatus) ([]models.PreferredNameRequest, error)
	FindPreferredNameRequestByID(ctx context.Context, id uint) (*models.PreferredNameRequest, error)
	FindPreferredNameRequestsByPlayer(ctx context.Context, playerID string) ([]models.PreferredNameRequest, error)
	UpdatePreferredNameRequest(ctx context.Context, request *models.PreferredNameRequest) error
	ApprovePreferredNameRequest(ctx context.Context, requestID uint, adminUsername string, adminNotes *string) error
	RejectPreferredNameRequest(ctx context.Context, requestID uint, adminUsername string, adminNotes *string) error
	IsPreferredNameAvailable(ctx context.Context, name string) (bool, error)
	UpdatePlayerPreferredName(ctx context.Context, playerID string, preferredName *string) error

	// Statistics
	CountByClub(ctx context.Context, clubID uint) (int, error)
	CountCaptains(ctx context.Context) (int, error)
	CountActiveInSeason(ctx context.Context, seasonID uint) (int, error)
	CountAll(ctx context.Context) (int, error)
	CountPendingPreferredNameRequests(ctx context.Context) (int, error)
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
		SELECT id, first_name, last_name, preferred_name, gender, reporting_privacy, club_id, fantasy_match_id, created_at, updated_at
		FROM players 
		ORDER BY last_name ASC, first_name ASC
	`)
	return players, err
}

// FindByID retrieves a player by their ID (UUID)
func (r *playerRepository) FindByID(ctx context.Context, id string) (*models.Player, error) {
	var player models.Player
	err := r.db.GetContext(ctx, &player, `
		SELECT id, first_name, last_name, preferred_name, gender, reporting_privacy, club_id, fantasy_match_id, created_at, updated_at
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
		INSERT INTO players (id, first_name, last_name, preferred_name, gender, reporting_privacy, club_id, fantasy_match_id, created_at, updated_at)
		VALUES (:id, :first_name, :last_name, :preferred_name, :gender, :reporting_privacy, :club_id, :fantasy_match_id, :created_at, :updated_at)
	`, player)

	return err
}

// Update modifies an existing player record
func (r *playerRepository) Update(ctx context.Context, player *models.Player) error {
	player.UpdatedAt = time.Now()

	_, err := r.db.NamedExecContext(ctx, `
		UPDATE players 
		SET first_name = :first_name, last_name = :last_name, preferred_name = :preferred_name, gender = :gender,
		    reporting_privacy = :reporting_privacy, club_id = :club_id, fantasy_match_id = :fantasy_match_id, updated_at = :updated_at
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
		SELECT id, first_name, last_name, preferred_name, gender, reporting_privacy, club_id, fantasy_match_id, created_at, updated_at
		FROM players 
		WHERE club_id = ?
		ORDER BY last_name ASC, first_name ASC
	`, clubID)
	return players, err
}

// FindByName retrieves players with exact first and last name match
func (r *playerRepository) FindByName(ctx context.Context, firstName, lastName string) ([]models.Player, error) {
	var players []models.Player
	err := r.db.SelectContext(ctx, &players, `
		SELECT id, first_name, last_name, preferred_name, gender, reporting_privacy, club_id, fantasy_match_id, created_at, updated_at
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
		SELECT id, first_name, last_name, preferred_name, gender, reporting_privacy, club_id, fantasy_match_id, created_at, updated_at
		FROM players 
		WHERE first_name LIKE ? OR last_name LIKE ? OR (first_name || ' ' || last_name) LIKE ?
		ORDER BY last_name ASC, first_name ASC
	`, searchPattern, searchPattern, searchPattern)
	return players, err
}

// FindByTeam retrieves all players in a specific team for a season
func (r *playerRepository) FindByTeam(ctx context.Context, teamID uint, seasonID uint) ([]models.Player, error) {
	var players []models.Player
	err := r.db.SelectContext(ctx, &players, `
		SELECT p.id, p.first_name, p.last_name, p.preferred_name, p.gender, p.reporting_privacy, p.club_id, p.fantasy_match_id, p.created_at, p.updated_at
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
		SELECT DISTINCT p.id, p.first_name, p.last_name, p.club_id, p.created_at, p.updated_at
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
		SELECT DISTINCT p.id, p.first_name, p.last_name, p.club_id, p.created_at, p.updated_at
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
		SELECT p.id, p.first_name, p.last_name, p.club_id, p.created_at, p.updated_at
		FROM players p
		INNER JOIN captains c ON p.id = c.player_id
		WHERE c.team_id = ? AND c.season_id = ? AND c.is_active = TRUE
		ORDER BY p.last_name ASC, p.first_name ASC
	`, teamID, seasonID)
	return players, err
}

// FindCaptainsByClub retrieves all captains for a specific club and season
func (r *playerRepository) FindCaptainsByClub(ctx context.Context, clubID uint, seasonID uint) ([]models.Player, error) {
	var players []models.Player
	err := r.db.SelectContext(ctx, &players, `
		SELECT DISTINCT p.id, p.first_name, p.last_name, p.club_id, p.created_at, p.updated_at
		FROM players p
		INNER JOIN captains c ON p.id = c.player_id
		INNER JOIN teams t ON c.team_id = t.id
		WHERE t.club_id = ? AND c.season_id = ? AND c.is_active = TRUE
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

// FindCaptainRoles retrieves all captain roles for a player in a season
func (r *playerRepository) FindCaptainRoles(ctx context.Context, playerID string, seasonID uint) ([]models.Captain, error) {
	var captains []models.Captain
	err := r.db.SelectContext(ctx, &captains, `
		SELECT id, player_id, team_id, role, season_id, is_active, created_at, updated_at
		FROM captains
		WHERE player_id = ? AND season_id = ? AND is_active = TRUE
		ORDER BY created_at ASC
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
		SELECT id, home_team_id, away_team_id, division_id, season_id, week_id, 
		       scheduled_date, venue_location, status, completed_date, day_captain_id, notes,
		       created_at, updated_at
		FROM fixtures
		WHERE day_captain_id = ?
		ORDER BY scheduled_date ASC
	`, playerID)
	return fixtures, err
}

// SearchPlayers searches for players by name
func (r *playerRepository) SearchPlayers(ctx context.Context, query string) ([]models.Player, error) {
	var players []models.Player
	searchPattern := "%" + query + "%"
	err := r.db.SelectContext(ctx, &players, `
		SELECT id, first_name, last_name, preferred_name, gender, reporting_privacy, club_id, fantasy_match_id, created_at, updated_at
		FROM players 
		WHERE first_name LIKE ? OR last_name LIKE ?
		ORDER BY last_name ASC, first_name ASC
	`, searchPattern, searchPattern)
	return players, err
}

// FindActivePlayersInSeason retrieves all players who are active in a specific season
func (r *playerRepository) FindActivePlayersInSeason(ctx context.Context, seasonID uint) ([]models.Player, error) {
	var players []models.Player
	err := r.db.SelectContext(ctx, &players, `
		SELECT DISTINCT p.id, p.first_name, p.last_name, p.preferred_name, p.gender, p.reporting_privacy, p.club_id, p.fantasy_match_id, p.created_at, p.updated_at
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
		SELECT p.id, p.first_name, p.last_name, p.preferred_name, p.gender, p.reporting_privacy, p.club_id, p.fantasy_match_id, p.created_at, p.updated_at
		FROM players p
		WHERE p.id NOT IN (
			SELECT DISTINCT player_id 
			FROM player_teams 
			WHERE season_id = ? AND is_active = TRUE
		)
		ORDER BY p.last_name ASC, p.first_name ASC
	`, seasonID)
	return players, err
}

// FindPlayersWhoPlayedForClubInSeason retrieves distinct players who appeared in any matchup
// for fixtures in the given season where the home or away team belongs to the specified club.
func (r *playerRepository) FindPlayersWhoPlayedForClubInSeason(ctx context.Context, clubID uint, seasonID uint) ([]models.Player, error) {
	var players []models.Player
	err := r.db.SelectContext(ctx, &players, `
		SELECT DISTINCT p.id, p.first_name, p.last_name, p.preferred_name, p.gender, p.reporting_privacy, p.club_id, p.fantasy_match_id, p.created_at, p.updated_at
		FROM players p
		INNER JOIN matchup_players mp ON mp.player_id = p.id
		INNER JOIN matchups m ON m.id = mp.matchup_id
		INNER JOIN fixtures f ON f.id = m.fixture_id
		INNER JOIN teams th ON th.id = f.home_team_id
		INNER JOIN teams ta ON ta.id = f.away_team_id
		WHERE f.season_id = ? AND p.club_id = ? AND (th.club_id = ? OR ta.club_id = ?)
		ORDER BY p.last_name ASC, p.first_name ASC
	`, seasonID, clubID, clubID, clubID)
	return players, err
}

// FindPlayersWhoPlayedForTeamInSeason retrieves distinct players who appeared in any matchup
// for fixtures in the given season where the specified team was home or away.
func (r *playerRepository) FindPlayersWhoPlayedForTeamInSeason(ctx context.Context, teamID uint, seasonID uint) ([]models.Player, error) {
	var players []models.Player
	err := r.db.SelectContext(ctx, &players, `
		SELECT DISTINCT p.id, p.first_name, p.last_name, p.preferred_name, p.gender, p.reporting_privacy, p.club_id, p.fantasy_match_id, p.created_at, p.updated_at
		FROM players p
		INNER JOIN matchup_players mp ON mp.player_id = p.id
		INNER JOIN matchups m ON m.id = mp.matchup_id
		INNER JOIN fixtures f ON f.id = m.fixture_id
		WHERE f.season_id = ? AND (f.home_team_id = ? OR f.away_team_id = ?)
		ORDER BY p.last_name ASC, p.first_name ASC
	`, seasonID, teamID, teamID)
	return players, err
}

// FindPlayersWhoPlayedForClubDivisionInSeason retrieves distinct players who appeared in any matchup
// in fixtures for the given season and division where either team belongs to the specified club.
func (r *playerRepository) FindPlayersWhoPlayedForClubDivisionInSeason(ctx context.Context, clubID uint, divisionID uint, seasonID uint) ([]models.Player, error) {
	var players []models.Player
	err := r.db.SelectContext(ctx, &players, `
		SELECT DISTINCT p.id, p.first_name, p.last_name, p.preferred_name, p.gender, p.reporting_privacy, p.club_id, p.fantasy_match_id, p.created_at, p.updated_at
		FROM players p
		INNER JOIN matchup_players mp ON mp.player_id = p.id
		INNER JOIN matchups m ON m.id = mp.matchup_id
		INNER JOIN fixtures f ON f.id = m.fixture_id
		INNER JOIN teams th ON th.id = f.home_team_id
		INNER JOIN teams ta ON ta.id = f.away_team_id
		WHERE f.season_id = ? AND f.division_id = ? AND (th.club_id = ? OR ta.club_id = ?)
		ORDER BY p.last_name ASC, p.first_name ASC
	`, seasonID, divisionID, clubID, clubID)
	return players, err
}

// CountPlayerAppearancesForTeamInSeason counts the number of matchup appearances for a player
// in fixtures where the given team is home or away in the given season.
func (r *playerRepository) CountPlayerAppearancesForTeamInSeason(ctx context.Context, playerID string, teamID uint, seasonID uint) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(*)
		FROM matchup_players mp
		INNER JOIN matchups m ON m.id = mp.matchup_id
		INNER JOIN fixtures f ON f.id = m.fixture_id
		WHERE mp.player_id = ? AND f.season_id = ? AND (f.home_team_id = ? OR f.away_team_id = ?)
	`, playerID, seasonID, teamID, teamID)
	return count, err
}

// CountPlayerAppearancesForClubDivisionInSeason counts the number of matchup appearances for a player
// in fixtures for the given division and season where either team belongs to the specified club.
func (r *playerRepository) CountPlayerAppearancesForClubDivisionInSeason(ctx context.Context, playerID string, clubID uint, divisionID uint, seasonID uint) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(*)
		FROM matchup_players mp
		INNER JOIN matchups m ON m.id = mp.matchup_id
		INNER JOIN fixtures f ON f.id = m.fixture_id
		INNER JOIN teams th ON th.id = f.home_team_id
		INNER JOIN teams ta ON ta.id = f.away_team_id
		WHERE mp.player_id = ? AND f.season_id = ? AND f.division_id = ? AND (th.club_id = ? OR ta.club_id = ?)
	`, playerID, seasonID, divisionID, clubID, clubID)
	return count, err
}

// CountByClub returns the number of players in a club
func (r *playerRepository) CountByClub(ctx context.Context, clubID uint) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM players WHERE club_id = ?
	`, clubID)
	return count, err
}

// CountCaptains returns the number of active captains
func (r *playerRepository) CountCaptains(ctx context.Context) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(DISTINCT player_id) FROM captains WHERE is_active = TRUE
	`)
	return count, err
}

// CountActiveInSeason returns the number of active players in a season
func (r *playerRepository) CountActiveInSeason(ctx context.Context, seasonID uint) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(DISTINCT player_id) FROM player_teams 
		WHERE season_id = ? AND is_active = TRUE
	`, seasonID)
	return count, err
}

// CountAll returns the total number of players
func (r *playerRepository) CountAll(ctx context.Context) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `SELECT COUNT(*) FROM players`)
	return count, err
}

// FindByFantasyMatchID retrieves a player by their fantasy match ID
func (r *playerRepository) FindByFantasyMatchID(ctx context.Context, fantasyMatchID uint) (*models.Player, error) {
	var player models.Player
	err := r.db.GetContext(ctx, &player, `
		SELECT id, first_name, last_name, preferred_name, club_id, fantasy_match_id, created_at, updated_at
		FROM players 
		WHERE fantasy_match_id = ?
	`, fantasyMatchID)
	if err != nil {
		return nil, err
	}
	return &player, nil
}

// CreatePreferredNameRequest creates a new preferred name request
func (r *playerRepository) CreatePreferredNameRequest(ctx context.Context, request *models.PreferredNameRequest) error {
	now := time.Now()
	request.CreatedAt = now
	request.UpdatedAt = now
	request.Status = models.PreferredNamePending

	_, err := r.db.NamedExecContext(ctx, `
		INSERT INTO preferred_name_requests (player_id, requested_name, status, created_at, updated_at)
		VALUES (:player_id, :requested_name, :status, :created_at, :updated_at)
	`, request)

	return err
}

// FindPreferredNameRequestsByStatus retrieves preferred name requests by status
func (r *playerRepository) FindPreferredNameRequestsByStatus(ctx context.Context, status models.PreferredNameRequestStatus) ([]models.PreferredNameRequest, error) {
	var requests []models.PreferredNameRequest
	err := r.db.SelectContext(ctx, &requests, `
		SELECT pnr.id, pnr.player_id, pnr.requested_name, pnr.status, pnr.admin_notes, 
		       pnr.approved_by, pnr.created_at, pnr.updated_at, pnr.processed_at
		FROM preferred_name_requests pnr
		WHERE pnr.status = ?
		ORDER BY pnr.created_at ASC
	`, string(status))
	return requests, err
}

// FindPreferredNameRequestByID retrieves a preferred name request by ID
func (r *playerRepository) FindPreferredNameRequestByID(ctx context.Context, id uint) (*models.PreferredNameRequest, error) {
	var request models.PreferredNameRequest
	err := r.db.GetContext(ctx, &request, `
		SELECT id, player_id, requested_name, status, admin_notes, approved_by, 
		       created_at, updated_at, processed_at
		FROM preferred_name_requests
		WHERE id = ?
	`, id)
	if err != nil {
		return nil, err
	}
	return &request, nil
}

// FindPreferredNameRequestsByPlayer retrieves all preferred name requests for a player
func (r *playerRepository) FindPreferredNameRequestsByPlayer(ctx context.Context, playerID string) ([]models.PreferredNameRequest, error) {
	var requests []models.PreferredNameRequest
	err := r.db.SelectContext(ctx, &requests, `
		SELECT id, player_id, requested_name, status, admin_notes, approved_by, 
		       created_at, updated_at, processed_at
		FROM preferred_name_requests
		WHERE player_id = ?
		ORDER BY created_at DESC
	`, playerID)
	return requests, err
}

// UpdatePreferredNameRequest updates a preferred name request
func (r *playerRepository) UpdatePreferredNameRequest(ctx context.Context, request *models.PreferredNameRequest) error {
	request.UpdatedAt = time.Now()

	_, err := r.db.NamedExecContext(ctx, `
		UPDATE preferred_name_requests 
		SET status = :status, admin_notes = :admin_notes, approved_by = :approved_by, 
		    processed_at = :processed_at, updated_at = :updated_at
		WHERE id = :id
	`, request)

	return err
}

// ApprovePreferredNameRequest approves a preferred name request and updates the player
func (r *playerRepository) ApprovePreferredNameRequest(ctx context.Context, requestID uint, adminUsername string, adminNotes *string) error {
	// Start transaction
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get the request
	var request models.PreferredNameRequest
	err = tx.GetContext(ctx, &request, `
		SELECT id, player_id, requested_name, status
		FROM preferred_name_requests
		WHERE id = ?
	`, requestID)
	if err != nil {
		return err
	}

	// Check if still pending
	if request.Status != models.PreferredNamePending {
		return fmt.Errorf("request is no longer pending")
	}

	now := time.Now()

	// Update the request to approved
	_, err = tx.ExecContext(ctx, `
		UPDATE preferred_name_requests 
		SET status = ?, admin_notes = ?, approved_by = ?, processed_at = ?, updated_at = ?
		WHERE id = ?
	`, string(models.PreferredNameApproved), adminNotes, adminUsername, now, now, requestID)
	if err != nil {
		return err
	}

	// Update the player's preferred name
	_, err = tx.ExecContext(ctx, `
		UPDATE players 
		SET preferred_name = ?, updated_at = ?
		WHERE id = ?
	`, request.RequestedName, now, request.PlayerID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// RejectPreferredNameRequest rejects a preferred name request
func (r *playerRepository) RejectPreferredNameRequest(ctx context.Context, requestID uint, adminUsername string, adminNotes *string) error {
	now := time.Now()

	_, err := r.db.ExecContext(ctx, `
		UPDATE preferred_name_requests 
		SET status = ?, admin_notes = ?, approved_by = ?, processed_at = ?, updated_at = ?
		WHERE id = ? AND status = ?
	`, string(models.PreferredNameRejected), adminNotes, adminUsername, now, now, requestID, string(models.PreferredNamePending))

	return err
}

// IsPreferredNameAvailable checks if a preferred name is available
func (r *playerRepository) IsPreferredNameAvailable(ctx context.Context, name string) (bool, error) {
	var count int

	// Check if name exists in players table or is pending in requests
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM (
			SELECT 1 FROM players WHERE preferred_name = ?
			UNION
			SELECT 1 FROM preferred_name_requests WHERE requested_name = ? AND status = ?
		)
	`, name, name, string(models.PreferredNamePending))

	return count == 0, err
}

// UpdatePlayerPreferredName updates a player's preferred name directly
func (r *playerRepository) UpdatePlayerPreferredName(ctx context.Context, playerID string, preferredName *string) error {
	now := time.Now()

	_, err := r.db.ExecContext(ctx, `
		UPDATE players 
		SET preferred_name = ?, updated_at = ?
		WHERE id = ?
	`, preferredName, now, playerID)

	return err
}

// CountPendingPreferredNameRequests returns the number of pending preferred name requests
func (r *playerRepository) CountPendingPreferredNameRequests(ctx context.Context) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM preferred_name_requests WHERE status = ?
	`, string(models.PreferredNamePending))
	return count, err
}
