// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"time"
)

// Prompting people who sign in with only a password to add a passkey.
//
// After a password sign-in, an account with no passkey is sent to the
// sign-in methods page with an "add a passkey?" box (prompt=1) and a
// Continue link to where it was going, but only where passkeys can be used
// and unless this browser said "Not now" in the last passkeyPromptQuiet. It
// is never required: Continue and "Not now" both carry on.

// PasskeyPromptDismissPath records "Not now" for the passkey prompt (POST, signed in).
const PasskeyPromptDismissPath = "/account/passkey-prompt/dismiss"

const (
	passkeyPromptCookie = "passkey_prompt_dismissed"
	passkeyPromptQuiet  = 30 * 24 * time.Hour
)

// passkeyPromptTarget returns where a password sign-in of userID should land:
// target, or the sign-in methods page offering a passkey first.
func (h *Handler) passkeyPromptTarget(r *http.Request, userID int64, target string) string {
	if h.passkeys == nil || !h.passkeys.Available(r) {
		return target
	}
	if c, err := r.Cookie(passkeyPromptCookie); err == nil && c.Value != "" {
		return target
	}
	n, err := h.service.passkeyCount(r.Context(), userID)
	if err != nil {
		log.Printf("Passkey prompt: count passkeys of user ID %d: %v", userID, err)
		return target
	}
	if n > 0 {
		return target
	}
	return SignInMethodsPath + "?prompt=1&next=" + url.QueryEscape(target)
}

func (s *Service) passkeyCount(ctx context.Context, userID int64) (int, error) {
	var n int
	err := s.db.GetContext(ctx, &n, `SELECT COUNT(*) FROM webauthn_credentials WHERE user_id = ?`, userID)
	return n, err
}

// dismissPasskeyPrompt remembers "Not now" in this browser and carries on.
func (h *PasskeyHandler) dismissPasskeyPrompt(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     passkeyPromptCookie,
		Value:    "1",
		Path:     "/",
		MaxAge:   int(passkeyPromptQuiet / time.Second),
		HttpOnly: true,
		Secure:   h.service.config.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	next := safeRedirectPath(r.PostFormValue("next"))
	if next == "" {
		next = h.redirectPath
		if user, err := GetUserFromContext(r.Context()); err == nil {
			next = HomePathFor(user, h.redirectPath)
		}
	}
	http.Redirect(w, r, next, http.StatusSeeOther)
}
