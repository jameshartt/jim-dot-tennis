package players

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"

	"jim-dot-tennis/internal/models"
)

// StandingsHandler handles the public league standings page
type StandingsHandler struct {
	service     *Service
	templateDir string
}

// NewStandingsHandler creates a new standings handler
func NewStandingsHandler(service *Service, templateDir string) *StandingsHandler {
	return &StandingsHandler{
		service:     service,
		templateDir: templateDir,
	}
}

// TeamStanding represents a single team's standings row
type TeamStanding struct {
	TeamID        uint
	ClubID        uint
	TeamName      string
	ClubName      string
	IsStAnns      bool
	Played        int
	Won           int
	Lost          int
	Drawn         int
	RubberFor     int
	RubberAgainst int
	LeaguePoints  int
}

// DivisionStandings represents standings for a single division
type DivisionStandings struct {
	DivisionID   uint
	DivisionName string
	Level        int
	Teams        []TeamStanding
}

// StandingsPageData holds all data for the standings template
type StandingsPageData struct {
	Divisions       []DivisionStandings
	ActiveDivisionID uint
	Seasons         []models.Season
	ActiveSeason    *models.Season
	StAnnsClubID    uint
}

// HandleStandings handles GET /standings
func (h *StandingsHandler) HandleStandings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	// Determine season
	var season *models.Season
	seasonIDParam := r.URL.Query().Get("season")
	if seasonIDParam != "" {
		sid, err := strconv.ParseUint(seasonIDParam, 10, 32)
		if err == nil {
			s, err := h.service.seasonRepository.FindByID(ctx, uint(sid))
			if err == nil {
				season = s
			}
		}
	}

	if season == nil {
		s, err := h.service.seasonRepository.FindActive(ctx)
		if err != nil {
			log.Printf("No active season: %v", err)
			http.Error(w, "No active season found", http.StatusNotFound)
			return
		}
		season = s
	}

	// Get all seasons for the selector
	allSeasons, err := h.service.seasonRepository.FindAll(ctx)
	if err != nil {
		log.Printf("Failed to load seasons: %v", err)
		allSeasons = []models.Season{}
	}

	// Get divisions for this season
	divisions, err := h.service.divisionRepository.FindBySeason(ctx, season.ID)
	if err != nil {
		log.Printf("Failed to load divisions: %v", err)
		http.Error(w, "Failed to load divisions", http.StatusInternalServerError)
		return
	}

	// Find St Ann's club ID
	var stAnnsClubID uint
	stAnnsClubs, err := h.service.clubRepository.FindByNameLike(ctx, "St Ann")
	if err == nil && len(stAnnsClubs) > 0 {
		stAnnsClubID = stAnnsClubs[0].ID
	}

	// Calculate standings for each division
	var divisionStandings []DivisionStandings
	for _, div := range divisions {
		standings, err := h.calculateDivisionStandings(season.ID, div.ID, stAnnsClubID)
		if err != nil {
			log.Printf("Failed to calculate standings for division %d: %v", div.ID, err)
			continue
		}
		divisionStandings = append(divisionStandings, DivisionStandings{
			DivisionID:   div.ID,
			DivisionName: div.Name,
			Level:        div.Level,
			Teams:        standings,
		})
	}

	// Sort divisions by level
	sort.Slice(divisionStandings, func(i, j int) bool {
		return divisionStandings[i].Level < divisionStandings[j].Level
	})

	// Determine active division (from query or first)
	var activeDivisionID uint
	divIDParam := r.URL.Query().Get("division")
	if divIDParam != "" {
		did, err := strconv.ParseUint(divIDParam, 10, 32)
		if err == nil {
			activeDivisionID = uint(did)
		}
	}
	if activeDivisionID == 0 && len(divisionStandings) > 0 {
		activeDivisionID = divisionStandings[0].DivisionID
	}

	data := StandingsPageData{
		Divisions:        divisionStandings,
		ActiveDivisionID: activeDivisionID,
		Seasons:          allSeasons,
		ActiveSeason:     season,
		StAnnsClubID:     stAnnsClubID,
	}

	// Check for HTMX partial request
	if r.Header.Get("HX-Request") == "true" {
		tmpl, err := parseTemplate(h.templateDir, "players/standings_table.html")
		if err != nil {
			log.Printf("Error parsing standings table template: %v", err)
			http.Error(w, "Template error", http.StatusInternalServerError)
			return
		}
		if err := renderTemplate(w, tmpl, data); err != nil {
			log.Printf("Error rendering standings table: %v", err)
		}
		return
	}

	tmpl, err := parseTemplate(h.templateDir, "players/standings.html")
	if err != nil {
		log.Printf("Error parsing standings template: %v", err)
		renderFallbackHTML(w, "League Standings", "League Standings",
			"Standings page - template error", "/")
		return
	}

	if err := renderTemplate(w, tmpl, data); err != nil {
		log.Printf("Error rendering standings: %v", err)
	}
}

