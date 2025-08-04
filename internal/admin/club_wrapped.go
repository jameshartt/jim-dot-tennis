package admin

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// ClubWrappedHandler handles club-wide season wrapped requests
type ClubWrappedHandler struct {
	service     *Service
	templateDir string
}

// NewClubWrappedHandler creates a new club wrapped handler
func NewClubWrappedHandler(service *Service, templateDir string) *ClubWrappedHandler {
	return &ClubWrappedHandler{
		service:     service,
		templateDir: templateDir,
	}
}

// PlayerSummary represents basic player information
type PlayerSummary struct {
	ID           string
	Name         string
	MatchesCount int
}

// Page 1: Club Overall Stats
type ClubOverallStats struct {
	TotalMatchups     int
	TotalFixtures     int
	ClubWinPercentage float64
	HoursOnCourt      float64
	PlayersUsed       int
	MostActivePlayer  PlayerSummary
	LeagueAverage     float64
	Percentile        int
}

// Page 2: Club Fixture Results (points-based: 0-8 scale)
type ClubFixtureBreakdown struct {
	Perfect_8_0       int // 8-0: Perfect Victory
	NearPerfect_7_1   int // 7-1: Near Perfect Victory
	Strong_6_2        int // 6-2: Strong Victory
	Close_5_3         int // 5-3: Close Victory
	Draw_4_4          int // 4-4: Perfect Draw
	CloseDefeat_3_5   int // 3-5: Close Defeat
	Heavy_2_6         int // 2-6: Heavy Defeat
	NearWhitewash_1_7 int // 1-7: Near Whitewash
	Whitewash_0_8     int // 0-8: Complete Whitewash
	TotalFixtures     int
}

// Page 3: Playing Style Players
type ClubPlayingStyleStats struct {
	MixedOnlyPlayers  []PlayerSummary // "As God intended 😉"
	MensOnlyPlayers   []PlayerSummary // "Manly Men 💪"
	WomensOnlyPlayers []PlayerSummary // "Fabulous Women ✨"
}

// Page 4-6: Player achievements
type PlayerAchievement struct {
	PlayerSummary
	Value       float64
	Description string
	Rank        int
}

// Page 7: Club Partnerships
type ClubPartnership struct {
	Player1Name     string
	Player2Name     string
	MatchesTogether int
	WinPercentage   float64
	Wins            int
	Draws           int
}

// Page 9: Club Venue Stats
type ClubVenueStats struct {
	VenueName     string
	MatchesPlayed int
	WinPercentage float64
	Wins          int
}

// Page 10: Club Season Highlights
type ClubSeasonHighlights struct {
	BestFixtureResult   string
	LongestMatch        string
	MostUsedPartnership ClubPartnership
	SeasonTimeline      []ClubTimelineEvent
	TotalWeeksActive    int
	BiggestUpset        string
}

type ClubTimelineEvent struct {
	Week        int
	Description string
	Date        string
}

// Main club wrapped data structure - for all players collectively
type ClubWrappedData struct {
	ClubName            string
	SeasonYear          int
	OverallStats        ClubOverallStats
	FixtureBreakdown    ClubFixtureBreakdown
	PlayingStylePlayers ClubPlayingStyleStats
	ThreeSetWarriors    []PlayerAchievement
	GameGrinders        []PlayerAchievement
	UndefeatedLegends   []PlayerAchievement
	TopWinPercentage    []PlayerAchievement // New field for top win percentage players
	BestPartnerships    []ClubPartnership
	LuckyVenue          *ClubVenueStats
	SeasonHighlights    ClubSeasonHighlights
}

// HandleWrapped handles the main club-wide season wrapped
func (h *ClubWrappedHandler) HandleWrapped(w http.ResponseWriter, r *http.Request) {
	log.Printf("Club wrapped handler called with method: %s", r.Method)

	// Get user from context
	user, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Generate wrapped data for all players (club-wide)
	wrappedData, err := h.generateClubWrappedData()
	if err != nil {
		logAndError(w, "Failed to generate wrapped data", err, http.StatusInternalServerError)
		return
	}

	// Render the wrapped pages
	h.renderClubWrapped(w, user, wrappedData)
}

