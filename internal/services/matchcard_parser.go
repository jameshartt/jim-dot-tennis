package services

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// APIResponse represents the JSON response from BHPLTA API
type APIResponse struct {
	Status  string `json:"status"`
	Output  string `json:"output"`
	EventID string `json:"event_id"`
}

// MatchCardData represents parsed match card data from BHPLTA
type MatchCardData struct {
	ExternalID int
	HomeTeam   string
	AwayTeam   string
	Division   string
	Week       int
	EventDate  time.Time
	PlayedDate time.Time
	Matchups   []MatchupData
}

// MatchupData represents a single matchup within a match card
type MatchupData struct {
	Type        string // "First mixed", "Second mixed", "Men's", "Ladies'"
	HomePlayers []string
	AwayPlayers []string
	HomeScores  []int // Individual set scores for home team
	AwayScores  []int // Individual set scores for away team
	HomeSets    int   // Number of sets won by home team
	AwaySets    int   // Number of sets won by away team
}

// MatchCardParser handles parsing HTML responses from BHPLTA
type MatchCardParser struct{}

// NewMatchCardParser creates a new match card parser
func NewMatchCardParser() *MatchCardParser {
	return &MatchCardParser{}
}

// ParseResponse parses the BHPLTA API response and extracts match card data
func (p *MatchCardParser) ParseResponse(responseBody []byte) ([]MatchCardData, error) {
	// First try to parse as JSON (API response format)
	var apiResponse APIResponse
	if err := json.Unmarshal(responseBody, &apiResponse); err == nil && apiResponse.Status == "success" {
		// Use the HTML content from the "output" field
		return p.parseHTML(apiResponse.Output)
	}

	// Fallback to parsing as raw HTML
	return p.parseHTML(string(responseBody))
}

// parseHTML parses HTML content and extracts match card data
func (p *MatchCardParser) parseHTML(htmlContent string) ([]MatchCardData, error) {
	// Load HTML document
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var matchCards []MatchCardData

	// Find each match wrapper
	doc.Find(".bhplta-club-scores-match-wrapper").Each(func(i int, matchWrapper *goquery.Selection) {
		matchCard, err := p.parseMatchCard(matchWrapper)
		if err != nil {
			fmt.Printf("Warning: failed to parse match card %d: %v\n", i+1, err)
			return
		}
		matchCards = append(matchCards, matchCard)
	})

	return matchCards, nil
}

// parseMatchCard parses a single match card from HTML
func (p *MatchCardParser) parseMatchCard(matchWrapper *goquery.Selection) (MatchCardData, error) {
	var matchCard MatchCardData

	// Extract external ID from the copy link button
	linkButton := matchWrapper.Find(".bhplta-copy-text-btn")
	if linkHref, exists := linkButton.Attr("data-linkhref"); exists {
		// Extract ID from URL like "https://www.bhplta.co.uk/bhplta_tables/parks-league-match-cards/?id=3335"
		re := regexp.MustCompile(`id=(\d+)`)
		matches := re.FindStringSubmatch(linkHref)
		if len(matches) > 1 {
			if id, err := strconv.Atoi(matches[1]); err == nil {
				matchCard.ExternalID = id
			}
		}
	}

	// Extract team names from header
	header := matchWrapper.Find(".bhplta-club-scores-header h2").Text()
	teams := p.parseTeamNames(header)
	if len(teams) == 2 {
		matchCard.HomeTeam = teams[0]
		matchCard.AwayTeam = teams[1]
	}

	// Extract division and week info
	metaText := matchWrapper.Find(".bhplta-club-scores-meta p").First().Text()
	matchCard.Division, matchCard.Week = p.parseDivisionAndWeek(metaText)

	// Extract dates
	dateText := matchWrapper.Find(".bhplta-club-scores-meta p").Last().Text()
	matchCard.EventDate, matchCard.PlayedDate = p.parseDates(dateText)

	// Extract matchups
	matchups := p.parseMatchups(matchWrapper)
	matchCard.Matchups = matchups

	return matchCard, nil
}

// parseTeamNames extracts team names from header text
func (p *MatchCardParser) parseTeamNames(header string) []string {
	// Replace non-breaking spaces and normalize whitespace
	header = strings.ReplaceAll(header, "\u00a0", " ")
	header = regexp.MustCompile(`\s+`).ReplaceAllString(header, " ")
	header = strings.TrimSpace(header)

	// Split on " v " pattern
	parts := strings.Split(header, " v ")
	if len(parts) != 2 {
		return []string{}
	}

	homeTeam := strings.TrimSpace(parts[0])
	awayTeam := strings.TrimSpace(parts[1])

	// Handle HTML entities
	homeTeam = strings.ReplaceAll(homeTeam, "&#039;", "'")
	awayTeam = strings.ReplaceAll(awayTeam, "&#039;", "'")

	return []string{homeTeam, awayTeam}
}

