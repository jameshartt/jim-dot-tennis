// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package setup

import (
	"context"

	"github.com/jmoiron/sqlx"

	"jim-dot-tennis/internal/bhplta"
	"jim-dot-tennis/internal/config"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
)

// NewBHPLTAProvisioner returns the provisioner the server uses: it reads the
// current season from the BHPLTA website and imports the league, divisions,
// every club and team, the weeks and the fixtures, marks the chosen club as
// home, stores the club settings derivable from it
// (config.PrefillClubSettings) and records setup as complete, all in one
// transaction. The website is read before the transaction opens.
//
// If BHPLTA has not published any upcoming fixtures (between seasons) setup
// still completes, with the league and clubs only; the summary says how to
// add the season later. A season that already has divisions here is left
// alone.
func NewBHPLTAProvisioner(b *bhplta.Bootstrapper) HomeClubProvisioner {
	return &bhpltaProvisioner{b: b}
}

type bhpltaProvisioner struct {
	b *bhplta.Bootstrapper
}

func (p *bhpltaProvisioner) ProvisionHomeClub(ctx context.Context, choice ClubChoice, complete CompleteFunc) (*ProvisionResult, error) {
	opts := bhplta.Options{
		HomeSlug:           choice.Slug,
		HomeName:           choice.DisplayName,
		KeepExistingSeason: true,
	}
	progress := progressFrom(ctx)
	sum, err := p.b.Run(ctx, opts, bhplta.Progress(progress), func(ctx context.Context, tx *sqlx.Tx, sum *bhplta.Summary) error {
		club := &models.Club{ID: sum.HomeClubID, Name: sum.HomeClubName}
		if _, err := config.PrefillClubSettings(ctx, repository.NewSettingsRepository(tx), club); err != nil {
			return err
		}
		return complete(ctx, tx, sum.HomeClubID)
	})
	if err != nil {
		return nil, &ProvisionError{Message: bhplta.Describe(err), Err: err}
	}
	return &ProvisionResult{
		ClubID:      sum.HomeClubID,
		ClubName:    sum.HomeClubName,
		CreatedClub: sum.HomeClubCreated,
		Import:      sum,
	}, nil
}
