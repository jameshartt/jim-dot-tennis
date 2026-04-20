// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"jim-dot-tennis/internal/models"
)

// PlanningHandler is the captain planning dashboard at /admin/league/planning.
// The dashboard is desktop/tablet-first; narrow portrait mobile gets a viewport
// nudge (dismissable) rather than a cramped UI.
type PlanningHandler struct {
	service     *Service
	templateDir string
}

// NewPlanningHandler constructs the planning dashboard handler.
func NewPlanningHandler(service *Service, templateDir string) *PlanningHandler {
	return &PlanningHandler{service: service, templateDir: templateDir}
}

// HandleCellToggle toggles a player's membership in a (fixture, team)
// team-selection pool from the planning matrix and returns the new cell
// fragment. Endpoint: POST /admin/league/planning/cell.
//
// Form params: fixture_id, player_id, team_id, is_home.
// Derby vs regular is inferred from the teams' clubs.
func (h *PlanningHandler) HandleCellToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	fxID, err := strconv.ParseUint(r.FormValue("fixture_id"), 10, 32)
	if err != nil || fxID == 0 {
		http.Error(w, "fixture_id required", http.StatusBadRequest)
		return
	}
	teamID, err := strconv.ParseUint(r.FormValue("team_id"), 10, 32)
	if err != nil || teamID == 0 {
		http.Error(w, "team_id required", http.StatusBadRequest)
		return
	}
	playerID := strings.TrimSpace(r.FormValue("player_id"))
	if playerID == "" {
		http.Error(w, "player_id required", http.StatusBadRequest)
		return
	}
	isHome := r.FormValue("is_home") == "true"

	ctx := r.Context()
	fixture, err := h.service.fixtureRepository.FindByID(ctx, uint(fxID))
	if err != nil || fixture == nil {
		http.NotFound(w, r)
		return
	}

	// Determine derby: both teams belong to the home club. In that case the
	// managing_team_id column discriminates the two selections.
	home, _ := h.service.teamRepository.FindByID(ctx, fixture.HomeTeamID)
	away, _ := h.service.teamRepository.FindByID(ctx, fixture.AwayTeamID)
	isDerby := home != nil && away != nil && home.ClubID == h.service.homeClubID && away.ClubID == h.service.homeClubID

	// Toggle: look up current membership by column key.
	current, err := h.service.fixtureRepository.FindSelectedPlayers(ctx, uint(fxID))
	if err != nil {
		logAndError(w, "load selection failed", err, http.StatusInternalServerError)
		return
	}
	wantKey := fmt.Sprintf("%d-%d", fixture.ID, teamID)
	alreadyIn := false
	for i := range current {
		if current[i].PlayerID != playerID {
			continue
		}
		if selectionColumnKey(fixture, &current[i]) == wantKey {
			alreadyIn = true
			break
		}
	}

	if alreadyIn {
		if isDerby {
			err = h.service.RemovePlayerFromFixtureByTeam(uint(fxID), playerID, uint(teamID))
		} else {
			err = h.service.RemovePlayerFromFixture(uint(fxID), playerID)
		}
	} else {
		if isDerby {
			err = h.service.AddPlayerToFixtureWithTeam(uint(fxID), playerID, isHome, uint(teamID))
		} else {
			err = h.service.AddPlayerToFixture(uint(fxID), playerID, isHome)
		}
	}
	if err != nil {
		logAndError(w, "toggle selection failed", err, http.StatusInternalServerError)
		return
	}

	// Re-resolve the cell.
	player, err := h.service.playerRepository.FindByID(ctx, playerID)
	if err != nil || player == nil {
		http.NotFound(w, r)
		return
	}
	cell := resolveCell(ctx, h.service, playerID, fixture)
	cell.ColumnKey = wantKey
	cell.InTeamSelection = !alreadyIn

	div, _ := h.service.divisionRepository.FindByID(ctx, fixture.DivisionID)
	divName := ""
	if div != nil {
		divName = div.Name
	}
	teamName, opponentName := "", ""
	switch {
	case uint(teamID) == fixture.HomeTeamID:
		if home != nil {
			teamName = home.Name
		}
		if away != nil {
			opponentName = away.Name
		}
	case uint(teamID) == fixture.AwayTeamID:
		if away != nil {
			teamName = away.Name
		}
		if home != nil {
			opponentName = home.Name
		}
	}
	col := &MatrixColumn{
		Fixture:           fixture,
		PerspectiveTeamID: uint(teamID),
		TeamName:          teamName,
		OpponentName:      opponentName,
		DivisionName:      divName,
		IsHome:            isHome,
		Key:               wantKey,
	}

	tmpl, err := parseTemplate(h.templateDir, "admin/partials/planning_cell.html")
	if err != nil {
		logAndError(w, "template parse failed", err, http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, "admin/partials/planning_cell.html", map[string]interface{}{
		"Cell":   cell,
		"Col":    col,
		"Player": player,
	}); err != nil {
		log.Printf("cell render failed: %v", err)
	}
}

