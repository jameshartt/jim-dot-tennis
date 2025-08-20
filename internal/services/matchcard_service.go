package services

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// MatchCardService handles importing match card data from BHPLTA
type MatchCardService struct {
	fixtureRepo    repository.FixtureRepository
	matchupRepo    repository.MatchupRepository
	teamRepo       repository.TeamRepository
	clubRepo       repository.ClubRepository
	playerRepo     repository.PlayerRepository
	parser         *MatchCardParser
	matcher        *PlayerMatcher
	httpClient     *http.Client
	nonceExtractor *NonceExtractor
}

// ImportConfig holds configuration for importing match cards
type ImportConfig struct {
	ClubName              string
	ClubID                int // Club ID from BHPLTA
	Year                  int // Year for the season
	Nonce                 string
	ClubCode              string // Club code cookie value
	BaseURL               string
	RateLimit             time.Duration // Delay between requests
	DryRun                bool          // If true, don't save to database
	Verbose               bool          // If true, output detailed logs
	ClearExistingMatchups bool          // If true, clear existing matchups before processing
}

// ImportResult holds the results of an import operation
type ImportResult struct {
	ProcessedMatches int
	UpdatedFixtures  int
	CreatedMatchups  int
	UpdatedMatchups  int
	MatchedPlayers   int
	UnmatchedPlayers []string
	Errors           []string
}

// NewMatchCardService creates a new match card service
func NewMatchCardService(
	fixtureRepo repository.FixtureRepository,
	matchupRepo repository.MatchupRepository,
	teamRepo repository.TeamRepository,
	clubRepo repository.ClubRepository,
	playerRepo repository.PlayerRepository,
) *MatchCardService {
	return &MatchCardService{
		fixtureRepo:    fixtureRepo,
		matchupRepo:    matchupRepo,
		teamRepo:       teamRepo,
		clubRepo:       clubRepo,
		playerRepo:     playerRepo,
		parser:         NewMatchCardParser(),
		matcher:        NewPlayerMatcher(playerRepo),
		nonceExtractor: NewNonceExtractor(),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ImportWeekMatchCardsWithAutoNonce imports match cards with automatic nonce extraction
func (s *MatchCardService) ImportWeekMatchCardsWithAutoNonce(ctx context.Context, config ImportConfig, week int) (*ImportResult, error) {
	// If no nonce is provided, extract it automatically
	if config.Nonce == "" {
		if config.Verbose {
			fmt.Printf("No nonce provided, attempting to extract from website...\n")
		}

		var nonceResult *NonceResult
		var err error

		if config.ClubCode != "" {
			// Use club code if available for better access
			nonceResult, err = s.nonceExtractor.ExtractNonceWithClubCode(config.ClubCode)
		} else {
			// Extract without club code
			nonceResult, err = s.nonceExtractor.ExtractNonce()
		}

		if err != nil {
			return nil, fmt.Errorf("failed to extract nonce: %w", err)
		}

		config.Nonce = nonceResult.Nonce
		if config.Verbose {
			fmt.Printf("Successfully extracted nonce: %s...\n", config.Nonce[:10])
		}
	}

	return s.ImportWeekMatchCards(ctx, config, week)
}

// ImportWeekMatchCards imports match cards for a specific week
func (s *MatchCardService) ImportWeekMatchCards(ctx context.Context, config ImportConfig, week int) (*ImportResult, error) {
	if config.Verbose {
		fmt.Printf("Fetching match cards for week %d...\n", week)
	}

	// Fetch match cards from BHPLTA API
	responseBody, err := s.fetchMatchCards(config, week)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch match cards for week %d: %w", week, err)
	}

	// Parse match cards from response
	matchCards, err := s.parser.ParseResponse(responseBody)
	if err != nil {
		return nil, fmt.Errorf("failed to parse match cards for week %d: %w", week, err)
	}

	if config.Verbose {
		fmt.Printf("Found %d match cards for week %d\n", len(matchCards), week)
	}

	// Process each match card
	totalResult := &ImportResult{
		UnmatchedPlayers: []string{},
		Errors:           []string{},
	}

	for _, matchCard := range matchCards {
		result, err := s.processMatchCard(ctx, config, matchCard)
		if err != nil {
			errMsg := fmt.Sprintf("Error processing match card %d: %v", matchCard.ExternalID, err)
			totalResult.Errors = append(totalResult.Errors, errMsg)
			continue
		}

		// Aggregate results
		totalResult.ProcessedMatches++
		totalResult.UpdatedFixtures += result.UpdatedFixtures
		totalResult.CreatedMatchups += result.CreatedMatchups
		totalResult.UpdatedMatchups += result.UpdatedMatchups
		totalResult.MatchedPlayers += result.MatchedPlayers
		totalResult.UnmatchedPlayers = append(totalResult.UnmatchedPlayers, result.UnmatchedPlayers...)
		totalResult.Errors = append(totalResult.Errors, result.Errors...)

		// Rate limiting
		if config.RateLimit > 0 {
			time.Sleep(config.RateLimit)
		}
	}

	return totalResult, nil
}

// fetchMatchCards fetches match card data from BHPLTA API
func (s *MatchCardService) fetchMatchCards(config ImportConfig, week int) ([]byte, error) {
	// Prepare form data
	formData := url.Values{}
	formData.Set("nonce", config.Nonce)
	formData.Set("action", "bhplta_club_scores_get_scores_week_change")
	formData.Set("selected_week", strconv.Itoa(week))
	formData.Set("year", strconv.Itoa(config.Year))
	formData.Set("club_id", strconv.Itoa(config.ClubID))
	formData.Set("club_name", config.ClubName)
	formData.Set("passcode", "") // Empty as per user's example

	// Create request
	req, err := http.NewRequest("POST", config.BaseURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers to match the browser request
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-GB,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:140.0) Gecko/20100101 Firefox/140.0")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Origin", "https://www.bhplta.co.uk")
	req.Header.Set("Referer", "https://www.bhplta.co.uk/bhplta_tables/parks-league-match-cards/")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")

	// Set cookies for authentication - use the same format as working curl
	cookieParts := []string{
		"clubcode=" + config.ClubCode,
	}

	// Set the Cookie header directly
	req.Header.Set("Cookie", strings.Join(cookieParts, "; "))

	// Execute request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Handle compressed response
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	// Read response body
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return body, nil
}

