// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"os"
	"path/filepath"
	"testing"

	"jim-dot-tennis/internal/models"
)

// The passkey migration's down file removes exactly what the up file added,
// with data present, and the up file can be applied again afterwards.
func TestPasskeyMigrationDownAndUp(t *testing.T) {
	env := newPasskeyEnv(t)
	_, _, userID := env.enrolledUser("alice", false)
	db := env.svc.db

	tables := func() int {
		return env.count(`SELECT COUNT(*) FROM sqlite_master WHERE name IN
			('webauthn_credentials', 'webauthn_user_handles', 'idx_webauthn_credentials_user')`)
	}
	if tables() != 3 {
		t.Fatalf("after up: %d of 3 objects", tables())
	}
	read := func(name string) string {
		data, err := os.ReadFile(filepath.Join(findMigrationsPath(t), name))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	if _, err := db.Exec(read("033_webauthn_credentials.down.sql")); err != nil {
		t.Fatalf("down: %v", err)
	}
	if tables() != 0 {
		t.Fatalf("after down: %d objects left", tables())
	}
	// The account itself is untouched and can still sign in by password.
	var user models.User
	if err := db.Get(&user, `SELECT * FROM users WHERE id = ?`, userID); err != nil || user.Username != "alice" {
		t.Fatalf("user after down: %+v, %v", user, err)
	}
	env.browser("198.51.100.1").passwordSignIn("alice")

	if _, err := db.Exec(read("033_webauthn_credentials.up.sql")); err != nil {
		t.Fatalf("up again: %v", err)
	}
	if tables() != 3 {
		t.Fatalf("after up again: %d of 3 objects", tables())
	}
}

// Deleting an account removes its passkeys and handle with it.
func TestPasskeysAreDeletedWithTheAccount(t *testing.T) {
	env := newPasskeyEnv(t)
	_, _, userID := env.enrolledUser("alice", false)
	if _, err := env.svc.db.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.db.Exec(`DELETE FROM users WHERE id = ?`, userID); err != nil {
		t.Fatal(err)
	}
	if n := env.count(`SELECT (SELECT COUNT(*) FROM webauthn_credentials) + (SELECT COUNT(*) FROM webauthn_user_handles)`); n != 0 {
		t.Errorf("%d passkey rows left after deleting the account", n)
	}
}
