// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package branding

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jim-dot-tennis/internal/config"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/repository"
)

func testImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	return img
}

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, testImage(w, h)); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func jpegBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, testImage(w, h), nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// webpLossless builds the header of a lossless (VP8L) WebP of the given size.
func webpLossless(w, h int) []byte {
	chunk := make([]byte, 5, 16)
	chunk[0] = 0x2f
	binary.LittleEndian.PutUint32(chunk[1:5], uint32(w-1)|uint32(h-1)<<14)
	chunk = append(chunk, make([]byte, 11)...)
	var b bytes.Buffer
	b.WriteString("RIFF")
	_ = binary.Write(&b, binary.LittleEndian, uint32(4+8+len(chunk)))
	b.WriteString("WEBPVP8L")
	_ = binary.Write(&b, binary.LittleEndian, uint32(len(chunk)))
	b.Write(chunk)
	return b.Bytes()
}

// webpExtended builds the header of an extended (VP8X) WebP of the given size.
func webpExtended(w, h int) []byte {
	chunk := make([]byte, 10)
	chunk[4], chunk[5], chunk[6] = byte(w-1), byte((w-1)>>8), byte((w-1)>>16)
	chunk[7], chunk[8], chunk[9] = byte(h-1), byte((h-1)>>8), byte((h-1)>>16)
	var b bytes.Buffer
	b.WriteString("RIFF")
	_ = binary.Write(&b, binary.LittleEndian, uint32(4+8+len(chunk)))
	b.WriteString("WEBPVP8X")
	_ = binary.Write(&b, binary.LittleEndian, uint32(len(chunk)))
	b.Write(chunk)
	return b.Bytes()
}

func TestValidateLogoAccepts(t *testing.T) {
	cases := []struct {
		name      string
		data      []byte
		mime, ext string
		w, h      int
	}{
		{"png", pngBytes(t, 200, 100), "image/png", ".png", 200, 100},
		{"jpeg", jpegBytes(t, 64, 48), "image/jpeg", ".jpg", 64, 48},
		{"webp lossless", webpLossless(300, 300), "image/webp", ".webp", 300, 300},
		{"webp extended", webpExtended(512, 256), "image/webp", ".webp", 512, 256},
	}
	for _, c := range cases {
		logo, err := ValidateLogo(c.data)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if logo.MIME != c.mime || logo.Ext != c.ext || logo.Width != c.w || logo.Height != c.h {
			t.Errorf("%s: got %s %s %dx%d", c.name, logo.MIME, logo.Ext, logo.Width, logo.Height)
		}
	}
}

func TestValidateLogoRejects(t *testing.T) {
	var gifBuf bytes.Buffer
	if err := gif.Encode(&gifBuf, testImage(10, 10), nil); err != nil {
		t.Fatal(err)
	}
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
	svgXML := append([]byte(`<?xml version="1.0"?>`), svg...)
	html := []byte(`<!DOCTYPE html><html><script>alert(1)</script></html>`)
	truncated := pngBytes(t, 10, 10)[:20]
	huge := pngBytes(t, MaxLogoDimension+1, 1)
	tooBig := append(pngBytes(t, 10, 10), make([]byte, MaxLogoBytes)...)

	cases := []struct {
		name string
		data []byte
		want error
	}{
		{"empty", nil, ErrLogoEmpty},
		{"svg", svg, ErrLogoType},
		{"svg with xml prolog", svgXML, ErrLogoType},
		{"html", html, ErrLogoType},
		{"gif", gifBuf.Bytes(), ErrLogoType},
		{"truncated png", truncated, ErrLogoCorrupt},
		{"broken webp", []byte("RIFF\x00\x00\x00\x00WEBPVP8 " + strings.Repeat("\x00", 20)), ErrLogoCorrupt},
		{"too many pixels", huge, ErrLogoSize},
		{"too many bytes", tooBig, ErrLogoTooLarge},
	}
	for _, c := range cases {
		if _, err := ValidateLogo(c.data); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
	}
}

