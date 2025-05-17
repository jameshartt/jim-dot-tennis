package scraper

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
)

// ImportMockPlayers creates mock player data for each club
func (s *Scraper) ImportMockPlayers(ctx context.Context) error {
	// Initialize player repository if needed
	if s.playerRepo == nil {
		s.playerRepo = repository.NewPlayerRepository(s.db)
	}

	// Get all clubs
	clubs, err := s.clubRepo.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list clubs: %w", err)
	}

	// First names for players
	maleFirstNames := []string{
		"James", "David", "Michael", "John", "Robert", "William", "Richard", 
		"Thomas", "Christopher", "Daniel", "Matthew", "Anthony", "Mark", "Paul", 
		"Steven", "Andrew", "Kenneth", "George", "Edward", "Brian", "Ronald",
		"Oliver", "Harry", "Jack", "Charlie", "Thomas", "Jacob", "Alfie",
		"Freddie", "Oscar", "Noah", "Muhammad", "Max", "Callum", "Alexander",
	}

	femaleFirstNames := []string{
		"Mary", "Patricia", "Jennifer", "Linda", "Elizabeth", "Barbara", "Susan", 
		"Jessica", "Sarah", "Karen", "Lisa", "Nancy", "Betty", "Margaret", "Sandra", 
		"Ashley", "Emily", "Michelle", "Amanda", "Melissa", "Donna", "Deborah",
		"Olivia", "Sophie", "Emily", "Lily", "Amelia", "Jessica", "Ruby",
		"Chloe", "Grace", "Evie", "Isla", "Mia", "Charlotte", "Sophia",
	}

	lastNames := []string{
		"Smith", "Johnson", "Williams", "Jones", "Brown", "Davis", "Miller", "Wilson", 
		"Moore", "Taylor", "Anderson", "Thomas", "Jackson", "White", "Harris", "Martin", 
		"Thompson", "Garcia", "Martinez", "Robinson", "Clark", "Rodriguez", "Lewis", 
		"Lee", "Walker", "Hall", "Allen", "Young", "Hernandez", "King", "Wright", 
		"Lopez", "Hill", "Scott", "Green", "Adams", "Baker", "Gonzalez", "Nelson", 
		"Carter", "Mitchell", "Perez", "Roberts", "Turner", "Phillips", "Campbell", 
		"Parker", "Evans", "Edwards", "Collins", "Stewart", "Sanchez", "Morris",
		"Rogers", "Reed", "Cook", "Morgan", "Bell", "Murphy", "Bailey", "Rivera",
	}

	// Seed the random number generator
	rand.Seed(time.Now().UnixNano())

	// Create 10 to 20 players for each club
	for _, club := range clubs {
		// Determine number of players to create for this club (10-20)
		numPlayers := rand.Intn(11) + 10

		for i := 0; i < numPlayers; i++ {
			// Determine gender (roughly 50/50 split)
			isMale := rand.Intn(2) == 0

			var firstName string
			if isMale {
				firstName = maleFirstNames[rand.Intn(len(maleFirstNames))]
			} else {
				firstName = femaleFirstNames[rand.Intn(len(femaleFirstNames))]
			}

			lastName := lastNames[rand.Intn(len(lastNames))]
			email := fmt.Sprintf("%s.%s@example.com", firstName, lastName)
			phoneNumber := fmt.Sprintf("07%d", 700000000+rand.Intn(99999999))

			// Create the player
			player := &models.Player{
				ID:        uuid.New().String(),
				FirstName: firstName,
				LastName:  lastName,
				Email:     email,
				Phone:     phoneNumber,
				ClubID:    club.ID,
			}

			// Try to insert the player
			err := s.playerRepo.Create(ctx, player)
			if err != nil {
				log.Printf("Failed to create player %s %s: %v", firstName, lastName, err)
				continue
			}

			log.Printf("Created player: %s %s (ID: %s) for club %s", 
				player.FirstName, player.LastName, player.ID, club.Name)
		}
	}

	// Associate players with teams
	return s.associatePlayersWithTeams(ctx)
}

// associatePlayersWithTeams assigns players to teams
func (s *Scraper) associatePlayersWithTeams(ctx context.Context) error {
	// Get all teams
	teams, err := s.teamRepo.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list teams: %w", err)
	}

	// For each team, assign 6-12 players from their club
	for _, team := range teams {
		// Get all players for this club
		players, err := s.playerRepo.GetByClub(ctx, team.ClubID)
		if err != nil {
			return fmt.Errorf("failed to get players for club ID %d: %w", team.ClubID, err)
		}

		if len(players) == 0 {
			continue
		}

		// Shuffle the players
		rand.Shuffle(len(players), func(i, j int) {
			players[i], players[j] = players[j], players[i]
		})

		// Determine number of players to assign (6-12)
		numPlayers := rand.Intn(7) + 6
		if numPlayers > len(players) {
			numPlayers = len(players)
		}

		// Assign players to team
		for i := 0; i < numPlayers; i++ {
			err := s.teamRepo.AddPlayer(ctx, players[i].ID, team.ID, 1) // Season ID hardcoded to 1
			if err != nil {
				log.Printf("Failed to add player %s %s to team %s: %v", 
					players[i].FirstName, players[i].LastName, team.Name, err)
				continue
			}

			// 20% chance to make them a captain
			if i == 0 || (i < 2 && rand.Intn(5) == 0) {
				role := models.TeamCaptain
				if i > 0 {
					role = models.DayCaptain
				}
				
				err := s.teamRepo.AddCaptain(ctx, players[i].ID, team.ID, role, 1) // Season ID hardcoded to 1
				if err != nil {
					log.Printf("Failed to set player %s %s as captain for team %s: %v", 
						players[i].FirstName, players[i].LastName, team.Name, err)
				} else {
					log.Printf("Set player %s %s as %s captain for team %s", 
						players[i].FirstName, players[i].LastName, role, team.Name)
				}
			}

			log.Printf("Added player %s %s to team %s", 
				players[i].FirstName, players[i].LastName, team.Name)
		}
	}

	return nil
} 