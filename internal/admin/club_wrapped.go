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
	Losses          int // New field for losses
	Rank            int // New field for ranking
}

// Page 9: Club Venue Stats
type ClubVenueStats struct {
	VenueName     string
	MatchesPlayed int
	WinPercentage float64
	Wins          int
	Draws         int
	Losses        int
	Rank          int // New field for ranking
}

// Page 9: Availability Engagement Stats
type ClubAvailabilityEngagement struct {
	PlayersSetAvailability             int     // Players who set availability at least once
	PlayersWhoPlayed                   int     // Players who played at least one match
	EngagementPercentage               float64 // Percentage of players who played that also set availability
	TotalAvailabilityUpdates           int     // Total number of availability updates
	AvailabilityActivePlayerPercentage float64 // What % of active players used availability
}

// Page 10: Comeback Kings/Queens - Players who won after losing first set
type ComebackAchievement struct {
	PlayerSummary
	ComebackWins       int
	TotalMatches       int
	ComebackPercentage float64
	Description        string
	Rank               int
}

// Page 11: Social Butterflies - Players with most different partners
type SocialButterflyAchievement struct {
	PlayerSummary
	UniquePartners    int
	TotalPartnerships int
	Description       string
	Rank              int
}

// Page 12: Championship Tiebreak Masters
type TiebreakMasterAchievement struct {
	PlayerSummary
	TiebreakMatches int
	TiebreakWins    int
	TiebreakWinRate float64
	Description     string
	Rank            int
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
	ClubName               string
	SeasonYear             int
	OverallStats           ClubOverallStats
	FixtureBreakdown       ClubFixtureBreakdown
	PlayingStylePlayers    ClubPlayingStyleStats
	ThreeSetWarriors       []PlayerAchievement
	GameGrinders           []PlayerAchievement
	TopWinPercentage       []PlayerAchievement // New field for top win percentage players
	TopPairings            []ClubPartnership   // New field for top pairings
	BestPartnerships       []ClubPartnership
	BestAwayVenues         []ClubVenueStats             // New field for best away venues
	AvailabilityEngagement ClubAvailabilityEngagement   // New field for availability engagement
	ComebackKings          []ComebackAchievement        // New field for comeback achievements
	SocialButterflies      []SocialButterflyAchievement // New field for social butterflies
	TiebreakMasters        []TiebreakMasterAchievement  // New field for tiebreak masters
	LuckyVenue             *ClubVenueStats
	SeasonHighlights       ClubSeasonHighlights
	// Optional per-player section when accessed via player availability
	Personal *PersonalWrappedData
}

// PersonalWrappedData: per-player season summary
type PersonalWrappedData struct {
	PlayerID                   string
	PlayerName                 string
	FixturesPlayed             int
	MatchesPlayed              int
	WinPercentage              float64
	UniquePartners             int
	ThreeSetMatches            int
	TiebreakMatches            int
	MostPlayedDivision         string
	Divisions                  []DivisionBreakdown
	BestPartnerName            string
	BestPartnerMatchesTogether int
	BestPartnerWinPercentage   float64
	// Additional requested stats
	MostCommonMatchupType      string
	MostCommonMatchupCount     int
	LongestWinStreak           int
	LongestLosingStreak        int
	DecidingSetMatches         int
	DecidingSetWins            int
	DecidingSetWinRate         float64
	BagelsDelivered            int
	MostFrequentPartnerName    string
	MostFrequentPartnerMatches int
	AverageGamesPerSet         float64
	ComebackWins               int
}

