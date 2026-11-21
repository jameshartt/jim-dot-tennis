// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package main

import (
	"context"
	"strings"
	"testing"

	"jim-dot-tennis/internal/accounts"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
)

// TestCLIAdminPasskeys: the operator can see and change "Require passkeys
// for administrators"; "off" always works (the way back in for a club whose
// administrators cannot use their passkeys), "on" only when every active
// administrator has one.
func TestCLIAdminPasskeys(t *testing.T) {
	db := cliTestDB(t)
	ctx := context.Background()
	svc := accounts.New(db)
	_, link, err := svc.Invite(ctx, accounts.CLIActor("test"), "alice", models.RoleAdmin, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RedeemLink(ctx, link.Token, accounts.Redemption{Password: "a long enough secret"}); err != nil {
		t.Fatal(err)
	}

	r := runCLITest(t, "admin-passkeys", "on")
	if r.code != exitError || !strings.Contains(r.stderr, "alice") {
		t.Fatalf("on while alice has no passkey: %+v", r)
	}
	if on, _ := repository.AdminPasskeysRequired(ctx, db); on {
		t.Fatal("turned on although refused")
	}
	if r := runCLITest(t, "admin-passkeys", "status"); r.code != exitOK || !strings.Contains(r.stdout, "may sign in with a passkey or a password") || !strings.Contains(r.stdout, "alice") {
		t.Fatalf("status: %+v", r)
	}
	alice := loadUser(t, db, "alice")
	if _, err := db.Exec(`INSERT INTO webauthn_credentials (user_id, credential_id, public_key, name) VALUES (?, x'0102', x'00', 'k')`, alice.ID); err != nil {
		t.Fatal(err)
	}
	if r := runCLITest(t, "admin-passkeys", "on"); r.code != exitOK || !strings.Contains(r.stdout, "must sign in with a passkey") || !strings.Contains(r.stderr, "action=settings.admin_passkeys") {
		t.Fatalf("on: %+v", r)
	}
	if r := runCLITest(t, "admin-passkeys", "off"); r.code != exitOK || !strings.Contains(r.stdout, "may sign in with a passkey or a password") {
		t.Fatalf("off: %+v", r)
	}
	if on, _ := repository.AdminPasskeysRequired(ctx, db); on {
		t.Fatal("still on after off")
	}
	if r := runCLITest(t, "admin-passkeys", "maybe"); r.code != exitUsage {
		t.Fatalf("bad word: %+v", r)
	}
}
