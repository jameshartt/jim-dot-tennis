// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"jim-dot-tennis/internal/database"
)

func newMigratedTestDB(t *testing.T) *database.DB {
	t.Helper()
	db, err := database.New(database.Config{Driver: "sqlite3", FilePath: filepath.Join(t.TempDir(), "test.db")})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.ExecuteMigrations(findMigrationsPath(t)); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestSettingsRepositoryTypedRoundTrips(t *testing.T) {
	db := newMigratedTestDB(t)
	ctx := context.Background()
	repo := NewSettingsRepository(db)

	if _, ok, err := repo.GetString(ctx, "missing"); err != nil || ok {
		t.Fatalf("GetString(missing) = ok %v, err %v; want not found, nil", ok, err)
	}
	if _, ok, err := repo.GetUint(ctx, "missing"); err != nil || ok {
		t.Fatalf("GetUint(missing) = ok %v, err %v; want not found, nil", ok, err)
	}

	if err := repo.SetString(ctx, "s", "hello"); err != nil {
		t.Fatal(err)
	}
	if v, ok, err := repo.GetString(ctx, "s"); err != nil || !ok || v != "hello" {
		t.Fatalf("GetString = %q, %v, %v", v, ok, err)
	}
	// Overwrite replaces the value (upsert, not a second row).
	if err := repo.SetString(ctx, "s", "again"); err != nil {
		t.Fatal(err)
	}
	if v, _, _ := repo.GetString(ctx, "s"); v != "again" {
		t.Fatalf("after overwrite GetString = %q, want again", v)
	}

	if err := repo.SetUint(ctx, "u", 42); err != nil {
		t.Fatal(err)
	}
	if v, ok, err := repo.GetUint(ctx, "u"); err != nil || !ok || v != 42 {
		t.Fatalf("GetUint = %d, %v, %v", v, ok, err)
	}

	if err := repo.SetBool(ctx, "b", true); err != nil {
		t.Fatal(err)
	}
	if v, ok, err := repo.GetBool(ctx, "b"); err != nil || !ok || !v {
		t.Fatalf("GetBool = %v, %v, %v", v, ok, err)
	}

	when := time.Date(2026, 11, 21, 10, 30, 0, 123, time.FixedZone("x", 3600))
	if err := repo.SetTime(ctx, "t", when); err != nil {
		t.Fatal(err)
	}
	if v, ok, err := repo.GetTime(ctx, "t"); err != nil || !ok || !v.Equal(when) {
		t.Fatalf("GetTime = %v, %v, %v; want %v", v, ok, err, when)
	}

	// A value of the wrong type is an error, not "not found".
	if _, ok, err := repo.GetUint(ctx, "s"); err == nil || !ok {
		t.Fatalf("GetUint on a string value = ok %v, err %v; want found + error", ok, err)
	}
	if _, _, err := repo.GetBool(ctx, "s"); err == nil {
		t.Fatal("GetBool on a string value: want error")
	}
	if _, _, err := repo.GetTime(ctx, "s"); err == nil {
		t.Fatal("GetTime on a string value: want error")
	}

	if err := repo.Delete(ctx, "u"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := repo.GetUint(ctx, "u"); ok {
		t.Fatal("after Delete, setting still found")
	}
	if err := repo.Delete(ctx, "never-set"); err != nil {
		t.Fatalf("Delete of unset key: %v", err)
	}
}

func TestSettingsRepositoryInTransaction(t *testing.T) {
	db := newMigratedTestDB(t)
	ctx := context.Background()

	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := NewSettingsRepository(tx).SetUint(ctx, SettingHomeClubID, 7); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := NewSettingsRepository(db).GetUint(ctx, SettingHomeClubID); ok {
		t.Fatal("rolled-back setting is visible")
	}

	tx, err = db.BeginTxx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := NewSettingsRepository(tx).SetUint(ctx, SettingHomeClubID, 7); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if v, ok, err := NewSettingsRepository(db).GetUint(ctx, SettingHomeClubID); err != nil || !ok || v != 7 {
		t.Fatalf("committed setting = %d, %v, %v", v, ok, err)
	}
}
