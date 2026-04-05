// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"jim-dot-tennis/internal/models"
)

// CourtHive calendar API response types

type courthiveCalendarResponse struct {
	Success  bool                   `json:"success"`
	Calendar courthiveCalendarData  `json:"calendar"`
}

type courthiveCalendarData struct {
	Tournaments []courthiveTournamentEntry `json:"tournaments"`
}

type courthiveTournamentEntry struct {
	TournamentID string                    `json:"tournamentId"`
	Tournament   courthiveTournamentDetail `json:"tournament"`
}

type courthiveTournamentDetail struct {
	TournamentName string `json:"tournamentName"`
	StartDate      string `json:"startDate"`
	EndDate        string `json:"endDate"`
}

// SyncResult summarises what happened during a sync operation
type SyncResult struct {
	New       int
	Updated   int
	Unchanged int
}

// --- Tournament Provider methods ---

func (s *Service) GetAllTournamentProviders() ([]models.TournamentProvider, error) {
	ctx := context.Background()
	return s.tournamentProviderRepository.FindAllWithCounts(ctx)
}

func (s *Service) GetTournamentProviderByID(id uint) (*models.TournamentProvider, error) {
	ctx := context.Background()
	return s.tournamentProviderRepository.FindByID(ctx, id)
}

func (s *Service) CreateTournamentProvider(provider *models.TournamentProvider) error {
	ctx := context.Background()
	return s.tournamentProviderRepository.Create(ctx, provider)
}

func (s *Service) UpdateTournamentProvider(provider *models.TournamentProvider) error {
	ctx := context.Background()
	return s.tournamentProviderRepository.Update(ctx, provider)
}

func (s *Service) DeleteTournamentProvider(id uint) error {
	ctx := context.Background()
	count, err := s.tournamentProviderRepository.CountTournaments(ctx, id)
	if err != nil {
		return fmt.Errorf("checking tournament count: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("cannot delete provider with %d tournaments — remove them first", count)
	}
	return s.tournamentProviderRepository.Delete(ctx, id)
}

// --- Tournament methods ---

func (s *Service) GetAllTournaments() ([]models.Tournament, error) {
	ctx := context.Background()
	return s.tournamentRepository.FindAll(ctx)
}

func (s *Service) GetTournamentsByProvider(providerID uint) ([]models.Tournament, error) {
	ctx := context.Background()
	return s.tournamentRepository.FindByProviderID(ctx, providerID)
}

func (s *Service) GetVisibleTournaments() ([]models.Tournament, error) {
	ctx := context.Background()
	return s.tournamentRepository.FindVisible(ctx)
}

func (s *Service) GetTournamentByID(id uint) (*models.Tournament, error) {
	ctx := context.Background()
	return s.tournamentRepository.FindByID(ctx, id)
}

func (s *Service) CreateTournament(tournament *models.Tournament) error {
	ctx := context.Background()
	return s.tournamentRepository.Create(ctx, tournament)
}

func (s *Service) UpdateTournament(tournament *models.Tournament) error {
	ctx := context.Background()
	return s.tournamentRepository.Update(ctx, tournament)
}

func (s *Service) DeleteTournament(id uint) error {
	ctx := context.Background()
	return s.tournamentRepository.Delete(ctx, id)
}

func (s *Service) ToggleTournamentVisibility(id uint) (*models.Tournament, error) {
	ctx := context.Background()
	tournament, err := s.tournamentRepository.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	tournament.IsVisible = !tournament.IsVisible
	if err := s.tournamentRepository.Update(ctx, tournament); err != nil {
		return nil, err
	}
	return tournament, nil
}

// --- CourtHive sync ---

func (s *Service) SyncFromCourtHive(providerID uint) (*SyncResult, error) {
	ctx := context.Background()

	provider, err := s.tournamentProviderRepository.FindByID(ctx, providerID)
	if err != nil {
		return nil, fmt.Errorf("finding provider: %w", err)
	}

	entries, err := s.fetchCourtHiveCalendar(provider.ProviderAbbr)
	if err != nil {
		return nil, fmt.Errorf("fetching CourtHive calendar for %q: %w", provider.ProviderAbbr, err)
	}

	result := &SyncResult{}

	for _, entry := range entries {
		existing, err := s.tournamentRepository.FindByCourthiveTournamentID(ctx, entry.TournamentID)
		if err != nil {
			// Not found — create new
			tournament := &models.Tournament{
				Name:                  entry.Tournament.TournamentName,
				CourthiveTournamentID: entry.TournamentID,
				ProviderID:            provider.ID,
				StartDate:             entry.Tournament.StartDate,
				EndDate:               entry.Tournament.EndDate,
				IsVisible:             false,
				DisplayOrder:          0,
			}
			if createErr := s.tournamentRepository.Create(ctx, tournament); createErr != nil {
				return nil, fmt.Errorf("creating tournament %q: %w", entry.Tournament.TournamentName, createErr)
			}
			result.New++
			continue
		}

		// Existing — update name and dates if changed
		changed := false
		if existing.Name != entry.Tournament.TournamentName {
			existing.Name = entry.Tournament.TournamentName
			changed = true
		}
		if existing.StartDate != entry.Tournament.StartDate {
			existing.StartDate = entry.Tournament.StartDate
			changed = true
		}
		if existing.EndDate != entry.Tournament.EndDate {
			existing.EndDate = entry.Tournament.EndDate
			changed = true
		}

		if changed {
			if updateErr := s.tournamentRepository.Update(ctx, existing); updateErr != nil {
				return nil, fmt.Errorf("updating tournament %q: %w", existing.Name, updateErr)
			}
			result.Updated++
		} else {
			result.Unchanged++
		}
	}

	return result, nil
}

func (s *Service) fetchCourtHiveCalendar(providerAbbr string) ([]courthiveTournamentEntry, error) {
	reqBody, err := json.Marshal(map[string]string{"providerAbbr": providerAbbr})
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	url := s.courthiveAPIURL + "/provider/calendar"

	resp, err := client.Post(url, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("calling %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("CourtHive returned %d: %s", resp.StatusCode, string(body))
	}

	var calResp courthiveCalendarResponse
	if err := json.NewDecoder(resp.Body).Decode(&calResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if !calResp.Success {
		return nil, fmt.Errorf("CourtHive returned success=false")
	}

	return calResp.Calendar.Tournaments, nil
}
