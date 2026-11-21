// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package services

import (
	"strings"
	"testing"
	"time"
)

func TestICalUIDUsesPublicHost(t *testing.T) {
	ev := ICalEvent{FixtureID: 42, StartTime: time.Now(), EndTime: time.Now().Add(time.Hour), UIDDomain: "jim.tennis"}
	// Existing subscriptions use fixture-<id>@<public host>; it must not change.
	if out := GenerateICalEvent(ev); !strings.Contains(out, "UID:fixture-42@jim.tennis\r\n") {
		t.Errorf("UID not derived from the domain:\n%s", out)
	}
	ev.UIDDomain = "tennis.example.org"
	if out := GenerateICalEvent(ev); !strings.Contains(out, "UID:fixture-42@tennis.example.org\r\n") {
		t.Errorf("UID did not follow the domain:\n%s", out)
	}
}
