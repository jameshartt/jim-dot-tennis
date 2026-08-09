// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"

	_ "github.com/mattn/go-sqlite3"
)

// A rescheduled fixture must stay within its season's year. A date-picker fat-finger
// that lands the fixture in a different year (e.g. year 0008 instead of 2026, which
// happened in production to fixture 529) must be rejected, not silently saved.
func TestUpdateFixtureScheduleRejectsWrongYear(t *testing.T) {
	tmp := t.TempDir()
	db, err := database.New(database.Config{Driver: "sqlite3", FilePath: filepath.Join(tmp, "resched_year.db")})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := db.ExecuteMigrations(findMigrationsPathAdmin(t)); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	ctx := context.Background()
	exec := func(q string, args ...interface{}) {
		t.Helper()
		if _, err := db.ExecContext(ctx, q, args...); err != nil {
			t.Fatalf("seed %q: %v", q, err)
		}
	}

	seasonStart := time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC)
	seasonEnd := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	exec(`INSERT INTO seasons (id, name, year, start_date, end_date, is_active) VALUES (1, '2026 Season', 2026, ?, ?, 1)`, seasonStart, seasonEnd)
	exec(`INSERT INTO weeks (id, week_number, season_id, start_date, end_date, name) VALUES (8, 8, 1, ?, ?, 'Week 8')`, seasonStart, seasonEnd)
	exec(`INSERT INTO leagues (id, name, type, year, region) VALUES (1, 'Test League', 'Parks', 2026, 'Brighton')`)
	exec(`INSERT INTO divisions (id, name, level, play_day, league_id, season_id) VALUES (1, 'Division 1', 1, 'Tuesday', 1, 1)`)
	exec(`INSERT INTO clubs (id, name) VALUES (1, 'St Ann''s'), (2, 'Rivals')`)
	exec(`INSERT INTO teams (id, name, club_id, division_id, season_id) VALUES (1, 'St Ann''s', 1, 1, 1), (2, 'Rivals', 2, 1, 1)`)
	exec(`INSERT INTO fixtures (id, home_team_id, away_team_id, division_id, season_id, week_id, scheduled_date, venue_location, status, notes)
		VALUES (900, 1, 2, 1, 1, 8, ?, '', 'Scheduled', '')`, time.Date(2026, 6, 2, 18, 0, 0, 0, time.UTC))

	svc := NewService(db, "", 1, "")

	// Wrong year (the year-8 fat-finger) must be rejected and the fixture left as-is.
	badDate := time.Date(8, 9, 22, 14, 0, 0, 0, time.UTC)
	if err := svc.UpdateFixtureSchedule(900, badDate, models.OtherReason, ""); err == nil {
		t.Fatalf("expected an error rescheduling to year 8, got nil")
	}
	fx, err := svc.fixtureRepository.FindByID(ctx, 900)
	if err != nil {
		t.Fatalf("reload fixture: %v", err)
	}
	if fx.ScheduledDate.Year() != 2026 || fx.Status != models.Scheduled {
		t.Fatalf("after rejected reschedule: got year=%d status=%s, want year=2026 status=Scheduled (fixture must be untouched)",
			fx.ScheduledDate.Year(), fx.Status)
	}

	// A valid same-year date within the season is accepted and marks the fixture Rescheduled.
	goodDate := time.Date(2026, 9, 22, 14, 0, 0, 0, time.UTC)
	if err := svc.UpdateFixtureSchedule(900, goodDate, models.OtherReason, ""); err != nil {
		t.Fatalf("valid same-year reschedule failed: %v", err)
	}
	fx, err = svc.fixtureRepository.FindByID(ctx, 900)
	if err != nil {
		t.Fatalf("reload fixture: %v", err)
	}
	if fx.Status != models.Rescheduled || !fx.ScheduledDate.Equal(goodDate) {
		t.Fatalf("after valid reschedule: got status=%s date=%v, want status=Rescheduled date=%v",
			fx.Status, fx.ScheduledDate, goodDate)
	}
}
