// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package jimdottennis

import (
	"embed"
	"io/fs"
)

// guideFiles holds the user guides (docs/guide-*.md) so the app can show them
// itself, matching the installed version. The Dockerfile build context lets
// these files through (.dockerignore).
//
//go:embed docs/guide-*.md
var guideFiles embed.FS

// Guides returns the embedded user guides, rooted at docs/.
func Guides() fs.FS {
	f, err := fs.Sub(guideFiles, "docs")
	if err != nil {
		panic(err)
	}
	return f
}
