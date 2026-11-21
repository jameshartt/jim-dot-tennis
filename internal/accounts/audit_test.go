// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package accounts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/models"
)

// auditRows returns every audit entry as "actor_kind|actor|action|target|detail".
func auditRows(t *testing.T, s *Service) []string {
	t.Helper()
	var rows []string
	if err := s.db.Select(&rows, `SELECT actor_kind || '|' || actor || '|' || action || '|' || target_username || '|' || detail
		FROM account_audit_log ORDER BY id`); err != nil {
		t.Fatal(err)
	}
	return rows
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// Every account change writes an audit row, and no row holds a password, a
// link token, a session token, or a hash of any of them.
func TestAuditRowsAreWrittenAndHoldNoSecrets(t *testing.T) {
	_ = captureLog(t)
	ctx := context.Background()
	s, db := newTestService(t)
	admin := mustCreate(t, s, "ann", models.RoleAdmin)
	actor := UserActor(admin.ID, admin.Username)

	var secrets []string
	_, invite, err := s.Invite(ctx, actor, "bob", models.RoleCaptain, nil)
	if err != nil {
		t.Fatal(err)
	}
	secrets = append(secrets, invite.Token, sha256Hex(invite.Token))
	const bobPassword = "first secret phrase here"
	if _, err := s.RedeemLink(ctx, invite.Token, Redemption{Password: bobPassword}); err != nil {
		t.Fatal(err)
	}
	secrets = append(secrets, bobPassword, passwordHash(t, db, invite.UserID))
	const newPassword = "second secret phrase here"
	if _, err := s.SetOwnPassword(ctx, invite.UserID, PasswordChange{Current: bobPassword, New: newPassword}); err != nil {
		t.Fatal(err)
	}
	secrets = append(secrets, newPassword, passwordHash(t, db, invite.UserID))
	recovery, err := s.IssueLink(ctx, actor, invite.UserID, LinkRecovery)
	if err != nil {
		t.Fatal(err)
	}
	secrets = append(secrets, recovery.Token, sha256Hex(recovery.Token))
	if _, err := s.RevokeLink(ctx, actor, recovery.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetRole(ctx, actor, invite.UserID, models.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetActive(ctx, CLIActor("user disable"), invite.UserID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetActive(ctx, CLIActor("user enable"), invite.UserID, true); err != nil {
		t.Fatal(err)
	}
	const sessionToken = "a-session-token-for-the-test-0123456789"
	addSession(t, db, sessionToken, invite.UserID)
	secrets = append(secrets, sessionToken, auth.HashSessionToken(sessionToken))
	var sessionID int64
	if err := db.Get(&sessionID, `SELECT id FROM sessions WHERE token_hash = ?`, auth.HashSessionToken(sessionToken)); err != nil {
		t.Fatal(err)
	}
	if err := s.RevokeSession(ctx, actor, sessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RevokeSessions(ctx, CLIActor("user sessions revoke"), invite.UserID); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordSignIn(ctx, admin.ID, admin.Username, auth.SignInPasskey, "203.0.113.7"); err != nil {
		t.Fatal(err)
	}

	rows := auditRows(t, s)
	all := strings.Join(rows, "\n")
	for _, want := range []string{
		"user|ann|user.create|bob|",
		"user|ann|link.create|bob|",
		"|link.use|bob|",
		"user|bob|user.change_password|bob|",
		"user|ann|link.revoke|bob|",
		"user|ann|user.role_change|bob|role captain -> admin",
		"cli|user disable|user.disable|bob|",
		"cli|user enable|user.enable|bob|",
		"user|ann|session.revoke|bob|",
		"cli|user sessions revoke|user.sessions_revoke|bob|",
		"user|ann|user.sign_in|ann|method passkey from 203.0.113.7",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("no audit row like %q in:\n%s", want, all)
		}
	}
	for _, secret := range secrets {
		if secret != "" && strings.Contains(all, secret) {
			t.Errorf("an audit row contains a secret (%d chars)", len(secret))
		}
	}
	// No session id either: the revoked session's row has no detail at all.
	for _, row := range rows {
		if strings.Contains(row, "|session.revoke|") && !strings.HasSuffix(row, "|bob|") {
			t.Errorf("session.revoke row carries a detail (session id %d?): %q", sessionID, row)
		}
	}

	// Paging: newest first.
	page, total, err := s.AuditPage(ctx, 2, 0)
	if err != nil || total != len(rows) || len(page) != 2 || page[0].Action != ActionSignIn || page[0].ID < page[1].ID {
		t.Fatalf("first page: %+v total %d (want %d), %v", page, total, len(rows), err)
	}
	if page[0].ActorUserID == nil || *page[0].ActorUserID != admin.ID || page[0].TargetUserID == nil || *page[0].TargetUserID != admin.ID {
		t.Errorf("sign-in entry user ids: %+v", page[0])
	}
	last, _, err := s.AuditPage(ctx, 2, total-1)
	if err != nil || len(last) != 1 || last[0].Action != ActionCreate {
		t.Fatalf("last page: %+v %v", last, err)
	}
	cliEntries, _, _ := s.AuditPage(ctx, total, 0)
	for _, e := range cliEntries {
		if e.ActorKind == "cli" && e.ActorUserID != nil {
			t.Errorf("command line entry with a user id: %+v", e)
		}
	}
}

// The audit row is written in the transaction that made the change: a change
// that is refused leaves no row, and a failed audit write undoes the change.
func TestAuditRowIsPartOfTheChange(t *testing.T) {
	_ = captureLog(t)
	ctx := context.Background()
	s, db := newTestService(t)
	admin := mustCreate(t, s, "ann", models.RoleAdmin)
	captain := mustCreate(t, s, "cap", models.RoleCaptain)
	before := len(auditRows(t, s))

	// Refused (last administrator): no change, no row.
	if _, err := s.SetActive(ctx, cli, admin.ID, false); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("disabling the last admin: %v", err)
	}
	if n := len(auditRows(t, s)); n != before {
		t.Errorf("%d audit rows after a refused change, want %d", n, before)
	}

	// The audit insert fails: the role change is rolled back with it.
	if _, err := db.Exec(`CREATE TRIGGER fail_audit BEFORE INSERT ON account_audit_log BEGIN SELECT RAISE(ABORT, 'audit down'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetRole(ctx, cli, captain.ID, models.RoleAdmin); err == nil {
		t.Fatal("SetRole succeeded although its audit record could not be written")
	}
	if a, _ := s.Get(ctx, captain.ID); a.Role != models.RoleCaptain {
		t.Errorf("role is %s after a failed audit write, want captain", a.Role)
	}
	if _, err := db.Exec(`DROP TRIGGER fail_audit`); err != nil {
		t.Fatal(err)
	}
}

// Entries cannot be changed or deleted, even directly in the database.
func TestAuditLogIsAppendOnly(t *testing.T) {
	_ = captureLog(t)
	s, db := newTestService(t)
	mustCreate(t, s, "ann", models.RoleAdmin)
	if _, err := db.Exec(`UPDATE account_audit_log SET detail = 'edited'`); err == nil {
		t.Error("an audit entry could be updated")
	}
	if _, err := db.Exec(`DELETE FROM account_audit_log`); err == nil {
		t.Error("an audit entry could be deleted")
	}
	if len(auditRows(t, s)) == 0 {
		t.Fatal("no audit rows")
	}
}
