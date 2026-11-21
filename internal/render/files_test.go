// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package render

import (
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestParseFromFSWithAssetFunc(t *testing.T) {
	UseFS(fstest.MapFS{
		"layout.html":           {Data: []byte(`{{define "layout"}}<link href="{{asset "/static/a.css"}}">{{end}}`)},
		"admin/page.html":       {Data: []byte(`{{template "layout"}}page`)},
		"admin/partials/p.html": {Data: []byte(`partial`)},
	})
	SetAssetVersion("v1")
	t.Cleanup(func() { UseFS(nil); SetAssetVersion("") })

	tmpl, err := ParseFiles(template.New("page.html"), "layout.html", filepath.Join(".", "admin", "page.html"))
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	if err := tmpl.ExecuteTemplate(&b, "page.html", nil); err != nil {
		t.Fatal(err)
	}
	if got := b.String(); got != `<link href="/static/a.css?v=v1">page` {
		t.Fatalf("rendered %q", got)
	}

	names, err := Glob(filepath.Join(".", "admin", "partials", "*.html"))
	if err != nil || len(names) != 1 || names[0] != "admin/partials/p.html" {
		t.Fatalf("Glob = %v, %v", names, err)
	}
	if data, err := ReadFile(names[0]); err != nil || string(data) != "partial" {
		t.Fatalf("ReadFile = %q, %v", data, err)
	}
}

func TestParseFromDiskByDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.html")
	if err := os.WriteFile(path, []byte(`<script src="{{asset "/static/x.js"}}"></script>`), 0o600); err != nil {
		t.Fatal(err)
	}
	tmpl, err := ParseGlob(template.New(""), filepath.Join(dir, "*.html"))
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	if err := tmpl.ExecuteTemplate(&b, "x.html", nil); err != nil {
		t.Fatal(err)
	}
	// No version set: the plain path.
	if got := b.String(); got != `<script src="/static/x.js"></script>` {
		t.Fatalf("rendered %q", got)
	}
}
