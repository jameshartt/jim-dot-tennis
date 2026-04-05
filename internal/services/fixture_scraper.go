// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package services

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// ScrapedFixture represents a single fixture parsed from the BHPLTA fixtures page
type ScrapedFixture struct {
	Division string
	Week     int
	Date     time.Time
	HomeTeam string
	AwayTeam string
}

// ScrapedDivision represents a division with its teams and fixtures from the BHPLTA page
type ScrapedDivision struct {
	Name     string
	Teams    []string
	Fixtures []ScrapedFixture
}

// ScrapeFixtures fetches and parses the BHPLTA fixtures page, returning divisions with their teams and fixtures
func ScrapeFixtures(url string) ([]ScrapedDivision, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch fixtures page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fixtures page returned status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse fixtures HTML: %w", err)
	}

	return ParseFixturesHTML(doc)
}

// ParseFixturesHTML parses a goquery document of the BHPLTA fixtures page
func ParseFixturesHTML(doc *goquery.Document) ([]ScrapedDivision, error) {
	var divisions []ScrapedDivision

	doc.Find("div.tab-pane").Each(func(i int, tabPane *goquery.Selection) {
		divName := strings.TrimSpace(tabPane.Find("h2.bhplta_fixtures_heading").Text())
		if divName == "" {
			return
		}

		div := ScrapedDivision{
			Name: divName,
		}

		teamSet := make(map[string]bool)

		tabPane.Find("table.bhplta_fixtures_table").Each(func(j int, table *goquery.Selection) {
			weekNum := parseWeekNumber(table.Find("caption").Text())
			dateStr := strings.TrimSpace(table.Find("thead th").Text())
			fixtureDate := parseFixtureDateBHPLTA(dateStr)

			table.Find("tbody tr").Each(func(k int, row *goquery.Selection) {
				homeTeam := normalizeApostrophes(strings.TrimSpace(row.Find("td.bhplta_fixtures_home_team").Text()))
				awayTeam := normalizeApostrophes(strings.TrimSpace(row.Find("td.bhplta_fixtures_away_team").Text()))

				if homeTeam == "" || awayTeam == "" {
					return
				}

				div.Fixtures = append(div.Fixtures, ScrapedFixture{
					Division: divName,
					Week:     weekNum,
					Date:     fixtureDate,
					HomeTeam: homeTeam,
					AwayTeam: awayTeam,
				})

				teamSet[homeTeam] = true
				teamSet[awayTeam] = true
			})
		})

		for team := range teamSet {
			div.Teams = append(div.Teams, team)
		}

		divisions = append(divisions, div)
	})

	if len(divisions) == 0 {
		return nil, fmt.Errorf("no divisions found on fixtures page")
	}

	return divisions, nil
}

// normalizeApostrophes replaces curly/smart apostrophes with standard ASCII apostrophe
// to ensure consistent matching against database records
func normalizeApostrophes(s string) string {
	s = strings.ReplaceAll(s, "\u2019", "'") // right single quotation mark '
	s = strings.ReplaceAll(s, "\u2018", "'") // left single quotation mark '
	s = strings.ReplaceAll(s, "\u02BC", "'") // modifier letter apostrophe ʼ
	return s
}

var weekNumberRe = regexp.MustCompile(`Week\s+(\d+)`)

// parseWeekNumber extracts the week number from text like "Week 1"
func parseWeekNumber(text string) int {
	matches := weekNumberRe.FindStringSubmatch(strings.TrimSpace(text))
	if len(matches) >= 2 {
		n, _ := strconv.Atoi(matches[1])
		return n
	}
	return 0
}

// parseFixtureDateBHPLTA parses a date like "16 Apr 2026" from the BHPLTA page
func parseFixtureDateBHPLTA(dateStr string) time.Time {
	dateStr = strings.TrimSpace(dateStr)
	if dateStr == "" {
		return time.Time{}
	}

	t, err := time.Parse("2 Jan 2006", dateStr)
	if err != nil {
		return time.Time{}
	}
	return t
}