type DivisionBreakdown struct {
	Division      string
	Fixtures      int
	WinPercentage float64
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

// HandlePublicWrapped renders the club wrapped for non-admins using a simple password gate.
// Access model:
// - POST to /my-availability/{token}/wrapped-auth with correct password sets a short-lived cookie
// - GET /club/wrapped checks cookie and renders wrapped if present
func (h *ClubWrappedHandler) HandlePublicWrapped(w http.ResponseWriter, r *http.Request) {
	// Allow only GET
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check simple access cookie
	const cookieName = "wrapped_access"
	cookie, err := r.Cookie(cookieName)
	if err != nil || cookie.Value != "granted" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Generate wrapped data (same as admin)
	wrappedData, genErr := h.generateClubWrappedData()
	if genErr != nil {
		logAndError(w, "Failed to generate wrapped data", genErr, http.StatusInternalServerError)
		return
	}

	// If a player context cookie is present, enrich with personal stats
	if playerCookie, perr := r.Cookie("wrapped_player_id"); perr == nil && playerCookie.Value != "" {
		if personal := h.getPersonalWrappedData(r.Context(), playerCookie.Value); personal != nil {
			wrappedData.Personal = personal
		}
	}

	// Render with a minimal user context label for template (no admin user)
	h.renderClubWrapped(w, map[string]interface{}{"Username": "guest"}, wrappedData)
}

// getPersonalWrappedData builds a per-player season summary
func (h *ClubWrappedHandler) getPersonalWrappedData(ctx context.Context, playerID string) *PersonalWrappedData {
	pd := &PersonalWrappedData{PlayerID: playerID}

	// Player display name
	_ = h.service.db.QueryRowContext(ctx, `
        SELECT COALESCE(p.preferred_name, p.first_name || ' ' || p.last_name) as name
        FROM players p WHERE p.id = ?
    `, playerID).Scan(&pd.PlayerName)

	// Fixtures played, matches played, wins/draws for win %
	_ = h.service.db.QueryRowContext(ctx, `
        WITH player_matchups AS (
            SELECT m.*, mp.is_home
            FROM matchup_players mp
            INNER JOIN matchups m ON mp.matchup_id = m.id
            WHERE mp.player_id = ? AND m.status = 'Finished'
        )
        SELECT 
            COUNT(DISTINCT fixture_id) as fixtures_played,
            COUNT(*) as matches_played,
            ROUND((
                SUM(CASE WHEN (is_home = 1 AND home_score > away_score) OR (is_home = 0 AND away_score > home_score) THEN 1 ELSE 0 END)
                + SUM(CASE WHEN home_score = away_score THEN 0.5 ELSE 0 END)
            ) * 100.0 / COUNT(*), 1) as win_pct
        FROM player_matchups
    `, playerID).Scan(&pd.FixturesPlayed, &pd.MatchesPlayed, &pd.WinPercentage)

	// Unique partners
	_ = h.service.db.QueryRowContext(ctx, `
        SELECT COUNT(DISTINCT mp2.player_id)
        FROM matchup_players mp1
        JOIN matchup_players mp2 ON mp1.matchup_id = mp2.matchup_id AND mp1.is_home = mp2.is_home AND mp1.player_id <> mp2.player_id
        JOIN matchups m ON mp1.matchup_id = m.id
        WHERE mp1.player_id = ? AND m.status = 'Finished'
    `, playerID).Scan(&pd.UniquePartners)

	// Three-set and tiebreak matches
	_ = h.service.db.QueryRowContext(ctx, `
        SELECT 
            SUM(CASE WHEN (m.home_set3 IS NOT NULL OR m.away_set3 IS NOT NULL) THEN 1 ELSE 0 END) AS three_sets,
            SUM(CASE WHEN (m.home_set3 >= 10 OR m.away_set3 >= 10) THEN 1 ELSE 0 END) AS tiebreaks
        FROM matchup_players mp
        JOIN matchups m ON mp.matchup_id = m.id
        WHERE mp.player_id = ? AND m.status = 'Finished'
    `, playerID).Scan(&pd.ThreeSetMatches, &pd.TiebreakMatches)

	// Division breakdown and most played division
	rows, err := h.service.db.QueryContext(ctx, `
        WITH pm AS (
            SELECT m.fixture_id, mp.is_home, m.home_score, m.away_score
            FROM matchup_players mp
            JOIN matchups m ON mp.matchup_id = m.id
            WHERE mp.player_id = ? AND m.status = 'Finished'
        )
        SELECT d.name as division,
               COUNT(DISTINCT f.id) as fixtures,
               ROUND((
                    SUM(CASE WHEN ((pm.is_home = 1 AND pm.home_score > pm.away_score) OR (pm.is_home = 0 AND pm.away_score > pm.home_score)) THEN 1 ELSE 0 END)
                    + SUM(CASE WHEN pm.home_score = pm.away_score THEN 0.5 ELSE 0 END)
               ) * 100.0 / COUNT(*), 1) as win_pct
        FROM pm
        JOIN fixtures f ON f.id = pm.fixture_id
        JOIN divisions d ON d.id = f.division_id
        GROUP BY d.name
        ORDER BY fixtures DESC, d.name ASC
    `, playerID)
	if err == nil {
		defer rows.Close()
		var most string
		var mostCount int
		for rows.Next() {
			var db DivisionBreakdown
			if err := rows.Scan(&db.Division, &db.Fixtures, &db.WinPercentage); err == nil {
				pd.Divisions = append(pd.Divisions, db)
				if db.Fixtures > mostCount {
					mostCount = db.Fixtures
					most = db.Division
				}
			}
		}
		pd.MostPlayedDivision = most
	}

	// Best partner by win percentage (min 2 matches together)
	_ = h.service.db.QueryRowContext(ctx, `
        WITH my_pairs AS (
            SELECT mp2.player_id as partner_id,
                   COALESCE(p2.preferred_name, p2.first_name || ' ' || p2.last_name) as partner_name,
                   COUNT(*) as matches_together,
                   SUM(CASE WHEN (mp1.is_home = 1 AND m.home_score > m.away_score) OR (mp1.is_home = 0 AND m.away_score > m.home_score) THEN 1 ELSE 0 END) as wins,
                   SUM(CASE WHEN m.home_score = m.away_score THEN 1 ELSE 0 END) as draws
            FROM matchup_players mp1
            JOIN matchup_players mp2 ON mp1.matchup_id = mp2.matchup_id AND mp1.is_home = mp2.is_home AND mp1.player_id <> mp2.player_id
            JOIN matchups m ON mp1.matchup_id = m.id
            JOIN players p2 ON p2.id = mp2.player_id
            WHERE mp1.player_id = ? AND m.status = 'Finished'
            GROUP BY mp2.player_id, partner_name
            HAVING COUNT(*) >= 2
        )
        SELECT partner_name,
               matches_together,
               ROUND((wins + draws) * 100.0 / matches_together, 1) as pct
        FROM my_pairs
        ORDER BY pct DESC, matches_together DESC, partner_name ASC
        LIMIT 1
    `, playerID).Scan(&pd.BestPartnerName, &pd.BestPartnerMatchesTogether, &pd.BestPartnerWinPercentage)

	// Most frequent partner (by matches together)
	_ = h.service.db.QueryRowContext(ctx, `
        WITH my_pairs AS (
            SELECT mp2.player_id as partner_id,
                   COALESCE(p2.preferred_name, p2.first_name || ' ' || p2.last_name) as partner_name,
                   COUNT(*) as matches_together
            FROM matchup_players mp1
            JOIN matchup_players mp2 ON mp1.matchup_id = mp2.matchup_id AND mp1.is_home = mp2.is_home AND mp1.player_id <> mp2.player_id
            JOIN matchups m ON mp1.matchup_id = m.id
            JOIN players p2 ON p2.id = mp2.player_id
            WHERE mp1.player_id = ? AND m.status = 'Finished'
            GROUP BY mp2.player_id, partner_name
        )
        SELECT partner_name, matches_together
        FROM my_pairs
        ORDER BY matches_together DESC, partner_name ASC
        LIMIT 1
    `, playerID).Scan(&pd.MostFrequentPartnerName, &pd.MostFrequentPartnerMatches)

	// Most common matchup type
	_ = h.service.db.QueryRowContext(ctx, `
        SELECT m.type as matchup_type, COUNT(*) as cnt
        FROM matchup_players mp
        JOIN matchups m ON mp.matchup_id = m.id
        WHERE mp.player_id = ? AND m.status = 'Finished'
        GROUP BY m.type
        ORDER BY cnt DESC, matchup_type ASC
        LIMIT 1
    `, playerID).Scan(&pd.MostCommonMatchupType, &pd.MostCommonMatchupCount)

	// Deciding set performance
	_ = h.service.db.QueryRowContext(ctx, `
        WITH pm AS (
            SELECT m.home_set3, m.away_set3, mp.is_home, m.home_score, m.away_score
            FROM matchup_players mp
            JOIN matchups m ON mp.matchup_id = m.id
            WHERE mp.player_id = ? AND m.status = 'Finished'
        )
        SELECT 
            SUM(CASE WHEN (home_set3 IS NOT NULL OR away_set3 IS NOT NULL) THEN 1 ELSE 0 END) as deciding_matches,
            SUM(CASE WHEN (home_set3 IS NOT NULL OR away_set3 IS NOT NULL) AND ((is_home = 1 AND home_score > away_score) OR (is_home = 0 AND away_score > home_score)) THEN 1 ELSE 0 END) as deciding_wins
        FROM pm
    `, playerID).Scan(&pd.DecidingSetMatches, &pd.DecidingSetWins)
	if pd.DecidingSetMatches > 0 {
		pd.DecidingSetWinRate = float64(pd.DecidingSetWins) * 100.0 / float64(pd.DecidingSetMatches)
	}

	// Bagels delivered (6-0 sets won)
	_ = h.service.db.QueryRowContext(ctx, `
        SELECT (
            SUM(CASE WHEN is_home = 1 AND home_set1 = 6 AND away_set1 = 0 THEN 1 ELSE 0 END) +
            SUM(CASE WHEN is_home = 1 AND home_set2 = 6 AND away_set2 = 0 THEN 1 ELSE 0 END) +
            SUM(CASE WHEN is_home = 1 AND home_set3 = 6 AND away_set3 = 0 THEN 1 ELSE 0 END) +
            SUM(CASE WHEN is_home = 0 AND away_set1 = 6 AND home_set1 = 0 THEN 1 ELSE 0 END) +
            SUM(CASE WHEN is_home = 0 AND away_set2 = 6 AND home_set2 = 0 THEN 1 ELSE 0 END) +
            SUM(CASE WHEN is_home = 0 AND away_set3 = 6 AND home_set3 = 0 THEN 1 ELSE 0 END)
        ) as bagels
        FROM matchup_players mp
        JOIN matchups m ON mp.matchup_id = m.id
        WHERE mp.player_id = ? AND m.status = 'Finished'
    `, playerID).Scan(&pd.BagelsDelivered)

	// Average games per set
	_ = h.service.db.QueryRowContext(ctx, `
        WITH s AS (
            SELECT 
                (CASE WHEN m.home_set1 IS NOT NULL AND m.away_set1 IS NOT NULL THEN (m.home_set1 + m.away_set1) ELSE 0 END) +
                (CASE WHEN m.home_set2 IS NOT NULL AND m.away_set2 IS NOT NULL THEN (m.home_set2 + m.away_set2) ELSE 0 END) +
                (CASE WHEN m.home_set3 IS NOT NULL AND m.away_set3 IS NOT NULL AND (m.home_set3 + m.away_set3) < 10 THEN (m.home_set3 + m.away_set3) ELSE 0 END) AS games,
                (CASE WHEN m.home_set1 IS NOT NULL AND m.away_set1 IS NOT NULL THEN 1 ELSE 0 END) +
                (CASE WHEN m.home_set2 IS NOT NULL AND m.away_set2 IS NOT NULL THEN 1 ELSE 0 END) +
                (CASE WHEN m.home_set3 IS NOT NULL AND m.away_set3 IS NOT NULL AND (m.home_set3 + m.away_set3) < 10 THEN 1 ELSE 0 END) AS sets
            FROM matchup_players mp
            JOIN matchups m ON mp.matchup_id = m.id
            WHERE mp.player_id = ? AND m.status = 'Finished'
        )
        SELECT ROUND(CASE WHEN SUM(sets) > 0 THEN CAST(SUM(games) AS FLOAT) / SUM(sets) ELSE 0 END, 2)
        FROM s
    `, playerID).Scan(&pd.AverageGamesPerSet)

	// Comeback wins
	_ = h.service.db.QueryRowContext(ctx, `
        SELECT SUM(CASE
            WHEN ((mp.is_home = 1 AND m.home_set1 < m.away_set1) OR (mp.is_home = 0 AND m.away_set1 < m.home_set1))
                 AND ((mp.is_home = 1 AND m.home_score > m.away_score) OR (mp.is_home = 0 AND m.away_score > m.home_score))
            THEN 1 ELSE 0 END)
        FROM matchup_players mp
        JOIN matchups m ON mp.matchup_id = m.id
        WHERE mp.player_id = ? AND m.status = 'Finished' AND m.home_set1 IS NOT NULL AND m.away_set1 IS NOT NULL
    `, playerID).Scan(&pd.ComebackWins)

	// Streaks
	pd.LongestWinStreak, pd.LongestLosingStreak = h.computePlayerStreaks(ctx, playerID)

	return pd
}

// computePlayerStreaks computes longest win and losing streak for a player
func (h *ClubWrappedHandler) computePlayerStreaks(ctx context.Context, playerID string) (int, int) {
	rows, err := h.service.db.QueryContext(ctx, `
        SELECT CASE 
            WHEN (mp.is_home = 1 AND m.home_score > m.away_score) OR (mp.is_home = 0 AND m.away_score > m.home_score) THEN 1
            WHEN m.home_score = m.away_score THEN 0
            ELSE -1
        END as result
        FROM matchup_players mp
        JOIN matchups m ON mp.matchup_id = m.id
        JOIN fixtures f ON f.id = m.fixture_id
        WHERE mp.player_id = ? AND m.status = 'Finished'
        ORDER BY f.scheduled_date ASC, m.id ASC
    `, playerID)
	if err != nil {
		return 0, 0
	}
	defer rows.Close()
	maxW, curW, maxL, curL := 0, 0, 0, 0
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			continue
		}
		switch v {
		case 1:
			curW++
			if curW > maxW {
				maxW = curW
			}
			curL = 0
		case -1:
			curL++
			if curL > maxL {
				maxL = curL
			}
			curW = 0
		default:
			curW = 0
			curL = 0
		}
	}
	return maxW, maxL
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
	wrappedData.TopWinPercentage = h.getTopWinPercentagePlayers(ctx)          // New method call
	wrappedData.TopPairings = h.getTopPairings(ctx)                           // New method call for top pairings
	wrappedData.BestAwayVenues = h.getClubBestAwayVenues(ctx)                 // New method call
	wrappedData.AvailabilityEngagement = h.getClubAvailabilityEngagement(ctx) // New method call for availability engagement
	wrappedData.ComebackKings = h.getComebackKings(ctx)                       // New method call for comeback achievements
	wrappedData.SocialButterflies = h.getSocialButterflies(ctx)               // New method call for social butterflies
	wrappedData.TiebreakMasters = h.getTiebreakMasters(ctx)                   // New method call for tiebreak masters
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
	query := `
		WITH player_set_stats AS (
			SELECT 
				p.id,
				COALESCE(p.preferred_name, p.first_name || ' ' || p.last_name) as name,
				COUNT(*) as total_matches,
				SUM(CASE 
					WHEN m.home_set3 IS NOT NULL OR m.away_set3 IS NOT NULL 
					THEN 1 
					ELSE 0 
				END) as three_set_matches,
				ROUND(
					SUM(CASE 
						WHEN m.home_set3 IS NOT NULL OR m.away_set3 IS NOT NULL 
						THEN 1 
						ELSE 0 
					END) * 100.0 / COUNT(*), 1
				) as three_set_percentage
			FROM matchup_players mp
			INNER JOIN matchups m ON mp.matchup_id = m.id
			INNER JOIN players p ON mp.player_id = p.id
			WHERE m.status = 'Finished'
			GROUP BY p.id, name
			HAVING total_matches >= 5
		)
		SELECT 
			id,
			name,
			total_matches,
			three_set_matches,
			three_set_percentage
		FROM player_set_stats
		WHERE three_set_percentage > 0
		ORDER BY three_set_percentage DESC, three_set_matches DESC
		LIMIT 10
	`

	rows, err := h.service.db.QueryContext(ctx, query)
	if err != nil {
		log.Printf("Error getting three set warriors: %v", err)
		return []PlayerAchievement{}
	}
	defer rows.Close()

	var achievements []PlayerAchievement
	currentRank := 1
	var previousPercentage *float64

	for rows.Next() {
		var playerID, playerName string
		var totalMatches, threeSetMatches int
		var threeSetPercentage float64

		err := rows.Scan(&playerID, &playerName, &totalMatches, &threeSetMatches, &threeSetPercentage)
		if err != nil {
			log.Printf("Error scanning three set warrior: %v", err)
			continue
		}

		// Dense ranking: same percentage => same rank, next distinct => rank+1
		if previousPercentage != nil && threeSetPercentage != *previousPercentage {
			currentRank++
		}

		achievement := PlayerAchievement{
			PlayerSummary: PlayerSummary{
				ID:           playerID,
				Name:         playerName,
				MatchesCount: totalMatches,
			},
			Value:       threeSetPercentage,
			Description: fmt.Sprintf("%.1f%% three-set matches (%d/%d)", threeSetPercentage, threeSetMatches, totalMatches),
			Rank:        currentRank,
		}

		achievements = append(achievements, achievement)
		previousPercentage = &threeSetPercentage
	}

	return achievements
}

