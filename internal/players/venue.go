package players

import (
	"context"
	"fmt"

	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/services"
)

// FixtureVenueDetail contains all the data needed for the player-facing fixture venue page
type FixtureVenueDetail struct {
	Fixture         *models.Fixture      `json:"fixture"`
	HomeTeam        *models.Team         `json:"home_team"`
	AwayTeam        *models.Team         `json:"away_team"`
	Division        *models.Division     `json:"division"`
	WeekNumber      int                  `json:"week_number"`
	VenueClub       *models.Club         `json:"venue_club"`
	IsOverridden    bool                 `json:"is_overridden"`
	OverrideReason  string               `json:"override_reason,omitempty"`
	SelectedPlayers []SelectedPlayerInfo `json:"selected_players,omitempty"`
	PlayerTeamName  string               `json:"player_team_name"` // The viewing player's team
	IsHome          bool                 `json:"is_home"`
}

// SelectedPlayerInfo contains privacy-focused player info for the venue page
type SelectedPlayerInfo struct {
	FirstName       string `json:"first_name"`
	LastInitial     string `json:"last_initial"`
	IsHome          bool   `json:"is_home"`
	IsViewingPlayer bool   `json:"is_viewing_player"`
	FullName        string `json:"full_name,omitempty"` // Only set for the viewing player
}

// GetFixtureVenueDetail retrieves comprehensive venue detail for a fixture
func (s *Service) GetFixtureVenueDetail(playerID string, fixtureID uint) (*FixtureVenueDetail, error) {
	ctx := context.Background()

	// Load fixture
	fixture, err := s.fixtureRepository.FindByID(ctx, fixtureID)
	if err != nil {
		return nil, fmt.Errorf("fixture not found: %w", err)
	}

	// Load teams
	homeTeam, err := s.teamRepository.FindByID(ctx, fixture.HomeTeamID)
	if err != nil {
		return nil, fmt.Errorf("home team not found: %w", err)
	}

	awayTeam, err := s.teamRepository.FindByID(ctx, fixture.AwayTeamID)
	if err != nil {
		return nil, fmt.Errorf("away team not found: %w", err)
	}

	// Load division
	var division *models.Division
	if d, err := s.divisionRepository.FindByID(ctx, fixture.DivisionID); err == nil {
		division = d
	}

	// Load week number
	weekNumber := 0
	if w, err := s.weekRepository.FindByID(ctx, fixture.WeekID); err == nil {
		weekNumber = w.WeekNumber
	}

	// Resolve venue using the resolution order
	venueResolver := services.NewVenueResolver(s.clubRepository, s.teamRepository, s.venueOverrideRepository)
	resolution, err := venueResolver.ResolveFixtureVenue(ctx, fixture)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve venue: %w", err)
	}

	// Determine which team the viewing player is on
	_, isPlayerHome, _ := s.determinePlayerTeamContext(ctx, playerID, fixtureID, homeTeam.ID, awayTeam.ID)
	playerTeamName := awayTeam.Name
	if isPlayerHome {
		playerTeamName = homeTeam.Name
	}

	// Load selected players with privacy
	selectedPlayers, err := s.getSelectedPlayersForVenue(ctx, playerID, fixtureID)
	if err != nil {
		// Non-fatal, just show no players
		selectedPlayers = []SelectedPlayerInfo{}
	}

	return &FixtureVenueDetail{
		Fixture:         fixture,
		HomeTeam:        homeTeam,
		AwayTeam:        awayTeam,
		Division:        division,
		WeekNumber:      weekNumber,
		VenueClub:       resolution.Club,
		IsOverridden:    resolution.IsOverridden,
		OverrideReason:  resolution.OverrideReason,
		SelectedPlayers: selectedPlayers,
		PlayerTeamName:  playerTeamName,
		IsHome:          isPlayerHome,
	}, nil
}

// getSelectedPlayersForVenue loads selected players with privacy formatting
func (s *Service) getSelectedPlayersForVenue(ctx context.Context, viewingPlayerID string, fixtureID uint) ([]SelectedPlayerInfo, error) {
	fixturePlayers, err := s.fixtureRepository.FindSelectedPlayers(ctx, fixtureID)
	if err != nil {
		return nil, err
	}

	if len(fixturePlayers) == 0 {
		return nil, nil
	}

	var result []SelectedPlayerInfo
	for _, fp := range fixturePlayers {
		player, err := s.playerRepository.FindByID(ctx, fp.PlayerID)
		if err != nil {
			continue
		}

		displayName := player.FirstName
		if player.PreferredName != nil && *player.PreferredName != "" {
			displayName = *player.PreferredName
		}

		info := SelectedPlayerInfo{
			FirstName:       displayName,
			LastInitial:     string([]rune(player.LastName)[0:1]) + ".",
			IsHome:          fp.IsHome,
			IsViewingPlayer: fp.PlayerID == viewingPlayerID,
		}

		// Show full name only for the viewing player
		if fp.PlayerID == viewingPlayerID {
			info.FullName = displayName + " " + player.LastName
		}

		result = append(result, info)
	}

	return result, nil
}

// GenerateFixtureICal generates an iCal event for a fixture
func (s *Service) GenerateFixtureICal(fixtureID uint) (string, error) {
	ctx := context.Background()

	fixture, err := s.fixtureRepository.FindByID(ctx, fixtureID)
	if err != nil {
		return "", fmt.Errorf("fixture not found: %w", err)
	}

	homeTeam, err := s.teamRepository.FindByID(ctx, fixture.HomeTeamID)
	if err != nil {
		return "", fmt.Errorf("home team not found: %w", err)
	}

	awayTeam, err := s.teamRepository.FindByID(ctx, fixture.AwayTeamID)
	if err != nil {
		return "", fmt.Errorf("away team not found: %w", err)
	}

	// Resolve venue
	venueResolver := services.NewVenueResolver(s.clubRepository, s.teamRepository, s.venueOverrideRepository)
	resolution, err := venueResolver.ResolveFixtureVenue(ctx, fixture)
	if err != nil {
		return "", fmt.Errorf("failed to resolve venue: %w", err)
	}

	// Get division and week
	divisionName := ""
	if d, err := s.divisionRepository.FindByID(ctx, fixture.DivisionID); err == nil {
		divisionName = d.Name
	}
	weekNumber := 0
	if w, err := s.weekRepository.FindByID(ctx, fixture.WeekID); err == nil {
		weekNumber = w.WeekNumber
	}

	event := services.BuildICalEventFromFixture(
		fixture,
		homeTeam.Name,
		awayTeam.Name,
		divisionName,
		weekNumber,
		resolution.Club,
	)

	return services.GenerateICalEvent(event), nil
}
