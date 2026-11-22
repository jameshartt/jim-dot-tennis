// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

// Package assets serves the static files under /static/ with cache headers
// that suit how they are referenced:
//
//   - A URL carrying the current cache-buster (?v=<Version>, written by the
//     asset template function in internal/render) is immutable for a year:
//     a new build with different files has a different version, so browsers
//     fetch the new URL.
//   - Any other URL (images and icons, files referenced from CSS or JS, old
//     versions) must be revalidated (no-cache), answered with 304 from the
//     file's ETag when it has not changed.
//   - In development (files read from disk) nothing is marked immutable, so
//     edits show on the next reload.
//
// Directory listings are not served.
package assets

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"net/http"
	"path"
	"sort"
	"strings"
)

// Version returns a short hash of every file's path and content in fsys. It
// changes whenever any static file changes, which is what a cache-buster
// needs.
func Version(fsys fs.FS) (string, error) {
	sums, err := hashFiles(fsys)
	if err != nil {
		return "", err
	}
	names := make([]string, 0, len(sums))
	for name := range sums {
		names = append(names, name)
	}
	sort.Strings(names)
	h := sha256.New()
	for _, name := range names {
		h.Write([]byte(name))
		h.Write([]byte{0})
		h.Write([]byte(sums[name]))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:12], nil
}

// hashFiles returns the SHA-256 (hex) of every regular file in fsys by path.
func hashFiles(fsys fs.FS) (map[string]string, error) {
	sums := map[string]string{}
	err := fs.WalkDir(fsys, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		sums[name] = hex.EncodeToString(sum[:])
		return nil
	})
	return sums, err
}

// Handler serves fsys under prefix (for example "/static/").
type Handler struct {
	prefix  string
	version string
	dev     bool
	files   http.Handler
	etags   map[string]string // by path inside fsys; nil in development
}

// NewHandler returns a handler for the files in fsys. version is the current
// cache-buster (from Version). dev marks files read from disk that may change
// while the server runs: then nothing is cached as immutable and the file
// server's Last-Modified is used for revalidation instead of a precomputed
// ETag.
func NewHandler(prefix string, fsys fs.FS, version string, dev bool) (*Handler, error) {
	h := &Handler{prefix: prefix, version: version, dev: dev, files: http.FileServerFS(fsys)}
	if !dev {
		sums, err := hashFiles(fsys)
		if err != nil {
			return nil, err
		}
		h.etags = make(map[string]string, len(sums))
		for name, sum := range sums {
			h.etags[name] = `"` + sum[:32] + `"`
		}
	}
	return h, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name, ok := strings.CutPrefix(r.URL.Path, h.prefix)
	if !ok || name == "" || strings.HasSuffix(name, "/") {
		http.NotFound(w, r)
		return
	}
	name = path.Clean(name)
	if !h.dev {
		etag, known := h.etags[name]
		if !known {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("ETag", etag)
	}
	if !h.dev && h.version != "" && r.URL.Query().Get("v") == h.version {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	r2 := r.Clone(r.Context())
	r2.URL.Path = "/" + name
	r2.URL.RawPath = ""
	h.files.ServeHTTP(w, r2)
}
