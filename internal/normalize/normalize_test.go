// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package normalize

import "testing"

func TestApostrophes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"ASCII apostrophe unchanged", "St Ann's", "St Ann's"},
		{"right single quote U+2019", "St Ann\u2019s", "St Ann's"},
		{"left single quote U+2018", "St Ann\u2018s", "St Ann's"},
		{"modifier letter U+02BC", "St Ann\u02BCs", "St Ann's"},
		{"prime U+2032", "St Ann\u2032s", "St Ann's"},
		{"grave accent", "St Ann`s", "St Ann's"},
		{"HTML entity", "St Ann&#039;s", "St Ann's"},
		{"mixed variants", "O\u2019Brien\u2018s", "O'Brien's"},
		{"no apostrophes", "Hove Park", "Hove Park"},
		{"empty string", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Apostrophes(tt.input)
			if got != tt.want {
				t.Errorf("Apostrophes(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestForComparison(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"basic name", "St Ann's", "st anns"},
		{"right single quote", "St Ann\u2019s", "st anns"},
		{"left single quote", "St Ann\u2018s", "st anns"},
		{"modifier letter", "St Ann\u02BCs", "st anns"},
		{"with periods", "St. Ann's", "st anns"},
		{"mixed case", "ST ANN'S Tennis Club", "st anns tennis club"},
		{"extra whitespace", "  St   Ann's  ", "st anns"},
		{"O'Brien variants match", "O'Brien", "obrien"},
		{"O'Brien curly matches", "O\u2019Brien", "obrien"},
		{"HTML entity", "Queen&#039;s", "queens"},
		{"empty string", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ForComparison(tt.input)
			if got != tt.want {
				t.Errorf("ForComparison(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCrossVariantEquality(t *testing.T) {
	// All these should produce the same comparison string
	variants := []string{
		"St Ann's",
		"St Ann\u2019s",
		"St Ann\u2018s",
		"St Ann\u02BCs",
		"St Ann\u2032s",
		"St Ann`s",
		"St Ann&#039;s",
	}
	expected := ForComparison(variants[0])
	for _, v := range variants[1:] {
		got := ForComparison(v)
		if got != expected {
			t.Errorf("ForComparison(%q) = %q, want %q (same as first variant)", v, got, expected)
		}
	}
}
