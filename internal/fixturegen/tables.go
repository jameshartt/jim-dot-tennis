// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

// Package fixturegen serves the public fixture generator: the page itself and
// the final league tables it starts from. The generator runs in the browser
// (static/fixture-generator); nothing here touches the database.
package fixturegen

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"jim-dot-tennis/internal/normalize"

	"github.com/PuerkitoBio/goquery"
)

// DefaultTablesURL is the BHPLTA league tables page.
const DefaultTablesURL = "https://www.bhplta.co.uk/bhplta_tables/league-table/"

// Tables is the final standing of every division in a season.
type Tables struct {
	League    League     `json:"league"`
	Season    int        `json:"season"`
	Source    string     `json:"source"`
	FetchedAt time.Time  `json:"fetchedAt"`
	Divisions []Division `json:"divisions"`
}

// League names the competition.
type League struct {
	Name      string `json:"name"`
	ShortName string `json:"shortName"`
}

// Division is one division's table, top team first.
type Division struct {
	Name    string `json:"name"`
	Level   int    `json:"level"`
	PlayDay string `json:"playDay"`
	Teams   []Row  `json:"teams"`
}

// Row is one team's line in a league table.
type Row struct {
	Name       string  `json:"name"`
	Played     int     `json:"played"`
	EventsWon  float64 `json:"eventsWon"`
	EventsLost float64 `json:"eventsLost"`
	SetsWon    int     `json:"setsWon"`
	SetsLost   int     `json:"setsLost"`
	Points     float64 `json:"points"`
	Penalty    bool    `json:"penalty,omitempty"`
}

var yearPattern = regexp.MustCompile(`\b(20\d{2})\b`)

// PlayDayForLevel is the night each division plays under BHPLTA league
// regulation 1: Division 1 on Thursdays, 2 on Wednesdays, 3 and 4 on Tuesdays.
func PlayDayForLevel(level int) string {
	switch level {
	case 1:
		return "Thursday"
	case 2:
		return "Wednesday"
	default:
		return "Tuesday"
	}
}

// ParseTables reads the BHPLTA league tables page.
func ParseTables(r io.Reader) (*Tables, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse league tables HTML: %w", err)
	}

	tables := &Tables{
		League: League{Name: "Brighton & Hove Parks League", ShortName: "BHPLTA"},
	}
	if m := yearPattern.FindStringSubmatch(doc.Find(".bhplta_title_wrapper h1").First().Text()); m != nil {
		tables.Season, _ = strconv.Atoi(m[1])
	}

	doc.Find("table.bhplta_league_table").Each(func(_ int, table *goquery.Selection) {
		name := strings.TrimSpace(table.PrevAllFiltered("h2.bhplta_league_table_heading").First().Text())
		if name == "" {
			name = strings.TrimSpace(table.Parent().Find("h2").First().Text())
		}
		level := len(tables.Divisions) + 1
		if name == "" {
			name = fmt.Sprintf("Division %d", level)
		}
		division := Division{Name: name, Level: level, PlayDay: PlayDayForLevel(level)}

		table.Find("tbody tr").Each(func(_ int, tr *goquery.Selection) {
			cells := tr.Find("td")
			if cells.Length() < 7 {
				return
			}
			text := func(i int) string { return strings.TrimSpace(cells.Eq(i).Text()) }
			points := text(6)
			row := Row{
				Name:       strings.Join(strings.Fields(normalize.Apostrophes(text(0))), " "),
				Played:     int(number(text(1))),
				EventsWon:  number(text(2)),
				EventsLost: number(text(3)),
				SetsWon:    int(number(text(4))),
				SetsLost:   int(number(text(5))),
				Points:     number(points),
				Penalty:    strings.Contains(points, "*"),
			}
			if row.Name != "" {
				division.Teams = append(division.Teams, row)
			}
		})
		if len(division.Teams) > 0 {
			tables.Divisions = append(tables.Divisions, division)
		}
	})

	if len(tables.Divisions) == 0 {
		return nil, fmt.Errorf("no league tables found on the page")
	}
	return tables, nil
}

// number parses a table cell, ignoring footnote markers such as "54*".
func number(s string) float64 {
	s = strings.TrimRight(strings.TrimSpace(s), "*† ")
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// FetchTables downloads and parses the league tables page.
func FetchTables(ctx context.Context, client *http.Client, url string) (*Tables, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "jim.tennis fixture generator (+https://github.com/jameshartt/jim-dot-tennis)")
	req.Header.Set("Accept", "text/html")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch league tables: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("league tables page returned status %d", resp.StatusCode)
	}

	tables, err := ParseTables(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	tables.Source = url
	tables.FetchedAt = time.Now().UTC()
	if tables.Season == 0 {
		tables.Season = tables.FetchedAt.Year()
	}
	return tables, nil
}
