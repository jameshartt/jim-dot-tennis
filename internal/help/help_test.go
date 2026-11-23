// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package help

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const (
	docsDir      = "../../docs"
	templatesDir = "../../templates"
)

func guideFS() fs.FS { return os.DirFS(docsDir) }

func TestEveryGuideExistsAndRenders(t *testing.T) {
	for _, g := range Guides() {
		src, err := fs.ReadFile(guideFS(), g.File)
		if err != nil {
			t.Fatalf("%s: %v", g.Slug, err)
		}
		r, err := Render(src)
		if err != nil {
			t.Fatalf("%s: %v", g.Slug, err)
		}
		if r.Title == "" || len(r.Headings) < 3 {
			t.Errorf("%s: title %q, %d section headings", g.Slug, r.Title, len(r.Headings))
		}
		if !strings.Contains(string(r.HTML), "<h2 id=") {
			t.Errorf("%s: headings carry no ids", g.Slug)
		}
	}
}

func TestGuidesAvoidWordsWeKeepOutOfUserText(t *testing.T) {
	for _, g := range Guides() {
		src, _ := fs.ReadFile(guideFS(), g.File)
		lower := strings.ToLower(string(src))
		for _, bad := range []string{"provider", "jim.tennis", "—", "–"} {
			if strings.Contains(lower, bad) {
				t.Errorf("%s contains %q", g.File, bad)
			}
		}
	}
}

func TestPlayerSheetHasNoWarnings(t *testing.T) {
	src, _ := fs.ReadFile(guideFS(), "guide-player.md")
	lower := strings.ToLower(string(src))
	for _, bad := range []string{"must not", "do not ", "don't", "never", "warning", "rule"} {
		if strings.Contains(lower, bad) {
			t.Errorf("the player sheet contains %q; keep it positive", bad)
		}
	}
}

func TestLinksBetweenGuidesStayInTheApp(t *testing.T) {
	r, err := Render([]byte("See the [captain guide](guide-captain.md#pick-the-team) and the [install guide](install.md) now.\n"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(r.HTML)
	if !strings.Contains(html, `href="/help/captain#pick-the-team"`) {
		t.Errorf("guide link not rewritten: %s", html)
	}
	if strings.Contains(html, "install.md\"") || !strings.Contains(html, "install guide (docs/install.md)") {
		t.Errorf("link to an unserved document not turned into text: %s", html)
	}
}

func TestRawHTMLIsNotPassedThrough(t *testing.T) {
	r, err := Render([]byte("# T\n\n<script>alert(1)</script>\n\n[x](javascript:alert(1))\n"))
	if err != nil {
		t.Fatal(err)
	}
	h := string(r.HTML)
	if strings.Contains(h, "<script>") || strings.Contains(h, `href="javascript:`) {
		t.Errorf("unsafe HTML rendered: %s", h)
	}
}

func TestTablesRender(t *testing.T) {
	src, _ := fs.ReadFile(guideFS(), "guide-admin.md")
	r, _ := Render(src)
	if !strings.Contains(string(r.HTML), "<table>") {
		t.Error("the roles table did not render as a table")
	}
}

// The help links in the admin templates must point at a guide section that exists.
func TestAdminHelpLinksPointAtRealSections(t *testing.T) {
	anchors := map[string]map[string]bool{}
	for _, g := range Guides() {
		src, _ := fs.ReadFile(guideFS(), g.File)
		ids, err := Anchors(src)
		if err != nil {
			t.Fatal(err)
		}
		anchors[g.Slug] = map[string]bool{}
		for _, id := range ids {
			anchors[g.Slug][id] = true
		}
	}
	re := regexp.MustCompile(`href="/help/([a-z]+)(?:#([a-z0-9-]+))?"`)
	files, _ := filepath.Glob(filepath.Join(templatesDir, "admin", "*.html"))
	more, _ := filepath.Glob(filepath.Join(templatesDir, "admin_standalone.html"))
	files = append(files, more...)
	found := 0
	for _, f := range files {
		b, _ := os.ReadFile(f)
		for _, m := range re.FindAllStringSubmatch(string(b), -1) {
			found++
			set, ok := anchors[m[1]]
			if !ok {
				t.Errorf("%s: unknown guide %q", f, m[1])
				continue
			}
			if m[2] != "" && !set[m[2]] {
				t.Errorf("%s: guide %q has no section %q", f, m[1], m[2])
			}
		}
	}
	if found < 10 {
		t.Errorf("only %d help links found in the admin templates", found)
	}
}

func newTestMux(t *testing.T) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	requireAuth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Test-Signed-In") == "" {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
	New(guideFS(), templatesDir).RegisterRoutes(mux, requireAuth)
	return mux
}

func get(mux *http.ServeMux, path string, signedIn bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if signedIn {
		req.Header.Set("X-Test-Signed-In", "1")
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestAccess(t *testing.T) {
	mux := newTestMux(t)

	if rec := get(mux, "/help/player", false); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Welcome to the team") {
		t.Errorf("player sheet signed out: %d", rec.Code)
	}
	for _, p := range []string{"/help", "/help/admin", "/help/captain", "/help/tournaments"} {
		if rec := get(mux, p, false); rec.Code != http.StatusSeeOther {
			t.Errorf("%s signed out: %d, want a redirect to sign in", p, rec.Code)
		}
		if rec := get(mux, p, true); rec.Code != http.StatusOK {
			t.Errorf("%s signed in: %d", p, rec.Code)
		}
	}
	if rec := get(mux, "/help/nope", true); rec.Code != http.StatusNotFound {
		t.Errorf("unknown guide: %d", rec.Code)
	}
	rec := get(mux, "/help/admin", true)
	body := rec.Body.String()
	for _, want := range []string{`data-guide="admin"`, `id="invite-a-captain"`, "<table>", "On this page"} {
		if !strings.Contains(body, want) {
			t.Errorf("admin guide page lacks %q", want)
		}
	}
}
