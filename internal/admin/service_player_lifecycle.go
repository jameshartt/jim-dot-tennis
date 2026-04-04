package admin

import (
	"context"
	"fmt"
	"time"
)

// DeactivatePlayer safely removes a player from active duty while preserving all historical data.
// The operation is wrapped in a transaction — all changes succeed or none do.
// Returns an error if the player is an active Team Captain (must be reassigned first).
func (s *Service) DeactivatePlayer(ctx context.Context, playerID string) error {
	// Get active season
	season, err := s.seasonRepository.FindActive(ctx)
	if err != nil || season == nil {
		return fmt.Errorf("failed to get active season: %w", err)
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Guard: refuse if player is an active Team Captain for this season
	var teamCaptainCount int
	err = tx.GetContext(ctx, &teamCaptainCount, `
		SELECT COUNT(*) FROM captains
		WHERE player_id = ? AND season_id = ? AND role = 'Team' AND is_active = TRUE
	`, playerID, season.ID)
	if err != nil {
		return fmt.Errorf("failed to check team captain status: %w", err)
	}
	if teamCaptainCount > 0 {
		return fmt.Errorf("cannot deactivate player: they are an active Team Captain — reassign the captaincy first")
	}

	now := time.Now()

	// 1. Set players.is_active = FALSE and clear fantasy_match_id
	_, err = tx.ExecContext(ctx, `
		UPDATE players SET is_active = FALSE, fantasy_match_id = NULL, updated_at = ? WHERE id = ?
	`, now, playerID)
	if err != nil {
		return fmt.Errorf("failed to deactivate player: %w", err)
	}

	// 2. Set player_teams.is_active = FALSE for the active season
	_, err = tx.ExecContext(ctx, `
		UPDATE player_teams SET is_active = FALSE, updated_at = ?
		WHERE player_id = ? AND season_id = ?
	`, now, playerID, season.ID)
	if err != nil {
		return fmt.Errorf("failed to deactivate player teams: %w", err)
	}

	// 3. Set captains.is_active = FALSE for active season (Day role only)
	_, err = tx.ExecContext(ctx, `
		UPDATE captains SET is_active = FALSE, updated_at = ?
		WHERE player_id = ? AND season_id = ? AND role = 'Day'
	`, now, playerID, season.ID)
	if err != nil {
		return fmt.Errorf("failed to deactivate day captain roles: %w", err)
	}

	// 4. Null out fixtures.day_captain_id for future fixtures
	_, err = tx.ExecContext(ctx, `
		UPDATE fixtures SET day_captain_id = NULL, updated_at = ?
		WHERE day_captain_id = ? AND status NOT IN ('Completed', 'Cancelled')
	`, now, playerID)
	if err != nil {
		return fmt.Errorf("failed to clear day captain from future fixtures: %w", err)
	}

	// 5. Remove fixture_players rows for future fixtures only
	_, err = tx.ExecContext(ctx, `
		DELETE FROM fixture_players
		WHERE player_id = ? AND fixture_id IN (
			SELECT id FROM fixtures WHERE status NOT IN ('Completed', 'Cancelled')
		)
	`, playerID)
	if err != nil {
		return fmt.Errorf("failed to remove player from future fixtures: %w", err)
	}

	// 6. Delete player_general_availability for current season
	_, err = tx.ExecContext(ctx, `
		DELETE FROM player_general_availability
		WHERE player_id = ? AND season_id = ?
	`, playerID, season.ID)
	if err != nil {
		return fmt.Errorf("failed to delete general availability: %w", err)
	}

	// 7. Delete player_availability_exceptions where end_date is in the future
	_, err = tx.ExecContext(ctx, `
		DELETE FROM player_availability_exceptions
		WHERE player_id = ? AND end_date > ?
	`, playerID, now)
	if err != nil {
		return fmt.Errorf("failed to delete future availability exceptions: %w", err)
	}

	// 8. Delete player_fixture_availability for future fixtures
	_, err = tx.ExecContext(ctx, `
		DELETE FROM player_fixture_availability
		WHERE player_id = ? AND fixture_id IN (
			SELECT id FROM fixtures WHERE status NOT IN ('Completed', 'Cancelled')
		)
	`, playerID)
	if err != nil {
		return fmt.Errorf("failed to delete future fixture availability: %w", err)
	}

	// 9. Delete player_availability for future fixtures
	_, err = tx.ExecContext(ctx, `
		DELETE FROM player_availability
		WHERE player_id = ? AND fixture_id IN (
			SELECT id FROM fixtures WHERE status NOT IN ('Completed', 'Cancelled')
		)
	`, playerID)
	if err != nil {
		return fmt.Errorf("failed to delete future player availability: %w", err)
	}

	// 10. Delete player_divisions for current season
	_, err = tx.ExecContext(ctx, `
		DELETE FROM player_divisions
		WHERE player_id = ? AND season_id = ?
	`, playerID, season.ID)
	if err != nil {
		return fmt.Errorf("failed to delete player divisions: %w", err)
	}

	// 11. Delete pending preferred_name_requests
	_, err = tx.ExecContext(ctx, `
		DELETE FROM preferred_name_requests
		WHERE player_id = ? AND status = 'Pending'
	`, playerID)
	if err != nil {
		return fmt.Errorf("failed to delete pending preferred name requests: %w", err)
	}

	// 12. If player has a linked user, deactivate user and clear sessions
	var userID *int64
	err = tx.GetContext(ctx, &userID, `
		SELECT id FROM users WHERE player_id = ? AND is_active = TRUE
	`, playerID)
	if err == nil && userID != nil {
		_, err = tx.ExecContext(ctx, `
			UPDATE users SET is_active = FALSE WHERE id = ?
		`, *userID)
		if err != nil {
			return fmt.Errorf("failed to deactivate linked user: %w", err)
		}
		_, err = tx.ExecContext(ctx, `
			UPDATE sessions SET is_valid = FALSE WHERE user_id = ?
		`, *userID)
		if err != nil {
			return fmt.Errorf("failed to invalidate user sessions: %w", err)
		}
	}

	return tx.Commit()
}

// ReactivatePlayer sets a player's is_active flag back to TRUE.
// Re-adding them to teams and divisions is a separate manual step.
func (s *Service) ReactivatePlayer(ctx context.Context, playerID string) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `
		UPDATE players SET is_active = TRUE, updated_at = ? WHERE id = ?
	`, now, playerID)
	if err != nil {
		return fmt.Errorf("failed to reactivate player: %w", err)
	}
	return nil
}
