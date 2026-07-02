// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
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

// insertSession writes a session row directly so its timestamps can be forged.
func insertSession(t *testing.T, s *Service, id string, createdAt time.Time, expiresAt time.Time) {
	t.Helper()
	_, err := s.db.Exec(`
		INSERT INTO sessions (id, user_id, role, created_at, expires_at, last_activity_at, ip, user_agent, device_info, is_valid)
		VALUES (?, 1, 'admin', ?, ?, ?, '127.0.0.1', 'test', '{}', true)
	`, id, createdAt, expiresAt, createdAt)
	if err != nil {
		t.Fatalf("insert session %s: %v", id, err)
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

func TestRedactToken(t *testing.T) {
	const secret = "super-secret-session-id-value-1234567890"

	got := redactToken(secret)

	// The fingerprint must never contain the raw token (that is the whole point).
	if strings.Contains(got, secret) {
		t.Fatalf("redactToken leaked the raw token: %q", got)
	}
	if strings.Contains(secret, got) {
		t.Fatalf("redactToken output is a prefix of the token: %q", got)
	}

	// It must be deterministic so log lines correlate across a session.
	if again := redactToken(secret); again != got {
		t.Fatalf("redactToken not deterministic: %q != %q", got, again)
	}

	// Different tokens must produce different fingerprints.
	if other := redactToken(secret + "x"); other == got {
		t.Fatal("redactToken collided for distinct tokens")
	}

	if !strings.HasPrefix(got, "sha256:") {
		t.Fatalf("expected sha256: prefix, got %q", got)
	}

	if empty := redactToken(""); empty != "<empty>" {
		t.Fatalf("expected <empty> for empty token, got %q", empty)
	}
}
