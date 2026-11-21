// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"jim-dot-tennis/internal/bhplta"
	"jim-dot-tennis/internal/services"
)

// "Start a new season from BHPLTA" against the saved BHPLTA pages, on an
// instance that already has seasons and clubs: it adds the season, reuses the
// league and matching clubs, leaves the active season alone, and refuses to
// import the same season twice.
func TestStartSeasonFromBHPLTA(t *testing.T) {
	svc, db := newSeasonTestService(t)
	root := filepath.Join(filepath.Dir(findMigrationsPathAdmin(t)), "tests", "e2e", "fixtures", "bhplta")
	srv := httptest.NewServer(http.FileServer(http.Dir(root)))
	t.Cleanup(srv.Close)
	t.Setenv(services.BHPLTABaseURLEnv, srv.URL)

	sum, err := svc.StartSeasonFromBHPLTA(context.Background())
	if err != nil {
		t.Fatalf("StartSeasonFromBHPLTA: %v", err)
	}
	if sum.SeasonName != "2024 Season" || sum.SeasonActivated || sum.LeagueCreated || sum.Fixtures != 228 || sum.Teams != 41 {
		t.Fatalf("summary %+v", sum)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM clubs WHERE name LIKE 'St Ann%'`); n != 1 {
		t.Errorf("%d St Ann's clubs, want the existing one only", n)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM teams WHERE club_id = 1 AND season_id = ?`, sum.SeasonID); n != 4 {
		t.Errorf("St Ann's has %d teams in the new season, want 4", n)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM seasons WHERE is_active = 1 AND id = 1`); n != 1 {
		t.Error("the active season changed")
	}

	before := count(t, db, `SELECT COUNT(*) FROM teams`)
	_, err = svc.StartSeasonFromBHPLTA(context.Background())
	var exists *bhplta.SeasonExistsError
	if !errors.As(err, &exists) || exists.SeasonID != sum.SeasonID {
		t.Fatalf("second import: %v", err)
	}
	if after := count(t, db, `SELECT COUNT(*) FROM teams`); after != before {
		t.Fatalf("refused import changed teams: %d -> %d", before, after)
	}
}
