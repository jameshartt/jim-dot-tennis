// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// "Require passkeys for administrators" (WI-130).
//
// The setting is a row in app_settings. It is read straight from the
// database every time it matters (never cached), so the server command line
// can turn it off for a running server.
//
// A "usable" passkey is one without a clone warning: a blocked passkey cannot
// sign in, so it does not count towards an administrator having a passkey.

// SettingRequireAdminPasskeys is "true" while administrators must sign in
// with a passkey. Anything else (or no row) means off.
const SettingRequireAdminPasskeys = "require_admin_passkeys"

// ErrAdminNeedsPasskey: the change would leave an active administrator
// without a usable passkey while passkeys are required for administrators.
var ErrAdminNeedsPasskey = errors.New("passkeys are required for administrators, and this would leave an administrator without one")

// adminPasskeysRequiredSQL is true while the setting is on.
const adminPasskeysRequiredSQL = `EXISTS (SELECT 1 FROM app_settings WHERE key = '` + SettingRequireAdminPasskeys + `' AND value = 'true')`

// UsablePasskeysSQL counts the usable passkeys of the user whose id is bound
// to its one placeholder.
const UsablePasskeysSQL = `(SELECT COUNT(*) FROM webauthn_credentials WHERE user_id = ? AND clone_warning_at IS NULL)`

// AdminLacksRequiredPasskeySQL is true for the user whose id is bound to
// both placeholders when the setting is on and that user would be an active
// administrator with no usable passkey. Use it in the WHERE of the statement
// that makes the change (NOT (...)), so the check and the write are one step.
const AdminLacksRequiredPasskeySQL = `(` + adminPasskeysRequiredSQL + `
	AND EXISTS (SELECT 1 FROM users WHERE id = ? AND role = 'admin' AND is_active = TRUE)
	AND ` + UsablePasskeysSQL + ` = 0)`

// AdminPasskeysRequired reports whether the setting is on. q may be a
// transaction.
func AdminPasskeysRequired(ctx context.Context, q sqlx.QueryerContext) (bool, error) {
	var on bool
	if err := sqlx.GetContext(ctx, q, &on, `SELECT `+adminPasskeysRequiredSQL); err != nil {
		return false, fmt.Errorf("read %s: %w", SettingRequireAdminPasskeys, err)
	}
	return on, nil
}

// ActiveAdminsWithoutPasskey lists, by username, the active administrators
// with no usable passkey. q may be a transaction.
func ActiveAdminsWithoutPasskey(ctx context.Context, q sqlx.QueryerContext) ([]string, error) {
	var names []string
	err := sqlx.SelectContext(ctx, q, &names, `
		SELECT u.username FROM users u
		WHERE u.role = 'admin' AND u.is_active = TRUE
		  AND NOT EXISTS (SELECT 1 FROM webauthn_credentials c WHERE c.user_id = u.id AND c.clone_warning_at IS NULL)
		ORDER BY u.username COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("list administrators without a passkey: %w", err)
	}
	return names, nil
}

// AdminLacksRequiredPasskey reports whether AdminLacksRequiredPasskeySQL
// holds for userID. q may be a transaction.
func AdminLacksRequiredPasskey(ctx context.Context, q sqlx.QueryerContext, userID int64) (bool, error) {
	var lacks bool
	if err := sqlx.GetContext(ctx, q, &lacks, `SELECT `+AdminLacksRequiredPasskeySQL, userID, userID); err != nil {
		return false, fmt.Errorf("check administrator passkey: %w", err)
	}
	return lacks, nil
}
