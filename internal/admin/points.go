package admin

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"jim-dot-tennis/internal/models"
)

// PlayerPoints represents a player's points and match statistics
type PlayerPoints struct {
	ID               string
	Name             string
	LastName         string
	Gender           models.PlayerGender
	ReportingPrivacy models.PlayerReportingPrivacy
	MatchesPlayed    int
	TotalPoints      float64
	WinPoints        float64
	SetPoints        float64
}

// PointsHandler handles points table requests
type PointsHandler struct {
	service     *Service
	templateDir string
}

// NewPointsHandler creates a new points handler
func NewPointsHandler(service *Service, templateDir string) *PointsHandler {
	return &PointsHandler{
		service:     service,
		templateDir: templateDir,
	}
}

// HandlePointsTable handles the points table page
func (h *PointsHandler) HandlePointsTable(w http.ResponseWriter, r *http.Request) {
	log.Printf("Points table handler called with method: %s", r.Method)

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

	// Calculate points for all players
	menPlayers, womenPlayers, err := h.calculatePlayerPoints()
	if err != nil {
		logAndError(w, "Failed to calculate player points", err, http.StatusInternalServerError)
		return
	}

	// Get current week information
	currentWeek, err := h.getCurrentWeekNumber()
	if err != nil {
		log.Printf("Warning: Failed to get current week: %v", err)
		currentWeek = 1 // Default fallback
	}

	// Build rescheduled fixtures header for this week if applicable
	rescheduledHeader, err := h.getRescheduledWeekHeader()
	if err != nil {
		log.Printf("Warning: Failed to build rescheduled fixtures header: %v", err)
	}

	// Load the points table template
	tmpl, err := parseTemplate(h.templateDir, "admin/points_table.html")
	if err != nil {
		log.Printf("Error parsing points table template: %v", err)
		renderFallbackHTML(w, "Points Table", "Points Table",
			"Points table coming soon", "/admin/dashboard")
		return
	}

	// Execute template with data
	templateData := map[string]interface{}{
		"User":              user,
		"MenPlayers":        menPlayers,
		"WomenPlayers":      womenPlayers,
		"CurrentWeek":       currentWeek,
		"RescheduledHeader": rescheduledHeader,
	}

	if err := renderTemplate(w, tmpl, templateData); err != nil {
		logAndError(w, err.Error(), err, http.StatusInternalServerError)
	}
}

// calculatePlayerPoints calculates points for all players based on their matchup results
func (h *PointsHandler) calculatePlayerPoints() ([]PlayerPoints, []PlayerPoints, error) {
	ctx := context.Background()

	// Get all players with their gender information
	players, err := h.service.GetPlayers()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get players: %w", err)
	}

	// Create a map for quick player lookup
	playerMap := make(map[string]*models.Player)
	for i := range players {
		playerMap[players[i].ID] = &players[i]
	}

	// Get all completed matchups with player information
	matchups, err := h.getCompletedMatchupsWithPlayers(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get matchups: %w", err)
	}

	// Calculate points for each player
	playerPointsMap := make(map[string]*PlayerPoints)

	// Initialize all players with zero points
	for _, player := range players {
		displayName := player.FirstName + " " + player.LastName
		if player.PreferredName != nil && *player.PreferredName != "" {
			displayName = *player.PreferredName
		}

		playerPointsMap[player.ID] = &PlayerPoints{
			ID:               player.ID,
			Name:             displayName,
			LastName:         player.LastName,
			Gender:           player.Gender,
			ReportingPrivacy: player.ReportingPrivacy,
			MatchesPlayed:    0,
			TotalPoints:      0,
			WinPoints:        0,
			SetPoints:        0,
		}
	}

	// Process each matchup to calculate points
	for _, matchup := range matchups {
		h.processMatchupPoints(matchup, playerPointsMap)
	}

	// Apply Division 4 rule (max 18 matches per player)
	h.applyDivision4Rule(playerPointsMap)

	// Separate men and women, and sort by points
	var menPlayers, womenPlayers []PlayerPoints

	for _, playerPoints := range playerPointsMap {
		// Only include players who have played at least one match and have visible reporting privacy
		if playerPoints.MatchesPlayed > 0 && playerPoints.ReportingPrivacy == models.PlayerReportingVisible {
			switch playerPoints.Gender {
			case models.PlayerGenderMen:
				menPlayers = append(menPlayers, *playerPoints)
			case models.PlayerGenderWomen:
				womenPlayers = append(womenPlayers, *playerPoints)
				// Skip Unknown gender players for now
			}
		}
	}

	// Sort by total points (descending), then by last name (ascending) as tiebreaker
	sortPlayersByPoints := func(players []PlayerPoints) {
		sort.Slice(players, func(i, j int) bool {
			// Use small epsilon for float comparison
			const epsilon = 1e-9
			if math.Abs(players[i].TotalPoints-players[j].TotalPoints) < epsilon {
				// If points are equal, sort by last name alphabetically (case-insensitive)
				return strings.ToLower(players[i].LastName) < strings.ToLower(players[j].LastName)
			}
			return players[i].TotalPoints > players[j].TotalPoints
		})
	}

	sortPlayersByPoints(menPlayers)
	sortPlayersByPoints(womenPlayers)

	return menPlayers, womenPlayers, nil
}

