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

// PlayerGender represents the gender of a tennis player
type PlayerGender string

const (
	PlayerGenderMen     PlayerGender = "Men"
	PlayerGenderWomen   PlayerGender = "Women"
	PlayerGenderUnknown PlayerGender = "Unknown"
)

// PlayerReportingPrivacy represents the reporting privacy level of a tennis player
type PlayerReportingPrivacy string

const (
	PlayerReportingVisible PlayerReportingPrivacy = "visible" // Player appears on public reports
	PlayerReportingHidden  PlayerReportingPrivacy = "hidden"  // Player is hidden from public reports
)

// Player represents a player in the tennis league
type Player struct {
	ID               string                 `json:"id" db:"id"` // UUID for player identification
	FirstName        string                 `json:"first_name" db:"first_name"`
	LastName         string                 `json:"last_name" db:"last_name"`
	PreferredName    *string                `json:"preferred_name,omitempty" db:"preferred_name"`     // Optional preferred name for display
	Gender           PlayerGender           `json:"gender" db:"gender"`                               // Player gender: Men, Women, or Unknown
	ReportingPrivacy PlayerReportingPrivacy `json:"reporting_privacy" db:"reporting_privacy"`         // Controls visibility on public reports
	ClubID           uint                   `json:"club_id" db:"club_id"`                             // Player belongs to a club, not directly to a team
	FantasyMatchID   *uint                  `json:"fantasy_match_id,omitempty" db:"fantasy_match_id"` // Links player to fantasy mixed doubles match for auth
	CreatedAt        time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at" db:"updated_at"`
	Teams            []uint                 `json:"teams,omitempty"` // Player can be part of multiple teams through PlayerTeam
}

// PreferredNameRequestStatus represents the status of a preferred name request
type PreferredNameRequestStatus string

const (
	PreferredNamePending  PreferredNameRequestStatus = "Pending"
	PreferredNameApproved PreferredNameRequestStatus = "Approved"
	PreferredNameRejected PreferredNameRequestStatus = "Rejected"
)

