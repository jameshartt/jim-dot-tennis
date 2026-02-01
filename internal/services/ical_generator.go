package services

import (
	"fmt"
	"strings"
	"time"

	"jim-dot-tennis/internal/models"
)

// ICalEvent contains the data needed to generate an .ics calendar event
type ICalEvent struct {
	FixtureID   uint
	Summary     string // e.g., "Tennis: St Ann's A vs Hove B"
	StartTime   time.Time
	EndTime     time.Time
	Location    string // Full address
	Latitude    *float64
	Longitude   *float64
	Description string // Division, week, teams, venue details
	URL         string // Google Maps URL or club website
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

	if event.URL != "" {
		b.WriteString(fmt.Sprintf("URL:%s\r\n", event.URL))
	}

	b.WriteString("CATEGORIES:Tennis\r\n")
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

	// Build location string from venue club data, prefixed with club name
	location := buildLocationString(venueClub)

	// Build rich description with all available venue information
	description := buildDescription(homeTeamName, awayTeamName, divisionName, weekNumber, venueClub)

	// Pick the best URL: Google Maps link, or club website
	url := ""
	if venueClub != nil {
		if venueClub.GoogleMapsURL != nil && *venueClub.GoogleMapsURL != "" {
			url = *venueClub.GoogleMapsURL
		} else if venueClub.Website != "" {
			url = venueClub.Website
		}
	}

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
		URL:         url,
	}

	if venueClub != nil {
		event.Latitude = venueClub.Latitude
		event.Longitude = venueClub.Longitude
	}

	return event
}

// buildLocationString builds an iCal LOCATION string from club data.
// Prepends the club name so calendar apps show the venue name prominently.
func buildLocationString(venueClub *models.Club) string {
	if venueClub == nil {
		return ""
	}

	addressParts := []string{}
	if venueClub.AddressLine1 != nil && *venueClub.AddressLine1 != "" {
		addressParts = append(addressParts, *venueClub.AddressLine1)
	}
	if venueClub.AddressLine2 != nil && *venueClub.AddressLine2 != "" {
		addressParts = append(addressParts, *venueClub.AddressLine2)
	}
	if venueClub.City != nil && *venueClub.City != "" {
		addressParts = append(addressParts, *venueClub.City)
	}
	if venueClub.Postcode != nil && *venueClub.Postcode != "" {
		addressParts = append(addressParts, *venueClub.Postcode)
	}

	if len(addressParts) > 0 {
		return venueClub.Name + ", " + strings.Join(addressParts, ", ")
	}
	if venueClub.Address != "" {
		return venueClub.Name + ", " + venueClub.Address
	}
	return venueClub.Name
}

// buildDescription builds a rich iCal description with match info, venue details,
// directions, and visitor tips. Uses real newlines which escapeICalText converts
// to iCal \n sequences.
func buildDescription(homeTeamName, awayTeamName, divisionName string, weekNumber int, venueClub *models.Club) string {
	var sections []string

	// Match information
	matchLines := []string{}
	matchLines = append(matchLines, fmt.Sprintf("%s vs %s", homeTeamName, awayTeamName))
	if divisionName != "" {
		matchLines = append(matchLines, fmt.Sprintf("Division: %s", divisionName))
	}
	if weekNumber > 0 {
		matchLines = append(matchLines, fmt.Sprintf("Week: %d", weekNumber))
	}
	sections = append(sections, strings.Join(matchLines, "\n"))

	if venueClub == nil {
		return strings.Join(sections, "\n\n")
	}

	// Venue details
	venueLines := []string{}
	venueLines = append(venueLines, fmt.Sprintf("VENUE: %s", venueClub.Name))

	// Full address
	address := buildFullAddress(venueClub)
	if address != "" {
		venueLines = append(venueLines, address)
	}

	if venueClub.PhoneNumber != "" {
		venueLines = append(venueLines, fmt.Sprintf("Phone: %s", venueClub.PhoneNumber))
	}
	if venueClub.Website != "" {
		venueLines = append(venueLines, fmt.Sprintf("Website: %s", venueClub.Website))
	}
	if venueClub.GoogleMapsURL != nil && *venueClub.GoogleMapsURL != "" {
		venueLines = append(venueLines, fmt.Sprintf("Map: %s", *venueClub.GoogleMapsURL))
	}
	sections = append(sections, strings.Join(venueLines, "\n"))

	// Court information
	courtLines := []string{}
	if venueClub.CourtSurface != nil && *venueClub.CourtSurface != "" {
		courtLines = append(courtLines, fmt.Sprintf("Surface: %s", *venueClub.CourtSurface))
	}
	if venueClub.CourtCount != nil && *venueClub.CourtCount > 0 {
		courtLines = append(courtLines, fmt.Sprintf("Courts: %d", *venueClub.CourtCount))
	}
	if len(courtLines) > 0 {
		sections = append(sections, strings.Join(courtLines, "\n"))
	}

	// Getting there
	gettingThere := []string{}
	if venueClub.ParkingInfo != nil && *venueClub.ParkingInfo != "" {
		gettingThere = append(gettingThere, fmt.Sprintf("Parking: %s", *venueClub.ParkingInfo))
	}
	if venueClub.TransportInfo != nil && *venueClub.TransportInfo != "" {
		gettingThere = append(gettingThere, fmt.Sprintf("Transport: %s", *venueClub.TransportInfo))
	}
	if len(gettingThere) > 0 {
		sections = append(sections, strings.Join(gettingThere, "\n"))
	}

	// Tips
	if venueClub.Tips != nil && *venueClub.Tips != "" {
		sections = append(sections, fmt.Sprintf("Tips: %s", *venueClub.Tips))
	}

	return strings.Join(sections, "\n\n")
}

// buildFullAddress builds a single-line address string from club structured address fields
func buildFullAddress(club *models.Club) string {
	parts := []string{}
	if club.AddressLine1 != nil && *club.AddressLine1 != "" {
		parts = append(parts, *club.AddressLine1)
	}
	if club.AddressLine2 != nil && *club.AddressLine2 != "" {
		parts = append(parts, *club.AddressLine2)
	}
	if club.City != nil && *club.City != "" {
		parts = append(parts, *club.City)
	}
	if club.Postcode != nil && *club.Postcode != "" {
		parts = append(parts, *club.Postcode)
	}
	if len(parts) > 0 {
		return strings.Join(parts, ", ")
	}
	if club.Address != "" {
		return club.Address
	}
	return ""
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
