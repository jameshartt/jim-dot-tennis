package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type TennisPlayer struct {
	ID           int    `json:"id"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	CommonName   string `json:"common_name"`
	Nationality  string `json:"nationality"`
	Gender       string `json:"gender"`
	CurrentRank  int    `json:"current_rank"`
	HighestRank  int    `json:"highest_rank"`
	YearPro      int    `json:"year_pro"`
	WikipediaURL string `json:"wikipedia_url"`
	Hand         string `json:"hand"`
	BirthDate    string `json:"birth_date"`
	BirthPlace   string `json:"birth_place"`
}

type PlayerData struct {
	LastUpdated string         `json:"last_updated"`
	ATPPlayers  []TennisPlayer `json:"atp_players"`
	WTAPlayers  []TennisPlayer `json:"wta_players"`
}

type PlayerProfile struct {
	WikipediaURL string
	BirthDate    string
	BirthPlace   string
	Hand         string
	YearPro      int
	HighestRank  int
	Nationality  string
}

func main() {
	log.Println("Starting tennis player data collection...")

	// Collect ATP players
	atpPlayers, err := collectPlayersFromTennisAbstract(true, 10)
	if err != nil {
		log.Fatalf("Error collecting ATP players: %v", err)
	}
	log.Printf("Successfully collected %d ATP players", len(atpPlayers))

	// Collect WTA players
	wtaPlayers, err := collectPlayersFromTennisAbstract(false, 10)
	if err != nil {
		log.Fatalf("Error collecting WTA players: %v", err)
	}
	log.Printf("Successfully collected %d WTA players", len(wtaPlayers))

	// Create the player data structure
	playerData := PlayerData{
		LastUpdated: time.Now().Format("2006-01-02T15:04:05Z"),
		ATPPlayers:  atpPlayers,
		WTAPlayers:  wtaPlayers,
	}

	// Output to JSON file
	jsonData, err := json.MarshalIndent(playerData, "", "  ")
	if err != nil {
		log.Fatalf("Error marshaling JSON: %v", err)
	}

	err = os.WriteFile("tennis_players.json", jsonData, 0644)
	if err != nil {
		log.Fatalf("Error writing JSON file: %v", err)
	}

	log.Printf("Successfully wrote tennis player data to tennis_players.json")
	log.Printf("Total players: %d ATP + %d WTA = %d",
		len(playerData.ATPPlayers),
		len(playerData.WTAPlayers),
		len(playerData.ATPPlayers)+len(playerData.WTAPlayers))

	// Display sample for verification
	fmt.Println("\nSample ATP Player:")
	if len(playerData.ATPPlayers) > 0 {
		sampleJSON, _ := json.MarshalIndent(playerData.ATPPlayers[0], "", "  ")
		fmt.Println(string(sampleJSON))
	}

	fmt.Println("\nSample WTA Player:")
	if len(playerData.WTAPlayers) > 0 {
		sampleJSON, _ := json.MarshalIndent(playerData.WTAPlayers[0], "", "  ")
		fmt.Println(string(sampleJSON))
	}
}

func collectPlayersFromTennisAbstract(isATP bool, limit int) ([]TennisPlayer, error) {
	var url string
	if isATP {
		url = "https://www.tennisabstract.com/reports/atp_elo_ratings.html"
	} else {
		url = "https://www.tennisabstract.com/reports/wta_elo_ratings.html"
	}

	log.Printf("Fetching data from %s", url)
	doc, err := fetchDocument(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch rankings: %v", err)
	}

	var players []TennisPlayer
	startID := 1
	if !isATP {
		startID = 1001 // Start WTA IDs at 1001
	}

	playerID := startID
	gender := "Male"
	if !isATP {
		gender = "Female"
	}

	// Parse the rankings table
	doc.Find("table tr").Each(func(i int, s *goquery.Selection) {
		if len(players) >= limit || i == 0 { // Skip header row
			return
		}

		// Extract rank and player name
		cells := s.Find("td")
		if cells.Length() < 2 {
			return
		}

		rankText := strings.TrimSpace(cells.Eq(0).Text())
		rank, err := strconv.Atoi(rankText)
		if err != nil {
			log.Printf("Warning: Could not parse rank '%s': %v", rankText, err)
			return
		}

		playerLink := cells.Eq(1).Find("a")
		if playerLink.Length() == 0 {
			log.Printf("Warning: No player link found in row %d", i)
			return
		}

		playerName := strings.TrimSpace(playerLink.Text())
		firstName, lastName := parsePlayerName(playerName)

		// Get player profile URL
		profileURL, exists := playerLink.Attr("href")
		if !exists {
			log.Printf("Warning: No profile URL for player %s", playerName)
			return
		}
		// Make URL absolute if it's relative
		if !strings.HasPrefix(profileURL, "http") {
			profileURL = "https://www.tennisabstract.com" + profileURL
		}

		// Try to get nationality from the next cell if available
		nationality := "Unknown"
		// Note: Nationality is not available in the rankings table, will be extracted from profile page

		// Get detailed profile information
		profile, err := fetchPlayerProfile(profileURL)
		if err != nil {
			log.Printf("Warning: Could not fetch profile for %s: %v", playerName, err)
			// Continue with basic info
			profile = &PlayerProfile{
				WikipediaURL: generateWikipediaURL(playerName),
			}
		}

		// If no Wikipedia URL was found in the profile, try constructing one from the player name
		if profile.WikipediaURL == "" {
			log.Printf("No Wikipedia URL found in profile for %s, trying to construct one from name", playerName)
			constructedURL, err := tryWikipediaURLFromName(playerName)
			if err != nil {
				log.Printf("Warning: Could not find valid Wikipedia page for %s: %v", playerName, err)
				// Fall back to generated URL (may not be valid)
				profile.WikipediaURL = generateWikipediaURL(playerName)
			} else {
				profile.WikipediaURL = constructedURL
				log.Printf("Successfully found Wikipedia URL for %s: %s", playerName, constructedURL)
			}
		}

		// Get Wikipedia data if we have a URL
		if profile.WikipediaURL != "" {
			wikiData, err := fetchWikipediaData(profile.WikipediaURL)
			if err != nil {
				log.Printf("Warning: Could not fetch Wikipedia data for %s: %v", playerName, err)
			} else {
				// Update profile with Wikipedia data if available
				if profile.BirthDate == "" {
					profile.BirthDate = wikiData.BirthDate
				}
				if profile.BirthPlace == "" {
					profile.BirthPlace = wikiData.BirthPlace
				}
				if profile.Hand == "" {
					profile.Hand = wikiData.Hand
				}
				if profile.YearPro == 0 {
					profile.YearPro = wikiData.YearPro
				}
			}
		}

		// Use nationality from profile if available
		if profile.Nationality != "" {
			nationality = profile.Nationality
		}

		player := TennisPlayer{
			ID:           playerID,
			FirstName:    firstName,
			LastName:     lastName,
			CommonName:   playerName,
			Nationality:  nationality,
			Gender:       gender,
			CurrentRank:  rank,
			HighestRank:  profile.HighestRank,
			YearPro:      profile.YearPro,
			WikipediaURL: profile.WikipediaURL,
			Hand:         profile.Hand,
			BirthDate:    profile.BirthDate,
			BirthPlace:   profile.BirthPlace,
		}

		players = append(players, player)
		playerID++

		// Add a small delay to be nice to the servers
		time.Sleep(5000 * time.Millisecond)
	})

	if len(players) == 0 {
		return nil, fmt.Errorf("no players found in the rankings table")
	}

	log.Printf("Successfully parsed %d players from %s", len(players), url)
	return players, nil
}

func fetchDocument(url string) (*goquery.Document, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Tennis-Data-Collector/1.0)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status code error: %d %s", resp.StatusCode, resp.Status)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

func parsePlayerName(fullName string) (firstName, lastName string) {
	parts := strings.Fields(fullName)
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}

	firstName = parts[0]
	lastName = strings.Join(parts[1:], " ")
	return firstName, lastName
}

func generateWikipediaURL(playerName string) string {
	// Clean the name for Wikipedia URL format - replace spaces with underscores
	cleanName := strings.ReplaceAll(playerName, " ", "_")

	return fmt.Sprintf("https://en.wikipedia.org/wiki/%s", cleanName)
}

// tryWikipediaURLFromName attempts to find a Wikipedia page for a player by constructing a URL from their name
func tryWikipediaURLFromName(playerName string) (string, error) {
	// Clean the name for Wikipedia URL format - replace spaces with underscores
	cleanName := strings.ReplaceAll(playerName, " ", "_")

	// Remove special characters that might cause issues, but keep underscores
	reg := regexp.MustCompile(`[^a-zA-Z0-9_]`)
	cleanName = reg.ReplaceAllString(cleanName, "")

	wikiURL := fmt.Sprintf("https://en.wikipedia.org/wiki/%s", cleanName)

	log.Printf("Trying constructed Wikipedia URL: %s", wikiURL)

	// Try to fetch the page
	doc, err := fetchDocument(wikiURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch Wikipedia page: %v", err)
	}

	// Check if this is a valid tennis player page by looking for tennis-related content
	pageText := doc.Text()

	// Look for tennis-related keywords that would indicate this is a tennis player page
	tennisKeywords := []string{
		"tennis", "ATP", "WTA", "Grand Slam", "tournament", "ranking", "singles", "doubles",
		"Australian Open", "French Open", "Wimbledon", "US Open", "Davis Cup", "Fed Cup",
		"tour", "championship", "final", "semifinal", "quarterfinal", "round", "match",
		"serve", "forehand", "backhand", "volley", "ace", "break point", "set", "game",
		"coach", "player", "professional", "amateur", "junior", "career", "retired",
	}

	tennisKeywordCount := 0
	for _, keyword := range tennisKeywords {
		if strings.Contains(strings.ToLower(pageText), strings.ToLower(keyword)) {
			tennisKeywordCount++
		}
	}

	// Also check for disambiguation pages or "does not exist" indicators
	if strings.Contains(pageText, "may refer to:") ||
		strings.Contains(pageText, "disambiguation") ||
		strings.Contains(pageText, "Wikipedia does not have an article with this exact name") {
		log.Printf("Wikipedia page appears to be disambiguation or non-existent: %s", wikiURL)
		return "", fmt.Errorf("page is disambiguation or non-existent")
	}

	// If we found enough tennis-related keywords, consider this a valid tennis player page
	if tennisKeywordCount >= 3 {
		log.Printf("Found valid tennis player Wikipedia page with %d tennis keywords: %s", tennisKeywordCount, wikiURL)
		return wikiURL, nil
	}

	log.Printf("Wikipedia page does not appear to be a tennis player (only %d tennis keywords found): %s", tennisKeywordCount, wikiURL)
	return "", fmt.Errorf("page does not appear to be a tennis player")
}

func fetchPlayerProfile(url string) (*PlayerProfile, error) {
	log.Printf("Fetching player profile from %s", url)
	doc, err := fetchDocument(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch profile: %v", err)
	}

	profile := &PlayerProfile{
		WikipediaURL: "", // Will be updated if found
	}

	// Extract JavaScript variables from the page
	doc.Find("script").Each(func(i int, s *goquery.Selection) {
		scriptContent := s.Text()

		// Extract country/nationality from JavaScript variable
		if strings.Contains(scriptContent, "var country =") {
			countryMatch := regexp.MustCompile(`var country = '([^']+)'`).FindStringSubmatch(scriptContent)
			if len(countryMatch) > 1 {
				profile.Nationality = countryMatch[1]
				log.Printf("Found nationality: %s", profile.Nationality)
			}
		}

		// Extract Wikipedia ID from JavaScript variable
		if strings.Contains(scriptContent, "var wiki_id =") {
			wikiMatch := regexp.MustCompile(`var wiki_id = '([^']+)'`).FindStringSubmatch(scriptContent)
			if len(wikiMatch) > 1 {
				wikiID := wikiMatch[1]
				if wikiID != "" {
					profile.WikipediaURL = fmt.Sprintf("https://en.wikipedia.org/wiki/%s", wikiID)
					log.Printf("Found Wikipedia URL: %s", profile.WikipediaURL)
				}
			}
		}
	})

	// If no Wikipedia URL found from JavaScript, try the old method as fallback
	if profile.WikipediaURL == "" {
		// Find the Wikipedia link in the profile - look for the specific pattern
		doc.Find("a").Each(func(i int, s *goquery.Selection) {
			href, exists := s.Attr("href")
			if !exists {
				return
			}

			// Look for links with text "Wikipedia" and target="_blank"
			linkText := strings.TrimSpace(s.Text())
			target, hasTarget := s.Attr("target")

			if linkText == "Wikipedia" && hasTarget && target == "_blank" &&
				(strings.Contains(href, "wikipedia.org/wiki/") || strings.Contains(href, "en.wikipedia.org/wiki/")) {
				profile.WikipediaURL = href
				log.Printf("Found Wikipedia URL (fallback): %s", href)
			}
		})

		// If no Wikipedia URL found with the specific pattern, try a more general search
		if profile.WikipediaURL == "" {
			doc.Find("a").Each(func(i int, s *goquery.Selection) {
				href, exists := s.Attr("href")
				if !exists {
					return
				}

				// Look for any link containing wikipedia.org/wiki/
				if strings.Contains(href, "wikipedia.org/wiki/") || strings.Contains(href, "en.wikipedia.org/wiki/") {
					linkText := strings.TrimSpace(s.Text())
					// Check if the link text looks like it could be a Wikipedia link
					if linkText == "Wikipedia" || linkText == "wiki" ||
						strings.Contains(strings.ToLower(linkText), "wikipedia") ||
						strings.Contains(strings.ToLower(linkText), "wiki") {
						profile.WikipediaURL = href
						log.Printf("Found Wikipedia URL (general search): %s", href)
					}
				}
			})
		}
	}

	// Extract player details from the profile page
	doc.Find("table").Each(func(i int, table *goquery.Selection) {
		// Look for the player info table - be more specific
		tableText := table.Text()
		if !strings.Contains(tableText, "Birth") &&
			!strings.Contains(tableText, "Turned Pro") &&
			!strings.Contains(tableText, "Plays") {
			return
		}

		table.Find("tr").Each(func(j int, row *goquery.Selection) {
			cells := row.Find("td")
			if cells.Length() < 2 {
				return
			}

			label := strings.TrimSpace(cells.Eq(0).Text())
			value := strings.TrimSpace(cells.Eq(1).Text())

			switch {
			case strings.Contains(label, "Birth"):
				if strings.Contains(label, "Place") {
					profile.BirthPlace = value
				} else if strings.Contains(label, "Date") {
					profile.BirthDate = value
				}
			case strings.Contains(label, "Turned Pro"):
				if year, err := strconv.Atoi(value); err == nil {
					profile.YearPro = year
				}
			case strings.Contains(label, "Highest Rank"):
				if rank, err := strconv.Atoi(value); err == nil {
					profile.HighestRank = rank
				}
			case strings.Contains(label, "Plays"):
				profile.Hand = value
			}
		})
	})

	// Also try to find info in div elements or other structures
	doc.Find("div").Each(func(i int, div *goquery.Selection) {
		divText := div.Text()
		if strings.Contains(divText, "Birth") || strings.Contains(divText, "Turned Pro") {
			// Look for specific patterns in the text
			if strings.Contains(divText, "Birth:") {
				parts := strings.Split(divText, "Birth:")
				if len(parts) > 1 {
					birthInfo := strings.TrimSpace(parts[1])
					// Extract birth date and place
					lines := strings.Split(birthInfo, "\n")
					for _, line := range lines {
						line = strings.TrimSpace(line)
						if strings.Contains(line, ",") {
							datePlace := strings.Split(line, ",")
							if len(datePlace) > 0 {
								profile.BirthDate = strings.TrimSpace(datePlace[0])
							}
							if len(datePlace) > 1 {
								profile.BirthPlace = strings.TrimSpace(datePlace[1])
							}
							break
						}
					}
				}
			}
		}
	})

	log.Printf("Profile data extracted - Nationality: %s, Wikipedia: %s, Birth: %s, Place: %s, Hand: %s, YearPro: %d, HighestRank: %d",
		profile.Nationality, profile.WikipediaURL, profile.BirthDate, profile.BirthPlace, profile.Hand, profile.YearPro, profile.HighestRank)

	return profile, nil
}

