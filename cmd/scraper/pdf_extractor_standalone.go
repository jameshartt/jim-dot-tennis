package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/gen2brain/go-fitz"
)

// FixtureRow represents a single fixture row in the CSV
type FixtureRow struct {
	Week           int
	Date           string
	HomeTeamFirst  string
	AwayTeamFirst  string
	HomeTeamSecond string
	AwayTeamSecond string
}

// DivisionPDFConfig holds configuration for each division PDF
type DivisionPDFConfig struct {
	Name     string
	URL      string
	Filename string
}

// Division PDF configurations
var divisionPDFs = []DivisionPDFConfig{
	{
		Name:     "Division 1",
		URL:      "https://www.bhplta.co.uk/wp-content/uploads/2025/03/Div_1_2025.pdf",
		Filename: "Div_1_2025.pdf",
	},
	{
		Name:     "Division 2",
		URL:      "https://www.bhplta.co.uk/wp-content/uploads/2025/03/Div_2_2025.pdf",
		Filename: "Div_2_2025.pdf",
	},
	{
		Name:     "Division 3",
		URL:      "https://www.bhplta.co.uk/wp-content/uploads/2025/03/Div_3_2025.pdf",
		Filename: "Div_3_2025.pdf",
	},
	{
		Name:     "Division 4",
		URL:      "https://www.bhplta.co.uk/wp-content/uploads/2025/03/Div_4_2025.pdf",
		Filename: "Div_4_2025.pdf",
	},
}

// ProcessDivisionPDFs downloads and extracts text from all division PDFs
func ProcessDivisionPDFs(outputDir string) error {
	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	for _, config := range divisionPDFs {
		fmt.Printf("Processing %s...\n", config.Name)

		// Download PDF
		pdfPath := filepath.Join(outputDir, config.Filename)
		if err := downloadPDF(config.URL, pdfPath); err != nil {
			return fmt.Errorf("failed to download %s: %w", config.Name, err)
		}

		// Extract text from PDF
		text, err := extractTextFromPDF(pdfPath)
		if err != nil {
			return fmt.Errorf("failed to extract text from %s: %w", config.Name, err)
		}

		// Save raw text for debugging
		textPath := filepath.Join(outputDir, strings.ReplaceAll(config.Filename, ".pdf", "_text.txt"))
		if err := os.WriteFile(textPath, []byte(text), 0644); err != nil {
			fmt.Printf("Warning: failed to save text file for %s: %v\n", config.Name, err)
		} else {
			fmt.Printf("📝 Saved extracted text to: %s\n", textPath)
		}

		// Parse fixtures from text
		fixtures, err := parseFixturesFromText(text, config.Name)
		if err != nil {
			return fmt.Errorf("failed to parse fixtures from %s: %w", config.Name, err)
		}

		// Save to CSV
		csvPath := filepath.Join(outputDir, strings.ReplaceAll(config.Filename, ".pdf", "_fixtures.csv"))
		if err := saveFixturesToCSV(fixtures, csvPath); err != nil {
			return fmt.Errorf("failed to save CSV for %s: %w", config.Name, err)
		}

		fmt.Printf("✅ Saved %s fixtures to: %s (%d rows)\n", config.Name, csvPath, len(fixtures))

		// Clean up PDF file
		os.Remove(pdfPath)
	}

	return nil
}

// downloadPDF downloads a PDF from the given URL to the specified path
func downloadPDF(url, filepath string) error {
	fmt.Printf("Downloading PDF from %s...\n", url)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download PDF: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download PDF: status %d", resp.StatusCode)
	}

	file, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save PDF: %w", err)
	}

	fmt.Printf("PDF downloaded successfully: %s\n", filepath)
	return nil
}

// extractTextFromPDF extracts readable text from a PDF file using go-fitz
func extractTextFromPDF(pdfPath string) (string, error) {
	doc, err := fitz.New(pdfPath)
	if err != nil {
		return "", fmt.Errorf("failed to open PDF: %w", err)
	}
	defer doc.Close()

	numPages := doc.NumPage()
	fmt.Printf("PDF has %d pages\n", numPages)

	var allText strings.Builder

	// Extract text from each page
	for pageNum := 0; pageNum < numPages; pageNum++ {
		text, err := doc.Text(pageNum)
		if err != nil {
			fmt.Printf("Warning: failed to extract text from page %d: %v\n", pageNum+1, err)
			continue
		}

		if len(strings.TrimSpace(text)) > 0 {
			allText.WriteString(fmt.Sprintf("=== PAGE %d ===\n", pageNum+1))
			allText.WriteString(text)
			allText.WriteString("\n\n")
		}
	}

	return allText.String(), nil
}

