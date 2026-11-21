// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"jim-dot-tennis/internal/models"
)

// Proof of ownership.
//
// A session says "this browser signed in at some point". It is not enough to
// change how an account signs in: someone who gets hold of a session (an
// unlocked laptop, a stolen cookie) could otherwise add their own passkey,
// remove the owner's, and keep the account for good. So every change to an
// account's sign-in methods needs fresh proof that the person at the keyboard
// is the owner:
//
//	add a passkey, remove a passkey          (this package)
//	add, change or remove the password       (internal/accounts)
//
// The proof is one of:
//
//   - the account's current password, typed now (CheckCurrentPassword, limited
//     to currentPasswordFailureLimit wrong tries per address), or
//   - a passkey confirmation made in this session in the last few minutes
//     (reauth.go), from a passkey the account already had: a passkey added
//     through a session cannot confirm anything for
//     NewPasskeyQuarantine, nor ever in the session that added it. Without
//     that, a proof from another passkey could be stretched: add a passkey,
//     then use the new one for everything else.
//
// There is no exemption for a new account. Someone who has just set a
// password (first-run setup, an invite or recovery link) proves ownership
// with that password; someone whose link enrolled a passkey proves it with
// that passkey (a passkey from a link is not quarantined: the link, not a
// session, authorised it). An account with no usable sign-in method at all
// (its only passkey is blocked for a security check) cannot prove anything
// and is deliberately not let off: an administrator clears the block or
// makes a recovery link.
//
// After any such change every other session of the account is ended
// (EndOtherSessions); the browser that made the change stays signed in.

// Wrong current passwords typed to confirm a change are limited per client
// address (in memory), so a stolen session cannot be used to guess the
// password at leisure. One limiter serves every form that takes the current
// password.
const (
	currentPasswordFailureLimit  = 5
	currentPasswordFailureWindow = 15 * time.Minute
)

// NewPasskeyQuarantine is how long a passkey added from a signed-in session
// cannot be used to confirm a change to the account's sign-in methods.
const NewPasskeyQuarantine = 10 * time.Minute

var (
	// ErrProofRequired: the change needs the current password or a passkey
	// confirmation, and neither was given (or the confirmation had expired).
	ErrProofRequired = errors.New("confirm it's you first, with your current password or a passkey")
	// ErrWrongCurrentPassword: the current password given as proof is wrong.
	ErrWrongCurrentPassword = errors.New("your current password is not right")
	// ErrProofThrottled: too many wrong current passwords from this address.
	ErrProofThrottled = errors.New("too many wrong passwords; wait a few minutes and try again")
	// ErrPasskeyTooNew: the passkey was added from a session too recently
	// (or in this very session) to confirm a change.
	ErrPasskeyTooNew = errors.New("that passkey was only just added, so it can't confirm it's you yet")
	// ErrUserVerificationRequired: an administrator's passkey answered
	// without verifying the person (no PIN, fingerprint, face or screen
	// lock).
	ErrUserVerificationRequired = errors.New("administrators need a passkey that checks a PIN, fingerprint, face or screen lock")
)

// Proof methods, as recorded in the audit log.
const (
	ProofPassword = "password"
	ProofPasskey  = "passkey"
)

// CurrentPasswordBlocked reports whether the address behind r has used up
// its wrong current passwords.
func (s *Service) CurrentPasswordBlocked(r *http.Request) bool {
	return s.currentPassword.Blocked(r, time.Now())
}

// CurrentPasswordFailed counts one wrong current password for the address
// behind r.
func (s *Service) CurrentPasswordFailed(r *http.Request) {
	s.currentPassword.Fail(r, time.Now())
}

