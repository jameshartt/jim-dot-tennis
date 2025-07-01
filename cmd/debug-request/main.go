package main

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func main() {
	// Exact same data as the working curl
	formData := "nonce=a28808dc3b&action=bhplta_club_scores_get_scores_week_change&selected_week=1&year=2025&club_id=10&club_name=St+Anns&passcode="

	// Create request
	req, err := http.NewRequest("POST", "https://www.bhplta.co.uk/wp-admin/admin-ajax.php", strings.NewReader(formData))
	if err != nil {
		panic(err)
	}

	// Set exact same headers as the working curl, but prefer gzip over brotli
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:140.0) Gecko/20100101 Firefox/140.0")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-GB,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate") // Remove br and zstd to force gzip
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Origin", "https://www.bhplta.co.uk")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Referer", "https://www.bhplta.co.uk/bhplta_tables/parks-league-match-cards/?id=3356")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Priority", "u=0")
	req.Header.Set("TE", "trailers")

	// Set cookies exactly as in the working curl (URL-encoded)
	cookieValue := "wordpress_sec_d9e736f9c59ae0b57f0c59c5392dc843=St%20Anns%7C1751455274%7CEtPSwgycHbBo0R6DumkDYnJsjdn9WyV9oQARpt0EtdD%7C829751d744b5000185d44e937aadf143cbdc07fbdc7e7a17e366c4b0f24b4834; clubcode=resident-beard-font; wordpress_test_cookie=WP%20Cookie%20check; wordpress_logged_in_d9e736f9c59ae0b57f0c59c5392dc843=St%20Anns%7C1751455274%7CEtPSwgycHbBo0R6DumkDYnJsjdn9WyV9oQARpt0EtdD%7C85d3a05f8410be0dab625c3b7c68adc3aa91d75e3ef70534b221ce5632522a53"
	req.Header.Set("Cookie", cookieValue)

	fmt.Printf("Making request...\n")

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	fmt.Printf("Response Status: %d %s\n", resp.StatusCode, resp.Status)
	fmt.Printf("Content-Encoding: %s\n", resp.Header.Get("Content-Encoding"))

	// Handle compressed response
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		fmt.Printf("Decompressing gzip response...\n")
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			panic(fmt.Sprintf("Failed to create gzip reader: %v", err))
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	// Read response
	body, err := io.ReadAll(reader)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Response Length: %d bytes\n", len(body))
	if len(body) > 300 {
		fmt.Printf("First 300 chars: %s\n", string(body[:300]))
	} else {
		fmt.Printf("Full response: %s\n", string(body))
	}
}
