// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package players

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"jim-dot-tennis/internal/models"
)

// MyTennisHandler renders the 'My Tennis' self-expression form and applies
// merge-semantic POST updates.
//
// PRIVACY CONTRACT (WI-097):
//
//   - GET never queries player_tennis_preferences or player_preferred_partners.
//     The form renders with all inputs blank regardless of stored state.
//   - POST applies merge semantics — absent / empty fields leave stored values
//     in place — and the confirmation page echoes only what was submitted in
//     this request. It must never read stored state.
//   - Partner lists are replaced only if the partnership section was saved
//     (signalled by an explicit hidden control); absent = no change.
//   - Stored answers are only readable via the admin-session surfaces
//     (WI-099 and the Sprint 017 planning dashboard).
//
// Future editors: preserve the blank GET contract. If you need read-back for
// players, add a magic-link flow over email (Sprint 007 infrastructure).
// Do not read stored state from this handler.
type MyTennisHandler struct {
	service     *Service
	templateDir string
}

// NewMyTennisHandler constructs a handler for the /my-profile/{token} surface.
func NewMyTennisHandler(service *Service, templateDir string) *MyTennisHandler {
	return &MyTennisHandler{service: service, templateDir: templateDir}
}

// HandleGet renders the blank My Tennis form. Does NOT read preferences.
func (h *MyTennisHandler) HandleGet(w http.ResponseWriter, r *http.Request, player *models.Player, authToken string) {
	roster, err := h.service.GetPartnerRoster(player.ID)
	if err != nil {
		log.Printf("my_tennis: partner roster lookup failed for %s: %v", player.ID, err)
		// A degraded picker is fine — the rest of the form is usable.
		roster = nil
	}

	data := map[string]interface{}{
		"Player":    player,
		"AuthToken": authToken,
		"Gender":    string(player.Gender),
		"Roster":    roster,
		// Intentionally NO stored preferences fields — the page is write-only.
	}

	tmpl, err := parseTemplate(h.templateDir, "players/my_tennis.html")
	if err != nil {
		log.Printf("my_tennis: template parse failed: %v", err)
		renderFallbackHTML(w, "My Tennis", "My Tennis",
			"Sorry — the My Tennis form could not be loaded.",
			"/my-availability/"+authToken)
		return
	}
	if err := renderTemplate(w, tmpl, data); err != nil {
		logAndError(w, err.Error(), err, http.StatusInternalServerError)
	}
}

// HandlePost parses a partial form submission, applies merge semantics to the
// stored preferences, and renders a confirmation page showing ONLY the fields
// submitted in this request.
func (h *MyTennisHandler) HandlePost(w http.ResponseWriter, r *http.Request, player *models.Player, authToken string) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad form submission", http.StatusBadRequest)
		return
	}

	section := strings.TrimSpace(r.FormValue("section"))

	prefs, submitted := buildPartialPreferences(r)
	partnerUpdates := map[models.PreferredPartnerKind]PartnerListUpdate{}
	if has(r, "__update_partners_clicks_with") {
		partnerUpdates[models.PreferredPartnerClicksWith] = PartnerListUpdate{
			Set: r.Form["partners_clicks_with"],
		}
		submitted = append(submitted, submittedField{
			Label: "Partners I click with",
			Value: describePartnerSet(r.Form["partners_clicks_with"], h.service, player.ID),
		})
	}
	if has(r, "__update_partners_would_love_to_try") {
		partnerUpdates[models.PreferredPartnerWouldLoveToTry] = PartnerListUpdate{
			Set: r.Form["partners_would_love_to_try"],
		}
		submitted = append(submitted, submittedField{
			Label: "Partners I'd love to try",
			Value: describePartnerSet(r.Form["partners_would_love_to_try"], h.service, player.ID),
		})
	}

	if err := h.service.UpdateMyTennisPreferences(player.ID, prefs, partnerUpdates); err != nil {
		log.Printf("my_tennis: update failed for %s: %v", player.ID, err)
		http.Error(w, "Sorry — we couldn't save your answers. Please try again.", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Player":          player,
		"AuthToken":       authToken,
		"Section":         section,
		"SubmittedFields": submitted,
		"FieldCount":      len(submitted),
	}

	tmpl, err := parseTemplate(h.templateDir, "players/my_tennis_confirmation.html")
	if err != nil {
		log.Printf("my_tennis: confirmation template parse failed: %v", err)
		renderFallbackHTML(w, "My Tennis", "Thanks!",
			"Your answers were saved.",
			"/my-availability/"+authToken)
		return
	}
	if err := renderTemplate(w, tmpl, data); err != nil {
		logAndError(w, err.Error(), err, http.StatusInternalServerError)
	}
}

