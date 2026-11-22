// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package models

import (
	"database/sql/driver"
	"fmt"
	"time"
)

// PasswordHash is a bcrypt password hash, or "" for an account that has no
// password (it signs in with passkeys, or has not accepted its invite yet).
// "No password" is stored as NULL.
type PasswordHash string

// Scan reads the column; NULL becomes "".
func (h *PasswordHash) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*h = ""
	case string:
		*h = PasswordHash(v)
	case []byte:
		*h = PasswordHash(v)
	default:
		return fmt.Errorf("password hash: unexpected column type %T", src)
	}
	return nil
}

// Value stores "" as NULL.
func (h PasswordHash) Value() (driver.Value, error) {
	if h == "" {
		return nil, nil
	}
	return string(h), nil
}

// Role represents user access levels
type Role string

const (
	RoleAdmin   Role = "admin"
	RoleCaptain Role = "captain"
	// RoleTournamentsOnly is stored as "player", the retired role value (see
	// roles.go): it grants no league access. With tournament access (own or
	// all) the account is shown as "Tournaments only".
	RoleTournamentsOnly Role = "player"
)

// User represents an authenticated user
type User struct {
	ID           int64        `db:"id"`
	Username     string       `db:"username"`
	PasswordHash PasswordHash `db:"password_hash"` // "" when the account has no password
	Role         Role         `db:"role"`
	PlayerID     *string      `db:"player_id"`
	IsActive     bool         `db:"is_active"`
	CreatedAt    time.Time    `db:"created_at"`
	LastLoginAt  time.Time    `db:"last_login_at"`
	// MustChangePassword mirrors a retired column (migration 032, cleared by
	// 034) so that SELECT * still scans. It is always false and nothing reads
	// it: nobody sets a password for someone else any more.
	MustChangePassword bool `db:"must_change_password"`
	// TournamentAccess is the stored tournament access (migration 036).
	// Read it through TournamentAccessLevel, never directly: administrators
	// and disabled accounts do not use the stored value.
	TournamentAccess TournamentAccess `db:"tournament_access"`
}

// Session represents a user's authenticated session. The browser's cookie
// carries a random token; the database stores only its SHA-256 (TokenHash),
// so a copy of the database holds no usable session. ID is a plain number
// for naming a session (admin pages, URLs) and reveals nothing.
type Session struct {
	ID        int64  `db:"id"`
	TokenHash string `db:"token_hash"`
	// Token is the raw cookie value. It is set only on a session that has
	// just been created (to put in the cookie) and is never stored.
	Token          string    `db:"-"`
	UserID         int64     `db:"user_id"`
	Role           Role      `db:"role"`
	CreatedAt      time.Time `db:"created_at"`
	ExpiresAt      time.Time `db:"expires_at"`
	LastActivityAt time.Time `db:"last_activity_at"`
	IP             string    `db:"ip"`
	UserAgent      string    `db:"user_agent"`
	DeviceInfo     string    `db:"device_info"`
	IsValid        bool      `db:"is_valid"`
}

// LoginAttempt tracks authentication attempts
type LoginAttempt struct {
	ID        int64     `db:"id"`
	Username  string    `db:"username"`
	IP        string    `db:"ip"`
	UserAgent string    `db:"user_agent"`
	Success   bool      `db:"success"`
	CreatedAt time.Time `db:"created_at"`
}

// WebAuthnCredential is one enrolled passkey.
type WebAuthnCredential struct {
	ID                int64      `db:"id"`
	UserID            int64      `db:"user_id"`
	CredentialID      []byte     `db:"credential_id"`
	PublicKey         []byte     `db:"public_key"`
	AttestationType   string     `db:"attestation_type"`
	AttestationFormat string     `db:"attestation_format"`
	AAGUID            []byte     `db:"aaguid"`
	SignCount         uint32     `db:"sign_count"`
	Transports        string     `db:"transports"` // comma-separated transport hints
	Attachment        string     `db:"attachment"`
	UserPresent       bool       `db:"user_present"`
	UserVerified      bool       `db:"user_verified"`
	BackupEligible    bool       `db:"backup_eligible"`
	BackupState       bool       `db:"backup_state"`
	Name              string     `db:"name"`
	CreatedAt         time.Time  `db:"created_at"`
	LastUsedAt        *time.Time `db:"last_used_at"`
	// CloneWarningAt is when the authenticator's sign counter was seen going
	// backwards (a possible copy). While set the passkey cannot sign in.
	CloneWarningAt *time.Time `db:"clone_warning_at"`
}

// PasskeyWarning is a passkey with a clone warning, with its owner, for the
// administrators' list.
type PasskeyWarning struct {
	ID             int64     `db:"id"`
	UserID         int64     `db:"user_id"`
	Username       string    `db:"username"`
	Name           string    `db:"name"`
	CloneWarningAt time.Time `db:"clone_warning_at"`
}