// calculateDivisionStandings computes team standings for a division
func (h *StandingsHandler) calculateDivisionStandings(seasonID, divisionID, stAnnsClubID uint) ([]TeamStanding, error) {
	ctx := fmt.Sprintf("season:%d,division:%d", seasonID, divisionID)
	_ = ctx // We use context.Background() for DB queries

	// Use a raw SQL query for efficiency
	// For each completed fixture in this division+season, sum up rubber points from matchups
	type fixtureResult struct {
		HomeTeamID       uint   `db:"home_team_id"`
		AwayTeamID       uint   `db:"away_team_id"`
		HomeTeamName     string `db:"home_team_name"`
		AwayTeamName     string `db:"away_team_name"`
		HomeClubID       uint   `db:"home_club_id"`
		AwayClubID       uint   `db:"away_club_id"`
		HomeClubName     string `db:"home_club_name"`
		AwayClubName     string `db:"away_club_name"`
		HomeRubberPoints int    `db:"home_rubber_points"`
		AwayRubberPoints int    `db:"away_rubber_points"`
	}

	var results []fixtureResult
	err := h.service.db.Select(&results, `
		SELECT
			f.home_team_id,
			f.away_team_id,
			ht.name AS home_team_name,
			at2.name AS away_team_name,
			ht.club_id AS home_club_id,
			at2.club_id AS away_club_id,
			hc.name AS home_club_name,
			ac.name AS away_club_name,
			COALESCE(SUM(m.home_score), 0) AS home_rubber_points,
			COALESCE(SUM(m.away_score), 0) AS away_rubber_points
		FROM fixtures f
		JOIN teams ht ON f.home_team_id = ht.id
		JOIN teams at2 ON f.away_team_id = at2.id
		JOIN clubs hc ON ht.club_id = hc.id
		JOIN clubs ac ON at2.club_id = ac.id
		LEFT JOIN matchups m ON m.fixture_id = f.id
		WHERE f.season_id = ? AND f.division_id = ? AND f.status = 'Completed'
		GROUP BY f.id, f.home_team_id, f.away_team_id, ht.name, at2.name, ht.club_id, at2.club_id, hc.name, ac.name
	`, seasonID, divisionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query fixture results: %w", err)
	}

	// Accumulate standings per team
	teamMap := make(map[uint]*TeamStanding)

	ensureTeam := func(teamID, clubID uint, teamName, clubName string) {
		if _, ok := teamMap[teamID]; !ok {
			teamMap[teamID] = &TeamStanding{
				TeamID:   teamID,
				ClubID:   clubID,
				TeamName: teamName,
				ClubName: clubName,
				IsStAnns: clubID == stAnnsClubID,
			}
		}
	}

	for _, r := range results {
		ensureTeam(r.HomeTeamID, r.HomeClubID, r.HomeTeamName, r.HomeClubName)
		ensureTeam(r.AwayTeamID, r.AwayClubID, r.AwayTeamName, r.AwayClubName)

		home := teamMap[r.HomeTeamID]
		away := teamMap[r.AwayTeamID]

		home.Played++
		away.Played++
		home.RubberFor += r.HomeRubberPoints
		home.RubberAgainst += r.AwayRubberPoints
		away.RubberFor += r.AwayRubberPoints
		away.RubberAgainst += r.HomeRubberPoints

		if r.HomeRubberPoints > r.AwayRubberPoints {
			home.Won++
			home.LeaguePoints += 3
			away.Lost++
		} else if r.AwayRubberPoints > r.HomeRubberPoints {
			away.Won++
			away.LeaguePoints += 3
			home.Lost++
		} else {
			home.Drawn++
			away.Drawn++
			home.LeaguePoints += 1
			away.LeaguePoints += 1
		}
	}

	// Also include teams with no completed fixtures
	bgCtx := context.Background()
	teams, err := h.service.teamRepository.FindByDivisionAndSeason(bgCtx, divisionID, seasonID)
	if err == nil {
		for _, t := range teams {
			if _, ok := teamMap[t.ID]; !ok {
				club, clubErr := h.service.clubRepository.FindByID(bgCtx, t.ClubID)
				clubName := ""
				if clubErr == nil {
					clubName = club.Name
				}
				teamMap[t.ID] = &TeamStanding{
					TeamID:   t.ID,
					ClubID:   t.ClubID,
					TeamName: t.Name,
					ClubName: clubName,
					IsStAnns: t.ClubID == stAnnsClubID,
				}
			}
		}
	}

	// Convert to slice and sort
	var standings []TeamStanding
	for _, ts := range teamMap {
		standings = append(standings, *ts)
	}

	sort.Slice(standings, func(i, j int) bool {
		if standings[i].LeaguePoints != standings[j].LeaguePoints {
			return standings[i].LeaguePoints > standings[j].LeaguePoints
		}
		// Rubber difference as tiebreaker
		diffI := standings[i].RubberFor - standings[i].RubberAgainst
		diffJ := standings[j].RubberFor - standings[j].RubberAgainst
		if diffI != diffJ {
			return diffI > diffJ
		}
		return standings[i].TeamName < standings[j].TeamName
	})

	return standings, nil
}
