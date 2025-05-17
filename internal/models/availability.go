package models

import (
	"time"
)

// AvailabilityStatus defines the availability states for a player
type AvailabilityStatus string

const (
	// Available indicates a player is available to play
	Available AvailabilityStatus = "Available"
	// Unavailable indicates a player is not available to play
	Unavailable AvailabilityStatus = "Unavailable"
	// Unknown indicates a player's availability is not yet determined
	Unknown AvailabilityStatus = "Unknown"
)

// PlayerDivision represents which divisions a player is eligible to play in
type PlayerDivision struct {
	ID         uint      `json:"id"`
	PlayerID   string    `json:"player_id"`    // UUID reference to player
	DivisionID uint      `json:"division_id"`
	SeasonID   uint      `json:"season_id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// PlayerGeneralAvailability represents a player's default availability for a day of the week
type PlayerGeneralAvailability struct {
	ID        uint              `json:"id"`
	PlayerID  string            `json:"player_id"`  // UUID reference to player
	DayOfWeek string            `json:"day_of_week"` // Monday, Tuesday, etc.
	Status    AvailabilityStatus `json:"status"`
	SeasonID  uint              `json:"season_id"`
	Notes     string            `json:"notes,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// PlayerAvailabilityException represents a specific date range when a player's
// availability differs from their general availability
type PlayerAvailabilityException struct {
	ID        uint              `json:"id"`
	PlayerID  string            `json:"player_id"`    // UUID reference to player
	Status    AvailabilityStatus `json:"status"`
	StartDate time.Time         `json:"start_date"`
	EndDate   time.Time         `json:"end_date"`
	Reason    string            `json:"reason,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// PlayerFixtureAvailability represents a player's availability for a specific fixture
type PlayerFixtureAvailability struct {
	ID        uint              `json:"id"`
	PlayerID  string            `json:"player_id"`    // UUID reference to player
	FixtureID uint              `json:"fixture_id"`
	Status    AvailabilityStatus `json:"status"`
	Notes     string            `json:"notes,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// Helper functions for determining effective availability

// CalculateAvailabilityForFixture determines a player's effective availability for a fixture
// by considering fixture-specific availability, date exceptions, and general day-of-week availability
func CalculateAvailabilityForFixture(playerID string, fixtureID uint, fixtureDate time.Time) AvailabilityStatus {
	// Note: This would be implemented with actual database queries in a real application
	// This is a placeholder function that shows the logic flow
	
	// 1. Check for fixture-specific availability first (highest priority)
	// fixtureAvailability := GetPlayerFixtureAvailability(playerID, fixtureID)
	// if fixtureAvailability != Unknown {
	//    return fixtureAvailability
	// }
	
	// 2. Check for date-specific exceptions
	// exception := GetPlayerDateException(playerID, fixtureDate)
	// if exception != nil {
	//    return exception.Status
	// }
	
	// 3. Fall back to general day-of-week availability
	// dayOfWeek := fixtureDate.Weekday().String()
	// generalAvailability := GetPlayerGeneralAvailability(playerID, dayOfWeek)
	// if generalAvailability != nil {
	//    return generalAvailability.Status
	// }
	
	// 4. Default to Unknown if nothing is specified
	return Unknown
} 