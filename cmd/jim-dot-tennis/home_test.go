// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package main

import (
	"bytes"
	"html/template"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"jim-dot-tennis/internal/config"
)

func TestHomePageVariants(t *testing.T) {
	funcs := template.FuncMap{"currentYear": func() int { return time.Now().Year() }}
	tmpl, err := loadTemplatesWithFuncs(filepath.Join("..", "..", "templates"), funcs)
	if err != nil {
		t.Fatal(err)
	}

	var club bytes.Buffer
	b := config.Branding{Name: "Alpha TC", LogoPath: "/branding/logo.png", PrimaryColour: "#112233", SecondaryColour: "#445566"}
	if err := tmpl.ExecuteTemplate(&club, "club_home.html", map[string]interface{}{"Branding": b}); err != nil {
		t.Fatal(err)
	}
	page := club.String()
	for _, want := range []string{"Alpha TC", "/branding/logo.png", `href="/standings"`, `href="/login"`, "#112233"} {
		if !strings.Contains(page, want) {
			t.Errorf("club home lacks %q", want)
		}
	}
	for _, bad := range []string{"/tournaments", "/public", "Tournament"} {
		if strings.Contains(page, bad) {
			t.Errorf("club home contains %q", bad)
		}
	}

	var tour bytes.Buffer
	if err := tmpl.ExecuteTemplate(&tour, "index.html", map[string]interface{}{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(tour.String(), `href="/tournaments"`) {
		t.Error("the tournament home page lost its tournament link")
	}
}
