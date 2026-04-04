package repository

import (
	"context"
	"time"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
)

// TournamentProviderRepository defines the interface for tournament provider data access
type TournamentProviderRepository interface {
	FindAll(ctx context.Context) ([]models.TournamentProvider, error)
	FindAllWithCounts(ctx context.Context) ([]models.TournamentProvider, error)
	FindByID(ctx context.Context, id uint) (*models.TournamentProvider, error)
	FindByAbbr(ctx context.Context, abbr string) (*models.TournamentProvider, error)
	Create(ctx context.Context, provider *models.TournamentProvider) error
	Update(ctx context.Context, provider *models.TournamentProvider) error
	Delete(ctx context.Context, id uint) error
	CountTournaments(ctx context.Context, providerID uint) (int, error)
}

type tournamentProviderRepository struct {
	db *database.DB
}

func NewTournamentProviderRepository(db *database.DB) TournamentProviderRepository {
	return &tournamentProviderRepository{db: db}
}

func (r *tournamentProviderRepository) FindAll(ctx context.Context) ([]models.TournamentProvider, error) {
	var providers []models.TournamentProvider
	err := r.db.SelectContext(ctx, &providers, `
		SELECT id, name, provider_abbr, created_at, updated_at
		FROM tournament_providers
		ORDER BY name ASC
	`)
	return providers, err
}

func (r *tournamentProviderRepository) FindAllWithCounts(ctx context.Context) ([]models.TournamentProvider, error) {
	var providers []models.TournamentProvider
	err := r.db.SelectContext(ctx, &providers, `
		SELECT tp.id, tp.name, tp.provider_abbr, tp.created_at, tp.updated_at,
		       COUNT(t.id) AS tournament_count
		FROM tournament_providers tp
		LEFT JOIN tournaments t ON t.provider_id = tp.id
		GROUP BY tp.id
		ORDER BY tp.name ASC
	`)
	return providers, err
}

func (r *tournamentProviderRepository) FindByID(ctx context.Context, id uint) (*models.TournamentProvider, error) {
	var provider models.TournamentProvider
	err := r.db.GetContext(ctx, &provider, `
		SELECT id, name, provider_abbr, created_at, updated_at
		FROM tournament_providers
		WHERE id = ?
	`, id)
	if err != nil {
		return nil, err
	}
	return &provider, nil
}

func (r *tournamentProviderRepository) FindByAbbr(ctx context.Context, abbr string) (*models.TournamentProvider, error) {
	var provider models.TournamentProvider
	err := r.db.GetContext(ctx, &provider, `
		SELECT id, name, provider_abbr, created_at, updated_at
		FROM tournament_providers
		WHERE provider_abbr = ?
	`, abbr)
	if err != nil {
		return nil, err
	}
	return &provider, nil
}

func (r *tournamentProviderRepository) Create(ctx context.Context, provider *models.TournamentProvider) error {
	now := time.Now()
	provider.CreatedAt = now
	provider.UpdatedAt = now

	result, err := r.db.NamedExecContext(ctx, `
		INSERT INTO tournament_providers (name, provider_abbr, created_at, updated_at)
		VALUES (:name, :provider_abbr, :created_at, :updated_at)
	`, provider)
	if err != nil {
		return err
	}

	if id, err := result.LastInsertId(); err == nil {
		provider.ID = uint(id)
	}
	return nil
}

func (r *tournamentProviderRepository) Update(ctx context.Context, provider *models.TournamentProvider) error {
	provider.UpdatedAt = time.Now()

	_, err := r.db.NamedExecContext(ctx, `
		UPDATE tournament_providers
		SET name = :name, provider_abbr = :provider_abbr, updated_at = :updated_at
		WHERE id = :id
	`, provider)
	return err
}

func (r *tournamentProviderRepository) Delete(ctx context.Context, id uint) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM tournament_providers WHERE id = ?`, id)
	return err
}

func (r *tournamentProviderRepository) CountTournaments(ctx context.Context, providerID uint) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM tournaments WHERE provider_id = ?
	`, providerID)
	return count, err
}
