package admin

import (
	"context"
	"fmt"
	"log"
	"sort"

	"jim-dot-tennis/internal/models"
)

// WeekSelectionOverview contains all data for the selection overview dashboard
type WeekSelectionOverview struct {
	Week             *models.Week
	Fixtures         []FixtureOverviewCard
	DivisionSections []DivisionSection
	DivisionFilters  []DivisionFilter
	AllWeeks         []models.Week
}

// DivisionSection groups fixtures by division for display
type DivisionSection struct {
	DivisionName string
	Fixtures     []FixtureOverviewCard
}

// FixtureOverviewCard represents one fixture card in the overview
type FixtureOverviewCard struct {
	Fixture                  *FixtureWithRelations
	DivisionName             string
	DivisionLevel            int
	SelectedCount            int
	RequiredCount            int
	HigherDivisionSelections []DivisionSelectionSummary
	AvailablePlayerCount     int
	IsDerby                  bool
	HomeTeamSelectedCount    int
	AwayTeamSelectedCount    int
}

// DivisionSelectionSummary shows what a higher division selected
type DivisionSelectionSummary struct {
	DivisionName  string
	DivisionLevel int
	PlayerCount   int
}

// DivisionFilter for the filter UI
type DivisionFilter struct {
	DivisionID   uint
	DivisionName string
	Level        int
	IsFiltered   bool
}

// GetWeekSelectionOverview gets selection status for all St Ann's fixtures in a week
func (s *Service) GetWeekSelectionOverview(weekID uint, filteredDivisionIDs []uint) (*WeekSelectionOverview, error) {
	ctx := context.Background()

	// Get the week
	week, err := s.weekRepository.FindByID(ctx, weekID)
	if err != nil {
		return nil, fmt.Errorf("failed to get week: %w", err)
	}

	// Get all weeks for the selector dropdown
	allWeeks, err := s.weekRepository.FindBySeason(ctx, week.SeasonID)
	if err != nil {
		return nil, fmt.Errorf("failed to get all weeks: %w", err)
	}

	// Get all fixtures for the week
	fixtures, err := s.fixtureRepository.FindByWeek(ctx, weekID)
	if err != nil {
		return nil, fmt.Errorf("failed to get fixtures: %w", err)
	}

	// Get St Ann's club
	clubs, err := s.clubRepository.FindByNameLike(ctx, "St Ann")
	if err != nil {
		return nil, fmt.Errorf("failed to get St Ann's club: %w", err)
	}
	if len(clubs) == 0 {
		return nil, fmt.Errorf("St Ann's club not found")
	}
	stAnnsClub := &clubs[0]

	// Get all divisions to build filters
	divisions, err := s.divisionRepository.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get divisions: %w", err)
	}

	// Build division filter map
	filterMap := make(map[uint]bool)
	if len(filteredDivisionIDs) > 0 {
		for _, id := range filteredDivisionIDs {
			filterMap[id] = true
		}
	}

	// Build division filters
	var divisionFilters []DivisionFilter
	for _, div := range divisions {
		isFiltered := len(filteredDivisionIDs) == 0 || filterMap[div.ID]
		divisionFilters = append(divisionFilters, DivisionFilter{
			DivisionID:   div.ID,
			DivisionName: div.Name,
			Level:        div.Level,
			IsFiltered:   isFiltered,
		})
	}

	// Build fixture overview cards
	var fixtureCards []FixtureOverviewCard
	for _, fixture := range fixtures {
		// Load fixture relations using buildFixturesWithRelations
		fixturesWithRelations := s.buildFixturesWithRelations(ctx, []models.Fixture{fixture}, stAnnsClub)
		if len(fixturesWithRelations) == 0 {
			log.Printf("Error loading fixture relations for fixture %d", fixture.ID)
			continue
		}
		fixtureWithRelations := &fixturesWithRelations[0]

		// Check if this is a St Ann's fixture
		if !fixtureWithRelations.IsStAnnsHome && !fixtureWithRelations.IsStAnnsAway {
			continue
		}

		// Filter by division if requested
		if len(filterMap) > 0 && !filterMap[fixture.DivisionID] {
			continue
		}

		// Get division info
		division, err := s.divisionRepository.FindByID(ctx, fixture.DivisionID)
		if err != nil {
			log.Printf("Error getting division for fixture %d: %v", fixture.ID, err)
			continue
		}

		// Calculate selection counts
		var selectedCount int
		var homeSelectedCount int
		var awaySelectedCount int

		if fixtureWithRelations.IsDerby {
			// Derby: count separately for each team
			homeSelectedCount, err = s.fixtureRepository.GetSelectedPlayerCountByFixture(ctx, fixture.ID, &fixture.HomeTeamID)
			if err != nil {
				log.Printf("Error getting home team selection count: %v", err)
			}
			awaySelectedCount, err = s.fixtureRepository.GetSelectedPlayerCountByFixture(ctx, fixture.ID, &fixture.AwayTeamID)
			if err != nil {
				log.Printf("Error getting away team selection count: %v", err)
			}
			selectedCount = homeSelectedCount + awaySelectedCount
		} else {
			// Non-derby: total count
			selectedCount, err = s.fixtureRepository.GetSelectedPlayerCountByFixture(ctx, fixture.ID, nil)
			if err != nil {
				log.Printf("Error getting selection count: %v", err)
			}
		}

		// Get higher division selections (for divisions > 1)
		var higherDivSelections []DivisionSelectionSummary
		if division.Level > 1 {
			higherDivSelections, err = s.GetHigherDivisionSelectionsForWeek(weekID, division.Level)
			if err != nil {
				log.Printf("Error getting higher division selections: %v", err)
			}
		}

		// Calculate available players
		availableCount, err := s.GetAvailablePlayerCountForDivision(weekID, division.Level, stAnnsClub.ID)
		if err != nil {
			log.Printf("Error calculating available players: %v", err)
			availableCount = 0
		}

		fixtureCard := FixtureOverviewCard{
			Fixture:                  fixtureWithRelations,
			DivisionName:             division.Name,
			DivisionLevel:            division.Level,
			SelectedCount:            selectedCount,
			RequiredCount:            8,
			HigherDivisionSelections: higherDivSelections,
			AvailablePlayerCount:     availableCount,
			IsDerby:                  fixtureWithRelations.IsDerby,
			HomeTeamSelectedCount:    homeSelectedCount,
			AwayTeamSelectedCount:    awaySelectedCount,
		}

		fixtureCards = append(fixtureCards, fixtureCard)
	}

	// Group fixtures by division
	divisionMap := make(map[string][]FixtureOverviewCard)
	divisionOrder := []string{}
	for _, card := range fixtureCards {
		if _, exists := divisionMap[card.DivisionName]; !exists {
			divisionOrder = append(divisionOrder, card.DivisionName)
			divisionMap[card.DivisionName] = []FixtureOverviewCard{}
		}
		divisionMap[card.DivisionName] = append(divisionMap[card.DivisionName], card)
	}

	// Build division sections in order
	var divisionSections []DivisionSection
	for _, divName := range divisionOrder {
		divisionSections = append(divisionSections, DivisionSection{
			DivisionName: divName,
			Fixtures:     divisionMap[divName],
		})
	}

	return &WeekSelectionOverview{
		Week:             week,
		Fixtures:         fixtureCards,
		DivisionSections: divisionSections,
		DivisionFilters:  divisionFilters,
		AllWeeks:         allWeeks,
	}, nil
}

