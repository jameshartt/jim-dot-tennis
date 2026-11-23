// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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

// tournamentAddSetPath adds a further set of tournaments (a provider) beside
// the club's own, and tournamentRenameSetPath changes the name people see
// for one (WI-154). Administrators only.
const (
	tournamentAddSetPath    = "/admin/league/tournaments/setup/add"
	tournamentRenameSetPath = "/admin/league/tournaments/setup/rename"
)

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

	// NotApplied counts people whose latest change of tournament access the
	// tournament server has not taken yet (it was unreachable); the app keeps
	// trying every minute.
	NotApplied int

	// Sets are the sets of tournaments the app manages (the club's own
	// first); Found are those on the tournament server it has not taken on.
	Sets  []tournaments.ManagedProvider
	Found []tournaments.FoundProvider

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
	if v.Configured {
		if waiting, err := svc.PendingAccounts(r.Context()); err != nil {
			log.Printf("Tournament accounts waiting to be updated: %v", err)
		} else {
			v.NotApplied = len(waiting)
		}
	}
	if v.Configured && v.SetUp {
		if v.Sets, err = svc.ManagedProviders(r.Context()); err != nil {
			log.Printf("Tournament set-up: the sets of tournaments could not be read: %v", err)
		}
		// Asking the tournament server must not hold the page up for long.
		ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
		defer cancel()
		if v.Found, err = svc.NotTakenOn(ctx); err != nil {
			log.Printf("Tournament set-up: the tournament server's other sets of tournaments could not be listed: %v", err)
		}
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

// HandleAddSet serves POST /admin/league/tournaments/setup/add: add a
// further set of tournaments beside the club's own, or take on one that is
// already on the tournament server with that code.
func (h *TournamentsHandler) HandleAddSet(w http.ResponseWriter, r *http.Request) {
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
	ctx, cancel := context.WithTimeout(r.Context(), tournamentSetupTimeout)
	defer cancel()
	res, err := h.service.tournamentSetup.AddProvider(ctx, accounts.UserActor(user.ID, user.Username), r.FormValue("code"), r.FormValue("name"))
	q := url.Values{}
	var stepErr *tournaments.StepError
	switch {
	case err == nil && res.OK:
		q.Set("setup", setupDone)
	case errors.As(err, &stepErr):
		log.Printf("Adding a set of tournaments stopped at %q: %v", stepErr.Step, stepErr.Err)
		q.Set("setup", setupFailed)
	case errors.Is(err, tournaments.ErrNotConfigured):
		q.Set("setup", setupExternal)
	case errors.Is(err, tournaments.ErrNotSetUp):
		q.Set("setup", setupFailed)
		q.Set("setup_problem", "Set the club up first, then add further sets of tournaments.")
	default:
		q.Set("setup", setupFailed)
		q.Set("setup_problem", sentence(err.Error())+".")
	}
	http.Redirect(w, r, "/admin/league/tournaments?"+q.Encode()+"#tournament-setup", http.StatusSeeOther)
}

// HandleRenameSet serves POST /admin/league/tournaments/setup/rename: change
// the name people see for a set of tournaments. Its code stays.
func (h *TournamentsHandler) HandleRenameSet(w http.ResponseWriter, r *http.Request) {
	if _, err := getUserFromContext(r); err != nil {
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
	id, err := strconv.ParseUint(r.FormValue("id"), 10, 32)
	name := strings.TrimSpace(r.FormValue("name"))
	if err != nil || name == "" || len(name) > 80 {
		http.Redirect(w, r, "/admin/league/tournaments?error="+url.QueryEscape("Give the tournaments a name of up to 80 characters.")+"#tournament-setup", http.StatusSeeOther)
		return
	}
	provider, err := h.service.GetTournamentProviderByID(uint(id))
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	provider.Name = name
	if err := h.service.UpdateTournamentProvider(provider); err != nil {
		logAndError(w, "Failed to rename", err, http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/league/tournaments?success="+url.QueryEscape("Name changed")+"#tournament-setup", http.StatusSeeOther)
}
