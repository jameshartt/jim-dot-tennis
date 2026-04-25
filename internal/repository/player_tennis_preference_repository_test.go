// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"jim-dot-tennis/internal/database"

	_ "github.com/mattn/go-sqlite3"
)

// Sprint 018 WI-108 + WI-111 backfill regression.
//
// Boots a fresh in-memory-style SQLite database, runs every migration
// (including 027), seeds a player whose tier-3 answers are present but
// wizard_progress_tier was never set, and asserts the cascading backfill
// promoted the player to tier 3 — and that BumpWizardProgressTier never
// decrements once they're there.
func TestWizardProgressBackfillAndBump(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "wizard_test.db")

	cfg := database.Config{
		Driver:   "sqlite3",
		FilePath: dbPath,
	}
	db, err := database.New(cfg)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	migrationsPath := findMigrationsPath(t)
	if err := db.ExecuteMigrations(migrationsPath); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	ctx := context.Background()

	// Seed a minimal player (FK requirement on player_tennis_preferences).
	if _, err := db.ExecContext(ctx, `
		INSERT INTO clubs (id, name) VALUES (1, 'Test Club')
		ON CONFLICT DO NOTHING
	`); err != nil {
		t.Fatalf("seed club: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO players (id, first_name, last_name, club_id, gender)
		VALUES ('p-test', 'T', 'Player', 1, 'Unknown')
	`); err != nil {
		t.Fatalf("seed player: %v", err)
	}

	// Seed a tier-3 row but explicitly leave wizard_progress_tier at 0,
	// emulating the state of a Sprint 016 user post-migration before the
	// backfill UPDATE has run on their row. Then re-run the backfill SQL
	// to verify it cascades correctly.
	if _, err := db.ExecContext(ctx, `
		INSERT INTO player_tennis_preferences (player_id, handedness, signature_shot, wizard_progress_tier)
		VALUES ('p-test', 'right', 'inside-out forehand', 0)
	`); err != nil {
		t.Fatalf("seed prefs: %v", err)
	}

	// Re-apply the backfill expression on this single row.
	if _, err := db.ExecContext(ctx, `
		UPDATE player_tennis_preferences
		SET wizard_progress_tier = CASE
			WHEN years_playing IS NOT NULL OR how_i_got_into_tennis IS NOT NULL OR tennis_hero_or_style IS NOT NULL OR pre_match_ritual IS NOT NULL OR tennis_spirit_animal IS NOT NULL OR walkout_song IS NOT NULL OR celebration_style IS NOT NULL OR post_match IS NOT NULL OR my_tennis_in_one_line IS NOT NULL THEN 6
			WHEN season_goal IS NOT NULL OR improvement_focus IS NOT NULL OR what_to_know_about_my_game IS NOT NULL OR accessibility_notes IS NOT NULL OR weather_tolerance IS NOT NULL OR notes_to_captain IS NOT NULL THEN 5
			WHEN partner_consistency IS NOT NULL OR on_court_vibe IS NOT NULL OR competitiveness IS NOT NULL OR pressure_response IS NOT NULL OR EXISTS (SELECT 1 FROM player_preferred_partners pp WHERE pp.player_id = player_tennis_preferences.player_id) THEN 4
			WHEN handedness IS NOT NULL OR backhand IS NOT NULL OR serve_style IS NOT NULL OR net_comfort IS NOT NULL OR preferred_court_side IS NOT NULL OR signature_shot IS NOT NULL OR shot_im_working_on IS NOT NULL OR favourite_tactic IS NOT NULL THEN 3
			WHEN preferred_days IS NOT NULL OR preferred_times IS NOT NULL OR max_travel_miles IS NOT NULL OR transport IS NOT NULL OR home_court_matters IS NOT NULL THEN 2
			WHEN mixed_doubles_appetite IS NOT NULL OR same_gender_doubles_appetite IS NOT NULL OR open_to_fill_in IS NOT NULL OR preferred_contact IS NOT NULL OR best_window_for_last_minute IS NOT NULL THEN 1
			ELSE 0
		END
		WHERE player_id = 'p-test'
	`); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	repo := NewPlayerTennisPreferenceRepository(db)

	tier, err := repo.GetWizardProgressTier(ctx, "p-test")
	if err != nil {
		t.Fatalf("get progress: %v", err)
	}
	if tier != 3 {
		t.Errorf("after backfill of tier-3 answers: progress = %d; want 3", tier)
	}

	// Bump up to 4 — should advance.
	if err := repo.BumpWizardProgressTier(ctx, "p-test", 4); err != nil {
		t.Fatalf("bump 4: %v", err)
	}
	if got, _ := repo.GetWizardProgressTier(ctx, "p-test"); got != 4 {
		t.Errorf("after bump to 4: progress = %d; want 4", got)
	}

	// Bump back to 2 — must NOT decrement (monotonic contract).
	if err := repo.BumpWizardProgressTier(ctx, "p-test", 2); err != nil {
		t.Fatalf("bump 2: %v", err)
	}
	if got, _ := repo.GetWizardProgressTier(ctx, "p-test"); got != 4 {
		t.Errorf("after bump to 2 (lower): progress = %d; want 4 (monotonic)", got)
	}

	// Bump for a fresh player creates a row at the supplied tier.
	if _, err := db.ExecContext(ctx, `
		INSERT INTO players (id, first_name, last_name, club_id, gender)
		VALUES ('p-fresh', 'F', 'Player', 1, 'Unknown')
	`); err != nil {
		t.Fatalf("seed fresh player: %v", err)
	}
	if err := repo.BumpWizardProgressTier(ctx, "p-fresh", 1); err != nil {
		t.Fatalf("bump fresh: %v", err)
	}
	if got, _ := repo.GetWizardProgressTier(ctx, "p-fresh"); got != 1 {
		t.Errorf("fresh player after bump 1: progress = %d; want 1", got)
	}
}

// findMigrationsPath walks up from the test file to locate the project's
// migrations directory, since `go test` runs from the package directory.
func findMigrationsPath(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := cwd
	for i := 0; i < 6; i++ {
		candidate := filepath.Join(dir, "migrations")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not locate migrations directory from %s", cwd)
	return ""
}
