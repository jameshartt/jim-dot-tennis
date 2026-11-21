// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"jim-dot-tennis/internal/bhplta"
)

// seasonFromBHPLTATimeout keeps the import inside the server's write
// timeout, so a slow BHPLTA site gives a readable message rather than a
// dropped connection. Nothing is written when it runs out.
const seasonFromBHPLTATimeout = 25 * time.Second

// StartSeasonFromBHPLTA reads the season currently on the BHPLTA website and
// imports it: the league if missing, any clubs this instance does not have
// (existing clubs are matched, never duplicated), and a new season with its
// weeks, divisions, teams and fixtures. It is all-or-nothing. A season that
// already has divisions is refused with a *bhplta.SeasonExistsError.
func (s *Service) StartSeasonFromBHPLTA(ctx context.Context) (*bhplta.Summary, error) {
	return bhplta.NewBootstrapper(s.db).Run(ctx, bhplta.Options{}, nil, nil)
}

// HandleStartFromBHPLTA shows (GET) and runs (POST) "Start a new season from
// BHPLTA", for instances with no previous season to copy.
func (h *SeasonSetupHandler) HandleStartFromBHPLTA(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}
	data := map[string]interface{}{"User": user}

	status := http.StatusOK
	if r.Method == http.MethodPost {
		ctx, cancel := context.WithTimeout(r.Context(), seasonFromBHPLTATimeout)
		defer cancel()
		sum, err := h.service.StartSeasonFromBHPLTA(ctx)
		if err != nil {
			log.Printf("Start a new season from BHPLTA: %v", err)
			data["Error"] = bhplta.Describe(err)
			var exists *bhplta.SeasonExistsError
			if errors.As(err, &exists) {
				data["ExistingSeasonID"] = exists.SeasonID
				status = http.StatusConflict
			} else {
				status = http.StatusBadGateway
			}
		} else {
			log.Printf("Start a new season from BHPLTA: %s: %d divisions, %d teams, %d fixtures, %d clubs created",
				sum.SeasonName, sum.Divisions, sum.Teams, sum.Fixtures, sum.ClubsCreated)
			data["Summary"] = sum
		}
	}

	tmpl, err := parseTemplate(h.templateDir, "admin/season_bhplta.html")
	if err != nil {
		logAndError(w, "Failed to parse template", err, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(status)
	if err := renderTemplate(w, tmpl, data); err != nil {
		log.Printf("Failed to render season_bhplta: %v", err)
	}
}
