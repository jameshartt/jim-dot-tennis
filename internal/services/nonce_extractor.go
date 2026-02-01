package services

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// NonceExtractor handles extracting WordPress nonces from BHPLTA website
type NonceExtractor struct {
	httpClient *http.Client
	baseURL    string
}

// NonceResult contains the extracted nonce and related information
type NonceResult struct {
	Nonce     string
	ExpiresAt time.Time
	ClubCode  string // If available from cookies
}

// NewNonceExtractor creates a new nonce extractor
func NewNonceExtractor() *NonceExtractor {
	return &NonceExtractor{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: "https://www.bhplta.co.uk/bhplta_tables/parks-league-match-cards/",
	}
}

// ExtractNonce fetches the BHPLTA page and extracts the WordPress nonce
func (n *NonceExtractor) ExtractNonce() (*NonceResult, error) {
	// Make request to the match cards page
	req, err := http.NewRequest("GET", n.baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers to look like a regular browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:140.0) Gecko/20100101 Firefox/140.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-GB,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
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

	// Read response body first for debugging
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse the HTML
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Method 1: Look for the nonce in JavaScript variables
	nonce := n.extractNonceFromScript(doc)
	if nonce != "" {
		return &NonceResult{
			Nonce:     nonce,
			ExpiresAt: time.Now().Add(12 * time.Hour), // WordPress nonces typically expire in 12-24 hours
		}, nil
	}

	// Method 2: Look for nonce in hidden form fields
	nonce = n.extractNonceFromForms(doc)
	if nonce != "" {
		return &NonceResult{
			Nonce:     nonce,
			ExpiresAt: time.Now().Add(12 * time.Hour),
		}, nil
	}

	// Method 3: Look for nonce in data attributes
	nonce = n.extractNonceFromDataAttributes(doc)
	if nonce != "" {
		return &NonceResult{
			Nonce:     nonce,
			ExpiresAt: time.Now().Add(12 * time.Hour),
		}, nil
	}

	return nil, fmt.Errorf("could not find nonce in page")
}

// ExtractNonceWithClubCode extracts nonce when already having a club code cookie
func (n *NonceExtractor) ExtractNonceWithClubCode(clubCode string) (*NonceResult, error) {
	req, err := http.NewRequest("GET", n.baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers and club code cookie
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:140.0) Gecko/20100101 Firefox/140.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-GB,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Cookie", fmt.Sprintf("clubcode=%s", clubCode))

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
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

	// Read response body first for debugging
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	nonce := n.extractNonceFromScript(doc)
	if nonce == "" {
		nonce = n.extractNonceFromForms(doc)
	}
	if nonce == "" {
		nonce = n.extractNonceFromDataAttributes(doc)
	}

	if nonce != "" {
		return &NonceResult{
			Nonce:     nonce,
			ExpiresAt: time.Now().Add(12 * time.Hour),
			ClubCode:  clubCode,
		}, nil
	}

	return nil, fmt.Errorf("could not find nonce in page")
}

// extractNonceFromScript looks for nonce in JavaScript variables like my_ajax_object2.nonce
func (n *NonceExtractor) extractNonceFromScript(doc *goquery.Document) string {
	var nonce string

	// Look through all script tags
	doc.Find("script").Each(func(i int, s *goquery.Selection) {
		scriptContent := s.Text()

		// Pattern 1: my_ajax_object2.nonce = "nonce_value"
		if strings.Contains(scriptContent, "my_ajax_object2") && strings.Contains(scriptContent, "nonce") {
			// Look for the nonce value in the my_ajax_object2 object - handle escaped JSON
			nonceRegex := regexp.MustCompile(`"nonce"\s*:\s*"([^"]+)"`)
			matches := nonceRegex.FindStringSubmatch(scriptContent)
			if len(matches) > 1 {
				nonce = matches[1]
				return
			}

			// Alternative pattern: my_ajax_object2 = {"nonce":"value",...}
			objectRegex := regexp.MustCompile(`my_ajax_object2\s*=\s*\{[^}]*"nonce"\s*:\s*"([^"]+)"`)
			matches = objectRegex.FindStringSubmatch(scriptContent)
			if len(matches) > 1 {
				nonce = matches[1]
				return
			}

			// Pattern for inline variable assignment: var my_ajax_object2 = {...}
			varRegex := regexp.MustCompile(`var\s+my_ajax_object2\s*=\s*\{[^}]*"nonce"\s*:\s*"([^"]+)"`)
			matches = varRegex.FindStringSubmatch(scriptContent)
			if len(matches) > 1 {
				nonce = matches[1]
				return
			}
		}

		// Pattern 2: Direct nonce variable
		if strings.Contains(scriptContent, "var nonce") {
			nonceRegex := regexp.MustCompile(`var\s+nonce\s*=\s*["']([^"']+)["']`)
			matches := nonceRegex.FindStringSubmatch(scriptContent)
			if len(matches) > 1 {
				nonce = matches[1]
				return
			}
		}

		// Pattern 3: WordPress wp_localize_script pattern
		wpNonceRegex := regexp.MustCompile(`wp_nonce['"]\s*:\s*['"]([^'"]+)['"]`)
		matches := wpNonceRegex.FindStringSubmatch(scriptContent)
		if len(matches) > 1 {
			nonce = matches[1]
			return
		}
	})

	return nonce
}

// extractNonceFromForms looks for nonce in hidden form fields
func (n *NonceExtractor) extractNonceFromForms(doc *goquery.Document) string {
	var nonce string

	// Look for hidden inputs with nonce-related names
	doc.Find("input[type=hidden]").Each(func(i int, s *goquery.Selection) {
		name, exists := s.Attr("name")
		if !exists {
			return
		}

		// Common WordPress nonce field patterns
		if strings.Contains(name, "nonce") || strings.Contains(name, "_wpnonce") {
			value, exists := s.Attr("value")
			if exists && value != "" {
				nonce = value
				return
			}
		}
	})

	return nonce
}

// extractNonceFromDataAttributes looks for nonce in HTML data attributes
func (n *NonceExtractor) extractNonceFromDataAttributes(doc *goquery.Document) string {
	var nonce string

	// Look for elements with data-nonce attributes
	doc.Find("[data-nonce]").Each(func(i int, s *goquery.Selection) {
		value, exists := s.Attr("data-nonce")
		if exists && value != "" {
			nonce = value
			return
		}
	})

	// Look for elements with data-wp-nonce attributes
	doc.Find("[data-wp-nonce]").Each(func(i int, s *goquery.Selection) {
		value, exists := s.Attr("data-wp-nonce")
		if exists && value != "" {
			nonce = value
			return
		}
	})

	return nonce
}