// processMatchCard processes a single match card
func (s *MatchCardService) processMatchCard(ctx context.Context, config ImportConfig, matchCard MatchCardData) (*ImportResult, error) {
	result := &ImportResult{
		UnmatchedPlayers: []string{},
		Errors:           []string{},
	}

	// Find the fixture in our database by matching teams and date
	fixture, err := s.findMatchingFixture(ctx, matchCard)
	if err != nil {
		return nil, fmt.Errorf("failed to find matching fixture: %w", err)
	}

	if fixture == nil {
		if config.Verbose {
			fmt.Printf("No matching fixture found for %s vs %s on %s\n",
				matchCard.HomeTeam, matchCard.AwayTeam, matchCard.EventDate.Format("2006-01-02"))
		}
		return result, nil
	}

	// Check if this is a derby match (both teams are St. Ann's)
	isDerby, homeTeamID, awayTeamID, err := s.isDerbyMatch(ctx, fixture.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to determine if derby match: %w", err)
	}

	if config.Verbose {
		if isDerby {
			fmt.Printf("Processing derby match: %s vs %s (fixture %d)\n",
				matchCard.HomeTeam, matchCard.AwayTeam, fixture.ID)
		} else {
			fmt.Printf("Processing regular match: %s vs %s (fixture %d)\n",
				matchCard.HomeTeam, matchCard.AwayTeam, fixture.ID)
		}
	}

	// Clear existing matchups if requested
	if config.ClearExistingMatchups {
		if err := s.clearExistingMatchups(ctx, config, fixture.ID, isDerby, homeTeamID, awayTeamID); err != nil {
			return nil, fmt.Errorf("failed to clear existing matchups: %w", err)
		}
	}

	// Update fixture with external match card ID and mark as completed
	if !config.DryRun {
		fixture.ExternalMatchCardID = &matchCard.ExternalID

		// Mark fixture as completed since we have a match card (final results)
		fixture.Status = models.Completed

		// Set completion date to the played date from match card, or event date if not available
		completionDate := matchCard.PlayedDate
		if completionDate.IsZero() {
			completionDate = matchCard.EventDate
		}
		fixture.CompletedDate = &completionDate

		if err := s.fixtureRepo.Update(ctx, fixture); err != nil {
			return nil, fmt.Errorf("failed to update fixture: %w", err)
		}
	}
	result.UpdatedFixtures++

	if config.Verbose {
		fmt.Printf("Marked fixture %d as Completed (match card data is authoritative)\n", fixture.ID)
	}

	// Process matchups from the match card
	for _, matchupData := range matchCard.Matchups {
		if isDerby {
			// For derby matches, process matchups for both teams
			// Process matchup for home team
			homeResult, err := s.processMatchupForTeam(ctx, config, fixture.ID, matchupData, homeTeamID, "home")
			if err != nil {
				errMsg := fmt.Sprintf("Error processing %s matchup for home team: %v", matchupData.Type, err)
				result.Errors = append(result.Errors, errMsg)
				if config.Verbose {
					fmt.Printf("  %s\n", errMsg)
				}
			} else {
				// Aggregate home team results
				result.CreatedMatchups += homeResult.CreatedMatchups
				result.UpdatedMatchups += homeResult.UpdatedMatchups
				result.MatchedPlayers += homeResult.MatchedPlayers
				result.UnmatchedPlayers = append(result.UnmatchedPlayers, homeResult.UnmatchedPlayers...)
				result.Errors = append(result.Errors, homeResult.Errors...)
			}

			// Process matchup for away team
			awayResult, err := s.processMatchupForTeam(ctx, config, fixture.ID, matchupData, awayTeamID, "away")
			if err != nil {
				errMsg := fmt.Sprintf("Error processing %s matchup for away team: %v", matchupData.Type, err)
				result.Errors = append(result.Errors, errMsg)
				if config.Verbose {
					fmt.Printf("  %s\n", errMsg)
				}
			} else {
				// Aggregate away team results
				result.CreatedMatchups += awayResult.CreatedMatchups
				result.UpdatedMatchups += awayResult.UpdatedMatchups
				result.MatchedPlayers += awayResult.MatchedPlayers
				result.UnmatchedPlayers = append(result.UnmatchedPlayers, awayResult.UnmatchedPlayers...)
				result.Errors = append(result.Errors, awayResult.Errors...)
			}
		} else {
			// For regular matches, process matchup normally
			matchupResult, err := s.processMatchup(ctx, config, fixture.ID, matchupData)
			if err != nil {
				errMsg := fmt.Sprintf("Error processing matchup %s: %v", matchupData.Type, err)
				result.Errors = append(result.Errors, errMsg)
				if config.Verbose {
					fmt.Printf("  %s\n", errMsg)
				}
				continue
			}

			// Aggregate matchup results
			result.CreatedMatchups += matchupResult.CreatedMatchups
			result.UpdatedMatchups += matchupResult.UpdatedMatchups
			result.MatchedPlayers += matchupResult.MatchedPlayers
			result.UnmatchedPlayers = append(result.UnmatchedPlayers, matchupResult.UnmatchedPlayers...)
			result.Errors = append(result.Errors, matchupResult.Errors...)
		}
	}

	if config.Verbose {
		if isDerby {
			fmt.Printf("Processed derby match card %d for fixture %d: %d matchups (processed for both teams)\n",
				matchCard.ExternalID, fixture.ID, len(matchCard.Matchups))
		} else {
			fmt.Printf("Processed match card %d for fixture %d: %d matchups\n",
				matchCard.ExternalID, fixture.ID, len(matchCard.Matchups))
		}
	}

	return result, nil
}

