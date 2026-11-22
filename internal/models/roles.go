// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package models

// Two roles have meaning (WI-132):
//
//   - captain: everything a captain does, for every team (planning,
//     availability, selection, fixtures, results, players, standings).
//   - admin: everything a captain can do, plus running the instance (users,
//     sessions, settings, seasons, imports configuration, backups).
//
// The stored role value "player" (RoleTournamentsOnly) grants no league
// access: players use link tokens, never accounts. Since WI-153 the value is
// reused for "Tournaments only" accounts, which reach no league page; what
// they can use is their tournament access (users.tournament_access, read
// through EffectiveTournamentAccess). An account left with the player role
// from before WI-132 has no tournament access, so it can sign in but reaches
// nothing until an administrator gives it a role or tournament access.

// RoleAtLeast reports whether role grants at least min. An administrator has
// every captain permission; any other role (tournaments only, empty, unknown)
// grants nothing in the league.
func RoleAtLeast(role, min Role) bool {
	rank := func(r Role) int {
		switch r {
		case RoleAdmin:
			return 2
		case RoleCaptain:
			return 1
		}
		return 0
	}
	return rank(min) > 0 && rank(role) >= rank(min)
}

// IsAdmin reports whether the user is an administrator.
func (u User) IsAdmin() bool { return u.Role == RoleAdmin }
