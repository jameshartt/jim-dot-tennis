// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package outbound

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// allowedHosts is every host that may appear in a URL in the shipped source
// (Go code, templates, static files, migrations, deployment files), with how
// it is used. Each "contacted" entry must be listed in
// docs/outbound_connections.md. Adding a host here is a deliberate decision
// that a club's instance may talk to it, or that it is only a link.
var allowedHosts = map[string]string{
	// Contacted by the server.
	"www.bhplta.co.uk":       "contacted: BHPLTA league data, match cards, club lookup (docs/outbound_connections.md)",
	"api.open-meteo.com":     "contacted: weather forecast for upcoming fixtures",
	"courthive-server":       "contacted: CourtHive API inside the instance's own Docker network",
	"courthive-hiveid":       "contacted: CourtHive identity service inside the instance's own Docker network (tournament set-up signs in there; WI-145)",
	"127.0.0.1":              "internal: a container's health check of itself (deploy/images)",
	"app":                    "internal: the app's own Docker service name (health checks, local stack)",
	"app-e2e":                "internal: the end-to-end test app's Docker service name",
	"app-fresh":              "internal: the fresh-install end-to-end app's Docker service name (make test-e2e-fresh only)",
	"app-fresh-demo":         "internal: the fresh-install demo-data end-to-end app's Docker service name (make test-e2e-fresh only)",
	"fresh-fixtures":         "internal: where the fresh-install test run serves the saved BHPLTA pages (BHPLTA_BASE_URL override; make test-e2e-fresh only)",
	"www.tennisabstract.com": "contacted only by the developer utility cmd/collect_tennis_data, never by the server",

	// Loaded by visitors' browsers when a page is shown.
	"www.openstreetmap.org": "browser: embedded venue map on fixture venue and club pages",

	// Links a person may choose to follow; nothing is fetched automatically.
	"en.wikipedia.org": "link: professional player pages (fantasy doubles names)",
	"www.google.com":   "link: Google Maps directions",
	"maps.google.com":  "link: example text in a form field placeholder",
	"wa.me":            "link: share to WhatsApp",
	"github.com":       "link: source code and issues (About page)",
	"courthive.com":    "link: CourtHive project (About page)",

	// Identifiers that are never contacted.
	"www.w3.org": "identifier: XML/SVG namespace",
}

// placeholderHost reports hosts reserved for documentation and examples.
func placeholderHost(h string) bool {
	return h == "localhost" || h == "example.org" || h == "example.com" ||
		strings.HasSuffix(h, ".example.org") || strings.HasSuffix(h, ".example.com") ||
		strings.HasSuffix(h, ".example") || strings.HasSuffix(h, ".test") || strings.HasSuffix(h, ".invalid")
}

var (
	urlHost = regexp.MustCompile(`(?i)\b(?:https?|wss?)://([a-z0-9-]+(?:\.[a-z0-9-]+)*)`)
	// Scheme-relative URLs ("//host/...") in markup, scripts and styles.
	relHost = regexp.MustCompile(`(?i)["'(=]\s*//([a-z0-9-]+(?:\.[a-z0-9-]+)+)`)
)

// scanned is what the scan covers, relative to the repository root.
var (
	scanDirs  = []string{"cmd", "internal", "templates", "static", "migrations", "deploy"}
	scanFiles = []string{"Dockerfile", "docker-compose.yml"}
	textExts  = map[string]bool{
		".go": true, ".html": true, ".js": true, ".mjs": true, ".css": true, ".json": true,
		".sql": true, ".yml": true, ".yaml": true, ".sh": true, ".svg": true, ".webmanifest": true, ".example": true,
	}
	webExts = map[string]bool{".html": true, ".js": true, ".mjs": true, ".css": true, ".svg": true}
)

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("could not find the repository root")
	return ""
}

func scannable(path string) bool {
	base := filepath.Base(path)
	if strings.HasSuffix(base, "_test.go") {
		return false
	}
	if base == "Caddyfile" || strings.HasPrefix(base, "Dockerfile") {
		return true
	}
	return textExts[filepath.Ext(base)]
}

// TestNoUnexpectedOutboundHosts fails when the source names a host that is
// not in allowedHosts. Test files and testdata are not scanned (they never
// run in an instance); hosts built at run time from data (push subscription
// endpoints, an admin-entered fixtures URL, LEAGUE_TABLES_URL) are covered by
// the documentation, not by this scan.
func TestNoUnexpectedOutboundHosts(t *testing.T) {
	root := repoRoot(t)
	found := map[string][]string{}
	scanFile := func(rel string) {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		add := func(re *regexp.Regexp) {
			for _, m := range re.FindAllSubmatch(data, -1) {
				h := strings.ToLower(strings.TrimRight(string(m[1]), ".-"))
				found[h] = append(found[h], rel)
			}
		}
		add(urlHost)
		if webExts[filepath.Ext(rel)] {
			add(relHost)
		}
	}
	files := 0
	for _, dir := range scanDirs {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "testdata" || d.Name() == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			if !scannable(path) {
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			scanFile(rel)
			files++
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range scanFiles {
		scanFile(f)
		files++
	}
	if files < 100 {
		t.Fatalf("scanned only %d files; the scan is not covering the source tree", files)
	}

	var unexpected []string
	for h, where := range found {
		if _, ok := allowedHosts[h]; ok || placeholderHost(h) {
			continue
		}
		sort.Strings(where)
		unexpected = append(unexpected, h+" (in "+strings.Join(uniq(where), ", ")+")")
	}
	sort.Strings(unexpected)
	for _, u := range unexpected {
		t.Errorf("unexpected host %s: add it to allowedHosts and docs/outbound_connections.md only if an instance should contact it", u)
	}

	// Keep the allowlist honest: every entry is still used somewhere.
	for h := range allowedHosts {
		if _, ok := found[h]; !ok {
			t.Errorf("allowedHosts lists %s, which no longer appears in the source; remove it (and from the docs)", h)
		}
	}
}

// The scan must see through the usual ways of writing a URL.
func TestHostPatterns(t *testing.T) {
	cases := map[string]string{
		`<script src="https://cdn.example.net/x.js">`:         "cdn.example.net",
		`@import url('https://fonts.example.net/css?f=1');`:   "fonts.example.net",
		`fmt.Sprintf("https://api.example.net/v1?lat=%f", x)`: "api.example.net",
		`new WebSocket("wss://ws.example.net/socket")`:        "ws.example.net",
		`HTTP://UPPER.EXAMPLE.NET/`:                           "upper.example.net",
	}
	for in, want := range cases {
		m := urlHost.FindStringSubmatch(in)
		if m == nil || strings.ToLower(m[1]) != want {
			t.Errorf("urlHost(%q) = %v, want %s", in, m, want)
		}
	}
	m := relHost.FindStringSubmatch(`<link href="//cdn.example.net/a.css">`)
	if m == nil || m[1] != "cdn.example.net" {
		t.Errorf("relHost missed a scheme-relative URL: %v", m)
	}
}

func uniq(s []string) []string {
	out := s[:0]
	for i, v := range s {
		if i == 0 || v != s[i-1] {
			out = append(out, v)
		}
	}
	return out
}
