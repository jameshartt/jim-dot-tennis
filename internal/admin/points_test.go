// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"testing"

	"jim-dot-tennis/internal/models"
)

func ip(n int) *int { return &n }

func rb(v models.RetiredBy) *models.RetiredBy { return &v }

// newPointsMap seeds the map processMatchupPoints awards into (it only credits
// player IDs already present).
func newPointsMap(ids ...string) map[string]*PlayerPoints {
	m := make(map[string]*PlayerPoints, len(ids))
	for _, id := range ids {
		m[id] = &PlayerPoints{ID: id}
	}
	return m
}

func matchupWith(m models.Matchup) CompletedMatchupWithPlayers {
	return CompletedMatchupWithPlayers{
		Matchup:     m,
		HomePlayers: []models.Player{{ID: "h1"}, {ID: "h2"}},
		AwayPlayers: []models.Player{{ID: "a1"}, {ID: "a2"}},
	}
}

func TestProcessMatchupPoints(t *testing.T) {
	h := &PointsHandler{}

	type want struct {
		matches        int
		winPts, setPts float64
		totalPts       float64
	}
	assertPlayer := func(t *testing.T, pp *PlayerPoints, w want) {
		t.Helper()
		if pp.MatchesPlayed != w.matches || pp.WinPoints != w.winPts || pp.SetPoints != w.setPts || pp.TotalPoints != w.totalPts {
			t.Errorf("got {matches:%d win:%v set:%v total:%v}, want {matches:%d win:%v set:%v total:%v}",
				pp.MatchesPlayed, pp.WinPoints, pp.SetPoints, pp.TotalPoints,
				w.matches, w.winPts, w.setPts, w.totalPts)
		}
	}

	t.Run("straight-sets home win", func(t *testing.T) {
		pm := newPointsMap("h1", "h2", "a1", "a2")
		h.processMatchupPoints(matchupWith(models.Matchup{
			HomeScore: 2, AwayScore: 0,
			HomeSet1: ip(6), AwaySet1: ip(4),
			HomeSet2: ip(6), AwaySet2: ip(3),
		}), pm)
		assertPlayer(t, pm["h1"], want{1, 1.0, 2.0, 3.0})
		assertPlayer(t, pm["a1"], want{1, 0.0, 0.0, 0.0})
	})

	t.Run("three-set away win", func(t *testing.T) {
		pm := newPointsMap("h1", "h2", "a1", "a2")
		h.processMatchupPoints(matchupWith(models.Matchup{
			HomeScore: 0, AwayScore: 2,
			HomeSet1: ip(6), AwaySet1: ip(4), // home
			HomeSet2: ip(4), AwaySet2: ip(6), // away
			HomeSet3: ip(5), AwaySet3: ip(7), // away
		}), pm)
		assertPlayer(t, pm["a1"], want{1, 1.0, 2.0, 3.0})
		assertPlayer(t, pm["h1"], want{1, 0.0, 1.0, 1.0})
	})

	t.Run("halved match (1-1 marker)", func(t *testing.T) {
		pm := newPointsMap("h1", "h2", "a1", "a2")
		h.processMatchupPoints(matchupWith(models.Matchup{
			HomeScore: 1, AwayScore: 1,
			HomeSet1: ip(6), AwaySet1: ip(4), // home
			HomeSet2: ip(4), AwaySet2: ip(6), // away
		}), pm)
		assertPlayer(t, pm["h1"], want{1, 0.5, 1.0, 1.5})
		assertPlayer(t, pm["a1"], want{1, 0.5, 1.0, 1.5})
	})

	t.Run("retirement by home denies home, full win to away", func(t *testing.T) {
		pm := newPointsMap("h1", "h2", "a1", "a2")
		h.processMatchupPoints(matchupWith(models.Matchup{
			HomeScore: 0, AwayScore: 2,
			// Partial/contradictory set scores must be overridden by retirement.
			HomeSet1: ip(6), AwaySet1: ip(0),
			RetiredBy: rb(models.RetiredHome),
		}), pm)
		assertPlayer(t, pm["a1"], want{1, 1.0, 2.0, 3.0})
		assertPlayer(t, pm["h1"], want{1, 0.0, 0.0, 0.0})
	})

	t.Run("even sets without halved marker is a draw", func(t *testing.T) {
		pm := newPointsMap("h1", "h2", "a1", "a2")
		h.processMatchupPoints(matchupWith(models.Matchup{
			HomeScore: 0, AwayScore: 0,
			HomeSet1: ip(6), AwaySet1: ip(4), // home
			HomeSet2: ip(4), AwaySet2: ip(6), // away
		}), pm)
		assertPlayer(t, pm["h1"], want{1, 0.5, 1.0, 1.5})
		assertPlayer(t, pm["a1"], want{1, 0.5, 1.0, 1.5})
	})

	t.Run("accumulates across two matchups", func(t *testing.T) {
		pm := newPointsMap("h1", "h2", "a1", "a2")
		win := matchupWith(models.Matchup{HomeScore: 2, AwayScore: 0, HomeSet1: ip(6), AwaySet1: ip(4), HomeSet2: ip(6), AwaySet2: ip(2)})
		h.processMatchupPoints(win, pm)
		h.processMatchupPoints(win, pm)
		assertPlayer(t, pm["h1"], want{2, 2.0, 4.0, 6.0})
	})
}