// processMatchup processes a single matchup from the match card
func (s *MatchCardService) processMatchup(ctx context.Context, config ImportConfig, fixtureID uint, matchupData MatchupData) (*ImportResult, error) {
	result := &ImportResult{
		UnmatchedPlayers: []string{},
		Errors:           []string{},
	}

	// Map the parsed matchup type to our enum
	matchupType, err := s.mapMatchupType(matchupData.Type)
	if err != nil {
		return nil, fmt.Errorf("unknown matchup type: %s", matchupData.Type)
	}

	// Determine managing team ID (which St Ann's team this belongs to)
	managingTeamID, err := s.determineManagingTeamID(ctx, fixtureID)
	if err != nil {
		return nil, fmt.Errorf("failed to determine managing team: %w", err)
	}

	// Calculate matchup points based on overall result (win/draw/lose)
	homePoints, awayPoints := s.calculateMatchupPoints(matchupData.HomeScores, matchupData.AwayScores)

	// Check if matchup already exists
	existingMatchup, err := s.matchupRepo.FindByFixtureTypeAndTeam(ctx, fixtureID, matchupType, managingTeamID)
	if err != nil && existingMatchup == nil {
		// Create new matchup if it doesn't exist
		matchup := &models.Matchup{
			FixtureID:      fixtureID,
			Type:           matchupType,
			Status:         models.Finished, // Finished since this data comes from a completed match card
			HomeScore:      homePoints,
			AwayScore:      awayPoints,
			ManagingTeamID: &managingTeamID,
		}

		// Set individual set scores
		s.setIndividualSetScores(matchup, matchupData)

		if !config.DryRun {
			if err := s.matchupRepo.Create(ctx, matchup); err != nil {
				return nil, fmt.Errorf("failed to create matchup: %w", err)
			}
		}
		result.CreatedMatchups++
		existingMatchup = matchup

		if config.Verbose {
			fmt.Printf("  Created %s matchup for fixture %d - marked as Finished\n", matchupType, fixtureID)
		}
	} else if existingMatchup != nil {
		// Update existing matchup with scores
		existingMatchup.HomeScore = homePoints
		existingMatchup.AwayScore = awayPoints

		// Set individual set scores
		s.setIndividualSetScores(existingMatchup, matchupData)

		// Update status to Finished since this data comes from a completed match card
		existingMatchup.Status = models.Finished

		if !config.DryRun {
			if err := s.matchupRepo.Update(ctx, existingMatchup); err != nil {
				return nil, fmt.Errorf("failed to update matchup: %w", err)
			}
		}
		result.UpdatedMatchups++

		if config.Verbose {
			homeSetsWon, awaySetsWon := s.calculateSetsWon(matchupData.HomeScores, matchupData.AwayScores)
			setDetails := s.formatSetScores(matchupData)
			resultStr := s.formatMatchupResult(homePoints, awayPoints)
			fmt.Printf("  Updated %s matchup for fixture %d (sets: %d-%d%s) - %s\n",
				matchupType, fixtureID, homeSetsWon, awaySetsWon, setDetails, resultStr)
		}
	}

	// Process players for this matchup
	playerResult, err := s.processMatchupPlayers(ctx, config, existingMatchup.ID, matchupData)
	if err != nil {
		return nil, fmt.Errorf("failed to process players: %w", err)
	}

	// Aggregate player results
	result.MatchedPlayers += playerResult.MatchedPlayers
	result.UnmatchedPlayers = append(result.UnmatchedPlayers, playerResult.UnmatchedPlayers...)
	result.Errors = append(result.Errors, playerResult.Errors...)

	return result, nil
}