func (h *ClubWrappedHandler) getClubGameGrinders(ctx context.Context) []PlayerAchievement {
	// Page 5: Game Grinders - players with highest games per set
	query := `
		WITH player_game_stats AS (
			SELECT 
				p.id,
				COALESCE(p.preferred_name, p.first_name || ' ' || p.last_name) as name,
				COUNT(*) as total_matchups,
				-- Count all valid sets (excluding championship tiebreaks in set 3)
				SUM(
					CASE WHEN m.home_set1 IS NOT NULL AND m.away_set1 IS NOT NULL THEN 1 ELSE 0 END +
					CASE WHEN m.home_set2 IS NOT NULL AND m.away_set2 IS NOT NULL THEN 1 ELSE 0 END +
					CASE WHEN m.home_set3 IS NOT NULL AND m.away_set3 IS NOT NULL 
						AND (m.home_set3 + m.away_set3) < 10 THEN 1 ELSE 0 END
				) as total_sets,
				-- Sum all games from valid sets
				SUM(
					COALESCE(m.home_set1, 0) + COALESCE(m.away_set1, 0) +
					COALESCE(m.home_set2, 0) + COALESCE(m.away_set2, 0) +
					CASE WHEN m.home_set3 IS NOT NULL AND m.away_set3 IS NOT NULL 
						AND (m.home_set3 + m.away_set3) < 10 
						THEN m.home_set3 + m.away_set3 
						ELSE 0 END
				) as total_games
			FROM matchup_players mp
			INNER JOIN matchups m ON mp.matchup_id = m.id
			INNER JOIN players p ON mp.player_id = p.id
			WHERE m.status = 'Finished'
				AND (m.home_set1 IS NOT NULL OR m.home_set2 IS NOT NULL OR m.home_set3 IS NOT NULL)
			GROUP BY p.id, name
			HAVING total_matchups >= 5 AND total_sets >= 10
		)
		SELECT 
			id,
			name,
			total_matchups,
			total_sets,
			total_games,
			ROUND(CAST(total_games AS FLOAT) / total_sets, 2) as avg_games_per_set
		FROM player_game_stats
		ORDER BY avg_games_per_set DESC, total_games DESC
		LIMIT 10
	`

	rows, err := h.service.db.QueryContext(ctx, query)
	if err != nil {
		log.Printf("Error getting game grinders: %v", err)
		return []PlayerAchievement{}
	}
	defer rows.Close()

	var achievements []PlayerAchievement
	currentRank := 1
	var previousAverage *float64

	for rows.Next() {
		var playerID, playerName string
		var totalMatchups, totalSets, totalGames int
		var avgGamesPerSet float64

		err := rows.Scan(&playerID, &playerName, &totalMatchups, &totalSets, &totalGames, &avgGamesPerSet)
		if err != nil {
			log.Printf("Error scanning game grinder: %v", err)
			continue
		}

		// Dense ranking: same average => same rank, next distinct => rank+1
		if previousAverage != nil && avgGamesPerSet != *previousAverage {
			currentRank++
		}

		achievement := PlayerAchievement{
			PlayerSummary: PlayerSummary{
				ID:           playerID,
				Name:         playerName,
				MatchesCount: totalMatchups,
			},
			Value:       avgGamesPerSet,
			Description: fmt.Sprintf("%.2f games per set (%d games/%d sets)", avgGamesPerSet, totalGames, totalSets),
			Rank:        currentRank,
		}

		achievements = append(achievements, achievement)
		previousAverage = &avgGamesPerSet
	}

	return achievements
}