// CompletedMatchupWithPlayers represents a matchup with associated player information
type CompletedMatchupWithPlayers struct {
	Matchup       models.Matchup
	HomePlayers   []models.Player
	AwayPlayers   []models.Player
	FixtureStatus models.FixtureStatus
}

// getCompletedMatchupsWithPlayers retrieves all completed matchups with player information
func (h *PointsHandler) getCompletedMatchupsWithPlayers(ctx context.Context) ([]CompletedMatchupWithPlayers, error) {
	// Get all completed matchups that are not *halved* matchups
	query := `
		SELECT m.id, m.fixture_id, m.type, m.status, m.home_score, m.away_score,
		       m.home_set1, m.away_set1, m.home_set2, m.away_set2, m.home_set3, m.away_set3,
		       m.notes, m.managing_team_id, m.created_at, m.updated_at, f.status as fixture_status
		FROM matchups m
		INNER JOIN fixtures f ON m.fixture_id = f.id
		WHERE m.status = 'Finished' AND f.status = 'Completed'
		  AND f.id NOT IN (
		    SELECT f2.id
		    FROM fixtures f2
		    INNER JOIN matchups m2 ON m2.fixture_id = f2.id
		    WHERE f2.status = 'Completed'
		    GROUP BY f2.id
		    HAVING COUNT(*) > 0
		       AND SUM(CASE WHEN m2.home_score = 1 AND m2.away_score = 1 THEN 1 ELSE 0 END) = COUNT(*)
		  )
		ORDER BY m.created_at ASC
	`

	rows, err := h.service.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query matchups: %w", err)
	}
	defer rows.Close()

	var matchupsWithPlayers []CompletedMatchupWithPlayers

	for rows.Next() {
		var matchup models.Matchup
		var fixtureStatusStr string

		err := rows.Scan(
			&matchup.ID, &matchup.FixtureID, &matchup.Type, &matchup.Status,
			&matchup.HomeScore, &matchup.AwayScore,
			&matchup.HomeSet1, &matchup.AwaySet1, &matchup.HomeSet2, &matchup.AwaySet2,
			&matchup.HomeSet3, &matchup.AwaySet3, &matchup.Notes, &matchup.ManagingTeamID,
			&matchup.CreatedAt, &matchup.UpdatedAt, &fixtureStatusStr,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan matchup: %w", err)
		}

		// Get players for this matchup
		homePlayers, awayPlayers, err := h.getMatchupPlayers(ctx, matchup.ID)
		if err != nil {
			log.Printf("Warning: Failed to get players for matchup %d: %v", matchup.ID, err)
			continue
		}

		matchupsWithPlayers = append(matchupsWithPlayers, CompletedMatchupWithPlayers{
			Matchup:       matchup,
			HomePlayers:   homePlayers,
			AwayPlayers:   awayPlayers,
			FixtureStatus: models.FixtureStatus(fixtureStatusStr),
		})
	}

	return matchupsWithPlayers, nil
}

// getMatchupPlayers retrieves home and away players for a specific matchup
func (h *PointsHandler) getMatchupPlayers(ctx context.Context, matchupID uint) ([]models.Player, []models.Player, error) {
	query := `
		SELECT p.id, p.first_name, p.last_name, p.preferred_name, p.gender, p.reporting_privacy,
		       p.club_id, p.fantasy_match_id, p.created_at, p.updated_at, mp.is_home
		FROM players p
		INNER JOIN matchup_players mp ON p.id = mp.player_id
		WHERE mp.matchup_id = ?
		ORDER BY mp.is_home DESC, p.last_name ASC
	`

	rows, err := h.service.db.QueryContext(ctx, query, matchupID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query matchup players: %w", err)
	}
	defer rows.Close()

	var homePlayers, awayPlayers []models.Player

	for rows.Next() {
		var player models.Player
		var isHome bool

		err := rows.Scan(
			&player.ID, &player.FirstName, &player.LastName, &player.PreferredName,
			&player.Gender, &player.ReportingPrivacy, &player.ClubID, &player.FantasyMatchID,
			&player.CreatedAt, &player.UpdatedAt, &isHome,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to scan player: %w", err)
		}

		if isHome {
			homePlayers = append(homePlayers, player)
		} else {
			awayPlayers = append(awayPlayers, player)
		}
	}

	return homePlayers, awayPlayers, nil
}