// submittedField is echoed only on the confirmation page — sourced from the
// just-submitted POST, never from stored state.
type submittedField struct {
	Label string
	Value string
}

// has reports whether the named form key was submitted (any value, including "").
func has(r *http.Request, key string) bool {
	_, ok := r.Form[key]
	return ok
}

// buildPartialPreferences maps the submitted form values onto a partial
// PlayerTennisPreferences struct. Fields that were not submitted (or submitted
// empty) are left nil so merge-semantic upsert leaves stored values alone.
//
// Multi-select JSON columns (preferred_days, improvement_focus) follow an
// explicit-clear convention: an accompanying '__clear_<name>' hidden sets the
// column to '[]'; otherwise absent == no change.
func buildPartialPreferences(r *http.Request) (*models.PlayerTennisPreferences, []submittedField) {
	var submitted []submittedField
	p := &models.PlayerTennisPreferences{}
	any := false

	setStr := func(label, key string, dst **string) {
		v := strings.TrimSpace(r.FormValue(key))
		if v == "" {
			return
		}
		cp := v
		*dst = &cp
		any = true
		submitted = append(submitted, submittedField{Label: label, Value: v})
	}
	setInt := func(label, key string, dst **int) {
		v := strings.TrimSpace(r.FormValue(key))
		if v == "" {
			return
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			return
		}
		*dst = &n
		any = true
		submitted = append(submitted, submittedField{Label: label, Value: v})
	}
	setBool := func(label, key string, dst **bool) {
		// Presence-based: include a sibling hidden `<key>__present` so an
		// unchecked box can still drive the value. If `__present` is absent,
		// treat the field as 'not submitted' and leave stored value alone.
		if !has(r, key+"__present") {
			return
		}
		v := r.FormValue(key)
		b := v == "on" || v == "true" || v == "1"
		*dst = &b
		any = true
		if b {
			submitted = append(submitted, submittedField{Label: label, Value: "Yes"})
		} else {
			submitted = append(submitted, submittedField{Label: label, Value: "No"})
		}
	}
	setJSONArray := func(label, key string, dst **string) {
		values := r.Form[key]
		cleared := has(r, "__clear_"+key)
		if len(values) == 0 && !cleared {
			return
		}
		// Drop empties (browser sometimes sends a blank checkbox placeholder).
		cleaned := values[:0]
		for _, v := range values {
			if s := strings.TrimSpace(v); s != "" {
				cleaned = append(cleaned, s)
			}
		}
		j, err := json.Marshal(cleaned)
		if err != nil {
			return
		}
		s := string(j)
		*dst = &s
		any = true
		if len(cleaned) == 0 {
			submitted = append(submitted, submittedField{Label: label, Value: "(cleared)"})
		} else {
			submitted = append(submitted, submittedField{Label: label, Value: strings.Join(cleaned, ", ")})
		}
	}

	// Identity & Vibe
	setInt("Years playing", "years_playing", &p.YearsPlaying)
	setStr("How I got into tennis", "how_i_got_into_tennis", &p.HowIGotIntoTennis)
	setStr("Tennis hero / style", "tennis_hero_or_style", &p.TennisHeroOrStyle)
	setStr("Pre-match ritual", "pre_match_ritual", &p.PreMatchRitual)

	// Match Types
	setStr("Mixed doubles appetite", "mixed_doubles_appetite", &p.MixedDoublesAppetite)
	setStr("Same-gender doubles appetite", "same_gender_doubles_appetite", &p.SameGenderDoublesAppetite)
	setBool("Open to fill in", "open_to_fill_in", &p.OpenToFillIn)

	// Playing Style
	setStr("Handedness", "handedness", &p.Handedness)
	setStr("Backhand", "backhand", &p.Backhand)
	setStr("Serve style", "serve_style", &p.ServeStyle)
	setStr("Net comfort", "net_comfort", &p.NetComfort)
	setStr("Preferred court side", "preferred_court_side", &p.PreferredCourtSide)
	setStr("Signature shot", "signature_shot", &p.SignatureShot)
	setStr("Shot I'm working on", "shot_im_working_on", &p.ShotImWorkingOn)
	setStr("Favourite tactic", "favourite_tactic", &p.FavouriteTactic)

	// Partnership (scalars)
	setStr("Partner consistency", "partner_consistency", &p.PartnerConsistency)
	setStr("On-court vibe", "on_court_vibe", &p.OnCourtVibe)

	// Intensity & Goals
	setInt("Competitiveness (1–5)", "competitiveness", &p.Competitiveness)
	setStr("Pressure response", "pressure_response", &p.PressureResponse)
	setStr("Season goal", "season_goal", &p.SeasonGoal)
	setJSONArray("Improvement focus", "improvement_focus", &p.ImprovementFocus)

	// Logistics
	setJSONArray("Preferred match nights", "preferred_days", &p.PreferredDays)
	setStr("Home court matters", "home_court_matters", &p.HomeCourtMatters)

	// Health & Access
	setStr("What to know about my game", "what_to_know_about_my_game", &p.WhatToKnowAboutMyGame)
	setStr("Accessibility notes", "accessibility_notes", &p.AccessibilityNotes)
	setStr("Weather tolerance", "weather_tolerance", &p.WeatherTolerance)

	// Fun & Playful
	setStr("Tennis spirit animal", "tennis_spirit_animal", &p.TennisSpiritAnimal)
	setStr("Walkout song", "walkout_song", &p.WalkoutSong)
	setStr("Celebration style", "celebration_style", &p.CelebrationStyle)
	setStr("Post-match ritual", "post_match", &p.PostMatch)
	setStr("My tennis in one line", "my_tennis_in_one_line", &p.MyTennisInOneLine)

	// Communications
	setStr("Preferred contact", "preferred_contact", &p.PreferredContact)
	setStr("Best last-minute window", "best_window_for_last_minute", &p.BestWindowForLastMinute)
	setStr("Notes to captain", "notes_to_captain", &p.NotesToCaptain)

	if !any {
		return nil, submitted
	}
	return p, submitted
}

// describePartnerSet renders a selected partner set as initials-only names,
// so the confirmation page does not leak roster-mate full names through a
// screenshot / forwarded URL. Sourced from form data, not from stored state.
func describePartnerSet(ids []string, s *Service, self string) string {
	if len(ids) == 0 {
		return "(cleared)"
	}
	roster, err := s.GetPartnerRoster(self)
	if err != nil {
		return strconv.Itoa(len(ids)) + " selected"
	}
	byID := make(map[string]PartnerOption, len(roster))
	for _, p := range roster {
		byID[p.ID] = p
	}
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		if p, ok := byID[id]; ok {
			parts = append(parts, initialsFor(p.FirstName, p.LastName))
		}
	}
	if len(parts) == 0 {
		return strconv.Itoa(len(ids)) + " selected"
	}
	return strings.Join(parts, ", ")
}

// logAndError is defined elsewhere in the package (availability.go) — shared.
