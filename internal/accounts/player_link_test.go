// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package accounts

import (
	"context"
	"errors"
	"testing"

	"jim-dot-tennis/internal/models"
)

// TestParseRoleRetiresPlayer: only admin and captain can be given to an
// account (WI-132); the player role is refused for new accounts and changes.
func TestParseRoleRetiresPlayer(t *testing.T) {
	for in, want := range map[string]models.Role{"admin": models.RoleAdmin, " Captain ": models.RoleCaptain} {
		if got, err := ParseRole(in); err != nil || got != want {
			t.Errorf("ParseRole(%q) = %q, %v", in, got, err)
		}
	}
	for _, in := range []string{"player", "owner", ""} {
		if _, err := ParseRole(in); !errors.Is(err, ErrInvalidRole) {
			t.Errorf("ParseRole(%q): err %v, want ErrInvalidRole", in, err)
		}
	}

	ctx := context.Background()
	s, _ := newTestService(t)
	mustCreate(t, s, "alice", models.RoleAdmin)
	bob := mustCreate(t, s, "bob", models.RoleCaptain)
	if _, _, err := s.Invite(ctx, cli, "pat", models.RolePlayer, nil); !errors.Is(err, ErrInvalidRole) {
		t.Errorf("invite as player: err %v", err)
	}
	if _, err := s.SetRole(ctx, cli, bob.ID, models.RolePlayer); !errors.Is(err, ErrInvalidRole) {
		t.Errorf("change to player: err %v", err)
	}
}

func TestSetPlayerLink(t *testing.T) {
	ctx := context.Background()
	s, db := newTestService(t)
	if _, err := db.Exec(`INSERT INTO clubs (id, name) VALUES (1, 'Club')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO players (id, first_name, last_name, club_id) VALUES ('p1', 'Pat', 'Player', 1), ('p2', 'Sam', 'Player', 1)`); err != nil {
		t.Fatal(err)
	}
	mustCreate(t, s, "alice", models.RoleAdmin)
	carol := mustCreate(t, s, "carol", models.RoleCaptain)
	dave := mustCreate(t, s, "dave", models.RoleCaptain)

	a, err := s.SetPlayerLink(ctx, cli, carol.ID, "p1")
	if err != nil || a.PlayerID == nil || *a.PlayerID != "p1" {
		t.Fatalf("link carol to p1: %+v %v", a, err)
	}
	// Linking again to the same player is allowed (no change).
	if _, err := s.SetPlayerLink(ctx, cli, carol.ID, "p1"); err != nil {
		t.Errorf("relink carol to p1: %v", err)
	}
	if _, err := s.SetPlayerLink(ctx, cli, dave.ID, "p1"); !errors.Is(err, ErrPlayerLinked) {
		t.Errorf("p1 is carol's: err %v", err)
	}
	if _, err := s.SetPlayerLink(ctx, cli, dave.ID, "nobody"); !errors.Is(err, ErrPlayerNotFound) {
		t.Errorf("unknown player: err %v", err)
	}
	if _, err := s.SetPlayerLink(ctx, cli, 9999, "p2"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown account: err %v", err)
	}
	// Change carol's link, then remove it.
	if a, err := s.SetPlayerLink(ctx, cli, carol.ID, " p2 "); err != nil || a.PlayerID == nil || *a.PlayerID != "p2" {
		t.Fatalf("change carol to p2: %+v %v", a, err)
	}
	if a, err := s.SetPlayerLink(ctx, cli, carol.ID, ""); err != nil || a.PlayerID != nil {
		t.Fatalf("unlink carol: %+v %v", a, err)
	}
	// p1 is free again.
	if _, err := s.SetPlayerLink(ctx, cli, dave.ID, "p1"); err != nil {
		t.Errorf("link dave to the freed p1: %v", err)
	}
}
