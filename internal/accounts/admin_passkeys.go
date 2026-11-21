// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package accounts

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"jim-dot-tennis/internal/repository"
)

// The instance setting "Require passkeys for administrators".
//
// While it is on:
//   - an administrator cannot sign in with a password (auth.Service.Login);
//   - an administrator cannot use an invite or recovery link to set a
//     password, only a passkey (RedeemLink);
//   - an administrator cannot remove their last usable passkey (the passkey
//     repository's Delete);
//   - an account without a usable passkey cannot be made an administrator
//     (SetRole).
//
// It cannot be turned on until every active administrator has a usable
// passkey (one without a clone warning), and there is at least one active
// administrator. Both are checked inside the transaction that turns it on,
// after its first write, so a passkey removed or an administrator added at
// the same moment cannot slip past.
//
// Nothing here can lock a club out for good: the server command line turns
// the setting off (`jim-dot-tennis admin-passkeys off`), `user reset` still
// makes a recovery link with which an administrator enrols a new passkey,
// and the sign-in rules apply only where passkeys can be used at all (with
// no usable public URL, administrators may use passwords again).

// Audit action for the setting.
const ActionAdminPasskeys = "settings.admin_passkeys" // the setting was turned on or off

// ErrAdminNeedsPasskey: the change would leave an administrator without a
// usable passkey while passkeys are required for administrators.
var ErrAdminNeedsPasskey = errors.New("the club requires passkeys for administrators, and this account has no passkey: it must add one before it can be made an administrator")

// ErrAdminPasskeyRequired: an administrator tried to use a link to set a
// password while passkeys are required for administrators.
var ErrAdminPasskeyRequired = errors.New("administrators must use a passkey here: a password can't be set up for this account")

// ErrNoActiveAdmins: there is no active administrator, so the requirement
// cannot be turned on (it would apply to nobody now, and could catch the
// next administrator without a passkey).
var ErrNoActiveAdmins = errors.New("there is no active administrator yet")

// AdminsWithoutPasskeyError lists the active administrators that stop the
// requirement being turned on.
type AdminsWithoutPasskeyError struct{ Usernames []string }

func (e *AdminsWithoutPasskeyError) Error() string {
	return "these administrators have no working passkey yet: " + strings.Join(e.Usernames, ", ") +
		". Each must add one (on their Sign-in methods page) before passkeys can be required"
}

// promoteNeedsPasskeyCondition (bound once with the account's id) holds when
// the account may be given the admin role: it is already one, or the
// requirement is off, or it has a usable passkey.
const promoteNeedsPasskeyCondition = `(role = 'admin'
	OR NOT EXISTS (SELECT 1 FROM app_settings WHERE key = '` + repository.SettingRequireAdminPasskeys + `' AND value = 'true')
	OR ` + repository.UsablePasskeysSQL + ` > 0)`

// AdminPasskeyStatus is the setting and who would stop it being turned on.
type AdminPasskeyStatus struct {
	Required bool
	// AdminsWithoutPasskey: active administrators with no usable passkey.
	AdminsWithoutPasskey []string
	ActiveAdmins         int
}

// AdminPasskeyStatus reads the setting and the administrators' passkeys.
func (s *Service) AdminPasskeyStatus(ctx context.Context) (AdminPasskeyStatus, error) {
	var st AdminPasskeyStatus
	var err error
	if st.Required, err = repository.AdminPasskeysRequired(ctx, s.db); err != nil {
		return st, err
	}
	if st.AdminsWithoutPasskey, err = repository.ActiveAdminsWithoutPasskey(ctx, s.db); err != nil {
		return st, err
	}
	if err := s.db.GetContext(ctx, &st.ActiveAdmins, `SELECT COUNT(*) FROM users WHERE role = 'admin' AND is_active = TRUE`); err != nil {
		return st, fmt.Errorf("count administrators: %w", err)
	}
	return st, nil
}

// SetAdminPasskeysRequired turns the requirement on or off. Turning it on is
// refused with *AdminsWithoutPasskeyError (naming them) while any active
// administrator has no usable passkey, and with ErrNoActiveAdmins when there
// is no active administrator. Turning it off always works.
func (s *Service) SetAdminPasskeysRequired(ctx context.Context, actor Actor, on bool) error {
	return s.inTx(ctx, func(tx *sqlx.Tx) error {
		// The write comes first, so this transaction holds the write lock
		// before it checks anything.
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, ?)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
			repository.SettingRequireAdminPasskeys, fmt.Sprint(on), time.Now().UTC()); err != nil {
			return fmt.Errorf("store setting: %w", err)
		}
		if on {
			var admins int
			if err := tx.GetContext(ctx, &admins, `SELECT COUNT(*) FROM users WHERE role = 'admin' AND is_active = TRUE`); err != nil {
				return fmt.Errorf("count administrators: %w", err)
			}
			if admins == 0 {
				return ErrNoActiveAdmins
			}
			missing, err := repository.ActiveAdminsWithoutPasskey(ctx, tx)
			if err != nil {
				return err
			}
			if len(missing) > 0 {
				return &AdminsWithoutPasskeyError{Usernames: missing}
			}
		}
		return recordAudit(ctx, tx, Record{Time: s.now(), Actor: actor, Action: ActionAdminPasskeys, Detail: fmt.Sprintf("required=%v", on)})
	})
}
