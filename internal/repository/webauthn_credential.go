// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"

	"github.com/jmoiron/sqlx"
)

// SettingPasskeyDecoyKey is the per-instance random key (hex) behind the
// decoy credential ids the username-first passkey sign-in returns for
// usernames that have no passkeys. Generated on first use; never shown.
const SettingPasskeyDecoyKey = "passkey_decoy_key"

// userHandleBytes is the length of a WebAuthn user handle (the maximum the
// specification allows is 64).
const userHandleBytes = 32

var (
	// ErrPasskeyExists means the credential id is already enrolled (for this
	// or any other user).
	ErrPasskeyExists = errors.New("passkey is already registered")
	// ErrPasskeyNotFound means the user has no passkey with that id.
	ErrPasskeyNotFound = errors.New("passkey not found")
	// ErrLastSignInMethod means removing the passkey would leave the user
	// with no way to sign in.
	ErrLastSignInMethod = errors.New("cannot remove the last sign-in method")
)

// CountSignInMethods is the number of ways userID can sign in: one per
// passkey, plus one for a password. It is the one place that defines "sign-in
// method", so the last-method guard and the pages agree. q may be a
// transaction.
func CountSignInMethods(ctx context.Context, q sqlx.QueryerContext, userID int64) (int, error) {
	var n int
	err := sqlx.GetContext(ctx, q, &n, `
		SELECT (SELECT COUNT(*) FROM webauthn_credentials WHERE user_id = ?)
		     + (SELECT COUNT(*) FROM users WHERE id = ? AND COALESCE(password_hash, '') <> '')
	`, userID, userID)
	return n, err
}

// WebAuthnCredentialRepository stores passkeys and user handles.
type WebAuthnCredentialRepository interface {
	// UserHandle returns userID's stable random handle, creating it on first use.
	UserHandle(ctx context.Context, userID int64) ([]byte, error)
	// UserIDByHandle resolves a handle presented by an authenticator.
	UserIDByHandle(ctx context.Context, handle []byte) (int64, bool, error)
	ListByUser(ctx context.Context, userID int64) ([]models.WebAuthnCredential, error)
	// Create stores a new passkey; ErrPasskeyExists when the credential id is taken.
	Create(ctx context.Context, cred *models.WebAuthnCredential) error
	// RecordUse stores the counter and flags of a successful sign-in.
	RecordUse(ctx context.Context, cred *models.WebAuthnCredential, usedAt time.Time) error
	// SetCloneWarning marks (at non-nil) or clears (at nil) the clone warning.
	// It reports whether a passkey with that id exists.
	SetCloneWarning(ctx context.Context, id int64, at *time.Time) (bool, error)
	// CloneWarnings lists every passkey with a clone warning, newest first.
	CloneWarnings(ctx context.Context) ([]models.PasskeyWarning, error)
	// Rename renames userID's passkey id; ErrPasskeyNotFound when it is not theirs.
	Rename(ctx context.Context, userID, id int64, name string) error
	// Delete removes userID's passkey id unless it is their last sign-in
	// method (ErrLastSignInMethod); ErrPasskeyNotFound when it is not theirs.
	Delete(ctx context.Context, userID, id int64) error
}

type webAuthnCredentialRepository struct {
	db *database.DB
}

// NewWebAuthnCredentialRepository returns the passkey repository.
func NewWebAuthnCredentialRepository(db *database.DB) WebAuthnCredentialRepository {
	return &webAuthnCredentialRepository{db: db}
}

