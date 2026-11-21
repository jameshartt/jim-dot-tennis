// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"jim-dot-tennis/internal/database"

	_ "github.com/mattn/go-sqlite3"
)

// findMigrationsPath walks up from the test's working directory to locate the
// repo-root migrations directory.
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

// newTestService boots a fresh migrated SQLite DB, seeds an admin user, and
// returns an auth Service wired to it.
func newTestService(t *testing.T, config Config) *Service {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "auth_test.db")
	db, err := database.New(database.Config{Driver: "sqlite3", FilePath: dbPath})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.ExecuteMigrations(findMigrationsPath(t)); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Ensure a user with id=1 exists for the session FK (migrations seed a
	// default admin at id=1; this is a no-op if so, or backfills otherwise).
	if _, err := db.Exec(`INSERT OR IGNORE INTO users (id, username, password_hash, role) VALUES (1, 'test-fk-user', 'x', 'admin')`); err != nil {
		t.Fatalf("ensure user: %v", err)
	}
	return NewService(db, config)
}

// insertSession writes a session row directly so its timestamps can be
// forged. token is the cookie value; only its hash is stored.
func insertSession(t *testing.T, s *Service, token string, createdAt time.Time, expiresAt time.Time) {
	t.Helper()
	_, err := s.db.Exec(`
		INSERT INTO sessions (token_hash, user_id, role, created_at, expires_at, last_activity_at, ip, user_agent, device_info, is_valid)
		VALUES (?, 1, 'admin', ?, ?, ?, '127.0.0.1', 'test', '{}', true)
	`, HashSessionToken(token), createdAt, expiresAt, createdAt)
	if err != nil {
		t.Fatalf("insert session %s: %v", token, err)
	}
}

func TestValidateSessionAbsoluteLifetimeCap(t *testing.T) {
	cfg := DefaultConfig() // 7d sliding, 30d absolute
	s := newTestService(t, cfg)
	r := httptest.NewRequest("GET", "/admin", nil)
	now := time.Now()

	// A session created 40 days ago but still inside its sliding window: the
	// sliding check would let it through, so only the absolute cap can kill it.
	insertSession(t, s, "old-active", now.Add(-40*24*time.Hour), now.Add(1*time.Hour))
	if _, err := s.ValidateSession("old-active", r); err != ErrSessionExpired {
		t.Fatalf("expected ErrSessionExpired for session past absolute cap, got %v", err)
	}

	// A freshly created session inside both windows must still validate.
	insertSession(t, s, "fresh", now.Add(-1*time.Hour), now.Add(1*time.Hour))
	if _, err := s.ValidateSession("fresh", r); err != nil {
		t.Fatalf("expected fresh session to validate, got %v", err)
	}

	// With the absolute cap disabled (0), the old-but-active session survives.
	cfgNoCap := DefaultConfig()
	cfgNoCap.AbsoluteSessionDuration = 0
	s2 := newTestService(t, cfgNoCap)
	insertSession(t, s2, "old-active", now.Add(-40*24*time.Hour), now.Add(1*time.Hour))
	if _, err := s2.ValidateSession("old-active", r); err != nil {
		t.Fatalf("expected old session to validate with cap disabled, got %v", err)
	}
}

// A session is stored under the SHA-256 of its cookie token: the database
// holds nothing that works as a cookie, and the hash itself does not either.
func TestSessionsAreStoredHashed(t *testing.T) {
	s := newTestService(t, DefaultConfig())
	w := httptest.NewRecorder()
	if err := s.StartSession(w, httptest.NewRequest("POST", "/setup/admin", nil), 1); err != nil {
		t.Fatal(err)
	}
	var token string
	for _, c := range w.Result().Cookies() {
		if c.Name == "session_token" {
			token = c.Value
		}
	}
	if len(token) < 40 {
		t.Fatalf("session cookie %q, want a long random token", token)
	}

	var row struct {
		ID        int64  `db:"id"`
		TokenHash string `db:"token_hash"`
	}
	if err := s.db.Get(&row, `SELECT id, token_hash FROM sessions`); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(token))
	if row.TokenHash != hex.EncodeToString(sum[:]) {
		t.Fatalf("token_hash %q is not the SHA-256 of the cookie token", row.TokenHash)
	}
	// No column of any session row holds the token.
	var leaks int
	if err := s.db.Get(&leaks, `SELECT COUNT(*) FROM sessions WHERE instr(id || token_hash || ip || user_agent || device_info, ?) > 0`, token); err != nil {
		t.Fatal(err)
	}
	if leaks != 0 {
		t.Fatal("the raw session token is stored in the sessions table")
	}

	r := httptest.NewRequest("GET", "/admin/league", nil)
	session, err := s.ValidateSession(token, r)
	if err != nil || session.ID != row.ID {
		t.Fatalf("lookup by cookie token: %+v, %v", session, err)
	}
	for _, guess := range []string{row.TokenHash, strings.ToUpper(row.TokenHash), "", fmt.Sprint(row.ID)} {
		if _, err := s.ValidateSession(guess, r); err == nil {
			t.Fatalf("ValidateSession(%q) succeeded; only the cookie token may", guess)
		}
	}
	if err := s.InvalidateSession(token); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ValidateSession(token, r); err == nil {
		t.Fatal("session still valid after logout")
	}
}
