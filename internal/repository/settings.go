// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"
)

// Well-known app_settings keys.
const (
	// SettingHomeClubID is the clubs.id of the home club chosen during setup.
	// HOME_CLUB_ID / HOME_CLUB_NAME in the environment take precedence over it.
	SettingHomeClubID = "home_club_id"
	// SettingSetupCompletedAt records when first-run setup was completed (RFC 3339, UTC).
	SettingSetupCompletedAt = "setup_completed_at"
)

// SettingsRepository provides typed access to the app_settings key/value table.
//
// Every getter returns (value, found, err): found is false and err is nil when
// the key has never been set. A stored value that cannot be parsed as the
// requested type is reported as an error, not as "not found".
type SettingsRepository interface {
	GetString(ctx context.Context, key string) (string, bool, error)
	SetString(ctx context.Context, key, value string) error
	GetUint(ctx context.Context, key string) (uint, bool, error)
	SetUint(ctx context.Context, key string, value uint) error
	GetBool(ctx context.Context, key string) (bool, bool, error)
	SetBool(ctx context.Context, key string, value bool) error
	GetTime(ctx context.Context, key string) (time.Time, bool, error)
	SetTime(ctx context.Context, key string, value time.Time) error
	Delete(ctx context.Context, key string) error
}

type settingsRepository struct {
	db sqlx.ExtContext
}

// NewSettingsRepository returns a settings repository over db, which may be a
// *database.DB or a *sqlx.Tx (so settings can be written in the same
// transaction as other setup work).
func NewSettingsRepository(db sqlx.ExtContext) SettingsRepository {
	return &settingsRepository{db: db}
}

// GetString returns the raw stored value for key.
func (r *settingsRepository) GetString(ctx context.Context, key string) (string, bool, error) {
	var value string
	err := sqlx.GetContext(ctx, r.db, &value, `SELECT value FROM app_settings WHERE key = ?`, key)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read setting %q: %w", key, err)
	}
	return value, true, nil
}

// SetString stores value under key, replacing any previous value.
func (r *settingsRepository) SetString(ctx context.Context, key, value string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, key, value, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("write setting %q: %w", key, err)
	}
	return nil
}

// GetUint returns the stored value for key parsed as an unsigned integer.
func (r *settingsRepository) GetUint(ctx context.Context, key string) (uint, bool, error) {
	raw, ok, err := r.GetString(ctx, key)
	if err != nil || !ok {
		return 0, ok, err
	}
	n, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, true, fmt.Errorf("setting %q=%q is not an unsigned integer: %w", key, raw, err)
	}
	return uint(n), true, nil
}

// SetUint stores an unsigned integer under key.
func (r *settingsRepository) SetUint(ctx context.Context, key string, value uint) error {
	return r.SetString(ctx, key, strconv.FormatUint(uint64(value), 10))
}

// GetBool returns the stored value for key parsed with strconv.ParseBool.
func (r *settingsRepository) GetBool(ctx context.Context, key string) (bool, bool, error) {
	raw, ok, err := r.GetString(ctx, key)
	if err != nil || !ok {
		return false, ok, err
	}
	b, err := strconv.ParseBool(raw)
	if err != nil {
		return false, true, fmt.Errorf("setting %q=%q is not a boolean: %w", key, raw, err)
	}
	return b, true, nil
}

// SetBool stores a boolean under key ("true" / "false").
func (r *settingsRepository) SetBool(ctx context.Context, key string, value bool) error {
	return r.SetString(ctx, key, strconv.FormatBool(value))
}

// GetTime returns the stored value for key parsed as RFC 3339.
func (r *settingsRepository) GetTime(ctx context.Context, key string) (time.Time, bool, error) {
	raw, ok, err := r.GetString(ctx, key)
	if err != nil || !ok {
		return time.Time{}, ok, err
	}
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, true, fmt.Errorf("setting %q=%q is not an RFC 3339 time: %w", key, raw, err)
	}
	return t, true, nil
}

// SetTime stores a time under key as RFC 3339 in UTC.
func (r *settingsRepository) SetTime(ctx context.Context, key string, value time.Time) error {
	return r.SetString(ctx, key, value.UTC().Format(time.RFC3339Nano))
}

// Delete removes key. Deleting a key that is not set is not an error.
func (r *settingsRepository) Delete(ctx context.Context, key string) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM app_settings WHERE key = ?`, key); err != nil {
		return fmt.Errorf("delete setting %q: %w", key, err)
	}
	return nil
}
