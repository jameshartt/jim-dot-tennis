package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
	"os"
	"regexp"
	"strings"
	"time"
)

// ProTennisPlayerRepository defines the interface for tennis player data access
type ProTennisPlayerRepository interface {
	// Basic CRUD operations
	FindAll(ctx context.Context) ([]models.ProTennisPlayer, error)
	FindByID(ctx context.Context, id int) (*models.ProTennisPlayer, error)
	Create(ctx context.Context, player *models.ProTennisPlayer) error
	Update(ctx context.Context, player *models.ProTennisPlayer) error
	Delete(ctx context.Context, id int) error

	// Tour-specific queries
	FindByTour(ctx context.Context, tour string) ([]models.ProTennisPlayer, error)
	FindATPPlayers(ctx context.Context) ([]models.ProTennisPlayer, error)
	FindWTAPlayers(ctx context.Context) ([]models.ProTennisPlayer, error)
	FindByGender(ctx context.Context, gender string) ([]models.ProTennisPlayer, error)

	// Ranking queries
	FindByRankRange(ctx context.Context, minRank, maxRank int) ([]models.ProTennisPlayer, error)
	FindTopRanked(ctx context.Context, limit int) ([]models.ProTennisPlayer, error)
	FindByNationality(ctx context.Context, nationality string) ([]models.ProTennisPlayer, error)

	// Search and filtering
	SearchByName(ctx context.Context, name string) ([]models.ProTennisPlayer, error)
	FindByLastName(ctx context.Context, lastName string) ([]models.ProTennisPlayer, error)

	// Import operations
	ImportFromJSON(ctx context.Context, filePath string) error
	ClearAll(ctx context.Context) error

	// Statistics
	CountByTour(ctx context.Context, tour string) (int, error)
	CountAll(ctx context.Context) (int, error)
}

// FantasyMixedDoublesRepository defines the interface for fantasy mixed doubles data access
type FantasyMixedDoublesRepository interface {
	// Basic CRUD operations
	FindAll(ctx context.Context) ([]models.FantasyMixedDoubles, error)
	FindByID(ctx context.Context, id uint) (*models.FantasyMixedDoubles, error)
	Create(ctx context.Context, match *models.FantasyMixedDoubles) error
	Update(ctx context.Context, match *models.FantasyMixedDoubles) error
	Delete(ctx context.Context, id uint) error

	// Authentication queries
	FindByAuthToken(ctx context.Context, authToken string) (*models.FantasyMixedDoubles, error)
	FindActive(ctx context.Context) ([]models.FantasyMixedDoubles, error)
	FindInactive(ctx context.Context) ([]models.FantasyMixedDoubles, error)

	// Team-based queries
	FindByTeamAWoman(ctx context.Context, teamAWomanID int) ([]models.FantasyMixedDoubles, error)
	FindByTeamAMan(ctx context.Context, teamAManID int) ([]models.FantasyMixedDoubles, error)
	FindByTeamBWoman(ctx context.Context, teamBWomanID int) ([]models.FantasyMixedDoubles, error)
	FindByTeamBMan(ctx context.Context, teamBManID int) ([]models.FantasyMixedDoubles, error)

	// Utility methods
	GenerateRandomMatches(ctx context.Context, count int) error
	GenerateAuthToken(teamAWoman, teamAMan, teamBWoman, teamBMan *models.ProTennisPlayer) string
}

// tennisPlayerRepository implements ProTennisPlayerRepository
type tennisPlayerRepository struct {
	db *database.DB
}

// fantasyMixedDoublesRepository implements FantasyMixedDoublesRepository
type fantasyMixedDoublesRepository struct {
	db *database.DB
}

// NewProTennisPlayerRepository creates a new tennis player repository
func NewProTennisPlayerRepository(db *database.DB) ProTennisPlayerRepository {
	return &tennisPlayerRepository{
		db: db,
	}
}

// NewFantasyMixedDoublesRepository creates a new fantasy mixed doubles repository
func NewFantasyMixedDoublesRepository(db *database.DB) FantasyMixedDoublesRepository {
	return &fantasyMixedDoublesRepository{
		db: db,
	}
}

