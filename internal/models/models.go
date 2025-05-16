package models

import (
	"time"
)

// Player represents a player in the tennis league
type Player struct {
	ID        uint      `json:"id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone"`
	TeamID    uint      `json:"team_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Captain represents a team captain with additional permissions
type Captain struct {
	ID        uint      `json:"id"`
	PlayerID  uint      `json:"player_id"`
	TeamID    uint      `json:"team_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Team represents a tennis team in the league
type Team struct {
	ID         uint      `json:"id"`
	Name       string    `json:"name"`
	Club       string    `json:"club"`
	DivisionID uint      `json:"division_id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	Players    []Player  `json:"players,omitempty"`
	Captains   []Captain `json:"captains,omitempty"`
}

// Division represents a division in the league
type Division struct {
	ID           uint      `json:"id"`
	Name         string    `json:"name"`
	Level        int       `json:"level"`
	PlayDay      string    `json:"play_day"` // Day of the week: "Monday", "Tuesday", etc.
	Season       string    `json:"season"`   // e.g., "Summer 2023"
	LeagueID     uint      `json:"league_id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Teams        []Team    `json:"teams,omitempty"`
	Fixtures     []Fixture `json:"fixtures,omitempty"`
}

// League represents the overall tennis league
type League struct {
	ID         uint       `json:"id"`
	Name       string     `json:"name"`
	Region     string     `json:"region"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	Divisions  []Division `json:"divisions,omitempty"`
}

// Fixture represents a scheduled match between two teams
type Fixture struct {
	ID              uint      `json:"id"`
	HomeTeamID      uint      `json:"home_team_id"`
	AwayTeamID      uint      `json:"away_team_id"`
	DivisionID      uint      `json:"division_id"`
	ScheduledDate   time.Time `json:"scheduled_date"`
	VenueLocation   string    `json:"venue_location"`
	IsCompleted     bool      `json:"is_completed"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	Matchups        []Matchup `json:"matchups,omitempty"`
}

// MatchupType represents the type of matchup (Men's, Women's, 1st Mixed, 2nd Mixed)
type MatchupType string

const (
	Mens       MatchupType = "Mens"
	Womens     MatchupType = "Womens"
	FirstMixed MatchupType = "1st Mixed"
	SecondMixed MatchupType = "2nd Mixed"
)

// Matchup represents one of the four matchups in a fixture
type Matchup struct {
	ID            uint        `json:"id"`
	FixtureID     uint        `json:"fixture_id"`
	Type          MatchupType `json:"type"`
	HomeScore     int         `json:"home_score"`
	AwayScore     int         `json:"away_score"`
	HomePlayers   []uint      `json:"home_players"` // Array of Player IDs
	AwayPlayers   []uint      `json:"away_players"` // Array of Player IDs
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
} 