// GetHigherDivisionSelectionsForWeek calculates how many players higher divisions selected
func (s *Service) GetHigherDivisionSelectionsForWeek(weekID uint, currentDivisionLevel int) ([]DivisionSelectionSummary, error) {
	ctx := context.Background()

	if currentDivisionLevel <= 1 {
		return []DivisionSelectionSummary{}, nil
	}

	// Get all fixtures for higher divisions in this week
	fixtures, err := s.fixtureRepository.FindByWeekAndDivisionLevels(ctx, weekID, 1, currentDivisionLevel-1)
	if err != nil {
		return nil, fmt.Errorf("failed to get higher division fixtures: %w", err)
	}

	// Track selected players by division
	divisionSelections := make(map[uint]map[string]bool) // divisionID -> playerID -> selected
	divisionInfo := make(map[uint]*models.Division)

	for _, fixture := range fixtures {
		// Get selected players for this fixture
		selectedPlayers, err := s.fixtureRepository.FindSelectedPlayers(ctx, fixture.ID)
		if err != nil {
			log.Printf("Error getting selected players for fixture %d: %v", fixture.ID, err)
			continue
		}

		// Initialize map for this division if needed
		if _, exists := divisionSelections[fixture.DivisionID]; !exists {
			divisionSelections[fixture.DivisionID] = make(map[string]bool)

			// Load division info
			division, err := s.divisionRepository.FindByID(ctx, fixture.DivisionID)
			if err != nil {
				log.Printf("Error loading division %d: %v", fixture.DivisionID, err)
			} else {
				divisionInfo[fixture.DivisionID] = division
			}
		}

		// Add players to the set
		for _, fp := range selectedPlayers {
			divisionSelections[fixture.DivisionID][fp.PlayerID] = true
		}
	}

	// Build summary
	var summaries []DivisionSelectionSummary
	for divID, playerSet := range divisionSelections {
		div, ok := divisionInfo[divID]
		if !ok {
			continue
		}
		summaries = append(summaries, DivisionSelectionSummary{
			DivisionName:  div.Name,
			DivisionLevel: div.Level,
			PlayerCount:   len(playerSet),
		})
	}

	// Sort by division level
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].DivisionLevel < summaries[j].DivisionLevel
	})

	return summaries, nil
}

// GetAvailablePlayerCountForDivision calculates available players excluding higher divisions
func (s *Service) GetAvailablePlayerCountForDivision(weekID uint, divisionLevel int, clubID uint) (int, error) {
	ctx := context.Background()

	// Get total St Ann's player count
	players, err := s.playerRepository.FindByClub(ctx, clubID)
	if err != nil {
		return 0, fmt.Errorf("failed to get players: %w", err)
	}
	totalPlayers := len(players)

	// If division 1, return total (no exclusions)
	if divisionLevel == 1 {
		return totalPlayers, nil
	}

	// Get all fixtures in this week for higher divisions
	higherDivFixtures, err := s.fixtureRepository.FindByWeekAndDivisionLevels(ctx, weekID, 1, divisionLevel-1)
	if err != nil {
		return 0, fmt.Errorf("failed to get higher division fixtures: %w", err)
	}

	// Get all selected players in those fixtures (use map to deduplicate)
	selectedPlayerIDs := make(map[string]bool)
	for _, fixture := range higherDivFixtures {
		selectedPlayers, err := s.fixtureRepository.FindSelectedPlayers(ctx, fixture.ID)
		if err != nil {
			log.Printf("Error getting selected players for fixture %d: %v", fixture.ID, err)
			continue
		}
		for _, fp := range selectedPlayers {
			selectedPlayerIDs[fp.PlayerID] = true
		}
	}

	// Return count of available players
	return totalPlayers - len(selectedPlayerIDs), nil
}

// GetCurrentOrNextWeek determines the default week to show
func (s *Service) GetCurrentOrNextWeek(seasonID uint) (*models.Week, error) {
	ctx := context.Background()
	return s.weekRepository.FindCurrentOrNextWeek(ctx, seasonID)
}
