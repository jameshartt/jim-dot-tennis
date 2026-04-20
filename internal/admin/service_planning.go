// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"context"
	"fmt"
	"sort"

	"jim-dot-tennis/internal/models"
)

// ResolvedScope is the materialised team/division list the matrix queries
// against. An empty team_id selection resolves to every active home-club team
// ('All Teams' is the default) — selection is additive from there.
type ResolvedScope struct {
	TeamIDs     []uint
	DivisionIDs []uint
}

// SetPlayerIDOnUser writes users.player_id for a user (nil to unlink).
func (s *Service) SetPlayerIDOnUser(ctx context.Context, userID int64, playerID *string) error {
	if playerID == nil {
		_, err := s.db.ExecContext(ctx, `UPDATE users SET player_id = NULL WHERE id = ?`, userID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `UPDATE users SET player_id = ? WHERE id = ?`, *playerID, userID)
	return err
}

// ListHomeClubPlayersForLink returns active home-club players, alphabetised
// by last name for the 'I am…' picker.
func (s *Service) ListHomeClubPlayersForLink(ctx context.Context) ([]models.Player, error) {
	players, err := s.playerRepository.FindByClub(ctx, s.homeClubID)
	if err != nil {
		return nil, err
	}
	active := players[:0]
	for _, p := range players {
		if p.IsActive {
			active = append(active, p)
		}
	}
	sort.Slice(active, func(i, j int) bool {
		if active[i].LastName == active[j].LastName {
			return active[i].FirstName < active[j].FirstName
		}
		return active[i].LastName < active[j].LastName
	})
	return active, nil
}

// ResolveTeamSelection turns the ?team_id=X&team_id=Y querystring into the
// concrete list of teams the matrix considers. Empty input ⇒ every active
// home-club team; non-empty input ⇒ only the subset that actually belongs
// to the home club (IDs from other clubs are silently dropped).
func (s *Service) ResolveTeamSelection(ctx context.Context, selected []uint, seasonID uint) (*ResolvedScope, error) {
	allTeams, err := s.teamRepository.FindByClubAndSeason(ctx, s.homeClubID, seasonID)
	if err != nil {
		return nil, fmt.Errorf("load home-club season teams: %w", err)
	}

	activeByID := make(map[uint]models.Team, len(allTeams))
	for _, t := range allTeams {
		if t.Active {
			activeByID[t.ID] = t
		}
	}

	resolved := &ResolvedScope{}
	divSet := make(map[uint]bool)

	if len(selected) == 0 {
		for _, t := range activeByID {
			resolved.TeamIDs = append(resolved.TeamIDs, t.ID)
			divSet[t.DivisionID] = true
		}
	} else {
		seen := make(map[uint]bool, len(selected))
		for _, id := range selected {
			if seen[id] {
				continue
			}
			t, ok := activeByID[id]
			if !ok {
				continue
			}
			seen[id] = true
			resolved.TeamIDs = append(resolved.TeamIDs, id)
			divSet[t.DivisionID] = true
		}
		if len(resolved.TeamIDs) == 0 {
			for _, t := range activeByID {
				resolved.TeamIDs = append(resolved.TeamIDs, t.ID)
				divSet[t.DivisionID] = true
			}
		}
	}

	for id := range divSet {
		resolved.DivisionIDs = append(resolved.DivisionIDs, id)
	}
	sort.Slice(resolved.TeamIDs, func(i, j int) bool { return resolved.TeamIDs[i] < resolved.TeamIDs[j] })
	sort.Slice(resolved.DivisionIDs, func(i, j int) bool { return resolved.DivisionIDs[i] < resolved.DivisionIDs[j] })
	return resolved, nil
}
