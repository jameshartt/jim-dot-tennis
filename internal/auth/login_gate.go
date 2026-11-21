// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// What keeps password sign-in bounded under a burst of requests.
//
// Two things used to be unbounded. The throttle was read, then the password
// was compared (about 200 ms of CPU), then the failure was recorded, so any
// number of guesses sent at the same moment all read "not throttled yet" and
// were all checked. And every one of those compares ran at once, which pins
// a small host.
//
//   - addressLocks serialises password sign-in per client address
//     (ThrottleKey): the throttle check, the compare and the record of one
//     attempt finish before the next attempt from that address reads the
//     throttle. Both limits (per username and address, and per address) only
//     ever count one address, so serialising per address makes both exact.
//     Other addresses are not held up, so nobody can use this to keep a named
//     person from signing in from somewhere else, and passkey sign-in does not
//     come through here at all. The lock is in this process's memory, which
//     is enough: the only other process that opens the database (the command
//     line) never signs anyone in.
//   - bcryptSlots bounds how many bcrypt operations (compares and hashes) run
//     at the same time, across every caller in the process.
//
// A request that cannot get its turn within loginWait, or that finds too many
// requests already waiting, is refused with ErrLoginBusy. It is not counted
// as a failed attempt: nothing was checked.

const (
	// MaxUsernameBytes is the longest username sign-in looks at. Account
	// names are at most 64 characters; anything longer cannot be an account,
	// so it is refused without a password compare and stored cut to this.
	MaxUsernameBytes = 128
	// MaxUserAgentBytes is how much of a User-Agent header is stored (login
	// attempts and sessions).
	MaxUserAgentBytes = 256
	// MaxLoginBody bounds the body of a sign-in form post.
	MaxLoginBody = 16 << 10

	// passwordCompareSlots is how many bcrypt operations may run at once.
	passwordCompareSlots = 2
	// maxLoginWaitersPerAddress bounds the sign-ins of one address that may
	// be waiting or running at the same time.
	maxLoginWaitersPerAddress = 8
	// maxCompareWaiters bounds the sign-ins of all addresses that may be
	// waiting for a bcrypt slot.
	maxCompareWaiters = 64
)

// loginWait is how long a password sign-in waits for its turn (a variable
// so tests can shorten it).
var loginWait = 5 * time.Second

// ErrLoginBusy means the sign-in was not attempted because too many others
// were in progress. Nothing was checked and nothing was recorded.
var ErrLoginBusy = errors.New("the site is busy signing people in; please try again in a moment")

// bcryptSlots is the process-wide bound on concurrent bcrypt work.
var bcryptSlots = make(chan struct{}, passwordCompareSlots)

// acquireBcryptSlot waits for a slot until ctx is done. The caller must call
// releaseBcryptSlot when it reports true.
func acquireBcryptSlot(ctx context.Context) bool {
	select {
	case bcryptSlots <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

func releaseBcryptSlot() { <-bcryptSlots }

// compareWaiters bounds the sign-ins queued for a bcrypt slot.
type compareWaiters struct {
	mu sync.Mutex
	n  int
}

func (w *compareWaiters) enter() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.n >= maxCompareWaiters {
		return false
	}
	w.n++
	return true
}

func (w *compareWaiters) leave() {
	w.mu.Lock()
	w.n--
	w.mu.Unlock()
}

// comparePasswordBounded is PasswordMatches for sign-in: it gives up with
// ErrLoginBusy rather than wait without limit for a bcrypt slot.
func (s *Service) comparePasswordBounded(ctx context.Context, hash, password string) (bool, error) {
	if !s.compareQueue.enter() {
		return false, ErrLoginBusy
	}
	defer s.compareQueue.leave()
	if !acquireBcryptSlot(ctx) {
		return false, ErrLoginBusy
	}
	defer releaseBcryptSlot()
	s.compares.Add(1)
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil, nil
}

// addressLocks is one lock per client address, created on demand and dropped
// when nobody holds or waits for it.
type addressLocks struct {
	mu    sync.Mutex
	byKey map[string]*addressLock
}

type addressLock struct {
	held chan struct{} // capacity 1: holding it is holding the lock
	refs int           // holders plus waiters
}

func newAddressLocks() *addressLocks {
	return &addressLocks{byKey: map[string]*addressLock{}}
}

// acquire takes key's lock, waiting until ctx is done. ok is false when ctx
// ended first or maxLoginWaitersPerAddress are already queued for key; the
// caller must call release when ok is true.
func (l *addressLocks) acquire(ctx context.Context, key string) (release func(), ok bool) {
	l.mu.Lock()
	lock := l.byKey[key]
	if lock == nil {
		lock = &addressLock{held: make(chan struct{}, 1)}
		l.byKey[key] = lock
	}
	if lock.refs >= maxLoginWaitersPerAddress {
		l.mu.Unlock()
		return nil, false
	}
	lock.refs++
	l.mu.Unlock()

	drop := func() {
		l.mu.Lock()
		lock.refs--
		if lock.refs == 0 {
			delete(l.byKey, key)
		}
		l.mu.Unlock()
	}
	select {
	case lock.held <- struct{}{}:
		return func() {
			<-lock.held
			drop()
		}, true
	case <-ctx.Done():
		drop()
		return nil, false
	}
}

// dummyPasswordHash is a bcrypt hash at BcryptCost of a random password that
// was thrown away. Sign-in compares against it when there is no real hash to
// compare with (no such account, an inactive one, or one with no password),
// so those answers take as long as a wrong password for a real account and
// the response time does not say which usernames exist. It is fixed rather
// than made at startup so the first such sign-in is not slower than the
// rest. TestDummyPasswordHashCost keeps its cost equal to BcryptCost. It is
// a variable only so tests can swap in a cheap one.
var dummyPasswordHash = "$2a$12$VDkc/jU7QBkNExpPOU4X6e7763aYuzuEnowlAmQMHT4ObvtS5eiiC"

// clip cuts s to at most max bytes without splitting a UTF-8 sequence.
func clip(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return strings.ToValidUTF8(s[:max], "")
}

// ClipUserAgent is the part of a User-Agent header that is stored.
func ClipUserAgent(ua string) string { return clip(ua, MaxUserAgentBytes) }

// ClipUsername is the part of a submitted username that is stored.
func ClipUsername(name string) string { return clip(name, MaxUsernameBytes) }