// Tennis Player Repository Methods

// FindAll retrieves all tennis players ordered by tour, current rank
func (r *tennisPlayerRepository) FindAll(ctx context.Context) ([]models.ProTennisPlayer, error) {
	var players []models.ProTennisPlayer
	err := r.db.SelectContext(ctx, &players, `
		SELECT id, first_name, last_name, common_name, nationality, gender, 
		       current_rank, highest_rank, year_pro, wikipedia_url, hand, 
		       birth_date, birth_place, tour, created_at, updated_at
		FROM tennis_players 
		ORDER BY tour ASC, current_rank ASC
	`)
	return players, err
}

// FindByID retrieves a tennis player by their ID
func (r *tennisPlayerRepository) FindByID(ctx context.Context, id int) (*models.ProTennisPlayer, error) {
	var player models.ProTennisPlayer
	err := r.db.GetContext(ctx, &player, `
		SELECT id, first_name, last_name, common_name, nationality, gender, 
		       current_rank, highest_rank, year_pro, wikipedia_url, hand, 
		       birth_date, birth_place, tour, created_at, updated_at
		FROM tennis_players 
		WHERE id = ?
	`, id)
	if err != nil {
		return nil, err
	}
	return &player, nil
}

// Create inserts a new tennis player record
func (r *tennisPlayerRepository) Create(ctx context.Context, player *models.ProTennisPlayer) error {
	now := time.Now()
	player.CreatedAt = now
	player.UpdatedAt = now

	_, err := r.db.NamedExecContext(ctx, `
		INSERT INTO tennis_players (id, first_name, last_name, common_name, nationality, gender, 
		                           current_rank, highest_rank, year_pro, wikipedia_url, hand, 
		                           birth_date, birth_place, tour, created_at, updated_at)
		VALUES (:id, :first_name, :last_name, :common_name, :nationality, :gender, 
		        :current_rank, :highest_rank, :year_pro, :wikipedia_url, :hand, 
		        :birth_date, :birth_place, :tour, :created_at, :updated_at)
	`, player)

	return err
}

// Update modifies an existing tennis player record
func (r *tennisPlayerRepository) Update(ctx context.Context, player *models.ProTennisPlayer) error {
	player.UpdatedAt = time.Now()

	_, err := r.db.NamedExecContext(ctx, `
		UPDATE tennis_players 
		SET first_name = :first_name, last_name = :last_name, common_name = :common_name,
		    nationality = :nationality, gender = :gender, current_rank = :current_rank,
		    highest_rank = :highest_rank, year_pro = :year_pro, wikipedia_url = :wikipedia_url,
		    hand = :hand, birth_date = :birth_date, birth_place = :birth_place,
		    tour = :tour, updated_at = :updated_at
		WHERE id = :id
	`, player)

	return err
}

