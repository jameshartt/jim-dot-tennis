// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package main

import (
	"html/template"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	jimdottennis "jim-dot-tennis"
	"jim-dot-tennis/internal/render"
)

// WI-135: without TEMPLATE_RELOAD the server uses the copies built into the
// binary, even from a directory with no templates/ or static/.
func TestBuiltInFilesUsedOutsideTheRepository(t *testing.T) {
	t.Setenv(render.ReloadEnv, "")
	t.Cleanup(func() { render.UseFS(nil); render.SetAssetVersion("") })

	files := loadFiles(t.TempDir())
	if files.templateDir != "." || render.AssetVersion() == "" {
		t.Fatalf("templateDir %q, asset version %q: want the built-in copies", files.templateDir, render.AssetVersion())
	}

	tmpl, err := loadTemplatesWithFuncs(files.templateDir, template.FuncMap{"currentYear": func() int { return 2026 }})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	if err := render.Execute(rec, tmpl, "about.html", nil); err != nil {
		t.Fatal(err)
	}
	wantCSS := "/static/css/main.css?v=" + render.AssetVersion()
	if !strings.Contains(rec.Body.String(), wantCSS) {
		t.Fatalf("about page does not link %s", wantCSS)
	}

	rec = httptest.NewRecorder()
	files.staticHandler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, wantCSS, nil))
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatalf("versioned main.css: %d, Cache-Control %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
	if _, err := fs.ReadFile(files.static, "service-worker.js"); err != nil {
		t.Fatalf("service worker not built in: %v", err)
	}
}

// With TEMPLATE_RELOAD the files on disk are used, so edits show at once.
func TestReloadReadsFromDisk(t *testing.T) {
	t.Setenv(render.ReloadEnv, "true")
	t.Cleanup(func() { render.UseFS(nil); render.SetAssetVersion("") })
	root, err := getProjectRoot()
	if err != nil {
		t.Fatal(err)
	}
	files := loadFiles(root)
	if files.templateDir != filepath.Join(root, "templates") || render.AssetVersion() != "" {
		t.Fatalf("templateDir %q, asset version %q: want the disk copies", files.templateDir, render.AssetVersion())
	}
}

// Every migration on disk is built in (the embed pattern is migrations/*.sql).
func TestEveryMigrationIsBuiltIn(t *testing.T) {
	onDisk, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.sql"))
	if err != nil || len(onDisk) == 0 {
		t.Fatalf("no migrations on disk: %v", err)
	}
	for _, path := range onDisk {
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		got, err := fs.ReadFile(jimdottennis.Migrations(), filepath.Base(path))
		if err != nil || string(got) != string(want) {
			t.Fatalf("%s is not built in as on disk: %v", path, err)
		}
	}
}