// renderClubWrapped renders the club wrapped pages
func (h *ClubWrappedHandler) renderClubWrapped(w http.ResponseWriter, user interface{}, wrappedData *ClubWrappedData) {
	// Load the club wrapped template
	tmpl, err := parseTemplate(h.templateDir, "admin/wrapped_club.html")
	if err != nil {
		log.Printf("Error parsing club wrapped template: %v", err)
		renderFallbackHTML(w, "Season Wrapped", "St. Ann's Season Wrapped",
			"Season wrapped coming soon", "/admin/dashboard")
		return
	}

	templateData := map[string]interface{}{
		"User":        user,
		"WrappedData": wrappedData,
	}

	if err := renderTemplate(w, tmpl, templateData); err != nil {
		logAndError(w, err.Error(), err, http.StatusInternalServerError)
	}
}

// generateClubWrappedData generates all wrapped statistics for the entire club
func (h *ClubWrappedHandler) generateClubWrappedData() (*ClubWrappedData, error) {
	ctx := context.Background()

	wrappedData := &ClubWrappedData{
		ClubName:   "St. Ann's Tennis Club",
		SeasonYear: 2025, // Could be dynamic
	}

	// Calculate all club statistics
	if err := h.calculateClubOverallStats(ctx, &wrappedData.OverallStats); err != nil {
		log.Printf("Error calculating club overall stats: %v", err)
	}

	if err := h.calculateClubFixtureBreakdown(ctx, &wrappedData.FixtureBreakdown); err != nil {
		log.Printf("Error calculating club fixture breakdown: %v", err)
	}

	if err := h.calculateClubPlayingStylePlayers(ctx, &wrappedData.PlayingStylePlayers); err != nil {
		log.Printf("Error calculating playing style players: %v", err)
	}

	wrappedData.ThreeSetWarriors = h.getClubThreeSetWarriors(ctx)
	wrappedData.GameGrinders = h.getClubGameGrinders(ctx)
	wrappedData.UndefeatedLegends = h.getClubUndefeatedLegends(ctx)
	wrappedData.TopWinPercentage = h.getTopWinPercentagePlayers(ctx) // New method call
	wrappedData.BestPartnerships = h.getClubBestPartnerships(ctx)
	wrappedData.LuckyVenue = h.getClubLuckyVenue(ctx)

	if err := h.calculateClubSeasonHighlights(ctx, &wrappedData.SeasonHighlights); err != nil {
		log.Printf("Error calculating club season highlights: %v", err)
	}

	return wrappedData, nil
}

// Page 1: Calculate club overall statistics (all players combined)
func (h *ClubWrappedHandler) calculateClubOverallStats(ctx context.Context, stats *ClubOverallStats) error {
	// Get total matchups across all teams
	matchupQuery := `
		SELECT COUNT(*)
		FROM matchups m
		INNER JOIN fixtures f ON m.fixture_id = f.id
		WHERE m.status = 'Finished'
	`

	err := h.service.db.QueryRowContext(ctx, matchupQuery).Scan(&stats.TotalMatchups)
	if err != nil {
		return err
	}

	// Get total fixtures
	fixtureQuery := `
		SELECT COUNT(*)
		FROM fixtures f
		WHERE f.status = 'Completed'
	`

	err = h.service.db.QueryRowContext(ctx, fixtureQuery).Scan(&stats.TotalFixtures)
	if err != nil {
		return err
	}

	// Calculate hours on court (45 minutes per matchup)
	stats.HoursOnCourt = float64(stats.TotalMatchups) * 0.75

	// Get number of different players used
	playersQuery := `
		SELECT COUNT(DISTINCT mp.player_id)
		FROM matchup_players mp
		INNER JOIN matchups m ON mp.matchup_id = m.id
		WHERE m.status = 'Finished'
	`

	err = h.service.db.QueryRowContext(ctx, playersQuery).Scan(&stats.PlayersUsed)
	if err != nil {
		stats.PlayersUsed = 0
	}

	// Get most active player across all teams
	activePlayerQuery := `
		SELECT p.id, 
			   COALESCE(p.preferred_name, p.first_name || ' ' || p.last_name) as name,
			   COUNT(*) as matches_count
		FROM matchup_players mp
		INNER JOIN matchups m ON mp.matchup_id = m.id
		INNER JOIN players p ON mp.player_id = p.id
		WHERE m.status = 'Finished'
		GROUP BY p.id, name
		ORDER BY matches_count DESC
		LIMIT 1
	`

	err = h.service.db.QueryRowContext(ctx, activePlayerQuery).Scan(
		&stats.MostActivePlayer.ID, &stats.MostActivePlayer.Name, &stats.MostActivePlayer.MatchesCount)
	if err != nil {
		// No active player found
		stats.MostActivePlayer = PlayerSummary{}
	}

	// Calculate simple club win percentage (placeholder - could be more sophisticated)
	stats.ClubWinPercentage = 65.0 // Placeholder
	stats.LeagueAverage = 50.0     // Placeholder
	stats.Percentile = 78          // Placeholder

	return nil
}

