// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"context"
	"encoding/json"
	"fmt"

	"jim-dot-tennis/internal/models"
)

// myTennisEnumLabels maps "<field>::<value>" to the positively-framed label
// the summary partial renders. Keeping the strings here (rather than in a
// template) makes the friendly copy reviewable in one place and keeps the
// template dumb.
var myTennisEnumLabels = map[string]string{
	// Match Types
	"mixed_doubles_appetite::love_it":          "Loves mixed doubles",
	"mixed_doubles_appetite::happy_to":         "Happy with mixed",
	"mixed_doubles_appetite::prefer_not":       "Prefers singles-gender doubles",
	"mixed_doubles_appetite::open_to_learn":    "Open to learning mixed",
	"same_gender_doubles_appetite::love_it":    "Loves same-gender doubles",
	"same_gender_doubles_appetite::happy_to":   "Happy with same-gender",
	"same_gender_doubles_appetite::prefer_not": "Prefers mixed",

	// Playing Style
	"handedness::right":            "Right-handed",
	"handedness::left":             "Left-handed",
	"handedness::ambidextrous":     "Both hands",
	"backhand::one_handed":         "One-handed backhand",
	"backhand::two_handed":         "Two-handed backhand",
	"backhand::slice_specialist":   "Slice specialist",
	"serve_style::cannon":          "Big serve",
	"serve_style::placement":       "Placement serve",
	"serve_style::spin":            "Heavy-spin serve",
	"serve_style::slice":           "Slice serve",
	"serve_style::kick":            "Kick serve",
	"serve_style::reliable":        "Reliable serve",
	"serve_style::developing":      "Serve still developing",
	"net_comfort::love":            "Lives at the net",
	"net_comfort::happy":           "Happy at the net",
	"net_comfort::working_on_it":   "Working on the net",
	"net_comfort::rather_not":      "Prefers the baseline",
	"preferred_court_side::deuce":  "Deuce side",
	"preferred_court_side::ad":     "Ad side",
	"preferred_court_side::either": "Either side",

	// Partnership
	"partner_consistency::same_always":  "Same partner always",
	"partner_consistency::few_regulars": "A few regulars",
	"partner_consistency::mix_it_up":    "Loves mixing it up",
	"on_court_vibe::quiet_focus":        "Quiet focus",
	"on_court_vibe::chatty_encouraging": "Chatty + encouraging",
	"on_court_vibe::fiery_competitive":  "Fiery competitor",
	"on_court_vibe::laid_back":          "Laid back",

	// Intensity
	"pressure_response::thrive":     "Thrives under pressure",
	"pressure_response::even_keel":  "Even keel",
	"pressure_response::needs_calm": "Plays best calm",

	// Logistics
	"home_court_matters::strong_preference": "Strongly prefers home",
	"home_court_matters::slight_preference": "Slight preference for home",
	"home_court_matters::no_preference":     "No preference",
	"home_court_matters::away_please":       "Enjoys away days",

	// Health & Access
	"weather_tolerance::all_weather":  "All-weather player",
	"weather_tolerance::no_cold":      "Not a fan of the cold",
	"weather_tolerance::no_heat":      "Not a fan of the heat",
	"weather_tolerance::fair_weather": "Fair-weather only",

	// Fun
	"celebration_style::fist_pump":             "Fist pump",
	"celebration_style::quiet_nod":             "Quiet nod",
	"celebration_style::silent_satisfaction":   "Silent satisfaction",
	"celebration_style::racquet_twirl":         "Racquet twirl",
	"celebration_style::high_five_partner":     "High-fives with partner",
	"celebration_style::whole_routine":         "Whole routine",
	"celebration_style::depends_on_the_moment": "Depends on the moment",
	"celebration_style::save_for_match":        "Saves it for match point",

	// Comms
	"preferred_contact::whatsapp": "WhatsApp",
	"preferred_contact::text":     "Text",
	"preferred_contact::email":    "Email",
	"preferred_contact::call":     "Call",
}

// PartnerSummary pairs a stored preferred-partner row with the partner's
// resolved first/last name (if still findable on the roster).
type PartnerSummary struct {
	PartnerID string
	FirstName string
	LastName  string
	Kind      models.PreferredPartnerKind
}

// PlayerTennisSummary is the composite view a captain / admin sees on read-
// side surfaces (admin player detail, Sprint 017 planning dashboard). It's
// assembled from the preferences row plus the partner join rows, with JSON
// array columns already parsed into string slices for easy templating.
type PlayerTennisSummary struct {
	Player      *models.Player
	Preferences *models.PlayerTennisPreferences // nil if the player has never filled the form
	HasAny      bool

	// Parsed JSON arrays (nil when the underlying column is nil).
	PreferredDays    []string
	PreferredTimes   []string
	ImprovementFocus []string

	// Partner lists, each already joined to resolve names.
	PartnersClicksWith     []PartnerSummary
	PartnersWouldLoveToTry []PartnerSummary
}