// Delete removes a tennis player record by ID
func (r *tennisPlayerRepository) Delete(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM tennis_players WHERE id = ?`, id)
	return err
}

// FindByTour retrieves all players for a specific tour (ATP or WTA)
func (r *tennisPlayerRepository) FindByTour(ctx context.Context, tour string) ([]models.ProTennisPlayer, error) {
	var players []models.ProTennisPlayer
	err := r.db.SelectContext(ctx, &players, `
		SELECT id, first_name, last_name, common_name, nationality, gender, 
		       current_rank, highest_rank, year_pro, wikipedia_url, hand, 
		       birth_date, birth_place, tour, created_at, updated_at
		FROM tennis_players 
		WHERE tour = ?
		ORDER BY current_rank ASC
	`, tour)
	return players, err
}

// FindATPPlayers retrieves all ATP players
func (r *tennisPlayerRepository) FindATPPlayers(ctx context.Context) ([]models.ProTennisPlayer, error) {
	return r.FindByTour(ctx, "ATP")
}

// FindWTAPlayers retrieves all WTA players
func (r *tennisPlayerRepository) FindWTAPlayers(ctx context.Context) ([]models.ProTennisPlayer, error) {
	return r.FindByTour(ctx, "WTA")
}

// FindByGender retrieves all players of a specific gender
func (r *tennisPlayerRepository) FindByGender(ctx context.Context, gender string) ([]models.ProTennisPlayer, error) {
	var players []models.ProTennisPlayer
	err := r.db.SelectContext(ctx, &players, `
		SELECT id, first_name, last_name, common_name, nationality, gender, 
		       current_rank, highest_rank, year_pro, wikipedia_url, hand, 
		       birth_date, birth_place, tour, created_at, updated_at
		FROM tennis_players 
		WHERE gender = ?
		ORDER BY tour ASC, current_rank ASC
	`, gender)
	return players, err
}

// FindByRankRange retrieves players within a rank range
func (r *tennisPlayerRepository) FindByRankRange(ctx context.Context, minRank, maxRank int) ([]models.ProTennisPlayer, error) {
	var players []models.ProTennisPlayer
	err := r.db.SelectContext(ctx, &players, `
		SELECT id, first_name, last_name, common_name, nationality, gender, 
		       current_rank, highest_rank, year_pro, wikipedia_url, hand, 
		       birth_date, birth_place, tour, created_at, updated_at
		FROM tennis_players 
		WHERE current_rank >= ? AND current_rank <= ?
		ORDER BY tour ASC, current_rank ASC
	`, minRank, maxRank)
	return players, err
}

// FindTopRanked retrieves the top ranked players
func (r *tennisPlayerRepository) FindTopRanked(ctx context.Context, limit int) ([]models.ProTennisPlayer, error) {
	var players []models.ProTennisPlayer
	err := r.db.SelectContext(ctx, &players, `
		SELECT id, first_name, last_name, common_name, nationality, gender, 
		       current_rank, highest_rank, year_pro, wikipedia_url, hand, 
		       birth_date, birth_place, tour, created_at, updated_at
		FROM tennis_players 
		ORDER BY current_rank ASC
		LIMIT ?
	`, limit)
	return players, err
}

// FindByNationality retrieves players by nationality
func (r *tennisPlayerRepository) FindByNationality(ctx context.Context, nationality string) ([]models.ProTennisPlayer, error) {
	var players []models.ProTennisPlayer
	err := r.db.SelectContext(ctx, &players, `
		SELECT id, first_name, last_name, common_name, nationality, gender, 
		       current_rank, highest_rank, year_pro, wikipedia_url, hand, 
		       birth_date, birth_place, tour, created_at, updated_at
		FROM tennis_players 
		WHERE nationality = ?
		ORDER BY tour ASC, current_rank ASC
	`, nationality)
	return players, err
}

// SearchByName retrieves players with names containing the search string
func (r *tennisPlayerRepository) SearchByName(ctx context.Context, name string) ([]models.ProTennisPlayer, error) {
	var players []models.ProTennisPlayer
	searchPattern := "%" + name + "%"
	err := r.db.SelectContext(ctx, &players, `
		SELECT id, first_name, last_name, common_name, nationality, gender, 
		       current_rank, highest_rank, year_pro, wikipedia_url, hand, 
		       birth_date, birth_place, tour, created_at, updated_at
		FROM tennis_players 
		WHERE first_name LIKE ? OR last_name LIKE ? OR common_name LIKE ?
		ORDER BY tour ASC, current_rank ASC
	`, searchPattern, searchPattern, searchPattern)
	return players, err
}

// FindByLastName retrieves players by last name
func (r *tennisPlayerRepository) FindByLastName(ctx context.Context, lastName string) ([]models.ProTennisPlayer, error) {
	var players []models.ProTennisPlayer
	err := r.db.SelectContext(ctx, &players, `
		SELECT id, first_name, last_name, common_name, nationality, gender, 
		       current_rank, highest_rank, year_pro, wikipedia_url, hand, 
		       birth_date, birth_place, tour, created_at, updated_at
		FROM tennis_players 
		WHERE last_name = ?
		ORDER BY tour ASC, current_rank ASC
	`, lastName)
	return players, err
}

// ImportFromJSON imports tennis players from a JSON file
func (r *tennisPlayerRepository) ImportFromJSON(ctx context.Context, filePath string) error {
	// Read the JSON file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read JSON file: %w", err)
	}

	// Parse the JSON structure
	var jsonData struct {
		LastUpdated string `json:"last_updated"`
		ATPPlayers  []struct {
			ID           int    `json:"id"`
			FirstName    string `json:"first_name"`
			LastName     string `json:"last_name"`
			CommonName   string `json:"common_name"`
			Nationality  string `json:"nationality"`
			Gender       string `json:"gender"`
			CurrentRank  int    `json:"current_rank"`
			HighestRank  int    `json:"highest_rank"`
			YearPro      int    `json:"year_pro"`
			WikipediaURL string `json:"wikipedia_url"`
			Hand         string `json:"hand"`
			BirthDate    string `json:"birth_date"`
			BirthPlace   string `json:"birth_place"`
		} `json:"atp_players"`
		WTAPlayers []struct {
			ID           int    `json:"id"`
			FirstName    string `json:"first_name"`
			LastName     string `json:"last_name"`
			CommonName   string `json:"common_name"`
			Nationality  string `json:"nationality"`
			Gender       string `json:"gender"`
			CurrentRank  int    `json:"current_rank"`
			HighestRank  int    `json:"highest_rank"`
			YearPro      int    `json:"year_pro"`
			WikipediaURL string `json:"wikipedia_url"`
			Hand         string `json:"hand"`
			BirthDate    string `json:"birth_date"`
			BirthPlace   string `json:"birth_place"`
		} `json:"wta_players"`
	}

	if err := json.Unmarshal(data, &jsonData); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Clear existing data
	if err := r.ClearAll(ctx); err != nil {
		return fmt.Errorf("failed to clear existing data: %w", err)
	}

	// Import ATP players
	for _, atpPlayer := range jsonData.ATPPlayers {
		player := &models.ProTennisPlayer{
			ID:           atpPlayer.ID,
			FirstName:    atpPlayer.FirstName,
			LastName:     atpPlayer.LastName,
			CommonName:   atpPlayer.CommonName,
			Nationality:  atpPlayer.Nationality,
			Gender:       atpPlayer.Gender,
			CurrentRank:  atpPlayer.CurrentRank,
			HighestRank:  atpPlayer.HighestRank,
			YearPro:      atpPlayer.YearPro,
			WikipediaURL: atpPlayer.WikipediaURL,
			Hand:         atpPlayer.Hand,
			BirthDate:    atpPlayer.BirthDate,
			BirthPlace:   atpPlayer.BirthPlace,
			Tour:         "ATP",
		}
		if err := r.Create(ctx, player); err != nil {
			return fmt.Errorf("failed to create ATP player %s: %w", atpPlayer.CommonName, err)
		}
	}

	// Import WTA players
	for _, wtaPlayer := range jsonData.WTAPlayers {
		player := &models.ProTennisPlayer{
			ID:           wtaPlayer.ID,
			FirstName:    wtaPlayer.FirstName,
			LastName:     wtaPlayer.LastName,
			CommonName:   wtaPlayer.CommonName,
			Nationality:  wtaPlayer.Nationality,
			Gender:       wtaPlayer.Gender,
			CurrentRank:  wtaPlayer.CurrentRank,
			HighestRank:  wtaPlayer.HighestRank,
			YearPro:      wtaPlayer.YearPro,
			WikipediaURL: wtaPlayer.WikipediaURL,
			Hand:         wtaPlayer.Hand,
			BirthDate:    wtaPlayer.BirthDate,
			BirthPlace:   wtaPlayer.BirthPlace,
			Tour:         "WTA",
		}
		if err := r.Create(ctx, player); err != nil {
			return fmt.Errorf("failed to create WTA player %s: %w", wtaPlayer.CommonName, err)
		}
	}

	return nil
}

// ClearAll removes all tennis players
func (r *tennisPlayerRepository) ClearAll(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM tennis_players`)
	return err
}

