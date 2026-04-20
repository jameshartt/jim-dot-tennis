// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package repository

// Privacy invariant: nothing under internal/players/* may import this package.
// Captain notes are an admin-only surface — they store the 'no-nos' and other
// sensitive planning context that players should never read back about
// themselves. Sprint 017 WI-107 adds an E2E regression asserting seeded notes
// never appear on any /my-availability/{token} or /my-profile/{token} page.

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
)

// CaptainNoteRepository is the data access layer for captain_player_notes.
type CaptainNoteRepository interface {
	ListByPlayer(ctx context.Context, playerID string) ([]models.CaptainPlayerNote, error)
	ListByPlayers(ctx context.Context, playerIDs []string) (map[string][]models.CaptainPlayerNote, error)
	FindByID(ctx context.Context, id uint) (*models.CaptainPlayerNote, error)
	Create(ctx context.Context, note *models.CaptainPlayerNote) error
	Update(ctx context.Context, note *models.CaptainPlayerNote) error
	Delete(ctx context.Context, id uint) error
	CountsByPlayer(ctx context.Context, playerIDs []string) (map[string]int, error)
}

type captainNoteRepository struct {
	db *database.DB
}

// NewCaptainNoteRepository creates a new captain notes repository.
func NewCaptainNoteRepository(db *database.DB) CaptainNoteRepository {
	return &captainNoteRepository{db: db}
}

func (r *captainNoteRepository) ListByPlayer(ctx context.Context, playerID string) ([]models.CaptainPlayerNote, error) {
	var rows []models.CaptainPlayerNote
	err := r.db.SelectContext(ctx, &rows, `
		SELECT id, player_id, author_user_id, kind, body, created_at, updated_at
		FROM captain_player_notes
		WHERE player_id = ?
		ORDER BY updated_at DESC, id DESC
	`, playerID)
	return rows, err
}

func (r *captainNoteRepository) ListByPlayers(ctx context.Context, playerIDs []string) (map[string][]models.CaptainPlayerNote, error) {
	result := map[string][]models.CaptainPlayerNote{}
	if len(playerIDs) == 0 {
		return result, nil
	}
	query, args, err := sqlx.In(`
		SELECT id, player_id, author_user_id, kind, body, created_at, updated_at
		FROM captain_player_notes
		WHERE player_id IN (?)
		ORDER BY updated_at DESC, id DESC
	`, playerIDs)
	if err != nil {
		return nil, err
	}
	var rows []models.CaptainPlayerNote
	if err := r.db.SelectContext(ctx, &rows, r.db.Rebind(query), args...); err != nil {
		return nil, err
	}
	for _, n := range rows {
		result[n.PlayerID] = append(result[n.PlayerID], n)
	}
	return result, nil
}

func (r *captainNoteRepository) FindByID(ctx context.Context, id uint) (*models.CaptainPlayerNote, error) {
	var note models.CaptainPlayerNote
	err := r.db.GetContext(ctx, &note, `
		SELECT id, player_id, author_user_id, kind, body, created_at, updated_at
		FROM captain_player_notes
		WHERE id = ?
	`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &note, nil
}

func (r *captainNoteRepository) Create(ctx context.Context, note *models.CaptainPlayerNote) error {
	now := time.Now()
	note.CreatedAt = now
	note.UpdatedAt = now
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO captain_player_notes (player_id, author_user_id, kind, body, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, note.PlayerID, note.AuthorUserID, string(note.Kind), note.Body, note.CreatedAt, note.UpdatedAt)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	note.ID = uint(id)
	return nil
}

func (r *captainNoteRepository) Update(ctx context.Context, note *models.CaptainPlayerNote) error {
	note.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx, `
		UPDATE captain_player_notes
		SET kind = ?, body = ?, updated_at = ?
		WHERE id = ?
	`, string(note.Kind), note.Body, note.UpdatedAt, note.ID)
	return err
}

func (r *captainNoteRepository) Delete(ctx context.Context, id uint) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM captain_player_notes WHERE id = ?`, id)
	return err
}

func (r *captainNoteRepository) CountsByPlayer(ctx context.Context, playerIDs []string) (map[string]int, error) {
	result := map[string]int{}
	if len(playerIDs) == 0 {
		return result, nil
	}
	query, args, err := sqlx.In(`
		SELECT player_id, COUNT(*) AS n
		FROM captain_player_notes
		WHERE player_id IN (?)
		GROUP BY player_id
	`, playerIDs)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryxContext(ctx, r.db.Rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var pid string
		var n int
		if err := rows.Scan(&pid, &n); err != nil {
			return nil, err
		}
		result[pid] = n
	}
	return result, rows.Err()
}
