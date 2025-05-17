package repository

import (
	"context"
	"database/sql"
	"time"

	"jim-dot-tennis/internal/models"
)

// AvailabilityRepository handles database operations for player availability
type AvailabilityRepository struct {
	db *sql.DB
}

// NewAvailabilityRepository creates a new repository for availability operations
func NewAvailabilityRepository(db *sql.DB) *AvailabilityRepository {
	return &AvailabilityRepository{
		db: db,
	}
}

// GetPlayerDivisions returns all divisions a player is eligible to play in for a given season
func (r *AvailabilityRepository) GetPlayerDivisions(ctx context.Context, playerID string, seasonID uint) ([]models.PlayerDivision, error) {
	query := `
		SELECT id, player_id, division_id, season_id, created_at, updated_at
		FROM player_divisions
		WHERE player_id = $1 AND season_id = $2
	`
	
	rows, err := r.db.QueryContext(ctx, query, playerID, seasonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var divisions []models.PlayerDivision
	for rows.Next() {
		var div models.PlayerDivision
		if err := rows.Scan(
			&div.ID, 
			&div.PlayerID, 
			&div.DivisionID, 
			&div.SeasonID, 
			&div.CreatedAt, 
			&div.UpdatedAt,
		); err != nil {
			return nil, err
		}
		divisions = append(divisions, div)
	}
	
	return divisions, nil
}

// AddPlayerDivision adds a division to a player's eligible divisions
func (r *AvailabilityRepository) AddPlayerDivision(ctx context.Context, division models.PlayerDivision) (uint, error) {
	query := `
		INSERT INTO player_divisions (player_id, division_id, season_id)
		VALUES ($1, $2, $3)
		RETURNING id
	`
	
	var id uint
	err := r.db.QueryRowContext(
		ctx, 
		query, 
		division.PlayerID, 
		division.DivisionID, 
		division.SeasonID,
	).Scan(&id)
	
	return id, err
}

// RemovePlayerDivision removes a division from a player's eligible divisions
func (r *AvailabilityRepository) RemovePlayerDivision(ctx context.Context, playerID string, divisionID, seasonID uint) error {
	query := `
		DELETE FROM player_divisions
		WHERE player_id = $1 AND division_id = $2 AND season_id = $3
	`
	
	_, err := r.db.ExecContext(ctx, query, playerID, divisionID, seasonID)
	return err
}

// SetGeneralAvailability sets a player's general availability for a day of the week
func (r *AvailabilityRepository) SetGeneralAvailability(
	ctx context.Context, 
	availability models.PlayerGeneralAvailability,
) (uint, error) {
	query := `
		INSERT INTO player_general_availability 
		(player_id, day_of_week, status, season_id, notes)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (player_id, day_of_week, season_id) 
		DO UPDATE SET 
			status = EXCLUDED.status,
			notes = EXCLUDED.notes,
			updated_at = CURRENT_TIMESTAMP
		RETURNING id
	`
	
	var id uint
	err := r.db.QueryRowContext(
		ctx, 
		query, 
		availability.PlayerID, 
		availability.DayOfWeek, 
		availability.Status, 
		availability.SeasonID,
		availability.Notes,
	).Scan(&id)
	
	return id, err
}

// GetGeneralAvailability gets a player's general availability for a season
func (r *AvailabilityRepository) GetGeneralAvailability(
	ctx context.Context, 
	playerID string, 
	seasonID uint,
) ([]models.PlayerGeneralAvailability, error) {
	query := `
		SELECT id, player_id, day_of_week, status, season_id, notes, created_at, updated_at
		FROM player_general_availability
		WHERE player_id = $1 AND season_id = $2
	`
	
	rows, err := r.db.QueryContext(ctx, query, playerID, seasonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var availabilities []models.PlayerGeneralAvailability
	for rows.Next() {
		var avail models.PlayerGeneralAvailability
		if err := rows.Scan(
			&avail.ID,
			&avail.PlayerID,
			&avail.DayOfWeek,
			&avail.Status,
			&avail.SeasonID,
			&avail.Notes,
			&avail.CreatedAt,
			&avail.UpdatedAt,
		); err != nil {
			return nil, err
		}
		availabilities = append(availabilities, avail)
	}
	
	return availabilities, nil
}

// AddAvailabilityException adds a date range exception to a player's availability
func (r *AvailabilityRepository) AddAvailabilityException(
	ctx context.Context, 
	exception models.PlayerAvailabilityException,
) (uint, error) {
	query := `
		INSERT INTO player_availability_exceptions 
		(player_id, status, start_date, end_date, reason)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`
	
	var id uint
	err := r.db.QueryRowContext(
		ctx, 
		query, 
		exception.PlayerID, 
		exception.Status, 
		exception.StartDate, 
		exception.EndDate,
		exception.Reason,
	).Scan(&id)
	
	return id, err
}

// GetActiveAvailabilityExceptions gets a player's active availability exceptions for a date
func (r *AvailabilityRepository) GetActiveAvailabilityExceptions(
	ctx context.Context, 
	playerID string, 
	date time.Time,
) ([]models.PlayerAvailabilityException, error) {
	query := `
		SELECT id, player_id, status, start_date, end_date, reason, created_at, updated_at
		FROM player_availability_exceptions
		WHERE player_id = $1 
		AND start_date <= $2 
		AND end_date >= $2
		ORDER BY created_at DESC
	`
	
	rows, err := r.db.QueryContext(ctx, query, playerID, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var exceptions []models.PlayerAvailabilityException
	for rows.Next() {
		var ex models.PlayerAvailabilityException
		if err := rows.Scan(
			&ex.ID,
			&ex.PlayerID,
			&ex.Status,
			&ex.StartDate,
			&ex.EndDate,
			&ex.Reason,
			&ex.CreatedAt,
			&ex.UpdatedAt,
		); err != nil {
			return nil, err
		}
		exceptions = append(exceptions, ex)
	}
	
	return exceptions, nil
}

// DeleteAvailabilityException deletes an availability exception
func (r *AvailabilityRepository) DeleteAvailabilityException(
	ctx context.Context, 
	exceptionID uint,
) error {
	query := `
		DELETE FROM player_availability_exceptions
		WHERE id = $1
	`
	
	_, err := r.db.ExecContext(ctx, query, exceptionID)
	return err
}

// SetFixtureAvailability sets a player's availability for a specific fixture
func (r *AvailabilityRepository) SetFixtureAvailability(
	ctx context.Context, 
	availability models.PlayerFixtureAvailability,
) (uint, error) {
	query := `
		INSERT INTO player_fixture_availability 
		(player_id, fixture_id, status, notes)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (player_id, fixture_id) 
		DO UPDATE SET 
			status = EXCLUDED.status,
			notes = EXCLUDED.notes,
			updated_at = CURRENT_TIMESTAMP
		RETURNING id
	`
	
	var id uint
	err := r.db.QueryRowContext(
		ctx, 
		query, 
		availability.PlayerID, 
		availability.FixtureID, 
		availability.Status, 
		availability.Notes,
	).Scan(&id)
	
	return id, err
}

// GetFixtureAvailability gets a player's availability for a specific fixture
func (r *AvailabilityRepository) GetFixtureAvailability(
	ctx context.Context, 
	playerID string, 
	fixtureID uint,
) (*models.PlayerFixtureAvailability, error) {
	query := `
		SELECT id, player_id, fixture_id, status, notes, created_at, updated_at
		FROM player_fixture_availability
		WHERE player_id = $1 AND fixture_id = $2
	`
	
	var avail models.PlayerFixtureAvailability
	err := r.db.QueryRowContext(ctx, query, playerID, fixtureID).Scan(
		&avail.ID,
		&avail.PlayerID,
		&avail.FixtureID,
		&avail.Status,
		&avail.Notes,
		&avail.CreatedAt,
		&avail.UpdatedAt,
	)
	
	if err == sql.ErrNoRows {
		return nil, nil
	}
	
	if err != nil {
		return nil, err
	}
	
	return &avail, nil
}

// GetPlayerAvailabilityForFixture calculates a player's effective availability for a fixture
func (r *AvailabilityRepository) GetPlayerAvailabilityForFixture(
	ctx context.Context, 
	playerID string, 
	fixtureID uint, 
) (models.AvailabilityStatus, error) {
	// First get the fixture details to get the date
	var fixtureDate time.Time
	var dayOfWeek string
	
	err := r.db.QueryRowContext(ctx, 
		"SELECT scheduled_date, to_char(scheduled_date, 'Day') FROM fixtures WHERE id = $1", 
		fixtureID,
	).Scan(&fixtureDate, &dayOfWeek)
	
	if err != nil {
		return models.Unknown, err
	}
	
	// Check if there's a fixture-specific availability
	fixtureAvail, err := r.GetFixtureAvailability(ctx, playerID, fixtureID)
	if err != nil {
		return models.Unknown, err
	}
	
	if fixtureAvail != nil {
		return fixtureAvail.Status, nil
	}
	
	// Check if there's a date exception
	exceptions, err := r.GetActiveAvailabilityExceptions(ctx, playerID, fixtureDate)
	if err != nil {
		return models.Unknown, err
	}
	
	if len(exceptions) > 0 {
		// Use the most recently created exception if multiple overlap
		return exceptions[0].Status, nil
	}
	
	// Fall back to general availability
	var status models.AvailabilityStatus
	err = r.db.QueryRowContext(ctx,
		"SELECT status FROM player_general_availability WHERE player_id = $1 AND day_of_week = $2",
		playerID, dayOfWeek,
	).Scan(&status)
	
	if err == sql.ErrNoRows {
		return models.Unknown, nil
	}
	
	if err != nil {
		return models.Unknown, err
	}
	
	return status, nil
}

// GetPlayersAvailableForFixture returns all eligible players who are available for a specific fixture
// This uses joins to efficiently query the database
func (r *AvailabilityRepository) GetPlayersAvailableForFixture(
	ctx context.Context,
	fixtureID uint,
) ([]models.Player, error) {
	query := `
		WITH fixture_details AS (
			SELECT f.id, f.division_id, f.scheduled_date, f.season_id, 
			       to_char(f.scheduled_date, 'Day') as day_of_week,
			       d.play_day
			FROM fixtures f
			JOIN divisions d ON f.division_id = d.id
			WHERE f.id = $1
		),
		eligible_players AS (
			-- Players eligible to play in this division/season
			SELECT pd.player_id
			FROM player_divisions pd
			JOIN fixture_details fd ON pd.division_id = fd.division_id AND pd.season_id = fd.season_id
			
			UNION
			
			-- Players in teams in this division
			SELECT pt.player_id
			FROM player_teams pt
			JOIN teams t ON pt.team_id = t.id
			JOIN fixture_details fd ON t.division_id = fd.division_id AND t.season_id = fd.season_id
			WHERE pt.is_active = true
		),
		player_availability AS (
			-- Check fixture-specific availability first (highest priority)
			SELECT p.id, p.first_name, p.last_name, p.email, p.phone, p.club_id, 
			       COALESCE(pfa.status, 'Unknown') as status,
			       1 as priority -- Fixture-specific has highest priority
			FROM players p
			JOIN eligible_players ep ON p.id = ep.player_id
			LEFT JOIN player_fixture_availability pfa ON p.id = pfa.player_id AND pfa.fixture_id = $1
			WHERE pfa.status = 'Available'
			
			UNION ALL
			
			-- Then check for date-specific exceptions (medium priority)
			SELECT p.id, p.first_name, p.last_name, p.email, p.phone, p.club_id,
			       pae.status,
			       2 as priority -- Date exceptions have medium priority
			FROM players p
			JOIN eligible_players ep ON p.id = ep.player_id
			JOIN fixture_details fd ON 1=1
			JOIN player_availability_exceptions pae ON p.id = pae.player_id 
			     AND pae.start_date <= fd.scheduled_date 
			     AND pae.end_date >= fd.scheduled_date
			LEFT JOIN player_fixture_availability pfa ON p.id = pfa.player_id AND pfa.fixture_id = $1
			WHERE pfa.id IS NULL AND pae.status = 'Available'
			
			UNION ALL
			
			-- Finally check general day-of-week availability (lowest priority)
			SELECT p.id, p.first_name, p.last_name, p.email, p.phone, p.club_id,
			       pga.status,
			       3 as priority -- General availability has lowest priority
			FROM players p
			JOIN eligible_players ep ON p.id = ep.player_id
			JOIN fixture_details fd ON 1=1
			JOIN player_general_availability pga ON p.id = pga.player_id 
			     AND pga.day_of_week = fd.day_of_week
			     AND pga.season_id = fd.season_id
			LEFT JOIN player_fixture_availability pfa ON p.id = pfa.player_id AND pfa.fixture_id = $1
			LEFT JOIN player_availability_exceptions pae ON p.id = pae.player_id 
			     AND pae.start_date <= fd.scheduled_date 
			     AND pae.end_date >= fd.scheduled_date
			WHERE pfa.id IS NULL AND pae.id IS NULL AND pga.status = 'Available'
		)
		-- Get distinct players with highest priority status (most specific)
		SELECT DISTINCT ON (id) id, first_name, last_name, email, phone, club_id
		FROM player_availability
		ORDER BY id, priority
	`
	
	rows, err := r.db.QueryContext(ctx, query, fixtureID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var players []models.Player
	for rows.Next() {
		var player models.Player
		if err := rows.Scan(
			&player.ID,
			&player.FirstName,
			&player.LastName,
			&player.Email,
			&player.Phone,
			&player.ClubID,
		); err != nil {
			return nil, err
		}
		players = append(players, player)
	}
	
	return players, nil
}

// GetTeamPlayersWithAvailability returns all players in a team with their availability for a fixture
func (r *AvailabilityRepository) GetTeamPlayersWithAvailability(
	ctx context.Context,
	teamID uint,
	fixtureID uint,
) ([]struct {
	Player      models.Player
	Availability models.AvailabilityStatus
}, error) {
	query := `
		WITH fixture_details AS (
			SELECT f.id, f.scheduled_date, f.season_id,
			       to_char(f.scheduled_date, 'Day') as day_of_week
			FROM fixtures f
			WHERE f.id = $2
		)
		SELECT p.id, p.first_name, p.last_name, p.email, p.phone, p.club_id,
		       COALESCE(
		           -- First check fixture-specific availability
		           (SELECT pfa.status FROM player_fixture_availability pfa 
		            WHERE pfa.player_id = p.id AND pfa.fixture_id = $2),
		            
		           -- Then check for date exceptions
		           (SELECT pae.status FROM player_availability_exceptions pae
		            JOIN fixture_details fd ON 1=1
		            WHERE pae.player_id = p.id 
		            AND pae.start_date <= fd.scheduled_date 
		            AND pae.end_date >= fd.scheduled_date
		            ORDER BY pae.created_at DESC
		            LIMIT 1),
		            
		           -- Then check general day-of-week availability
		           (SELECT pga.status FROM player_general_availability pga
		            JOIN fixture_details fd ON 1=1
		            WHERE pga.player_id = p.id 
		            AND pga.day_of_week = fd.day_of_week
		            AND pga.season_id = fd.season_id),
		            
		           -- Default to Unknown
		           'Unknown'
		       ) as availability_status
		FROM players p
		JOIN player_teams pt ON p.id = pt.player_id
		JOIN fixture_details fd ON 1=1
		WHERE pt.team_id = $1
		AND pt.season_id = fd.season_id
		AND pt.is_active = true
		ORDER BY p.last_name, p.first_name
	`
	
	rows, err := r.db.QueryContext(ctx, query, teamID, fixtureID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var result []struct {
		Player      models.Player
		Availability models.AvailabilityStatus
	}
	
	for rows.Next() {
		var player models.Player
		var status string
		
		if err := rows.Scan(
			&player.ID,
			&player.FirstName,
			&player.LastName,
			&player.Email,
			&player.Phone,
			&player.ClubID,
			&status,
		); err != nil {
			return nil, err
		}
		
		result = append(result, struct{
			Player      models.Player
			Availability models.AvailabilityStatus
		}{
			Player:      player,
			Availability: models.AvailabilityStatus(status),
		})
	}
	
	return result, nil
}

// GetFixturesWithPlayerAvailability gets all fixtures for a division with player availability
func (r *AvailabilityRepository) GetFixturesWithPlayerAvailability(
	ctx context.Context,
	divisionID uint,
	playerID string,
) ([]struct {
	Fixture      models.Fixture
	Availability models.AvailabilityStatus
}, error) {
	query := `
		SELECT f.id, f.home_team_id, f.away_team_id, f.division_id, f.season_id,
		       f.scheduled_date, f.venue_location, f.status, f.completed_date,
		       f.day_captain_id, f.notes, f.created_at, f.updated_at,
		       COALESCE(
		           -- First check fixture-specific availability
		           (SELECT pfa.status FROM player_fixture_availability pfa 
		            WHERE pfa.player_id = $2 AND pfa.fixture_id = f.id),
		            
		           -- Then check for date exceptions
		           (SELECT pae.status FROM player_availability_exceptions pae
		            WHERE pae.player_id = $2 
		            AND pae.start_date <= f.scheduled_date 
		            AND pae.end_date >= f.scheduled_date
		            ORDER BY pae.created_at DESC
		            LIMIT 1),
		            
		           -- Then check general day-of-week availability
		           (SELECT pga.status FROM player_general_availability pga
		            WHERE pga.player_id = $2 
		            AND pga.day_of_week = to_char(f.scheduled_date, 'Day')
		            AND pga.season_id = f.season_id),
		            
		           -- Default to Unknown
		           'Unknown'
		       ) as availability_status
		FROM fixtures f
		WHERE f.division_id = $1
		AND f.scheduled_date >= CURRENT_DATE
		ORDER BY f.scheduled_date
	`
	
	rows, err := r.db.QueryContext(ctx, query, divisionID, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var result []struct {
		Fixture      models.Fixture
		Availability models.AvailabilityStatus
	}
	
	for rows.Next() {
		var fixture models.Fixture
		var status string
		
		if err := rows.Scan(
			&fixture.ID,
			&fixture.HomeTeamID,
			&fixture.AwayTeamID,
			&fixture.DivisionID,
			&fixture.SeasonID,
			&fixture.ScheduledDate,
			&fixture.VenueLocation,
			&fixture.Status,
			&fixture.CompletedDate,
			&fixture.DayCaptainID,
			&fixture.Notes,
			&fixture.CreatedAt,
			&fixture.UpdatedAt,
			&status,
		); err != nil {
			return nil, err
		}
		
		result = append(result, struct{
			Fixture      models.Fixture
			Availability models.AvailabilityStatus
		}{
			Fixture:      fixture,
			Availability: models.AvailabilityStatus(status),
		})
	}
	
	return result, nil
} 