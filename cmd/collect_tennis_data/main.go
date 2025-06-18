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
	atpPlayers, err := collectPlayersFromTennisAbstract(true, 100)
	if err != nil {
		log.Fatalf("Error collecting ATP players: %v", err)
	}
	log.Printf("Successfully collected %d ATP players", len(atpPlayers))

	// Collect WTA players
	wtaPlayers, err := collectPlayersFromTennisAbstract(false, 100)
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
				if profile.HighestRank == 0 {
					profile.HighestRank = wikiData.HighestRank
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
	BirthDate   string
	BirthPlace  string
	Hand        string
	YearPro     int
	HighestRank int
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

			// Clean up HTML entities and normalize text
			label = cleanText(label)
			value = cleanText(value)

			switch {
			case strings.Contains(strings.ToLower(label), "born"):
				// Extract birth date and place from complex format like:
				// "(2003-05-05) 5 May 2003 (age 22)El Palmar, Murcia, Spain"
				// First, try to extract the date from the ISO format in parentheses
				isoDateMatch := regexp.MustCompile(`\((\d{4}-\d{2}-\d{2})\)`).FindStringSubmatch(value)
				if len(isoDateMatch) > 1 {
					// Convert ISO date to readable format
					isoDate := isoDateMatch[1]
					parts := strings.Split(isoDate, "-")
					if len(parts) == 3 {
						year := parts[0]
						month := parts[1]
						day := parts[2]
						// Convert month number to name
						monthNames := map[string]string{
							"01": "January", "02": "February", "03": "March", "04": "April",
							"05": "May", "06": "June", "07": "July", "08": "August",
							"09": "September", "10": "October", "11": "November", "12": "December",
						}
						if monthName, ok := monthNames[month]; ok {
							data.BirthDate = fmt.Sprintf("%s %s, %s", monthName, day, year)
						}
					}
				}

				// Extract birth place - look for the last part after the age
				ageMatch := regexp.MustCompile(`\(age\s+\d+\)`).FindStringSubmatch(value)
				if len(ageMatch) > 0 {
					// Remove the age part and everything before it
					parts := strings.Split(value, ageMatch[0])
					if len(parts) > 1 {
						birthPlace := strings.TrimSpace(parts[1])
						// Clean up any remaining parentheses or extra text
						birthPlace = regexp.MustCompile(`^[^a-zA-Z]*`).ReplaceAllString(birthPlace, "")
						if birthPlace != "" {
							data.BirthPlace = birthPlace
						}
					}
				}

				// Fallback: if we didn't get birth place from above method, try simple comma split
				if data.BirthPlace == "" {
					// Remove the ISO date and age parts
					cleanValue := regexp.MustCompile(`\(\d{4}-\d{2}-\d{2}\)\s*`).ReplaceAllString(value, "")
					cleanValue = regexp.MustCompile(`\(age\s+\d+\)\s*`).ReplaceAllString(cleanValue, "")

					// Look for the last comma-separated part as birth place
					parts := strings.Split(cleanValue, ",")
					if len(parts) > 1 {
						// Take the last two parts as city, country
						if len(parts) >= 2 {
							city := strings.TrimSpace(parts[len(parts)-2])
							country := strings.TrimSpace(parts[len(parts)-1])
							data.BirthPlace = fmt.Sprintf("%s, %s", city, country)
						}
					}
				}

			case strings.Contains(strings.ToLower(label), "plays"):
				data.Hand = value
			case strings.Contains(strings.ToLower(label), "turned pro"):
				if year, err := strconv.Atoi(value); err == nil {
					data.YearPro = year
				}
			case strings.Contains(strings.ToLower(label), "highest ranking"):
				// Extract the number from "No. 1 (12 September 2022)"
				// Only process singles rankings, not doubles
				if !strings.Contains(strings.ToLower(label), "doubles") {
					rankMatch := regexp.MustCompile(`No\.\s*(\d+)`).FindStringSubmatch(value)
					if len(rankMatch) > 1 {
						if rank, err := strconv.Atoi(rankMatch[1]); err == nil {
							// Only update if we haven't found a ranking yet, or if this is a better (lower) ranking
							if data.HighestRank == 0 || rank < data.HighestRank {
								data.HighestRank = rank
							}
						}
					}
				}
			}
		})
	})

	return data, nil
}

func cleanText(text string) string {
	// Replace HTML entities
	text = strings.ReplaceAll(text, "&#160;", " ") // non-breaking space
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")

	// Normalize whitespace
	text = strings.Join(strings.Fields(text), " ")

	return text
}