// PreferredNameRequest represents a request from a player to set their preferred name
type PreferredNameRequest struct {
	ID            uint                       `json:"id" db:"id"`
	PlayerID      string                     `json:"player_id" db:"player_id"`               // UUID reference to player
	RequestedName string                     `json:"requested_name" db:"requested_name"`     // The preferred name being requested
	Status        PreferredNameRequestStatus `json:"status" db:"status"`                     // Current status of the request
	AdminNotes    *string                    `json:"admin_notes,omitempty" db:"admin_notes"` // Optional notes from admin
	ApprovedBy    *string                    `json:"approved_by,omitempty" db:"approved_by"` // Admin username who processed the request
	CreatedAt     time.Time                  `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time                  `json:"updated_at" db:"updated_at"`
	ProcessedAt   *time.Time                 `json:"processed_at,omitempty" db:"processed_at"` // When admin took action
	Player        *Player                    `json:"player,omitempty"`                         // Associated player (for joins)
}

// Club represents a tennis club that has players and teams
type Club struct {
	ID            uint      `json:"id" db:"id"`
	Name          string    `json:"name" db:"name"`
	Address       string    `json:"address" db:"address"`
	Website       string    `json:"website" db:"website"`
	PhoneNumber   string    `json:"phone_number" db:"phone_number"`
	Latitude      *float64  `json:"latitude,omitempty" db:"latitude"`
	Longitude     *float64  `json:"longitude,omitempty" db:"longitude"`
	Postcode      *string   `json:"postcode,omitempty" db:"postcode"`
	AddressLine1  *string   `json:"address_line_1,omitempty" db:"address_line_1"`
	AddressLine2  *string   `json:"address_line_2,omitempty" db:"address_line_2"`
	City          *string   `json:"city,omitempty" db:"city"`
	CourtSurface  *string   `json:"court_surface,omitempty" db:"court_surface"`
	CourtCount    *int      `json:"court_count,omitempty" db:"court_count"`
	ParkingInfo   *string   `json:"parking_info,omitempty" db:"parking_info"`
	TransportInfo *string   `json:"transport_info,omitempty" db:"transport_info"`
	Tips          *string   `json:"tips,omitempty" db:"tips"`
	GoogleMapsURL *string   `json:"google_maps_url,omitempty" db:"google_maps_url"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
	Players       []Player  `json:"players,omitempty"`
	Teams         []Team    `json:"teams,omitempty"`
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

// RescheduledReason represents the reason for rescheduling a fixture
type RescheduledReason string

const (
	WeatherReason     RescheduledReason = "Weather"           // Weather-related rescheduling
	CourtAvailability RescheduledReason = "CourtAvailability" // Court not available
	OtherReason       RescheduledReason = "Other"             // Other reasons
)

// Fixture represents a scheduled match between two teams
type Fixture struct {
	ID                  uint               `json:"id" db:"id"`
	HomeTeamID          uint               `json:"home_team_id" db:"home_team_id"`
	AwayTeamID          uint               `json:"away_team_id" db:"away_team_id"`
	DivisionID          uint               `json:"division_id" db:"division_id"`
	SeasonID            uint               `json:"season_id" db:"season_id"` // Reference to season
	WeekID              uint               `json:"week_id" db:"week_id"`     // Reference to week
	ScheduledDate       time.Time          `json:"scheduled_date" db:"scheduled_date"`
	VenueLocation       string             `json:"venue_location" db:"venue_location"`
	Status              FixtureStatus      `json:"status" db:"status"`
	CompletedDate       *time.Time         `json:"completed_date,omitempty" db:"completed_date"`                 // When fixture was completed
	DayCaptainID        *string            `json:"day_captain_id,omitempty" db:"day_captain_id"`                 // Optional day captain for this fixture (UUID)
	ExternalMatchCardID *int               `json:"external_match_card_id,omitempty" db:"external_match_card_id"` // BHPLTA match card ID
	Notes               string             `json:"notes" db:"notes"`
	PreviousDates       []time.Time        `json:"previous_dates,omitempty" db:"previous_dates"`         // Previous scheduled dates (stored as JSON)
	RescheduledReason   *RescheduledReason `json:"rescheduled_reason,omitempty" db:"rescheduled_reason"` // Reason for rescheduling
	VenueClubID         *uint              `json:"venue_club_id,omitempty" db:"venue_club_id"`           // Per-fixture venue override (FK to clubs)
	CreatedAt           time.Time          `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time          `json:"updated_at" db:"updated_at"`
	Matchups            []Matchup          `json:"matchups,omitempty"`
	SelectedPlayers     []FixturePlayer    `json:"selected_players,omitempty"`
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

// ConcededBy indicates which side conceded
type ConcededBy string

const (
	ConcededNone ConcededBy = ""
	ConcededHome ConcededBy = "Home"
	ConcededAway ConcededBy = "Away"
)

// Matchup represents one of the four matchups in a fixture
type Matchup struct {
	ID             uint          `json:"id" db:"id"`
	FixtureID      uint          `json:"fixture_id" db:"fixture_id"`
	Type           MatchupType   `json:"type" db:"type"`
	Status         MatchupStatus `json:"status" db:"status"`
	HomeScore      int           `json:"home_score" db:"home_score"`         // Matchup points for home team (2=win, 1=draw, 0=loss)
	AwayScore      int           `json:"away_score" db:"away_score"`         // Matchup points for away team (2=win, 1=draw, 0=loss)
	HomeSet1       *int          `json:"home_set1,omitempty" db:"home_set1"` // Home team score in set 1
	AwaySet1       *int          `json:"away_set1,omitempty" db:"away_set1"` // Away team score in set 1
	HomeSet2       *int          `json:"home_set2,omitempty" db:"home_set2"` // Home team score in set 2
	AwaySet2       *int          `json:"away_set2,omitempty" db:"away_set2"` // Away team score in set 2
	HomeSet3       *int          `json:"home_set3,omitempty" db:"home_set3"` // Home team score in set 3
	AwaySet3       *int          `json:"away_set3,omitempty" db:"away_set3"` // Away team score in set 3
	Notes          string        `json:"notes" db:"notes"`
	ManagingTeamID *uint         `json:"managing_team_id,omitempty" db:"managing_team_id"` // Which team is managing this matchup (for derby matches)
	ConcededBy     *ConcededBy   `json:"conceded_by,omitempty" db:"conceded_by"`
	CreatedAt      time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at" db:"updated_at"`
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
	ID             uint      `json:"id" db:"id"`
	FixtureID      uint      `json:"fixture_id" db:"fixture_id"`
	PlayerID       string    `json:"player_id" db:"player_id"`                         // UUID reference to player
	IsHome         bool      `json:"is_home" db:"is_home"`                             // true for home team, false for away team
	Position       int       `json:"position" db:"position"`                           // Order of selection (1-8 typically)
	ManagingTeamID *uint     `json:"managing_team_id,omitempty" db:"managing_team_id"` // Which team is managing this player selection (for derby matches)
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
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
