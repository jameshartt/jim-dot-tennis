// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package config

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
)

// clearClubSettingsEnv unsets every override and points DB_PATH at a temp dir.
func clearClubSettingsEnv(t *testing.T) {
	t.Helper()
	t.Setenv(EnvLogoPath, "")
	t.Setenv(EnvBHPLTAClubCode, "")
	t.Setenv(EnvPublicURL, "")
	t.Setenv("DB_PATH", filepath.Join(t.TempDir(), "tennis.db"))
}

func TestClubSettingsDefaults(t *testing.T) {
	clearClubSettingsEnv(t)
	e := newTestEnv(t)
	cs := NewClubSettings(context.Background(), e.settings)

	club := &models.Club{ID: 1, Name: "Alpha Tennis Club"}
	if got := cs.DisplayName(club); got != "Alpha Tennis Club" {
		t.Errorf("DisplayName = %q, want the club's name", got)
	}
	if got := cs.DisplayName(nil); got != DefaultDisplayName {
		t.Errorf("DisplayName(nil) = %q", got)
	}
	if got := cs.LogoPath(nil); got != "" {
		t.Errorf("LogoPath = %q, want no logo by default", got)
	}
	if cs.PrimaryColour() != DefaultPrimaryColour || cs.SecondaryColour() != DefaultSecondaryColour {
		t.Errorf("colours = %q %q", cs.PrimaryColour(), cs.SecondaryColour())
	}
	if cs.BHPLTAClubCode() != "" || cs.BHPLTAClubName(club) != "" || cs.BHPLTAClubID(club) != 0 || cs.PublicURL() != "" {
		t.Errorf("unexpected BHPLTA/public URL defaults")
	}
	if want := filepath.Join(filepath.Dir(os.Getenv("DB_PATH")), "branding"); cs.Dir() != want {
		t.Errorf("Dir = %q, want %q", cs.Dir(), want)
	}

	// A nil *ClubSettings (no settings wired) behaves like defaults.
	var none *ClubSettings
	if b := none.Branding(club); b.Name != "Alpha Tennis Club" || b.PrimaryColour != DefaultPrimaryColour || b.LogoPath != "" {
		t.Errorf("nil Branding = %+v", b)
	}
}

func TestClubSettingsStoredValuesTakeEffectOnSave(t *testing.T) {
	ctx := context.Background()
	clearClubSettingsEnv(t)
	e := newTestEnv(t)
	cs := NewClubSettings(ctx, e.settings)
	club := &models.Club{ID: 1, Name: "Alpha Tennis Club"}

	if err := cs.Save(ctx, map[string]string{
		repository.SettingClubDisplayName:     "Alpha TC",
		repository.SettingClubLogoFile:        "logo-0123456789abcdef.png",
		repository.SettingClubLogoSize:        "256x256",
		repository.SettingClubPrimaryColour:   "#112233",
		repository.SettingClubSecondaryColour: "#445566",
		repository.SettingBHPLTAClubCode:      "ALPHA01",
		repository.SettingBHPLTAClubName:      "Alpha",
		repository.SettingBHPLTAClubID:        "42",
		repository.SettingPublicURL:           "https://tennis.example.org",
	}); err != nil {
		t.Fatal(err)
	}
	b := cs.Branding(club)
	want := Branding{Name: "Alpha TC", LogoPath: "/branding/logo-0123456789abcdef.png", LogoType: "image/png",
		LogoWidth: 256, LogoHeight: 256, PrimaryColour: "#112233", SecondaryColour: "#445566"}
	if b != want {
		t.Errorf("Branding = %+v, want %+v", b, want)
	}
	if cs.BHPLTAClubCode() != "ALPHA01" || cs.BHPLTAClubName(club) != "Alpha" || cs.BHPLTAClubID(club) != 42 {
		t.Errorf("BHPLTA = %q %q %d", cs.BHPLTAClubCode(), cs.BHPLTAClubName(club), cs.BHPLTAClubID(club))
	}
	if cs.PublicURL() != "https://tennis.example.org" {
		t.Errorf("PublicURL = %q", cs.PublicURL())
	}

	// An empty value deletes the key: back to the default.
	if err := cs.Save(ctx, map[string]string{repository.SettingClubDisplayName: "", repository.SettingClubPrimaryColour: ""}); err != nil {
		t.Fatal(err)
	}
	if cs.DisplayName(club) != "Alpha Tennis Club" || cs.PrimaryColour() != DefaultPrimaryColour {
		t.Errorf("after delete: %q %q", cs.DisplayName(club), cs.PrimaryColour())
	}
	if _, ok, _ := e.settings.GetString(ctx, repository.SettingClubDisplayName); ok {
		t.Errorf("display name still stored after saving it empty")
	}
}

