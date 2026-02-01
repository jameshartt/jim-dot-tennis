package models

import (
	"time"
)

// MessageCategory defines the types of messages that can be sent
type MessageCategory string

const (
	// MessageGeneral represents general announcements
	MessageGeneral MessageCategory = "General"
	// MessageFixture represents fixture-related messages
	MessageFixture MessageCategory = "Fixture"
	// MessageAvailability represents availability-related messages
	MessageAvailability MessageCategory = "Availability"
	// MessageSelection represents team selection messages
	MessageSelection MessageCategory = "Selection"
	// MessageResults represents match results messages
	MessageResults MessageCategory = "Results"
	// MessageAdministrative represents administrative messages
	MessageAdministrative MessageCategory = "Administrative"
)

// MessageImportance defines the importance level of messages and notifications
type MessageImportance string

const (
	// ImportanceLow indicates a low importance message
	ImportanceLow MessageImportance = "Low"
	// ImportanceMedium indicates a medium importance message
	ImportanceMedium MessageImportance = "Medium"
	// ImportanceHigh indicates a high importance message
	ImportanceHigh MessageImportance = "High"
	// ImportanceUrgent indicates an urgent message
	ImportanceUrgent MessageImportance = "Urgent"
)

// SenderType defines who sent the message
type SenderType string

const (
	// SenderSystem indicates a system-generated message
	SenderSystem SenderType = "System"
	// SenderCaptain indicates a message from a captain
	SenderCaptain SenderType = "Captain"
	// SenderAdmin indicates a message from an admin
	SenderAdmin SenderType = "Admin"
	// SenderPlayer indicates a message from a player
	SenderPlayer SenderType = "Player"
)

// RecipientType defines who should receive the message
type RecipientType string

const (
	// RecipientPlayer indicates a message for a specific player
	RecipientPlayer RecipientType = "Player"
	// RecipientCaptain indicates a message for a captain
	RecipientCaptain RecipientType = "Captain"
	// RecipientTeam indicates a message for an entire team
	RecipientTeam RecipientType = "Team"
	// RecipientDivision indicates a message for an entire division
	RecipientDivision RecipientType = "Division"
	// RecipientAll indicates a broadcast message for everyone
	RecipientAll RecipientType = "All"
)

// EntityType defines what entity a message or notification is related to
type EntityType string

const (
	// EntityFixture indicates relation to a fixture
	EntityFixture EntityType = "Fixture"
	// EntityTeam indicates relation to a team
	EntityTeam EntityType = "Team"
	// EntityDivision indicates relation to a division
	EntityDivision EntityType = "Division"
	// EntityLeague indicates relation to a league
	EntityLeague EntityType = "League"
	// EntitySeason indicates relation to a season
	EntitySeason EntityType = "Season"
)

// ActionType defines what action a notification prompts the user to take
type ActionType string

const (
	// ActionSubmitAvailability prompts user to submit availability
	ActionSubmitAvailability ActionType = "SubmitAvailability"
	// ActionViewSelection prompts user to view team selection
	ActionViewSelection ActionType = "ViewSelection"
	// ActionConfirmAttendance prompts user to confirm attendance
	ActionConfirmAttendance ActionType = "ConfirmAttendance"
	// ActionViewResults prompts user to view match results
	ActionViewResults ActionType = "ViewResults"
	// ActionAcknowledge prompts user to acknowledge the notification
	ActionAcknowledge ActionType = "Acknowledge"
)

// Message represents a persistent message in the system
type Message struct {
	ID                uint               `json:"id"`
	Title             string             `json:"title"`
	Content           string             `json:"content"`
	SenderType        SenderType         `json:"sender_type"`
	SenderID          string             `json:"sender_id,omitempty"` // UUID or ID of sender
	Category          MessageCategory    `json:"category"`
	Importance        MessageImportance  `json:"importance"`
	RelatedEntityType EntityType         `json:"related_entity_type,omitempty"`
	RelatedEntityID   uint               `json:"related_entity_id,omitempty"`
	IsDraft           bool               `json:"is_draft"`
	SentAt            *time.Time         `json:"sent_at,omitempty"`
	SeasonID          uint               `json:"season_id,omitempty"`
	CreatedAt         time.Time          `json:"created_at"`
	UpdatedAt         time.Time          `json:"updated_at"`
	Recipients        []MessageRecipient `json:"recipients,omitempty"`
}

// MessageRecipient represents a recipient of a message
type MessageRecipient struct {
	ID            uint          `json:"id"`
	MessageID     uint          `json:"message_id"`
	RecipientType RecipientType `json:"recipient_type"`
	RecipientID   string        `json:"recipient_id,omitempty"` // Null for broadcast messages
	IsRead        bool          `json:"is_read"`
	ReadAt        *time.Time    `json:"read_at,omitempty"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

// Notification represents an ephemeral notification
type Notification struct {
	ID                uint              `json:"id"`
	Title             string            `json:"title"`
	Content           string            `json:"content"`
	RecipientID       string            `json:"recipient_id"` // UUID of player or captain
	RecipientType     RecipientType     `json:"recipient_type"`
	MessageID         *uint             `json:"message_id,omitempty"` // Optional reference to message
	ActionType        ActionType        `json:"action_type,omitempty"`
	ActionURL         string            `json:"action_url,omitempty"`
	IsRead            bool              `json:"is_read"`
	ReadAt            *time.Time        `json:"read_at,omitempty"`
	ExpiresAt         *time.Time        `json:"expires_at,omitempty"`
	Importance        MessageImportance `json:"importance"`
	RelatedEntityType EntityType        `json:"related_entity_type,omitempty"`
	RelatedEntityID   uint              `json:"related_entity_id,omitempty"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

// ScheduledMessage represents a message scheduled to be sent in the future
type ScheduledMessage struct {
	ID            uint       `json:"id"`
	MessageID     uint       `json:"message_id"`
	ScheduledTime time.Time  `json:"scheduled_time"`
	Recurrence    string     `json:"recurrence,omitempty"` // None, Daily, Weekly, Monthly
	IsSent        bool       `json:"is_sent"`
	LastSentAt    *time.Time `json:"last_sent_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	Message       *Message   `json:"message,omitempty"`
}

// DeliveryPreference represents a player's preferences for receiving notifications
type DeliveryPreference struct {
	ID                     uint      `json:"id"`
	PlayerID               string    `json:"player_id"` // UUID of player
	EmailEnabled           bool      `json:"email_enabled"`
	PushEnabled            bool      `json:"push_enabled"`
	SMSEnabled             bool      `json:"sms_enabled"`
	InAppEnabled           bool      `json:"in_app_enabled"`
	AvailabilityReminders  bool      `json:"availability_reminders"`
	SelectionNotifications bool      `json:"selection_notifications"`
	FixtureReminders       bool      `json:"fixture_reminders"`
	ResultNotifications    bool      `json:"result_notifications"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}
