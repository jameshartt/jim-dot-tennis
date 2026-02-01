package services

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// ClubSlugMapping maps a URL slug to the expected display name on the BHPLTA website
type ClubSlugMapping struct {
	Slug        string
	DisplayName string // expected club name as shown on the page
}

// KnownClubSlugs contains all known BHPLTA club page slugs with their expected display names
var KnownClubSlugs = []ClubSlugMapping{
	{Slug: "blakers", DisplayName: "Blakers Park Tennis Club"},
	{Slug: "blagss", DisplayName: "BLAGSS Tennis"},
	{Slug: "dyke-park-tennis-club", DisplayName: "Dyke Park Tennis Club"},
	{Slug: "hollingbury-park-tennis-club", DisplayName: "Hollingbury Park Tennis Club"},
	{Slug: "hove-park-tennis-club", DisplayName: "Hove Park Tennis Club"},
	{Slug: "king-alfred-tennis-club", DisplayName: "King Alfred Tennis Club"},
	{Slug: "park-avenue-tennis-club", DisplayName: "Park Avenue Tennis Club"},
	{Slug: "preston-park-tennis-club", DisplayName: "Preston Park Tennis Club"},
	{Slug: "queens-park", DisplayName: "Queens Park Tennis Club"},
	{Slug: "rookery-tennis-club", DisplayName: "Rookery Tennis Club"},
	{Slug: "saltdean-tennis-club", DisplayName: "Saltdean Tennis Club"},
	{Slug: "st-anns", DisplayName: "St Ann's Tennis"},
}

// ScrapedClubInfo holds data scraped from a BHPLTA club page
type ScrapedClubInfo struct {
	Slug          string
	Name          string
	Address       string
	AddressLine1  string
	AddressLine2  string
	City          string
	Postcode      string
	Website       string
	Email         string
	Phone         string
	ContactPerson string
	Latitude      *float64
	Longitude     *float64
}

// ClubScraperSummary tracks the results of a scrape operation
type ClubScraperSummary struct {
	Matched int
	Created int
	Updated int
	Skipped int
	Errors  []string
}

// ClubScraper handles scraping club information from the BHPLTA website
type ClubScraper struct {
	db         *database.DB
	clubRepo   repository.ClubRepository
	httpClient *http.Client
	dryRun     bool
	verbose    bool
}

// NewClubScraper creates a new ClubScraper instance
func NewClubScraper(db *database.DB, clubRepo repository.ClubRepository, dryRun bool, verbose bool) *ClubScraper {
	return &ClubScraper{
		db:       db,
		clubRepo: clubRepo,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		dryRun:  dryRun,
		verbose: verbose,
	}
}

// ScrapeAll scrapes all known club pages and updates the database
func (s *ClubScraper) ScrapeAll(ctx context.Context) (*ClubScraperSummary, error) {
	summary := &ClubScraperSummary{}

	for _, mapping := range KnownClubSlugs {
		if s.verbose {
			fmt.Printf("\n--- Scraping club: %s (slug: %s) ---\n", mapping.DisplayName, mapping.Slug)
		}

		result, err := s.ScrapeClub(ctx, mapping.Slug)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to scrape %s: %v", mapping.Slug, err)
			summary.Errors = append(summary.Errors, errMsg)
			fmt.Printf("ERROR: %s\n", errMsg)
			continue
		}

		// Merge single-club result into overall summary
		summary.Matched += result.Matched
		summary.Created += result.Created
		summary.Updated += result.Updated
		summary.Skipped += result.Skipped
		summary.Errors = append(summary.Errors, result.Errors...)

		// Rate limit between requests
		time.Sleep(1 * time.Second)
	}

	return summary, nil
}

