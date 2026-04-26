// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package players

import "testing"

// Sprint 018 WI-109 acceptance test: every form field referenced by the
// wizard belongs to exactly one tier. Catches accidental duplicates and
// reordering mistakes when fields move between tiers.
func TestTiersFieldsAreDisjoint(t *testing.T) {
	owner := map[string]int{}
	for _, tier := range Tiers() {
		for _, field := range tier.Fields {
			if prev, seen := owner[field]; seen {
				t.Fatalf("field %q claimed by tiers %d and %d", field, prev, tier.ID)
			}
			owner[field] = tier.ID
		}
	}
}

func TestTiersAreOrderedAndContiguous(t *testing.T) {
	got := Tiers()
	if len(got) != MaxTier {
		t.Fatalf("expected %d tiers, got %d", MaxTier, len(got))
	}
	for i, tier := range got {
		if tier.ID != i+1 {
			t.Errorf("tier index %d has ID %d (want %d)", i, tier.ID, i+1)
		}
	}
}

func TestParseTierForm(t *testing.T) {
	cases := map[string]int{
		"":    0,
		"0":   0,
		"1":   1,
		"6":   6,
		"7":   0,
		"-1":  0,
		"abc": 0,
	}
	for in, want := range cases {
		if got := parseTierForm(in); got != want {
			t.Errorf("parseTierForm(%q) = %d; want %d", in, got, want)
		}
	}
}
