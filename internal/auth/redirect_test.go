// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import "testing"

func TestSafeRedirectPath(t *testing.T) {
	ok := []string{
		"/",
		"/admin/league",
		"/admin/league/fixtures?week=3&team=2",
		"/admin/league/players#top",
	}
	for _, in := range ok {
		if got := safeRedirectPath(in); got != in {
			t.Errorf("safeRedirectPath(%q) = %q, want it unchanged", in, got)
		}
	}
	bad := []string{
		"",
		"https://evil.example/",
		"http://evil.example",
		"//evil.example/path",
		"/\\evil.example",
		"\\\\evil.example",
		"/admin\\..\\x",
		"evil.example",
		"javascript:alert(1)",
		"/admin\r\nSet-Cookie: x=1",
		"/admin\x00",
		"admin/league",
	}
	for _, in := range bad {
		if got := safeRedirectPath(in); got != "" {
			t.Errorf("safeRedirectPath(%q) = %q, want rejected", in, got)
		}
	}
}
