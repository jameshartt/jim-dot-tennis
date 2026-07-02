// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package services

import (
	"reflect"
	"testing"
)

func TestParseTeamNames(t *testing.T) {
	p := NewMatchCardParser()
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"simple", "Hove A v Dyke B", []string{"Hove A", "Dyke B"}},
		{"nbsp and extra spaces", "St Ann's A   v   Hove B", []string{"St Ann's A", "Hove B"}},
		{"no separator returns empty", "Hove A vs Dyke B", []string{}},
		{"three parts returns empty", "A v B v C", []string{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := p.parseTeamNames(c.in); !reflect.DeepEqual(got, c.want) {
				t.Errorf("parseTeamNames(%q) = %#v, want %#v", c.in, got, c.want)
			}
		})
	}
}

func TestParseDivisionAndWeek(t *testing.T) {
	p := NewMatchCardParser()
	cases := []struct {
		name     string
		in       string
		wantDiv  string
		wantWeek int
	}{
		{"standard", "Division 1  |  Week 1", "Division 1", 1},
		{"two digit week", "Division 3 | Week 12", "Division 3", 12},
		{"missing week", "Division 2", "Division 2", 0},
		{"missing division", "Week 5", "", 5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			div, week := p.parseDivisionAndWeek(c.in)
			if div != c.wantDiv || week != c.wantWeek {
				t.Errorf("parseDivisionAndWeek(%q) = (%q, %d), want (%q, %d)", c.in, div, week, c.wantDiv, c.wantWeek)
			}
		})
	}
}

func TestParseDates(t *testing.T) {
	p := NewMatchCardParser()

	event, played := p.parseDates("Event date: 17 Apr 2025    |    Date played: 18 Apr 2025")
	if event.Format("2006-01-02") != "2025-04-17" {
		t.Errorf("event date = %s, want 2025-04-17", event.Format("2006-01-02"))
	}
	if played.Format("2006-01-02") != "2025-04-18" {
		t.Errorf("played date = %s, want 2025-04-18", played.Format("2006-01-02"))
	}

	// Unparseable text yields zero times rather than panicking.
	z1, z2 := p.parseDates("no dates here")
	if !z1.IsZero() || !z2.IsZero() {
		t.Errorf("expected zero times for junk input, got %v / %v", z1, z2)
	}
}

func TestParsePlayerNamesFromText(t *testing.T) {
	p := NewMatchCardParser()
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", []string{}},
		{"concession marker", "Conceded by Hove A", []string{}},
		{"given-to marker", "Given to St Ann's", []string{}},
		// The real-world BHPLTA case: two names run together with no space.
		{"concatenated pair", "John SmithJane Doe", []string{"John Smith", "Jane Doe"}},
		{"four spaced words", "John Smith Jane Doe", []string{"John Smith", "Jane Doe"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := p.parsePlayerNamesFromText(c.in); !reflect.DeepEqual(got, c.want) {
				t.Errorf("parsePlayerNamesFromText(%q) = %#v, want %#v", c.in, got, c.want)
			}
		})
	}
}
