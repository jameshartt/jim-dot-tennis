// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package repository

import (
	"context"
	"time"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
)

// TournamentRepository defines the interface for tournament data access
type TournamentRepository interface {
	FindAll(ctx context.Context) ([]models.Tournament, error)
	FindByID(ctx context.Context, id uint) (*models.Tournament, error)
	FindByProviderID(ctx context.Context, providerID uint) ([]models.Tournament, error)
	FindVisible(ctx context.Context) ([]models.Tournament, error)
	FindByCourthiveTournamentID(ctx context.Context, courthiveID string) (*models.Tournament, error)
	Create(ctx context.Context, tournament *models.Tournament) error
	Update(ctx context.Context, tournament *models.Tournament) error
	Delete(ctx context.Context, id uint) error
}

type tournamentRepository struct {
	db *database.DB
}

func NewTournamentRepository(db *database.DB) TournamentRepository {
	return &tournamentRepository{db: db}
}

func (r *tournamentRepository) FindAll(ctx context.Context) ([]models.Tournament, error) {
	var tournaments []models.Tournament
	err := r.db.SelectContext(ctx, &tournaments, `
		SELECT t.id, t.name, t.description, t.courthive_tournament_id, t.provider_id,
		       t.start_date, t.end_date, t.is_visible, t.display_order, t.created_at, t.updated_at,
		       tp.name AS provider_name
		FROM tournaments t
		JOIN tournament_providers tp ON tp.id = t.provider_id
		ORDER BY tp.name ASC, t.display_order ASC, t.start_date ASC
	`)
	return tournaments, err
}

func (r *tournamentRepository) FindByID(ctx context.Context, id uint) (*models.Tournament, error) {
	var tournament models.Tournament
	err := r.db.GetContext(ctx, &tournament, `
		SELECT t.id, t.name, t.description, t.courthive_tournament_id, t.provider_id,
		       t.start_date, t.end_date, t.is_visible, t.display_order, t.created_at, t.updated_at,
		       tp.name AS provider_name
		FROM tournaments t
		JOIN tournament_providers tp ON tp.id = t.provider_id
		WHERE t.id = ?
	`, id)
	if err != nil {
		return nil, err
	}
	return &tournament, nil
}

func (r *tournamentRepository) FindByProviderID(ctx context.Context, providerID uint) ([]models.Tournament, error) {
	var tournaments []models.Tournament
	err := r.db.SelectContext(ctx, &tournaments, `
		SELECT t.id, t.name, t.description, t.courthive_tournament_id, t.provider_id,
		       t.start_date, t.end_date, t.is_visible, t.display_order, t.created_at, t.updated_at,
		       tp.name AS provider_name
		FROM tournaments t
		JOIN tournament_providers tp ON tp.id = t.provider_id
		WHERE t.provider_id = ?
		ORDER BY t.display_order ASC, t.start_date ASC
	`, providerID)
	return tournaments, err
}

func (r *tournamentRepository) FindVisible(ctx context.Context) ([]models.Tournament, error) {
	var tournaments []models.Tournament
	err := r.db.SelectContext(ctx, &tournaments, `
		SELECT t.id, t.name, t.description, t.courthive_tournament_id, t.provider_id,
		       t.start_date, t.end_date, t.is_visible, t.display_order, t.created_at, t.updated_at,
		       tp.name AS provider_name
		FROM tournaments t
		JOIN tournament_providers tp ON tp.id = t.provider_id
		WHERE t.is_visible = 1
		ORDER BY t.display_order ASC, t.start_date ASC
	`)
	return tournaments, err
}

func (r *tournamentRepository) FindByCourthiveTournamentID(ctx context.Context, courthiveID string) (*models.Tournament, error) {
	var tournament models.Tournament
	err := r.db.GetContext(ctx, &tournament, `
		SELECT t.id, t.name, t.description, t.courthive_tournament_id, t.provider_id,
		       t.start_date, t.end_date, t.is_visible, t.display_order, t.created_at, t.updated_at,
		       tp.name AS provider_name
		FROM tournaments t
		JOIN tournament_providers tp ON tp.id = t.provider_id
		WHERE t.courthive_tournament_id = ?
	`, courthiveID)
	if err != nil {
		return nil, err
	}
	return &tournament, nil
}

func (r *tournamentRepository) Create(ctx context.Context, tournament *models.Tournament) error {
	now := time.Now()
	tournament.CreatedAt = now
	tournament.UpdatedAt = now

	result, err := r.db.NamedExecContext(ctx, `
		INSERT INTO tournaments (name, description, courthive_tournament_id, provider_id,
		                         start_date, end_date, is_visible, display_order, created_at, updated_at)
		VALUES (:name, :description, :courthive_tournament_id, :provider_id,
		        :start_date, :end_date, :is_visible, :display_order, :created_at, :updated_at)
	`, tournament)
	if err != nil {
		return err
	}

	if id, err := result.LastInsertId(); err == nil {
		tournament.ID = uint(id)
	}
	return nil
}

func (r *tournamentRepository) Update(ctx context.Context, tournament *models.Tournament) error {
	tournament.UpdatedAt = time.Now()

	_, err := r.db.NamedExecContext(ctx, `
		UPDATE tournaments
		SET name = :name, description = :description, courthive_tournament_id = :courthive_tournament_id,
		    provider_id = :provider_id, start_date = :start_date, end_date = :end_date,
		    is_visible = :is_visible, display_order = :display_order, updated_at = :updated_at
		WHERE id = :id
	`, tournament)
	return err
}

func (r *tournamentRepository) Delete(ctx context.Context, id uint) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM tournaments WHERE id = ?`, id)
	return err
}