// parseFixturesFromText parses fixture data from extracted text
func parseFixturesFromText(text, divisionName string) ([]FixtureRow, error) {
	fmt.Printf("Parsing fixtures for %s from %d characters of text\n", divisionName, len(text))

	lines := strings.Split(text, "\n")
	var fixtures []FixtureRow

	// Regular expressions for parsing
	weekRegex := regexp.MustCompile(`^Wk\s+(\d+)$`)

	// Track which weeks we've already processed to avoid duplicates
	processedWeeks := make(map[int]bool)

	i := 0
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])

		// Skip empty lines and headers
		if line == "" || strings.Contains(line, "Fixtures") || strings.Contains(line, "Division") ||
			strings.Contains(line, "Home Team") || strings.Contains(line, "Wks") ||
			strings.Contains(line, "===") || strings.Contains(line, "PAGE") {
			i++
			continue
		}

		// Look for week number to start a fixture block
		if weekMatch := weekRegex.FindStringSubmatch(line); weekMatch != nil {
			firstWeek, err := strconv.Atoi(weekMatch[1])
			if err != nil {
				i++
				continue
			}

			// Only process weeks 1-9 as they start new fixture blocks
			// Weeks 10-18 are second halves and will be handled within the blocks
			// Also check if we've already processed this week to avoid duplicates
			if firstWeek <= 9 && !processedWeeks[firstWeek] {
				fmt.Printf("Found first-half week %d at line %d: '%s'\n", firstWeek, i+1, line)

				// Mark this week as processed
				processedWeeks[firstWeek] = true

				// Parse the complete fixture block
				blockFixtures, nextIndex := parseFixtureBlock(lines, i, firstWeek, divisionName)
				fixtures = append(fixtures, blockFixtures...)
				fmt.Printf("Parsed %d fixtures from week %d block, next index: %d\n", len(blockFixtures), firstWeek, nextIndex)
				i = nextIndex
			} else {
				// Skip weeks 10-18 as they are handled as second halves, or already processed weeks
				if firstWeek <= 9 {
					fmt.Printf("Skipping already processed week %d at line %d\n", firstWeek, i+1)
				} else {
					fmt.Printf("Skipping second-half week %d at line %d\n", firstWeek, i+1)
				}
				i++
			}
		} else {
			i++
		}
	}

	fmt.Printf("Extracted %d fixtures\n", len(fixtures))
	return fixtures, nil
}

// parseFixtureBlock parses a complete fixture block starting from the given index
func parseFixtureBlock(lines []string, startIndex, firstWeek int, divisionName string) ([]FixtureRow, int) {
	var fixtures []FixtureRow
	i := startIndex + 1 // Skip the "Wk X" line

	// Determine number of teams based on division
	teamCount := 5 // Default for Divisions 1-3
	if strings.Contains(divisionName, "Division 4") {
		teamCount = 6
	}

	fmt.Printf("  Parsing block starting at index %d for week %d (division: %s, teams: %d)\n", startIndex, firstWeek, divisionName, teamCount)

	// Parse first half
	firstMonth, firstDay, i := parseDate(lines, i)
	fmt.Printf("  First half date: %s %s, next index: %d\n", firstMonth, firstDay, i)

	homeTeams, i := parseTeams(lines, i, teamCount)
	fmt.Printf("  Home teams: %v, next index: %d\n", homeTeams, i)

	i = skipVs(lines, i, teamCount)
	fmt.Printf("  After skipping vs, next index: %d\n", i)

	awayTeams, i := parseTeams(lines, i, teamCount)
	fmt.Printf("  Away teams: %v, next index: %d\n", awayTeams, i)

	// Parse second half
	secondWeek := 0
	secondMonth := ""
	secondDay := ""

	// Look for second week number, skipping empty lines
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			i++
			continue
		}

		fmt.Printf("  Looking for second week at index %d: '%s'\n", i, line)
		if weekMatch := regexp.MustCompile(`^Wk\s+(\d+)$`).FindStringSubmatch(line); weekMatch != nil {
			week, err := strconv.Atoi(weekMatch[1])
			if err == nil {
				secondWeek = week
				i++
				fmt.Printf("  Found second week: %d, next index: %d\n", secondWeek, i)
				secondMonth, secondDay, i = parseDate(lines, i)
				fmt.Printf("  Second half date: %s %s, next index: %d\n", secondMonth, secondDay, i)
			}
		}
		break // Only check the first non-empty line
	}

	// Create fixtures for both halves if we have valid data
	if len(homeTeams) == teamCount && len(awayTeams) == teamCount {
		// First half fixtures
		firstDate := ""
		if firstMonth != "" && firstDay != "" {
			firstDate = fmt.Sprintf("%s %s", firstMonth, firstDay)
		}

		for j := 0; j < teamCount; j++ {
			fixture := FixtureRow{
				Week:           firstWeek,
				Date:           firstDate,
				HomeTeamFirst:  cleanTeamName(homeTeams[j]),
				AwayTeamFirst:  cleanTeamName(awayTeams[j]),
				HomeTeamSecond: "",
				AwayTeamSecond: "",
			}
			fixtures = append(fixtures, fixture)
		}

		// Second half fixtures (if we found the second week)
		if secondWeek > 0 {
			secondDate := ""
			if secondMonth != "" && secondDay != "" {
				secondDate = fmt.Sprintf("%s %s", secondMonth, secondDay)
			}

			for j := 0; j < teamCount; j++ {
				fixture := FixtureRow{
					Week:           secondWeek,
					Date:           secondDate,
					HomeTeamFirst:  "",
					AwayTeamFirst:  "",
					HomeTeamSecond: cleanTeamName(awayTeams[j]), // Away becomes home
					AwayTeamSecond: cleanTeamName(homeTeams[j]), // Home becomes away
				}
				fixtures = append(fixtures, fixture)
			}
		}
	}

	fmt.Printf("  Block complete, returning %d fixtures, next index: %d\n", len(fixtures), i)
	return fixtures, i
}