// CountByTour counts players by tour
func (r *tennisPlayerRepository) CountByTour(ctx context.Context, tour string) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `SELECT COUNT(*) FROM tennis_players WHERE tour = ?`, tour)
	return count, err
}

// CountAll counts all tennis players
func (r *tennisPlayerRepository) CountAll(ctx context.Context) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `SELECT COUNT(*) FROM tennis_players`)
	return count, err
}

// Fantasy Mixed Doubles Repository Methods

// FindAll retrieves all fantasy mixed doubles matches
func (r *fantasyMixedDoublesRepository) FindAll(ctx context.Context) ([]models.FantasyMixedDoubles, error) {
	var matches []models.FantasyMixedDoubles
	err := r.db.SelectContext(ctx, &matches, `
		SELECT id, team_a_woman_id, team_a_man_id, team_b_woman_id, team_b_man_id, auth_token, is_active, created_at, updated_at
		FROM fantasy_mixed_doubles 
		ORDER BY created_at DESC
	`)
	return matches, err
}

// FindByID retrieves a fantasy mixed doubles match by ID
func (r *fantasyMixedDoublesRepository) FindByID(ctx context.Context, id uint) (*models.FantasyMixedDoubles, error) {
	var match models.FantasyMixedDoubles
	err := r.db.GetContext(ctx, &match, `
		SELECT id, team_a_woman_id, team_a_man_id, team_b_woman_id, team_b_man_id, auth_token, is_active, created_at, updated_at
		FROM fantasy_mixed_doubles 
		WHERE id = ?
	`, id)
	if err != nil {
		return nil, err
	}
	return &match, nil
}