// processMatchupPoints calculates and assigns points for a single matchup
func (h *PointsHandler) processMatchupPoints(matchup CompletedMatchupWithPlayers, playerPointsMap map[string]*PlayerPoints) {
	// Count sets won by each team
	homeSetsWon := 0
	awaySetsWon := 0

	// Count set wins
	if matchup.Matchup.HomeSet1 != nil && matchup.Matchup.AwaySet1 != nil {
		if *matchup.Matchup.HomeSet1 > *matchup.Matchup.AwaySet1 {
			homeSetsWon++
		} else if *matchup.Matchup.AwaySet1 > *matchup.Matchup.HomeSet1 {
			awaySetsWon++
		}
	}

	if matchup.Matchup.HomeSet2 != nil && matchup.Matchup.AwaySet2 != nil {
		if *matchup.Matchup.HomeSet2 > *matchup.Matchup.AwaySet2 {
			homeSetsWon++
		} else if *matchup.Matchup.AwaySet2 > *matchup.Matchup.HomeSet2 {
			awaySetsWon++
		}
	}

	if matchup.Matchup.HomeSet3 != nil && matchup.Matchup.AwaySet3 != nil {
		if *matchup.Matchup.HomeSet3 > *matchup.Matchup.AwaySet3 {
			homeSetsWon++
		} else if *matchup.Matchup.AwaySet3 > *matchup.Matchup.HomeSet3 {
			awaySetsWon++
		}
	}

	// Determine match result
	var homeWinPoints, awayWinPoints float64

	if homeSetsWon > awaySetsWon {
		// Home team wins the match
		homeWinPoints = 1.0
		awayWinPoints = 0.0
	} else if awaySetsWon > homeSetsWon {
		// Away team wins the match
		homeWinPoints = 0.0
		awayWinPoints = 1.0
	} else {
		// Halved match (equal sets won)
		homeWinPoints = 0.5
		awayWinPoints = 0.5
	}

	// Award points to home players
	for _, player := range matchup.HomePlayers {
		if playerPoints, exists := playerPointsMap[player.ID]; exists {
			playerPoints.MatchesPlayed++

			// Win points (1 point for match win, 0.5 for halved match)
			playerPoints.WinPoints += homeWinPoints

			// Set points (1 point per set win)
			playerPoints.SetPoints += float64(homeSetsWon)

			// Update total points
			playerPoints.TotalPoints = playerPoints.WinPoints + playerPoints.SetPoints
		}
	}

	// Award points to away players
	for _, player := range matchup.AwayPlayers {
		if playerPoints, exists := playerPointsMap[player.ID]; exists {
			playerPoints.MatchesPlayed++

			// Win points (1 point for match win, 0.5 for halved match)
			playerPoints.WinPoints += awayWinPoints

			// Set points (1 point per set win)
			playerPoints.SetPoints += float64(awaySetsWon)

			// Update total points
			playerPoints.TotalPoints = playerPoints.WinPoints + playerPoints.SetPoints
		}
	}
}

// applyDivision4Rule limits each player to maximum 18 matches (Division 4 rule)
func (h *PointsHandler) applyDivision4Rule(playerPointsMap map[string]*PlayerPoints) {
	// For now, we'll just cap the matches played at 18
	// In a more sophisticated implementation, we'd need to track match history
	// and only count points from the first 18 matches chronologically
	for _, playerPoints := range playerPointsMap {
		if playerPoints.MatchesPlayed > 18 {
			// This is a simplified implementation
			// Ideally we'd recalculate points based on first 18 matches only
			playerPoints.MatchesPlayed = 18
		}
	}
}

// getCurrentWeekNumber gets the current week number from the most recently completed fixture
func (h *PointsHandler) getCurrentWeekNumber() (int, error) {
	ctx := context.Background()

	query := `
		SELECT w.week_number
		FROM weeks w
		INNER JOIN fixtures f ON w.id = f.week_id
		WHERE f.status = 'Completed'
		ORDER BY f.completed_date DESC, w.week_number DESC
		LIMIT 1
	`

	var weekNumber int
	err := h.service.db.GetContext(ctx, &weekNumber, query)
	if err != nil {
		// If no completed fixtures found, try to get the current active week
		query = `
			SELECT week_number
			FROM weeks
			WHERE is_active = TRUE
			ORDER BY week_number DESC
			LIMIT 1
		`

		err = h.service.db.GetContext(ctx, &weekNumber, query)
		if err != nil {
			return 1, fmt.Errorf("failed to get current week: %w", err)
		}
	}

	return weekNumber, nil
}

