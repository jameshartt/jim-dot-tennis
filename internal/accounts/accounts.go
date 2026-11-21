// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

// Package accounts is the one place account operations happen: create, list,
// enable/disable, role change, password reset and session revocation. The
// admin users and sessions pages and the server command line
// (`jim-dot-tennis user ...`) both call it, so the rules - above all "never
// leave the instance without an active administrator" - hold whichever way an
// account is changed, and every change is recorded through recordAudit.
package accounts

import (
	"context"
	"crypto/rand"
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
	ErrInvalidRole   = errors.New("the role must be admin, captain or player")
	// ErrLastAdmin: the change would leave no active administrator.
	ErrLastAdmin = errors.New("this is the last active administrator: make another account an active administrator first")
	// ErrWrongPassword: the current password given to ChangeOwnPassword is wrong.
	ErrWrongPassword = errors.New("your current password is not right")
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
func ParseRole(s string) (models.Role, error) {
	switch r := models.Role(strings.ToLower(strings.TrimSpace(s))); r {
	case models.RoleAdmin, models.RoleCaptain, models.RolePlayer:
		return r, nil
	}
	return "", ErrInvalidRole
}

// Account is an account as shown to administrators and the command line. It
// deliberately has no password hash.
type Account struct {
	ID                 int64       `db:"id"`
	Username           string      `db:"username"`
	Role               models.Role `db:"role"`
	PlayerID           *string     `db:"player_id"`
	IsActive           bool        `db:"is_active"`
	MustChangePassword bool        `db:"must_change_password"`
	CreatedAt          time.Time   `db:"created_at"`
	LastLoginAt        time.Time   `db:"last_login_at"`
	ActiveSessions     int         `db:"active_sessions"` // valid, unexpired sessions
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
	u.id, u.username, u.role, u.player_id, u.is_active, u.must_change_password,
	u.created_at, u.last_login_at,
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

// Create makes a new active account. The password must meet the password
// policy (auth.ValidatePassword). mustChange marks it as chosen by someone
// else, so the user is asked to replace it at their first sign-in.
func (s *Service) Create(ctx context.Context, actor Actor, username string, role models.Role, password string, mustChange bool) (Account, error) {
	username = strings.TrimSpace(username)
	if !usernamePattern.MatchString(username) {
		return Account{}, &InvalidUsernameError{Username: username}
	}
	if _, err := ParseRole(string(role)); err != nil {
		return Account{}, err
	}
	if err := auth.ValidatePassword(password, username); err != nil {
		return Account{}, err
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return Account{}, fmt.Errorf("hash password: %w", err)
	}
	var created Account
	err = s.inTx(ctx, func(tx *sqlx.Tx) error {
		var taken int
		if err := tx.GetContext(ctx, &taken, `SELECT COUNT(*) FROM users WHERE username = ? COLLATE NOCASE`, username); err != nil {
			return fmt.Errorf("check username: %w", err)
		}
		if taken > 0 {
			return ErrUsernameTaken
		}
		now := s.now()
		res, err := tx.ExecContext(ctx, `
			INSERT INTO users (username, password_hash, role, is_active, must_change_password, created_at, last_login_at)
			VALUES (?, ?, ?, TRUE, ?, ?, ?)`, username, hash, role, mustChange, now, now)
		if err != nil {
			return fmt.Errorf("create account: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("create account: %w", err)
		}
		if created, err = getAccount(ctx, tx, `u.id = ?`, id); err != nil {
			return err
		}
		return recordAudit(ctx, tx, Record{Time: now, Actor: actor, Action: ActionCreate,
			TargetUserID: id, TargetUsername: username, Detail: "role " + string(role)})
	})
	return created, err
}

// CreateWithTemporaryPassword makes a new account with a generated password,
// returned once for the caller to hand over; the user must replace it at
// their first sign-in.
func (s *Service) CreateWithTemporaryPassword(ctx context.Context, actor Actor, username string, role models.Role) (Account, string, error) {
	temp, err := TemporaryPassword(username)
	if err != nil {
		return Account{}, "", err
	}
	a, err := s.Create(ctx, actor, username, role, temp, true)
	if err != nil {
		return Account{}, "", err
	}
	return a, temp, nil
}

// SetPassword gives account id the password an administrator typed and ends
// all of its sessions. Unless the administrator is setting their own password
// the user must replace it at their next sign-in.
func (s *Service) SetPassword(ctx context.Context, actor Actor, id int64, password string) error {
	a, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	mustChange := !(actor.Kind == ActorUser && actor.UserID == id)
	_, err = s.setPassword(ctx, actor, a, password, mustChange, ActionSetPassword)
	return err
}

// ResetWithTemporaryPassword gives account id a generated password (returned
// once), marks it as one the user must replace at their next sign-in and ends
// all of the account's sessions. It returns the number of sessions ended.
//
// WI-129 replaces this with a single-use recovery link.
func (s *Service) ResetWithTemporaryPassword(ctx context.Context, actor Actor, id int64) (string, int64, error) {
	a, err := s.Get(ctx, id)
	if err != nil {
		return "", 0, err
	}
	temp, err := TemporaryPassword(a.Username)
	if err != nil {
		return "", 0, err
	}
	ended, err := s.setPassword(ctx, actor, a, temp, true, ActionResetPassword)
	if err != nil {
		return "", 0, err
	}
	return temp, ended, nil
}

func (s *Service) setPassword(ctx context.Context, actor Actor, a Account, password string, mustChange bool, action string) (int64, error) {
	if err := auth.ValidatePassword(password, a.Username); err != nil {
		return 0, err
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return 0, fmt.Errorf("hash password: %w", err)
	}
	var ended int64
	err = s.inTx(ctx, func(tx *sqlx.Tx) error {
		res, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = ?, must_change_password = ? WHERE id = ?`, hash, mustChange, a.ID)
		if err != nil {
			return fmt.Errorf("set password: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		if ended, err = endSessions(ctx, tx, a.ID); err != nil {
			return err
		}
		detail := fmt.Sprintf("must_change=%v sessions_ended=%d", mustChange, ended)
		return recordAudit(ctx, tx, Record{Time: s.now(), Actor: actor, Action: action,
			TargetUserID: a.ID, TargetUsername: a.Username, Detail: detail})
	})
	return ended, err
}

// ChangeOwnPassword lets user id replace their password after giving the
// current one. It clears the must-change mark and ends all of the user's
// sessions (the caller signs the browser in again with a fresh session).
func (s *Service) ChangeOwnPassword(ctx context.Context, id int64, current, newPassword string) error {
	var u struct {
		Username     string `db:"username"`
		PasswordHash string `db:"password_hash"`
	}
	err := s.db.GetContext(ctx, &u, `SELECT username, password_hash FROM users WHERE id = ? AND is_active = TRUE`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("look up account: %w", err)
	}
	if !auth.PasswordMatches(u.PasswordHash, current) {
		return ErrWrongPassword
	}
	if current == newPassword {
		return fmt.Errorf("%w: Choose a password different from the current one.", auth.ErrPasswordPolicy)
	}
	a := Account{ID: id, Username: u.Username}
	_, err = s.setPassword(ctx, UserActor(id, u.Username), a, newPassword, false, ActionChangePassword)
	return err
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

// tempAlphabet is lower-case letters and digits without look-alikes (0/o,
// 1/l/i), so a temporary password read off a terminal is easy to type.
const tempAlphabet = "23456789abcdefghjkmnpqrstuvwxyz"

// TemporaryPassword returns a random password for username, grouped as
// xxxx-xxxx-xxxx-xxxx (16 symbols from 31, about 79 bits). It always meets
// the password policy.
func TemporaryPassword(username string) (string, error) {
	// Bytes at or above limit are discarded so every symbol is equally likely.
	limit := byte(256 / len(tempAlphabet) * len(tempAlphabet))
	for {
		symbols := make([]byte, 0, 16)
		buf := make([]byte, 32)
		for len(symbols) < 16 {
			if _, err := rand.Read(buf); err != nil {
				return "", fmt.Errorf("generate password: %w", err)
			}
			for _, v := range buf {
				if v < limit && len(symbols) < 16 {
					symbols = append(symbols, tempAlphabet[int(v)%len(tempAlphabet)])
				}
			}
		}
		var b strings.Builder
		for i, c := range symbols {
			if i > 0 && i%4 == 0 {
				b.WriteByte('-')
			}
			b.WriteByte(c)
		}
		if p := b.String(); auth.ValidatePassword(p, username) == nil {
			return p, nil
		}
	}
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