// calculateMatchupPoints determines matchup points based on overall result (win/draw/lose)
func (s *MatchCardService) calculateMatchupPoints(homeScores, awayScores []int) (int, int) {
	// First calculate sets won using tennis rules
	homeSetsWon, awaySetsWon := s.calculateSetsWon(homeScores, awayScores)

	// Award points based on overall matchup result
	// Win = 2 points, Draw = 1 point each, Lose = 0 points
	if homeSetsWon > awaySetsWon {
		// Home team wins
		return 2, 0
	} else if awaySetsWon > homeSetsWon {
		// Away team wins
		return 0, 2
	} else {
		// Draw (equal sets won)
		return 1, 1
	}
}

// calculateSetsWon determines sets won for each team based on tennis scoring rules
func (s *MatchCardService) calculateSetsWon(homeScores, awayScores []int) (int, int) {
	homeSetsWon := 0
	awaySetsWon := 0

	// Compare scores set by set
	maxSets := len(homeScores)
	if len(awayScores) > maxSets {
		maxSets = len(awayScores)
	}

	for i := 0; i < maxSets; i++ {
		var homeScore, awayScore int

		// Get scores for this set (default to 0 if not available)
		if i < len(homeScores) {
			homeScore = homeScores[i]
		}
		if i < len(awayScores) {
			awayScore = awayScores[i]
		}

		// Skip if both scores are 0 (no data for this set)
		if homeScore == 0 && awayScore == 0 {
			continue
		}

		// Apply tennis scoring rules to determine set winner
		if s.isSetWinner(homeScore, awayScore) {
			homeSetsWon++
		} else if s.isSetWinner(awayScore, homeScore) {
			awaySetsWon++
		}
		// If neither wins by tennis rules, don't award the set to anyone
	}

	return homeSetsWon, awaySetsWon
}

// isSetWinner determines if the first score beats the second score in a tennis set
func (s *MatchCardService) isSetWinner(score1, score2 int) bool {
	// Standard tennis set rules:
	// - Must win at least 6 games
	// - Must win by at least 2 games, OR
	// - Win 7-6 (tiebreak scenario), OR
	// - Win 7-5 (no tiebreak needed)

	if score1 >= 6 {
		// Win by 2+ games
		if score1-score2 >= 2 {
			return true
		}
		// Tiebreak scenario (7-6)
		if score1 == 7 && score2 == 6 {
			return true
		}
	}

	return false
}