// getRescheduledWeekHeader checks the most recent week of completed fixtures by completed date.
// If all fixtures completed in that window are rescheduled (completed outside their scheduled week),
// it returns a descriptive header listing those fixtures. Otherwise returns an empty string.
func (h *PointsHandler) getRescheduledWeekHeader() (string, error) {
	ctx := context.Background()

	// Find the most recent completed date
	var latestCompleted time.Time
	err := h.service.db.GetContext(ctx, &latestCompleted, `
		SELECT completed_date
		FROM fixtures
		WHERE status = 'Completed' AND completed_date IS NOT NULL
		ORDER BY completed_date DESC
		LIMIT 1
	`)
	if err != nil {
		// No rows or other error – treat as no header
		return "", nil
	}
	if latestCompleted.IsZero() {
		return "", nil
	}

	// Determine the start (Sunday) and end (Saturday) of that week in UTC
	completedUTC := latestCompleted.UTC()
	weekStart := completedUTC.AddDate(0, 0, -int(completedUTC.Weekday()))
	weekStart = time.Date(weekStart.Year(), weekStart.Month(), weekStart.Day(), 0, 0, 0, 0, time.UTC)
	weekEnd := weekStart.AddDate(0, 0, 6)
	weekEnd = time.Date(weekEnd.Year(), weekEnd.Month(), weekEnd.Day(), 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC)

	// Fetch completed fixtures in that completed window, including their scheduled week numbers
	rows, err := h.service.db.QueryContext(ctx, `
		SELECT f.id, f.home_team_id, f.away_team_id, f.completed_date, w.week_number
		FROM fixtures f
		INNER JOIN weeks w ON w.id = f.week_id
		WHERE f.status = 'Completed'
		  AND f.completed_date IS NOT NULL
		  AND f.completed_date >= ? AND f.completed_date <= ?
		  AND f.id NOT IN (
		    SELECT f2.id
		    FROM fixtures f2
		    INNER JOIN matchups m2 ON m2.fixture_id = f2.id
		    WHERE f2.status = 'Completed'
		    GROUP BY f2.id
		    HAVING COUNT(*) > 0
		       AND SUM(CASE WHEN m2.home_score = 1 AND m2.away_score = 1 THEN 1 ELSE 0 END) = COUNT(*)
		  )
		ORDER BY f.completed_date ASC
	`, weekStart, weekEnd)
	if err != nil {
		return "", fmt.Errorf("failed to query completed fixtures in window: %w", err)
	}
	defer rows.Close()

	type fixtureRow struct {
		ID               uint
		HomeTeamID       uint
		AwayTeamID       uint
		CompletedDate    time.Time
		ScheduledWeekNum int
	}

	var fixturesInWindow []fixtureRow
	for rows.Next() {
		var r fixtureRow
		if err := rows.Scan(&r.ID, &r.HomeTeamID, &r.AwayTeamID, &r.CompletedDate, &r.ScheduledWeekNum); err != nil {
			return "", fmt.Errorf("failed to scan fixture: %w", err)
		}
		fixturesInWindow = append(fixturesInWindow, r)
	}

	if len(fixturesInWindow) == 0 {
		return "", nil
	}

	// Helper to find the week that contains a date
	getWeekNumberForDate := func(ts time.Time) (int, bool) {
		var num int
		err := h.service.db.GetContext(ctx, &num, `
			SELECT week_number FROM weeks WHERE start_date <= ? AND end_date >= ? LIMIT 1
		`, ts, ts)
		if err != nil {
			return 0, false
		}
		return num, true
	}

	// Determine if every completed fixture in this window is rescheduled
	allRescheduled := true
	for _, f := range fixturesInWindow {
		if completedWeekNum, ok := getWeekNumberForDate(f.CompletedDate); ok {
			if completedWeekNum == f.ScheduledWeekNum {
				allRescheduled = false
				break
			}
		} else {
			// No enclosing week found (post-season) => still rescheduled
			continue
		}
	}

	if !allRescheduled {
		return "", nil
	}

	// Build descriptive header for these rescheduled fixtures
	var parts []string
	for _, f := range fixturesInWindow {
		homeTeam, _ := h.service.teamRepository.FindByID(ctx, f.HomeTeamID)
		awayTeam, _ := h.service.teamRepository.FindByID(ctx, f.AwayTeamID)
		homeName := fmt.Sprintf("Team %d", f.HomeTeamID)
		awayName := fmt.Sprintf("Team %d", f.AwayTeamID)
		if homeTeam != nil && homeTeam.Name != "" {
			homeName = homeTeam.Name
		}
		if awayTeam != nil && awayTeam.Name != "" {
			awayName = awayTeam.Name
		}
		parts = append(parts, fmt.Sprintf("%s vs %s (Week %d)", homeName, awayName, f.ScheduledWeekNum))
	}

	return "Rescheduled fixtures played this week: " + strings.Join(parts, "; "), nil
}
