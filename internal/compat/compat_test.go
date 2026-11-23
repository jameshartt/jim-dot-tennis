// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package compat

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func repoFile(t *testing.T, name string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for range 6 {
		p := filepath.Join(dir, name)
		if b, err := os.ReadFile(p); err == nil {
			return string(b)
		}
		dir = filepath.Dir(dir)
	}
	t.Fatalf("could not find %s", name)
	return ""
}

// The tournament images, by repository name.
var bundled = []string{"courthive-server", "courthive-hiveid", "courthive-tmx", "courthive-public"}

// composeDefaults returns, for each image in deploy/docker-compose.yml, the
// tag it runs when .env sets no version. Every mention of an image (the
// services, and COURTHIVE_SERVER_IMAGE given to the app and the pre-upgrade
// step) must use the same default.
func composeDefaults(t *testing.T, compose string) map[string]string {
	t.Helper()
	re := regexp.MustCompile(`ghcr\.io/jameshartt/([a-z-]+):\$\{([A-Z_]+):-([^}]+)\}`)
	out := map[string]string{}
	for _, m := range re.FindAllStringSubmatch(compose, -1) {
		image, tag := m[1], m[3]
		if prev, ok := out[image]; ok && prev != tag {
			t.Errorf("deploy/docker-compose.yml names %s with two default tags: %s and %s", image, prev, tag)
		}
		out[image] = tag
	}
	return out
}

type section struct {
	heading string // "Unreleased" or "X.Y.Z"
	body    string
}

// changelogSections splits CHANGELOG.md at its "## [...]" headings, newest
// first.
func changelogSections(changelog string) []section {
	re := regexp.MustCompile(`(?m)^## \[([^\]]+)\].*$`)
	idx := re.FindAllStringSubmatchIndex(changelog, -1)
	var out []section
	for i, m := range idx {
		end := len(changelog)
		if i+1 < len(idx) {
			end = idx[i+1][0]
		}
		out = append(out, section{heading: changelog[m[2]:m[3]], body: changelog[m[1]:end]})
	}
	return out
}

// tableImages reads the "| Part | Image |" table of a section: image name to
// tag. ok is false when the section has none.
func tableImages(body string) (map[string]string, bool) {
	if !strings.Contains(body, "| Part | Image |") {
		return nil, false
	}
	re := regexp.MustCompile("(?m)^\\|[^|]+\\| `ghcr\\.io/jameshartt/([a-z-]+):([^`]+)` \\|\\s*$")
	out := map[string]string{}
	for _, m := range re.FindAllStringSubmatch(body, -1) {
		out[m[1]] = m[2]
	}
	return out, true
}

// The newest table of the bundled set in CHANGELOG.md names exactly the
// tags deploy/docker-compose.yml runs by default; every release after 0.1.0
// (the last without the bundle) has a table with the app's own version in it.
func TestCompatibilityTableMatchesCompose(t *testing.T) {
	defaults := composeDefaults(t, repoFile(t, "deploy/docker-compose.yml"))
	for _, image := range append([]string{"jim-dot-tennis"}, bundled...) {
		if defaults[image] == "" {
			t.Fatalf("deploy/docker-compose.yml has no default tag for %s", image)
		}
	}

	sections := changelogSections(repoFile(t, "CHANGELOG.md"))
	checked := false
	for _, s := range sections {
		table, ok := tableImages(s.body)
		if s.heading != "Unreleased" && s.heading != "0.1.0" {
			if !ok {
				t.Errorf("CHANGELOG.md [%s] has no table of the bundled set (RELEASING.md: every release lists it)", s.heading)
			} else if table["jim-dot-tennis"] != s.heading {
				t.Errorf("CHANGELOG.md [%s]: the table's jim-dot-tennis row says %q, want %s", s.heading, table["jim-dot-tennis"], s.heading)
			}
		}
		if !ok || checked {
			continue
		}
		checked = true
		var diff []string
		for _, image := range bundled {
			if table[image] != defaults[image] {
				diff = append(diff, fmt.Sprintf("%s: changelog %q, compose %q", image, table[image], defaults[image]))
			}
		}
		for image := range table {
			if image != "jim-dot-tennis" && !contains(bundled, image) {
				diff = append(diff, "unexpected image in the table: "+image)
			}
		}
		sort.Strings(diff)
		if len(diff) > 0 {
			t.Errorf("CHANGELOG.md [%s]'s table of the bundled set does not match deploy/docker-compose.yml:\n  %s",
				s.heading, strings.Join(diff, "\n  "))
		}
	}
	if !checked {
		t.Error("CHANGELOG.md has no table of the bundled set")
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// The test's own parsing, on a small example.
func TestTableParsing(t *testing.T) {
	body := "Bundled set:\n\n| Part | Image |\n|---|---|\n| App | `ghcr.io/jameshartt/jim-dot-tennis:0.2.0` |\n| Tournament server | `ghcr.io/jameshartt/courthive-server:4.19.1-jdt.1` |\n"
	got, ok := tableImages(body)
	if !ok || got["jim-dot-tennis"] != "0.2.0" || got["courthive-server"] != "4.19.1-jdt.1" || len(got) != 2 {
		t.Errorf("tableImages: %v %v", got, ok)
	}
	if _, ok := tableImages("no table"); ok {
		t.Error("found a table in plain text")
	}
	s := changelogSections("# C\n## [Unreleased]\nu\n## [1.0.0] - 2027-01-01\nr\n")
	if len(s) != 2 || s[0].heading != "Unreleased" || s[1].heading != "1.0.0" || strings.TrimSpace(s[1].body) != "r" {
		t.Errorf("sections: %+v", s)
	}
}
