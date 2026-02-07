package admin

import (
	"context"
	"fmt"
	"time"

	"jim-dot-tennis/internal/models"
)

// MatchupPlayerWithInfo represents a matchup player with their details
type MatchupPlayerWithInfo struct {
	MatchupPlayer models.MatchupPlayer `json:"matchup_player"`
	Player        models.Player        `json:"player"`
}

// MatchupWithPlayers represents a matchup with its assigned players
type MatchupWithPlayers struct {
	Matchup models.Matchup          `json:"matchup"`
	Players []MatchupPlayerWithInfo `json:"players"`
}

// DuplicatePlayerWarning represents a warning about duplicate player assignments
type DuplicatePlayerWarning struct {
	PlayerID   string   `json:"player_id"`
	PlayerName string   `json:"player_name"`
	Matchups   []string `json:"matchups"` // List of matchup types where this player appears
}

// detectDuplicatePlayersInMatchups checks for players assigned to multiple matchups
func (s *Service) detectDuplicatePlayersInMatchups(matchups []MatchupWithPlayers) []DuplicatePlayerWarning {
	// Map to track which matchups each player appears in
	playerMatchups := make(map[string][]string)
	playerNames := make(map[string]string)

	// Collect all player assignments
	for _, matchup := range matchups {
		matchupType := string(matchup.Matchup.Type)
		for _, player := range matchup.Players {
			playerID := player.Player.ID
			playerName := player.Player.FirstName + " " + player.Player.LastName

			playerMatchups[playerID] = append(playerMatchups[playerID], matchupType)
			playerNames[playerID] = playerName
		}
	}

	// Find duplicates
	var warnings []DuplicatePlayerWarning
	for playerID, matchupTypes := range playerMatchups {
		if len(matchupTypes) > 1 {
			warnings = append(warnings, DuplicatePlayerWarning{
				PlayerID:   playerID,
				PlayerName: playerNames[playerID],
				Matchups:   matchupTypes,
			})
		}
	}

	return warnings
}

// getMatchupOrder returns the sort order for matchup types
// Order: 1st Mixed, 2nd Mixed, Mens, Womens
func getMatchupOrder(matchupType models.MatchupType) int {
	switch matchupType {
	case models.FirstMixed:
		return 0
	case models.SecondMixed:
		return 1
	case models.Mens:
		return 2
	case models.Womens:
		return 3
	default:
		return 4 // Unknown types go last
	}
}

// CreateMatchupWithTeam creates a new matchup with explicit managing team ID (for derby matches)
func (s *Service) CreateMatchupWithTeam(fixtureID uint, matchupType models.MatchupType, managingTeamID uint) (*models.Matchup, error) {
	ctx := context.Background()

	// Check if matchup already exists for this fixture, type, and managing team
	existingMatchup, err := s.matchupRepository.FindByFixtureTypeAndTeam(ctx, fixtureID, matchupType, managingTeamID)
	if err == nil && existingMatchup != nil {
		return existingMatchup, fmt.Errorf("matchup of type %s already exists for this fixture and team", matchupType)
	}

	// Create new matchup
	matchup := &models.Matchup{
		FixtureID:      fixtureID,
		Type:           matchupType,
		Status:         models.Pending,
		HomeScore:      0,
		AwayScore:      0,
		Notes:          "",
		ManagingTeamID: &managingTeamID,
	}

	err = s.matchupRepository.Create(ctx, matchup)
	if err != nil {
		return nil, err
	}

	return matchup, nil
}

// determineManagingTeamID determines which team should manage a matchup for a given fixture
func (s *Service) determineManagingTeamID(ctx context.Context, fixtureID uint) (uint, error) {
	// Get the fixture to determine the teams
	fixture, err := s.fixtureRepository.FindByID(ctx, fixtureID)
	if err != nil {
		return 0, err
	}

	// Find the St Ann's club ID
	stAnnsClubs, err := s.clubRepository.FindByNameLike(ctx, "St Ann")
	if err != nil {
		return 0, err
	}
	if len(stAnnsClubs) == 0 {
		return 0, fmt.Errorf("St Ann's club not found")
	}
	stAnnsClubID := stAnnsClubs[0].ID

	// Get home and away teams
	homeTeam, err := s.teamRepository.FindByID(ctx, fixture.HomeTeamID)
	if err != nil {
		return 0, err
	}

	awayTeam, err := s.teamRepository.FindByID(ctx, fixture.AwayTeamID)
	if err != nil {
		return 0, err
	}

	// Check which team is St Ann's - prefer home team if both are St Ann's
	if homeTeam.ClubID == stAnnsClubID {
		return homeTeam.ID, nil
	} else if awayTeam.ClubID == stAnnsClubID {
		return awayTeam.ID, nil
	} else {
		return 0, fmt.Errorf("no St Ann's team found in this fixture")
	}
}