// ScrapeClub scrapes a single club page and updates/creates the database record
func (s *ClubScraper) ScrapeClub(ctx context.Context, slug string) (*ClubScraperSummary, error) {
	summary := &ClubScraperSummary{}

	// Fetch and parse the club page
	info, err := s.fetchClubPage(slug)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch club page: %w", err)
	}

	if s.verbose {
		s.printScrapedInfo(info)
	}

	// Try to match to an existing club in the database
	club, matched, err := s.matchClubToDatabase(ctx, info)
	if err != nil {
		return nil, fmt.Errorf("failed to match club to database: %w", err)
	}

	if matched {
		summary.Matched++

		// Update existing club with scraped data
		updated := s.updateClubFromScrapedInfo(club, info)

		if updated {
			if s.dryRun {
				fmt.Printf("[DRY RUN] Would update club: %s (ID: %d)\n", club.Name, club.ID)
			} else {
				if err := s.clubRepo.Update(ctx, club); err != nil {
					return nil, fmt.Errorf("failed to update club: %w", err)
				}
				if s.verbose {
					fmt.Printf("Updated club: %s (ID: %d)\n", club.Name, club.ID)
				}
			}
			summary.Updated++
		} else {
			if s.verbose {
				fmt.Printf("Club %s (ID: %d) already up to date, skipping\n", club.Name, club.ID)
			}
			summary.Skipped++
		}
	} else {
		// Create new club
		newClub := s.buildClubFromScrapedInfo(info)

		if s.dryRun {
			fmt.Printf("[DRY RUN] Would create new club: %s\n", newClub.Name)
		} else {
			if err := s.clubRepo.Create(ctx, newClub); err != nil {
				return nil, fmt.Errorf("failed to create club: %w", err)
			}
			if s.verbose {
				fmt.Printf("Created new club: %s (ID: %d)\n", newClub.Name, newClub.ID)
			}
		}
		summary.Created++
	}

	return summary, nil
}

// fetchClubPage fetches and parses a single club page from the BHPLTA website
func (s *ClubScraper) fetchClubPage(slug string) (*ScrapedClubInfo, error) {
	url := fmt.Sprintf("https://www.bhplta.co.uk/clubs/%s/", slug)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:140.0) Gecko/20100101 Firefox/140.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-GB,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("Connection", "keep-alive")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d for %s", resp.StatusCode, url)
	}

	// Handle compressed response
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	info := &ScrapedClubInfo{
		Slug: slug,
	}

	// Extract club name from page title or h1/h2
	s.extractClubName(doc, info)

	// Extract address from the "Club Address" section
	s.extractAddress(doc, info)

	// Extract website and email links
	s.extractContactLinks(doc, info)

	// Extract phone number
	s.extractPhoneNumber(doc, info, string(body))

	// Try to extract coordinates from map markers
	s.extractCoordinates(doc, info, string(body))

	// If name is still empty, use the slug display name
	if info.Name == "" {
		for _, m := range KnownClubSlugs {
			if m.Slug == slug {
				info.Name = m.DisplayName
				break
			}
		}
	}

	return info, nil
}

// extractClubName extracts the club name from the page
func (s *ClubScraper) extractClubName(doc *goquery.Document, info *ScrapedClubInfo) {
	// Try the entry title first (WordPress pattern)
	title := doc.Find(".entry-title").First().Text()
	if title != "" {
		info.Name = strings.TrimSpace(title)
		return
	}

	// Try h1 tag
	h1 := doc.Find("h1").First().Text()
	if h1 != "" {
		info.Name = strings.TrimSpace(h1)
		return
	}

	// Try h2 inside article content
	h2 := doc.Find("article h2, .entry-content h2").First().Text()
	if h2 != "" {
		info.Name = strings.TrimSpace(h2)
		return
	}
}

// extractAddress extracts address information from the club page
func (s *ClubScraper) extractAddress(doc *goquery.Document, info *ScrapedClubInfo) {
	// Look for the "Club Address" h3 and get the following content
	doc.Find("h3").Each(func(i int, sel *goquery.Selection) {
		heading := strings.TrimSpace(sel.Text())
		if !strings.EqualFold(heading, "Club Address") {
			return
		}

		// The address is typically in the next sibling <p> element
		nextP := sel.Next()
		if nextP.Length() == 0 {
			return
		}

		// Get the HTML content of the paragraph to split by <br>
		html, err := nextP.Html()
		if err != nil {
			return
		}

		// Split by <br> or <br/> tags
		lines := splitHTMLByBR(html)

		// Store the full address text
		var addressParts []string
		for _, line := range lines {
			cleaned := strings.TrimRight(strings.TrimSpace(line), ",")
			if cleaned != "" {
				addressParts = append(addressParts, cleaned)
			}
		}
		info.Address = strings.Join(addressParts, ", ")

		// Parse address components
		s.parseAddressComponents(addressParts, info)
	})
}

