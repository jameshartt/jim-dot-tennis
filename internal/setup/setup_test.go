// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package setup

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"jim-dot-tennis/internal/config"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/repository"

	_ "github.com/mattn/go-sqlite3"
)

// findRepoDir walks up from the package directory to find a top-level directory.
func findRepoDir(t *testing.T, name string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 6; i++ {
		candidate := filepath.Join(dir, name)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		dir = filepath.Dir(dir)
	}
	t.Fatalf("could not locate %s directory", name)
	return ""
}

// newTestState returns a setup state over a freshly migrated database with no
// HOME_CLUB_* variables and no users or clubs.
func newTestState(t *testing.T) (*State, *database.DB) {
	t.Helper()
	t.Setenv("HOME_CLUB_ID", "")
	t.Setenv("HOME_CLUB_NAME", "")
	db, err := database.New(database.Config{Driver: "sqlite3", FilePath: filepath.Join(t.TempDir(), "test.db")})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.ExecuteMigrations(findRepoDir(t, "migrations")); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	var users int
	if err := db.Get(&users, `SELECT COUNT(*) FROM users`); err != nil || users != 0 {
		t.Fatalf("fresh database has %d user accounts (err %v), want none", users, err)
	}
	holder, err := config.NewHomeClubHolder(context.Background(), repository.NewClubRepository(db), repository.NewSettingsRepository(db))
	if err != nil {
		t.Fatal(err)
	}
	return NewState(db, holder), db
}

func TestStatusTransitions(t *testing.T) {
	ctx := context.Background()
	s, db := newTestState(t)

	if got := s.Status(ctx); got != StatusNeedsSetup {
		t.Fatalf("empty database: status %v, want needs-setup", got)
	}

	if _, err := db.Exec(`INSERT INTO users (username, password_hash, role) VALUES ('first', 'x', 'admin')`); err != nil {
		t.Fatal(err)
	}
	if got := s.Status(ctx); got != StatusNeedsClub {
		t.Fatalf("admin but no club: status %v, want needs-club", got)
	}

	if _, err := db.Exec(`INSERT INTO clubs (id, name) VALUES (3, 'Chosen Club')`); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkComplete(ctx, 3); err != nil {
		t.Fatalf("MarkComplete: %v", err)
	}
	if got := s.Status(ctx); got != StatusReady {
		t.Fatalf("after MarkComplete: status %v, want ready", got)
	}
	if s.HomeClub().ID() != 3 || s.HomeClub().Source() != config.SourceStored {
		t.Fatalf("home club after MarkComplete: id %d source %q", s.HomeClub().ID(), s.HomeClub().Source())
	}
	if _, ok, err := repository.NewSettingsRepository(db).GetTime(ctx, repository.SettingSetupCompletedAt); err != nil || !ok {
		t.Fatalf("setup_completed_at: ok %v err %v", ok, err)
	}
}

func TestMarkCompleteRejectsUnknownClub(t *testing.T) {
	ctx := context.Background()
	s, db := newTestState(t)
	if err := s.MarkComplete(ctx, 77); err == nil {
		t.Fatal("MarkComplete with a missing club: want error")
	}
	if _, ok, _ := repository.NewSettingsRepository(db).GetUint(ctx, repository.SettingHomeClubID); ok {
		t.Fatal("home_club_id written despite the error")
	}
	if s.Status(ctx) == StatusReady {
		t.Fatal("status ready after failed MarkComplete")
	}
}

func TestWriteCompletionInCallerTransaction(t *testing.T) {
	ctx := context.Background()
	s, db := newTestState(t)
	if _, err := db.Exec(`INSERT INTO clubs (id, name) VALUES (5, 'Club')`); err != nil {
		t.Fatal(err)
	}

	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO users (username, password_hash, role) VALUES ('first', 'x', 'admin')`); err != nil {
		t.Fatal(err)
	}
	if err := WriteCompletion(ctx, tx, 5); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := s.Activate(ctx); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if s.Status(ctx) != StatusReady || s.HomeClub().ID() != 5 {
		t.Fatalf("after Activate: status %v id %d", s.Status(ctx), s.HomeClub().ID())
	}
}

func TestGateRoutesToSetupUntilComplete(t *testing.T) {
	ctx := context.Background()
	s, db := newTestState(t)
	reached := false
	gated := s.Gate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
	}))

	cases := []struct {
		path    string
		allowed bool
	}{
		{"/", false},
		{"/login", false},
		{"/admin/league/dashboard", false},
		{"/setupx", false},
		{"/healthz", true},
		{"/setup", true},
		{"/setup/admin", true},
		{"/static/css/main.css", true},
		{"/branding/logo-0123abcd.png", true},
		{"/brandingx", false},
	}
	for _, c := range cases {
		reached = false
		rec := httptest.NewRecorder()
		gated.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, c.path, nil))
		if reached != c.allowed {
			t.Errorf("%s: reached=%v, want %v", c.path, reached, c.allowed)
		}
		if !c.allowed && (rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != Path) {
			t.Errorf("%s: got %d Location %q, want 303 to %s", c.path, rec.Code, rec.Header().Get("Location"), Path)
		}
	}

	// With an administrator but no club, sign-in stays reachable so the
	// administrator can finish the club step.
	if _, err := db.Exec(`INSERT INTO users (username, password_hash, role) VALUES ('first', 'x', 'admin')`); err != nil {
		t.Fatal(err)
	}
	for path, allowed := range map[string]bool{"/login": true, "/logout": true, "/admin/league/dashboard": false} {
		reached = false
		gated.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
		if reached != allowed {
			t.Errorf("needs-club %s: reached=%v, want %v", path, reached, allowed)
		}
	}

	// HTMX requests are told to navigate rather than given a redirect to swap in.
	req := httptest.NewRequest(http.MethodPost, "/admin/league/players", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	gated.ServeHTTP(rec, req)
	if rec.Header().Get("HX-Redirect") != Path {
		t.Fatalf("HTMX: HX-Redirect = %q", rec.Header().Get("HX-Redirect"))
	}

	// Once complete, everything passes through.
	if _, err := db.Exec(`INSERT INTO clubs (id, name) VALUES (1, 'Club')`); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkComplete(ctx, 1); err != nil {
		t.Fatal(err)
	}
	reached = false
	gated.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/admin/league/dashboard", nil))
	if !reached {
		t.Fatal("after setup: request did not reach the app")
	}
}
