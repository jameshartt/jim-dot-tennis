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

// MyTennisHandler renders the 'My Tennis' wizard and applies merge-semantic
// POST updates one tier at a time.
//
// PRIVACY CONTRACT (WI-097, preserved by Sprint 018):
//
//   - GET never queries stored answers from player_tennis_preferences or
//     player_preferred_partners. Only the integer wizard_progress_tier crosses
//     the wire to the player; the form renders blank inputs regardless.
//   - POST applies merge semantics on the submitted tier's fields and bumps
//     wizard_progress_tier monotonically (never decrements).
//   - Re-edit (?edit=N) always opens a BLANK form — the privacy contract is
//     identical for fresh and returning users.
//   - Stored answers are only readable via the admin-session surfaces.
type MyTennisHandler struct {
	service     *Service
	templateDir string
}

// NewMyTennisHandler constructs a handler for the /my-profile/{token} surface.
func NewMyTennisHandler(service *Service, templateDir string) *MyTennisHandler {
	return &MyTennisHandler{service: service, templateDir: templateDir}
}

// HandleGet renders the wizard for the player's next-up tier (or the
// 'all done' state once they've reached MaxTier). Reads only the integer
// wizard_progress_tier — never any stored answer.
func (h *MyTennisHandler) HandleGet(w http.ResponseWriter, r *http.Request, player *models.Player, authToken string) {
	progress, err := h.service.GetWizardProgress(player.ID)
	if err != nil {
		log.Printf("my_tennis: progress lookup failed for %s: %v", player.ID, err)
		// Degrade to tier 1 — losing the resume position beats blocking the form.
		progress = 0
	}

	// Re-edit takes precedence: a returning player explicitly chose this tier.
	editParam := strings.TrimSpace(r.URL.Query().Get("edit"))
	if editParam != "" {
		if n, err := strconv.Atoi(editParam); err == nil && n >= 1 && n <= MaxTier {
			h.renderTier(w, player, authToken, n, progress, true)
			return
		}
	}

	if progress >= MaxTier {
		h.renderAllDone(w, player, authToken, progress)
		return
	}

	h.renderTier(w, player, authToken, progress+1, progress, false)
}

// HandlePost merge-saves the submitted tier's fields, bumps the wizard
// progress monotonically, and routes the response: 'finish' renders the
// confirmation page, 'continue' renders the next tier inline (or the
// 'all done' state when the user just completed MaxTier).
func (h *MyTennisHandler) HandlePost(w http.ResponseWriter, r *http.Request, player *models.Player, authToken string) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad form submission", http.StatusBadRequest)
		return
	}

	tier := parseTierForm(r.FormValue("tier"))
	intent := strings.TrimSpace(r.FormValue("intent"))

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

	if tier > 0 {
		if err := h.service.BumpWizardProgress(player.ID, tier); err != nil {
			log.Printf("my_tennis: progress bump failed for %s: %v", player.ID, err)
			// Best-effort — keep going to the success path.
		}
	}

	// Continue routing: render the next tier inline so any tier-level transient
	// state (banners, copy variations) survives without a 302 round-trip.
	if intent == "continue" && tier > 0 && tier < MaxTier {
		progress, err := h.service.GetWizardProgress(player.ID)
		if err != nil {
			progress = tier // best guess if the read fails
		}
		h.renderTier(w, player, authToken, tier+1, progress, false)
		return
	}
	if intent == "continue" && tier >= MaxTier {
		progress, err := h.service.GetWizardProgress(player.ID)
		if err != nil {
			progress = MaxTier
		}
		h.renderAllDone(w, player, authToken, progress)
		return
	}

	// Finish (or legacy section-mode POST): render confirmation.
	h.renderConfirmation(w, authToken, tier, submitted)
}

// renderTier renders my_tennis.html for a single tier. progress is the
// player's stored wizard_progress_tier (used only to drive the progress
// strip + banner copy — never to populate inputs). When editing is true,
// the page is opened from a re-edit link rather than the natural advance.
func (h *MyTennisHandler) renderTier(w http.ResponseWriter, player *models.Player, authToken string, tierID, progress int, editing bool) {
	tier := TierByID(tierID)
	if tier.ID == 0 {
		// Out of range — fall back to tier 1 rather than 500.
		tier = TierByID(1)
		tierID = 1
	}

	// Roster is only loaded when the player needs the partner picker — the
	// tier 4 contract. Other tiers leave Roster nil.
	var roster []PartnerOption
	if tier.HasField("partners_clicks_with") {
		r, err := h.service.GetPartnerRoster(player.ID)
		if err != nil {
			log.Printf("my_tennis: partner roster lookup failed for %s: %v", player.ID, err)
		} else {
			roster = r
		}
	}

	tierList := Tiers()

	data := map[string]interface{}{
		"Player":      player,
		"AuthToken":   authToken,
		"Gender":      string(player.Gender),
		"Roster":      roster,
		"Tier":        tier,
		"CurrentTier": tierID,
		"Progress":    progress,
		"MaxTier":     MaxTier,
		"IsLastTier":  IsLastTier(tierID),
		"IsEditing":   editing,
		"AllDone":     false,
		"Tiers":       tierList,
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

// renderAllDone shows the warm 'you've shared everything' state with
// re-edit links to every tier. No inputs, no answers — just affordances.
func (h *MyTennisHandler) renderAllDone(w http.ResponseWriter, player *models.Player, authToken string, progress int) {
	data := map[string]interface{}{
		"Player":      player,
		"AuthToken":   authToken,
		"Gender":      string(player.Gender),
		"Tier":        Tier{},
		"CurrentTier": 0,
		"Progress":    progress,
		"MaxTier":     MaxTier,
		"IsLastTier":  false,
		"IsEditing":   false,
		"AllDone":     true,
		"Tiers":       Tiers(),
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

// renderConfirmation renders the post-finish confirmation page, echoing
// only the just-submitted fields (never any stored state).
func (h *MyTennisHandler) renderConfirmation(w http.ResponseWriter, authToken string, tier int, submitted []submittedField) {
	tierLabel := ""
	if t := TierByID(tier); t.ID != 0 {
		tierLabel = t.Title
	}
	data := map[string]interface{}{
		"AuthToken":       authToken,
		"Tier":            tier,
		"TierLabel":       tierLabel,
		"SubmittedFields": submitted,
		"FieldCount":      len(submitted),
		"IsLastTier":      tier == MaxTier,
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

// parseTierForm extracts a 1..MaxTier integer from the submitted 'tier'
// field; returns 0 (legacy / no-op) for missing or out-of-range values.
func parseTierForm(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > MaxTier {
		return 0
	}
	return n
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
// Multi-select JSON columns (preferred_days, preferred_times, improvement_focus)
// follow an explicit-clear convention: an accompanying '__clear_<name>' hidden
// sets the column to '[]'; otherwise absent == no change.
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
	setStr("Transport", "transport", &p.Transport)
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
