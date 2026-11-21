// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package models

import (
	"time"
)

// Role represents user access levels
type Role string

const (
	RoleAdmin   Role = "admin"
	RoleCaptain Role = "captain"
	RolePlayer  Role = "player"
)

// User represents an authenticated user
type User struct {
	ID           int64     `db:"id"`
	Username     string    `db:"username"`
	PasswordHash string    `db:"password_hash"`
	Role         Role      `db:"role"`
	PlayerID     *string   `db:"player_id"`
	IsActive     bool      `db:"is_active"`
	CreatedAt    time.Time `db:"created_at"`
	LastLoginAt  time.Time `db:"last_login_at"`
	// MustChangePassword is set when the password was chosen by someone else
	// (a temporary password from the server command line, or one an
	// administrator typed). Signed-in pages send the user to choose their own.
	MustChangePassword bool `db:"must_change_password"`
}

// Session represents a user's authenticated session
type Session struct {
	ID             string    `db:"id"`
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
