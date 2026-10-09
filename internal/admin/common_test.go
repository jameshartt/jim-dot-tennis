// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTemplateDir(t *testing.T, page string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "admin", "partials"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"admin/page.html":          page,
		"admin/partials/note.html": `<em>{{.}}</em>`,
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// Pages are parsed once, not re-read from disk on every request.
func TestParseTemplateIsCached(t *testing.T) {
	dir := writeTemplateDir(t, `<p>{{template "admin/partials/note.html" .}}</p>`)

	first, err := parseTemplate(dir, "admin/page.html")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	second, err := parseTemplate(dir, "admin/page.html")
	if err != nil {
		t.Fatalf("parse again: %v", err)
	}
	if first != second {
		t.Fatal("template was re-parsed on the second request")
	}

	rec := httptest.NewRecorder()
	if err := renderTemplate(rec, second, "hi"); err != nil {
		t.Fatalf("render: %v", err)
	}
	if got := rec.Body.String(); got != "<p><em>hi</em></p>" {
		t.Fatalf("render with partial: got %q", got)
	}
}

type failingPage struct{}

func (failingPage) Boom() (string, error) { return "", errors.New("boom") }

// A render that fails halfway must not commit a 200 with half a page: the
// caller's error response has to be what the browser sees.
func TestRenderTemplateFailureLeavesNoPartialPage(t *testing.T) {
	dir := writeTemplateDir(t, `<h1>Half a page</h1>{{.Boom}}`)
	tmpl, err := parseTemplate(dir, "admin/page.html")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	rec := httptest.NewRecorder()
	if err := renderTemplate(rec, tmpl, failingPage{}); err == nil {
		t.Fatal("expected the render error to be returned")
	} else {
		logAndError(rec, "Failed to render template", err, http.StatusInternalServerError)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "Half a page") {
		t.Errorf("partial page leaked into the error response:\n%s", rec.Body.String())
	}
}

// The fallback page is only ever rendered when a template failed to load. It
// must report a server error so broken templates show up in logs/monitoring
// instead of masquerading as a successful "coming soon" page.
func TestRenderFallbackHTMLIsServerError(t *testing.T) {
	rec := httptest.NewRecorder()
	renderFallbackHTML(rec, "Teams", "Teams", "Teams page coming soon", "/admin/league/dashboard")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want 500", rec.Code)
	}
}
