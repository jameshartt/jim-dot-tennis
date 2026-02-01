package services

import (
	"fmt"
	"strings"
	"time"

	"jim-dot-tennis/internal/models"
)

// ICalEvent contains the data needed to generate an .ics calendar event
type ICalEvent struct {
	FixtureID     uint
	Summary       string    // e.g., "Tennis: St Ann's A vs Hove B"
	StartTime     time.Time
	EndTime       time.Time
	Location      string    // Full address
	Latitude      *float64
	Longitude     *float64
	Description   string    // Division, week, teams
}

// GenerateICalEvent generates an .ics file content string for a fixture
func GenerateICalEvent(event ICalEvent) string {
	var b strings.Builder

	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("VERSION:2.0\r\n")
	b.WriteString("PRODID:-//Jim.Tennis//Fixture Calendar//EN\r\n")
	b.WriteString("CALSCALE:GREGORIAN\r\n")
	b.WriteString("METHOD:PUBLISH\r\n")
	b.WriteString("BEGIN:VEVENT\r\n")

	// Stable UID so re-downloads update not duplicate
	b.WriteString(fmt.Sprintf("UID:fixture-%d@jim.tennis\r\n", event.FixtureID))

	// Timestamps in UTC
	b.WriteString(fmt.Sprintf("DTSTAMP:%s\r\n", formatICalTime(time.Now())))
	b.WriteString(fmt.Sprintf("DTSTART:%s\r\n", formatICalTime(event.StartTime)))
	b.WriteString(fmt.Sprintf("DTEND:%s\r\n", formatICalTime(event.EndTime)))

	b.WriteString(fmt.Sprintf("SUMMARY:%s\r\n", escapeICalText(event.Summary)))

	if event.Location != "" {
		b.WriteString(fmt.Sprintf("LOCATION:%s\r\n", escapeICalText(event.Location)))
	}

	if event.Latitude != nil && event.Longitude != nil {
		b.WriteString(fmt.Sprintf("GEO:%.6f;%.6f\r\n", *event.Latitude, *event.Longitude))
	}

	if event.Description != "" {
		b.WriteString(fmt.Sprintf("DESCRIPTION:%s\r\n", escapeICalText(event.Description)))
	}

	b.WriteString("END:VEVENT\r\n")
	b.WriteString("END:VCALENDAR\r\n")

	return b.String()
}

// BuildICalEventFromFixture builds an ICalEvent from fixture and venue data
func BuildICalEventFromFixture(
	fixture *models.Fixture,
	homeTeamName, awayTeamName string,
	divisionName string,
	weekNumber int,
	venueClub *models.Club,
) ICalEvent {
	summary := fmt.Sprintf("Tennis: %s vs %s", homeTeamName, awayTeamName)

	// Build location string from venue club data
	location := ""
	if venueClub != nil {
		parts := []string{}
		if venueClub.AddressLine1 != nil && *venueClub.AddressLine1 != "" {
			parts = append(parts, *venueClub.AddressLine1)
		}
		if venueClub.AddressLine2 != nil && *venueClub.AddressLine2 != "" {
			parts = append(parts, *venueClub.AddressLine2)
		}
		if venueClub.City != nil && *venueClub.City != "" {
			parts = append(parts, *venueClub.City)
		}
		if venueClub.Postcode != nil && *venueClub.Postcode != "" {
			parts = append(parts, *venueClub.Postcode)
		}
		if len(parts) > 0 {
			location = strings.Join(parts, ", ")
		} else if venueClub.Address != "" {
			location = venueClub.Address
		} else {
			location = venueClub.Name
		}
	}

	// Build description
	descParts := []string{}
	if divisionName != "" {
		descParts = append(descParts, fmt.Sprintf("Division: %s", divisionName))
	}
	if weekNumber > 0 {
		descParts = append(descParts, fmt.Sprintf("Week: %d", weekNumber))
	}
	descParts = append(descParts, fmt.Sprintf("Home: %s", homeTeamName))
	descParts = append(descParts, fmt.Sprintf("Away: %s", awayTeamName))
	if venueClub != nil {
		descParts = append(descParts, fmt.Sprintf("Venue: %s", venueClub.Name))
	}
	description := strings.Join(descParts, "\\n")

	// 3-hour match duration
	startTime := fixture.ScheduledDate
	endTime := startTime.Add(3 * time.Hour)

	event := ICalEvent{
		FixtureID:   fixture.ID,
		Summary:     summary,
		StartTime:   startTime,
		EndTime:     endTime,
		Location:    location,
		Description: description,
	}

	if venueClub != nil {
		event.Latitude = venueClub.Latitude
		event.Longitude = venueClub.Longitude
	}

	return event
}

// formatICalTime formats a time.Time as an iCal UTC datetime string
func formatICalTime(t time.Time) string {
	return t.UTC().Format("20060102T150405Z")
}

// escapeICalText escapes special characters in iCal text values
func escapeICalText(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, ";", "\\;")
	s = strings.ReplaceAll(s, ",", "\\,")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}