// Page 2: Calculate club fixture breakdown (all fixtures)
func (h *ClubWrappedHandler) calculateClubFixtureBreakdown(ctx context.Context, breakdown *ClubFixtureBreakdown) error {
	// Debug: First let's see what clubs exist
	clubQuery := `SELECT id, name FROM clubs LIMIT 5`
	clubRows, err := h.service.db.QueryContext(ctx, clubQuery)
	if err == nil {
		defer clubRows.Close()
		log.Printf("=== DEBUG: Available clubs ===")
		for clubRows.Next() {
			var id int
			var name string
			clubRows.Scan(&id, &name)
			log.Printf("Club ID: %d, Name: '%s'", id, name)
		}
	}

	// Debug: Check total fixtures
	totalFixturesQuery := `SELECT COUNT(*) FROM fixtures WHERE status = 'Completed'`
	var totalFixtures int
	h.service.db.QueryRowContext(ctx, totalFixturesQuery).Scan(&totalFixtures)
	log.Printf("=== DEBUG: Total completed fixtures: %d ===", totalFixtures)

	// Simplified approach - let's get ALL completed fixtures first
	query := `
		SELECT 
			f.id,
			f.home_team_id,
			f.away_team_id,
			SUM(m.home_score) as home_total,
			SUM(m.away_score) as away_total,
			hc.name as home_club,
			ac.name as away_club
		FROM fixtures f
		INNER JOIN matchups m ON f.id = m.fixture_id
		INNER JOIN teams ht ON f.home_team_id = ht.id
		INNER JOIN teams at ON f.away_team_id = at.id
		INNER JOIN clubs hc ON ht.club_id = hc.id
		INNER JOIN clubs ac ON at.club_id = ac.id
		WHERE f.status = 'Completed' AND m.status = 'Finished'
		GROUP BY f.id, f.home_team_id, f.away_team_id, hc.name, ac.name
	`

	rows, err := h.service.db.QueryContext(ctx, query)
	if err != nil {
		log.Printf("=== DEBUG: Query error: %v ===", err)
		return err
	}
	defer rows.Close()

	breakdown.TotalFixtures = 0
	log.Printf("=== DEBUG: Processing fixtures ===")

	for rows.Next() {
		var fixtureID, homeTeamID, awayTeamID int
		var homeTotal, awayTotal float64
		var homeClub, awayClub string

		err := rows.Scan(&fixtureID, &homeTeamID, &awayTeamID, &homeTotal, &awayTotal, &homeClub, &awayClub)
		if err != nil {
			log.Printf("=== DEBUG: Scan error: %v ===", err)
			continue
		}

		log.Printf("Fixture %d: %s (%.1f) vs %s (%.1f)", fixtureID, homeClub, homeTotal, awayClub, awayTotal)

		// Check if St. Ann's is involved (try different variations)
		isStAnnsHome := strings.Contains(strings.ToLower(homeClub), "st ann") ||
			strings.Contains(strings.ToLower(homeClub), "saint ann")
		isStAnnsAway := strings.Contains(strings.ToLower(awayClub), "st ann") ||
			strings.Contains(strings.ToLower(awayClub), "saint ann")

		if !isStAnnsHome && !isStAnnsAway {
			continue // Not a St. Ann's fixture
		}

		breakdown.TotalFixtures++

		// Determine our score vs their score
		var ourScore, theirScore float64
		if isStAnnsHome {
			ourScore = homeTotal
			theirScore = awayTotal
		} else {
			ourScore = awayTotal
			theirScore = homeTotal
		}

		// Round scores to nearest integer (handles 0.5 values from halved matchups)
		ourRounded := int(ourScore + 0.5)
		theirRounded := int(theirScore + 0.5)

		log.Printf("St Ann's fixture: Our score %.1f (rounded %d) vs Their score %.1f (rounded %d)",
			ourScore, ourRounded, theirScore, theirRounded)

		// Categorize the fixture result based on 0-8 point scale
		if ourRounded == 8 && theirRounded == 0 {
			breakdown.Perfect_8_0++
		} else if ourRounded == 7 && theirRounded == 1 {
			breakdown.NearPerfect_7_1++
		} else if ourRounded == 6 && theirRounded == 2 {
			breakdown.Strong_6_2++
		} else if ourRounded == 5 && theirRounded == 3 {
			breakdown.Close_5_3++
		} else if ourRounded == 4 && theirRounded == 4 {
			breakdown.Draw_4_4++
		} else if ourRounded == 3 && theirRounded == 5 {
			breakdown.CloseDefeat_3_5++
		} else if ourRounded == 2 && theirRounded == 6 {
			breakdown.Heavy_2_6++
		} else if ourRounded == 1 && theirRounded == 7 {
			breakdown.NearWhitewash_1_7++
		} else if ourRounded == 0 && theirRounded == 8 {
			breakdown.Whitewash_0_8++
		}
	}

	log.Printf("=== DEBUG: Final breakdown - Total: %d, 8-0: %d, 7-1: %d, 6-2: %d, 5-3: %d, 4-4: %d, 3-5: %d, 2-6: %d, 1-7: %d, 0-8: %d ===",
		breakdown.TotalFixtures, breakdown.Perfect_8_0, breakdown.NearPerfect_7_1, breakdown.Strong_6_2,
		breakdown.Close_5_3, breakdown.Draw_4_4, breakdown.CloseDefeat_3_5, breakdown.Heavy_2_6,
		breakdown.NearWhitewash_1_7, breakdown.Whitewash_0_8)

	return nil
}