// splitHTMLByBR splits HTML content by <br>, <br/>, <br /> tags and strips HTML tags from parts
func splitHTMLByBR(html string) []string {
	// Replace <br> variants with a delimiter
	brRegex := regexp.MustCompile(`<br\s*/?>`)
	html = brRegex.ReplaceAllString(html, "\n")

	// Strip remaining HTML tags
	tagRegex := regexp.MustCompile(`<[^>]*>`)
	html = tagRegex.ReplaceAllString(html, "")

	// Decode common HTML entities
	html = strings.ReplaceAll(html, "&amp;", "&")
	html = strings.ReplaceAll(html, "&#8217;", "'")
	html = strings.ReplaceAll(html, "&#8216;", "'")
	html = strings.ReplaceAll(html, "&rsquo;", "'")
	html = strings.ReplaceAll(html, "&lsquo;", "'")

	parts := strings.Split(html, "\n")
	var result []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// parseAddressComponents breaks down address parts into structured fields
func (s *ClubScraper) parseAddressComponents(parts []string, info *ScrapedClubInfo) {
	if len(parts) == 0 {
		return
	}

	// UK postcode regex pattern
	postcodeRegex := regexp.MustCompile(`[A-Z]{1,2}\d[A-Z\d]?\s*\d[A-Z]{2}`)

	// Look for postcode in any part of the address
	for i, part := range parts {
		match := postcodeRegex.FindString(strings.ToUpper(part))
		if match != "" {
			info.Postcode = match

			// The city is typically in the same line as the postcode, or the line before
			// Extract city by removing the postcode from the line
			cityLine := postcodeRegex.ReplaceAllString(part, "")
			cityLine = strings.TrimRight(strings.TrimSpace(cityLine), ",")
			if cityLine != "" {
				info.City = cityLine
			} else if i > 0 {
				// Check previous line for city
				prevLine := strings.TrimRight(strings.TrimSpace(parts[i-1]), ",")
				// Check if it looks like a city name (not an address line)
				if !strings.ContainsAny(prevLine, "0123456789") {
					info.City = prevLine
				}
			}
		}
	}

	// First line is usually the club/venue name or first address line
	if len(parts) > 0 {
		info.AddressLine1 = strings.TrimRight(strings.TrimSpace(parts[0]), ",")
	}

	// Second line (if present and not the postcode line) is address line 2
	if len(parts) > 1 {
		line2 := strings.TrimRight(strings.TrimSpace(parts[1]), ",")
		// Only use as address line 2 if it's not the city/postcode line
		if !postcodeRegex.MatchString(strings.ToUpper(line2)) {
			info.AddressLine2 = line2
		}
	}
}

// extractContactLinks extracts website and email links from the page
func (s *ClubScraper) extractContactLinks(doc *goquery.Document, info *ScrapedClubInfo) {
	// Look for links within the entry content
	doc.Find(".entry-content a, article a").Each(func(i int, sel *goquery.Selection) {
		href, exists := sel.Attr("href")
		if !exists || href == "" {
			return
		}

		linkText := strings.TrimSpace(sel.Text())

		// Check for email links
		if strings.HasPrefix(href, "mailto:") {
			email := strings.TrimPrefix(href, "mailto:")
			if info.Email == "" {
				info.Email = email
			}
			return
		}

		// Check for website links (not internal BHPLTA links, not image links)
		if strings.EqualFold(linkText, "Website") || strings.EqualFold(linkText, "Club Website") {
			info.Website = href
			return
		}

		// Also detect external website links that are not to bhplta.co.uk
		if !strings.Contains(href, "bhplta.co.uk") &&
			!strings.HasPrefix(href, "#") &&
			!strings.HasPrefix(href, "/") &&
			(strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://")) &&
			info.Website == "" &&
			!strings.Contains(href, "wp-content") &&
			!strings.Contains(href, "facebook.com") &&
			!strings.Contains(href, "twitter.com") &&
			!strings.Contains(href, "instagram.com") {
			info.Website = href
		}
	})
}

// extractPhoneNumber extracts phone numbers from the page content
func (s *ClubScraper) extractPhoneNumber(doc *goquery.Document, info *ScrapedClubInfo, bodyHTML string) {
	// UK phone number patterns
	phoneRegex := regexp.MustCompile(`(?:(?:\+44\s?|0)(?:\d\s?){9,10}\d)`)

	// Look in the entry content text
	contentText := doc.Find(".entry-content, article").Text()
	match := phoneRegex.FindString(contentText)
	if match != "" {
		info.Phone = strings.TrimSpace(match)
		return
	}

	// Also try tel: links
	doc.Find("a[href^='tel:']").Each(func(i int, sel *goquery.Selection) {
		href, _ := sel.Attr("href")
		phone := strings.TrimPrefix(href, "tel:")
		if info.Phone == "" {
			info.Phone = strings.TrimSpace(phone)
		}
	})

	// Fallback: look in the full body HTML for phone patterns
	if info.Phone == "" {
		match = phoneRegex.FindString(bodyHTML)
		if match != "" {
			info.Phone = strings.TrimSpace(match)
		}
	}
}

// extractCoordinates tries to extract lat/lng coordinates from the page
func (s *ClubScraper) extractCoordinates(doc *goquery.Document, info *ScrapedClubInfo, bodyHTML string) {
	// Method 1: Look for ACF map marker elements with data-lat and data-lng
	doc.Find(".marker").Each(func(i int, sel *goquery.Selection) {
		if lat, exists := sel.Attr("data-lat"); exists {
			if lng, exists := sel.Attr("data-lng"); exists {
				latVal := parseFloat64(lat)
				lngVal := parseFloat64(lng)
				if latVal != nil && lngVal != nil {
					info.Latitude = latVal
					info.Longitude = lngVal
				}
			}
		}
	})

	// Method 2: Look for data-lat/data-lng on any element
	if info.Latitude == nil {
		doc.Find("[data-lat]").Each(func(i int, sel *goquery.Selection) {
			if info.Latitude != nil {
				return // already found
			}
			if lat, exists := sel.Attr("data-lat"); exists {
				if lng, exists := sel.Attr("data-lng"); exists {
					latVal := parseFloat64(lat)
					lngVal := parseFloat64(lng)
					if latVal != nil && lngVal != nil {
						info.Latitude = latVal
						info.Longitude = lngVal
					}
				}
			}
		})
	}

	// Method 3: Look for LatLng in JavaScript code
	if info.Latitude == nil {
		latLngRegex := regexp.MustCompile(`LatLng\(\s*(-?\d+\.?\d*)\s*,\s*(-?\d+\.?\d*)\s*\)`)
		matches := latLngRegex.FindStringSubmatch(bodyHTML)
		if len(matches) == 3 {
			latVal := parseFloat64(matches[1])
			lngVal := parseFloat64(matches[2])
			// Sanity check: Brighton area is roughly lat 50.8, lng -0.1
			if latVal != nil && lngVal != nil && *latVal != 0 && *lngVal != 0 {
				info.Latitude = latVal
				info.Longitude = lngVal
			}
		}
	}

	// Method 4: Look for initMap or coordinates in script tags
	if info.Latitude == nil {
		coordRegex := regexp.MustCompile(`(?:lat|latitude)\s*[:=]\s*(-?\d+\.?\d+)`)
		lngRegex := regexp.MustCompile(`(?:lng|longitude|lon)\s*[:=]\s*(-?\d+\.?\d+)`)

		latMatch := coordRegex.FindStringSubmatch(bodyHTML)
		lngMatch := lngRegex.FindStringSubmatch(bodyHTML)

		if len(latMatch) > 1 && len(lngMatch) > 1 {
			latVal := parseFloat64(latMatch[1])
			lngVal := parseFloat64(lngMatch[1])
			if latVal != nil && lngVal != nil && *latVal != 0 && *lngVal != 0 {
				info.Latitude = latVal
				info.Longitude = lngVal
			}
		}
	}

	if info.Latitude == nil && s.verbose {
		fmt.Printf("  WARNING: Could not extract coordinates for %s\n", info.Slug)
	}
}

// parseFloat64 parses a string to *float64, returning nil on failure
func parseFloat64(s string) *float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var val float64
	_, err := fmt.Sscanf(s, "%f", &val)
	if err != nil {
		return nil
	}
	return &val
}

// matchClubToDatabase attempts to match scraped club info to an existing club record
func (s *ClubScraper) matchClubToDatabase(ctx context.Context, info *ScrapedClubInfo) (*models.Club, bool, error) {
	// Strategy 1: Exact name match
	clubs, err := s.clubRepo.FindByName(ctx, info.Name)
	if err == nil && len(clubs) > 0 {
		if s.verbose {
			fmt.Printf("  Matched by exact name: %s (ID: %d)\n", clubs[0].Name, clubs[0].ID)
		}
		return &clubs[0], true, nil
	}

	// Strategy 2: Find the slug display name mapping and try matching by that
	for _, mapping := range KnownClubSlugs {
		if mapping.Slug == info.Slug {
			clubs, err = s.clubRepo.FindByName(ctx, mapping.DisplayName)
			if err == nil && len(clubs) > 0 {
				if s.verbose {
					fmt.Printf("  Matched by display name: %s (ID: %d)\n", clubs[0].Name, clubs[0].ID)
				}
				return &clubs[0], true, nil
			}
			break
		}
	}

	// Strategy 3: Fuzzy name matching with LIKE
	// Try partial matches based on key words from the club name
	searchTerms := s.generateSearchTerms(info)
	for _, term := range searchTerms {
		clubs, err = s.clubRepo.FindByNameLike(ctx, term)
		if err == nil && len(clubs) > 0 {
			// Take the best match if there's only one, or try to narrow down
			if len(clubs) == 1 {
				if s.verbose {
					fmt.Printf("  Matched by fuzzy search '%s': %s (ID: %d)\n", term, clubs[0].Name, clubs[0].ID)
				}
				return &clubs[0], true, nil
			}
			// Multiple matches: try to pick the best one
			best := s.pickBestMatch(clubs, info)
			if best != nil {
				if s.verbose {
					fmt.Printf("  Matched by best-pick from '%s': %s (ID: %d)\n", term, best.Name, best.ID)
				}
				return best, true, nil
			}
		}
	}

	// Strategy 4: Normalize and compare
	normalizedScraped := normalizeClubName(info.Name)
	allClubs, err := s.clubRepo.FindAll(ctx)
	if err == nil {
		for _, club := range allClubs {
			normalizedDB := normalizeClubName(club.Name)
			if normalizedScraped == normalizedDB {
				if s.verbose {
					fmt.Printf("  Matched by normalized name: %s -> %s (ID: %d)\n", info.Name, club.Name, club.ID)
				}
				c := club
				return &c, true, nil
			}
		}
	}

	if s.verbose {
		fmt.Printf("  No existing club match found for: %s\n", info.Name)
	}
	return nil, false, nil
}

// normalizeClubName normalizes a club name for fuzzy matching
func normalizeClubName(name string) string {
	name = strings.ToLower(name)

	// Replace smart quotes and apostrophe variants with nothing
	name = strings.ReplaceAll(name, string([]byte{226, 128, 152}), "") // left single quotation mark
	name = strings.ReplaceAll(name, string([]byte{226, 128, 153}), "") // right single quotation mark
	name = strings.ReplaceAll(name, "'", "")
	name = strings.ReplaceAll(name, "`", "")
	name = strings.ReplaceAll(name, "\u02bc", "") // modifier letter apostrophe
	name = strings.ReplaceAll(name, "\u2032", "") // prime symbol

	// Remove periods
	name = strings.ReplaceAll(name, ".", "")

	// Remove common suffixes
	name = strings.ReplaceAll(name, "tennis club", "")
	name = strings.ReplaceAll(name, "tennis", "")
	name = strings.ReplaceAll(name, "lawn", "")
	name = strings.ReplaceAll(name, "park", "")

	// Normalize whitespace
	name = strings.Join(strings.Fields(name), " ")
	name = strings.TrimSpace(name)

	return name
}

// generateSearchTerms generates search terms for fuzzy matching a club
func (s *ClubScraper) generateSearchTerms(info *ScrapedClubInfo) []string {
	var terms []string

	name := info.Name

	// Add the full name
	terms = append(terms, name)

	// Remove "Tennis Club", "Tennis", "Lawn" suffixes
	simplified := name
	for _, suffix := range []string{" Tennis Club", " Lawn Tennis Club", " Tennis"} {
		simplified = strings.TrimSuffix(simplified, suffix)
	}
	if simplified != name {
		terms = append(terms, simplified)
	}

	// Handle apostrophe variants: St Ann's -> St Ann, St Anns, St. Ann's
	if strings.Contains(name, "'") || strings.Contains(name, "\u2019") {
		noApostrophe := strings.ReplaceAll(name, "'", "")
		noApostrophe = strings.ReplaceAll(noApostrophe, "\u2019", "")
		terms = append(terms, noApostrophe)

		// Without "Tennis Club" too
		for _, suffix := range []string{" Tennis Club", " Tennis"} {
			trimmed := strings.TrimSuffix(noApostrophe, suffix)
			if trimmed != noApostrophe {
				terms = append(terms, trimmed)
			}
		}
	}

	// Extract the core name (first significant word that's not St/The)
	words := strings.Fields(simplified)
	if len(words) > 0 {
		// Use the last significant word as a search term
		lastWord := words[len(words)-1]
		if len(lastWord) > 2 {
			terms = append(terms, lastWord)
		}
	}

	return terms
}

// pickBestMatch picks the best matching club from multiple candidates
func (s *ClubScraper) pickBestMatch(clubs []models.Club, info *ScrapedClubInfo) *models.Club {
	normalizedTarget := normalizeClubName(info.Name)

	var bestClub *models.Club
	bestScore := -1

	for i, club := range clubs {
		normalizedCandidate := normalizeClubName(club.Name)
		score := 0

		// Exact normalized match gets highest score
		if normalizedCandidate == normalizedTarget {
			score = 100
		}

		// Check if one contains the other
		if strings.Contains(normalizedCandidate, normalizedTarget) || strings.Contains(normalizedTarget, normalizedCandidate) {
			score += 50
		}

		// Shorter names that still match are preferred (more specific)
		if score > 0 {
			score += 10 - len(strings.Fields(club.Name))
		}

		if score > bestScore {
			bestScore = score
			bestClub = &clubs[i]
		}
	}

	if bestScore > 0 {
		return bestClub
	}
	return nil
}

// updateClubFromScrapedInfo updates a club model with scraped data, returning whether any changes were made
func (s *ClubScraper) updateClubFromScrapedInfo(club *models.Club, info *ScrapedClubInfo) bool {
	changed := false

	// Update address if we have scraped data
	if info.Address != "" && club.Address != info.Address {
		club.Address = info.Address
		changed = true
	}

	// Update website
	if info.Website != "" && club.Website != info.Website {
		club.Website = info.Website
		changed = true
	}

	// Update phone number
	if info.Phone != "" && club.PhoneNumber != info.Phone {
		club.PhoneNumber = info.Phone
		changed = true
	}

	// Update optional fields when scraped data is available
	if info.Postcode != "" && (club.Postcode == nil || *club.Postcode != info.Postcode) {
		club.Postcode = strPtr(info.Postcode)
		changed = true
	}

	if info.AddressLine1 != "" && (club.AddressLine1 == nil || *club.AddressLine1 != info.AddressLine1) {
		club.AddressLine1 = strPtr(info.AddressLine1)
		changed = true
	}

	if info.AddressLine2 != "" && (club.AddressLine2 == nil || *club.AddressLine2 != info.AddressLine2) {
		club.AddressLine2 = strPtr(info.AddressLine2)
		changed = true
	}

	if info.City != "" && (club.City == nil || *club.City != info.City) {
		club.City = strPtr(info.City)
		changed = true
	}

	if info.Latitude != nil && (club.Latitude == nil || *club.Latitude != *info.Latitude) {
		club.Latitude = info.Latitude
		changed = true
	}

	if info.Longitude != nil && (club.Longitude == nil || *club.Longitude != *info.Longitude) {
		club.Longitude = info.Longitude
		changed = true
	}

	return changed
}

// buildClubFromScrapedInfo creates a new Club model from scraped data
func (s *ClubScraper) buildClubFromScrapedInfo(info *ScrapedClubInfo) *models.Club {
	club := &models.Club{
		Name:        info.Name,
		Address:     info.Address,
		Website:     info.Website,
		PhoneNumber: info.Phone,
		Latitude:    info.Latitude,
		Longitude:   info.Longitude,
	}

	if info.Postcode != "" {
		club.Postcode = strPtr(info.Postcode)
	}
	if info.AddressLine1 != "" {
		club.AddressLine1 = strPtr(info.AddressLine1)
	}
	if info.AddressLine2 != "" {
		club.AddressLine2 = strPtr(info.AddressLine2)
	}
	if info.City != "" {
		club.City = strPtr(info.City)
	}

	return club
}

// printScrapedInfo prints scraped club info for verbose output
func (s *ClubScraper) printScrapedInfo(info *ScrapedClubInfo) {
	fmt.Printf("  Name:          %s\n", info.Name)
	fmt.Printf("  Address:       %s\n", info.Address)
	fmt.Printf("  AddressLine1:  %s\n", info.AddressLine1)
	fmt.Printf("  AddressLine2:  %s\n", info.AddressLine2)
	fmt.Printf("  City:          %s\n", info.City)
	fmt.Printf("  Postcode:      %s\n", info.Postcode)
	fmt.Printf("  Website:       %s\n", info.Website)
	fmt.Printf("  Email:         %s\n", info.Email)
	fmt.Printf("  Phone:         %s\n", info.Phone)
	if info.Latitude != nil && info.Longitude != nil {
		fmt.Printf("  Coordinates:   %f, %f\n", *info.Latitude, *info.Longitude)
	} else {
		fmt.Printf("  Coordinates:   (not found)\n")
	}
}

// strPtr returns a pointer to a string
func strPtr(s string) *string {
	return &s
}
