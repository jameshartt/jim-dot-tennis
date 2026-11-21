// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package accounts

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
)

// A user's own password: add, change, remove (the sign-in methods page).
// Nobody else ever sets or sees it.

// Audit actions for a user's own password. (ActionChangePassword is in audit.go.)
const (
	ActionAddPassword    = "user.add_password"    // the user added a password to an account that had none
	ActionRemovePassword = "user.remove_password" // the user removed their password
)

var (
	// ErrReauthRequired: adding, changing or removing a password needs the
	// current password or a fresh passkey confirmation, and neither was given.
	ErrReauthRequired = errors.New("confirm it's you first, with your current password or a passkey")
	// ErrLastSignInMethod: removing the password would leave the account
	// without a working way to sign in.
	ErrLastSignInMethod = errors.New("your password is your only working way to sign in, so it can't be removed: add a passkey first")
)

// PasswordChange is a request to set the signed-in user's own password.
type PasswordChange struct {
	// Current is the current password, when the account has one and the
	// person typed it. It is checked here.
	Current string
	// ConsumePasskeyProof, when not nil, uses up a passkey re-authentication
	// made in this session (auth.Passkeys.ConsumeReauthProof) and reports
	// whether it was valid. It is called only when Current is empty, and only
	// once everything else has been checked, so a refused password (too
	// short, say) does not use the confirmation up.
	ConsumePasskeyProof func() bool
	// New is the new password, checked against auth.ValidatePassword.
	New string
}

// PasswordSet says what SetOwnPassword did.
type PasswordSet struct {
	Added         bool  // the account had no password before
	SessionsEnded int64 // every session is ended (the caller signs this browser in again)
	Reauth        string
}

