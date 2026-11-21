// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package setup

import (
	"errors"
	"log"
	"net/http"
)

// demo handles POST /setup/demo: load the demo data from the club step.
func (h *Handler) demo(w http.ResponseWriter, r *http.Request) {
	if !allowMethods(w, r, http.MethodPost) {
		return
	}
	ctx := r.Context()
	if h.svc.State().Status(ctx) != StatusNeedsClub {
		http.NotFound(w, r)
		return
	}
	if _, ok := h.signedInAdmin(r); !ok {
		http.Error(w, "Sign in as an administrator to load demo data.", http.StatusForbidden)
		return
	}
	data := pageData{Step: 3, Status: StatusNeedsClub.String(), Clubs: h.svc.Clubs(), DemoOffered: true}
	err := h.svc.LoadDemo(ctx)
	switch {
	case err == nil:
		log.Printf("setup: demo data loaded")
		http.Redirect(w, r, dashboardPath, http.StatusSeeOther)
	case errors.Is(err, ErrUnavailable):
		http.NotFound(w, r)
	case errors.Is(err, ErrDemoHasData):
		data.Error = "This instance already has data, so demo data cannot be loaded. Nothing was changed."
		h.render(w, http.StatusConflict, "club", data)
	case errors.Is(err, ErrDemoEnvOverride):
		data.Error = "This instance's home club is fixed by its configuration (HOME_CLUB_ID or HOME_CLUB_NAME), so demo data cannot be loaded."
		h.render(w, http.StatusConflict, "club", data)
	case errors.Is(err, ErrClubJobRunning):
		http.Redirect(w, r, ClubPath+"?"+workingQuery+"=1", http.StatusSeeOther)
	default:
		log.Printf("setup: load demo data: %v", err)
		data.Error = "Something went wrong loading the demo data. Nothing was changed; please try again."
		h.render(w, http.StatusInternalServerError, "club", data)
	}
}

// demoClear is /setup/demo/clear: GET asks an administrator to confirm, POST
// clears the demo data and returns to the club step. It is not found unless
// demo data is loaded.
func (h *Handler) demoClear(w http.ResponseWriter, r *http.Request) {
	if !allowMethods(w, r, http.MethodGet, http.MethodPost) {
		return
	}
	if !h.svc.DemoActive() {
		http.NotFound(w, r)
		return
	}
	if _, ok := h.signedInAdmin(r); !ok {
		if r.Method == http.MethodPost {
			http.Error(w, "Sign in as an administrator to clear demo data.", http.StatusForbidden)
			return
		}
		http.Redirect(w, r, loginPath, http.StatusSeeOther)
		return
	}
	data := pageData{Status: h.svc.State().Status(r.Context()).String()}
	if r.Method != http.MethodPost {
		h.render(w, http.StatusOK, "demo_clear", data)
		return
	}
	switch err := h.svc.ClearDemo(r.Context()); {
	case err == nil:
		log.Printf("setup: demo data cleared")
		http.Redirect(w, r, ClubPath, http.StatusSeeOther)
	case errors.Is(err, ErrNotDemo):
		http.NotFound(w, r)
	default:
		log.Printf("setup: clear demo data: %v", err)
		data.Error = "Something went wrong clearing the demo data. Nothing was changed; please try again."
		h.render(w, http.StatusInternalServerError, "demo_clear", data)
	}
}
