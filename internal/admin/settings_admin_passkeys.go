// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"errors"
	"log"
	"net/http"
	"net/url"

	"jim-dot-tennis/internal/accounts"
	"jim-dot-tennis/internal/config"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
)

// settingsAdminPasskeysPath turns "Require passkeys for administrators" on
// or off (POST, admin role; form field require=on|off).
const settingsAdminPasskeysPath = settingsPath + "/admin-passkeys"

// adminPasskeysView is the settings page's "Sign-in" section.
type adminPasskeysView struct {
	accounts.AdminPasskeyStatus
	// PasskeysAvailable: passkeys can be used at the address this page was
	// opened at (auth.Passkeys.Available: normally "a public URL is set",
	// and also localhost in development). Without them the requirement does
	// not apply.
	PasskeysAvailable bool
	Error             string
}

func (h *SettingsHandler) adminPasskeysView(r *http.Request, cs *config.ClubSettings) adminPasskeysView {
	var v adminPasskeysView
	st, err := h.service.accounts.AdminPasskeyStatus(r.Context())
	if err != nil {
		log.Printf("Settings: administrator passkey status: %v", err)
	}
	v.AdminPasskeyStatus = st
	if h.service.passkeysAvailable != nil {
		v.PasskeysAvailable = h.service.passkeysAvailable(r)
	} else {
		_, v.PasskeysAvailable, _ = cs.ResolvePublicURL()
	}
	v.Error = r.URL.Query().Get("passkey_error")
	return v
}

// setAdminPasskeys handles the toggle. Turning it on is refused, naming the
// administrators who have no passkey yet, until every one has one.
func (h *SettingsHandler) setAdminPasskeys(w http.ResponseWriter, r *http.Request, user *models.User) {
	on := r.PostFormValue("require") == "on"
	err := h.service.accounts.SetAdminPasskeysRequired(r.Context(), actorFor(user), on)
	var missing *accounts.AdminsWithoutPasskeyError
	switch {
	case err == nil:
		msg := "Administrators no longer need a passkey to sign in."
		if on {
			msg = "Administrators must now sign in with a passkey."
		}
		http.Redirect(w, r, settingsPath+"?message="+url.QueryEscape(msg)+"#signin", http.StatusSeeOther)
		return
	case errors.As(err, &missing), errors.Is(err, accounts.ErrNoActiveAdmins):
		msg := sentence(err.Error()) + "."
		http.Redirect(w, r, settingsPath+"?passkey_error="+url.QueryEscape(msg)+"#signin", http.StatusSeeOther)
		return
	default:
		logAndError(w, "Failed to save the setting", err, http.StatusInternalServerError)
	}
}

// publicURLChangeRefused stops the stored public URL changing while passkeys
// are required for administrators: passkeys only work at the address they
// were made for, so every administrator would be left with no way in but
// the server command line. values are the settings about to be saved.
func (h *SettingsHandler) publicURLChangeRefused(r *http.Request, cs *config.ClubSettings, values map[string]string) error {
	next, submitted := values[repository.SettingPublicURL]
	if !submitted || next == cs.Stored(repository.SettingPublicURL) {
		return nil
	}
	required, err := repository.AdminPasskeysRequired(r.Context(), h.service.db)
	if err != nil {
		return err
	}
	if required {
		return formError{"The public URL can't be changed while passkeys are required for administrators: " +
			"passkeys only work at the address they were made for, so every administrator would be locked out. " +
			"Turn off \"Require passkeys for administrators\" (below) first, change the address, have each administrator " +
			"add a passkey at the new address, then turn it back on."}
	}
	return nil
}
