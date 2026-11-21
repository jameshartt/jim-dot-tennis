// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package repository

import (
	"context"
	"testing"
)

// A clubs row created with only a name (address, website and phone_number are
// nullable in the schema) must scan into models.Club. It used to fail, which
// surfaced as a misleading "does not match any club" home club error.
func TestClubWithNullOptionalColumnsScans(t *testing.T) {
	db := newMigratedTestDB(t)
	ctx := context.Background()

	if _, err := db.ExecContext(ctx, `INSERT INTO clubs (id, name) VALUES (5, 'Nulls Club')`); err != nil {
		t.Fatalf("seed club: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO players (id, first_name, last_name, club_id, gender)
		VALUES ('p-null', 'N', 'Player', 5, 'Unknown')`); err != nil {
		t.Fatalf("seed player: %v", err)
	}
	repo := NewClubRepository(db)

	club, err := repo.FindByID(ctx, 5)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if club.Name != "Nulls Club" || club.Address != "" || club.Website != "" || club.PhoneNumber != "" {
		t.Fatalf("FindByID = %+v", club)
	}

	all, err := repo.FindAll(ctx)
	if err != nil || len(all) != 1 {
		t.Fatalf("FindAll = %v, %v", all, err)
	}
	if like, err := repo.FindByNameLike(ctx, "Nulls"); err != nil || len(like) != 1 {
		t.Fatalf("FindByNameLike = %v, %v", like, err)
	}
	if byName, err := repo.FindByName(ctx, "Nulls Club"); err != nil || len(byName) != 1 {
		t.Fatalf("FindByName = %v, %v", byName, err)
	}
	if byPlayer, err := repo.FindByPlayerID(ctx, "p-null"); err != nil || byPlayer.ID != 5 {
		t.Fatalf("FindByPlayerID = %v, %v", byPlayer, err)
	}
}
