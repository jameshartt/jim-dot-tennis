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
	ActionResetPassword   = "user.reset_password"  // temporary password issued (CLI)
	ActionSetPassword     = "user.set_password"    // password typed by an administrator
	ActionChangePassword  = "user.change_password" // the user chose their own password
	ActionDisable         = "user.disable"         // account deactivated
	ActionEnable          = "user.enable"          // account reactivated
	ActionRoleChange      = "user.role_change"     // role changed
	ActionRevokeSessions  = "user.sessions_revoke" // every session of the user ended
	ActionRevokeSession   = "session.revoke"       // one session ended
	ActionCleanupSessions = "sessions.cleanup"     // expired sessions marked invalid
	ActionSetupCode       = "setup.code_issue"     // a new first-run setup code was issued
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
// inside the transaction that made the change (db is that *sqlx.Tx), so a
// later audit table (WI-131) can be written atomically with the change it
// describes and a failed write can abort it.
//
// Today it writes one structured log line. For a command run on the server
// that line goes to the operator's terminal (stderr), not to the server log.
func recordAudit(_ context.Context, _ sqlx.ExtContext, rec Record) error {
	log.Print(formatRecord(rec))
	return nil
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