func TestStoreAndRemoveOtherLogos(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "branding")
	first, _ := ValidateLogo(pngBytes(t, 10, 10))
	second, _ := ValidateLogo(jpegBytes(t, 10, 10))

	a, err := Store(dir, first)
	if err != nil {
		t.Fatal(err)
	}
	if !ValidLogoName(a) || !strings.HasSuffix(a, ".png") {
		t.Fatalf("stored name %q", a)
	}
	again, _ := Store(dir, first)
	if again != a {
		t.Errorf("same content stored under a different name: %q vs %q", again, a)
	}
	b, err := Store(dir, second)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "unrelated.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := RemoveOtherLogos(dir, b); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, a)); !os.IsNotExist(err) {
		t.Errorf("old logo %s still present", a)
	}
	if _, err := os.Stat(filepath.Join(dir, b)); err != nil {
		t.Errorf("kept logo missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "unrelated.txt")); err != nil {
		t.Errorf("unrelated file removed")
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".upload-") {
			t.Errorf("temporary file left behind: %s", e.Name())
		}
	}
	if err := RemoveOtherLogos(filepath.Join(dir, "missing"), ""); err != nil {
		t.Errorf("missing dir: %v", err)
	}
}

type brandingEnv struct {
	cs      *config.ClubSettings
	handler *Handler
	mux     *http.ServeMux
}