// HandleDashboard serves GET /admin/league/planning and its HTMX partial at
// /admin/league/planning/matrix.
func (h *PlanningHandler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	user, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	// Route tail handling.
	path := r.URL.Path
	if strings.HasSuffix(path, "/matrix") {
		h.handleMatrixPartial(w, r, user)
		return
	}

	// First-visit gate: if the user isn't linked to a player yet AND hasn't
	// explicitly opted past the picker (?skip-link=1), show the picker. The
	// link is optional, so the skip escape is honoured forever after.
	if user.PlayerID == nil && r.URL.Query().Get("skip-link") != "1" {
		http.Redirect(w, r, "/admin/league/planning/link", http.StatusSeeOther)
		return
	}

	h.renderFullPage(w, r, user)
}

// handleMatrixPartial serves the matrix re-render for HTMX selection/filter changes.
func (h *PlanningHandler) handleMatrixPartial(w http.ResponseWriter, r *http.Request, user *models.User) {
	view, err := h.buildDashboardView(r.Context(), r, user)
	if err != nil {
		logAndError(w, "build dashboard view failed", err, http.StatusInternalServerError)
		return
	}

	tmpl, err := parseTemplate(h.templateDir, "admin/partials/planning_matrix.html")
	if err != nil {
		logAndError(w, "template parse failed", err, http.StatusInternalServerError)
		return
	}
	if err := renderTemplate(w, tmpl, view); err != nil {
		log.Printf("planning matrix render failed: %v", err)
	}
}

func (h *PlanningHandler) renderFullPage(w http.ResponseWriter, r *http.Request, user *models.User) {
	view, err := h.buildDashboardView(r.Context(), r, user)
	if err != nil {
		logAndError(w, "build dashboard view failed", err, http.StatusInternalServerError)
		return
	}

	tmpl, err := parseTemplate(h.templateDir, "admin/planning_dashboard.html")
	if err != nil {
		logAndError(w, "template parse failed", err, http.StatusInternalServerError)
		return
	}
	if err := renderTemplate(w, tmpl, view); err != nil {
		log.Printf("planning dashboard render failed: %v", err)
	}
}

// PlanningDashboardView is the full template data shape.
type PlanningDashboardView struct {
	User              *models.User
	LinkedPlayer      *models.Player
	ClubName          string
	Season            *models.Season
	PastSeasons       []models.Season
	ShowingPast       bool
	Week              *models.Week
	AllWeeks          []models.Week
	ResolvedScope     *ResolvedScope
	TeamOptions       []TeamOption
	SelectedTeamIDs   []uint
	IsAllTeams        bool
	Matrix            *PlanningMatrix
	QueryString       string
	Filters           MatrixFilters
	ActiveFilterCount int
	ClearFiltersURL   string
}

// TeamOption is a single team checkbox row (all active home-club teams in
// the season, alphabetised, with the user's own captain teams flagged).
type TeamOption struct {
	TeamID     uint
	TeamName   string
	DivisionID uint
	IsMine     bool
	IsSelected bool
}

