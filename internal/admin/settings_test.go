// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/config"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
	"jim-dot-tennis/internal/services"
)

type settingsTestEnv struct {
	db      *database.DB
	holder  *config.HomeClubHolder
	handler *SettingsHandler
}

func newSettingsTestEnv(t *testing.T) settingsTestEnv {
	t.Helper()
	for _, k := range []string{"HOME_CLUB_NAME", config.EnvLogoPath, config.EnvBHPLTAClubCode, config.EnvPublicURL} {
		t.Setenv(k, "")
	}
	t.Setenv("HOME_CLUB_ID", "1")
	t.Setenv("DB_PATH", filepath.Join(t.TempDir(), "tennis.db"))

	db, err := database.New(database.Config{Driver: "sqlite3", FilePath: filepath.Join(t.TempDir(), "settings.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.ExecuteMigrations(findMigrationsPathAdmin(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO clubs (id, name) VALUES (1, 'Alpha Tennis Club')`); err != nil {
		t.Fatal(err)
	}
	return newSettingsTestEnvOn(t, db)
}

func newSettingsTestEnvOn(t *testing.T, db *database.DB) settingsTestEnv {
	t.Helper()
	holder, err := config.NewHomeClubHolder(context.Background(), repository.NewClubRepository(db), repository.NewSettingsRepository(db))
	if err != nil {
		t.Fatal(err)
	}
	h := NewSettingsHandler(NewService(db, "", holder, ""), findTemplatesPathAdmin(t))
	h.lookup = func(context.Context, string) (services.BHPLTAClubIdentity, error) {
		return services.BHPLTAClubIdentity{}, errors.New("no network in tests")
	}
	return settingsTestEnv{db: db, holder: holder, handler: h}
}

// do sends the request as role (empty role: no signed-in user), with the
// home club and branding in context as the server's middleware sets them.
func (e settingsTestEnv) do(req *http.Request, role models.Role) *httptest.ResponseRecorder {
	ctx := config.WithHomeClub(req.Context(), e.holder.ID(), e.holder.Club(), e.holder.LogoPath())
	ctx = config.WithBranding(ctx, e.holder.Branding())
	if role != "" {
		ctx = context.WithValue(ctx, auth.UserContextKey, models.User{ID: 1, Username: "someone", Role: role})
	}
	rec := httptest.NewRecorder()
	e.handler.HandleSettings(rec, req.WithContext(ctx))
	return rec
}

type upload struct {
	field, filename string
	data            []byte
}

func settingsForm(t *testing.T, fields map[string]string, files ...upload) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range files {
		fw, err := mw.CreateFormFile(f.field, f.filename)
		if err != nil {
			t.Fatal(err)
		}
		fw.Write(f.data)
	}
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, settingsPath, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func validSettingsFields() map[string]string {
	return map[string]string{
		"display_name":     "Alpha TC",
		"primary_colour":   "#112233",
		"secondary_colour": "#445566",
		"bhplta_club_code": "ALPHA01",
		"bhplta_club_name": "Alpha+Park",
		"bhplta_club_id":   "42",
		"public_url":       "https://tennis.example.org/",
	}
}

func smallPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestSettingsAdminOnly(t *testing.T) {
	e := newSettingsTestEnv(t)
	get := func() *http.Request { return httptest.NewRequest(http.MethodGet, settingsPath, nil) }

	if rec := e.do(get(), ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("signed out GET: %d", rec.Code)
	}
	for _, role := range []models.Role{models.RoleCaptain, models.RoleTournamentsOnly} {
		if rec := e.do(get(), role); rec.Code != http.StatusForbidden {
			t.Errorf("%s GET: %d", role, rec.Code)
		}
		if rec := e.do(settingsForm(t, validSettingsFields()), role); rec.Code != http.StatusForbidden {
			t.Errorf("%s POST: %d", role, rec.Code)
		}
		lookup := httptest.NewRequest(http.MethodPost, settingsLookupPath, nil)
		if rec := e.do(lookup, role); rec.Code != http.StatusForbidden {
			t.Errorf("%s lookup: %d", role, rec.Code)
		}
	}
	if v, _, _ := repository.NewSettingsRepository(e.db).GetString(context.Background(), repository.SettingClubDisplayName); v != "" {
		t.Errorf("a non-admin POST stored a display name: %q", v)
	}

	rec := e.do(get(), models.RoleAdmin)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin GET: %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"Club Settings", `name="display_name"`, `placeholder="Alpha Tennis Club"`, `value="#4cb04d"`, "Where to find the club name and id"} {
		if !strings.Contains(body, want) {
			t.Errorf("settings page lacks %q", want)
		}
	}
}

func TestSettingsSaveTakesEffectWithoutRestart(t *testing.T) {
	e := newSettingsTestEnv(t)
	logo := smallPNG(t, 256, 256)

	rec := e.do(settingsForm(t, validSettingsFields(), upload{"logo", "logo.svg", logo}), models.RoleAdmin)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}

	b := e.holder.Branding()
	if b.Name != "Alpha TC" || b.PrimaryColour != "#112233" || b.SecondaryColour != "#445566" {
		t.Errorf("branding = %+v", b)
	}
	if !strings.HasPrefix(b.LogoPath, config.LogoURLPrefix+"logo-") || !strings.HasSuffix(b.LogoPath, ".png") || b.LogoWidth != 256 {
		t.Errorf("logo = %q %dx%d (named by content, not by the upload's file name)", b.LogoPath, b.LogoWidth, b.LogoHeight)
	}
	stored := filepath.Join(config.BrandingDir(), strings.TrimPrefix(b.LogoPath, config.LogoURLPrefix))
	if data, err := os.ReadFile(stored); err != nil || !bytes.Equal(data, logo) {
		t.Errorf("logo not stored next to the database at %s: %v", stored, err)
	}
	cs := e.holder.ClubSettings()
	club := e.holder.Club()
	if cs.BHPLTAClubCode() != "ALPHA01" || cs.BHPLTAClubName(club) != "Alpha+Park" || cs.BHPLTAClubID(club) != 42 {
		t.Errorf("BHPLTA = %q %q %d", cs.BHPLTAClubCode(), cs.BHPLTAClubName(club), cs.BHPLTAClubID(club))
	}
	if cs.PublicURL() != "https://tennis.example.org" {
		t.Errorf("public URL = %q", cs.PublicURL())
	}

	// A new holder over the same database (a restart) sees the same values.
	if got := newSettingsTestEnvOn(t, e.db).holder.Branding(); got != b {
		t.Errorf("after restart: %+v, want %+v", got, b)
	}

	// Replacing the logo removes the old file; removing it clears the setting.
	rec = e.do(settingsForm(t, validSettingsFields(), upload{"logo", "x.png", smallPNG(t, 300, 300)}), models.RoleAdmin)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("replace: %d", rec.Code)
	}
	if _, err := os.Stat(stored); !os.IsNotExist(err) {
		t.Errorf("old logo still on disk")
	}
	fields := validSettingsFields()
	fields["remove_logo"] = "on"
	if rec := e.do(settingsForm(t, fields), models.RoleAdmin); rec.Code != http.StatusSeeOther {
		t.Fatalf("remove: %d", rec.Code)
	}
	if e.holder.LogoPath() != "" {
		t.Errorf("logo still set after removing it: %q", e.holder.LogoPath())
	}
	if entries, _ := os.ReadDir(config.BrandingDir()); len(entries) != 0 {
		t.Errorf("branding dir not empty after removing the logo: %d files", len(entries))
	}
}

