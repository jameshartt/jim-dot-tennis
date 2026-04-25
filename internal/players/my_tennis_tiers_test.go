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

func TestTierForField(t *testing.T) {
	cases := map[string]int{
		"mixed_doubles_appetite":    1,
		"preferred_contact":         1,
		"preferred_days":            2,
		"transport":                 2,
		"handedness":                3,
		"signature_shot":            3,
		"partner_consistency":       4,
		"partners_clicks_with":      4,
		"partners_would_love_to_try": 4,
		"competitiveness":           4,
		"season_goal":               5,
		"weather_tolerance":         5,
		"years_playing":             6,
		"my_tennis_in_one_line":     6,
		"unknown_field":             0,
	}
	for field, want := range cases {
		if got := TierForField(field); got != want {
			t.Errorf("TierForField(%q) = %d; want %d", field, got, want)
		}
	}
}

// Partner picker fields are unique to tier 4 — guards against accidental
// duplication when the picker UI is touched.
func TestPartnerPickerOnlyOnTier4(t *testing.T) {
	for _, picker := range []string{"partners_clicks_with", "partners_would_love_to_try"} {
		if got := TierForField(picker); got != 4 {
			t.Errorf("partner picker field %q on tier %d; want tier 4", picker, got)
		}
	}
}

func TestParseTierForm(t *testing.T) {
	cases := map[string]int{
		"":  0,
		"0": 0,
		"1": 1,
		"6": 6,
		"7": 0,
		"-1": 0,
		"abc": 0,
	}
	for in, want := range cases {
		if got := parseTierForm(in); got != want {
			t.Errorf("parseTierForm(%q) = %d; want %d", in, got, want)
		}
	}
}