func (h *PlanningHandler) buildDashboardView(ctx context.Context, r *http.Request, user *models.User) (*PlanningDashboardView, error) {
	showPast := r.URL.Query().Get("past") == "1"

	var (
		season *models.Season
		err    error
	)
	if sid := r.URL.Query().Get("season"); sid != "" && showPast {
		parsed, _ := strconv.ParseUint(sid, 10, 32)
		season, err = h.service.seasonRepository.FindByID(ctx, uint(parsed))
	}
	if season == nil {
		season, err = h.service.GetActiveSeason()
	}
	if err != nil || season == nil {
		return nil, fmt.Errorf("no season: %w", err)
	}

	allWeeks, err := h.service.GetWeeksBySeason(season.ID)
	if err != nil {
		return nil, fmt.Errorf("list weeks: %w", err)
	}
	sort.Slice(allWeeks, func(i, j int) bool { return allWeeks[i].WeekNumber < allWeeks[j].WeekNumber })

	var week *models.Week
	if wID := r.URL.Query().Get("week"); wID != "" {
		parsed, err := strconv.ParseUint(wID, 10, 32)
		if err == nil {
			for i := range allWeeks {
				if allWeeks[i].ID == uint(parsed) {
					week = &allWeeks[i]
					break
				}
			}
		}
	}
	if week == nil {
		w, err := h.service.GetCurrentOrNextWeek(season.ID)
		if err == nil && w != nil {
			week = w
		} else if len(allWeeks) > 0 {
			week = &allWeeks[0]
		}
	}

	selectedTeamIDs := parseTeamIDs(r.URL.Query()["team_id"])

	resolved, err := h.service.ResolveTeamSelection(ctx, selectedTeamIDs, season.ID)
	if err != nil {
		return nil, err
	}

	// Build the team-checkbox options. 'My teams' is the set of teams the
	// linked user captains this season — we flag them with a star so captains
	// can spot their own at a glance, but every team is tickable.
	var mySet map[uint]bool
	if user != nil && user.PlayerID != nil {
		captains, _ := h.service.playerRepository.FindCaptainRoles(ctx, *user.PlayerID, season.ID)
		mySet = make(map[uint]bool, len(captains))
		for _, c := range captains {
			mySet[c.TeamID] = true
		}
	}
	selectedSet := make(map[uint]bool, len(selectedTeamIDs))
	for _, id := range selectedTeamIDs {
		selectedSet[id] = true
	}
	teams, err := h.service.teamRepository.FindByClubAndSeason(ctx, h.service.homeClubID, season.ID)
	if err != nil {
		return nil, err
	}
	var teamOpts []TeamOption
	for _, t := range teams {
		if !t.Active {
			continue
		}
		teamOpts = append(teamOpts, TeamOption{
			TeamID:     t.ID,
			TeamName:   t.Name,
			DivisionID: t.DivisionID,
			IsMine:     mySet[t.ID],
			IsSelected: selectedSet[t.ID],
		})
	}
	sort.Slice(teamOpts, func(i, j int) bool { return teamOpts[i].TeamName < teamOpts[j].TeamName })

	var linked *models.Player
	if user.PlayerID != nil {
		linked, _ = h.service.playerRepository.FindByID(ctx, *user.PlayerID)
	}

	var pastSeasons []models.Season
	if showPast {
		all, err := h.service.GetAllSeasons()
		if err == nil {
			for _, s := range all {
				if !s.IsActive {
					pastSeasons = append(pastSeasons, s)
				}
			}
		}
	}

	filters := parseMatrixFilters(r)
	var matrix *PlanningMatrix
	if week != nil {
		matrix, err = h.service.ResolveAvailabilityMatrix(ctx, resolved, week, filters)
		if err != nil {
			log.Printf("matrix build warning: %v", err)
		}
	}

	// Clearing filters preserves team selection + week; 'All teams' (cleared
	// team selection) is the canonical empty state.
	clearURL := "/admin/league/planning?" + buildQueryString(selectedTeamIDs, week, showPast)

	return &PlanningDashboardView{
		User:              user,
		LinkedPlayer:      linked,
		ClubName:          homeClubNameFromContext(r),
		Season:            season,
		PastSeasons:       pastSeasons,
		ShowingPast:       showPast,
		Week:              week,
		AllWeeks:          allWeeks,
		ResolvedScope:     resolved,
		TeamOptions:       teamOpts,
		SelectedTeamIDs:   selectedTeamIDs,
		IsAllTeams:        len(selectedTeamIDs) == 0,
		Matrix:            matrix,
		QueryString:       buildQueryString(selectedTeamIDs, nil, showPast),
		Filters:           filters,
		ActiveFilterCount: filters.ActiveCount(),
		ClearFiltersURL:   clearURL,
	}, nil
}

