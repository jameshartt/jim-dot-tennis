// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

// Package accounts is the one place account operations happen: create, list,
// enable/disable, role change, invite and recovery links (links.go) and
// session revocation. Nobody but the account's owner ever sets or sees its
// password. The
// admin users and sessions pages and the server command line
// (`jim-dot-tennis user ...`) both call it, so the rules - above all "never
// leave the instance without an active administrator" - hold whichever way an
// account is changed, and every change is recorded through recordAudit.
package accounts

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
)

// Errors returned by the service. Their messages are written to be shown to
// the person who asked for the change.
var (
	ErrNotFound      = errors.New("no account with that username")
	ErrUsernameTaken = errors.New("that username is already used by another account")
	ErrInvalidRole   = errors.New("the role must be admin or captain")
	// ErrLastAdmin: the change would leave no active administrator.
	ErrLastAdmin = errors.New("this is the last active administrator: make another account an active administrator first")
	// ErrWrongPassword: the current password given to ChangeOwnPassword is wrong.
	ErrWrongPassword = errors.New("your current password is not right")
	// ErrNoPassword: ChangeOwnPassword on an account that has no password.
	ErrNoPassword = errors.New("this account has no password to change")
	// ErrPlayerNotFound: the player to link the account to does not exist.
	ErrPlayerNotFound = errors.New("there is no player with that id to link the account to")
	// ErrPlayerLinked: the player is already linked to another account.
	ErrPlayerLinked = errors.New("that player is already linked to another account")
)

// InvalidUsernameError explains why a username was refused.
type InvalidUsernameError struct{ Username string }

func (e *InvalidUsernameError) Error() string {
	return "use 3 to 64 letters or numbers for the username; . _ - and @ are allowed too (no spaces)"
}