type WikipediaData struct {
	BirthDate  string
	BirthPlace string
	Hand       string
	YearPro    int
}

func fetchWikipediaData(url string) (*WikipediaData, error) {
	log.Printf("Fetching Wikipedia data from %s", url)
	doc, err := fetchDocument(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Wikipedia page: %v", err)
	}

	data := &WikipediaData{}

	// Find the infobox table
	doc.Find("table.infobox").Each(func(i int, table *goquery.Selection) {
		table.Find("tr").Each(func(j int, row *goquery.Selection) {
			th := row.Find("th")
			td := row.Find("td")
			if th.Length() == 0 || td.Length() == 0 {
				return
			}

			label := strings.TrimSpace(th.Text())
			value := strings.TrimSpace(td.Text())

			switch {
			case strings.Contains(label, "Born"):
				// Extract birth date and place
				parts := strings.Split(value, "(")
				if len(parts) > 0 {
					birthInfo := strings.TrimSpace(parts[0])
					datePlace := strings.Split(birthInfo, ",")
					if len(datePlace) > 0 {
						data.BirthDate = strings.TrimSpace(datePlace[0])
					}
					if len(datePlace) > 1 {
						data.BirthPlace = strings.TrimSpace(datePlace[1])
					}
				}
			case strings.Contains(label, "Plays"):
				data.Hand = value
			case strings.Contains(label, "Turned pro"):
				if year, err := strconv.Atoi(value); err == nil {
					data.YearPro = year
				}
			}
		})
	})

	return data, nil
}
