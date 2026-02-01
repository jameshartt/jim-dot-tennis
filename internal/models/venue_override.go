package models

import "time"

// VenueOverride represents a date-range venue override for a club.
// When a club is displaced from their home venue, fixtures during the
// override period use the venue_club instead.
type VenueOverride struct {
	ID           uint      `json:"id" db:"id"`
	ClubID       uint      `json:"club_id" db:"club_id"`             // The club being displaced
	VenueClubID  uint      `json:"venue_club_id" db:"venue_club_id"` // Where they play instead
	StartDate    time.Time `json:"start_date" db:"start_date"`
	EndDate      time.Time `json:"end_date" db:"end_date"`
	Reason       string    `json:"reason" db:"reason"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
	VenueClub    *Club     `json:"venue_club,omitempty"`    // The replacement venue club (for display)
	DisplacedClub *Club    `json:"displaced_club,omitempty"` // The displaced club (for display)
}
