// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package models

// Two roles have meaning (WI-132):
//
//   - captain: everything a captain does, for every team (planning,
//     availability, selection, fixtures, results, players, standings).
//   - admin: everything a captain can do, plus running the instance (users,
//     sessions, settings, seasons, imports configuration, backups).
//
// RolePlayer is retired: players use link tokens, never accounts. An account
// left with the player role (from before WI-132) can sign in but reaches
// nothing that needs a role.

// RoleAtLeast reports whether role grants at least min. An administrator has
// every captain permission; any other role (player, empty, unknown) grants
// nothing.
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