func (r *webAuthnCredentialRepository) UserHandle(ctx context.Context, userID int64) ([]byte, error) {
	handle := make([]byte, userHandleBytes)
	if _, err := rand.Read(handle); err != nil {
		return nil, err
	}
	// The insert is ignored when the user already has a handle, so concurrent
	// first uses agree on whichever row won.
	if _, err := r.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO webauthn_user_handles (user_id, handle) VALUES (?, ?)`, userID, handle); err != nil {
		return nil, fmt.Errorf("store user handle: %w", err)
	}
	var stored []byte
	if err := r.db.GetContext(ctx, &stored, `SELECT handle FROM webauthn_user_handles WHERE user_id = ?`, userID); err != nil {
		return nil, fmt.Errorf("load user handle: %w", err)
	}
	return stored, nil
}

func (r *webAuthnCredentialRepository) UserIDByHandle(ctx context.Context, handle []byte) (int64, bool, error) {
	if len(handle) == 0 {
		return 0, false, nil
	}
	var id int64
	err := r.db.GetContext(ctx, &id, `SELECT user_id FROM webauthn_user_handles WHERE handle = ?`, handle)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	return id, err == nil, err
}

func (r *webAuthnCredentialRepository) ListByUser(ctx context.Context, userID int64) ([]models.WebAuthnCredential, error) {
	var creds []models.WebAuthnCredential
	err := r.db.SelectContext(ctx, &creds, `SELECT * FROM webauthn_credentials WHERE user_id = ? ORDER BY created_at, id`, userID)
	return creds, err
}

func (r *webAuthnCredentialRepository) Create(ctx context.Context, cred *models.WebAuthnCredential) error {
	return InsertWebAuthnCredential(ctx, r.db, cred)
}

// InsertWebAuthnCredential stores a new passkey using db, which may be a
// transaction (an account link stores the passkey in the transaction that
// consumes the link). It sets cred.ID; ErrPasskeyExists when the credential
// id is taken.
func InsertWebAuthnCredential(ctx context.Context, db sqlx.ExtContext, cred *models.WebAuthnCredential) error {
	res, err := sqlx.NamedExecContext(ctx, db, `
		INSERT INTO webauthn_credentials (
			user_id, credential_id, public_key, attestation_type, attestation_format, aaguid,
			sign_count, transports, attachment, user_present, user_verified,
			backup_eligible, backup_state, name, created_at
		) VALUES (
			:user_id, :credential_id, :public_key, :attestation_type, :attestation_format, :aaguid,
			:sign_count, :transports, :attachment, :user_present, :user_verified,
			:backup_eligible, :backup_state, :name, :created_at
		)`, cred)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed: webauthn_credentials.credential_id") {
			return ErrPasskeyExists
		}
		return err
	}
	cred.ID, err = res.LastInsertId()
	return err
}

func (r *webAuthnCredentialRepository) RecordUse(ctx context.Context, cred *models.WebAuthnCredential, usedAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE webauthn_credentials
		SET sign_count = ?, user_present = ?, user_verified = ?, backup_eligible = ?, backup_state = ?, last_used_at = ?
		WHERE id = ?`,
		cred.SignCount, cred.UserPresent, cred.UserVerified, cred.BackupEligible, cred.BackupState, usedAt, cred.ID)
	return err
}

func (r *webAuthnCredentialRepository) SetCloneWarning(ctx context.Context, id int64, at *time.Time) (bool, error) {
	res, err := r.db.ExecContext(ctx, `UPDATE webauthn_credentials SET clone_warning_at = ? WHERE id = ?`, at, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

func (r *webAuthnCredentialRepository) CloneWarnings(ctx context.Context) ([]models.PasskeyWarning, error) {
	var out []models.PasskeyWarning
	err := r.db.SelectContext(ctx, &out, `
		SELECT c.id, c.user_id, u.username, c.name, c.clone_warning_at
		FROM webauthn_credentials c JOIN users u ON u.id = c.user_id
		WHERE c.clone_warning_at IS NOT NULL
		ORDER BY c.clone_warning_at DESC, c.id`)
	return out, err
}

func (r *webAuthnCredentialRepository) Rename(ctx context.Context, userID, id int64, name string) error {
	res, err := r.db.ExecContext(ctx, `UPDATE webauthn_credentials SET name = ? WHERE id = ? AND user_id = ?`, name, id, userID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrPasskeyNotFound
	}
	return nil
}

func (r *webAuthnCredentialRepository) Delete(ctx context.Context, userID, id int64) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Delete first, then count what is left and roll back if nothing is.
	// Writing before the check makes this transaction the writer from the
	// start, so two concurrent removals cannot both see "two methods left".
	res, err := tx.ExecContext(ctx, `DELETE FROM webauthn_credentials WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrPasskeyNotFound
	}
	left, err := CountSignInMethods(ctx, tx, userID)
	if err != nil {
		return err
	}
	if left == 0 {
		return ErrLastSignInMethod
	}
	return tx.Commit()
}
