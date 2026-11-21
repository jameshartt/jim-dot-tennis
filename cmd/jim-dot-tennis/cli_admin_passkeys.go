// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package main

import (
	"context"
	"strings"

	"jim-dot-tennis/internal/accounts"
)

// adminPasskeys shows or changes "Require passkeys for administrators"
// (`jim-dot-tennis admin-passkeys status|on|off`). "off" always works: it is
// how the operator lets administrators sign in with a password again when
// none of them can use a passkey (for example after the site's address
// changed).
func adminPasskeys(ctx context.Context, c cliOut, svc *accounts.Service, actor accounts.Actor, what string) error {
	if what != "status" {
		if err := svc.SetAdminPasskeysRequired(ctx, actor, what == "on"); err != nil {
			return err
		}
	}
	st, err := svc.AdminPasskeyStatus(ctx)
	if err != nil {
		return err
	}
	switch {
	case st.Required:
		c.printf("Administrators must sign in with a passkey; they cannot use a password.\n")
	default:
		c.printf("Administrators may sign in with a passkey or a password.\n")
	}
	if len(st.AdminsWithoutPasskey) > 0 {
		c.printf("Active administrators with no working passkey: %s.\n", strings.Join(st.AdminsWithoutPasskey, ", "))
		if st.Required {
			c.printf("They can only get in with a recovery link (\"jim-dot-tennis user reset <username>\"), setting up a passkey.\n")
		} else {
			c.printf("Each must add a passkey (Sign-in methods page) before passkeys can be required.\n")
		}
	} else if st.ActiveAdmins > 0 {
		c.printf("Every active administrator (%d) has a passkey.\n", st.ActiveAdmins)
	}
	if st.Required {
		c.printf("To let administrators use passwords again: jim-dot-tennis admin-passkeys off\n")
	}
	return nil
}
