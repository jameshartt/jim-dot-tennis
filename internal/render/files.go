// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package render

import (
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

// Where templates are read from. The server calls UseFS once at startup with
// the templates built into the binary, and the handlers then pass paths
// inside that file system (their templateDir is "."). With no file system
// set (TEMPLATE_RELOAD development runs, and tests) paths are ordinary paths
// on disk, read with the os package, exactly as before.
var (
	sourceMu sync.RWMutex
	source   fs.FS

	assetVersion string // see SetAssetVersion
)

// UseFS makes every template read in this package come from fsys (nil means
// the disk). Call it before serving requests.
func UseFS(fsys fs.FS) {
	sourceMu.Lock()
	defer sourceMu.Unlock()
	source = fsys
}

func currentFS() fs.FS {
	sourceMu.RLock()
	defer sourceMu.RUnlock()
	return source
}

// ReloadEnabled reports whether TEMPLATE_RELOAD is set to a true value.
func ReloadEnabled() bool {
	reload, _ := strconv.ParseBool(os.Getenv(ReloadEnv))
	return reload
}

// SetAssetVersion sets the cache-buster the asset template function appends
// to static URLs ("" appends nothing). The server sets it to a hash of the
// static files it serves (internal/assets.Version); a release version could
// be fed in here instead.
func SetAssetVersion(v string) {
	sourceMu.Lock()
	defer sourceMu.Unlock()
	assetVersion = v
}

// AssetVersion returns the value set by SetAssetVersion.
func AssetVersion() string {
	sourceMu.RLock()
	defer sourceMu.RUnlock()
	return assetVersion
}

// Asset returns the URL of a static file with the cache-buster appended, for
// example "/static/css/main.css?v=3f2a9c01b7de". Templates use it as
// {{asset "/static/css/main.css"}}.
func Asset(path string) string {
	if v := AssetVersion(); v != "" {
		return path + "?v=" + v
	}
	return path
}

// funcs are the template functions every page gets from this package.
var funcs = template.FuncMap{"asset": Asset}

// ParseFiles is template.ParseFiles reading from the current source; it also
// adds this package's functions (asset) to t.
func ParseFiles(t *template.Template, names ...string) (*template.Template, error) {
	t = t.Funcs(funcs)
	if fsys := currentFS(); fsys != nil {
		return t.ParseFS(fsys, slashed(names)...)
	}
	return t.ParseFiles(names...)
}

// ParseGlob is template.ParseGlob reading from the current source; it also
// adds this package's functions (asset) to t.
func ParseGlob(t *template.Template, pattern string) (*template.Template, error) {
	t = t.Funcs(funcs)
	if fsys := currentFS(); fsys != nil {
		return t.ParseFS(fsys, filepath.ToSlash(pattern))
	}
	return t.ParseGlob(pattern)
}

// Glob is filepath.Glob over the current source.
func Glob(pattern string) ([]string, error) {
	if fsys := currentFS(); fsys != nil {
		return fs.Glob(fsys, filepath.ToSlash(pattern))
	}
	return filepath.Glob(pattern)
}

// ReadFile is os.ReadFile from the current source.
func ReadFile(name string) ([]byte, error) {
	if fsys := currentFS(); fsys != nil {
		return fs.ReadFile(fsys, filepath.ToSlash(name))
	}
	return os.ReadFile(name)
}

func slashed(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = filepath.ToSlash(n)
	}
	return out
}