// setIndividualSetScores populates the individual set score fields in a matchup
func (s *MatchCardService) setIndividualSetScores(matchup *models.Matchup, matchupData MatchupData) {
	// Helper function to convert int to *int
	intPtr := func(v int) *int {
		return &v
	}

	// Set home team set scores
	if len(matchupData.HomeScores) > 0 {
		matchup.HomeSet1 = intPtr(matchupData.HomeScores[0])
	}
	if len(matchupData.HomeScores) > 1 {
		matchup.HomeSet2 = intPtr(matchupData.HomeScores[1])
	}
	if len(matchupData.HomeScores) > 2 {
		matchup.HomeSet3 = intPtr(matchupData.HomeScores[2])
	}

	// Set away team set scores
	if len(matchupData.AwayScores) > 0 {
		matchup.AwaySet1 = intPtr(matchupData.AwayScores[0])
	}
	if len(matchupData.AwayScores) > 1 {
		matchup.AwaySet2 = intPtr(matchupData.AwayScores[1])
	}
	if len(matchupData.AwayScores) > 2 {
		matchup.AwaySet3 = intPtr(matchupData.AwayScores[2])
	}
}

// formatSetScores creates a human-readable string of set scores for logging
func (s *MatchCardService) formatSetScores(matchupData MatchupData) string {
	if len(matchupData.HomeScores) == 0 && len(matchupData.AwayScores) == 0 {
		return ""
	}

	var setStrings []string
	maxSets := len(matchupData.HomeScores)
	if len(matchupData.AwayScores) > maxSets {
		maxSets = len(matchupData.AwayScores)
	}

	for i := 0; i < maxSets; i++ {
		var homeScore, awayScore int
		if i < len(matchupData.HomeScores) {
			homeScore = matchupData.HomeScores[i]
		}
		if i < len(matchupData.AwayScores) {
			awayScore = matchupData.AwayScores[i]
		}
		setStrings = append(setStrings, fmt.Sprintf("%d-%d", homeScore, awayScore))
	}

	if len(setStrings) > 0 {
		return fmt.Sprintf(", %s", strings.Join(setStrings, " "))
	}
	return ""
}

// processMatchupPlayers processes players for a matchup
func (s *MatchCardService) processMatchupPlayers(ctx context.Context, config ImportConfig, matchupID uint, matchupData MatchupData) (*ImportResult, error) {
	result := &ImportResult{
		UnmatchedPlayers: []string{},
		Errors:           []string{},
	}

	// Clear existing players if not in dry run mode
	// Match card data is authoritative - it represents who actually played
	if !config.DryRun {
		if err := s.matchupRepo.ClearPlayers(ctx, matchupID); err != nil {
			return nil, fmt.Errorf("failed to clear existing players: %w", err)
		}
		if config.Verbose {
			fmt.Printf("    Cleared existing players (match card data is authoritative)\n")
		}
	}

	// Process home players
	for _, playerName := range matchupData.HomePlayers {
		if strings.TrimSpace(playerName) == "" {
			continue
		}

		playerID, err := s.matcher.MatchPlayer(ctx, playerName)
		if err != nil {
			result.UnmatchedPlayers = append(result.UnmatchedPlayers, fmt.Sprintf("%s (home)", playerName))
			if config.Verbose {
				fmt.Printf("    Could not match home player: %s\n", playerName)
			}
			continue
		}

		if !config.DryRun {
			if err := s.matchupRepo.AddPlayer(ctx, matchupID, playerID, true); err != nil {
				errMsg := fmt.Sprintf("Failed to add home player %s: %v", playerName, err)
				result.Errors = append(result.Errors, errMsg)
				continue
			}
		}

		result.MatchedPlayers++
		if config.Verbose {
			fmt.Printf("    Matched home player: %s -> %s\n", playerName, playerID)
		}
	}

	// Process away players
	for _, playerName := range matchupData.AwayPlayers {
		if strings.TrimSpace(playerName) == "" {
			continue
		}

		playerID, err := s.matcher.MatchPlayer(ctx, playerName)
		if err != nil {
			result.UnmatchedPlayers = append(result.UnmatchedPlayers, fmt.Sprintf("%s (away)", playerName))
			if config.Verbose {
				fmt.Printf("    Could not match away player: %s\n", playerName)
			}
			continue
		}

		if !config.DryRun {
			if err := s.matchupRepo.AddPlayer(ctx, matchupID, playerID, false); err != nil {
				errMsg := fmt.Sprintf("Failed to add away player %s: %v", playerName, err)
				result.Errors = append(result.Errors, errMsg)
				continue
			}
		}

		result.MatchedPlayers++
		if config.Verbose {
			fmt.Printf("    Matched away player: %s -> %s\n", playerName, playerID)
		}
	}

	return result, nil
}

