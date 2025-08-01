package main

import (
	"flag"
	"fmt"
	"log"

	"jim-dot-tennis/internal/services"
)

func main() {
	// Define command line flags
	var (
		clubCode = flag.String("club-code", "", "Club code (optional)")
		verbose  = flag.Bool("verbose", false, "Verbose output")
	)
	flag.Parse()

	// Create nonce extractor
	extractor := services.NewNonceExtractor()

	var result *services.NonceResult
	var err error

	fmt.Println("Extracting nonce from BHPLTA website...")

	if *clubCode != "" {
		if *verbose {
			fmt.Printf("Using club code: %s\n", *clubCode)
		}
		result, err = extractor.ExtractNonceWithClubCode(*clubCode)
	} else {
		if *verbose {
			fmt.Println("Extracting nonce without club code...")
		}
		result, err = extractor.ExtractNonce()
	}

	if err != nil {
		log.Fatalf("Failed to extract nonce: %v", err)
	}

	fmt.Printf("Successfully extracted nonce!\n")
	fmt.Printf("Nonce: %s\n", result.Nonce)
	fmt.Printf("Expires at: %s\n", result.ExpiresAt.Format("2006-01-02 15:04:05"))

	if result.ClubCode != "" {
		fmt.Printf("Club code: %s\n", result.ClubCode)
	}

	if *verbose {
		fmt.Printf("Full nonce length: %d characters\n", len(result.Nonce))
	}
}