// parseDate parses month and day from consecutive lines
func parseDate(lines []string, startIndex int) (string, string, int) {
	monthRegex := regexp.MustCompile(`^(April|May|June|July|August|September|October|November|December)$`)
	dayRegex := regexp.MustCompile(`^(\d{1,2})(?:st|nd|rd|th)?$`)

	month := ""
	day := ""
	i := startIndex

	// Look for month
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			i++
			continue
		}
		if monthRegex.MatchString(line) {
			month = line
			i++
			break
		}
		i++
	}

	// Look for day
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			i++
			continue
		}
		if dayMatch := dayRegex.FindStringSubmatch(line); dayMatch != nil {
			day = dayMatch[1]
			i++
			break
		}
		i++
	}

	return month, day, i
}

// parseTeams parses a specified number of team names from consecutive lines
func parseTeams(lines []string, startIndex, count int) ([]string, int) {
	var teams []string
	i := startIndex

	for len(teams) < count && i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			i++
			continue
		}

		// Skip if this looks like a "v" or week marker
		if line == "v" || regexp.MustCompile(`^Wk\s+\d+$`).MatchString(line) {
			break
		}

		// Skip month/day lines
		monthRegex := regexp.MustCompile(`^(April|May|June|July|August|September|October|November|December)$`)
		dayRegex := regexp.MustCompile(`^(\d{1,2})(?:st|nd|rd|th)?$`)
		if monthRegex.MatchString(line) || dayRegex.MatchString(line) {
			break
		}

		teams = append(teams, line)
		i++
	}

	return teams, i
}

// skipVs skips the specified number of "v" lines
func skipVs(lines []string, startIndex, count int) int {
	i := startIndex
	skipped := 0

	for skipped < count && i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			i++
			continue
		}
		if line == "v" {
			skipped++
		}
		i++
	}

	return i
}

// cleanTeamName cleans up OCR artifacts from team names while preserving team identifiers
func cleanTeamName(name string) string {
	// Remove common OCR artifacts
	name = strings.ReplaceAll(name, "vy", "")
	name = strings.ReplaceAll(name, "vv", "")
	name = strings.ReplaceAll(name, "Vv", "")

	// Remove trailing numbers and ordinals that might be dates, but preserve team letters A-F
	name = regexp.MustCompile(`\s+\d+(st|nd|rd|th)?\s*$`).ReplaceAllString(name, "")

	// Remove standalone letters at the end EXCEPT for A, B, C, D, E, F which are team identifiers
	name = regexp.MustCompile(`\s+[G-Z]\s*$`).ReplaceAllString(name, "")
	name = regexp.MustCompile(`\s+[g-z]\s*$`).ReplaceAllString(name, "")

	// Clean up extra spaces
	name = regexp.MustCompile(`\s+`).ReplaceAllString(name, " ")

	return strings.TrimSpace(name)
}

// saveFixturesToCSV saves the fixtures to a CSV file
func saveFixturesToCSV(fixtures []FixtureRow, csvPath string) error {
	file, err := os.Create(csvPath)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"Week",
		"Date",
		"Home_Team_First_Half",
		"Away_Team_First_Half",
		"Home_Team_Second_Half",
		"Away_Team_Second_Half",
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write fixtures
	for _, fixture := range fixtures {
		row := []string{
			strconv.Itoa(fixture.Week),
			fixture.Date,
			fixture.HomeTeamFirst,
			fixture.AwayTeamFirst,
			fixture.HomeTeamSecond,
			fixture.AwayTeamSecond,
		}
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	return nil
}

func main() {
	outputDir := "pdf_output"

	fmt.Println("Starting PDF text extraction and parsing...")

	if err := ProcessDivisionPDFs(outputDir); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ All PDFs processed successfully!")
}