// usernamePattern: letters, digits and . _ - @, starting with a letter or
// digit (the same rule as first-run setup).
var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._@-]{2,63}$`)

// ParseRole returns the role named s (case-insensitive) or ErrInvalidRole.
// Only admin and captain can be given to an account: the player role is
// retired (players use link tokens), so no account can be made or changed to
// it. An existing player-role account keeps its role and reaches nothing.
func ParseRole(s string) (models.Role, error) {
	switch r := models.Role(strings.ToLower(strings.TrimSpace(s))); r {
	case models.RoleAdmin, models.RoleCaptain:
		return r, nil
	}
	return "", ErrInvalidRole
}

// Account is an account as shown to administrators and the command line. It
// deliberately has no password hash.
type Account struct {
	ID             int64       `db:"id"`
	Username       string      `db:"username"`
	Role           models.Role `db:"role"`
	PlayerID       *string     `db:"player_id"`
	IsActive       bool        `db:"is_active"`
	CreatedAt      time.Time   `db:"created_at"`
	LastLoginAt    time.Time   `db:"last_login_at"`
	ActiveSessions int         `db:"active_sessions"` // valid, unexpired sessions
	HasPassword    bool        `db:"has_password"`
	Passkeys       int         `db:"passkeys"`
}

// SignInMethods is the number of ways the account can sign in (passkeys plus
// a password). Zero means it is waiting for its invite to be accepted.
func (a Account) SignInMethods() int {
	n := a.Passkeys
	if a.HasPassword {
		n++
	}
	return n
}

// IsActiveAdmin reports whether the account is an active administrator.
func (a Account) IsActiveAdmin() bool { return a.IsActive && a.Role == models.RoleAdmin }

// Service performs account operations on the database.
type Service struct {
	db  *database.DB
	now func() time.Time
}

// New returns the account service over db.
func New(db *database.DB) *Service {
	return &Service{db: db, now: time.Now}
}

const accountColumns = `
	u.id, u.username, u.role, u.player_id, u.is_active,
	u.created_at, u.last_login_at,
	(COALESCE(u.password_hash, '') <> '') AS has_password,
	(SELECT COUNT(*) FROM webauthn_credentials c WHERE c.user_id = u.id) AS passkeys,
	(SELECT COUNT(*) FROM sessions s
	  WHERE s.user_id = u.id AND s.is_valid = TRUE AND s.expires_at > CURRENT_TIMESTAMP) AS active_sessions`

// List returns every account, ordered by username.
func (s *Service) List(ctx context.Context) ([]Account, error) {
	var out []Account
	err := s.db.SelectContext(ctx, &out, `SELECT `+accountColumns+` FROM users u ORDER BY u.username COLLATE NOCASE ASC`)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	return out, nil
}

// Get returns the account with id.
func (s *Service) Get(ctx context.Context, id int64) (Account, error) {
	return getAccount(ctx, s.db, `u.id = ?`, id)
}

// Find returns the account called username: an exact match, otherwise the
// only account whose name matches ignoring case.
func (s *Service) Find(ctx context.Context, username string) (Account, error) {
	username = strings.TrimSpace(username)
	a, err := getAccount(ctx, s.db, `u.username = ?`, username)
	if !errors.Is(err, ErrNotFound) {
		return a, err
	}
	var matches []Account
	if err := s.db.SelectContext(ctx, &matches, `SELECT `+accountColumns+` FROM users u WHERE u.username = ? COLLATE NOCASE`, username); err != nil {
		return Account{}, fmt.Errorf("find account: %w", err)
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	return Account{}, ErrNotFound
}

func getAccount(ctx context.Context, db sqlx.QueryerContext, where string, arg any) (Account, error) {
	var a Account
	err := sqlx.GetContext(ctx, db, &a, `SELECT `+accountColumns+` FROM users u WHERE `+where, arg)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrNotFound
	}
	if err != nil {
		return Account{}, fmt.Errorf("look up account: %w", err)
	}
	return a, nil
}

// createAccount inserts an active account with no password inside tx and
// records it (Invite, in links.go, is the only way an account is made here:
// its owner sets up how they sign in through the invite link). playerID links
// the account to a player record and may be nil.
func (s *Service) createAccount(ctx context.Context, tx *sqlx.Tx, actor Actor, username string, role models.Role, playerID *string) (Account, error) {
	username = strings.TrimSpace(username)
	if !usernamePattern.MatchString(username) {
		return Account{}, &InvalidUsernameError{Username: username}
	}
	if _, err := ParseRole(string(role)); err != nil {
		return Account{}, err
	}
	now := s.now()
	// The insert is the transaction's first statement, so it holds the write
	// lock before the checks below read anything.
	res, err := tx.ExecContext(ctx, `
		INSERT INTO users (username, password_hash, role, player_id, is_active, created_at, last_login_at)
		SELECT ?, NULL, ?, NULL, TRUE, ?, ?
		WHERE NOT EXISTS (SELECT 1 FROM users WHERE username = ? COLLATE NOCASE)`,
		username, role, now, now, username)
	if err != nil {
		return Account{}, fmt.Errorf("create account: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Account{}, ErrUsernameTaken
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Account{}, fmt.Errorf("create account: %w", err)
	}
	detail := "role " + string(role)
	if playerID != nil && *playerID != "" {
		var exists, linked int
		if err := tx.GetContext(ctx, &exists, `SELECT COUNT(*) FROM players WHERE id = ?`, *playerID); err != nil {
			return Account{}, fmt.Errorf("check player: %w", err)
		}
		if exists == 0 {
			return Account{}, ErrPlayerNotFound
		}
		if err := tx.GetContext(ctx, &linked, `SELECT COUNT(*) FROM users WHERE player_id = ?`, *playerID); err != nil {
			return Account{}, fmt.Errorf("check player link: %w", err)
		}
		if linked > 0 {
			return Account{}, ErrPlayerLinked
		}
		if _, err := tx.ExecContext(ctx, `UPDATE users SET player_id = ? WHERE id = ?`, *playerID, id); err != nil {
			return Account{}, fmt.Errorf("link player: %w", err)
		}
		detail += " linked_player=yes"
	}
	created, err := getAccount(ctx, tx, `u.id = ?`, id)
	if err != nil {
		return Account{}, err
	}
	return created, recordAudit(ctx, tx, Record{Time: now, Actor: actor, Action: ActionCreate,
		TargetUserID: id, TargetUsername: username, Detail: detail})
}

// ChangeOwnPassword lets user id replace their password after giving the
// current one. It ends all of the user's sessions (the caller signs the
// browser in again with a fresh session). An account with no password gets
// ErrNoPassword.
func (s *Service) ChangeOwnPassword(ctx context.Context, id int64, current, newPassword string) error {
	var u struct {
		Username     string              `db:"username"`
		PasswordHash models.PasswordHash `db:"password_hash"`
	}
	err := s.db.GetContext(ctx, &u, `SELECT username, password_hash FROM users WHERE id = ? AND is_active = TRUE`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("look up account: %w", err)
	}
	if u.PasswordHash == "" {
		return ErrNoPassword
	}
	if !auth.PasswordMatches(string(u.PasswordHash), current) {
		return ErrWrongPassword
	}
	if current == newPassword {
		return fmt.Errorf("%w: Choose a password different from the current one.", auth.ErrPasswordPolicy)
	}
	if err := auth.ValidatePassword(newPassword, u.Username); err != nil {
		return err
	}
	hash, err := auth.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	return s.inTx(ctx, func(tx *sqlx.Tx) error {
		res, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, hash, id)
		if err != nil {
			return fmt.Errorf("set password: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		ended, err := endSessions(ctx, tx, id)
		if err != nil {
			return err
		}
		return recordAudit(ctx, tx, Record{Time: s.now(), Actor: UserActor(id, u.Username), Action: ActionChangePassword,
			TargetUserID: id, TargetUsername: u.Username, Detail: fmt.Sprintf("sessions_ended=%d", ended)})
	})
}

// SetActive enables or disables account id. Disabling ends all of the
// account's sessions and is refused (ErrLastAdmin) for the last active
// administrator. It returns the account as it is afterwards.
func (s *Service) SetActive(ctx context.Context, actor Actor, id int64, active bool) (Account, error) {
	var after Account
	err := s.inTx(ctx, func(tx *sqlx.Tx) error {
		before, err := getAccount(ctx, tx, `u.id = ?`, id)
		if err != nil {
			return err
		}
		if active {
			if _, err := tx.ExecContext(ctx, `UPDATE users SET is_active = TRUE WHERE id = ?`, id); err != nil {
				return fmt.Errorf("enable account: %w", err)
			}
		} else {
			// One statement, so two processes (server and command line) cannot
			// each disable one of the last two administrators.
			res, err := tx.ExecContext(ctx, `
				UPDATE users SET is_active = FALSE
				WHERE id = ? AND NOT (`+lastActiveAdminCondition+`)`, id, id, id)
			if err != nil {
				return fmt.Errorf("disable account: %w", err)
			}
			if n, _ := res.RowsAffected(); n == 0 {
				return ErrLastAdmin
			}
		}
		ended := int64(0)
		if !active {
			if ended, err = endSessions(ctx, tx, id); err != nil {
				return err
			}
		}
		if after, err = getAccount(ctx, tx, `u.id = ?`, id); err != nil {
			return err
		}
		action, detail := ActionEnable, ""
		if !active {
			action, detail = ActionDisable, fmt.Sprintf("sessions_ended=%d", ended)
		}
		if before.IsActive == active {
			detail = strings.TrimSpace(detail + " (no change)")
		}
		return recordAudit(ctx, tx, Record{Time: s.now(), Actor: actor, Action: action,
			TargetUserID: id, TargetUsername: before.Username, Detail: detail})
	})
	return after, err
}

// SetRole changes account id's role. Demoting the last active administrator
// is refused (ErrLastAdmin). It returns the role the account had before.
func (s *Service) SetRole(ctx context.Context, actor Actor, id int64, role models.Role) (models.Role, error) {
	if _, err := ParseRole(string(role)); err != nil {
		return "", err
	}
	var previous models.Role
	err := s.inTx(ctx, func(tx *sqlx.Tx) error {
		before, err := getAccount(ctx, tx, `u.id = ?`, id)
		if err != nil {
			return err
		}
		previous = before.Role
		query := `UPDATE users SET role = ? WHERE id = ?`
		args := []any{role, id}
		if role != models.RoleAdmin {
			query += ` AND NOT (` + lastActiveAdminCondition + `)`
			args = append(args, id, id)
		}
		res, err := tx.ExecContext(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("change role: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrLastAdmin
		}
		return recordAudit(ctx, tx, Record{Time: s.now(), Actor: actor, Action: ActionRoleChange,
			TargetUserID: id, TargetUsername: before.Username, Detail: fmt.Sprintf("role %s -> %s", before.Role, role)})
	})
	return previous, err
}

// lastActiveAdminCondition is true for the row being updated (bound twice
// with its id) when it is the only active administrator.
const lastActiveAdminCondition = `
	EXISTS (SELECT 1 FROM users WHERE id = ? AND role = 'admin' AND is_active = TRUE)
	AND (SELECT COUNT(*) FROM users WHERE role = 'admin' AND is_active = TRUE AND id <> ?) = 0`

// RevokeSessions ends every session of account id and returns how many were
// still valid.
func (s *Service) RevokeSessions(ctx context.Context, actor Actor, id int64) (int64, error) {
	var ended int64
	err := s.inTx(ctx, func(tx *sqlx.Tx) error {
		a, err := getAccount(ctx, tx, `u.id = ?`, id)
		if err != nil {
			return err
		}
		if ended, err = endSessions(ctx, tx, id); err != nil {
			return err
		}
		return recordAudit(ctx, tx, Record{Time: s.now(), Actor: actor, Action: ActionRevokeSessions,
			TargetUserID: id, TargetUsername: a.Username, Detail: fmt.Sprintf("sessions_ended=%d", ended)})
	})
	return ended, err
}

// RevokeSession ends one session (the admin sessions page). The session
// identifier is never recorded.
func (s *Service) RevokeSession(ctx context.Context, actor Actor, sessionID string) error {
	return s.inTx(ctx, func(tx *sqlx.Tx) error {
		var owner struct {
			ID       int64  `db:"id"`
			Username string `db:"username"`
		}
		err := tx.GetContext(ctx, &owner, `SELECT u.id, u.username FROM sessions s JOIN users u ON u.id = s.user_id WHERE s.id = ?`, sessionID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil // nothing to end
		}
		if err != nil {
			return fmt.Errorf("look up session: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE sessions SET is_valid = FALSE WHERE id = ?`, sessionID); err != nil {
			return fmt.Errorf("end session: %w", err)
		}
		return recordAudit(ctx, tx, Record{Time: s.now(), Actor: actor, Action: ActionRevokeSession,
			TargetUserID: owner.ID, TargetUsername: owner.Username})
	})
}

