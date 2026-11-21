// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package repository

import (
	"context"
	"testing"
)

func TestSeedIfEmptyLoadsOnceAndSkipsPopulated(t *testing.T) {
	db := newMigratedTestDB(t)
	ctx := context.Background()
	repo := NewProTennisPlayerRepository(db)

	n, err := repo.SeedIfEmpty(ctx)
	if err != nil || n == 0 {
		t.Fatalf("first seed = %d, %v; want rows, nil", n, err)
	}
	if c, _ := repo.CountAll(ctx); c != n {
		t.Fatalf("count %d != seeded %d", c, n)
	}

	// Idempotent: a second call leaves the table untouched.
	if n2, err := repo.SeedIfEmpty(ctx); err != nil || n2 != 0 {
		t.Fatalf("second seed = %d, %v; want 0, nil", n2, err)
	}
	if c, _ := repo.CountAll(ctx); c != n {
		t.Fatalf("count changed to %d after second seed", c)
	}

	// Populated by something else: untouched.
	if err := repo.ClearAll(ctx); err != nil {
		t.Fatal(err)
	}
	if err := repo.ImportFromData(ctx, []byte(`{"atp_players":[{"id":1,"last_name":"Solo","common_name":"A Solo"}]}`)); err != nil {
		t.Fatal(err)
	}
	if n3, err := repo.SeedIfEmpty(ctx); err != nil || n3 != 0 {
		t.Fatalf("seed on populated = %d, %v; want 0, nil", n3, err)
	}
	if c, _ := repo.CountAll(ctx); c != 1 {
		t.Fatalf("populated table modified: count %d", c)
	}
}

func TestImportFromDataRejectsMalformed(t *testing.T) {
	db := newMigratedTestDB(t)
	ctx := context.Background()
	repo := NewProTennisPlayerRepository(db)
	for _, in := range []string{`not json`, `{}`} {
		if err := repo.ImportFromData(ctx, []byte(in)); err == nil {
			t.Errorf("ImportFromData(%q) = nil; want error", in)
		}
	}
	if c, _ := repo.CountAll(ctx); c != 0 {
		t.Fatalf("count %d after failed imports", c)
	}
}
