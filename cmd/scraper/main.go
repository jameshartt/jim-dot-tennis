package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/ledongthuc/pdf"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <pdf-url-or-path>")
		fmt.Println("Example: go run main.go https://www.bhplta.co.uk/wp-content/uploads/2025/03/Fixture-Card-2025.pdf")
		os.Exit(1)
	}

	input := os.Args[1]
	var pdfPath string

	// Check if input is a URL or local file
	if isURL(input) {
		fmt.Printf("Downloading PDF from: %s\n", input)
		downloadedPath, err := downloadPDF(input)
		if err != nil {
			log.Fatalf("Failed to download PDF: %v", err)
		}
		pdfPath = downloadedPath
		defer os.Remove(downloadedPath) // Clean up downloaded file
	} else {
		pdfPath = input
	}

	fmt.Printf("Extracting text from: %s\n", pdfPath)
	text, err := extractPDFText(pdfPath)
	if err != nil {
		log.Fatalf("Failed to extract PDF text: %v", err)
	}

	fmt.Println("\n" + "="*80)
	fmt.Println("EXTRACTED TEXT:")
	fmt.Println("=" * 80)
	fmt.Println(text)
	fmt.Println("=" * 80)
	fmt.Printf("Total characters: %d\n", len(text))

	// Save to file for easier analysis
	outputFile := "fixture_card_text.txt"
	err = os.WriteFile(outputFile, []byte(text), 0644)
	if err != nil {
		log.Printf("Warning: Failed to save text to %s: %v", outputFile, err)
	} else {
		fmt.Printf("Text saved to: %s\n", outputFile)
	}
}

func isURL(str string) bool {
	return len(str) > 7 && (str[:7] == "http://" || str[:8] == "https://")
}

func downloadPDF(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "fixture_card_*.pdf")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	// Copy response body to file
	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}

	return tmpFile.Name(), nil
}

func extractPDFText(pdfPath string) (string, error) {
	file, reader, err := pdf.Open(pdfPath)
	if err != nil {
		return "", fmt.Errorf("failed to open PDF: %w", err)
	}
	defer file.Close()

	var text string
	totalPages := reader.NumPage()

	fmt.Printf("PDF has %d pages\n", totalPages)

	for pageNum := 1; pageNum <= totalPages; pageNum++ {
		page := reader.Page(pageNum)
		if page.V.IsNull() {
			continue
		}

		pageText, err := page.GetPlainText()
		if err != nil {
			fmt.Printf("Warning: Failed to extract text from page %d: %v\n", pageNum, err)
			continue
		}

		text += fmt.Sprintf("\n--- PAGE %d ---\n", pageNum)
		text += pageText
		text += "\n"
	}

	return text, nil
}
