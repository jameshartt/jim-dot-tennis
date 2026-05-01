// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"jim-dot-tennis/internal/config"
	"jim-dot-tennis/internal/models"
)

// ResultsHandler handles match result entry
type ResultsHandler struct {
	service     *Service
	templateDir string
}

// NewResultsHandler creates a new results handler
func NewResultsHandler(service *Service, templateDir string) *ResultsHandler {
	return &ResultsHandler{
		service:     service,
		templateDir: templateDir,
	}
}

// ResultsPageData holds data for the results entry template
type ResultsPageData struct {
	FixtureDetail  *FixtureDetail
	Matchups       []MatchupWithPlayers
	Errors         map[string]string
	IsEdit         bool
	Success        bool
	IsDerby        bool
	ManagingTeamID uint // populated when IsDerby; the slate the form is editing
}

// MatchupScoreEntry represents submitted scores for a single matchup
type MatchupScoreEntry struct {
	MatchupID  uint
	HomeSet1   *int
	AwaySet1   *int
	HomeSet2   *int
	AwaySet2   *int
	HomeSet3   *int
	AwaySet3   *int
	Conceded   bool
	ConcededBy models.ConcededBy
	Retired    bool
	RetiredBy  models.RetiredBy
}

// HandleResults handles both GET and POST for result entry
func (h *ResultsHandler) HandleResults(w http.ResponseWriter, r *http.Request) {
	// Extract fixture ID from URL path
	path := strings.TrimSuffix(r.URL.Path, "/results")
	fixtureID, err := parseIDFromPath(path, "/admin/league/fixtures/")
	if err != nil {
		logAndError(w, "Invalid fixture ID", err, http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleResultsGet(w, r, fixtureID)
	case http.MethodPost:
		h.handleResultsPost(w, r, fixtureID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// resolveResultsManagingTeam mirrors the derby-handling pattern used by
// /admin/league/fixtures/{id}: derbies are filtered to a single managing team's
// slate so only four matchup cards render. The query string ?managingTeam=N
// chooses which slate; if absent on a derby, we default to the fixture's home
// team. For non-derby fixtures it returns 0 (no filtering needed).
func (h *ResultsHandler) resolveResultsManagingTeam(r *http.Request, fixture *FixtureDetail) (uint, bool) {
	homeClubID := config.GetHomeClubID(r.Context())
	homeIsHomeClub := fixture.HomeTeam != nil && fixture.HomeTeam.ClubID == homeClubID
	awayIsHomeClub := fixture.AwayTeam != nil && fixture.AwayTeam.ClubID == homeClubID
	isDerby := homeIsHomeClub && awayIsHomeClub
	if !isDerby {
		return 0, false
	}
	if param := r.URL.Query().Get("managingTeam"); param != "" {
		if v, err := strconv.ParseUint(param, 10, 32); err == nil {
			return uint(v), true
		}
	}
	return fixture.HomeTeamID, true
}

// handleResultsGet renders the results entry form
func (h *ResultsHandler) handleResultsGet(w http.ResponseWriter, r *http.Request, fixtureID uint) {
	fixtureDetail, err := h.service.GetFixtureDetail(fixtureID)
	if err != nil {
		logAndError(w, "Fixture not found", err, http.StatusNotFound)
		return
	}

	managingTeamID, isDerby := h.resolveResultsManagingTeam(r, fixtureDetail)

	// Ensure all four matchup types exist for the active slate
	h.ensureAllMatchups(fixtureID, fixtureDetail, managingTeamID, isDerby)

	// Re-fetch to get the created matchups, scoped to the active managing team
	// for derbies so only four cards render.
	if isDerby {
		fixtureDetail, err = h.service.GetFixtureDetailWithTeamContext(fixtureID, managingTeamID)
	} else {
		fixtureDetail, err = h.service.GetFixtureDetail(fixtureID)
	}
	if err != nil {
		logAndError(w, "Failed to reload fixture", err, http.StatusInternalServerError)
		return
	}

	isEdit := fixtureDetail.Status == models.Completed
	for _, m := range fixtureDetail.Matchups {
		if m.Matchup.Status == models.Finished || m.Matchup.Status == models.Defaulted {
			isEdit = true
			break
		}
	}

	// In a derby each slate only stores its own players. Merge the mirror slate's
	// players so the form shows both sides of the "vs".
	if isDerby {
		if merged, mergeErr := h.service.MergeDerbyOpponentPlayers(fixtureID, managingTeamID, fixtureDetail.Matchups); mergeErr == nil {
			fixtureDetail.Matchups = merged
		} else {
			log.Printf("Warning: failed to merge derby opponent players for fixture %d: %v", fixtureID, mergeErr)
		}
	}

	data := ResultsPageData{
		FixtureDetail:  fixtureDetail,
		Matchups:       fixtureDetail.Matchups,
		Errors:         make(map[string]string),
		IsEdit:         isEdit,
		IsDerby:        isDerby,
		ManagingTeamID: managingTeamID,
	}

	tmpl, err := parseTemplate(h.templateDir, "admin/match_result_entry.html")
	if err != nil {
		log.Printf("Error parsing results template: %v", err)
		renderFallbackHTML(w, "Enter Results", "Enter Results",
			"Results entry page - template error", fmt.Sprintf("/admin/league/fixtures/%d", fixtureID))
		return
	}

	if err := renderTemplate(w, tmpl, data); err != nil {
		logAndError(w, err.Error(), err, http.StatusInternalServerError)
	}
}

// handleResultsPost processes submitted results
func (h *ResultsHandler) handleResultsPost(w http.ResponseWriter, r *http.Request, fixtureID uint) {
	if err := r.ParseForm(); err != nil {
		logAndError(w, "Failed to parse form", err, http.StatusBadRequest)
		return
	}

	fixtureDetail, err := h.service.GetFixtureDetail(fixtureID)
	if err != nil {
		logAndError(w, "Fixture not found", err, http.StatusNotFound)
		return
	}

	managingTeamID, isDerby := h.resolveResultsManagingTeam(r, fixtureDetail)
	// Scope the form's matchups to the active slate for derbies so we only
	// process the four IDs that were actually rendered.
	if isDerby {
		if scoped, scopedErr := h.service.GetFixtureDetailWithTeamContext(fixtureID, managingTeamID); scopedErr == nil {
			fixtureDetail = scoped
		}
	}

	// Parse and validate all matchup scores
	errors := make(map[string]string)
	var entries []MatchupScoreEntry

	for _, mwp := range fixtureDetail.Matchups {
		matchup := mwp.Matchup
		prefix := fmt.Sprintf("matchup_%d", matchup.ID)

		// Check if conceded
		conceded := r.FormValue(prefix+"_conceded") == "true"
		if conceded {
			concededByStr := r.FormValue(prefix + "_conceded_by")
			var concededBy models.ConcededBy
			switch concededByStr {
			case "Home":
				concededBy = models.ConcededHome
			case "Away":
				concededBy = models.ConcededAway
			default:
				errors[prefix] = "Conceded matchup must specify Home or Away"
				continue
			}
			entries = append(entries, MatchupScoreEntry{
				MatchupID:  matchup.ID,
				Conceded:   true,
				ConcededBy: concededBy,
			})
			continue
		}

		// Check if retired (play started but stopped mid-match — partial set scores
		// are accepted because they may not reach a completed set)
		retired := r.FormValue(prefix+"_retired") == "true"

		// Parse set scores
		entry := MatchupScoreEntry{MatchupID: matchup.ID}

		if retired {
			retiredByStr := r.FormValue(prefix + "_retired_by")
			var retiredBy models.RetiredBy
			switch retiredByStr {
			case "Home":
				retiredBy = models.RetiredHome
			case "Away":
				retiredBy = models.RetiredAway
			default:
				errors[prefix] = "Retired matchup must specify which side retired"
				continue
			}
			entry.Retired = true
			entry.RetiredBy = retiredBy

			// Accept whatever partial scores were entered without tennis-rule validation,
			// then save the entry and move to the next matchup.
			entry.HomeSet1, entry.AwaySet1 = parseRetiredSetScore(r, prefix+"_home_set1", prefix+"_away_set1")
			entry.HomeSet2, entry.AwaySet2 = parseRetiredSetScore(r, prefix+"_home_set2", prefix+"_away_set2")
			entry.HomeSet3, entry.AwaySet3 = parseRetiredSetScore(r, prefix+"_home_set3", prefix+"_away_set3")

			entries = append(entries, entry)
			continue
		}

		homeSet1, awaySet1, err := parseSetScore(r, prefix+"_home_set1", prefix+"_away_set1")
		if err != nil {
			errors[prefix+"_set1"] = err.Error()
		} else {
			entry.HomeSet1 = homeSet1
			entry.AwaySet1 = awaySet1
		}

		homeSet2, awaySet2, err := parseSetScore(r, prefix+"_home_set2", prefix+"_away_set2")
		if err != nil {
			errors[prefix+"_set2"] = err.Error()
		} else {
			entry.HomeSet2 = homeSet2
			entry.AwaySet2 = awaySet2
		}

		// Set 3 is optional
		homeSet3Str := r.FormValue(prefix + "_home_set3")
		awaySet3Str := r.FormValue(prefix + "_away_set3")
		if homeSet3Str != "" || awaySet3Str != "" {
			homeSet3, awaySet3, err := parseSetScore(r, prefix+"_home_set3", prefix+"_away_set3")
			if err != nil {
				errors[prefix+"_set3"] = err.Error()
			} else {
				entry.HomeSet3 = homeSet3
				entry.AwaySet3 = awaySet3
			}
		}

		// Validate individual set scores
		if entry.HomeSet1 != nil && entry.AwaySet1 != nil {
			if err := ValidateTennisScore(*entry.HomeSet1, *entry.AwaySet1); err != nil {
				errors[prefix+"_set1"] = "Set 1: " + err.Error()
			}
		} else if entry.HomeSet1 == nil && entry.AwaySet1 == nil {
			// Both empty — skip validation (not entered yet)
		} else {
			errors[prefix+"_set1"] = "Set 1: both home and away scores required"
		}

		if entry.HomeSet2 != nil && entry.AwaySet2 != nil {
			if err := ValidateTennisScore(*entry.HomeSet2, *entry.AwaySet2); err != nil {
				errors[prefix+"_set2"] = "Set 2: " + err.Error()
			}
		} else if entry.HomeSet2 == nil && entry.AwaySet2 == nil {
			// Both empty
		} else {
			errors[prefix+"_set2"] = "Set 2: both home and away scores required"
		}

		if entry.HomeSet3 != nil && entry.AwaySet3 != nil {
			if err := ValidateTennisScore(*entry.HomeSet3, *entry.AwaySet3); err != nil {
				errors[prefix+"_set3"] = "Set 3: " + err.Error()
			}
			// Validate that set 3 is only played if sets are split
			if entry.HomeSet1 != nil && entry.AwaySet1 != nil && entry.HomeSet2 != nil && entry.AwaySet2 != nil {
				homeWins := 0
				awayWins := 0
				if *entry.HomeSet1 > *entry.AwaySet1 {
					homeWins++
				} else {
					awayWins++
				}
				if *entry.HomeSet2 > *entry.AwaySet2 {
					homeWins++
				} else {
					awayWins++
				}
				if homeWins != 1 || awayWins != 1 {
					errors[prefix+"_set3"] = "Set 3: only played when sets are split 1-1"
				}
			}
		}

		// Require at least sets 1 and 2
		if entry.HomeSet1 == nil && entry.AwaySet1 == nil && entry.HomeSet2 == nil && entry.AwaySet2 == nil {
			errors[prefix] = "Please enter scores or mark as conceded"
		}

		entries = append(entries, entry)
	}

	// If validation errors, re-render with errors
	if len(errors) > 0 {
		data := ResultsPageData{
			FixtureDetail:  fixtureDetail,
			Matchups:       fixtureDetail.Matchups,
			Errors:         errors,
			IsEdit:         fixtureDetail.Status == models.Completed,
			IsDerby:        isDerby,
			ManagingTeamID: managingTeamID,
		}

		tmpl, err := parseTemplate(h.templateDir, "admin/match_result_entry.html")
		if err != nil {
			logAndError(w, "Template error", err, http.StatusInternalServerError)
			return
		}

		if err := renderTemplate(w, tmpl, data); err != nil {
			logAndError(w, err.Error(), err, http.StatusInternalServerError)
		}
		return
	}

	// Save results
	if err := h.service.SaveMatchupResults(fixtureID, entries); err != nil {
		logAndError(w, "Failed to save results", err, http.StatusInternalServerError)
		return
	}

	// For derbies, mirror the score/concession/retirement fields onto the other
	// team's slate so both captain views stay in sync. Players are NOT mirrored
	// — each slate keeps its own roster (the whole reason dual slates exist).
	if isDerby {
		if err := h.service.MirrorDerbyResults(fixtureID, managingTeamID, entries); err != nil {
			log.Printf("Warning: failed to mirror derby results for fixture %d: %v", fixtureID, err)
		}
	}

	// Mark fixture as completed
	if err := h.service.CompleteFixtureWithResults(fixtureID); err != nil {
		logAndError(w, "Failed to complete fixture", err, http.StatusInternalServerError)
		return
	}

	// Redirect back to fixture detail, preserving the active managing team for derbies
	redirectURL := fmt.Sprintf("/admin/league/fixtures/%d", fixtureID)
	if isDerby {
		redirectURL = fmt.Sprintf("%s?managingTeam=%d", redirectURL, managingTeamID)
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// ensureAllMatchups creates any missing standard matchup types for a fixture.
// For derbies, the active managingTeamID scopes the check so we don't accidentally
// look at the other team's slate when deciding what to create.
func (h *ResultsHandler) ensureAllMatchups(fixtureID uint, detail *FixtureDetail, managingTeamID uint, isDerby bool) {
	standardTypes := []models.MatchupType{models.FirstMixed, models.SecondMixed, models.Mens, models.Womens}

	existingTypes := make(map[models.MatchupType]bool)
	for _, m := range detail.Matchups {
		if isDerby && m.Matchup.ManagingTeamID != nil && *m.Matchup.ManagingTeamID != managingTeamID {
			continue
		}
		existingTypes[m.Matchup.Type] = true
	}

	for _, mt := range standardTypes {
		if existingTypes[mt] {
			continue
		}
		var err error
		if isDerby {
			_, err = h.service.GetOrCreateMatchupWithTeam(fixtureID, mt, managingTeamID)
		} else {
			_, err = h.service.GetOrCreateMatchup(fixtureID, mt)
		}
		if err != nil {
			log.Printf("Warning: failed to create matchup type %s for fixture %d: %v", mt, fixtureID, err)
		}
	}
}

// parseSetScore parses a home/away set score pair from form values
func parseSetScore(r *http.Request, homeKey, awayKey string) (*int, *int, error) {
	homeStr := strings.TrimSpace(r.FormValue(homeKey))
	awayStr := strings.TrimSpace(r.FormValue(awayKey))

	if homeStr == "" && awayStr == "" {
		return nil, nil, nil
	}

	if homeStr == "" || awayStr == "" {
		return nil, nil, fmt.Errorf("both scores are required")
	}

	home, err := strconv.Atoi(homeStr)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid home score")
	}

	away, err := strconv.Atoi(awayStr)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid away score")
	}

	return &home, &away, nil
}

// parseRetiredSetScore parses a home/away set score pair without enforcing tennis
// completion rules. Used for retired matches where set scores may be partial.
// Returns nil pointers when both inputs are blank.
func parseRetiredSetScore(r *http.Request, homeKey, awayKey string) (*int, *int) {
	homeStr := strings.TrimSpace(r.FormValue(homeKey))
	awayStr := strings.TrimSpace(r.FormValue(awayKey))

	if homeStr == "" && awayStr == "" {
		return nil, nil
	}

	var home, away int
	if homeStr != "" {
		if v, err := strconv.Atoi(homeStr); err == nil {
			home = v
		}
	}
	if awayStr != "" {
		if v, err := strconv.Atoi(awayStr); err == nil {
			away = v
		}
	}
	return &home, &away
}

// ValidateTennisScore validates a single set score
func ValidateTennisScore(homeGames, awayGames int) error {
	if homeGames < 0 || awayGames < 0 {
		return fmt.Errorf("scores cannot be negative")
	}
	if homeGames > 7 || awayGames > 7 {
		return fmt.Errorf("maximum 7 games in a set")
	}

	// At least one side must have 6 or more
	if homeGames < 6 && awayGames < 6 {
		return fmt.Errorf("at least one side must reach 6 games")
	}

	// If either side has 7, the other must have 5 or 6
	if homeGames == 7 {
		if awayGames != 5 && awayGames != 6 {
			return fmt.Errorf("7 games only valid with opponent at 5 or 6")
		}
	}
	if awayGames == 7 {
		if homeGames != 5 && homeGames != 6 {
			return fmt.Errorf("7 games only valid with opponent at 5 or 6")
		}
	}

	// If both have 6, invalid (should be 7-6 tiebreak)
	if homeGames == 6 && awayGames == 6 {
		return fmt.Errorf("6-6 not valid, tiebreak should result in 7-6")
	}

	// Standard wins: 6-0 through 6-4
	if homeGames == 6 && awayGames <= 4 {
		return nil
	}
	if awayGames == 6 && homeGames <= 4 {
		return nil
	}

	// 7-5 and 7-6 are valid
	if (homeGames == 7 && (awayGames == 5 || awayGames == 6)) ||
		(awayGames == 7 && (homeGames == 5 || homeGames == 6)) {
		return nil
	}

	return fmt.Errorf("invalid set score: %d-%d", homeGames, awayGames)
}
