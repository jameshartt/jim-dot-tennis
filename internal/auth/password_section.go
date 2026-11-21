// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"net/http"
	"strings"

	"jim-dot-tennis/internal/models"
)

// The password part of the sign-in methods page. The forms post to
// internal/accounts (PasswordPath, PasswordRemovePath), which redirects back
// here with a result code: ?done=password-... or ?perror=<code>#password.

// Password routes, served by internal/accounts (accounts.PasswordPage).
const (
	// PasswordPath: POST adds or changes the signed-in user's password. GET
	// redirects to the password section of the sign-in methods page.
	PasswordPath = ChangePasswordPath
	// PasswordRemovePath: POST removes the signed-in user's password.
	PasswordRemovePath = ChangePasswordPath + "/remove"
)

// Codes for a refused password change that are not policy codes
// (PasswordPolicyError.Code), carried in ?perror=.
const (
	PasswordErrWrong    = "wrong"    // the current password is not right
	PasswordErrReauth   = "reauth"   // no current password and no passkey confirmation
	PasswordErrMismatch = "mismatch" // the two new passwords differ
	PasswordErrLast     = "last"     // removing it would leave no way to sign in
	PasswordErrNone     = "none"     // there is no password to remove
	PasswordErrFailed   = "failed"
)

// passwordSection is the password part of signInMethodsData.
type passwordSection struct {
	// CanRemovePassword: the account has a password and a usable passkey.
	CanRemovePassword bool
	// CanConfirmWithPasskey: the account has a usable passkey here, so it can
	// confirm a password change with it instead of the current password.
	CanConfirmWithPasskey bool
	// PasskeyConfirmed: a passkey confirmation from this session is waiting
	// to be used (it lasts ReauthProofLifetime and works once).
	PasskeyConfirmed bool
	// CanSetPassword: there is a way to confirm a new password (the current
	// password, or a passkey here).
	CanSetPassword bool
	// NoProofPossible: the account has neither a password nor a passkey
	// usable here, so it cannot confirm any change to its sign-in methods.
	NoProofPossible bool
	// AdminPasswordUnusable: the user is an administrator and passkeys are
	// required for administrators, so a password cannot be used to sign in.
	AdminPasswordUnusable bool
	PasswordNotice        string
	PasswordError         string
	MinPassword           int
	MaxPasswordBytes      int
	// PromptPasskey: after a password sign-in, suggest adding a passkey
	// (dismissible; see passkey_prompt.go).
	PromptPasskey bool
	DismissPath   string
}

var passwordNotices = map[string]string{
	"password-added":   "Password added. You can now sign in with it as well. You have been signed out on your other devices.",
	"password-changed": "Password changed. You have been signed out on your other devices.",
	"password-removed": "Password removed. You sign in with a passkey. You have been signed out on your other devices.",
}

var passwordErrors = map[string]string{
	PasswordErrWrong:    "Your current password is not right.",
	PasswordErrReauth:   "First confirm it's you: type your current password, or confirm with a passkey you already have. A passkey confirmation lasts 5 minutes and works once.",
	PasswordErrMismatch: "The two new passwords are not the same. Type them again.",
	PasswordErrLast:     "Your password is your only working way to sign in, so it can't be removed. Add a passkey first.",
	PasswordErrNone:     "You have no password to remove.",
	PasswordErrFailed:   "Something went wrong. Nothing was changed; please try again.",
}

func passwordErrorMessage(code string) string {
	if msg, ok := passwordErrors[code]; ok {
		return msg
	}
	return PasswordPolicyMessage(code)
}

// fillPasswordSection completes the password part of the page.
func (h *PasskeyHandler) fillPasswordSection(r *http.Request, user *models.User, data *signInMethodsData) {
	q := r.URL.Query()
	usable := 0
	for _, p := range data.Passkeys {
		if !p.Blocked {
			usable++
		}
	}
	done := q.Get("done")
	if strings.HasPrefix(done, "password-") {
		data.PasswordNotice = passwordNotices[done]
	}
	data.PasswordError = passwordErrorMessage(q.Get("perror"))
	data.CanRemovePassword = data.HasPassword && usable > 0
	data.CanConfirmWithPasskey = data.PasskeysAvailable && usable > 0
	data.PasskeyConfirmed = h.passkeys.HasReauthProof(r, user.ID)
	data.CanSetPassword = data.HasPassword || data.CanConfirmWithPasskey
	// NoProofPossible: nothing here can confirm it's the owner (no password,
	// and no usable passkey or no passkeys at this address).
	data.NoProofPossible = !data.CanSetPassword
	if done == "verified" && data.PasskeyConfirmed {
		data.Notice = "Thanks, that's you. For the next 5 minutes you can make one change to your sign-in methods without confirming again."
	}
	data.AdminPasswordUnusable = data.IsAdmin && data.PasskeysAvailable && h.service.AdminPasskeysRequired(r.Context())
	data.MinPassword = MinPasswordLength
	data.MaxPasswordBytes = MaxPasswordBytes
	data.PromptPasskey = q.Get("prompt") != "" && len(data.Passkeys) == 0 && data.PasskeysAvailable
	data.DismissPath = PasskeyPromptDismissPath
}
