// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package config

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
)

// Environment variables that override stored club settings. An empty value
// counts as unset (the compose files pass them through as "${VAR:-}").
const (
	EnvLogoPath       = "HOME_CLUB_LOGO_PATH"
	EnvBHPLTAClubCode = "BHPLTA_CLUB_CODE"
	EnvPublicURL      = "PUBLIC_URL"
)

// Defaults used when a colour has not been chosen.
const (
	DefaultPrimaryColour   = "#4cb04d"
	DefaultSecondaryColour = "#1455a1"
	// DefaultDisplayName is shown before a home club is chosen.
	DefaultDisplayName = "Tennis Club"
)

// LogoURLPrefix is the path uploaded logos are served under (see internal/branding).
const LogoURLPrefix = "/branding/"

// envOverrides maps a stored setting key to the environment variable that overrides it.
var envOverrides = map[string]string{
	repository.SettingClubLogoFile:   EnvLogoPath,
	repository.SettingBHPLTAClubCode: EnvBHPLTAClubCode,
	repository.SettingPublicURL:      EnvPublicURL,
}

// clubSettingKeys are the keys ClubSettings caches.
var clubSettingKeys = []string{
	repository.SettingClubDisplayName,
	repository.SettingClubLogoFile,
	repository.SettingClubLogoSize,
	repository.SettingClubPrimaryColour,
	repository.SettingClubSecondaryColour,
	repository.SettingBHPLTAClubCode,
	repository.SettingBHPLTAClubName,
	repository.SettingBHPLTAClubID,
	repository.SettingPublicURL,
}

// Branding is what pages, the manifest and the service worker show for the club.
type Branding struct {
	Name            string // display name
	LogoPath        string // URL path of the logo; empty means no logo
	LogoType        string // MIME type of an uploaded logo (empty for an env-configured path)
	LogoWidth       int    // pixel size of an uploaded logo (0 when unknown)
	LogoHeight      int
	PrimaryColour   string // "#rrggbb"
	SecondaryColour string // "#rrggbb"
}

// ClubSettingsProvider is implemented by *HomeClubHolder; handlers use it to
// reach the live club settings without changing their constructors.
type ClubSettingsProvider interface {
	ClubSettings() *ClubSettings
}

// ClubSettings is the live, concurrency-safe view of the club settings stored
// in app_settings, with environment overrides applied. Stored values are
// cached in memory and re-read by Reload (Save reloads), so reads never touch
// the database and a change takes effect on the next request.
type ClubSettings struct {
	repo repository.SettingsRepository // may be nil (env and defaults only)
	env  map[string]string             // non-empty env overrides, by setting key
	dir  string                        // branding directory for uploaded logos

	mu     sync.RWMutex
	stored map[string]string
}

// NewClubSettings reads the environment overrides and loads the stored values.
// A load error is logged and leaves the defaults in place. repo may be nil.
func NewClubSettings(ctx context.Context, repo repository.SettingsRepository) *ClubSettings {
	c := &ClubSettings{repo: repo, env: map[string]string{}, dir: BrandingDir(), stored: map[string]string{}}
	for key, envVar := range envOverrides {
		if v := strings.TrimSpace(os.Getenv(envVar)); v != "" {
			c.env[key] = v
		}
	}
	if err := c.Reload(ctx); err != nil {
		log.Printf("Warning: could not load club settings: %v", err)
	}
	if _, ok := c.env[repository.SettingBHPLTAClubCode]; ok {
		log.Printf("BHPLTA club code: set by %s", EnvBHPLTAClubCode)
	}
	if v, ok := c.env[repository.SettingClubLogoFile]; ok {
		log.Printf("Home club logo path (from %s): %s", EnvLogoPath, v)
	}
	return c
}

// DataDir is the directory holding the SQLite database (DB_PATH, default
// ./tennis.db). In Docker it is the data volume, so files kept here survive upgrades.
func DataDir() string {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./tennis.db"
	}
	return filepath.Dir(dbPath)
}

// BrandingDir is where uploaded logos are stored: a "branding" directory next to the database.
func BrandingDir() string { return filepath.Join(DataDir(), "branding") }

