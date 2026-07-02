// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"strings"
	"testing"
)

func TestRedactToken(t *testing.T) {
	const secret = "super-secret-session-id-value-1234567890"

	got := redactToken(secret)

	// The fingerprint must never contain the raw token (that is the whole point).
	if strings.Contains(got, secret) {
		t.Fatalf("redactToken leaked the raw token: %q", got)
	}
	if strings.Contains(secret, got) {
		t.Fatalf("redactToken output is a prefix of the token: %q", got)
	}

	// It must be deterministic so log lines correlate across a session.
	if again := redactToken(secret); again != got {
		t.Fatalf("redactToken not deterministic: %q != %q", got, again)
	}

	// Different tokens must produce different fingerprints.
	if other := redactToken(secret + "x"); other == got {
		t.Fatal("redactToken collided for distinct tokens")
	}

	if !strings.HasPrefix(got, "sha256:") {
		t.Fatalf("expected sha256: prefix, got %q", got)
	}

	if empty := redactToken(""); empty != "<empty>" {
		t.Fatalf("expected <empty> for empty token, got %q", empty)
	}
}
