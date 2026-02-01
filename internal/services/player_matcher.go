package services

import (
	"context"
	"fmt"
	"strings"

	"jim-dot-tennis/internal/repository"
)

// PlayerMatcher handles matching player names from match cards to database players
type PlayerMatcher struct {
	playerRepo repository.PlayerRepository
}

// NewPlayerMatcher creates a new player matcher
func NewPlayerMatcher(playerRepo repository.PlayerRepository) *PlayerMatcher {
	return &PlayerMatcher{
		playerRepo: playerRepo,
	}
}

// MatchPlayer attempts to find a database player that matches the given name
func (m *PlayerMatcher) MatchPlayer(ctx context.Context, playerName string) (string, error) {
	// Normalize the input name
	normalizedName := m.normalizeName(playerName)

	// Get all players from the database
	players, err := m.playerRepo.FindAll(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to fetch players: %w", err)
	}

	// Try exact match first
	for _, player := range players {
		dbName := m.normalizeName(fmt.Sprintf("%s %s", player.FirstName, player.LastName))
		if normalizedName == dbName {
			return player.ID, nil
		}
	}

	// Try fuzzy matching
	bestMatch := ""
	bestScore := 0.0

	for _, player := range players {
		dbName := fmt.Sprintf("%s %s", player.FirstName, player.LastName)
		score := m.calculateSimilarity(normalizedName, m.normalizeName(dbName))

		// Consider it a match if similarity is high enough (>= 80%)
		if score >= 0.8 && score > bestScore {
			bestScore = score
			bestMatch = player.ID
		}
	}

	if bestMatch != "" {
		return bestMatch, nil
	}

	return "", fmt.Errorf("no matching player found for '%s'", playerName)
}

// normalizeName normalizes a player name for comparison
func (m *PlayerMatcher) normalizeName(name string) string {
	// Convert to lowercase
	name = strings.ToLower(name)

	// Remove extra whitespace
	name = strings.TrimSpace(name)
	name = strings.Join(strings.Fields(name), " ")

	// Remove common punctuation
	name = strings.ReplaceAll(name, ".", "")
	name = strings.ReplaceAll(name, ",", "")
	name = strings.ReplaceAll(name, "-", " ")

	return name
}

// calculateSimilarity calculates similarity between two strings using a simple algorithm
func (m *PlayerMatcher) calculateSimilarity(s1, s2 string) float64 {
	// Simple Levenshtein-like similarity calculation
	if s1 == s2 {
		return 1.0
	}

	// If either string is empty, no similarity
	if len(s1) == 0 || len(s2) == 0 {
		return 0.0
	}

	// Calculate edit distance
	distance := m.levenshteinDistance(s1, s2)
	maxLen := len(s1)
	if len(s2) > maxLen {
		maxLen = len(s2)
	}

	// Convert distance to similarity (0-1 range)
	similarity := 1.0 - float64(distance)/float64(maxLen)

	return similarity
}

// levenshteinDistance calculates the Levenshtein distance between two strings
func (m *PlayerMatcher) levenshteinDistance(s1, s2 string) int {
	r1 := []rune(s1)
	r2 := []rune(s2)

	// Create a matrix to store distances
	matrix := make([][]int, len(r1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(r2)+1)
	}

	// Initialize first row and column
	for i := 0; i <= len(r1); i++ {
		matrix[i][0] = i
	}
	for j := 0; j <= len(r2); j++ {
		matrix[0][j] = j
	}

	// Fill the matrix
	for i := 1; i <= len(r1); i++ {
		for j := 1; j <= len(r2); j++ {
			cost := 1
			if r1[i-1] == r2[j-1] {
				cost = 0
			}

			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len(r1)][len(r2)]
}

// min returns the minimum of three integers
func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}
