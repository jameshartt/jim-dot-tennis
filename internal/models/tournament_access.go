// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package models

import "strings"

// TournamentAccess is how much of the club's tournament management an account
// can use (WI-153; users.tournament_access, migration 036).
type TournamentAccess string

const (
	// TournamentAccessNone: no tournament management.
	TournamentAccessNone TournamentAccess = "none"
	// TournamentAccessOwn: the account's own tournaments and those it has
	// been given.
	TournamentAccessOwn TournamentAccess = "own"
	// TournamentAccessAll: every tournament of the club.
	TournamentAccessAll TournamentAccess = "all"
)

// ParseTournamentAccess returns the access named s (none, own or all, case
// ignored) and whether s named one.
func ParseTournamentAccess(s string) (TournamentAccess, bool) {
	switch a := TournamentAccess(strings.ToLower(strings.TrimSpace(s))); a {
	case TournamentAccessNone, TournamentAccessOwn, TournamentAccessAll:
		return a, true
	}
	return TournamentAccessNone, false
}

// Label is the access in words, for pages and the command line.
func (a TournamentAccess) Label() string {
	switch a {
	case TournamentAccessOwn:
		return "Own tournaments"
	case TournamentAccessAll:
		return "All tournaments"
	}
	return "No tournaments"
}

// EffectiveTournamentAccess is the one answer to "what tournament access does
// this account have": an active administrator always has all; any other
// active account has its stored value (an unknown value counts as none); a
// disabled account has none. Every caller uses it, directly or through
// User.TournamentAccessLevel.
func EffectiveTournamentAccess(role Role, active bool, stored TournamentAccess) TournamentAccess {
	if !active {
		return TournamentAccessNone
	}
	if role == RoleAdmin {
		return TournamentAccessAll
	}
	switch stored {
	case TournamentAccessOwn, TournamentAccessAll:
		return stored
	}
	return TournamentAccessNone
}

// TournamentAccessLevel is the user's effective tournament access
// (EffectiveTournamentAccess).
func (u User) TournamentAccessLevel() TournamentAccess {
	return EffectiveTournamentAccess(u.Role, u.IsActive, u.TournamentAccess)
}
