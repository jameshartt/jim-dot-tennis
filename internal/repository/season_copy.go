// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package repository

import (
	"context"
	"fmt"
	"time"

	"jim-dot-tennis/internal/models"
)

// CreateWithWeeks inserts a season and its weeks in one transaction, so a
// failed week insert never leaves a half-built season behind. On success
// season.ID and each week's ID/SeasonID are populated.
func (r *seasonRepository) CreateWithWeeks(ctx context.Context, season *models.Season, weeks []models.Week) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now()
	season.CreatedAt = now
	season.UpdatedAt = now
	result, err := tx.NamedExecContext(ctx, `
		INSERT INTO seasons (name, year, start_date, end_date, is_active, created_at, updated_at)
		VALUES (:name, :year, :start_date, :end_date, :is_active, :created_at, :updated_at)
	`, season)
	if err != nil {
		return fmt.Errorf("failed to create season: %w", err)
	}
	seasonID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to read season id: %w", err)
	}

	for i := range weeks {
		weeks[i].SeasonID = uint(seasonID)
		weeks[i].CreatedAt = now
		weeks[i].UpdatedAt = now
		res, err := tx.NamedExecContext(ctx, `
			INSERT INTO weeks (week_number, season_id, start_date, end_date, name, is_active, created_at, updated_at)
			VALUES (:week_number, :season_id, :start_date, :end_date, :name, :is_active, :created_at, :updated_at)
		`, &weeks[i])
		if err != nil {
			return fmt.Errorf("failed to create week %d: %w", weeks[i].WeekNumber, err)
		}
		if id, err := res.LastInsertId(); err == nil {
			weeks[i].ID = uint(id)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	season.ID = uint(seasonID)
	return nil
}

// CopyStructure copies divisions and/or teams from one season into another in
// a single transaction. Copied teams bring their active players and captains;
// inactive players are skipped. When divisions are not copied, teams are mapped
// onto the target season's existing divisions by name, and teams whose
// division has no match are skipped. Any failure rolls the whole copy back.
func (r *seasonRepository) CopyStructure(ctx context.Context, fromSeasonID, toSeasonID uint, copyDivisions, copyTeams bool) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now()
	var oldDivisions []models.Division
	if err := tx.SelectContext(ctx, &oldDivisions, `
		SELECT id, name, level, play_day, league_id, season_id, max_teams_per_club, created_at, updated_at
		FROM divisions WHERE season_id = ? ORDER BY level ASC, id ASC
	`, fromSeasonID); err != nil {
		return fmt.Errorf("failed to find divisions from previous season: %w", err)
	}

	// old division ID -> new division ID
	divisionIDMap := make(map[uint]uint)

	if copyDivisions {
		for _, oldDiv := range oldDivisions {
			res, err := tx.ExecContext(ctx, `
				INSERT INTO divisions (name, level, play_day, league_id, season_id, max_teams_per_club, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			`, oldDiv.Name, oldDiv.Level, oldDiv.PlayDay, oldDiv.LeagueID, toSeasonID, oldDiv.MaxTeamsPerClub, now, now)
			if err != nil {
				return fmt.Errorf("failed to create division %s: %w", oldDiv.Name, err)
			}
			id, err := res.LastInsertId()
			if err != nil {
				return fmt.Errorf("failed to read division id: %w", err)
			}
			divisionIDMap[oldDiv.ID] = uint(id)
		}
	} else if copyTeams {
		var newDivisions []models.Division
		if err := tx.SelectContext(ctx, &newDivisions, `
			SELECT id, name, level, play_day, league_id, season_id, max_teams_per_club, created_at, updated_at
			FROM divisions WHERE season_id = ?
		`, toSeasonID); err != nil {
			return fmt.Errorf("failed to find divisions in target season: %w", err)
		}
		newDivsByName := make(map[string]uint)
		for _, div := range newDivisions {
			newDivsByName[div.Name] = div.ID
		}
		for _, oldDiv := range oldDivisions {
			if newDivID, ok := newDivsByName[oldDiv.Name]; ok {
				divisionIDMap[oldDiv.ID] = newDivID
			}
		}
	}

	if copyTeams {
		var oldTeams []models.Team
		if err := tx.SelectContext(ctx, &oldTeams, `
			SELECT id, name, club_id, division_id, season_id, active, created_at, updated_at
			FROM teams WHERE season_id = ? ORDER BY id ASC
		`, fromSeasonID); err != nil {
			return fmt.Errorf("failed to find teams from previous season: %w", err)
		}

		for _, oldTeam := range oldTeams {
			newDivisionID, ok := divisionIDMap[oldTeam.DivisionID]
			if !ok {
				continue // division has no match in the new season
			}

			res, err := tx.ExecContext(ctx, `
				INSERT INTO teams (name, club_id, division_id, season_id, active, created_at, updated_at)
				VALUES (?, ?, ?, ?, TRUE, ?, ?)
			`, oldTeam.Name, oldTeam.ClubID, newDivisionID, toSeasonID, now, now)
			if err != nil {
				return fmt.Errorf("failed to create team %s: %w", oldTeam.Name, err)
			}
			newTeamID, err := res.LastInsertId()
			if err != nil {
				return fmt.Errorf("failed to read team id: %w", err)
			}

			if _, err := tx.ExecContext(ctx, `
				INSERT INTO player_teams (player_id, team_id, season_id, is_active, created_at, updated_at)
				SELECT pt.player_id, ?, ?, TRUE, ?, ?
				FROM player_teams pt JOIN players p ON p.id = pt.player_id
				WHERE pt.team_id = ? AND pt.season_id = ? AND p.is_active = TRUE
				ORDER BY pt.created_at ASC, pt.id ASC
			`, newTeamID, toSeasonID, now, now, oldTeam.ID, fromSeasonID); err != nil {
				return fmt.Errorf("failed to copy players for team %s: %w", oldTeam.Name, err)
			}

			if _, err := tx.ExecContext(ctx, `
				INSERT INTO captains (player_id, team_id, role, season_id, is_active, created_at, updated_at)
				SELECT c.player_id, ?, c.role, ?, TRUE, ?, ?
				FROM captains c JOIN players p ON p.id = c.player_id
				WHERE c.team_id = ? AND c.season_id = ? AND p.is_active = TRUE
				ORDER BY c.role ASC, c.created_at ASC
			`, newTeamID, toSeasonID, now, now, oldTeam.ID, fromSeasonID); err != nil {
				return fmt.Errorf("failed to copy captains for team %s: %w", oldTeam.Name, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}
