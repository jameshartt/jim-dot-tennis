// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package main

import (
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"

	jimdottennis "jim-dot-tennis"
	"jim-dot-tennis/internal/assets"
	"jim-dot-tennis/internal/render"
)

// serverFiles says where the server reads its templates and static files.
type serverFiles struct {
	templateDir   string       // passed to the handlers; a path inside render's source
	static        fs.FS        // static/ tree (service-worker.js is read from it)
	staticHandler http.Handler // serves /static/
}

// loadFiles picks the templates and static files the server uses. Normally
// they are the copies built into the binary, so it runs from any directory
// with only its data directory. With TEMPLATE_RELOAD=true (development) they
// are read from templates/ and static/ under projectRoot on every request, so
// edits show without a rebuild; if those directories are missing the built-in
// copies are used. Migrations are always the built-in ones.
func loadFiles(projectRoot string) serverFiles {
	diskTemplates := filepath.Join(projectRoot, "templates")
	diskStatic := filepath.Join(projectRoot, "static")
	if render.ReloadEnabled() {
		if isDir(diskTemplates) && isDir(diskStatic) {
			log.Printf("TEMPLATE_RELOAD: reading templates and static files from %s", projectRoot)
			static := os.DirFS(diskStatic)
			return serverFiles{
				templateDir:   diskTemplates,
				static:        static,
				staticHandler: mustStaticHandler(static, "", true),
			}
		}
		log.Printf("TEMPLATE_RELOAD is set but %s has no templates/ and static/; using the built-in copies", projectRoot)
	}

	render.UseFS(jimdottennis.Templates())
	static := jimdottennis.Static()
	version, err := assets.Version(static)
	if err != nil {
		log.Fatalf("Failed to read the built-in static files: %v", err)
	}
	render.SetAssetVersion(version)
	return serverFiles{
		templateDir:   ".",
		static:        static,
		staticHandler: mustStaticHandler(static, version, false),
	}
}

func mustStaticHandler(static fs.FS, version string, dev bool) http.Handler {
	h, err := assets.NewHandler("/static/", static, version, dev)
	if err != nil {
		log.Fatalf("Failed to read the static files: %v", err)
	}
	return h
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
