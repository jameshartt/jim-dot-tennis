// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package accounts

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

// Action names recorded in the audit trail.
const (
	ActionCreate          = "user.create"
	ActionChangePassword  = "user.change_password" // the user chose their own password
	ActionDisable         = "user.disable"         // account deactivated
	ActionEnable          = "user.enable"          // account reactivated
	ActionRoleChange      = "user.role_change"     // role changed
	ActionRevokeSessions  = "user.sessions_revoke" // every session of the user ended
	ActionRevokeSession   = "session.revoke"       // one session ended
	ActionCleanupSessions = "sessions.cleanup"     // expired sessions marked invalid
	ActionSetupCode       = "setup.code_issue"     // a new first-run setup code was issued
	ActionLinkCreate      = "link.create"          // an invite or recovery link was issued
	ActionLinkRevoke      = "link.revoke"          // an unused link was cancelled
	ActionLinkUse         = "link.use"             // a link was used: a sign-in method was set
	ActionSignIn          = "user.sign_in"         // an administrator signed in (detail: method, address)
)

// ActorKind says where an audited action came from.
type ActorKind string

const (
	// ActorCLI is the command line on the server (`jim-dot-tennis user ...`).
	ActorCLI ActorKind = "cli"
	// ActorUser is a signed-in user acting through the web interface.
	ActorUser ActorKind = "user"
)

// Actor is who (or what) performed an action.
type Actor struct {
	Kind     ActorKind
	UserID   int64  // ActorUser only
	Username string // ActorUser only
	Command  string // ActorCLI: the subcommand, e.g. "user reset"
}

// CLIActor is the server command line running command.
func CLIActor(command string) Actor { return Actor{Kind: ActorCLI, Command: command} }

// UserActor is the signed-in user id/username acting in the web interface.
func UserActor(id int64, username string) Actor {
	return Actor{Kind: ActorUser, UserID: id, Username: username}
}

// String renders the actor for the audit record.
func (a Actor) String() string {
	switch a.Kind {
	case ActorCLI:
		return "cli:" + a.Command
	case ActorUser:
		return fmt.Sprintf("user:%s(#%d)", a.Username, a.UserID)
	default:
		return "unknown"
	}
}

// Record is one audited account action. It never carries a secret: no
// password, temporary password, setup code, session token or link token.
type Record struct {
	Time           time.Time
	Actor          Actor
	Action         string
	TargetUserID   int64  // 0 when the action has no single target user
	TargetUsername string // "" when the action has no single target user
	Detail         string // short, non-secret context such as "role captain -> admin"
}

// recordAudit is the single place an account action is recorded. It is called
// inside the transaction that made the change (db is that *sqlx.Tx) and
// inserts one row into account_audit_log (migration 035) on it, so the entry
// is committed with the change it describes, and a failed insert fails the
// transaction and so the change. It also writes the entry as one log line
// (for a command run on the server that line goes to the operator's terminal,
// stderr). Neither ever holds a secret: see Record.
func recordAudit(ctx context.Context, db sqlx.ExtContext, rec Record) error {
	if rec.Time.IsZero() {
		rec.Time = time.Now()
	}
	var actorUserID, targetUserID any
	if rec.Actor.Kind == ActorUser {
		actorUserID = rec.Actor.UserID
	}
	if rec.TargetUserID != 0 {
		targetUserID = rec.TargetUserID
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO account_audit_log
			(created_at, actor_kind, actor_user_id, actor, action, target_user_id, target_username, detail)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.Time, rec.Actor.kind(), actorUserID, rec.Actor.name(), rec.Action,
		targetUserID, rec.TargetUsername, rec.Detail); err != nil {
		return fmt.Errorf("record %s in the audit log: %w", rec.Action, err)
	}
	log.Print(formatRecord(rec))
	return nil
}

// kind is the actor_kind column value.
func (a Actor) kind() string {
	switch a.Kind {
	case ActorCLI, ActorUser:
		return string(a.Kind)
	default:
		return "system"
	}
}

// name is the actor column value: the username, or the command line command.
func (a Actor) name() string {
	switch a.Kind {
	case ActorCLI:
		return a.Command
	case ActorUser:
		return a.Username
	default:
		return ""
	}
}

// RecordSignIn records that userID (username) signed in with method from
// address. The server records administrators' sign-ins only (see
// auth.Service.SetSignInAudit).
func (s *Service) RecordSignIn(ctx context.Context, userID int64, username, method, address string) error {
	return recordAudit(ctx, s.db, Record{Time: s.now(), Actor: UserActor(userID, username), Action: ActionSignIn,
		TargetUserID: userID, TargetUsername: username, Detail: "method " + method + " from " + address})
}

// AuditEntry is one row of the account audit log as the admin page shows it.
type AuditEntry struct {
	ID             int64     `db:"id"`
	CreatedAt      time.Time `db:"created_at"`
	ActorKind      string    `db:"actor_kind"`
	ActorUserID    *int64    `db:"actor_user_id"`
	Actor          string    `db:"actor"`
	Action         string    `db:"action"`
	TargetUserID   *int64    `db:"target_user_id"`
	TargetUsername string    `db:"target_username"`
	Detail         string    `db:"detail"`
}

// AuditPage returns up to limit audit entries, newest first, skipping the
// newest offset, and the total number of entries. Entries are read-only:
// there is no way to change or delete one through the service (the table
// refuses UPDATE and DELETE too).
func (s *Service) AuditPage(ctx context.Context, limit, offset int) ([]AuditEntry, int, error) {
	var total int
	if err := s.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM account_audit_log`); err != nil {
		return nil, 0, fmt.Errorf("count audit entries: %w", err)
	}
	var out []AuditEntry
	if err := s.db.SelectContext(ctx, &out, `
		SELECT id, created_at, actor_kind, actor_user_id, actor, action, target_user_id, target_username, detail
		FROM account_audit_log
		ORDER BY id DESC
		LIMIT ? OFFSET ?`, limit, offset); err != nil {
		return nil, 0, fmt.Errorf("list audit entries: %w", err)
	}
	return out, total, nil
}

// formatRecord renders rec as a single key=value log line.
func formatRecord(rec Record) string {
	var b strings.Builder
	fmt.Fprintf(&b, "audit: time=%s actor=%q action=%s", rec.Time.UTC().Format(time.RFC3339), rec.Actor.String(), rec.Action)
	if rec.TargetUserID != 0 || rec.TargetUsername != "" {
		fmt.Fprintf(&b, " target_id=%d target=%q", rec.TargetUserID, rec.TargetUsername)
	}
	if rec.Detail != "" {
		fmt.Fprintf(&b, " detail=%q", rec.Detail)
	}
	return b.String()
}
