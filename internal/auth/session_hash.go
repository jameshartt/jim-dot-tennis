// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
)

// HashSessionToken is the key a session is stored under: the lowercase hex
// SHA-256 of the token in the browser's cookie. Only this is stored
// (sessions.token_hash, migration 035), so a copy of the database holds no
// usable session. The token is 32 random bytes, so a plain unsalted hash is
// enough: there is nothing to guess.
func HashSessionToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// SessionContextKey is the request context key for the numeric id of the
// session RequireAuth validated (an int64).
const SessionContextKey ContextKey = "session_id"

// SessionIDFromContext returns the numeric id of the signed-in request's
// session (set by RequireAuth), or 0. The id names a session on admin pages
// and is not a credential.
func SessionIDFromContext(ctx context.Context) int64 {
	id, _ := ctx.Value(SessionContextKey).(int64)
	return id
}