// Dir returns the branding directory used by this instance.
func (c *ClubSettings) Dir() string { return c.dir }

// Reload re-reads every stored club setting.
func (c *ClubSettings) Reload(ctx context.Context) error {
	if c == nil || c.repo == nil {
		return nil
	}
	stored := make(map[string]string, len(clubSettingKeys))
	for _, key := range clubSettingKeys {
		v, ok, err := c.repo.GetString(ctx, key)
		if err != nil {
			return err
		}
		if ok {
			stored[key] = v
		}
	}
	c.mu.Lock()
	c.stored = stored
	c.mu.Unlock()
	return nil
}

// Save stores the given values (an empty value deletes the key) and reloads.
// Values are stored as given: callers validate them first (see the Normalize
// helpers).
func (c *ClubSettings) Save(ctx context.Context, values map[string]string) error {
	if c == nil || c.repo == nil {
		return fmt.Errorf("club settings are not backed by a database")
	}
	for key, v := range values {
		var err error
		if v == "" {
			err = c.repo.Delete(ctx, key)
		} else {
			err = c.repo.SetString(ctx, key, v)
		}
		if err != nil {
			return err
		}
	}
	return c.Reload(ctx)
}

// Stored returns the stored (not effective) value for key.
func (c *ClubSettings) Stored(key string) string {
	if c == nil {
		return ""
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stored[key]
}

// EnvOverride reports the environment variable overriding key, if it is set.
func (c *ClubSettings) EnvOverride(key string) (envVar string, ok bool) {
	if c == nil {
		return "", false
	}
	if _, set := c.env[key]; !set {
		return "", false
	}
	return envOverrides[key], true
}

func (c *ClubSettings) envValue(key string) (string, bool) {
	if c == nil {
		return "", false
	}
	v, ok := c.env[key]
	return v, ok
}

// DisplayName is the stored display name, else the home club's name, else DefaultDisplayName.
func (c *ClubSettings) DisplayName(club *models.Club) string {
	if v := c.Stored(repository.SettingClubDisplayName); v != "" {
		return v
	}
	if club != nil && club.Name != "" {
		return club.Name
	}
	return DefaultDisplayName
}

// LogoPath is HOME_CLUB_LOGO_PATH if set, else the uploaded logo's URL, else "" (no logo).
func (c *ClubSettings) LogoPath() string {
	if v, ok := c.envValue(repository.SettingClubLogoFile); ok {
		return v
	}
	if f := c.Stored(repository.SettingClubLogoFile); f != "" {
		return LogoURLPrefix + f
	}
	return ""
}

// PrimaryColour is the stored primary colour or DefaultPrimaryColour.
func (c *ClubSettings) PrimaryColour() string {
	if v := c.Stored(repository.SettingClubPrimaryColour); v != "" {
		return v
	}
	return DefaultPrimaryColour
}

// SecondaryColour is the stored secondary colour or DefaultSecondaryColour.
func (c *ClubSettings) SecondaryColour() string {
	if v := c.Stored(repository.SettingClubSecondaryColour); v != "" {
		return v
	}
	return DefaultSecondaryColour
}

// Branding returns the effective branding for the given home club (may be nil).
func (c *ClubSettings) Branding(club *models.Club) Branding {
	b := Branding{
		Name:            c.DisplayName(club),
		LogoPath:        c.LogoPath(),
		PrimaryColour:   c.PrimaryColour(),
		SecondaryColour: c.SecondaryColour(),
	}
	if _, env := c.envValue(repository.SettingClubLogoFile); !env {
		if f := c.Stored(repository.SettingClubLogoFile); f != "" {
			b.LogoType = LogoMIMEType(f)
			b.LogoWidth, b.LogoHeight = parseSize(c.Stored(repository.SettingClubLogoSize))
		}
	}
	return b
}

// BHPLTAClubCode is BHPLTA_CLUB_CODE if set, else the stored code.
func (c *ClubSettings) BHPLTAClubCode() string {
	if v, ok := c.envValue(repository.SettingBHPLTAClubCode); ok {
		return v
	}
	return c.Stored(repository.SettingBHPLTAClubCode)
}

// BHPLTAClubName is the stored BHPLTA club name, else the known value for the
// home club (see KnownBHPLTAClub), else "".
func (c *ClubSettings) BHPLTAClubName(club *models.Club) string {
	if v := c.Stored(repository.SettingBHPLTAClubName); v != "" {
		return v
	}
	if k, ok := KnownBHPLTAClub(club); ok {
		return k.Name
	}
	return ""
}

// BHPLTAClubID is the stored BHPLTA club id, else the known value for the home
// club, else 0.
func (c *ClubSettings) BHPLTAClubID(club *models.Club) int {
	if v := c.Stored(repository.SettingBHPLTAClubID); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	if k, ok := KnownBHPLTAClub(club); ok {
		return k.ID
	}
	return 0
}

// PublicURL is PUBLIC_URL if set, else the stored public URL, else "".
// Returned without a trailing slash.
func (c *ClubSettings) PublicURL() string {
	if v, ok := c.envValue(repository.SettingPublicURL); ok {
		return strings.TrimRight(v, "/")
	}
	return c.Stored(repository.SettingPublicURL)
}

// LogoMIMEType returns the MIME type for an uploaded logo file name by extension.
func LogoMIMEType(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	}
	return ""
}