// CheckCurrentPassword reports whether password is userID's current
// password: nil, ErrWrongCurrentPassword (counted against the address) or
// ErrProofThrottled. An account with no password always gets
// ErrWrongCurrentPassword, after a compare of the usual length.
func (s *Service) CheckCurrentPassword(r *http.Request, userID int64, password string) error {
	if s.CurrentPasswordBlocked(r) {
		return ErrProofThrottled
	}
	var stored models.PasswordHash
	err := s.db.GetContext(r.Context(), &stored, `SELECT password_hash FROM users WHERE id = ? AND is_active = TRUE`, userID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read password: %w", err)
	}
	hash, real := string(stored), err == nil && stored != ""
	if !real {
		hash = dummyPasswordHash
	}
	if !PasswordMatches(hash, password) || !real {
		s.CurrentPasswordFailed(r)
		return ErrWrongCurrentPassword
	}
	return nil
}

// ProveOwnership checks the proof sent with a request to change user's
// sign-in methods and returns how it was proven (ProofPassword or
// ProofPasskey). currentPassword is what the person typed ("" for none); with
// none, the request must carry a passkey confirmation made in this session,
// which is used up. Errors: ErrProofRequired, ErrWrongCurrentPassword,
// ErrProofThrottled.
func (p *Passkeys) ProveOwnership(w http.ResponseWriter, r *http.Request, user *models.User, currentPassword string) (string, error) {
	if currentPassword != "" {
		if err := p.service.CheckCurrentPassword(r, user.ID, currentPassword); err != nil {
			return "", err
		}
		return ProofPassword, nil
	}
	if p.ConsumeReauthProof(w, r, user.ID) {
		return ProofPasskey, nil
	}
	return "", ErrProofRequired
}

// EndOtherSessions ends every session of userID except keepSessionID (the
// session making the request; 0 ends them all) and returns how many it
// ended. It is called after a sign-in method was added or removed, and by
// "Sign out everywhere else".
func (s *Service) EndOtherSessions(ctx context.Context, userID, keepSessionID int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE sessions SET is_valid = FALSE
		WHERE user_id = ? AND id <> ? AND is_valid = TRUE`, userID, keepSessionID)
	if err != nil {
		return 0, fmt.Errorf("end other sessions: %w", err)
	}
	return res.RowsAffected()
}

// HasPassword reports whether userID has a password.
func (s *Service) HasPassword(ctx context.Context, userID int64) (bool, error) {
	var has bool
	err := s.db.GetContext(ctx, &has, `SELECT COALESCE(password_hash, '') <> '' FROM users WHERE id = ?`, userID)
	return has, err
}

// sessionEnrolment is a passkey added from a signed-in session.
type sessionEnrolment struct {
	session string // fingerprint of the session that added it
	at      time.Time
}

// enrolmentLog remembers, in memory, the passkeys added from signed-in
// sessions (by passkey row id), so they can be kept from confirming changes
// for a while. A restart forgets it, which only matters to someone who could
// already add a passkey without proof, and adding one needs proof.
type enrolmentLog struct {
	mu   sync.Mutex
	byID map[int64]sessionEnrolment
}

// maxEnrolmentLog bounds enrolmentLog. Entries older than
// enrolmentMemory are dropped first.
const (
	maxEnrolmentLog = 10000
	enrolmentMemory = 31 * 24 * time.Hour // longer than any session lives
)

func newEnrolmentLog() *enrolmentLog {
	return &enrolmentLog{byID: map[int64]sessionEnrolment{}}
}

func (l *enrolmentLog) add(passkeyID int64, session string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.byID) >= maxEnrolmentLog {
		var oldestID int64
		var oldest time.Time
		for id, e := range l.byID {
			if now.Sub(e.at) > enrolmentMemory {
				delete(l.byID, id)
				continue
			}
			if oldest.IsZero() || e.at.Before(oldest) {
				oldest, oldestID = e.at, id
			}
		}
		if len(l.byID) >= maxEnrolmentLog {
			delete(l.byID, oldestID)
		}
	}
	l.byID[passkeyID] = sessionEnrolment{session: session, at: now}
}

// quarantined reports whether passkeyID was added from a session less than
// NewPasskeyQuarantine ago, or from the session whose fingerprint is session.
func (l *enrolmentLog) quarantined(passkeyID int64, session string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.byID[passkeyID]
	if !ok {
		return false
	}
	return now.Sub(e.at) < NewPasskeyQuarantine || (session != "" && e.session == session)
}