func TestEnvironmentOverridesStoredClubSettings(t *testing.T) {
	ctx := context.Background()
	clearClubSettingsEnv(t)
	e := newTestEnv(t)
	for k, v := range map[string]string{
		repository.SettingClubLogoFile:   "logo-0123456789abcdef.png",
		repository.SettingBHPLTAClubCode: "STORED",
		repository.SettingPublicURL:      "https://stored.example.org",
	} {
		if err := e.settings.SetString(ctx, k, v); err != nil {
			t.Fatal(err)
		}
	}

	t.Setenv(EnvLogoPath, "/static/my-club.jpg")
	t.Setenv(EnvBHPLTAClubCode, "FROMENV")
	t.Setenv(EnvPublicURL, "https://env.example.org/")
	cs := NewClubSettings(ctx, e.settings)

	if cs.LogoPath(nil) != "/static/my-club.jpg" {
		t.Errorf("LogoPath = %q, want the env value", cs.LogoPath(nil))
	}
	if b := cs.Branding(nil); b.LogoType != "" || b.LogoPath != "/static/my-club.jpg" {
		t.Errorf("Branding with env logo = %+v", b)
	}
	if cs.BHPLTAClubCode() != "FROMENV" {
		t.Errorf("BHPLTAClubCode = %q, want the env value", cs.BHPLTAClubCode())
	}
	if cs.PublicURL() != "https://env.example.org" {
		t.Errorf("PublicURL = %q, want the env value without the trailing slash", cs.PublicURL())
	}
	for key, envVar := range map[string]string{
		repository.SettingClubLogoFile:   EnvLogoPath,
		repository.SettingBHPLTAClubCode: EnvBHPLTAClubCode,
		repository.SettingPublicURL:      EnvPublicURL,
	} {
		if got, ok := cs.EnvOverride(key); !ok || got != envVar {
			t.Errorf("EnvOverride(%s) = %q, %v", key, got, ok)
		}
	}
	// Stored values are still there (shown on the page, used once the env var goes).
	if cs.Stored(repository.SettingBHPLTAClubCode) != "STORED" {
		t.Errorf("stored code = %q", cs.Stored(repository.SettingBHPLTAClubCode))
	}
	if _, ok := cs.EnvOverride(repository.SettingClubDisplayName); ok {
		t.Errorf("display name has no env override")
	}

	// An empty env value counts as unset (compose passes "${VAR:-}").
	t.Setenv(EnvBHPLTAClubCode, "")
	if got := NewClubSettings(ctx, e.settings).BHPLTAClubCode(); got != "STORED" {
		t.Errorf("empty env: BHPLTAClubCode = %q, want the stored value", got)
	}
}

func TestHomeClubHolderUsesClubSettings(t *testing.T) {
	ctx := context.Background()
	clearClubSettingsEnv(t)
	e := newTestEnv(t)
	e.addClub(t, 1, "Alpha Tennis Club")
	t.Setenv("HOME_CLUB_ID", "1")
	h, err := NewHomeClubHolder(ctx, e.clubs, e.settings)
	if err != nil {
		t.Fatal(err)
	}
	if h.BHPLTAClubCode() != "" || h.LogoPath() != "" {
		t.Fatalf("unexpected initial values")
	}
	// Written behind the cache's back (as setup does in its own transaction):
	// Refresh picks it up.
	if err := e.settings.SetString(ctx, repository.SettingBHPLTAClubCode, "ALPHA01"); err != nil {
		t.Fatal(err)
	}
	if err := h.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	if h.BHPLTAClubCode() != "ALPHA01" {
		t.Errorf("after Refresh: BHPLTAClubCode = %q", h.BHPLTAClubCode())
	}
	if h.Branding().Name != "Alpha Tennis Club" {
		t.Errorf("Branding().Name = %q", h.Branding().Name)
	}
}

func TestKnownBHPLTAClubAndPrefill(t *testing.T) {
	ctx := context.Background()
	clearClubSettingsEnv(t)
	e := newTestEnv(t)

	for _, name := range []string{"St Ann's Tennis", "St Ann’s Tennis Club", "st anns"} {
		if ids, ok := KnownBHPLTAClub(&models.Club{Name: name}); !ok || ids.Name != "St+Anns" || ids.ID != 10 {
			t.Errorf("KnownBHPLTAClub(%q) = %+v, %v", name, ids, ok)
		}
	}
	if _, ok := KnownBHPLTAClub(&models.Club{Name: "St Ann's Wells"}); ok {
		t.Errorf("partial name must not match")
	}

	// Unknown club: nothing derivable.
	stored, err := PrefillClubSettings(ctx, e.settings, &models.Club{Name: "Alpha Tennis Club"})
	if err != nil || len(stored) != 0 {
		t.Fatalf("unknown club: stored %v, err %v", stored, err)
	}

	// Known club, id already set by hand: only the name is filled in.
	if err := e.settings.SetString(ctx, repository.SettingBHPLTAClubID, "77"); err != nil {
		t.Fatal(err)
	}
	stored, err = PrefillClubSettings(ctx, e.settings, &models.Club{Name: "St Ann's Tennis"})
	if err != nil || !slices.Equal(stored, []string{repository.SettingBHPLTAClubName}) {
		t.Fatalf("known club: stored %v, err %v", stored, err)
	}
	cs := NewClubSettings(ctx, e.settings)
	if cs.BHPLTAClubName(nil) != "St+Anns" || cs.BHPLTAClubID(nil) != 77 {
		t.Errorf("after prefill: %q %d", cs.BHPLTAClubName(nil), cs.BHPLTAClubID(nil))
	}

	// With nothing stored, the effective values fall back to the known list
	// (an existing deployment keeps working after upgrade).
	fresh := newTestEnv(t)
	cs = NewClubSettings(ctx, fresh.settings)
	club := &models.Club{Name: "St Ann's Tennis"}
	if cs.BHPLTAClubName(club) != "St+Anns" || cs.BHPLTAClubID(club) != 10 {
		t.Errorf("fallback: %q %d", cs.BHPLTAClubName(club), cs.BHPLTAClubID(club))
	}
}

