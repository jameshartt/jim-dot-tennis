// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package normalize

import "strings"

// Apostrophes normalizes all known apostrophe-like Unicode characters to standard ASCII apostrophe (U+0027).
// This does NOT remove apostrophes — it standardizes them for storage and display.
func Apostrophes(s string) string {
	// HTML entity first (before any other replacement)
	s = strings.ReplaceAll(s, "&#039;", "'")
	// Right single quotation mark U+2019 (most common in web content)
	s = strings.ReplaceAll(s, "\u2019", "'")
	// Left single quotation mark U+2018
	s = strings.ReplaceAll(s, "\u2018", "'")
	// Modifier letter apostrophe U+02BC
	s = strings.ReplaceAll(s, "\u02BC", "'")
	// Prime U+2032
	s = strings.ReplaceAll(s, "\u2032", "'")
	// Grave accent U+0060
	s = strings.ReplaceAll(s, "`", "'")
	return s
}

// ForComparison normalizes a string for fuzzy matching: lowercases, normalizes apostrophes,
// removes all apostrophes, removes periods, and collapses whitespace.
func ForComparison(s string) string {
	s = Apostrophes(s)
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "'", "")
	s = strings.ReplaceAll(s, ".", "")
	s = strings.Join(strings.Fields(s), " ")
	return s
}
