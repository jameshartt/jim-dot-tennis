// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package models

import "testing"

func TestEffectiveTournamentAccess(t *testing.T) {
	roles := []Role{RoleAdmin, RoleCaptain, RoleTournamentsOnly, "", "unknown"}
	stored := []TournamentAccess{TournamentAccessNone, TournamentAccessOwn, TournamentAccessAll, "", "bogus"}
	for _, role := range roles {
		for _, s := range stored {
			// Disabled: always none.
			if got := EffectiveTournamentAccess(role, false, s); got != TournamentAccessNone {
				t.Errorf("disabled %q with %q: %q, want none", role, s, got)
			}
			want := s
			switch {
			case role == RoleAdmin:
				want = TournamentAccessAll
			case s != TournamentAccessOwn && s != TournamentAccessAll:
				want = TournamentAccessNone
			}
			if got := EffectiveTournamentAccess(role, true, s); got != want {
				t.Errorf("active %q with %q: %q, want %q", role, s, got, want)
			}
			u := User{Role: role, IsActive: true, TournamentAccess: s}
			if u.TournamentAccessLevel() != want {
				t.Errorf("User.TournamentAccessLevel disagrees for %q/%q", role, s)
			}
		}
	}
}

func TestParseTournamentAccess(t *testing.T) {
	for in, want := range map[string]TournamentAccess{"none": TournamentAccessNone, "OWN": TournamentAccessOwn, " all ": TournamentAccessAll} {
		if got, ok := ParseTournamentAccess(in); !ok || got != want {
			t.Errorf("ParseTournamentAccess(%q) = %q, %v", in, got, ok)
		}
	}
	for _, in := range []string{"", "some", "player", "admin"} {
		if _, ok := ParseTournamentAccess(in); ok {
			t.Errorf("ParseTournamentAccess(%q) accepted", in)
		}
	}
}
