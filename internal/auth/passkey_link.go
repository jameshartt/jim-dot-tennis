// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"net/http"
	"strings"
	"time"

	"jim-dot-tennis/internal/models"

	"github.com/go-webauthn/webauthn/protocol"
)

// AccountLinkPathPrefix is the prefix of invite and recovery link URLs
// (internal/accounts serves them). What follows it is a secret token.
const AccountLinkPathPrefix = "/account/link/"

// RedactPath returns path in a form that is safe to log: the secret token of
// an account link URL is replaced. Anything that logs a request path which
// may be an account link must pass it through here.
func RedactPath(path string) string {
	if strings.HasPrefix(path, AccountLinkPathPrefix) {
		return AccountLinkPathPrefix + "[redacted]"
	}
	return path
}

func (p *Passkeys) setCeremonyCookie(w http.ResponseWriter, ticket string) {
	http.SetCookie(w, &http.Cookie{
		Name:     ceremonyCookie,
		Value:    ticket,
		Path:     "/",
		MaxAge:   int(ceremonyLifetime / time.Second),
		HttpOnly: true,
		Secure:   p.service.config.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

// takeCeremonyCookie returns the browser's ceremony ticket and clears the cookie.
func (p *Passkeys) takeCeremonyCookie(w http.ResponseWriter, r *http.Request) string {
	http.SetCookie(w, &http.Cookie{
		Name:     ceremonyCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   p.service.config.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
	c, err := r.Cookie(ceremonyCookie)
	if err != nil {
		return ""
	}
	return c.Value
}

// BeginLinkRegistration starts enrolling a passkey for user on behalf of
// someone who holds an invite or recovery link but has no session yet. It
// sets the ceremony cookie on w and returns the options for
// navigator.credentials.create().
//
// binding takes the place a session fingerprint has in a signed-in
// enrolment: a value only the holder of that link can produce (derived from
// the link's token; it must not be empty). The ceremony can then only be
// finished by a request that (1) carries this browser's ceremony cookie and
// (2) presents the same link, for (3) the same user. Nothing is stored and
// the link is not touched; an abandoned ceremony simply expires.
func (p *Passkeys) BeginLinkRegistration(w http.ResponseWriter, r *http.Request, user *models.User, binding string) (*protocol.CredentialCreation, error) {
	if binding == "" {
		return nil, ErrPasskeyCeremony
	}
	options, ticket, err := p.BeginRegistration(r, user, binding, proofLink)
	if err != nil {
		return nil, err
	}
	p.setCeremonyCookie(w, ticket)
	return options, nil
}

// VerifyLinkRegistration verifies the browser's response (the request body)
// to BeginLinkRegistration and returns the new passkey WITHOUT storing it:
// the caller stores it in the same transaction that consumes the link, so a
// link that loses a race leaves no passkey behind. The ceremony is consumed
// whatever the outcome; the link is not.
func (p *Passkeys) VerifyLinkRegistration(w http.ResponseWriter, r *http.Request, user *models.User, binding, name string) (*models.WebAuthnCredential, error) {
	ticket := p.takeCeremonyCookie(w, r)
	if binding == "" {
		return nil, ErrPasskeyCeremony
	}
	m, proof, err := p.verifyRegistration(r, user, binding, ticket, http.MaxBytesReader(w, r.Body, maxPasskeyBody), name)
	if err != nil {
		return nil, err
	}
	if proof != proofLink {
		return nil, ErrPasskeyCeremony
	}
	return m, nil
}

// WritePasskeyFailure writes err as the JSON error body the passkey script
// understands, with the matching status.
func WritePasskeyFailure(w http.ResponseWriter, err error) { writePasskeyFailure(w, err) }

// WritePasskeyJSON writes v as a JSON response that is never cached.
func WritePasskeyJSON(w http.ResponseWriter, status int, v any) { writePasskeyJSON(w, status, v) }

// RequireJSON refuses (415) a request whose body is not declared as JSON,
// which a cross-site form cannot send. It reports whether to carry on.
func RequireJSON(w http.ResponseWriter, r *http.Request) bool { return requireJSON(w, r) }

// AttemptLimiter counts failed attempts per client address in a sliding
// window, in memory. It is keyed by address only, so one client's failures
// never block another.
type AttemptLimiter struct{ l *windowLimiter }

// NewAttemptLimiter allows limit failures per address within window.
func NewAttemptLimiter(limit int, window time.Duration) *AttemptLimiter {
	return &AttemptLimiter{l: newWindowLimiter(limit, window)}
}

// Blocked reports whether the address behind r has used up its failures.
func (a *AttemptLimiter) Blocked(r *http.Request, now time.Time) bool {
	return a.l.blocked(ThrottleKey(r), now)
}

// Fail counts one failure for the address behind r.
func (a *AttemptLimiter) Fail(r *http.Request, now time.Time) {
	a.l.record(ThrottleKey(r), now)
}
