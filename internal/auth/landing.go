// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"context"

	"jim-dot-tennis/internal/models"
)

// TournamentHomePath is the tournament home page (internal/tournamenthome):
// where an account with no league role and tournament access lands.
const TournamentHomePath = "/tournament-home"

// HomePathFor is where user lands after signing in when no particular page
// was asked for: leagueHome (the league dashboard) for a captain or an
// administrator; the tournament home page for an account with no league role
// and tournament access (a "Tournaments only" account); leagueHome for anyone
// else, as before WI-153.
func HomePathFor(user models.User, leagueHome string) string {
	if !models.RoleAtLeast(user.Role, models.RoleCaptain) &&
		user.TournamentAccessLevel() != models.TournamentAccessNone {
		return TournamentHomePath
	}
	return leagueHome
}

// HomePath is HomePathFor for the account userID, read from the database.
// If it cannot be read, leagueHome.
func (s *Service) HomePath(ctx context.Context, userID int64, leagueHome string) string {
	var user models.User
	if err := s.db.GetContext(ctx, &user, `SELECT * FROM users WHERE id = ?`, userID); err != nil {
		return leagueHome
	}
	return HomePathFor(user, leagueHome)
}
