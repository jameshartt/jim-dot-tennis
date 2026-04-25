// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package players

// Sprint 018 WI-109: canonical tier definitions for the My Tennis wizard.
//
// Single source of truth for:
//   - which form-field name lives in which tier (mirrored by the
//     migration-027 backfill SQL — keep them aligned)
//   - tier ordering, titles, and the warm intro copy each tier opens with
//
// The template renders the tier whose number the handler hands it; this
// file decides what that tier *contains*. Adding, removing, or moving a
// field is a single-place edit here plus its UI markup in the template.

// MaxTier is the highest tier number. wizard_progress_tier is clamped to
// this value; once a player reaches it they enter the 'all done' state.
const MaxTier = 6

// Tier captures the immutable description of one tier in the wizard.
// Fields lists the form-field names POSTed by that tier — it's used by
// unit tests to assert disjoint, exhaustive coverage of the underlying
// PlayerTennisPreferences model + partner-picker contract.
type Tier struct {
	ID     int
	Title  string
	Intro  string
	Fields []string
}

// HasField reports whether this tier is responsible for the named form field.
func (t Tier) HasField(name string) bool {
	for _, f := range t.Fields {
		if f == name {
			return true
		}
	}
	return false
}

// tiers is the ordered list. Order is the wizard's progression order;
// captains-actually-use-this-first → pure-colour-last.
var tiers = []Tier{
	{
		ID:    1,
		Title: "Team basics",
		Intro: "A few quick things that help us schedule fairly. Stop whenever you've shared enough.",
		Fields: []string{
			"mixed_doubles_appetite",
			"same_gender_doubles_appetite",
			"open_to_fill_in",
			"preferred_contact",
			"best_window_for_last_minute",
		},
	},
	{
		ID:    2,
		Title: "When & where",
		Intro: "The practical stuff — which nights work, how you get to courts, and whether home or away matters.",
		Fields: []string{
			"preferred_days",
			"preferred_times",
			"max_travel_miles",
			"transport",
			"home_court_matters",
		},
	},
	{
		ID:    3,
		Title: "How you play",
		Intro: "The texture of your game — the shots, sides, and situations that feel like home.",
		Fields: []string{
			"handedness",
			"backhand",
			"serve_style",
			"net_comfort",
			"preferred_court_side",
			"signature_shot",
			"shot_im_working_on",
			"favourite_tactic",
		},
	},
	{
		// Tier 4 is the only tier with a partner-picker; the picker is
		// represented in Fields by the two control names so the disjoint-
		// coverage test sees them, but the handler treats them as a
		// special list-replace via partnerUpdates.
		ID:    4,
		Title: "Partners & pressure",
		Intro: "Reflective, useful for matchups. Positive only — nothing about people you'd rather avoid.",
		Fields: []string{
			"partner_consistency",
			"on_court_vibe",
			"partners_clicks_with",
			"partners_would_love_to_try",
			"competitiveness",
			"pressure_response",
		},
	},
	{
		ID:    5,
		Title: "Goals & anything to know",
		Intro: "What you're chasing this season, and anything captains should know so you can play at your best.",
		Fields: []string{
			"season_goal",
			"improvement_focus",
			"what_to_know_about_my_game",
			"accessibility_notes",
			"weather_tolerance",
			"notes_to_captain",
		},
	},
	{
		ID:    6,
		Title: "The fun stuff",
		Intro: "Pure colour. Lots of people stop before here, and that's perfect.",
		Fields: []string{
			"years_playing",
			"how_i_got_into_tennis",
			"tennis_hero_or_style",
			"pre_match_ritual",
			"tennis_spirit_animal",
			"walkout_song",
			"celebration_style",
			"post_match",
			"my_tennis_in_one_line",
		},
	},
}

// Tiers returns the ordered tier list. Callers must treat it as read-only.
func Tiers() []Tier {
	out := make([]Tier, len(tiers))
	copy(out, tiers)
	return out
}

// TierByID returns the tier with the given 1-based ID, or zero-Tier if out of
// range. Callers should check the returned ID > 0 before using it.
func TierByID(id int) Tier {
	if id < 1 || id > len(tiers) {
		return Tier{}
	}
	return tiers[id-1]
}

// TierForField returns the 1-based tier ID that owns the given form-field
// name, or 0 if the name is not part of any tier.
func TierForField(name string) int {
	for _, t := range tiers {
		if t.HasField(name) {
			return t.ID
		}
	}
	return 0
}

// IsLastTier reports whether tier id is the terminal tier of the wizard.
func IsLastTier(id int) bool { return id == MaxTier }
