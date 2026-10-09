// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"

	_ "github.com/mattn/go-sqlite3"
)

// Player and team names arrive from match-card imports and admin forms, so
// the HTML the players filter builds in Go must escape them.
func TestPlayersFilterEscapesNames(t *testing.T) {
	db, err := database.New(database.Config{Driver: "sqlite3", FilePath: filepath.Join(t.TempDir(), "filter.db")})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.ExecuteMigrations(findMigrationsPathAdmin(t)); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	exec := func(q string, args ...interface{}) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("seed %q: %v", q, err)
		}
	}
	start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	exec(`INSERT INTO seasons (id, name, year, start_date, end_date, is_active) VALUES (1, '2026', 2026, ?, ?, 1)`, start, start.AddDate(0, 6, 0))
	exec(`INSERT INTO leagues (id, name, type, year, region) VALUES (1, 'Parks', 'Parks', 2026, 'Brighton')`)
	exec(`INSERT INTO divisions (id, name, level, play_day, league_id, season_id) VALUES (1, 'Div <i>1</i>', 1, 'Tuesday', 1, 1)`)
	exec(`INSERT INTO clubs (id, name) VALUES (1, 'St Ann''s')`)
	exec(`INSERT INTO teams (id, name, club_id, division_id, season_id) VALUES (1, 'Team <b>A</b>', 1, 1, 1)`)
	exec(`INSERT INTO players (id, first_name, last_name, club_id, is_active) VALUES
		('p-1', '<img src=x onerror=alert(1)>', 'O''Brien "Bob"', 1, 1)`)

	h := NewPlayersHandler(NewService(db, "", 1, ""), "")
	filter := func(query string) string {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/admin/league/players/filter?"+query, nil)
		req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, models.User{Username: "admin", Role: "admin"}))
		rec := httptest.NewRecorder()
		h.HandlePlayersFilter(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
		}
		return rec.Body.String()
	}

	// Player rows (plain list) and dynamic team/division column headers.
	body := filter("status=all") + filter("team_id=1&division_id=1")
	if !strings.Contains(body, "&lt;img src=x") {
		t.Fatalf("player row missing or unescaped:\n%s", body)
	}
	for _, raw := range []string{"<img", "<b>A</b>", "<i>1</i>", `"Bob"`} {
		if strings.Contains(body, raw) {
			t.Errorf("raw %q in filter HTML", raw)
		}
	}
}

func TestAddPlayersTableBodyEscapesNames(t *testing.T) {
	rec := httptest.NewRecorder()
	(&TeamsHandler{}).renderAddPlayersTableBody(rec, []models.Player{
		{ID: `p"1`, FirstName: "<script>alert(1)</script>", LastName: `O'Brien "Bob"`},
	})
	body := rec.Body.String()
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Fatalf("player row missing or unescaped:\n%s", body)
	}
	for _, raw := range []string{"<script>", `"Bob"`, `p"1`} {
		if strings.Contains(body, raw) {
			t.Errorf("raw %q in add-players HTML", raw)
		}
	}
}