// CleanupExpiredSessions marks every expired session invalid.
func (s *Service) CleanupExpiredSessions(ctx context.Context, actor Actor) (int64, error) {
	var n int64
	err := s.inTx(ctx, func(tx *sqlx.Tx) error {
		res, err := tx.ExecContext(ctx, `UPDATE sessions SET is_valid = FALSE WHERE is_valid = TRUE AND expires_at < CURRENT_TIMESTAMP`)
		if err != nil {
			return fmt.Errorf("clean up sessions: %w", err)
		}
		n, _ = res.RowsAffected()
		return recordAudit(ctx, tx, Record{Time: s.now(), Actor: actor, Action: ActionCleanupSessions, Detail: fmt.Sprintf("sessions_ended=%d", n)})
	})
	return n, err
}

// RecordEvent records an action that is not an account change made here (for
// example `setup code`). detail must not contain a secret.
func (s *Service) RecordEvent(ctx context.Context, actor Actor, action, detail string) error {
	return recordAudit(ctx, s.db, Record{Time: s.now(), Actor: actor, Action: action, Detail: detail})
}

// endSessions marks every valid session of userID invalid and returns how
// many were still unexpired.
func endSessions(ctx context.Context, tx *sqlx.Tx, userID int64) (int64, error) {
	var live int64
	if err := tx.GetContext(ctx, &live, `SELECT COUNT(*) FROM sessions WHERE user_id = ? AND is_valid = TRUE AND expires_at > CURRENT_TIMESTAMP`, userID); err != nil {
		return 0, fmt.Errorf("count sessions: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE sessions SET is_valid = FALSE WHERE user_id = ? AND is_valid = TRUE`, userID); err != nil {
		return 0, fmt.Errorf("end sessions: %w", err)
	}
	return live, nil
}

func (s *Service) inTx(ctx context.Context, fn func(tx *sqlx.Tx) error) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