func TestSettingsRejectsBadInputAndStoresNothing(t *testing.T) {
	e := newSettingsTestEnv(t)
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)

	cases := []struct {
		name   string
		mutate func(map[string]string)
		file   *upload
		want   string
	}{
		{"svg logo", nil, &upload{"logo", "logo.png", svg}, "PNG, JPEG or WebP"},
		{"text logo", nil, &upload{"logo", "logo.png", []byte("not an image at all")}, "PNG, JPEG or WebP"},
		{"bad colour", func(f map[string]string) { f["primary_colour"] = "red;}" }, nil, "Primary colour"},
		{"bad club id", func(f map[string]string) { f["bhplta_club_id"] = "ten" }, nil, "club id"},
		{"bad public URL", func(f map[string]string) { f["public_url"] = "javascript:alert(1)" }, nil, "https://"},
		{"cookie injection", func(f map[string]string) { f["bhplta_club_code"] = "A; admin=1" }, nil, "semicolons"},
	}
	for _, c := range cases {
		fields := validSettingsFields()
		if c.mutate != nil {
			c.mutate(fields)
		}
		var files []upload
		if c.file != nil {
			files = append(files, *c.file)
		}
		rec := e.do(settingsForm(t, fields, files...), models.RoleAdmin)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), c.want) {
			t.Errorf("%s: %d, body lacks %q", c.name, rec.Code, c.want)
		}
	}
	if b := e.holder.Branding(); b.Name != "Alpha Tennis Club" || b.LogoPath != "" || b.PrimaryColour != config.DefaultPrimaryColour {
		t.Errorf("rejected posts changed the branding: %+v", b)
	}
	if entries, _ := os.ReadDir(config.BrandingDir()); len(entries) != 0 {
		t.Errorf("rejected uploads left files behind")
	}
}

