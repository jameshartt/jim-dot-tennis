// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

// Package jimdottennis holds the files built into the binary: the HTML
// templates, the static assets and the SQL migrations. go:embed cannot reach
// parent directories, so this file has to sit at the repository root, above
// templates/, static/ and migrations/.
//
// The server uses these copies unless TEMPLATE_RELOAD=true points it at the
// directories on disk for development (cmd/jim-dot-tennis/assets.go).
package jimdottennis

import (
	"embed"
	"io/fs"
)

//go:embed templates static migrations/*.sql
var files embed.FS

// Templates returns the embedded templates/ tree, rooted at templates/.
func Templates() fs.FS { return sub("templates") }

// Static returns the embedded static/ tree, rooted at static/.
func Static() fs.FS { return sub("static") }

// Migrations returns the embedded migration files (NNN_name.up.sql and
// .down.sql), rooted at migrations/.
func Migrations() fs.FS { return sub("migrations") }

func sub(dir string) fs.FS {
	f, err := fs.Sub(files, dir)
	if err != nil {
		// Only possible for an invalid path, which the names above are not.
		panic(err)
	}
	return f
}