// parseDivisionAndWeek extracts division and week from meta text
func (p *MatchCardParser) parseDivisionAndWeek(metaText string) (string, int) {
	// Text like "Division 1  |  Week 1"
	parts := strings.Split(metaText, "|")

	var division string
	var week int

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "Division") {
			division = part
		} else if strings.HasPrefix(part, "Week") {
			// Extract week number
			re := regexp.MustCompile(`Week (\d+)`)
			matches := re.FindStringSubmatch(part)
			if len(matches) > 1 {
				if w, err := strconv.Atoi(matches[1]); err == nil {
					week = w
				}
			}
		}
	}

	return division, week
}

// parseDates extracts event and played dates from date text
func (p *MatchCardParser) parseDates(dateText string) (time.Time, time.Time) {
	// Text like "Event date: 17 Apr 2025    |    Date played: 17 Apr 2025"
	var eventDate, playedDate time.Time

	// Split by pipe and process each part
	parts := strings.Split(dateText, "|")
	for _, part := range parts {
		part = strings.TrimSpace(part)

		if strings.Contains(part, "Event date:") {
			dateStr := strings.TrimSpace(strings.Split(part, ":")[1])
			if date, err := time.Parse("2 Jan 2006", dateStr); err == nil {
				eventDate = date
			}
		} else if strings.Contains(part, "Date played:") {
			dateStr := strings.TrimSpace(strings.Split(part, ":")[1])
			if date, err := time.Parse("2 Jan 2006", dateStr); err == nil {
				playedDate = date
			}
		}
	}

	return eventDate, playedDate
}

// parseMatchups extracts all matchups from the match card
func (p *MatchCardParser) parseMatchups(matchWrapper *goquery.Selection) []MatchupData {
	var matchups []MatchupData

	// Find each matchup section
	matchWrapper.Find("h4").Each(func(i int, header *goquery.Selection) {
		matchupType := strings.TrimSpace(header.Text())

		// Get the table that follows this header
		table := header.Next().Find("table.bhplta-club-scores-table")
		if table.Length() > 0 {
			matchup := p.parseMatchupTable(matchupType, table)
			matchups = append(matchups, matchup)
		}
	})

	return matchups
}

// parseMatchupTable parses a single matchup table
func (p *MatchCardParser) parseMatchupTable(matchupType string, table *goquery.Selection) MatchupData {
	matchup := MatchupData{
		Type: matchupType,
	}

	// Find home and away rows
	homeRow := table.Find("tr.bhplta-club-scores-home")
	awayRow := table.Find("tr.bhplta-club-scores-away")

	// Parse home team
	if homeRow.Length() > 0 {
		matchup.HomePlayers = p.parsePlayerNamesFromHTML(homeRow.Find("td.bhplta-club-scores-player-names"))
		matchup.HomeScores, matchup.HomeSets = p.parseTeamScores(homeRow)
	}

	// Parse away team
	if awayRow.Length() > 0 {
		matchup.AwayPlayers = p.parsePlayerNamesFromHTML(awayRow.Find("td.bhplta-club-scores-player-names"))
		matchup.AwayScores, matchup.AwaySets = p.parseTeamScores(awayRow)
	}

	return matchup
}

// parsePlayerNamesFromHTML extracts player names from HTML element preserving structure
func (p *MatchCardParser) parsePlayerNamesFromHTML(playerCell *goquery.Selection) []string {
	if playerCell.Length() == 0 {
		return []string{}
	}

	// Get the HTML content to preserve structure
	html, err := playerCell.Html()
	if err != nil {
		// Fallback to text parsing
		return p.parsePlayerNames(playerCell.Text())
	}

	// Handle special cases for conceded matches
	if strings.Contains(html, "Conceded by") || strings.Contains(html, "Given to") {
		return []string{}
	}

	var players []string

	// Split by <br> tags first (most common separator)
	parts := regexp.MustCompile(`<br\s*/?>`).Split(html, -1)

	for _, part := range parts {
		// Clean up HTML tags and whitespace
		part = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(part, "")
		part = strings.ReplaceAll(part, "&nbsp;", " ")
		part = strings.ReplaceAll(part, "\u00a0", " ")
		part = regexp.MustCompile(`\s+`).ReplaceAllString(part, " ")
		part = strings.TrimSpace(part)

		if part != "" {
			players = append(players, part)
		}
	}

	// If no <br> tags found, try other separators
	if len(players) <= 1 && len(parts) == 1 {
		// Try parsing the text content with improved heuristics
		text := playerCell.Text()
		players = p.parsePlayerNamesFromText(text)
	}

	return players
}