// Page 3: Calculate playing style players for all club players
func (h *ClubWrappedHandler) calculateClubPlayingStylePlayers(ctx context.Context, styles *ClubPlayingStyleStats) error {
	// Get all players who played and their matchup types
	query := `
		SELECT 
			p.id,
			COALESCE(p.preferred_name, p.first_name || ' ' || p.last_name) as name,
			COUNT(*) as matches_count,
			GROUP_CONCAT(DISTINCT m.type) as matchup_types
		FROM matchup_players mp
		INNER JOIN matchups m ON mp.matchup_id = m.id
		INNER JOIN players p ON mp.player_id = p.id
		WHERE m.status = 'Finished'
		GROUP BY p.id, name
		HAVING matches_count >= 3
	`

	rows, err := h.service.db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var playerID, playerName, matchupTypes string
		var matchesCount int

		err := rows.Scan(&playerID, &playerName, &matchesCount, &matchupTypes)
		if err != nil {
			continue
		}

		player := PlayerSummary{
			ID:           playerID,
			Name:         playerName,
			MatchesCount: matchesCount,
		}

		// Determine playing style based on matchup types
		types := strings.Split(matchupTypes, ",")
		hasMixed := false
		hasMens := false
		hasWomens := false

		for _, t := range types {
			switch strings.TrimSpace(t) {
			case "1st Mixed", "2nd Mixed":
				hasMixed = true
			case "Mens":
				hasMens = true
			case "Womens":
				hasWomens = true
			}
		}

		// Categorize exclusive playing styles
		if hasMixed && !hasMens && !hasWomens {
			styles.MixedOnlyPlayers = append(styles.MixedOnlyPlayers, player)
		} else if hasMens && !hasMixed && !hasWomens {
			styles.MensOnlyPlayers = append(styles.MensOnlyPlayers, player)
		} else if hasWomens && !hasMixed && !hasMens {
			styles.WomensOnlyPlayers = append(styles.WomensOnlyPlayers, player)
		}
	}

	return nil
}

