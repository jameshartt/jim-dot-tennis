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