// SetOwnPassword adds or changes user id's password. It needs proof that the
// request comes from the account's owner: the current password (for an
// account that has one), or a passkey confirmation (ConsumePasskeyProof). A
// session alone is never enough.
//
// Adding or changing a password ends every session of the account (the
// caller starts a new one for this browser, so the person stays signed in
// here and is signed out everywhere else).
func (s *Service) SetOwnPassword(ctx context.Context, id int64, ch PasswordChange) (PasswordSet, error) {
	var u struct {
		Username     string              `db:"username"`
		PasswordHash models.PasswordHash `db:"password_hash"`
	}
	err := s.db.GetContext(ctx, &u, `SELECT username, password_hash FROM users WHERE id = ? AND is_active = TRUE`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return PasswordSet{}, ErrNotFound
	}
	if err != nil {
		return PasswordSet{}, fmt.Errorf("look up account: %w", err)
	}
	oldHash := string(u.PasswordHash)
	had := oldHash != ""
	out := PasswordSet{Added: !had}

	usePassword := had && ch.Current != ""
	if usePassword && !auth.PasswordMatches(oldHash, ch.Current) {
		return PasswordSet{}, ErrWrongPassword
	}
	if !usePassword && ch.ConsumePasskeyProof == nil {
		return PasswordSet{}, ErrReauthRequired
	}
	if err := auth.ValidatePassword(ch.New, u.Username); err != nil {
		return PasswordSet{}, err
	}
	if had && auth.PasswordMatches(oldHash, ch.New) {
		return PasswordSet{}, auth.SamePasswordError()
	}
	if usePassword {
		out.Reauth = "password"
	} else {
		if !ch.ConsumePasskeyProof() {
			return PasswordSet{}, ErrReauthRequired
		}
		out.Reauth = "passkey"
	}
	// Hashed before the transaction, so the slow hash does not hold the
	// database's write lock.
	hash, err := auth.HashPassword(ch.New)
	if err != nil {
		return PasswordSet{}, fmt.Errorf("hash password: %w", err)
	}
	err = s.inTx(ctx, func(tx *sqlx.Tx) error {
		// Only if the password is still what was checked above: a concurrent
		// change in between makes this one fail rather than overwrite it.
		res, err := tx.ExecContext(ctx, `
			UPDATE users SET password_hash = ?
			WHERE id = ? AND is_active = TRUE AND COALESCE(password_hash, '') = ?`, hash, id, oldHash)
		if err != nil {
			return fmt.Errorf("set password: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return fmt.Errorf("set password: the account changed while this was being done; try again")
		}
		action := ActionAddPassword
		if had {
			action = ActionChangePassword
		}
		if out.SessionsEnded, err = endSessions(ctx, tx, id); err != nil {
			return err
		}
		return recordAudit(ctx, tx, Record{Time: s.now(), Actor: UserActor(id, u.Username), Action: action,
			TargetUserID: id, TargetUsername: u.Username, Detail: fmt.Sprintf("reauth=%s sessions_ended=%d", out.Reauth, out.SessionsEnded)})
	})
	if err != nil {
		return PasswordSet{}, err
	}
	return out, nil
}

// Proof is what the signed-in user offers to show a change to their sign-in
// methods comes from the account's owner (auth/ownership.go).
type Proof struct {
	// Current is the current password, when the person typed it.
	Current string
	// ConsumePasskeyProof, when not nil, uses up a passkey confirmation made
	// in this session and reports whether it was valid. It is called only
	// when Current is empty and the change is otherwise going ahead.
	ConsumePasskeyProof func() bool
}

// PasswordRemoved says what RemoveOwnPassword did.
type PasswordRemoved struct {
	SessionsEnded int64 // every session is ended (the caller signs this browser in again)
	Reauth        string
}

// RemoveOwnPassword removes user id's password. Like every change to how an
// account signs in it needs proof of ownership: the current password or a
// passkey confirmation (ErrWrongPassword, ErrReauthRequired). It is refused
// (ErrLastSignInMethod) unless the account keeps a usable passkey (one
// without a clone warning): the check is part of the UPDATE, so two
// concurrent removals of the last methods cannot both succeed. An account
// with no password gets ErrNoPassword. Every session of the account is ended;
// the caller starts a new one for this browser.
func (s *Service) RemoveOwnPassword(ctx context.Context, id int64, proof Proof) (PasswordRemoved, error) {
	var u struct {
		PasswordHash models.PasswordHash `db:"password_hash"`
		Usable       int                 `db:"usable"`
	}
	err := s.db.GetContext(ctx, &u, `
		SELECT password_hash, `+repository.UsablePasskeysSQL+` AS usable
		FROM users WHERE id = ? AND is_active = TRUE`, id, id)
	if errors.Is(err, sql.ErrNoRows) {
		return PasswordRemoved{}, ErrNotFound
	}
	if err != nil {
		return PasswordRemoved{}, fmt.Errorf("look up account: %w", err)
	}
	oldHash := string(u.PasswordHash)
	// Refusals that need no proof come first, so a passkey confirmation is
	// not used up on a removal that could never happen.
	if oldHash == "" {
		return PasswordRemoved{}, ErrNoPassword
	}
	if u.Usable == 0 {
		return PasswordRemoved{}, ErrLastSignInMethod
	}
	out := PasswordRemoved{Reauth: auth.ProofPassword}
	switch {
	case proof.Current != "":
		if !auth.PasswordMatches(oldHash, proof.Current) {
			return PasswordRemoved{}, ErrWrongPassword
		}
	case proof.ConsumePasskeyProof != nil && proof.ConsumePasskeyProof():
		out.Reauth = auth.ProofPasskey
	default:
		return PasswordRemoved{}, ErrReauthRequired
	}
	err = s.inTx(ctx, func(tx *sqlx.Tx) error {
		// Only the password that was checked, and only while a usable
		// passkey remains.
		res, err := tx.ExecContext(ctx, `
			UPDATE users SET password_hash = NULL
			WHERE id = ? AND is_active = TRUE AND password_hash = ?
			  AND `+repository.UsablePasskeysSQL+` > 0`, id, oldHash, id)
		if err != nil {
			return fmt.Errorf("remove password: %w", err)
		}
		a, err := getAccount(ctx, tx, `u.id = ?`, id)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			if !a.HasPassword {
				return ErrNoPassword
			}
			return ErrLastSignInMethod
		}
		if out.SessionsEnded, err = endSessions(ctx, tx, id); err != nil {
			return err
		}
		return recordAudit(ctx, tx, Record{Time: s.now(), Actor: UserActor(id, a.Username), Action: ActionRemovePassword,
			TargetUserID: id, TargetUsername: a.Username,
			Detail: fmt.Sprintf("passkeys=%d reauth=%s sessions_ended=%d", a.Passkeys, out.Reauth, out.SessionsEnded)})
	})
	if err != nil {
		return PasswordRemoved{}, err
	}
	return out, nil
}
