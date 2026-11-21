// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package services

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"
)

// DefaultBHPLTABaseURL is the BHPLTA website the fixture and club page
// scrapers read from.
const DefaultBHPLTABaseURL = "https://www.bhplta.co.uk"

// BHPLTABaseURLEnv names the environment variable that points the fixture and
// club page scrapers somewhere other than the BHPLTA website. It exists so
// tests can serve saved pages from a local server; production leaves it unset.
const BHPLTABaseURLEnv = "BHPLTA_BASE_URL"

// BHPLTABaseURL returns the base URL the fixture and club page scrapers use:
// BHPLTA_BASE_URL when set, else the real BHPLTA site. It never ends in "/".
func BHPLTABaseURL() string {
	if v := strings.TrimSpace(os.Getenv(BHPLTABaseURLEnv)); v != "" {
		return strings.TrimRight(v, "/")
	}
	return DefaultBHPLTABaseURL
}

// BHPLTAFixturesURL is the fixtures page under base.
func BHPLTAFixturesURL(base string) string {
	return strings.TrimRight(base, "/") + "/bhplta_tables/fixtures/"
}

// BHPLTAClubURL is the public page for the club with slug under base.
func BHPLTAClubURL(base, slug string) string {
	return strings.TrimRight(base, "/") + "/clubs/" + url.PathEscape(slug) + "/"
}

// ClubSlugFromURL returns the club slug from a BHPLTA club page link such as
// "https://www.bhplta.co.uk/clubs/hove-park-tennis-club/", or "" if the link
// is not a club page.
func ClubSlugFromURL(link string) string {
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil {
		return ""
	}
	p := strings.Trim(path.Clean("/"+u.Path), "/")
	parts := strings.Split(p, "/")
	if len(parts) != 2 || parts[0] != "clubs" || parts[1] == "" {
		return ""
	}
	return parts[1]
}

// HTTPStatusError is a BHPLTA page that answered with a status other than 200.
type HTTPStatusError struct {
	URL    string
	Status int
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("%s answered with HTTP status %d", e.URL, e.Status)
}