// parsePlayerNamesFromText attempts to split concatenated player names using heuristics
func (p *MatchCardParser) parsePlayerNamesFromText(text string) []string {
	// Clean up the text
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")
	text = strings.ReplaceAll(text, "\u00a0", " ")
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)

	// Handle special cases
	if text == "" {
		return []string{}
	}

	if strings.Contains(text, "Conceded by") || strings.Contains(text, "Given to") {
		return []string{}
	}

	var players []string

	// Strategy 1: Look for pattern of "FirstName LastNameFirstName LastName"
	// This regex looks for a lowercase letter followed by an uppercase letter (word boundary)
	re := regexp.MustCompile(`([A-Z][a-z]+(?:\s+[A-Z][a-z]+)*?)([A-Z][a-z].*?)$`)
	matches := re.FindStringSubmatch(text)

	if len(matches) == 3 {
		player1 := strings.TrimSpace(matches[1])
		player2 := strings.TrimSpace(matches[2])

		if player1 != "" && player2 != "" {
			players = append(players, player1, player2)
			return players
		}
	}

	// Strategy 2: Try to split on capital letters that follow lowercase letters
	// Look for pattern where a lowercase letter is followed by a capital letter
	words := strings.Fields(text)
	if len(words) >= 2 {
		var currentPlayer []string

		for i, word := range words {
			currentPlayer = append(currentPlayer, word)

			// Check if this might be the end of a player name
			// Heuristic: if the next word starts with a capital and current player has at least 2 words
			if i < len(words)-1 && len(currentPlayer) >= 2 {
				nextWord := words[i+1]
				if len(nextWord) > 0 && nextWord[0] >= 'A' && nextWord[0] <= 'Z' {
					// This might be the start of a new player name
					players = append(players, strings.Join(currentPlayer, " "))
					currentPlayer = []string{}
				}
			}
		}

		// Add the remaining words as the last player
		if len(currentPlayer) > 0 {
			players = append(players, strings.Join(currentPlayer, " "))
		}

		// If we got exactly 2 players, return them
		if len(players) == 2 {
			return players
		}
	}

	// Strategy 3: If we have 4 words, assume it's "FirstName1 LastName1 FirstName2 LastName2"
	if len(words) == 4 {
		player1 := words[0] + " " + words[1]
		player2 := words[2] + " " + words[3]
		return []string{player1, player2}
	}

	// Fallback: return the whole text as one player
	return []string{text}
}

// parsePlayerNames extracts player names from player cell text (legacy method)
func (p *MatchCardParser) parsePlayerNames(playersText string) []string {
	return p.parsePlayerNamesFromText(playersText)
}

// parseTeamScores extracts scores for one team from their row
func (p *MatchCardParser) parseTeamScores(row *goquery.Selection) ([]int, int) {
	var teamScores []int

	// Find score cells (excluding team name and player names)
	scoreCells := row.Find("td.bhplta-club-scores-sets")

	scoreCells.Each(func(i int, cell *goquery.Selection) {
		scoreText := strings.TrimSpace(cell.Text())

		// Skip empty cells or non-numeric content
		if scoreText == "" || scoreText == "&nbsp;" || scoreText == " " {
			return
		}

		// Parse the score - expect individual numbers like "6", "4", "7", "5"
		if score, err := strconv.Atoi(scoreText); err == nil {
			teamScores = append(teamScores, score)
		}
	})

	// Count sets won for this team
	// We can't determine sets won from just one team's scores
	// This will be calculated later when we have both teams' scores
	setsWon := 0

	// Basic heuristic: count scores >= 6 as potential set wins
	for _, score := range teamScores {
		if score >= 6 {
			setsWon++
		}
	}

	return teamScores, setsWon
}

// parseScores extracts individual set scores and calculates sets won from a table row
// This is the legacy method - keeping for backward compatibility
func (p *MatchCardParser) parseScores(row *goquery.Selection) ([]int, int) {
	return p.parseTeamScores(row)
}
