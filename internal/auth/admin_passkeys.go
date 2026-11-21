// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"context"
	"log"
	"net/http"

	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
)

// "Require passkeys for administrators" at sign-in.
//
// While the setting is on (repository.SettingRequireAdminPasskeys) an
// account with the admin role cannot sign in with its password: the attempt
// fails exactly like a wrong password (same message, same throttling, and
// the password is still checked first so the timing is the same), and the
// login page never says which usernames are administrators. The page does
// say, to everyone, that administrators sign in with a passkey.
//
// The requirement applies only where passkeys can be used. If the instance
// has no usable public URL (so no passkey can work), administrators may
// sign in with a password again; otherwise removing the public URL would
// lock every administrator out. In production the public URL is required at
// startup, so this matters only for a stored URL cleared on the settings page
// (which the page refuses while the setting is on) and for development.

// AdminPasskeysRequired reports whether the setting is on. It reads the
// database every time, so a change made by the server command line applies
// at once. A read error counts as "on" (the safe answer for sign-in).
func (s *Service) AdminPasskeysRequired(ctx context.Context) bool {
	on, err := repository.AdminPasskeysRequired(ctx, s.db)
	if err != nil {
		log.Printf("Could not read the administrator passkey setting (treating it as on): %v", err)
		return true
	}
	return on
}

// PasskeysAvailable reports whether passkeys can be used on this request's
// address (false when no passkey service is wired up).
func (s *Service) PasskeysAvailable(r *http.Request) bool {
	return s.passkeysAvailable != nil && s.passkeysAvailable(r)
}

// AdminPasswordSignInBlocked reports whether user, signing in on r, may not
// use a password because passkeys are required for administrators.
func (s *Service) AdminPasswordSignInBlocked(r *http.Request, user *models.User) bool {
	return user.Role == models.RoleAdmin && s.PasskeysAvailable(r) && s.AdminPasskeysRequired(r.Context())
}

// passwordSignInAllowed reports whether user, whose password has just been
// found right, may sign in with it: not an administrator while passkeys are
// required for administrators. Login asks only after the compare, so the
// refusal takes as long as a wrong password.
func (s *Service) passwordSignInAllowed(r *http.Request, user *models.User) bool {
	if s.AdminPasswordSignInBlocked(r, user) {
		log.Printf("Password sign-in refused for administrator ID %d: passkeys are required for administrators", user.ID)
		return false
	}
	return true
}

// rehash stores password hashed at BcryptCost in place of oldHash, unless the
// hash has changed in the meantime. A failure is logged; sign-in goes on.
func (s *Service) rehash(ctx context.Context, userID int64, oldHash, password string) {
	newHash, err := HashPassword(password)
	if err != nil {
		log.Printf("Could not upgrade the password hash of user ID %d: %v", userID, err)
		return
	}
	res, err := s.db.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ? AND password_hash = ?`, newHash, userID, oldHash)
	if err != nil {
		log.Printf("Could not upgrade the password hash of user ID %d: %v", userID, err)
		return
	}
	if n, _ := res.RowsAffected(); n == 1 {
		log.Printf("Upgraded the password hash of user ID %d to cost %d", userID, BcryptCost)
	}
}