// The known club keeps its bundled logo when nothing else is set; no other
// club gets a default, and an upload or HOME_CLUB_LOGO_PATH wins.
func TestKnownClubLogo(t *testing.T) {
	ctx := context.Background()
	clearClubSettingsEnv(t)
	e := newTestEnv(t)
	cs := NewClubSettings(ctx, e.settings)
	known := &models.Club{Name: "St Ann’s Tennis Club"}
	const bundled = "/static/st-anns-tennis.jpg"

	if got := cs.LogoPath(known); got != bundled {
		t.Errorf("known club LogoPath = %q, want %q", got, bundled)
	}
	if b := cs.Branding(known); b.LogoPath != bundled || b.LogoType != "" {
		t.Errorf("known club Branding = %+v", b)
	}
	for _, other := range []*models.Club{nil, {Name: "Alpha Tennis Club"}, {Name: "St Ann's Wells"}} {
		if got := cs.LogoPath(other); got != "" {
			t.Errorf("LogoPath(%v) = %q, want no logo", other, got)
		}
	}
	if _, err := os.Stat(filepath.Join(findStaticDir(t), "st-anns-tennis.jpg")); err != nil {
		t.Errorf("bundled logo missing: %v", err)
	}

	if err := e.settings.SetString(ctx, repository.SettingClubLogoFile, "logo-0123456789abcdef.png"); err != nil {
		t.Fatal(err)
	}
	cs = NewClubSettings(ctx, e.settings)
	if got := cs.LogoPath(known); got != LogoURLPrefix+"logo-0123456789abcdef.png" {
		t.Errorf("uploaded logo does not win: %q", got)
	}
	t.Setenv(EnvLogoPath, "/static/my-club.jpg")
	cs = NewClubSettings(ctx, e.settings)
	if got := cs.LogoPath(known); got != "/static/my-club.jpg" {
		t.Errorf("env logo does not win: %q", got)
	}
}

// findStaticDir walks up from the package directory to the repository's static/.
func findStaticDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		if info, err := os.Stat(filepath.Join(dir, "static")); err == nil && info.IsDir() {
			return filepath.Join(dir, "static")
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("could not locate static directory")
	return ""
}

func TestNormalizeHelpers(t *testing.T) {
	for in, want := range map[string]string{"#AABBCC": "#aabbcc", " #4cb04d ": "#4cb04d"} {
		if got, err := NormalizeColour(in); err != nil || got != want {
			t.Errorf("NormalizeColour(%q) = %q, %v", in, got, err)
		}
	}
	for _, in := range []string{"", "red", "#abc", "#abcdeg", "#aabbcc;}", "url(x)"} {
		if _, err := NormalizeColour(in); err == nil {
			t.Errorf("NormalizeColour(%q) accepted", in)
		}
	}

	for in, want := range map[string]string{
		"":                            "",
		"https://Tennis.Example.org/": "https://tennis.example.org",
		"http://localhost:8080":       "http://localhost:8080",
		"https://example.org/club/":   "https://example.org/club",
		"  https://example.org  ":     "https://example.org",
	} {
		if got, err := NormalizePublicURL(in); err != nil || got != want {
			t.Errorf("NormalizePublicURL(%q) = %q, %v", in, got, err)
		}
	}
	for _, in := range []string{"example.org", "ftp://example.org", "javascript:alert(1)", "https://", "https://u:p@example.org", "https://example.org/?a=1", "https://example.org/#x", "//example.org"} {
		if _, err := NormalizePublicURL(in); err == nil {
			t.Errorf("NormalizePublicURL(%q) accepted", in)
		}
	}

	if got, err := NormalizeBHPLTAClubID(" 10 "); err != nil || got != "10" {
		t.Errorf("NormalizeBHPLTAClubID = %q, %v", got, err)
	}
	for _, in := range []string{"0", "-1", "ten", "1.5"} {
		if _, err := NormalizeBHPLTAClubID(in); err == nil {
			t.Errorf("NormalizeBHPLTAClubID(%q) accepted", in)
		}
	}
	if _, err := NormalizeBHPLTAText("code", "abc; other=1"); err == nil {
		t.Errorf("semicolon accepted in a value sent as a cookie")
	}
	if got, err := NormalizeDisplayName("  Alpha   TC "); err != nil || got != "Alpha TC" {
		t.Errorf("NormalizeDisplayName = %q, %v", got, err)
	}
}
