// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"context"
	"net"
	"net/http"
	"time"

	"jim-dot-tennis/internal/models"
)

// Password sign-in throttling. Two limits apply, both counted from
// login_attempts over Config.LoginAttemptWindow (15 minutes by default):
//
//   - per username and address: Config.MaxLoginAttempts (5) failures for one
//     name from one address block that name from that address only;
//   - per address: MaxLoginFailuresPerAddress failures against any names
//     block password sign-in from that address (one password sprayed across
//     many usernames).
//
// Neither limit ever looks at failures from other addresses, so nobody can
// lock a named user out of signing in from somewhere else; and neither is
// consulted by passkey sign-in (its own limiter, in memory, counts passkey
// failures by address only), so password failures against a name never block
// that person's passkey. A refused (throttled) attempt is not recorded, so
// retrying while blocked does not extend the block.
//
// The per-address limit is well above the per-name one so that a club house
// or office sharing one address is not caught by its members' typos.
const MaxLoginFailuresPerAddress = 30

// ThrottleKey is the client address a request is throttled under: the
// address from clientIP, except that an IPv6 address is reduced to its /64
// network (one household or server is normally given a whole /64, so
// counting single IPv6 addresses would let one client rotate through 2^64 of
// them). login_attempts.ip stores this key.
func ThrottleKey(r *http.Request) string { return throttleKey(clientIP(r)) }

// throttleKey maps a client address to its throttle key (see ThrottleKey).
func throttleKey(addr string) string {
	ip := net.ParseIP(addr)
	if ip == nil {
		return addr
	}
	if v4 := ip.To4(); v4 != nil {
		return v4.String()
	}
	network := &net.IPNet{IP: ip.Mask(net.CIDRMask(64, 128)), Mask: net.CIDRMask(64, 128)}
	return network.String()
}

// loginThrottled reports whether a password sign-in for username from the
// request's address must be refused without checking the password.
func (s *Service) loginThrottled(username string, r *http.Request) (bool, error) {
	key := ThrottleKey(r)
	cutoff := time.Now().Add(-s.config.LoginAttemptWindow)
	recentFailures := func(attempts []models.LoginAttempt) int {
		n := 0
		for _, a := range attempts {
			if !a.Success && a.CreatedAt.After(cutoff) {
				n++
			}
		}
		return n
	}

	// The latest attempts for this name from this address: a success among
	// them means fewer than the limit are failures, so it lifts the block.
	byName, err := s.loginAttemptRepository.FindByUsernameAndIP(username, key, s.config.MaxLoginAttempts)
	if err != nil {
		return false, err
	}
	if recentFailures(byName) >= s.config.MaxLoginAttempts {
		return true, nil
	}

	byAddress, err := s.loginAttemptRepository.FindPasswordFailuresByIP(key, MaxLoginFailuresPerAddress)
	if err != nil {
		return false, err
	}
	return recentFailures(byAddress) >= MaxLoginFailuresPerAddress, nil
}

// Sign-in methods passed to the sign-in audit hook.
const (
	SignInPassword = "password"
	SignInPasskey  = "passkey"
	// SignInNewSession: StartSession, after first-run setup created the
	// account or an invite or recovery link was used.
	SignInNewSession = "setup_or_link"
)

// SignInAudit records a successful sign-in. It is called for administrators
// only, after the session has been created; it must not block or fail the
// sign-in.
type SignInAudit func(ctx context.Context, user *models.User, method, address string)

// SetSignInAudit sets where administrators' sign-ins are recorded (main.go
// wires it to the account audit log). Without it nothing is recorded.
func (s *Service) SetSignInAudit(audit SignInAudit) { s.signInAudit = audit }

func (s *Service) auditSignIn(ctx context.Context, user *models.User, method string, r *http.Request) {
	if s.signInAudit == nil || user.Role != models.RoleAdmin {
		return
	}
	s.signInAudit(ctx, user, method, clientIP(r))
}