// Stub implementations for remaining methods (Pages 4, 5, 6, 7, 9, 10)
func (h *ClubWrappedHandler) getClubThreeSetWarriors(ctx context.Context) []PlayerAchievement {
	// Page 4: Three-Set Warriors - players with highest proportion of 3-set matches
	return []PlayerAchievement{}
}

func (h *ClubWrappedHandler) getClubGameGrinders(ctx context.Context) []PlayerAchievement {
	// Page 5: Game Grinders - players with highest games per set
	return []PlayerAchievement{}
}

func (h *ClubWrappedHandler) getClubUndefeatedLegends(ctx context.Context) []PlayerAchievement {
	// Page 6: Undefeated Legends - players with perfect winning percentages
	return []PlayerAchievement{}
}

func (h *ClubWrappedHandler) getClubBestPartnerships(ctx context.Context) []ClubPartnership {
	// Page 7: Dynamic Duos - player combinations with perfect winning percentages
	return []ClubPartnership{}
}

func (h *ClubWrappedHandler) getClubLuckyVenue(ctx context.Context) *ClubVenueStats {
	// Page 9: Lucky Venue - venue where club has best win percentage
	return nil
}

func (h *ClubWrappedHandler) getTopWinPercentagePlayers(ctx context.Context) []PlayerAchievement {
	// Find players with highest win percentage (minimum 9 fixtures played)
	query := `
		WITH player_stats AS (
			SELECT 
				p.id,
				COALESCE(p.preferred_name, p.first_name || ' ' || p.last_name) as name,
				COUNT(DISTINCT m.fixture_id) as fixtures_played,
				COUNT(*) as total_matchups,
				SUM(CASE 
					WHEN (mp.is_home = 1 AND m.home_score > m.away_score) 
						OR (mp.is_home = 0 AND m.away_score > m.home_score)
					THEN 1 
					ELSE 0 
				END) as wins,
				SUM(CASE 
					WHEN m.home_score = m.away_score 
					THEN 0.5 
					ELSE 0 
				END) as draws
			FROM matchup_players mp
			INNER JOIN matchups m ON mp.matchup_id = m.id
			INNER JOIN players p ON mp.player_id = p.id
			INNER JOIN fixtures f ON m.fixture_id = f.id
			WHERE m.status = 'Finished'
			GROUP BY p.id, name
			HAVING fixtures_played >= 9
		)
		SELECT 
			id,
			name,
			fixtures_played,
			ROUND((wins + draws) * 100.0 / total_matchups, 1) as win_percentage
		FROM player_stats
		WHERE total_matchups > 0
		ORDER BY win_percentage DESC, fixtures_played DESC
		LIMIT 10
	`

	rows, err := h.service.db.QueryContext(ctx, query)
	if err != nil {
		log.Printf("Error getting top win percentage players: %v", err)
		return []PlayerAchievement{}
	}
	defer rows.Close()

	var achievements []PlayerAchievement
	currentRank := 1
	var previousWinPercentage *float64

	for rows.Next() {
		var playerID, playerName string
		var fixturesPlayed int
		var winPercentage float64

		err := rows.Scan(&playerID, &playerName, &fixturesPlayed, &winPercentage)
		if err != nil {
			log.Printf("Error scanning win percentage player: %v", err)
			continue
		}

		// Handle ties: if win percentage is different from previous, update rank
		if previousWinPercentage != nil && winPercentage != *previousWinPercentage {
			currentRank = len(achievements) + 1
		}

		achievement := PlayerAchievement{
			PlayerSummary: PlayerSummary{
				ID:           playerID,
				Name:         playerName,
				MatchesCount: fixturesPlayed,
			},
			Value:       winPercentage,
			Description: fmt.Sprintf("%.1f%% win rate", winPercentage),
			Rank:        currentRank,
		}

		achievements = append(achievements, achievement)
		previousWinPercentage = &winPercentage

		// Stop if we have 5 distinct positions or 5 players total
		if len(achievements) >= 5 {
			break
		}
	}

	return achievements
}

func (h *ClubWrappedHandler) calculateClubSeasonHighlights(ctx context.Context, highlights *ClubSeasonHighlights) error {
	// Page 10: Season Highlights - timeline, biggest upset, etc.
	return nil
}
