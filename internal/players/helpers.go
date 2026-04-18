// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package players

// initialsFor renders a player's name as initials (e.g. "A.B.") for use on
// shareable token URLs, where full names must never leak.
func initialsFor(first, last string) string {
	out := ""
	if r := firstRune(first); r != "" {
		out += r + "."
	}
	if r := firstRune(last); r != "" {
		out += r + "."
	}
	return out
}

func firstRune(s string) string {
	for _, r := range s {
		return string(r)
	}
	return ""
}