// Create inserts a new fantasy mixed doubles match
func (r *fantasyMixedDoublesRepository) Create(ctx context.Context, match *models.FantasyMixedDoubles) error {
	now := time.Now()
	match.CreatedAt = now
	match.UpdatedAt = now

	_, err := r.db.NamedExecContext(ctx, `
		INSERT INTO fantasy_mixed_doubles (
			team_a_woman_id, team_a_man_id, team_b_woman_id, team_b_man_id, 
			auth_token, is_active, created_at, updated_at
		) VALUES (
			:team_a_woman_id, :team_a_man_id, :team_b_woman_id, :team_b_man_id,
			:auth_token, :is_active, :created_at, :updated_at
		)
	`, match)
	return err
}

// Update modifies an existing fantasy mixed doubles match
func (r *fantasyMixedDoublesRepository) Update(ctx context.Context, match *models.FantasyMixedDoubles) error {
	match.UpdatedAt = time.Now()

	_, err := r.db.NamedExecContext(ctx, `
		UPDATE fantasy_mixed_doubles SET
			team_a_woman_id = :team_a_woman_id,
			team_a_man_id = :team_a_man_id,
			team_b_woman_id = :team_b_woman_id,
			team_b_man_id = :team_b_man_id,
			auth_token = :auth_token,
			is_active = :is_active,
			updated_at = :updated_at
		WHERE id = :id
	`, match)
	return err
}

