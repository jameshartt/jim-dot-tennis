// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
)

// Passkey ceremony limits.
const (
	// ceremonyLifetime is how long a begun registration or sign-in may take.
	// It is also the timeout the browser is given.
	ceremonyLifetime = 5 * time.Minute
	// maxCeremonies bounds the in-memory store. When it is full the ceremony
	// closest to expiry is dropped.
	maxCeremonies = 5000
	// ceremonyCookie carries the random ceremony ticket that binds a
	// ceremony to the browser that began it.
	ceremonyCookie = "passkey_ceremony"
)

// ceremonyKind separates registration from sign-in so a ticket for one can
// never finish the other.
type ceremonyKind int

const (
	ceremonyRegister ceremonyKind = iota + 1
	ceremonyLogin
)

// ceremony is the server's half of a begun WebAuthn ceremony: the challenge
// and what it was issued for.
type ceremony struct {
	kind    ceremonyKind
	data    webauthn.SessionData
	origin  string // the origin the ceremony was begun for
	userID  int64  // registration: the signed-in user who began it
	session string // registration: fingerprint of that user's session cookie
	proof   string // registration: how ownership was proven at the start (passkey.go)
	expires time.Time
}

// ceremonyStore holds begun ceremonies in memory, keyed by the hash of a
// random ticket that only the browser which began the ceremony holds (in an
// HttpOnly, SameSite=Strict cookie). Each ceremony is taken out on the first
// finish attempt, whatever the outcome, so a challenge is never used twice.
// The store is per process: a restart drops ceremonies in flight, which only
// means starting the passkey prompt again.
type ceremonyStore struct {
	mu    sync.Mutex
	byKey map[[sha256.Size]byte]ceremony
}

func newCeremonyStore() *ceremonyStore {
	return &ceremonyStore{byKey: map[[sha256.Size]byte]ceremony{}}
}

// put stores c and returns the ticket for the browser.
func (s *ceremonyStore) put(c ceremony, now time.Time) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	ticket := base64.RawURLEncoding.EncodeToString(raw)
	c.expires = now.Add(ceremonyLifetime)

	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.byKey) >= maxCeremonies {
		s.evictLocked(now)
	}
	s.byKey[sha256.Sum256([]byte(ticket))] = c
	return ticket, nil
}

// evictLocked removes expired ceremonies and, if the store is still full,
// the one closest to expiry.
func (s *ceremonyStore) evictLocked(now time.Time) {
	var oldestKey [sha256.Size]byte
	var oldest time.Time
	for k, c := range s.byKey {
		if !c.expires.After(now) {
			delete(s.byKey, k)
			continue
		}
		if oldest.IsZero() || c.expires.Before(oldest) {
			oldest, oldestKey = c.expires, k
		}
	}
	if len(s.byKey) >= maxCeremonies {
		delete(s.byKey, oldestKey)
	}
}

// take removes and returns the ceremony for ticket. ok is false when there is
// none, it has expired, or it is of another kind; the ceremony is removed in
// every case.
func (s *ceremonyStore) take(ticket string, kind ceremonyKind, now time.Time) (ceremony, bool) {
	if ticket == "" {
		return ceremony{}, false
	}
	key := sha256.Sum256([]byte(ticket))
	s.mu.Lock()
	c, found := s.byKey[key]
	delete(s.byKey, key)
	s.mu.Unlock()
	if !found || c.kind != kind || !c.expires.After(now) {
		return ceremony{}, false
	}
	return c, true
}

// windowLimiter counts events per key (a client address) in a sliding
// window. It is keyed by address only, never by username, so nobody can use
// it to lock a named user out.
type windowLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	events map[string][]time.Time
}

func newWindowLimiter(limit int, window time.Duration) *windowLimiter {
	return &windowLimiter{limit: limit, window: window, events: map[string][]time.Time{}}
}

func (l *windowLimiter) recent(key string, now time.Time) []time.Time {
	cutoff := now.Add(-l.window)
	kept := l.events[key][:0]
	for _, t := range l.events[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(l.events, key)
		return nil
	}
	l.events[key] = kept
	return kept
}

// blocked reports whether key has reached the limit within the window.
func (l *windowLimiter) blocked(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.recent(key, now)) >= l.limit
}

// record counts one event for key.
func (l *windowLimiter) record(key string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.events) > 20000 {
		// Many distinct addresses: drop everything that has aged out.
		for k := range l.events {
			l.recent(k, now)
		}
	}
	l.events[key] = append(l.recent(key, now), now)
}

// hit counts one event for key and reports whether it was within the limit.
func (l *windowLimiter) hit(key string, now time.Time) bool {
	if l.blocked(key, now) {
		return false
	}
	l.record(key, now)
	return true
}