// GetPlayerTennisSummary assembles the read-side summary for a player.
//
// Intended ONLY for admin-session surfaces. The /my-profile/{token} handler
// must not call this method (see WI-097).
func (s *Service) GetPlayerTennisSummary(ctx context.Context, playerID string) (*PlayerTennisSummary, error) {
	player, err := s.playerRepository.FindByID(ctx, playerID)
	if err != nil {
		return nil, fmt.Errorf("player not found: %w", err)
	}

	prefs, err := s.tennisPreferenceRepository.FindByPlayerID(ctx, playerID)
	if err != nil {
		return nil, fmt.Errorf("load preferences: %w", err)
	}

	partners, err := s.tennisPreferenceRepository.ListPreferredPartners(ctx, playerID)
	if err != nil {
		return nil, fmt.Errorf("load partners: %w", err)
	}

	summary := &PlayerTennisSummary{Player: player, Preferences: prefs}
	if prefs != nil {
		summary.HasAny = preferencesHaveAnyValue(prefs)
		summary.PreferredDays = parseJSONArray(prefs.PreferredDays)
		summary.PreferredTimes = parseJSONArray(prefs.PreferredTimes)
		summary.ImprovementFocus = parseJSONArray(prefs.ImprovementFocus)
	}

	if len(partners) > 0 {
		// Resolve partner names once; dropped rows = partner no longer on roster.
		byID := map[string]*models.Player{}
		for _, p := range partners {
			if _, seen := byID[p.PartnerPlayerID]; seen {
				continue
			}
			if peer, err := s.playerRepository.FindByID(ctx, p.PartnerPlayerID); err == nil {
				byID[p.PartnerPlayerID] = peer
			} else {
				byID[p.PartnerPlayerID] = nil
			}
		}
		for _, p := range partners {
			peer := byID[p.PartnerPlayerID]
			entry := PartnerSummary{PartnerID: p.PartnerPlayerID, Kind: p.Kind}
			if peer != nil {
				entry.FirstName = peer.FirstName
				entry.LastName = peer.LastName
			}
			switch p.Kind {
			case models.PreferredPartnerClicksWith:
				summary.PartnersClicksWith = append(summary.PartnersClicksWith, entry)
			case models.PreferredPartnerWouldLoveToTry:
				summary.PartnersWouldLoveToTry = append(summary.PartnersWouldLoveToTry, entry)
			}
		}
	}

	if len(summary.PartnersClicksWith) > 0 || len(summary.PartnersWouldLoveToTry) > 0 {
		summary.HasAny = true
	}

	return summary, nil
}

func parseJSONArray(raw *string) []string {
	if raw == nil || *raw == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(*raw), &out); err != nil {
		return nil
	}
	return out
}

// preferencesHaveAnyValue returns true when at least one user-authored field
// carries a value (i.e. the player has interacted with the form at all).
func preferencesHaveAnyValue(p *models.PlayerTennisPreferences) bool {
	scalars := []*string{
		p.HowIGotIntoTennis, p.TennisHeroOrStyle, p.PreMatchRitual,
		p.MixedDoublesAppetite, p.SameGenderDoublesAppetite,
		p.Handedness, p.Backhand, p.ServeStyle, p.NetComfort, p.PreferredCourtSide,
		p.SignatureShot, p.ShotImWorkingOn, p.FavouriteTactic,
		p.PartnerConsistency, p.OnCourtVibe,
		p.PressureResponse, p.SeasonGoal, p.ImprovementFocus,
		p.PreferredDays, p.PreferredTimes, p.Transport, p.HomeCourtMatters,
		p.WhatToKnowAboutMyGame, p.AccessibilityNotes, p.WeatherTolerance,
		p.TennisSpiritAnimal, p.WalkoutSong, p.CelebrationStyle, p.PostMatch, p.MyTennisInOneLine,
		p.PreferredContact, p.BestWindowForLastMinute, p.NotesToCaptain,
	}
	for _, s := range scalars {
		if s != nil && *s != "" {
			return true
		}
	}
	ints := []*int{p.YearsPlaying, p.Competitiveness, p.MaxTravelMiles}
	for _, n := range ints {
		if n != nil {
			return true
		}
	}
	return p.OpenToFillIn != nil
}
