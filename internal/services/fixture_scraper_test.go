// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package services

import (
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const sampleFixturesHTML = `
<html>
<body>
<div class="tab-pane active" id="1">
  <h2 class="bhplta_fixtures_heading">Division 1</h2>
  <div class="bhplta_fixtures-wrapper">
    <table class="bhplta_fixtures_table">
      <caption>Week 1</caption>
      <thead><tr><th colspan="3">16 Apr 2026</th></tr></thead>
      <tbody>
        <tr>
          <td class="bhplta_fixtures_home_team"><a href="/team/1">Dyke</a></td>
          <td class="bhplta_table_body_vs"> v </td>
          <td class="bhplta_fixtures_away_team">St Ann&#039;s A</td>
        </tr>
        <tr>
          <td class="bhplta_fixtures_home_team">Hove A</td>
          <td class="bhplta_table_body_vs"> v </td>
          <td class="bhplta_fixtures_away_team"><a href="/team/2">Preston Park A</a></td>
        </tr>
      </tbody>
    </table>
    <table class="bhplta_fixtures_table">
      <caption>Week 2</caption>
      <thead><tr><th colspan="3">23 Apr 2026</th></tr></thead>
      <tbody>
        <tr>
          <td class="bhplta_fixtures_home_team">St Ann&#039;s A</td>
          <td class="bhplta_table_body_vs"> v </td>
          <td class="bhplta_fixtures_away_team">Hove A</td>
        </tr>
      </tbody>
    </table>
  </div>
</div>
<div class="tab-pane" id="2">
  <h2 class="bhplta_fixtures_heading">Division 2</h2>
  <div class="bhplta_fixtures-wrapper">
    <table class="bhplta_fixtures_table">
      <caption>Week 1</caption>
      <thead><tr><th colspan="3">14 Apr 2026</th></tr></thead>
      <tbody>
        <tr>
          <td class="bhplta_fixtures_home_team">Saltdean A</td>
          <td class="bhplta_table_body_vs"> v </td>
          <td class="bhplta_fixtures_away_team">Hove B</td>
        </tr>
      </tbody>
    </table>
  </div>
</div>
</body>
</html>
`

func TestParseFixturesHTML(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(sampleFixturesHTML))
	if err != nil {
		t.Fatalf("failed to create goquery document: %v", err)
	}

	divisions, err := ParseFixturesHTML(doc)
	if err != nil {
		t.Fatalf("ParseFixturesHTML returned error: %v", err)
	}

	if len(divisions) != 2 {
		t.Fatalf("expected 2 divisions, got %d", len(divisions))
	}

	// Check Division 1
	div1 := divisions[0]
	if div1.Name != "Division 1" {
		t.Errorf("expected division name 'Division 1', got %q", div1.Name)
	}
	if len(div1.Fixtures) != 3 {
		t.Fatalf("expected 3 fixtures in Division 1, got %d", len(div1.Fixtures))
	}
	if len(div1.Teams) != 4 {
		t.Errorf("expected 4 unique teams in Division 1, got %d", len(div1.Teams))
	}

	// Check first fixture
	f := div1.Fixtures[0]
	if f.Week != 1 {
		t.Errorf("expected week 1, got %d", f.Week)
	}
	if f.HomeTeam != "Dyke" {
		t.Errorf("expected home team 'Dyke', got %q", f.HomeTeam)
	}
	if f.AwayTeam != "St Ann's A" {
		t.Errorf("expected away team 'St Ann's A', got %q", f.AwayTeam)
	}
	expectedDate := time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC)
	if !f.Date.Equal(expectedDate) {
		t.Errorf("expected date %v, got %v", expectedDate, f.Date)
	}

	// Check second fixture - away team inside <a> tag
	f2 := div1.Fixtures[1]
	if f2.AwayTeam != "Preston Park A" {
		t.Errorf("expected away team 'Preston Park A', got %q", f2.AwayTeam)
	}

	// Check week 2 fixture
	f3 := div1.Fixtures[2]
	if f3.Week != 2 {
		t.Errorf("expected week 2, got %d", f3.Week)
	}

	// Check Division 2
	div2 := divisions[1]
	if div2.Name != "Division 2" {
		t.Errorf("expected division name 'Division 2', got %q", div2.Name)
	}
	if len(div2.Fixtures) != 1 {
		t.Errorf("expected 1 fixture in Division 2, got %d", len(div2.Fixtures))
	}
}

func TestParseWeekNumber(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"Week 1", 1},
		{"Week 18", 18},
		{"  Week 5  ", 5},
		{"", 0},
		{"something else", 0},
	}

	for _, tt := range tests {
		result := parseWeekNumber(tt.input)
		if result != tt.expected {
			t.Errorf("parseWeekNumber(%q) = %d, want %d", tt.input, result, tt.expected)
		}
	}
}

func TestParseFixtureDateBHPLTA(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Time
	}{
		{"16 Apr 2026", time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC)},
		{"7 Apr 2026", time.Date(2026, 4, 7, 0, 0, 0, 0, time.UTC)},
		{"", time.Time{}},
		{"invalid", time.Time{}},
	}

	for _, tt := range tests {
		result := parseFixtureDateBHPLTA(tt.input)
		if !result.Equal(tt.expected) {
			t.Errorf("parseFixtureDateBHPLTA(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestParseFixturesHTML_Empty(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader("<html><body></body></html>"))
	_, err := ParseFixturesHTML(doc)
	if err == nil {
		t.Error("expected error for empty page, got nil")
	}
}