func TestSettingsLogoSizeLimit(t *testing.T) {
	e := newSettingsTestEnv(t)
	big := append(smallPNG(t, 10, 10), make([]byte, 3<<20)...)
	rec := e.do(settingsForm(t, validSettingsFields(), upload{"logo", "big.png", big}), models.RoleAdmin)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "at most 2 MB") {
		t.Errorf("oversized upload: %d", rec.Code)
	}
}

func TestSettingsEnvOverridesAreShownAndKept(t *testing.T) {
	e := newSettingsTestEnv(t)
	t.Setenv(config.EnvBHPLTAClubCode, "FROMENV")
	t.Setenv(config.EnvPublicURL, "https://env.example.org")
	t.Setenv(config.EnvLogoPath, "/static/my-club.jpg")
	e = newSettingsTestEnvOn(t, e.db)

	rec := e.do(httptest.NewRequest(http.MethodGet, settingsPath, nil), models.RoleAdmin)
	body := rec.Body.String()
	for _, want := range []string{"<code>BHPLTA_CLUB_CODE</code>", "<code>PUBLIC_URL</code>", "<code>HOME_CLUB_LOGO_PATH</code>", `value="https://env.example.org" disabled`} {
		if !strings.Contains(body, want) {
			t.Errorf("page lacks %q", want)
		}
	}
	if strings.Contains(body, "FROMENV") {
		t.Errorf("the env club code is shown on the page")
	}

	// Overridden fields are disabled (not submitted); saving must not clear what is stored.
	repo := repository.NewSettingsRepository(e.db)
	ctx := context.Background()
	if err := repo.SetString(ctx, repository.SettingBHPLTAClubCode, "STORED"); err != nil {
		t.Fatal(err)
	}
	fields := validSettingsFields()
	delete(fields, "bhplta_club_code")
	delete(fields, "public_url")
	if rec := e.do(settingsForm(t, fields), models.RoleAdmin); rec.Code != http.StatusSeeOther {
		t.Fatalf("save: %d", rec.Code)
	}
	if v, _, _ := repo.GetString(ctx, repository.SettingBHPLTAClubCode); v != "STORED" {
		t.Errorf("stored code = %q, want it kept", v)
	}
	if e.holder.BHPLTAClubCode() != "FROMENV" || e.holder.LogoPath() != "/static/my-club.jpg" {
		t.Errorf("env no longer wins: %q %q", e.holder.BHPLTAClubCode(), e.holder.LogoPath())
	}
}

