// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package setup

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

// codeAlphabet is the setup code's character set: upper-case letters and
// digits without the look-alikes 0/O and 1/I, so a code read off a log line
// is easy to type. 32 symbols = 5 bits each.
const codeAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"

// codeLength is the number of symbols in a setup code (12 x 5 = 60 bits).
const codeLength = 12

// newSetupCode returns a random setup code, grouped for reading as
// XXXX-XXXX-XXXX.
func newSetupCode() (string, error) {
	buf := make([]byte, codeLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	var b strings.Builder
	for i, v := range buf {
		if i > 0 && i%4 == 0 {
			b.WriteByte('-')
		}
		// 256 is a multiple of 32, so v%32 is uniform.
		b.WriteByte(codeAlphabet[int(v)%len(codeAlphabet)])
	}
	return b.String(), nil
}

// normalizeCode makes typed input comparable to a generated code: case,
// spaces and dashes are ignored.
func normalizeCode(code string) string {
	code = strings.ToUpper(code)
	return strings.Map(func(r rune) rune {
		if r == '-' || r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, code)
}

// hashSecret returns the hex SHA-256 of s. Setup codes (60 bits, throttled)
// and tickets (256 bits) are high-entropy random values, so a fast hash is
// enough to keep the stored form useless to someone reading the database.
func hashSecret(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// secretMatches compares a presented secret with a stored hash in constant time.
func secretMatches(presented, storedHash string) bool {
	if storedHash == "" {
		return false
	}
	got := hashSecret(presented)
	return subtle.ConstantTimeCompare([]byte(got), []byte(storedHash)) == 1
}

// newTicket returns a random 256-bit browser ticket.
func newTicket() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// Throttle limits for wrong setup codes. Both windows are fixed-length and
// in memory; a restart clears them but also replaces the code.
const (
	// perClientMaxFailures wrong codes from one client address within
	// throttleWindow lock that address out until the window has passed.
	perClientMaxFailures = 5
	// globalMaxFailures wrong codes from all clients together within
	// throttleWindow lock code entry for everyone (stops guessing spread over
	// many addresses; the owner can restart the app to get a fresh code and a
	// clear throttle).
	globalMaxFailures = 30
	throttleWindow    = 15 * time.Minute
)

// codeThrottle counts wrong setup codes per client address and overall.
type codeThrottle struct {
	mu       sync.Mutex
	byClient map[string][]time.Time
	all      []time.Time
}

func newCodeThrottle() *codeThrottle {
	return &codeThrottle{byClient: map[string][]time.Time{}}
}

// prune drops failures older than the window.
func prune(ts []time.Time, cutoff time.Time) []time.Time {
	i := 0
	for i < len(ts) && !ts[i].After(cutoff) {
		i++
	}
	return ts[i:]
}

// blocked reports whether client may not try a code at now.
func (t *codeThrottle) blocked(client string, now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	cutoff := now.Add(-throttleWindow)
	t.all = prune(t.all, cutoff)
	if len(t.byClient) > 256 {
		for c, ts := range t.byClient {
			if len(prune(ts, cutoff)) == 0 {
				delete(t.byClient, c)
			}
		}
	}
	ts := prune(t.byClient[client], cutoff)
	if len(ts) == 0 {
		delete(t.byClient, client)
	} else {
		t.byClient[client] = ts
	}
	return len(ts) >= perClientMaxFailures || len(t.all) >= globalMaxFailures
}

// fail records a wrong code from client at now.
func (t *codeThrottle) fail(client string, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.byClient[client] = append(t.byClient[client], now)
	t.all = append(t.all, now)
}

// reset clears all recorded failures (a new code was issued).
func (t *codeThrottle) reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.byClient = map[string][]time.Time{}
	t.all = nil
}