func parseSize(s string) (int, int) {
	w, h, ok := strings.Cut(s, "x")
	if !ok {
		return 0, 0
	}
	wi, err1 := strconv.Atoi(w)
	hi, err2 := strconv.Atoi(h)
	if err1 != nil || err2 != nil || wi < 0 || hi < 0 {
		return 0, 0
	}
	return wi, hi
}

var colourRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// NormalizeColour validates a "#rrggbb" colour and returns it in lower case.
func NormalizeColour(s string) (string, error) {
	s = strings.TrimSpace(s)
	if !colourRe.MatchString(s) {
		return "", fmt.Errorf("colour must look like #1a2b3c")
	}
	return strings.ToLower(s), nil
}

// MaxDisplayNameLength bounds the club display name.
const MaxDisplayNameLength = 80

// NormalizeDisplayName trims a display name and checks its length. Empty is allowed (use the club's name).
func NormalizeDisplayName(s string) (string, error) {
	s = strings.Join(strings.Fields(s), " ")
	if len([]rune(s)) > MaxDisplayNameLength {
		return "", fmt.Errorf("display name must be at most %d characters", MaxDisplayNameLength)
	}
	return s, nil
}

// NormalizePublicURL validates the public URL: http or https, a host, no
// credentials, query or fragment. An optional path is kept (for instances
// served under a sub-path); the trailing slash is removed. Empty is allowed.
func NormalizePublicURL(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("public URL is not a valid URL")
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", fmt.Errorf("public URL must start with https:// (or http:// for local testing)")
	}
	if u.Host == "" || u.Hostname() == "" {
		return "", fmt.Errorf("public URL must include a host name")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return "", fmt.Errorf("public URL must not contain credentials, a query or a fragment")
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	return strings.TrimRight(u.String(), "/"), nil
}

// NormalizeBHPLTAClubID validates BHPLTA's numeric club id. Empty is allowed.
func NormalizeBHPLTAClubID(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 || n > 1_000_000 {
		return "", fmt.Errorf("BHPLTA club id must be a positive whole number")
	}
	return strconv.Itoa(n), nil
}

// MaxBHPLTAFieldLength bounds the BHPLTA club code and name.
const MaxBHPLTAFieldLength = 100

// NormalizeBHPLTAText trims a BHPLTA club code or name and checks its length.
func NormalizeBHPLTAText(label, s string) (string, error) {
	s = strings.TrimSpace(s)
	if len(s) > MaxBHPLTAFieldLength || strings.ContainsAny(s, "\r\n;") {
		return "", fmt.Errorf("%s must be at most %d characters on one line, without semicolons", label, MaxBHPLTAFieldLength)
	}
	return s, nil
}