// GetOrCreateMatchup gets an existing matchup or creates a new one (legacy version for regular matches)
func (s *Service) GetOrCreateMatchup(fixtureID uint, matchupType models.MatchupType) (*models.Matchup, error) {
	ctx := context.Background()

	// Determine the managing team ID
	managingTeamID, err := s.determineManagingTeamID(ctx, fixtureID)
	if err != nil {
		return nil, err
	}

	return s.GetOrCreateMatchupWithTeam(fixtureID, matchupType, managingTeamID)
}

// GetOrCreateMatchupWithTeam gets an existing matchup or creates a new one for a specific managing team (for derby matches)
func (s *Service) GetOrCreateMatchupWithTeam(fixtureID uint, matchupType models.MatchupType, managingTeamID uint) (*models.Matchup, error) {
	ctx := context.Background()

	// Try to find existing matchup for this team
	matchup, err := s.matchupRepository.FindByFixtureTypeAndTeam(ctx, fixtureID, matchupType, managingTeamID)
	if err == nil && matchup != nil {
		return matchup, nil
	}

	// Create new matchup if it doesn't exist
	return s.CreateMatchupWithTeam(fixtureID, matchupType, managingTeamID)
}

// UpdateStAnnsMatchupPlayers updates St Ann's players for a matchup, determining if they're home or away
func (s *Service) UpdateStAnnsMatchupPlayers(matchupID uint, fixtureID uint, stAnnsPlayer1ID, stAnnsPlayer2ID string) error {
	ctx := context.Background()

	// Get the fixture to determine if St Ann's is home or away
	fixture, err := s.fixtureRepository.FindByID(ctx, fixtureID)
	if err != nil {
		return err
	}

	// Find the St Ann's club ID
	stAnnsClubs, err := s.clubRepository.FindByNameLike(ctx, "St Ann")
	if err != nil {
		return err
	}
	if len(stAnnsClubs) == 0 {
		return fmt.Errorf("St Ann's club not found")
	}
	stAnnsClubID := stAnnsClubs[0].ID

	// Get home and away teams
	homeTeam, err := s.teamRepository.FindByID(ctx, fixture.HomeTeamID)
	if err != nil {
		return err
	}

	awayTeam, err := s.teamRepository.FindByID(ctx, fixture.AwayTeamID)
	if err != nil {
		return err
	}

	// Determine if St Ann's is home or away
	var isStAnnsHome bool
	if homeTeam.ClubID == stAnnsClubID {
		isStAnnsHome = true
	} else if awayTeam.ClubID == stAnnsClubID {
		isStAnnsHome = false
	} else {
		return fmt.Errorf("no St Ann's team found in this fixture")
	}

	// Clear existing players
	err = s.matchupRepository.ClearPlayers(ctx, matchupID)
	if err != nil {
		return err
	}

	// Add St Ann's players with correct home/away designation
	if stAnnsPlayer1ID != "" {
		err = s.matchupRepository.AddPlayer(ctx, matchupID, stAnnsPlayer1ID, isStAnnsHome)
		if err != nil {
			return err
		}
	}
	if stAnnsPlayer2ID != "" {
		err = s.matchupRepository.AddPlayer(ctx, matchupID, stAnnsPlayer2ID, isStAnnsHome)
		if err != nil {
			return err
		}
	}

	// Update status to Playing if both St Ann's players are assigned
	// (In a real-world scenario, you'd need opponent players too, but for St Ann's tool this is sufficient)
	if stAnnsPlayer1ID != "" && stAnnsPlayer2ID != "" {
		err = s.matchupRepository.UpdateStatus(ctx, matchupID, models.Playing)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetAvailablePlayersForMatchup gets available players for a specific matchup
// Returns selected players if any, otherwise falls back to all St Ann's team players
func (s *Service) GetAvailablePlayersForMatchup(fixtureID uint) ([]models.Player, error) {
	ctx := context.Background()

	// First try to get selected players from the fixture
	selectedPlayers, err := s.fixtureRepository.FindSelectedPlayers(ctx, fixtureID)
	if err != nil {
		return nil, err
	}

	if len(selectedPlayers) > 0 {
		// Use selected players from fixture
		var players []models.Player
		for _, sp := range selectedPlayers {
			if player, err := s.playerRepository.FindByID(ctx, sp.PlayerID); err == nil {
				players = append(players, *player)
			}
		}
		return players, nil
	}

	// Fallback to St Ann's team players if no players selected
	teamPlayers, allStAnnPlayers, err := s.GetAvailablePlayersForFixture(fixtureID)
	if err != nil {
		return nil, err
	}

	// Prefer team players, but if none exist, use all St Ann's players
	if len(teamPlayers) > 0 {
		return teamPlayers, nil
	}

	// Combine team players and all St Ann's players as final fallback
	allPlayers := append(teamPlayers, allStAnnPlayers...)
	return allPlayers, nil
}

// getMatchupsForTeam gets matchups for a specific team - used for derby matches
func (s *Service) getMatchupsForTeam(ctx context.Context, fixtureID uint, managingTeamID uint) ([]models.Matchup, error) {
	// Get all matchups for the fixture
	allMatchups, err := s.matchupRepository.FindByFixture(ctx, fixtureID)
	if err != nil {
		return nil, err
	}

	// Filter by managing team ID
	var teamMatchups []models.Matchup
	for _, matchup := range allMatchups {
		// Include matchups that belong to this managing team
		if matchup.ManagingTeamID != nil && *matchup.ManagingTeamID == managingTeamID {
			teamMatchups = append(teamMatchups, matchup)
		} else if matchup.ManagingTeamID == nil {
			// Legacy matchups without managing team ID - include them for backward compatibility
			teamMatchups = append(teamMatchups, matchup)
		}
	}

	return teamMatchups, nil
}

// AddPlayerToMatchup adds a single player to a matchup without replacing existing players
func (s *Service) AddPlayerToMatchup(matchupID uint, playerID string, fixtureID uint) error {
	ctx := context.Background()

	// Get the fixture to determine if St Ann's is home or away
	fixture, err := s.fixtureRepository.FindByID(ctx, fixtureID)
	if err != nil {
		return err
	}

	// Find the St Ann's club ID
	stAnnsClubs, err := s.clubRepository.FindByNameLike(ctx, "St Ann")
	if err != nil {
		return err
	}
	if len(stAnnsClubs) == 0 {
		return fmt.Errorf("St Ann's club not found")
	}
	stAnnsClubID := stAnnsClubs[0].ID

	// Get home and away teams
	homeTeam, err := s.teamRepository.FindByID(ctx, fixture.HomeTeamID)
	if err != nil {
		return err
	}

	awayTeam, err := s.teamRepository.FindByID(ctx, fixture.AwayTeamID)
	if err != nil {
		return err
	}

	// Determine if St Ann's is home or away
	var isStAnnsHome bool
	if homeTeam.ClubID == stAnnsClubID {
		isStAnnsHome = true
	} else if awayTeam.ClubID == stAnnsClubID {
		isStAnnsHome = false
	} else {
		return fmt.Errorf("no St Ann's team found in this fixture")
	}

	// Check if player is already in this matchup
	existingPlayers, err := s.matchupRepository.FindPlayersInMatchup(ctx, matchupID)
	if err != nil {
		return err
	}

	for _, existingPlayer := range existingPlayers {
		if existingPlayer.PlayerID == playerID {
			return fmt.Errorf("player is already assigned to this matchup")
		}
	}

	// Add the player to the matchup
	err = s.matchupRepository.AddPlayer(ctx, matchupID, playerID, isStAnnsHome)
	if err != nil {
		return err
	}

	return nil
}

// SaveMatchupResults saves scores for all matchups in a fixture
func (s *Service) SaveMatchupResults(fixtureID uint, entries []MatchupScoreEntry) error {
	ctx := context.Background()

	for _, entry := range entries {
		matchup, err := s.matchupRepository.FindByID(ctx, entry.MatchupID)
		if err != nil {
			return fmt.Errorf("matchup %d not found: %w", entry.MatchupID, err)
		}

		if entry.Conceded {
			// Handle conceded matchup
			concededBy := entry.ConcededBy
			matchup.ConcededBy = &concededBy
			matchup.Status = models.Defaulted
			// Conceding side gets 0 points, other side gets 2
			if concededBy == models.ConcededHome {
				matchup.HomeScore = 0
				matchup.AwayScore = 2
			} else {
				matchup.HomeScore = 2
				matchup.AwayScore = 0
			}
			// Clear set scores
			matchup.HomeSet1 = nil
			matchup.AwaySet1 = nil
			matchup.HomeSet2 = nil
			matchup.AwaySet2 = nil
			matchup.HomeSet3 = nil
			matchup.AwaySet3 = nil
		} else {
			// Set scores
			matchup.HomeSet1 = entry.HomeSet1
			matchup.AwaySet1 = entry.AwaySet1
			matchup.HomeSet2 = entry.HomeSet2
			matchup.AwaySet2 = entry.AwaySet2
			matchup.HomeSet3 = entry.HomeSet3
			matchup.AwaySet3 = entry.AwaySet3
			matchup.Status = models.Finished
			matchup.ConcededBy = nil

			// Calculate HomeScore/AwayScore from sets won
			homeSetsWon, awaySetsWon := countSetsWon(entry)
			if homeSetsWon > awaySetsWon {
				matchup.HomeScore = 2
				matchup.AwayScore = 0
			} else if awaySetsWon > homeSetsWon {
				matchup.HomeScore = 0
				matchup.AwayScore = 2
			} else {
				matchup.HomeScore = 1
				matchup.AwayScore = 1
			}
		}

		if err := s.matchupRepository.Update(ctx, matchup); err != nil {
			return fmt.Errorf("failed to update matchup %d: %w", entry.MatchupID, err)
		}
	}

	return nil
}

// CompleteFixtureWithResults marks a fixture as completed
func (s *Service) CompleteFixtureWithResults(fixtureID uint) error {
	ctx := context.Background()

	fixture, err := s.fixtureRepository.FindByID(ctx, fixtureID)
	if err != nil {
		return fmt.Errorf("fixture not found: %w", err)
	}

	fixture.Status = models.Completed
	completedDate := time.Now()
	fixture.CompletedDate = &completedDate

	return s.fixtureRepository.Update(ctx, fixture)
}

// countSetsWon counts sets won by each side from a score entry
func countSetsWon(entry MatchupScoreEntry) (homeSetsWon, awaySetsWon int) {
	if entry.HomeSet1 != nil && entry.AwaySet1 != nil {
		if *entry.HomeSet1 > *entry.AwaySet1 {
			homeSetsWon++
		} else if *entry.AwaySet1 > *entry.HomeSet1 {
			awaySetsWon++
		}
	}
	if entry.HomeSet2 != nil && entry.AwaySet2 != nil {
		if *entry.HomeSet2 > *entry.AwaySet2 {
			homeSetsWon++
		} else if *entry.AwaySet2 > *entry.HomeSet2 {
			awaySetsWon++
		}
	}
	if entry.HomeSet3 != nil && entry.AwaySet3 != nil {
		if *entry.HomeSet3 > *entry.AwaySet3 {
			homeSetsWon++
		} else if *entry.AwaySet3 > *entry.HomeSet3 {
			awaySetsWon++
		}
	}
	return
}

// RemovePlayerFromMatchup removes a single player from a matchup
func (s *Service) RemovePlayerFromMatchup(matchupID uint, playerID string) error {
	ctx := context.Background()

	// Remove the player from the matchup
	if err := s.matchupRepository.RemovePlayer(ctx, matchupID, playerID); err != nil {
		return err
	}

	// If no players remain in this matchup, delete the matchup to keep the UI clean
	remainingPlayers, err := s.matchupRepository.FindPlayersInMatchup(ctx, matchupID)
	if err != nil {
		return err
	}

	if len(remainingPlayers) == 0 {
		if err := s.matchupRepository.Delete(ctx, matchupID); err != nil {
			return err
		}
	}

	return nil
}