func newBrandingEnv(t *testing.T) brandingEnv {
	t.Helper()
	t.Setenv(config.EnvLogoPath, "")
	t.Setenv("DB_PATH", filepath.Join(t.TempDir(), "tennis.db"))
	db, err := database.New(database.Config{Driver: "sqlite3", FilePath: filepath.Join(t.TempDir(), "b.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.ExecuteMigrations(filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatal(err)
	}
	cs := config.NewClubSettings(context.Background(), repository.NewSettingsRepository(db))
	h := NewHandler(cs, os.DirFS(filepath.Join("..", "..", "static")))
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	return brandingEnv{cs: cs, handler: h, mux: mux}
}

// serve runs the request with the branding in context, as the server's middleware does.
func (e brandingEnv) serve(req *http.Request) *httptest.ResponseRecorder {
	req = req.WithContext(config.WithBranding(req.Context(), e.cs.Branding(nil)))
	rec := httptest.NewRecorder()
	e.mux.ServeHTTP(rec, req)
	return rec
}

func TestServeLogoOnlyServesTheCurrentLogo(t *testing.T) {
	ctx := context.Background()
	e := newBrandingEnv(t)
	logo, _ := ValidateLogo(pngBytes(t, 256, 256))
	old, _ := ValidateLogo(pngBytes(t, 8, 8))
	name, err := Store(e.cs.Dir(), logo)
	if err != nil {
		t.Fatal(err)
	}
	oldName, _ := Store(e.cs.Dir(), old)

	// Nothing stored yet: 404 even though the file exists.
	if rec := e.serve(httptest.NewRequest(http.MethodGet, "/branding/"+name, nil)); rec.Code != http.StatusNotFound {
		t.Fatalf("before saving: status %d", rec.Code)
	}
	if err := e.cs.Save(ctx, map[string]string{repository.SettingClubLogoFile: name, repository.SettingClubLogoSize: "256x256"}); err != nil {
		t.Fatal(err)
	}

	rec := e.serve(httptest.NewRequest(http.MethodGet, "/branding/"+name, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	for k, want := range map[string]string{
		"Content-Type":            "image/png",
		"X-Content-Type-Options":  "nosniff",
		"Content-Security-Policy": "default-src 'none'; sandbox",
	} {
		if got := rec.Header().Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	if !bytes.Equal(rec.Body.Bytes(), logo.Data) {
		t.Errorf("body differs from the stored logo")
	}

	for _, p := range []string{"/branding/" + oldName, "/branding/../tennis.db", "/branding/", "/branding/logo-0123456789abcdef.svg"} {
		if rec := e.serve(httptest.NewRequest(http.MethodGet, p, nil)); rec.Code != http.StatusNotFound && (rec.Code < 300 || rec.Code > 399) {
			t.Errorf("%s: status %d, want 404", p, rec.Code)
		}
	}
	if rec := e.serve(httptest.NewRequest(http.MethodPost, "/branding/"+name, nil)); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST: status %d", rec.Code)
	}
}

func TestManifestUsesBranding(t *testing.T) {
	ctx := context.Background()
	e := newBrandingEnv(t)
	logo, _ := ValidateLogo(pngBytes(t, 256, 256))
	name, _ := Store(e.cs.Dir(), logo)
	if err := e.cs.Save(ctx, map[string]string{
		repository.SettingClubDisplayName:   `Alpha "TC"`,
		repository.SettingClubPrimaryColour: "#123456",
		repository.SettingClubLogoFile:      name,
		repository.SettingClubLogoSize:      "256x256",
	}); err != nil {
		t.Fatal(err)
	}

	decode := func(path string) manifest {
		t.Helper()
		rec := e.serve(httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/manifest+json" {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Header().Get("Content-Type"))
		}
		var m manifest
		if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
			t.Fatalf("%s: invalid JSON: %v", path, err)
		}
		return m
	}

	m := decode(ManifestPath)
	if m.Name != `Alpha "TC"` || m.ThemeColor != "#123456" || m.StartURL != "/" || m.ID != "/" {
		t.Errorf("manifest = %+v", m)
	}
	if len(m.Icons) == 0 || m.Icons[0].Src != "/branding/"+name || m.Icons[0].Sizes != "256x256" || m.Icons[0].Type != "image/png" {
		t.Errorf("icons = %+v", m.Icons)
	}

	m = decode(PlayerManifestPath + "?start=/my-availability/Some_Token")
	if m.StartURL != "/my-availability/Some_Token" || m.ID != m.StartURL || m.ShortName != "My Tennis" {
		t.Errorf("player manifest = %+v", m)
	}
	// Quotes in the start path cannot break out of the JSON string.
	m = decode(PlayerManifestPath + "?start=" + url.QueryEscape(`/x","scope":"https://evil`))
	if m.Scope != "/" || m.StartURL != `/x","scope":"https://evil` {
		t.Errorf("JSON injection: scope %q start %q", m.Scope, m.StartURL)
	}
	for _, bad := range []string{"https://evil.example", "//evil.example", "/\\evil", "javascript:alert(1)"} {
		m = decode(PlayerManifestPath + "?start=" + url.QueryEscape(bad))
		if m.StartURL != "/" {
			t.Errorf("start=%q gave start_url %q", bad, m.StartURL)
		}
	}
}

func TestServiceWorkerCarriesBranding(t *testing.T) {
	ctx := context.Background()
	e := newBrandingEnv(t)

	// The placeholder must exist in the real file, or branding silently stops applying.
	src, err := os.ReadFile(filepath.Join("..", "..", "static", "service-worker.js"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(src, []byte(serviceWorkerPlaceholder)) {
		t.Fatalf("static/service-worker.js no longer contains the branding placeholder")
	}

	if err := e.cs.Save(ctx, map[string]string{repository.SettingClubDisplayName: "Alpha </script> TC"}); err != nil {
		t.Fatal(err)
	}
	rec := e.serve(httptest.NewRequest(http.MethodGet, ServiceWorkerPath, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if rec.Header().Get("Service-Worker-Allowed") != "/" || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/javascript") {
		t.Errorf("headers = %v", rec.Header())
	}
	body := rec.Body.String()
	if strings.Contains(body, serviceWorkerPlaceholder) {
		t.Errorf("placeholder not replaced")
	}
	want := `var BRANDING = {"icon":"/static/icon-192.svg","name":"Alpha ` + "\\u003c/script\\u003e" + ` TC"};`
	if strings.Contains(body, "</script>") || !strings.Contains(body, want) {
		t.Errorf("branding line missing or unescaped:\n%s", firstLines(body, 8))
	}
}

func firstLines(s string, n int) string {
	lines := strings.SplitN(s, "\n", n+1)
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
