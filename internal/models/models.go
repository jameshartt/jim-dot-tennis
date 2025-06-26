package models

import (
	"time"
)

// Season represents a specific playing season
type Season struct {
	ID        uint      `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`             // e.g., "Summer 2023"
	Year      int       `json:"year" db:"year"`             // The year when the season occurs
	StartDate time.Time `json:"start_date" db:"start_date"` // When the season starts
	EndDate   time.Time `json:"end_date" db:"end_date"`     // When the season ends
	IsActive  bool      `json:"is_active" db:"is_active"`   // Whether this is the current active season
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
	Leagues   []League  `json:"leagues,omitempty"` // Leagues associated with this season
	Weeks     []Week    `json:"weeks,omitempty"`   // Weeks in this season
}

// Week represents a specific week within a season
type Week struct {
	ID         uint      `json:"id" db:"id"`
	WeekNumber int       `json:"week_number" db:"week_number"` // Week number within the season (1, 2, 3, etc.)
	SeasonID   uint      `json:"season_id" db:"season_id"`     // Reference to season
	StartDate  time.Time `json:"start_date" db:"start_date"`   // When the week starts
	EndDate    time.Time `json:"end_date" db:"end_date"`       // When the week ends
	Name       string    `json:"name" db:"name"`               // Optional name like "Week 1", "Semi-Finals", etc.
	IsActive   bool      `json:"is_active" db:"is_active"`     // Whether this is the current active week
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
	Fixtures   []Fixture `json:"fixtures,omitempty"` // Fixtures in this week
}

// Player represents a player in the tennis league
type Player struct {
	ID             string    `json:"id" db:"id"` // UUID for player identification
	FirstName      string    `json:"first_name" db:"first_name"`
	LastName       string    `json:"last_name" db:"last_name"`
	Email          string    `json:"email" db:"email"`
	Phone          string    `json:"phone" db:"phone"`
	ClubID         uint      `json:"club_id" db:"club_id"`                             // Player belongs to a club, not directly to a team
	FantasyMatchID *uint     `json:"fantasy_match_id,omitempty" db:"fantasy_match_id"` // Links player to fantasy mixed doubles match for auth
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
	Teams          []uint    `json:"teams,omitempty"` // Player can be part of multiple teams through PlayerTeam
}

// Club represents a tennis club that has players and teams
type Club struct {
	ID          uint      `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Address     string    `json:"address" db:"address"`
	Website     string    `json:"website" db:"website"`
	PhoneNumber string    `json:"phone_number" db:"phone_number"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
	Players     []Player  `json:"players,omitempty"`
	Teams       []Team    `json:"teams,omitempty"`
}