func (h *ClubWrappedHandler) getClubBestAwayVenues(ctx context.Context) []ClubVenueStats {
	// Find best away venues (excluding home fixtures and derbies)
	query := `
		WITH fixture_results AS (
			SELECT 
				f.venue_location,
				f.id as fixture_id,
				SUM(m.away_score) as away_total,
				SUM(m.home_score) as home_total,
				CASE 
					WHEN SUM(m.away_score) > SUM(m.home_score) THEN 1
					WHEN SUM(m.away_score) = SUM(m.home_score) THEN 0.5
					ELSE 0
				END as points
			FROM fixtures f
			INNER JOIN matchups m ON f.id = m.fixture_id
			INNER JOIN teams at ON f.away_team_id = at.id
			INNER JOIN teams ht ON f.home_team_id = ht.id
			INNER JOIN clubs ac ON at.club_id = ac.id
			INNER JOIN clubs hc ON ht.club_id = hc.id
			WHERE f.status = 'Completed' 
				AND m.status = 'Finished'
				AND (ac.name LIKE '%St Ann%' OR ac.name LIKE '%Saint Ann%')
				AND NOT (hc.name LIKE '%St Ann%' OR hc.name LIKE '%Saint Ann%')
			GROUP BY f.venue_location, f.id
		)
		SELECT 
			venue_location,
			COUNT(*) as matches_played,
			SUM(CASE WHEN points = 1 THEN 1 ELSE 0 END) as wins,
			SUM(CASE WHEN points = 0.5 THEN 1 ELSE 0 END) as draws,
			SUM(CASE WHEN points = 0 THEN 1 ELSE 0 END) as losses,
			ROUND(SUM(points) * 100.0 / COUNT(*), 1) as win_percentage
		FROM fixture_results
		GROUP BY venue_location
		HAVING matches_played >= 2
		ORDER BY win_percentage DESC, matches_played DESC
		LIMIT 5
	`

	rows, err := h.service.db.QueryContext(ctx, query)
	if err != nil {
		log.Printf("Error getting best away venues: %v", err)
		return []ClubVenueStats{}
	}
	defer rows.Close()

	var venues []ClubVenueStats
	currentRank := 1
	var previousWinPercentage *float64

	for rows.Next() {
		var venueName string
		var matchesPlayed, wins, draws, losses int
		var winPercentage float64

		err := rows.Scan(&venueName, &matchesPlayed, &wins, &draws, &losses, &winPercentage)
		if err != nil {
			log.Printf("Error scanning venue: %v", err)
			continue
		}

		// Handle ties: if win percentage is different from previous, update rank
		if previousWinPercentage != nil && winPercentage != *previousWinPercentage {
			currentRank = len(venues) + 1
		}

		venue := ClubVenueStats{
			VenueName:     venueName,
			MatchesPlayed: matchesPlayed,
			WinPercentage: winPercentage,
			Wins:          wins,
			Draws:         draws,
			Losses:        losses,
			Rank:          currentRank,
		}

		venues = append(venues, venue)
		previousWinPercentage = &winPercentage
	}

	return venues
}