// Delete removes a fantasy mixed doubles match by ID
func (r *fantasyMixedDoublesRepository) Delete(ctx context.Context, id uint) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM fantasy_mixed_doubles WHERE id = ?`, id)
	return err
}

// FindByAuthToken retrieves a match by its authentication token
func (r *fantasyMixedDoublesRepository) FindByAuthToken(ctx context.Context, authToken string) (*models.FantasyMixedDoubles, error) {
	var match models.FantasyMixedDoubles
	err := r.db.GetContext(ctx, &match, `
		SELECT id, team_a_woman_id, team_a_man_id, team_b_woman_id, team_b_man_id, auth_token, is_active, created_at, updated_at
		FROM fantasy_mixed_doubles 
		WHERE auth_token = ?
	`, authToken)
	if err != nil {
		return nil, err
	}
	return &match, nil
}

// FindActive retrieves all active fantasy mixed doubles matches
func (r *fantasyMixedDoublesRepository) FindActive(ctx context.Context) ([]models.FantasyMixedDoubles, error) {
	var matches []models.FantasyMixedDoubles
	err := r.db.SelectContext(ctx, &matches, `
		SELECT id, team_a_woman_id, team_a_man_id, team_b_woman_id, team_b_man_id, auth_token, is_active, created_at, updated_at
		FROM fantasy_mixed_doubles 
		WHERE is_active = 1
		ORDER BY created_at DESC
	`)
	return matches, err
}

// FindInactive retrieves all inactive fantasy mixed doubles matches
func (r *fantasyMixedDoublesRepository) FindInactive(ctx context.Context) ([]models.FantasyMixedDoubles, error) {
	var matches []models.FantasyMixedDoubles
	err := r.db.SelectContext(ctx, &matches, `
		SELECT id, team_a_woman_id, team_a_man_id, team_b_woman_id, team_b_man_id, auth_token, is_active, created_at, updated_at
		FROM fantasy_mixed_doubles 
		WHERE is_active = 0
		ORDER BY created_at DESC
	`)
	return matches, err
}

// FindByTeamAWoman retrieves matches by Team A Woman
func (r *fantasyMixedDoublesRepository) FindByTeamAWoman(ctx context.Context, teamAWomanID int) ([]models.FantasyMixedDoubles, error) {
	var matches []models.FantasyMixedDoubles
	err := r.db.SelectContext(ctx, &matches, `
		SELECT id, team_a_woman_id, team_a_man_id, team_b_woman_id, team_b_man_id, auth_token, is_active, created_at, updated_at
		FROM fantasy_mixed_doubles 
		WHERE team_a_woman_id = ?
		ORDER BY created_at DESC
	`, teamAWomanID)
	return matches, err
}

// FindByTeamAMan retrieves matches by Team A Man
func (r *fantasyMixedDoublesRepository) FindByTeamAMan(ctx context.Context, teamAManID int) ([]models.FantasyMixedDoubles, error) {
	var matches []models.FantasyMixedDoubles
	err := r.db.SelectContext(ctx, &matches, `
		SELECT id, team_a_woman_id, team_a_man_id, team_b_woman_id, team_b_man_id, auth_token, is_active, created_at, updated_at
		FROM fantasy_mixed_doubles 
		WHERE team_a_man_id = ?
		ORDER BY created_at DESC
	`, teamAManID)
	return matches, err
}

// FindByTeamBWoman retrieves matches by Team B Woman
func (r *fantasyMixedDoublesRepository) FindByTeamBWoman(ctx context.Context, teamBWomanID int) ([]models.FantasyMixedDoubles, error) {
	var matches []models.FantasyMixedDoubles
	err := r.db.SelectContext(ctx, &matches, `
		SELECT id, team_a_woman_id, team_a_man_id, team_b_woman_id, team_b_man_id, auth_token, is_active, created_at, updated_at
		FROM fantasy_mixed_doubles 
		WHERE team_b_woman_id = ?
		ORDER BY created_at DESC
	`, teamBWomanID)
	return matches, err
}

// FindByTeamBMan retrieves matches by Team B Man
func (r *fantasyMixedDoublesRepository) FindByTeamBMan(ctx context.Context, teamBManID int) ([]models.FantasyMixedDoubles, error) {
	var matches []models.FantasyMixedDoubles
	err := r.db.SelectContext(ctx, &matches, `
		SELECT id, team_a_woman_id, team_a_man_id, team_b_woman_id, team_b_man_id, auth_token, is_active, created_at, updated_at
		FROM fantasy_mixed_doubles 
		WHERE team_b_man_id = ?
		ORDER BY created_at DESC
	`, teamBManID)
	return matches, err
}

// GenerateRandomMatches creates random ATP/WTA player matches
func (r *fantasyMixedDoublesRepository) GenerateRandomMatches(ctx context.Context, count int) error {
	// Get ATP and WTA players
	var atpPlayers []models.ProTennisPlayer
	err := r.db.SelectContext(ctx, &atpPlayers, `
		SELECT id, first_name, last_name, common_name, nationality, gender, 
		       current_rank, highest_rank, year_pro, wikipedia_url, hand, 
		       birth_date, birth_place, tour, created_at, updated_at
		FROM tennis_players 
		WHERE tour = 'ATP'
		ORDER BY RANDOM()
		LIMIT ?
	`, count*2) // Need 2 ATP players per match
	if err != nil {
		return fmt.Errorf("failed to get ATP players: %w", err)
	}

	var wtaPlayers []models.ProTennisPlayer
	err = r.db.SelectContext(ctx, &wtaPlayers, `
		SELECT id, first_name, last_name, common_name, nationality, gender, 
		       current_rank, highest_rank, year_pro, wikipedia_url, hand, 
		       birth_date, birth_place, tour, created_at, updated_at
		FROM tennis_players 
		WHERE tour = 'WTA'
		ORDER BY RANDOM()
		LIMIT ?
	`, count*2) // Need 2 WTA players per match
	if err != nil {
		return fmt.Errorf("failed to get WTA players: %w", err)
	}

	// Create matches
	for i := 0; i < count && i*2+1 < len(atpPlayers) && i*2+1 < len(wtaPlayers); i++ {
		teamAWoman := wtaPlayers[i*2]
		teamAMan := atpPlayers[i*2]
		teamBWoman := wtaPlayers[i*2+1]
		teamBMan := atpPlayers[i*2+1]

		// Generate auth token
		authToken := r.GenerateAuthToken(&teamAWoman, &teamAMan, &teamBWoman, &teamBMan)

		// Create match
		match := &models.FantasyMixedDoubles{
			TeamAWomanID: teamAWoman.ID,
			TeamAManID:   teamAMan.ID,
			TeamBWomanID: teamBWoman.ID,
			TeamBManID:   teamBMan.ID,
			AuthToken:    authToken,
			IsActive:     true,
		}

		if err := r.Create(ctx, match); err != nil {
			return fmt.Errorf("failed to create match %d: %w", i+1, err)
		}
	}

	return nil
}

// GenerateAuthToken creates an authentication token from four tennis players
func (r *fantasyMixedDoublesRepository) GenerateAuthToken(teamAWoman, teamAMan, teamBWoman, teamBMan *models.ProTennisPlayer) string {
	// Regular expression to match any character that is not alphanumeric or hyphen
	urlUnsafeChars := regexp.MustCompile(`[^a-zA-Z0-9\-]`)

	// Helper function to clean surname by converting URL-unsafe characters to dashes
	cleanSurname := func(surname string) string {
		cleaned := strings.TrimSpace(surname)
		// Replace all URL-unsafe characters with dashes
		cleaned = urlUnsafeChars.ReplaceAllString(cleaned, "-")

		// Remove consecutive dashes and trim leading/trailing dashes
		consecutiveDashes := regexp.MustCompile(`-+`)
		cleaned = consecutiveDashes.ReplaceAllString(cleaned, "-")
		cleaned = strings.Trim(cleaned, "-")

		return cleaned
	}

	// Clean and concatenate surnames with underscore separators
	surname1 := cleanSurname(teamAWoman.LastName)
	surname2 := cleanSurname(teamAMan.LastName)
	surname3 := cleanSurname(teamBWoman.LastName)
	surname4 := cleanSurname(teamBMan.LastName)
	return fmt.Sprintf("%s_%s_%s_%s", surname1, surname2, surname3, surname4)
}

// CountActive counts active pairings
func (r *fantasyMixedDoublesRepository) CountActive(ctx context.Context) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `SELECT COUNT(*) FROM fantasy_mixed_doubles WHERE is_active = 1`)
	return count, err
}

// CountAll counts all pairings
func (r *fantasyMixedDoublesRepository) CountAll(ctx context.Context) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `SELECT COUNT(*) FROM fantasy_mixed_doubles`)
	return count, err
}
