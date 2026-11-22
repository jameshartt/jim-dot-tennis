// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"time"

	"jim-dot-tennis/internal/accounts"
	"jim-dot-tennis/internal/config"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/tournamenthome"
	"jim-dot-tennis/internal/tournaments"
)

// tournamentSetupPath runs the club's tournament set-up (WI-145) again from
// the tournaments page or the settings page. Administrators only.
const tournamentSetupPath = "/admin/league/tournaments/setup"

// tournamentSetupTimeout bounds one set-up run started from a page, inside
// the server's 30-second write timeout.
const tournamentSetupTimeout = 25 * time.Second

// SetTournamentSetup gives the settings and tournaments pages the club's
// tournament set-up service (built in main.go from the shared courthive
// client). Without it they use one with no client: "not switched on".
func (h *Handler) SetTournamentSetup(s *tournaments.Service) {
	if s != nil {
		h.service.tournamentSetup = s
	}
}

// What a page is told after a set-up run or a switch (query parameter).
const (
	setupDone     = "done"     // the run finished
	setupFailed   = "failed"   // the run stopped at a step, or could not start
	setupExternal = "external" // no bundled tournament stack in this deployment
)

// setupOutcome reads a set-up outcome from a query parameter ("" for none).
func setupOutcome(v string) string {
	switch v {
	case setupDone, setupFailed, setupExternal:
		return v
	}
	return ""
}

// tournamentSetupView is the "Tournament set-up" panel's data.
type tournamentSetupView struct {
	Configured   bool   // the bundled tournament stack is part of this deployment
	SetUp        bool   // the club's provider id is stored
	Abbreviation string // fixed once set up; otherwise the one to offer
	Last         *tournaments.Result
	LastRanAt    string // the last run's time, for people
	Outcome      string // "done", "failed" or "external" just after a run or a switch; "" otherwise
	Problem      string // a failure that stopped a run starting

	ManagementURL string // where tournaments are created (tournament management)
	UsersPath     string // where Tournaments only accounts are made
}

// clubNameForTournaments is the club's name for its provider: the home club's
// name, else the display name.
func clubNameForTournaments(r *http.Request, cs *config.ClubSettings) string {
	if club := config.GetHomeClub(r.Context()); club != nil && club.Name != "" {
		return club.Name
	}
	return cs.DisplayName(nil)
}

// tournamentSetupData reads the set-up state for a page.
func (s *Service) tournamentSetupData(r *http.Request) tournamentSetupView {
	v := tournamentSetupView{ManagementURL: tournamenthome.ManagementPath, UsersPath: "/admin/league/users"}
	svc := s.tournamentSetup
	if svc == nil {
		return v
	}
	st, err := svc.Status(r.Context())
	if err != nil {
		log.Printf("Tournament set-up state unavailable: %v", err)
	}
	v.Configured, v.SetUp, v.Abbreviation, v.Last = st.Configured, st.SetUp, st.Abbreviation, st.Last
	if !v.SetUp {
		v.Abbreviation = svc.DefaultAbbreviation(r.Context(), clubNameForTournaments(r, s.clubSettings))
	}
	if v.Last != nil {
		v.LastRanAt = v.Last.RanAt.Local().Format("2 Jan 2006 at 15:04")
	}
	return v
}

// runTournamentSetup runs set-up for the signed-in administrator and returns
// "done" or "failed", and for a run that could not start, why.
func (s *Service) runTournamentSetup(r *http.Request, user *models.User, abbreviation string) (outcome, problem string) {
	ctx, cancel := context.WithTimeout(r.Context(), tournamentSetupTimeout)
	defer cancel()
	res, err := s.tournamentSetup.Provision(ctx, accounts.UserActor(user.ID, user.Username), tournaments.Request{
		Abbreviation: abbreviation,
		Name:         clubNameForTournaments(r, s.clubSettings),
	})
	var stepErr *tournaments.StepError
	switch {
	case err == nil && res.OK:
		return setupDone, ""
	case errors.As(err, &stepErr):
		// Stored with the result; the page shows it.
		log.Printf("Tournament set-up stopped at %q: %v", stepErr.Step, stepErr.Err)
		return setupFailed, ""
	case errors.Is(err, tournaments.ErrNotConfigured):
		return setupExternal, ""
	default:
		return setupFailed, sentence(err.Error()) + "."
	}
}

// HandleSetup serves POST /admin/league/tournaments/setup: run the club's
// tournament set-up again (or for the first time), then show the outcome on
// the page the button was on.
func (h *TournamentsHandler) HandleSetup(w http.ResponseWriter, r *http.Request) {
	user, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}
	outcome, problem := h.service.runTournamentSetup(r, user, r.FormValue("abbreviation"))
	q := url.Values{"setup": {outcome}}
	if problem != "" {
		q.Set("setup_problem", problem)
	}
	target := "/admin/league/tournaments?" + q.Encode()
	if r.FormValue("return") == "settings" {
		q = url.Values{"tournament_setup": {outcome}}
		if problem != "" {
			q.Set("setup_problem", problem)
		}
		target = settingsPath + "?" + q.Encode()
	}
	http.Redirect(w, r, target+"#tournament-setup", http.StatusSeeOther)
}
