// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package assets

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func testFS() fstest.MapFS {
	return fstest.MapFS{
		"css/main.css": {Data: []byte("body{}")},
		"app.js":       {Data: []byte("console.log(1)")},
	}
}

func get(t *testing.T, h http.Handler, target string, hdr map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestVersionChangesWithContent(t *testing.T) {
	a, err := Version(testFS())
	if err != nil {
		t.Fatal(err)
	}
	changed := testFS()
	changed["app.js"] = &fstest.MapFile{Data: []byte("console.log(2)")}
	b, err := Version(changed)
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 12 || a == b {
		t.Fatalf("version %q / %q: want 12 hex characters that change with content", a, b)
	}
	again, _ := Version(testFS())
	if again != a {
		t.Fatalf("version not stable: %q then %q", a, again)
	}
}

func TestCacheHeaders(t *testing.T) {
	h, err := NewHandler("/static/", testFS(), "abc123", false)
	if err != nil {
		t.Fatal(err)
	}

	rec := get(t, h, "/static/css/main.css?v=abc123", nil)
	if rec.Code != http.StatusOK || rec.Body.String() != "body{}" {
		t.Fatalf("versioned: %d %q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("versioned Cache-Control = %q", got)
	}
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("no ETag")
	}

	for _, target := range []string{"/static/css/main.css", "/static/css/main.css?v=old"} {
		rec = get(t, h, target, nil)
		if got := rec.Header().Get("Cache-Control"); rec.Code != http.StatusOK || got != "no-cache" {
			t.Fatalf("%s: %d Cache-Control = %q, want no-cache", target, rec.Code, got)
		}
	}

	rec = get(t, h, "/static/css/main.css", map[string]string{"If-None-Match": etag})
	if rec.Code != http.StatusNotModified {
		t.Fatalf("revalidation with the ETag: %d, want 304", rec.Code)
	}

	for _, target := range []string{"/static/", "/static/css/", "/static/css", "/static/missing.js", "/static/../x"} {
		if rec := get(t, h, target, nil); rec.Code != http.StatusNotFound {
			t.Fatalf("%s: %d, want 404", target, rec.Code)
		}
	}
}

func TestDevelopmentIsNeverImmutable(t *testing.T) {
	h, err := NewHandler("/static/", testFS(), "abc123", true)
	if err != nil {
		t.Fatal(err)
	}
	rec := get(t, h, "/static/app.js?v=abc123", nil)
	if got := rec.Header().Get("Cache-Control"); rec.Code != http.StatusOK || got != "no-cache" {
		t.Fatalf("dev: %d Cache-Control = %q, want no-cache", rec.Code, got)
	}
}