// parseTeamIDs flattens repeated ?team_id=X and legacy comma-separated forms
// into a deduped slice of uints. Non-numeric entries are skipped.
func parseTeamIDs(raw []string) []uint {
	seen := make(map[uint]bool)
	var out []uint
	for _, v := range raw {
		for _, part := range strings.Split(v, ",") {
			p := strings.TrimSpace(part)
			if p == "" {
				continue
			}
			n, err := strconv.ParseUint(p, 10, 32)
			if err != nil {
				continue
			}
			id := uint(n)
			if seen[id] {
				continue
			}
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// MatrixFilters is the decoded preference filter state from the URL.
// WI-104 will extend this with additional chips; shell keeps the
// structure in place so the handler contract is stable.
type MatrixFilters struct {
	Handedness         []string
	CourtSide          []string
	MixedAppetite      []string
	SameGenderAppetite []string
	OpenToFillInOnly   bool
	MinCompetitiveness int
	MaxCompetitiveness int
}

// parseMatrixFilters reads filter state from the URL. Repeated checkbox params
// (?handedness=Right&handedness=Left) and legacy comma-separated values are
// both accepted so deep-links stay portable.
func parseMatrixFilters(r *http.Request) MatrixFilters {
	f := MatrixFilters{MinCompetitiveness: 1, MaxCompetitiveness: 5}
	q := r.URL.Query()
	f.Handedness = readMulti(q["handedness"])
	f.CourtSide = readMulti(q["court-side"])
	f.MixedAppetite = readMulti(q["mixed"])
	f.SameGenderAppetite = readMulti(q["same-gender"])
	f.OpenToFillInOnly = q.Get("open-only") == "1"
	if v := q.Get("comp-min"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 5 {
			f.MinCompetitiveness = n
		}
	}
	if v := q.Get("comp-max"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 5 {
			f.MaxCompetitiveness = n
		}
	}
	return f
}

// readMulti flattens repeated values AND comma-separated forms into a single
// slice, skipping blanks.
func readMulti(raw []string) []string {
	var out []string
	for _, v := range raw {
		for _, part := range strings.Split(v, ",") {
			p := strings.TrimSpace(part)
			if p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

// ActiveCount reports how many filters are non-default — drives the badge on
// the filters header so captains can see 'I've got stuff narrowed' at a glance.
func (f MatrixFilters) ActiveCount() int {
	n := 0
	if f.OpenToFillInOnly {
		n++
	}
	if len(f.Handedness) > 0 {
		n++
	}
	if len(f.CourtSide) > 0 {
		n++
	}
	if len(f.MixedAppetite) > 0 {
		n++
	}
	if len(f.SameGenderAppetite) > 0 {
		n++
	}
	if f.MinCompetitiveness > 1 || f.MaxCompetitiveness < 5 {
		n++
	}
	return n
}

// buildQueryString rebuilds the ?team_id=…&team_id=…&week=…&past=1 chain that
// other controls (week scrubber, past-season toggle) need to preserve.
func buildQueryString(selectedTeamIDs []uint, week *models.Week, past bool) string {
	var parts []string
	for _, id := range selectedTeamIDs {
		parts = append(parts, fmt.Sprintf("team_id=%d", id))
	}
	if week != nil {
		parts = append(parts, fmt.Sprintf("week=%d", week.ID))
	}
	if past {
		parts = append(parts, "past=1")
	}
	return strings.Join(parts, "&")
}
