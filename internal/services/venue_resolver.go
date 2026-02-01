package services

import (
	"context"
	"database/sql"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
)

// VenueResolver resolves the venue for a fixture following the resolution order:
// 1. Per-fixture venue_club_id override
// 2. Active club date-range override for the home team's club on the fixture date
// 3. Home team's club (default)
type VenueResolver struct {
	clubRepository          repository.ClubRepository
	teamRepository          repository.TeamRepository
	venueOverrideRepository repository.VenueOverrideRepository
}

// VenueResolution contains the resolved venue information
type VenueResolution struct {
	Club             *models.Club  `json:"club"`
	IsOverridden     bool          `json:"is_overridden"`
	OverrideReason   string        `json:"override_reason,omitempty"`
	OverrideType     string        `json:"override_type,omitempty"` // "fixture" or "date_range"
}

// NewVenueResolver creates a new venue resolver
func NewVenueResolver(
	clubRepo repository.ClubRepository,
	teamRepo repository.TeamRepository,
	venueOverrideRepo repository.VenueOverrideRepository,
) *VenueResolver {
	return &VenueResolver{
		clubRepository:          clubRepo,
		teamRepository:          teamRepo,
		venueOverrideRepository: venueOverrideRepo,
	}
}

// ResolveFixtureVenue resolves the venue club for a fixture
func (vr *VenueResolver) ResolveFixtureVenue(ctx context.Context, fixture *models.Fixture) (*VenueResolution, error) {
	// 1. Check per-fixture venue override
	if fixture.VenueClubID != nil {
		club, err := vr.clubRepository.FindByID(ctx, *fixture.VenueClubID)
		if err != nil {
			return nil, err
		}
		return &VenueResolution{
			Club:           club,
			IsOverridden:   true,
			OverrideReason: "Per-fixture venue override",
			OverrideType:   "fixture",
		}, nil
	}

	// 2. Get the home team's club
	homeTeam, err := vr.teamRepository.FindByID(ctx, fixture.HomeTeamID)
	if err != nil {
		return nil, err
	}

	homeClub, err := vr.clubRepository.FindByID(ctx, homeTeam.ClubID)
	if err != nil {
		return nil, err
	}

	// 3. Check for active date-range override for the home team's club
	override, err := vr.venueOverrideRepository.FindActiveForClubOnDate(ctx, homeClub.ID, fixture.ScheduledDate)
	if err != nil && err != sql.ErrNoRows {
		// If there's an actual error (not just "no rows"), we still fall through to default
		// but log it via the caller
	}
	if err == nil && override != nil {
		venueClub, err := vr.clubRepository.FindByID(ctx, override.VenueClubID)
		if err != nil {
			return nil, err
		}
		return &VenueResolution{
			Club:           venueClub,
			IsOverridden:   true,
			OverrideReason: override.Reason,
			OverrideType:   "date_range",
		}, nil
	}

	// 4. Default: home team's club
	return &VenueResolution{
		Club:         homeClub,
		IsOverridden: false,
	}, nil
}