func (h *ClubWrappedHandler) getClubLuckyVenue(ctx context.Context) *ClubVenueStats {
	// Page 9: Lucky Venue - venue where club has best win percentage
	return nil
}

func (h *ClubWrappedHandler) getClubAvailabilityEngagement(ctx context.Context) ClubAvailabilityEngagement {
	// Page 9: Availability Engagement Stats
	engagement := ClubAvailabilityEngagement{}

	// Get players who set availability at least once (from availability exceptions table)
	availabilityQuery := `
		SELECT COUNT(DISTINCT player_id)
		FROM player_availability_exceptions
	`
	err := h.service.db.QueryRowContext(ctx, availabilityQuery).Scan(&engagement.PlayersSetAvailability)
	if err != nil {
		log.Printf("Error getting players who set availability: %v", err)
	}

	// Get players who played at least one match
	playedQuery := `
		SELECT COUNT(DISTINCT mp.player_id)
		FROM matchup_players mp
		INNER JOIN matchups m ON mp.matchup_id = m.id
		WHERE m.status = 'Finished'
	`
	err = h.service.db.QueryRowContext(ctx, playedQuery).Scan(&engagement.PlayersWhoPlayed)
	if err != nil {
		log.Printf("Error getting players who played: %v", err)
	}

	// Calculate engagement percentage (what % of active players used availability system)
	if engagement.PlayersWhoPlayed > 0 {
		engagement.AvailabilityActivePlayerPercentage = float64(engagement.PlayersSetAvailability) / float64(engagement.PlayersWhoPlayed) * 100.0
	}

	// Get total availability updates (total records in exceptions table)
	totalUpdatesQuery := `
		SELECT COUNT(*)
		FROM player_availability_exceptions
	`
	err = h.service.db.QueryRowContext(ctx, totalUpdatesQuery).Scan(&engagement.TotalAvailabilityUpdates)
	if err != nil {
		log.Printf("Error getting total availability updates: %v", err)
	}

	// Calculate what percentage of players who set availability actually played
	if engagement.PlayersSetAvailability > 0 {
		engagement.EngagementPercentage = float64(engagement.PlayersWhoPlayed) / float64(engagement.PlayersSetAvailability) * 100.0
	}

	log.Printf("=== DEBUG: Availability Engagement - PlayersSetAvailability: %d, PlayersWhoPlayed: %d, ActivePlayerPercentage: %.1f%%, TotalUpdates: %d ===",
		engagement.PlayersSetAvailability, engagement.PlayersWhoPlayed, engagement.AvailabilityActivePlayerPercentage, engagement.TotalAvailabilityUpdates)

	return engagement
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

		// Dense ranking: same percentage => same rank, next distinct => rank+1
		if previousWinPercentage != nil && winPercentage != *previousWinPercentage {
			currentRank++
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

func (h *ClubWrappedHandler) getTopPairings(ctx context.Context) []ClubPartnership {
	// Find perfect partnerships with 100% win rate (minimum 2 matches together)
	query := `
		WITH pairing_stats AS (
			SELECT 
				p1.id as player1_id,
				COALESCE(p1.preferred_name, p1.first_name || ' ' || p1.last_name) as player1_name,
				p2.id as player2_id,
				COALESCE(p2.preferred_name, p2.first_name || ' ' || p2.last_name) as player2_name,
				COUNT(*) as matches_together,
				SUM(CASE 
					WHEN (mp1.is_home = 1 AND m.home_score > m.away_score) 
						OR (mp1.is_home = 0 AND m.away_score > m.home_score)
					THEN 1 
					ELSE 0 
				END) as wins,
				SUM(CASE 
					WHEN m.home_score = m.away_score 
					THEN 1 
					ELSE 0 
				END) as draws
			FROM matchup_players mp1
			INNER JOIN matchup_players mp2 ON mp1.matchup_id = mp2.matchup_id 
				AND mp1.is_home = mp2.is_home 
				AND mp1.player_id < mp2.player_id  -- Avoid duplicate pairs and self-pairs
			INNER JOIN matchups m ON mp1.matchup_id = m.id
			INNER JOIN players p1 ON mp1.player_id = p1.id
			INNER JOIN players p2 ON mp2.player_id = p2.id
			WHERE m.status = 'Finished'
			GROUP BY p1.id, player1_name, p2.id, player2_name
			HAVING matches_together >= 2
		)
		SELECT 
			player1_name,
			player2_name,
			matches_together,
			wins,
			draws,
			matches_together - wins - draws as losses,
			ROUND((wins + draws) * 100.0 / matches_together, 1) as win_percentage
		FROM pairing_stats
		WHERE (wins + draws) = matches_together  -- Only perfect partnerships (100% win rate)
		ORDER BY matches_together DESC, player1_name ASC
	`

	rows, err := h.service.db.QueryContext(ctx, query)
	if err != nil {
		log.Printf("Error getting perfect partnerships: %v", err)
		return []ClubPartnership{}
	}
	defer rows.Close()

	var partnerships []ClubPartnership
	currentRank := 1
	var previousMatches *int

	for rows.Next() {
		var player1Name, player2Name string
		var matchesTogether, wins, draws, losses int
		var winPercentage float64

		err := rows.Scan(&player1Name, &player2Name, &matchesTogether, &wins, &draws, &losses, &winPercentage)
		if err != nil {
			log.Printf("Error scanning pairing: %v", err)
			continue
		}

		// Dense ranking: same matchesTogether => same rank, next distinct value => rank+1
		if previousMatches != nil && matchesTogether != *previousMatches {
			currentRank++
		}

		partnership := ClubPartnership{
			Player1Name:     player1Name,
			Player2Name:     player2Name,
			MatchesTogether: matchesTogether,
			WinPercentage:   winPercentage,
			Wins:            wins,
			Draws:           draws,
			Losses:          losses,
			Rank:            currentRank, // Assign the calculated rank
		}

		partnerships = append(partnerships, partnership)
		previousMatches = &matchesTogether
	}

	return partnerships
}

func (h *ClubWrappedHandler) getComebackKings(ctx context.Context) []ComebackAchievement {
	// Page 10: Comeback Kings/Queens - Players who won matches after losing the first set
	query := `
        WITH pm AS (
            SELECT 
                p.id AS player_id,
                COALESCE(p.preferred_name, p.first_name || ' ' || p.last_name) AS name,
                mp.is_home,
                m.home_set1, m.away_set1,
                m.home_set2, m.away_set2,
                m.home_set3, m.away_set3,
                m.home_score, m.away_score
            FROM matchup_players mp
            INNER JOIN matchups m ON mp.matchup_id = m.id
            INNER JOIN players p ON mp.player_id = p.id
            WHERE m.status = 'Finished'
                AND m.home_set1 IS NOT NULL 
                AND m.away_set1 IS NOT NULL
                AND m.home_score IS NOT NULL
                AND m.away_score IS NOT NULL
        ), flags AS (
            SELECT 
                player_id,
                name,
                -- Win after losing the first set (necessarily a 3-set win)
                CASE 
                    WHEN ((is_home = 1 AND home_set1 < away_set1) OR (is_home = 0 AND away_set1 < home_set1))
                     AND ((is_home = 1 AND home_score > away_score) OR (is_home = 0 AND away_score > home_score))
                    THEN 1 ELSE 0 END AS is_comeback_win,
                -- Straight-set loss after losing the first set (0-2)
                CASE 
                    WHEN ((is_home = 1 AND home_set1 < away_set1) OR (is_home = 0 AND away_set1 < home_set1))
                     AND ((is_home = 1 AND home_set2 < away_set2) OR (is_home = 0 AND away_set2 < home_set2))
                     AND (home_set3 IS NULL AND away_set3 IS NULL)
                    THEN 1 ELSE 0 END AS is_two_set_loss
            FROM pm
        )
        SELECT 
            player_id AS id,
            name,
            SUM(is_comeback_win) + SUM(is_two_set_loss) AS total_opportunities,
            SUM(is_comeback_win) AS comeback_wins,
            ROUND(
                CASE WHEN (SUM(is_comeback_win) + SUM(is_two_set_loss)) > 0 
                     THEN SUM(is_comeback_win) * 100.0 / (SUM(is_comeback_win) + SUM(is_two_set_loss))
                     ELSE 0 END, 1
            ) AS comeback_percentage
        FROM flags
        GROUP BY player_id, name
        HAVING total_opportunities >= 2 AND comeback_wins > 0
        ORDER BY comeback_percentage DESC, comeback_wins DESC, total_opportunities DESC
        LIMIT 10
    `

	rows, err := h.service.db.QueryContext(ctx, query)
	if err != nil {
		log.Printf("Error getting comeback kings: %v", err)
		return []ComebackAchievement{}
	}
	defer rows.Close()

	var achievements []ComebackAchievement
	currentRank := 1
	var previousPercentage *float64

	for rows.Next() {
		var playerID, playerName string
		var totalMatches, comebackWins int
		var comebackPercentage float64

		err := rows.Scan(&playerID, &playerName, &totalMatches, &comebackWins, &comebackPercentage)
		if err != nil {
			log.Printf("Error scanning comeback achievement: %v", err)
			continue
		}

		// Dense ranking: same percentage => same rank, next distinct => rank+1
		if previousPercentage != nil && comebackPercentage != *previousPercentage {
			currentRank++
		}

		achievement := ComebackAchievement{
			PlayerSummary: PlayerSummary{
				ID:           playerID,
				Name:         playerName,
				MatchesCount: totalMatches,
			},
			ComebackWins:       comebackWins,
			TotalMatches:       totalMatches,
			ComebackPercentage: comebackPercentage,
			Description:        fmt.Sprintf("%.1f%% comeback rate (%d/%d matches)", comebackPercentage, comebackWins, totalMatches),
			Rank:               currentRank,
		}

		achievements = append(achievements, achievement)
		previousPercentage = &comebackPercentage
	}

	return achievements
}

func (h *ClubWrappedHandler) getSocialButterflies(ctx context.Context) []SocialButterflyAchievement {
	// Page 11: Social Butterflies - Players with most different partners
	query := `
		WITH player_partnerships AS (
			SELECT 
				mp1.player_id,
				COALESCE(p1.preferred_name, p1.first_name || ' ' || p1.last_name) as player_name,
				COUNT(DISTINCT mp2.player_id) as unique_partners,
				COUNT(*) as total_partnerships
			FROM matchup_players mp1
			INNER JOIN matchup_players mp2 ON mp1.matchup_id = mp2.matchup_id 
				AND mp1.is_home = mp2.is_home 
				AND mp1.player_id != mp2.player_id  -- Don't count self
			INNER JOIN matchups m ON mp1.matchup_id = m.id
			INNER JOIN players p1 ON mp1.player_id = p1.id
			WHERE m.status = 'Finished'
			GROUP BY mp1.player_id, player_name
			HAVING total_partnerships >= 5  -- Minimum partnerships to qualify
		)
		SELECT 
			player_id,
			player_name,
			unique_partners,
			total_partnerships
		FROM player_partnerships
		ORDER BY unique_partners DESC, total_partnerships DESC
		LIMIT 10
	`

	rows, err := h.service.db.QueryContext(ctx, query)
	if err != nil {
		log.Printf("Error getting social butterflies: %v", err)
		return []SocialButterflyAchievement{}
	}
	defer rows.Close()

	var achievements []SocialButterflyAchievement
	currentRank := 1
	var previousPartners *int

	for rows.Next() {
		var playerID, playerName string
		var uniquePartners, totalPartnerships int

		err := rows.Scan(&playerID, &playerName, &uniquePartners, &totalPartnerships)
		if err != nil {
			log.Printf("Error scanning social butterfly: %v", err)
			continue
		}

		// Dense ranking: same uniquePartners => same rank, next distinct => rank+1
		if previousPartners != nil && uniquePartners != *previousPartners {
			currentRank++
		}

		achievement := SocialButterflyAchievement{
			PlayerSummary: PlayerSummary{
				ID:           playerID,
				Name:         playerName,
				MatchesCount: totalPartnerships,
			},
			UniquePartners:    uniquePartners,
			TotalPartnerships: totalPartnerships,
			Description:       fmt.Sprintf("%d different partners (%d matches)", uniquePartners, totalPartnerships),
			Rank:              currentRank,
		}

		achievements = append(achievements, achievement)
		previousPartners = &uniquePartners
	}

	return achievements
}

func (h *ClubWrappedHandler) getTiebreakMasters(ctx context.Context) []TiebreakMasterAchievement {
	// Page 12: Championship Tiebreak Masters - Players in matches where final set had values >= 10
	query := `
		WITH tiebreak_stats AS (
			SELECT 
				p.id,
				COALESCE(p.preferred_name, p.first_name || ' ' || p.last_name) as name,
				COUNT(*) as tiebreak_matches,
				SUM(CASE 
					WHEN (mp.is_home = 1 AND m.home_score > m.away_score) 
						OR (mp.is_home = 0 AND m.away_score > m.home_score)
					THEN 1 
					ELSE 0 
				END) as tiebreak_wins
			FROM matchup_players mp
			INNER JOIN matchups m ON mp.matchup_id = m.id
			INNER JOIN players p ON mp.player_id = p.id
			WHERE m.status = 'Finished'
				AND (
					(m.home_set3 >= 10 OR m.away_set3 >= 10)  -- Championship tiebreak in set 3
				)
			GROUP BY p.id, name
			HAVING tiebreak_matches >= 2  -- Minimum tiebreak matches to qualify
		)
		SELECT 
			id,
			name,
			tiebreak_matches,
			tiebreak_wins,
			ROUND(tiebreak_wins * 100.0 / tiebreak_matches, 1) as tiebreak_win_rate
		FROM tiebreak_stats
		ORDER BY tiebreak_win_rate DESC, tiebreak_matches DESC
		LIMIT 10
	`

	rows, err := h.service.db.QueryContext(ctx, query)
	if err != nil {
		log.Printf("Error getting tiebreak masters: %v", err)
		return []TiebreakMasterAchievement{}
	}
	defer rows.Close()

	var achievements []TiebreakMasterAchievement
	currentRank := 1
	var previousWinRate *float64

	for rows.Next() {
		var playerID, playerName string
		var tiebreakMatches, tiebreakWins int
		var tiebreakWinRate float64

		err := rows.Scan(&playerID, &playerName, &tiebreakMatches, &tiebreakWins, &tiebreakWinRate)
		if err != nil {
			log.Printf("Error scanning tiebreak master: %v", err)
			continue
		}

		// Handle ties: if win rate is different from previous, update rank
		if previousWinRate != nil && tiebreakWinRate != *previousWinRate {
			currentRank = len(achievements) + 1
		}

		achievement := TiebreakMasterAchievement{
			PlayerSummary: PlayerSummary{
				ID:           playerID,
				Name:         playerName,
				MatchesCount: tiebreakMatches,
			},
			TiebreakMatches: tiebreakMatches,
			TiebreakWins:    tiebreakWins,
			TiebreakWinRate: tiebreakWinRate,
			Description:     fmt.Sprintf("%.1f%% win rate in tiebreaks (%d/%d)", tiebreakWinRate, tiebreakWins, tiebreakMatches),
			Rank:            currentRank,
		}

		achievements = append(achievements, achievement)
		previousWinRate = &tiebreakWinRate
	}

	return achievements
}

func (h *ClubWrappedHandler) calculateClubSeasonHighlights(ctx context.Context, highlights *ClubSeasonHighlights) error {
	// Page 10: Season Highlights - timeline, biggest upset, etc.
	return nil
}
