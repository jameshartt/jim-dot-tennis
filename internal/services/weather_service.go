package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// WeatherData represents weather information for a specific date
type WeatherData struct {
	Date                 string  `json:"date"`
	TemperatureMax       float64 `json:"temperature_max"`
	TemperatureMin       float64 `json:"temperature_min"`
	PrecipitationPercent int     `json:"precipitation_percent"`
	WeatherCode          int     `json:"weather_code"`
	Description          string  `json:"description"`
	Icon                 string  `json:"icon"`
}

// WeatherService provides weather forecasts via the Open-Meteo API
type WeatherService struct {
	cache  map[string]*weatherCacheEntry
	mu     sync.RWMutex
	client *http.Client
}

type weatherCacheEntry struct {
	data      []WeatherData
	fetchedAt time.Time
}

const weatherCacheTTL = 1 * time.Hour

// NewWeatherService creates a new weather service
func NewWeatherService() *WeatherService {
	return &WeatherService{
		cache: make(map[string]*weatherCacheEntry),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetForecast retrieves a 7-day forecast for the given coordinates
func (ws *WeatherService) GetForecast(lat, lng float64) ([]WeatherData, error) {
	key := fmt.Sprintf("%.2f,%.2f", lat, lng)

	// Check cache
	ws.mu.RLock()
	entry, ok := ws.cache[key]
	ws.mu.RUnlock()

	if ok && time.Since(entry.fetchedAt) < weatherCacheTTL {
		return entry.data, nil
	}

	// Fetch from API
	url := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f&daily=temperature_2m_max,temperature_2m_min,precipitation_probability_max,weathercode&timezone=Europe/London&forecast_days=7",
		lat, lng,
	)

	resp, err := ws.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch weather: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weather API returned status %d", resp.StatusCode)
	}

	var apiResp openMeteoResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode weather response: %w", err)
	}

	// Convert to our format
	var forecasts []WeatherData
	for i, date := range apiResp.Daily.Time {
		desc, icon := weatherCodeToInfo(apiResp.Daily.WeatherCode[i])
		forecasts = append(forecasts, WeatherData{
			Date:                 date,
			TemperatureMax:       apiResp.Daily.TemperatureMax[i],
			TemperatureMin:       apiResp.Daily.TemperatureMin[i],
			PrecipitationPercent: apiResp.Daily.PrecipitationProbMax[i],
			WeatherCode:          apiResp.Daily.WeatherCode[i],
			Description:          desc,
			Icon:                 icon,
		})
	}

	// Update cache
	ws.mu.Lock()
	ws.cache[key] = &weatherCacheEntry{
		data:      forecasts,
		fetchedAt: time.Now(),
	}
	ws.mu.Unlock()

	return forecasts, nil
}

// GetForecastForDate retrieves weather for a specific date at the given coordinates.
// Returns nil if the date is not within the 7-day forecast window.
func (ws *WeatherService) GetForecastForDate(lat, lng float64, date time.Time) (*WeatherData, error) {
	forecasts, err := ws.GetForecast(lat, lng)
	if err != nil {
		return nil, err
	}

	dateStr := date.Format("2006-01-02")
	for _, f := range forecasts {
		if f.Date == dateStr {
			return &f, nil
		}
	}

	return nil, nil // Date not in forecast window
}

// Open-Meteo API response structure
type openMeteoResponse struct {
	Daily openMeteoDailyData `json:"daily"`
}

type openMeteoDailyData struct {
	Time                []string  `json:"time"`
	TemperatureMax      []float64 `json:"temperature_2m_max"`
	TemperatureMin      []float64 `json:"temperature_2m_min"`
	PrecipitationProbMax []int    `json:"precipitation_probability_max"`
	WeatherCode         []int     `json:"weathercode"`
}

// weatherCodeToInfo maps WMO weather codes to descriptions and emoji icons
func weatherCodeToInfo(code int) (string, string) {
	switch {
	case code == 0:
		return "Clear sky", "☀️"
	case code == 1:
		return "Mainly clear", "🌤️"
	case code == 2:
		return "Partly cloudy", "⛅"
	case code == 3:
		return "Overcast", "☁️"
	case code >= 45 && code <= 48:
		return "Foggy", "🌫️"
	case code >= 51 && code <= 55:
		return "Drizzle", "🌦️"
	case code >= 56 && code <= 57:
		return "Freezing drizzle", "🌧️"
	case code >= 61 && code <= 65:
		return "Rain", "🌧️"
	case code >= 66 && code <= 67:
		return "Freezing rain", "🌧️"
	case code >= 71 && code <= 77:
		return "Snow", "❄️"
	case code >= 80 && code <= 82:
		return "Showers", "🌦️"
	case code >= 85 && code <= 86:
		return "Snow showers", "🌨️"
	case code == 95:
		return "Thunderstorm", "⛈️"
	case code >= 96 && code <= 99:
		return "Thunderstorm with hail", "⛈️"
	default:
		return "Unknown", "🌡️"
	}
}