// mapMatchupType maps parsed matchup type strings to our enum values
func (s *MatchCardService) mapMatchupType(parsedType string) (models.MatchupType, error) {
	// Normalize the type string
	normalized := strings.ToLower(strings.TrimSpace(parsedType))

	switch {
	case strings.Contains(normalized, "first mixed") || strings.Contains(normalized, "1st mixed"):
		return models.FirstMixed, nil
	case strings.Contains(normalized, "second mixed") || strings.Contains(normalized, "2nd mixed"):
		return models.SecondMixed, nil
	case strings.Contains(normalized, "men"):
		return models.Mens, nil
	case strings.Contains(normalized, "ladies") || strings.Contains(normalized, "women"):
		return models.Womens, nil
	default:
		return "", fmt.Errorf("unknown matchup type: %s", parsedType)
	}
}

// determineManagingTeamID determines which team should manage matchups for this fixture
func (s *MatchCardService) determineManagingTeamID(ctx context.Context, fixtureID uint) (uint, error) {
	// Get the fixture to determine the teams
	fixture, err := s.fixtureRepo.FindByID(ctx, fixtureID)
	if err != nil {
		return 0, err
	}

	// Find the St Ann's club ID
	stAnnsClubs, err := s.clubRepo.FindByNameLike(ctx, "St Ann")
	if err != nil {
		return 0, err
	}
	if len(stAnnsClubs) == 0 {
		return 0, fmt.Errorf("St Ann's club not found")
	}
	stAnnsClubID := stAnnsClubs[0].ID

	// Get home and away teams
	homeTeam, err := s.teamRepo.FindByID(ctx, fixture.HomeTeamID)
	if err != nil {
		return 0, err
	}

	awayTeam, err := s.teamRepo.FindByID(ctx, fixture.AwayTeamID)
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

// findMatchingFixture finds a fixture in our database that matches the match card
func (s *MatchCardService) findMatchingFixture(ctx context.Context, matchCard MatchCardData) (*models.Fixture, error) {
	// Helper to search within a date window centered on a date and return the closest matching fixture
	tryDateWindow := func(center time.Time, days int) (*models.Fixture, error) {
		startDate := center.AddDate(0, 0, -days)
		endDate := center.AddDate(0, 0, days)

		fixtures, err := s.fixtureRepo.FindByDateRange(ctx, startDate, endDate)
		if err != nil {
			return nil, err
		}

		var bestMatch *models.Fixture
		var bestDelta time.Duration

		for _, fixture := range fixtures {
			if s.fixtureMatchesCard(ctx, fixture, matchCard) {
				// Prefer the fixture whose scheduled date is closest to the center date
				delta := center.Sub(fixture.ScheduledDate)
				if delta < 0 {
					delta = -delta
				}
				if bestMatch == nil || delta < bestDelta {
					f := fixture // capture copy
					bestMatch = &f
					bestDelta = delta
				}
			}
		}

		return bestMatch, nil
	}

	// Prefer matching by played date for rescheduled fixtures
	if !matchCard.PlayedDate.IsZero() {
		if fixture, err := tryDateWindow(matchCard.PlayedDate, 14); err == nil && fixture != nil {
			return fixture, nil
		}
	}

	// Fallback to event date with a wider tolerance
	if !matchCard.EventDate.IsZero() {
		if fixture, err := tryDateWindow(matchCard.EventDate, 14); err == nil && fixture != nil {
			return fixture, nil
		}
		if fixture, err := tryDateWindow(matchCard.EventDate, 30); err == nil && fixture != nil {
			return fixture, nil
		}
	}

	return nil, nil
}

// fixtureMatchesCard checks if a fixture matches a match card
func (s *MatchCardService) fixtureMatchesCard(ctx context.Context, fixture models.Fixture, matchCard MatchCardData) bool {
	// Get home and away teams
	homeTeam, err := s.teamRepo.FindByID(ctx, fixture.HomeTeamID)
	if err != nil {
		return false
	}

	awayTeam, err := s.teamRepo.FindByID(ctx, fixture.AwayTeamID)
	if err != nil {
		return false
	}

	// Normalize team names to handle quote/apostrophe differences from PDF import
	normalizeTeamName := func(name string) string {
		// Replace UTF-8 smart quotes with regular ASCII apostrophe
		name = strings.ReplaceAll(name, string([]byte{226, 128, 152}), "'") // Replace left single quotation mark (U+2018)
		name = strings.ReplaceAll(name, string([]byte{226, 128, 153}), "'") // Replace right single quotation mark (U+2019)
		name = strings.ReplaceAll(name, "`", "'")                           // Replace backtick
		name = strings.ReplaceAll(name, "ʼ", "'")                           // Replace modifier letter apostrophe
		name = strings.ReplaceAll(name, "′", "'")                           // Replace prime symbol
		// Remove periods that might be inconsistent (St. Ann's vs St Ann's)
		name = strings.ReplaceAll(name, ".", "")
		// Normalize whitespace
		name = strings.Join(strings.Fields(name), " ")
		name = strings.ToLower(name)
		return name
	}

	// Match by normalized team names
	normalizedHomeCard := normalizeTeamName(matchCard.HomeTeam)
	normalizedAwayCard := normalizeTeamName(matchCard.AwayTeam)
	normalizedHomeDB := normalizeTeamName(homeTeam.Name)
	normalizedAwayDB := normalizeTeamName(awayTeam.Name)

	homeMatch := normalizedHomeCard == normalizedHomeDB
	awayMatch := normalizedAwayCard == normalizedAwayDB

	return homeMatch && awayMatch
}

// formatMatchupResult creates a human-readable string of matchup result for logging
func (s *MatchCardService) formatMatchupResult(homePoints, awayPoints int) string {
	if homePoints > awayPoints {
		return "Home team wins"
	} else if homePoints < awayPoints {
		return "Away team wins"
	} else {
		return "Draw"
	}
}

// isDerbyMatch checks if a fixture is a derby match (both teams are St. Ann's)
func (s *MatchCardService) isDerbyMatch(ctx context.Context, fixtureID uint) (bool, uint, uint, error) {
	// Get the fixture to determine the teams
	fixture, err := s.fixtureRepo.FindByID(ctx, fixtureID)
	if err != nil {
		return false, 0, 0, err
	}

	// Find the St Ann's club ID
	stAnnsClubs, err := s.clubRepo.FindByNameLike(ctx, "St Ann")
	if err != nil {
		return false, 0, 0, err
	}
	if len(stAnnsClubs) == 0 {
		return false, 0, 0, fmt.Errorf("St Ann's club not found")
	}
	stAnnsClubID := stAnnsClubs[0].ID

	// Get home and away teams
	homeTeam, err := s.teamRepo.FindByID(ctx, fixture.HomeTeamID)
	if err != nil {
		return false, 0, 0, err
	}

	awayTeam, err := s.teamRepo.FindByID(ctx, fixture.AwayTeamID)
	if err != nil {
		return false, 0, 0, err
	}

	// Check if both teams are St Ann's
	isHomeStAnns := homeTeam.ClubID == stAnnsClubID
	isAwayStAnns := awayTeam.ClubID == stAnnsClubID
	isDerby := isHomeStAnns && isAwayStAnns

	return isDerby, homeTeam.ID, awayTeam.ID, nil
}

// clearExistingMatchups clears existing matchups for a fixture
func (s *MatchCardService) clearExistingMatchups(ctx context.Context, config ImportConfig, fixtureID uint, isDerby bool, homeTeamID, awayTeamID uint) error {
	// Get all existing matchups for this fixture
	existingMatchups, err := s.matchupRepo.FindByFixture(ctx, fixtureID)
	if err != nil {
		return fmt.Errorf("failed to find existing matchups: %w", err)
	}

	if len(existingMatchups) == 0 {
		if config.Verbose {
			fmt.Printf("No existing matchups to clear for fixture %d\n", fixtureID)
		}
		return nil
	}

	if config.Verbose {
		if isDerby {
			fmt.Printf("Clearing %d existing matchups for derby fixture %d\n", len(existingMatchups), fixtureID)
		} else {
			fmt.Printf("Clearing %d existing matchups for fixture %d\n", len(existingMatchups), fixtureID)
		}
	}

	// Delete existing matchups (this will also cascade to matchup players)
	for _, matchup := range existingMatchups {
		if !config.DryRun {
			if err := s.matchupRepo.Delete(ctx, matchup.ID); err != nil {
				return fmt.Errorf("failed to delete matchup %d: %w", matchup.ID, err)
			}
		}
		if config.Verbose {
			fmt.Printf("  Deleted matchup %d (type: %s, managing team: %v)\n",
				matchup.ID, matchup.Type, matchup.ManagingTeamID)
		}
	}

	return nil
}

// processMatchupForTeam processes a matchup for a specific team (used for derby matches)
func (s *MatchCardService) processMatchupForTeam(ctx context.Context, config ImportConfig, fixtureID uint, matchupData MatchupData, managingTeamID uint, teamContext string) (*ImportResult, error) {
	result := &ImportResult{
		UnmatchedPlayers: []string{},
		Errors:           []string{},
	}

	// Map the parsed matchup type to our enum
	matchupType, err := s.mapMatchupType(matchupData.Type)
	if err != nil {
		return nil, fmt.Errorf("unknown matchup type: %s", matchupData.Type)
	}

	// Calculate matchup points based on overall result (win/draw/lose)
	homePoints, awayPoints := s.calculateMatchupPoints(matchupData.HomeScores, matchupData.AwayScores)

	// Create new matchup (since we cleared existing ones)
	matchup := &models.Matchup{
		FixtureID:      fixtureID,
		Type:           matchupType,
		Status:         models.Finished, // Finished since this data comes from a completed match card
		HomeScore:      homePoints,
		AwayScore:      awayPoints,
		ManagingTeamID: &managingTeamID,
	}

	// Set individual set scores
	s.setIndividualSetScores(matchup, matchupData)

	if !config.DryRun {
		if err := s.matchupRepo.Create(ctx, matchup); err != nil {
			return nil, fmt.Errorf("failed to create matchup: %w", err)
		}
	}
	result.CreatedMatchups++

	if config.Verbose {
		homeSetsWon, awaySetsWon := s.calculateSetsWon(matchupData.HomeScores, matchupData.AwayScores)
		setDetails := s.formatSetScores(matchupData)
		resultStr := s.formatMatchupResult(homePoints, awayPoints)
		fmt.Printf("  Created %s matchup for %s team (fixture %d, sets: %d-%d%s) - %s\n",
			matchupType, teamContext, fixtureID, homeSetsWon, awaySetsWon, setDetails, resultStr)
	}

	// Process players for this matchup with team context
	playerResult, err := s.processMatchupPlayersForTeam(ctx, config, matchup.ID, matchupData, teamContext)
	if err != nil {
		return nil, fmt.Errorf("failed to process players: %w", err)
	}

	// Aggregate player results
	result.MatchedPlayers += playerResult.MatchedPlayers
	result.UnmatchedPlayers = append(result.UnmatchedPlayers, playerResult.UnmatchedPlayers...)
	result.Errors = append(result.Errors, playerResult.Errors...)

	return result, nil
}

// processMatchupPlayersForTeam processes players for a matchup with team context (for derby matches)
func (s *MatchCardService) processMatchupPlayersForTeam(ctx context.Context, config ImportConfig, matchupID uint, matchupData MatchupData, teamContext string) (*ImportResult, error) {
	result := &ImportResult{
		UnmatchedPlayers: []string{},
		Errors:           []string{},
	}

	// Clear existing players if not in dry run mode
	if !config.DryRun {
		if err := s.matchupRepo.ClearPlayers(ctx, matchupID); err != nil {
			return nil, fmt.Errorf("failed to clear existing players: %w", err)
		}
		if config.Verbose {
			fmt.Printf("    Cleared existing players (match card data is authoritative)\n")
		}
	}

	// Determine which players to process based on team context
	var playersToProcess []string
	var arePlayersHome bool

	if teamContext == "home" {
		// For home team matchup, process the home players from match card
		playersToProcess = matchupData.HomePlayers
		arePlayersHome = true
		if config.Verbose {
			fmt.Printf("    Processing home players for home team matchup\n")
		}
	} else {
		// For away team matchup, process the away players from match card
		playersToProcess = matchupData.AwayPlayers
		arePlayersHome = false
		if config.Verbose {
			fmt.Printf("    Processing away players for away team matchup\n")
		}
	}

	// Process the relevant players
	for _, playerName := range playersToProcess {
		if strings.TrimSpace(playerName) == "" {
			continue
		}

		playerID, err := s.matcher.MatchPlayer(ctx, playerName)
		if err != nil {
			result.UnmatchedPlayers = append(result.UnmatchedPlayers, fmt.Sprintf("%s (%s)", playerName, teamContext))
			if config.Verbose {
				fmt.Printf("    Could not match %s player: %s\n", teamContext, playerName)
			}
			continue
		}

		if !config.DryRun {
			if err := s.matchupRepo.AddPlayer(ctx, matchupID, playerID, arePlayersHome); err != nil {
				errMsg := fmt.Sprintf("Failed to add %s player %s: %v", teamContext, playerName, err)
				result.Errors = append(result.Errors, errMsg)
				continue
			}
		}

		result.MatchedPlayers++
		if config.Verbose {
			fmt.Printf("    Matched %s player: %s -> %s\n", teamContext, playerName, playerID)
		}
	}

	return result, nil
}
