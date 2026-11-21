// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/repository"

	_ "github.com/mattn/go-sqlite3"
)

func findMigrationsPath(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 6; i++ {
		candidate := filepath.Join(dir, "migrations")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("could not locate migrations directory")
	return ""
}

type testEnv struct {
	db       *database.DB
	clubs    repository.ClubRepository
	settings repository.SettingsRepository
}

// newTestEnv returns a migrated database with no HOME_CLUB_* variables set.
func newTestEnv(t *testing.T) testEnv {
	t.Helper()
	t.Setenv("HOME_CLUB_ID", "")
	t.Setenv("HOME_CLUB_NAME", "")
	db, err := database.New(database.Config{Driver: "sqlite3", FilePath: filepath.Join(t.TempDir(), "test.db")})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.ExecuteMigrations(findMigrationsPath(t)); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return testEnv{db: db, clubs: repository.NewClubRepository(db), settings: repository.NewSettingsRepository(db)}
}

func (e testEnv) addClub(t *testing.T, id uint, name string) {
	t.Helper()
	// Only the name: the optional columns stay NULL, as on a freshly imported club.
	if _, err := e.db.Exec(`INSERT INTO clubs (id, name) VALUES (?, ?)`, id, name); err != nil {
		t.Fatalf("insert club: %v", err)
	}
}

func TestResolveHomeClubOrder(t *testing.T) {
	ctx := context.Background()
	e := newTestEnv(t)
	e.addClub(t, 1, "Alpha Tennis Club")
	e.addClub(t, 2, "Beta Lawn Tennis")
	e.addClub(t, 3, "Gamma Park")

	// Nothing configured.
	club, src, err := ResolveHomeClub(ctx, e.clubs, e.settings)
	if err != nil || club != nil || src != SourceNone {
		t.Fatalf("nothing set: got %v, %q, %v", club, src, err)
	}

	// Stored setting only.
	if err := e.settings.SetUint(ctx, repository.SettingHomeClubID, 3); err != nil {
		t.Fatal(err)
	}
	club, src, err = ResolveHomeClub(ctx, e.clubs, e.settings)
	if err != nil || club == nil || club.ID != 3 || src != SourceStored {
		t.Fatalf("stored: got %v, %q, %v", club, src, err)
	}

	// HOME_CLUB_NAME beats the stored setting.
	t.Setenv("HOME_CLUB_NAME", "Beta")
	club, src, err = ResolveHomeClub(ctx, e.clubs, e.settings)
	if err != nil || club == nil || club.ID != 2 || src != SourceEnvName {
		t.Fatalf("env name: got %v, %q, %v", club, src, err)
	}

	// HOME_CLUB_ID beats both.
	t.Setenv("HOME_CLUB_ID", "1")
	club, src, err = ResolveHomeClub(ctx, e.clubs, e.settings)
	if err != nil || club == nil || club.ID != 1 || src != SourceEnvID {
		t.Fatalf("env id: got %v, %q, %v", club, src, err)
	}

	// A nil settings repository just skips the stored setting.
	t.Setenv("HOME_CLUB_ID", "")
	t.Setenv("HOME_CLUB_NAME", "")
	club, src, err = ResolveHomeClub(ctx, e.clubs, nil)
	if err != nil || club != nil || src != SourceNone {
		t.Fatalf("nil settings: got %v, %q, %v", club, src, err)
	}
}

func TestResolveHomeClubEnvMismatchListsClubs(t *testing.T) {
	ctx := context.Background()
	e := newTestEnv(t)
	e.addClub(t, 1, "Alpha Tennis Club")
	// A valid stored setting must not rescue a bad env value: env wins, as before.
	if err := e.settings.SetUint(ctx, repository.SettingHomeClubID, 1); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME_CLUB_ID", "99")
	_, _, err := ResolveHomeClub(ctx, e.clubs, e.settings)
	var hcErr *HomeClubError
	if !errors.As(err, &hcErr) || hcErr.Source != SourceEnvID {
		t.Fatalf("env id mismatch: err = %v", err)
	}
	if !strings.Contains(err.Error(), "HOME_CLUB_ID=99 does not match any club") || !strings.Contains(err.Error(), "Alpha Tennis Club (ID: 1)") {
		t.Fatalf("env id mismatch message = %q", err)
	}

	t.Setenv("HOME_CLUB_ID", "not-a-number")
	if _, _, err := ResolveHomeClub(ctx, e.clubs, e.settings); err == nil || errors.As(err, &hcErr) {
		t.Fatalf("invalid HOME_CLUB_ID: err = %v, want a parse error", err)
	}

	t.Setenv("HOME_CLUB_ID", "")
	t.Setenv("HOME_CLUB_NAME", "Nowhere")
	_, _, err = ResolveHomeClub(ctx, e.clubs, e.settings)
	if !errors.As(err, &hcErr) || hcErr.Source != SourceEnvName || !strings.Contains(err.Error(), "Available clubs: [Alpha Tennis Club (ID: 1)]") {
		t.Fatalf("env name mismatch: err = %v", err)
	}
}

func TestNewHomeClubHolderStartup(t *testing.T) {
	ctx := context.Background()

	t.Run("env mismatch with clubs present is fatal, as before", func(t *testing.T) {
		e := newTestEnv(t)
		e.addClub(t, 1, "Alpha")
		t.Setenv("HOME_CLUB_ID", "99")
		if _, err := NewHomeClubHolder(ctx, e.clubs, e.settings); err == nil {
			t.Fatal("want error")
		}
	})

	t.Run("invalid HOME_CLUB_ID is fatal even on an empty database", func(t *testing.T) {
		e := newTestEnv(t)
		t.Setenv("HOME_CLUB_ID", "abc")
		if _, err := NewHomeClubHolder(ctx, e.clubs, e.settings); err == nil {
			t.Fatal("want error")
		}
	})

	t.Run("nothing configured starts unresolved", func(t *testing.T) {
		e := newTestEnv(t)
		h, err := NewHomeClubHolder(ctx, e.clubs, e.settings)
		if err != nil || h.Resolved() || h.ID() != 0 || h.Club() != nil {
			t.Fatalf("got %+v, %v", h, err)
		}
	})

	t.Run("env set on an empty clubs table resolves once the club appears", func(t *testing.T) {
		e := newTestEnv(t)
		t.Setenv("HOME_CLUB_ID", "1")
		h, err := NewHomeClubHolder(ctx, e.clubs, e.settings)
		if err != nil || h.Resolved() {
			t.Fatalf("startup: resolved=%v err=%v", h != nil && h.Resolved(), err)
		}
		if id, club := h.Current(ctx); id != 0 || club != nil {
			t.Fatalf("Current before seeding = %d, %v", id, club)
		}
		e.addClub(t, 1, "Seeded Club")
		id, club := h.Current(ctx)
		if id != 1 || club == nil || club.Name != "Seeded Club" || h.Source() != SourceEnvID {
			t.Fatalf("Current after seeding = %d, %v (source %q)", id, club, h.Source())
		}
	})

	t.Run("stale stored setting starts unresolved", func(t *testing.T) {
		e := newTestEnv(t)
		e.addClub(t, 1, "Alpha")
		if err := e.settings.SetUint(ctx, repository.SettingHomeClubID, 42); err != nil {
			t.Fatal(err)
		}
		h, err := NewHomeClubHolder(ctx, e.clubs, e.settings)
		if err != nil || h.Resolved() {
			t.Fatalf("got resolved=%v err=%v", h != nil && h.Resolved(), err)
		}
	})
}

func TestHomeClubHolderRefreshMakesStoredClubEffective(t *testing.T) {
	ctx := context.Background()
	e := newTestEnv(t)
	e.addClub(t, 4, "Delta")
	h, err := NewHomeClubHolder(ctx, e.clubs, e.settings)
	if err != nil {
		t.Fatal(err)
	}

	// Concurrent readers while the setting is stored and refreshed (run with -race).
	var wg sync.WaitGroup
	stop := make(chan struct{})
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = h.ID()
					_ = h.Club()
				}
			}
		}()
	}

	if err := e.settings.SetUint(ctx, repository.SettingHomeClubID, 4); err != nil {
		t.Fatal(err)
	}
	if err := h.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	close(stop)
	wg.Wait()

	if h.ID() != 4 || h.Club().Name != "Delta" || h.Source() != SourceStored {
		t.Fatalf("after Refresh: id %d club %v source %q", h.ID(), h.Club(), h.Source())
	}

	// The middleware injects the live value.
	var src HomeClubSource = h
	if src.ID() != 4 {
		t.Fatalf("HomeClubSource.ID() = %d", src.ID())
	}
}

func TestLoadRequiresAHomeClub(t *testing.T) {
	ctx := context.Background()
	e := newTestEnv(t)
	if _, err := Load(ctx, e.clubs, e.settings); err == nil {
		t.Fatal("Load with nothing configured: want error")
	}
	e.addClub(t, 1, "Alpha")
	t.Setenv("HOME_CLUB_NAME", "alp")
	cfg, err := Load(ctx, e.clubs, e.settings)
	if err != nil || cfg.HomeClubID != 1 || cfg.HomeClub.Name != "Alpha" {
		t.Fatalf("Load = %+v, %v", cfg, err)
	}
}