func TestSettingsBHPLTALookup(t *testing.T) {
	e := newSettingsTestEnv(t)
	var gotCode string
	e.handler.lookup = func(_ context.Context, code string) (services.BHPLTAClubIdentity, error) {
		gotCode = code
		return services.BHPLTAClubIdentity{Name: "Alpha+Park", ID: 42}, nil
	}
	if err := e.holder.ClubSettings().Save(context.Background(), map[string]string{repository.SettingBHPLTAClubCode: "ALPHA01"}); err != nil {
		t.Fatal(err)
	}
	rec := e.do(httptest.NewRequest(http.MethodPost, settingsLookupPath, nil), models.RoleAdmin)
	if rec.Code != http.StatusSeeOther || gotCode != "ALPHA01" {
		t.Fatalf("lookup: %d, code %q", rec.Code, gotCode)
	}
	cs := e.holder.ClubSettings()
	if cs.BHPLTAClubName(nil) != "Alpha+Park" || cs.BHPLTAClubID(nil) != 42 {
		t.Errorf("lookup did not store: %q %d", cs.BHPLTAClubName(nil), cs.BHPLTAClubID(nil))
	}
}

func TestMatchCardImportFormUsesStoredIdentifiers(t *testing.T) {
	e := newSettingsTestEnv(t)
	if err := e.holder.ClubSettings().Save(context.Background(), map[string]string{
		repository.SettingBHPLTAClubName: "Alpha+Park",
		repository.SettingBHPLTAClubID:   "42",
		repository.SettingBHPLTAClubCode: "ALPHA01",
	}); err != nil {
		t.Fatal(err)
	}
	h := NewMatchCardImportationHandler(e.handler.service, findTemplatesPathAdmin(t))
	req := httptest.NewRequest(http.MethodGet, "/admin/league/match-card-import", nil)
	ctx := config.WithHomeClub(req.Context(), e.holder.ID(), e.holder.Club(), "")
	ctx = context.WithValue(ctx, auth.UserContextKey, models.User{ID: 1, Username: "a", Role: models.RoleAdmin})
	rec := httptest.NewRecorder()
	h.HandleMatchCardImportation(rec, req.WithContext(ctx))
	body := rec.Body.String()
	for _, want := range []string{`value="Alpha&#43;Park"`, `value="42"`, `value="ALPHA01"`} {
		if !strings.Contains(body, want) {
			t.Errorf("import form lacks %s", want)
		}
	}
	if strings.Contains(body, "St+Anns") || strings.Contains(body, "St&#43;Anns") {
		t.Errorf("import form still hardcodes St Ann's")
	}
}

// The known club's bundled logo is shown, and labelled, when nothing is
// uploaded; another club sees no logo.
func TestSettingsShowsKnownClubLogo(t *testing.T) {
	e := newSettingsTestEnv(t)
	rec := e.do(httptest.NewRequest(http.MethodGet, settingsPath, nil), models.RoleAdmin)
	if body := rec.Body.String(); !strings.Contains(body, `id="no-logo"`) || strings.Contains(body, `id="logo-known"`) {
		t.Errorf("unknown club: want no logo")
	}

	if _, err := e.db.Exec(`UPDATE clubs SET name = 'St Ann''s Tennis Club' WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	e = newSettingsTestEnvOn(t, e.db)
	if got := e.holder.LogoPath(); got != "/static/st-anns-tennis.jpg" {
		t.Fatalf("known club LogoPath = %q", got)
	}
	rec = e.do(httptest.NewRequest(http.MethodGet, settingsPath, nil), models.RoleAdmin)
	body := rec.Body.String()
	if !strings.Contains(body, `src="/static/st-anns-tennis.jpg"`) || !strings.Contains(body, `id="logo-known"`) || strings.Contains(body, `id="remove_logo"`) {
		t.Errorf("known club: bundled logo not shown as such")
	}
}