// LeagueSeason represents the many-to-many relationship between leagues and seasons
type LeagueSeason struct {
	ID        uint      `json:"id" db:"id"`
	LeagueID  uint      `json:"league_id" db:"league_id"`
	SeasonID  uint      `json:"season_id" db:"season_id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// PlayerTeam represents a player's association with a team, including the season
// This allows players to move between teams over time
type PlayerTeam struct {
	ID        uint      `json:"id" db:"id"`
	PlayerID  string    `json:"player_id" db:"player_id"` // UUID reference to player
	TeamID    uint      `json:"team_id" db:"team_id"`
	SeasonID  uint      `json:"season_id" db:"season_id"` // Reference to season
	IsActive  bool      `json:"is_active" db:"is_active"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// CaptainRole defines the type of captain role
type CaptainRole string

const (
	TeamCaptain CaptainRole = "Team" // Main team captain
	DayCaptain  CaptainRole = "Day"  // Day captain responsible for scores
)

// Captain represents a team captain with additional permissions
type Captain struct {
	ID        uint        `json:"id" db:"id"`
	PlayerID  string      `json:"player_id" db:"player_id"` // UUID reference to player
	TeamID    uint        `json:"team_id" db:"team_id"`
	Role      CaptainRole `json:"role" db:"role"`           // Type of captaincy role
	SeasonID  uint        `json:"season_id" db:"season_id"` // Reference to season
	IsActive  bool        `json:"is_active" db:"is_active"`
	CreatedAt time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt time.Time   `json:"updated_at" db:"updated_at"`
}

// Team represents a tennis team in the league
type Team struct {
	ID         uint         `json:"id" db:"id"`
	Name       string       `json:"name" db:"name"`
	ClubID     uint         `json:"club_id" db:"club_id"` // Team belongs to a club
	DivisionID uint         `json:"division_id" db:"division_id"`
	SeasonID   uint         `json:"season_id" db:"season_id"` // Reference to season
	CreatedAt  time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at" db:"updated_at"`
	Players    []PlayerTeam `json:"players,omitempty"`
	Captains   []Captain    `json:"captains,omitempty"`
}

// Division represents a division in the league
type Division struct {
	ID              uint      `json:"id" db:"id"`
	Name            string    `json:"name" db:"name"`
	Level           int       `json:"level" db:"level"`
	PlayDay         string    `json:"play_day" db:"play_day"` // Day of the week: "Monday", "Tuesday", etc.
	LeagueID        uint      `json:"league_id" db:"league_id"`
	SeasonID        uint      `json:"season_id" db:"season_id"`                   // Reference to season
	MaxTeamsPerClub int       `json:"max_teams_per_club" db:"max_teams_per_club"` // Max teams allowed from same club (default 2)
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
	Teams           []Team    `json:"teams,omitempty"`
	Fixtures        []Fixture `json:"fixtures,omitempty"`
}

// LeagueType represents the type of league (Parks, Club, etc.)
type LeagueType string

const (
	ParksLeague LeagueType = "Parks"
	ClubLeague  LeagueType = "Club"
)

// League represents the overall tennis league
type League struct {
	ID        uint       `json:"id" db:"id"`
	Name      string     `json:"name" db:"name"`
	Type      LeagueType `json:"type" db:"type"`
	Year      int        `json:"year" db:"year"`
	Region    string     `json:"region" db:"region"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
	Divisions []Division `json:"divisions,omitempty"`
	Seasons   []Season   `json:"seasons,omitempty"` // Seasons associated with this league
}

// FixtureStatus represents the status of a fixture
type FixtureStatus string

const (
	Scheduled  FixtureStatus = "Scheduled"  // Fixture is scheduled but not played
	InProgress FixtureStatus = "InProgress" // Fixture has started but not completed
	Completed  FixtureStatus = "Completed"  // Fixture is completed
	Cancelled  FixtureStatus = "Cancelled"  // Fixture was cancelled
	Postponed  FixtureStatus = "Postponed"  // Fixture was postponed
)

// Fixture represents a scheduled match between two teams
type Fixture struct {
	ID              uint            `json:"id" db:"id"`
	HomeTeamID      uint            `json:"home_team_id" db:"home_team_id"`
	AwayTeamID      uint            `json:"away_team_id" db:"away_team_id"`
	DivisionID      uint            `json:"division_id" db:"division_id"`
	SeasonID        uint            `json:"season_id" db:"season_id"` // Reference to season
	WeekID          uint            `json:"week_id" db:"week_id"`     // Reference to week
	ScheduledDate   time.Time       `json:"scheduled_date" db:"scheduled_date"`
	VenueLocation   string          `json:"venue_location" db:"venue_location"`
	Status          FixtureStatus   `json:"status" db:"status"`
	CompletedDate   *time.Time      `json:"completed_date,omitempty" db:"completed_date"` // When fixture was completed
	DayCaptainID    *string         `json:"day_captain_id,omitempty" db:"day_captain_id"` // Optional day captain for this fixture (UUID)
	Notes           string          `json:"notes" db:"notes"`
	CreatedAt       time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at" db:"updated_at"`
	Matchups        []Matchup       `json:"matchups,omitempty"`
	SelectedPlayers []FixturePlayer `json:"selected_players,omitempty"`
}

// MatchupType represents the type of matchup (Men's, Women's, 1st Mixed, 2nd Mixed)
type MatchupType string

const (
	Mens        MatchupType = "Mens"
	Womens      MatchupType = "Womens"
	FirstMixed  MatchupType = "1st Mixed"
	SecondMixed MatchupType = "2nd Mixed"
)

// MatchupStatus represents the status of a matchup
type MatchupStatus string

const (
	Pending   MatchupStatus = "Pending"   // Not started or players not selected
	Playing   MatchupStatus = "Playing"   // Currently in progress
	Finished  MatchupStatus = "Finished"  // Completed with scores
	Defaulted MatchupStatus = "Defaulted" // One team didn't show or couldn't field players
)

// Matchup represents one of the four matchups in a fixture
type Matchup struct {
	ID        uint          `json:"id" db:"id"`
	FixtureID uint          `json:"fixture_id" db:"fixture_id"`
	Type      MatchupType   `json:"type" db:"type"`
	Status    MatchupStatus `json:"status" db:"status"`
	HomeScore int           `json:"home_score" db:"home_score"`
	AwayScore int           `json:"away_score" db:"away_score"`
	Notes     string        `json:"notes" db:"notes"`
	CreatedAt time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt time.Time     `json:"updated_at" db:"updated_at"`
}

// MatchupPlayer represents a player participating in a specific matchup
type MatchupPlayer struct {
	ID        uint      `json:"id" db:"id"`
	MatchupID uint      `json:"matchup_id" db:"matchup_id"`
	PlayerID  string    `json:"player_id" db:"player_id"` // UUID reference to player
	IsHome    bool      `json:"is_home" db:"is_home"`     // true for home team, false for away team
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// FixturePlayer represents a player selected for a fixture (holding pattern before matchup assignment)
type FixturePlayer struct {
	ID        uint      `json:"id" db:"id"`
	FixtureID uint      `json:"fixture_id" db:"fixture_id"`
	PlayerID  string    `json:"player_id" db:"player_id"` // UUID reference to player
	IsHome    bool      `json:"is_home" db:"is_home"`     // true for home team, false for away team
	Position  int       `json:"position" db:"position"`   // Order of selection (1-8 typically)
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// ProTennisPlayer represents a professional tennis player from ATP/WTA
type ProTennisPlayer struct {
	ID           int       `json:"id" db:"id"`
	FirstName    string    `json:"first_name" db:"first_name"`
	LastName     string    `json:"last_name" db:"last_name"`
	CommonName   string    `json:"common_name" db:"common_name"`
	Nationality  string    `json:"nationality" db:"nationality"`
	Gender       string    `json:"gender" db:"gender"`
	CurrentRank  int       `json:"current_rank" db:"current_rank"`
	HighestRank  int       `json:"highest_rank" db:"highest_rank"`
	YearPro      int       `json:"year_pro" db:"year_pro"`
	WikipediaURL string    `json:"wikipedia_url" db:"wikipedia_url"`
	Hand         string    `json:"hand" db:"hand"`
	BirthDate    string    `json:"birth_date" db:"birth_date"`
	BirthPlace   string    `json:"birth_place" db:"birth_place"`
	Tour         string    `json:"tour" db:"tour"` // "ATP" or "WTA"
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// FantasyMixedDoubles represents a fantasy mixed doubles match for player authentication
type FantasyMixedDoubles struct {
	ID           uint      `json:"id" db:"id"`
	TeamAWomanID int       `json:"team_a_woman_id" db:"team_a_woman_id"` // WTA player on Team A
	TeamAManID   int       `json:"team_a_man_id" db:"team_a_man_id"`     // ATP player on Team A
	TeamBWomanID int       `json:"team_b_woman_id" db:"team_b_woman_id"` // WTA player on Team B
	TeamBManID   int       `json:"team_b_man_id" db:"team_b_man_id"`     // ATP player on Team B
	AuthToken    string    `json:"auth_token" db:"auth_token"`           // Concatenated surnames with underscore
	IsActive     bool      `json:"is_active" db:"is_active"`             // Whether this match is currently active
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}
