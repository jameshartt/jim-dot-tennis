package models

import (
	"time"
)

// Season represents a specific playing season
type Season struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`       // e.g., "Summer 2023"
	Year      int       `json:"year"`       // The year when the season occurs
	StartDate time.Time `json:"start_date"` // When the season starts
	EndDate   time.Time `json:"end_date"`   // When the season ends
	IsActive  bool      `json:"is_active"`  // Whether this is the current active season
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Leagues   []League  `json:"leagues,omitempty"` // Leagues associated with this season
}

// Player represents a player in the tennis league
type Player struct {
	ID        string    `json:"id"`           // UUID for player identification
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone"`
	ClubID    uint      `json:"club_id"` // Player belongs to a club, not directly to a team
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Teams     []uint    `json:"teams,omitempty"` // Player can be part of multiple teams through PlayerTeam
}

// Club represents a tennis club that has players and teams
type Club struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	Address     string    `json:"address"`
	Website     string    `json:"website"`
	PhoneNumber string    `json:"phone_number"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Players     []Player  `json:"players,omitempty"`
	Teams       []Team    `json:"teams,omitempty"`
}

// LeagueSeason represents the many-to-many relationship between leagues and seasons
type LeagueSeason struct {
	ID        uint      `json:"id"`
	LeagueID  uint      `json:"league_id"`
	SeasonID  uint      `json:"season_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PlayerTeam represents a player's association with a team, including the season
// This allows players to move between teams over time
type PlayerTeam struct {
	ID        uint      `json:"id"`
	PlayerID  string    `json:"player_id"`    // UUID reference to player
	TeamID    uint      `json:"team_id"`
	SeasonID  uint      `json:"season_id"`    // Reference to season
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CaptainRole defines the type of captain role
type CaptainRole string

const (
	TeamCaptain CaptainRole = "Team"   // Main team captain 
	DayCaptain  CaptainRole = "Day"    // Day captain responsible for scores
)

// Captain represents a team captain with additional permissions
type Captain struct {
	ID        uint       `json:"id"`
	PlayerID  string     `json:"player_id"`    // UUID reference to player
	TeamID    uint       `json:"team_id"`
	Role      CaptainRole `json:"role"`        // Type of captaincy role
	SeasonID  uint       `json:"season_id"`    // Reference to season
	IsActive  bool       `json:"is_active"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// Team represents a tennis team in the league
type Team struct {
	ID         uint         `json:"id"`
	Name       string       `json:"name"`
	ClubID     uint         `json:"club_id"`     // Team belongs to a club
	DivisionID uint         `json:"division_id"`
	SeasonID   uint         `json:"season_id"`   // Reference to season
	CreatedAt  time.Time    `json:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at"`
	Players    []PlayerTeam `json:"players,omitempty"`
	Captains   []Captain    `json:"captains,omitempty"`
}

// Division represents a division in the league
type Division struct {
	ID             uint      `json:"id"`
	Name           string    `json:"name"`
	Level          int       `json:"level"`
	PlayDay        string    `json:"play_day"`           // Day of the week: "Monday", "Tuesday", etc.
	LeagueID       uint      `json:"league_id"`
	SeasonID       uint      `json:"season_id"`          // Reference to season 
	MaxTeamsPerClub int      `json:"max_teams_per_club"` // Max teams allowed from same club (default 2)
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Teams          []Team    `json:"teams,omitempty"`
	Fixtures       []Fixture `json:"fixtures,omitempty"`
}

// LeagueType represents the type of league (Parks, Club, etc.)
type LeagueType string

const (
	ParksLeague LeagueType = "Parks"
	ClubLeague  LeagueType = "Club"
)

// League represents the overall tennis league
type League struct {
	ID         uint       `json:"id"`
	Name       string     `json:"name"`
	Type       LeagueType `json:"type"`
	Year       int        `json:"year"`
	Region     string     `json:"region"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	Divisions  []Division `json:"divisions,omitempty"`
	Seasons    []Season   `json:"seasons,omitempty"` // Seasons associated with this league
}

// FixtureStatus represents the status of a fixture
type FixtureStatus string

const (
	Scheduled   FixtureStatus = "Scheduled"   // Fixture is scheduled but not played
	InProgress  FixtureStatus = "InProgress"  // Fixture has started but not completed
	Completed   FixtureStatus = "Completed"   // Fixture is completed
	Cancelled   FixtureStatus = "Cancelled"   // Fixture was cancelled
	Postponed   FixtureStatus = "Postponed"   // Fixture was postponed
)

// Fixture represents a scheduled match between two teams
type Fixture struct {
	ID              uint          `json:"id"`
	HomeTeamID      uint          `json:"home_team_id"`
	AwayTeamID      uint          `json:"away_team_id"`
	DivisionID      uint          `json:"division_id"`
	SeasonID        uint          `json:"season_id"`      // Reference to season
	ScheduledDate   time.Time     `json:"scheduled_date"`
	VenueLocation   string        `json:"venue_location"`
	Status          FixtureStatus `json:"status"`
	CompletedDate   *time.Time    `json:"completed_date,omitempty"` // When fixture was completed
	DayCaptainID    *string       `json:"day_captain_id,omitempty"` // Optional day captain for this fixture (UUID)
	Notes           string        `json:"notes"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
	Matchups        []Matchup     `json:"matchups,omitempty"`
}

// MatchupType represents the type of matchup (Men's, Women's, 1st Mixed, 2nd Mixed)
type MatchupType string

const (
	Mens       MatchupType = "Mens"
	Womens     MatchupType = "Womens"
	FirstMixed MatchupType = "1st Mixed"
	SecondMixed MatchupType = "2nd Mixed"
)

// MatchupStatus represents the status of a matchup
type MatchupStatus string

const (
	Pending    MatchupStatus = "Pending"    // Not started or players not selected
	Playing    MatchupStatus = "Playing"    // Currently in progress
	Finished   MatchupStatus = "Finished"   // Completed with scores
	Defaulted  MatchupStatus = "Defaulted"  // One team didn't show or couldn't field players
)

// Matchup represents one of the four matchups in a fixture
type Matchup struct {
	ID            uint          `json:"id"`
	FixtureID     uint          `json:"fixture_id"`
	Type          MatchupType   `json:"type"`
	Status        MatchupStatus `json:"status"`
	HomeScore     int           `json:"home_score"`
	AwayScore     int           `json:"away_score"`
	Notes         string        `json:"notes"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

// MatchupPlayer represents a player participating in a specific matchup
type MatchupPlayer struct {
	ID        uint      `json:"id"`
	MatchupID uint      `json:"matchup_id"`
	PlayerID  string    `json:"player_id"`    // UUID reference to player
	IsHome    bool      `json:"is_home"` // true for home team, false for away team
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
} 