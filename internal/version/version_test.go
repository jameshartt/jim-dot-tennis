// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package version

import (
	"strings"
	"testing"
)

func TestLongShowsAllThree(t *testing.T) {
	oldV, oldC, oldD := Version, Commit, Date
	defer func() { Version, Commit, Date = oldV, oldC, oldD }()
	Version, Commit, Date = "1.2.3", "0123456789abcdef", "2026-11-21T10:00:00Z"
	got := Long()
	for _, want := range []string{"1.2.3", "0123456789ab", "2026-11-21T10:00:00Z"} {
		if !strings.Contains(got, want) {
			t.Errorf("Long() = %q, missing %q", got, want)
		}
	}
	if strings.Contains(got, "0123456789abc") {
		t.Errorf("Long() = %q, commit should be shortened to 12 characters", got)
	}
	if Short() != "1.2.3" {
		t.Errorf("Short() = %q", Short())
	}
}

func TestUserAgentIsHonest(t *testing.T) {
	old := Version
	defer func() { Version = old }()
	Version = "1.2.3"
	if got, want := UserAgent(), "jim-dot-tennis/1.2.3 (+https://github.com/jameshartt/jim-dot-tennis)"; got != want {
		t.Errorf("UserAgent() = %q, want %q", got, want)
	}
	for _, browser := range []string{"Mozilla", "Chrome", "Safari", "Gecko", "Firefox"} {
		if strings.Contains(UserAgent(), browser) {
			t.Errorf("UserAgent() = %q pretends to be a browser (%s)", UserAgent(), browser)
		}
	}
}
