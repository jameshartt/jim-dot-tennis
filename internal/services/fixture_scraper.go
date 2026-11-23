// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"jim-dot-tennis/internal/normalize"
	"jim-dot-tennis/internal/version"

	"github.com/PuerkitoBio/goquery"
)

// ScrapedFixture represents a single fixture parsed from the BHPLTA fixtures page
type ScrapedFixture struct {
	Division string
	Week     int
	Date     time.Time
	HomeTeam string
	AwayTeam string
	// HomeClubSlug is the BHPLTA club page slug the home team links to
	// ("" when the cell has no club link).
	HomeClubSlug string
}

// ScrapedDivision represents a division with its teams and fixtures from the BHPLTA page
type ScrapedDivision struct {
	Name     string
	Teams    []string
	Fixtures []ScrapedFixture
}

// ErrNoFixturesPublished means the BHPLTA fixtures page loaded and says there
// are no upcoming fixtures (between seasons, before the new season's fixtures
// are published).
var ErrNoFixturesPublished = errors.New("the BHPLTA fixtures page has no upcoming fixtures")

// ErrUnexpectedMarkup means a BHPLTA page loaded but did not contain what the
// scraper looks for, which usually means the site has changed.
var ErrUnexpectedMarkup = errors.New("the page did not look as expected")

// ScrapeFixtures fetches and parses the BHPLTA fixtures page, returning divisions with their teams and fixtures
func ScrapeFixtures(url string) ([]ScrapedDivision, error) {
	return FetchFixtures(context.Background(), &http.Client{Timeout: 30 * time.Second}, url)
}

// FetchFixtures fetches and parses the BHPLTA fixtures page at url. Besides
// network errors it returns an *HTTPStatusError for a non-200 answer,
// ErrNoFixturesPublished when the page says there are no upcoming fixtures,
// and an error wrapping ErrUnexpectedMarkup when the page has neither.
func FetchFixtures(ctx context.Context, client *http.Client, url string) ([]ScrapedDivision, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", version.UserAgent())

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch fixtures page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &HTTPStatusError{URL: url, Status: resp.StatusCode}
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
				homeCell := row.Find("td.bhplta_fixtures_home_team")
				homeTeam := normalize.Apostrophes(strings.TrimSpace(homeCell.Text()))
				awayTeam := normalize.Apostrophes(strings.TrimSpace(row.Find("td.bhplta_fixtures_away_team").Text()))

				if homeTeam == "" || awayTeam == "" {
					return
				}
				// A team without a fixture that week is listed against a bye.
				if row.Find(".bhplta_bye").Length() > 0 || strings.EqualFold(homeTeam, "bye") || strings.EqualFold(awayTeam, "bye") {
					return
				}

				div.Fixtures = append(div.Fixtures, ScrapedFixture{
					Division:     divName,
					Week:         weekNum,
					Date:         fixtureDate,
					HomeTeam:     homeTeam,
					AwayTeam:     awayTeam,
					HomeClubSlug: ClubSlugFromURL(homeCell.Find("a").AttrOr("href", "")),
				})

				teamSet[homeTeam] = true
				teamSet[awayTeam] = true
			})
		})

		for team := range teamSet {
			div.Teams = append(div.Teams, team)
		}
		sort.Strings(div.Teams)

		divisions = append(divisions, div)
	})

	if len(divisions) == 0 {
		if strings.Contains(strings.ToLower(doc.Find("body").Text()), "no upcoming fixtures") {
			return nil, ErrNoFixturesPublished
		}
		return nil, fmt.Errorf("no divisions found on fixtures page: %w", ErrUnexpectedMarkup)
	}

	return divisions, nil
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
