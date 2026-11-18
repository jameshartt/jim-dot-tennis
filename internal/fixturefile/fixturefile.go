// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

// Package fixturefile reads the shareable season fixture file written by the
// fixture generator (static/fixture-generator/engine.js). One file carries a
// whole season, so a league organiser can generate the fixtures once and every
// club can load the same file into its own jim.tennis.
package fixturefile

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"

	"jim-dot-tennis/internal/normalize"
)

const (
	// Format identifies a fixture file.
	Format = "jim.tennis/fixtures"
	// Version is the newest file version this build reads.
	Version = 1
	// MaxSize bounds how much of an upload is read.
	MaxSize = 5 << 20

	dateLayout = "2006-01-02"
)

// File is a season of fixtures for every division of a league.
type File struct {
	Format      string     `json:"format"`
	Version     int        `json:"version"`
	League      League     `json:"league"`
	Season      Season     `json:"season"`
	Status      string     `json:"status"`
	AcceptedAt  string     `json:"acceptedAt,omitempty"`
	GeneratedAt string     `json:"generatedAt,omitempty"`
	Divisions   []Division `json:"divisions"`
	Decisions   []string   `json:"decisions,omitempty"`
	Checksum    string     `json:"checksum,omitempty"`
}

// League names the competition the file belongs to.
type League struct {
	Name      string `json:"name"`
	ShortName string `json:"shortName,omitempty"`
}

// Season describes the season the fixtures are for.
type Season struct {
	Year            int    `json:"year"`
	Weeks           int    `json:"weeks"`
	Week1Commencing string `json:"week1Commencing,omitempty"`
}

// Division is one division with its teams and fixtures.
type Division struct {
	Name     string    `json:"name"`
	Level    int       `json:"level"`
	PlayDay  string    `json:"playDay"`
	Teams    []Team    `json:"teams"`
	Fixtures []Fixture `json:"fixtures"`
}

// Team is a team entered in a division.
type Team struct {
	Name string `json:"name"`
	Club string `json:"club"`
}

// Fixture is one match. Extra marks the second round of a double-header week.
type Fixture struct {
	Week  int    `json:"week"`
	Date  string `json:"date"`
	Home  string `json:"home"`
	Away  string `json:"away"`
	Extra bool   `json:"extra,omitempty"`
}

// Time returns the fixture date. Validate guarantees it parses.
func (f Fixture) Time() time.Time {
	t, _ := time.Parse(dateLayout, f.Date)
	return t
}

// FixtureCount is the number of fixtures across all divisions.
func (f *File) FixtureCount() int {
	n := 0
	for _, d := range f.Divisions {
		n += len(d.Fixtures)
	}
	return n
}

// Intact reports whether the fixtures still match the checksum the generator
// wrote. A file without a checksum counts as intact. This catches accidental
// edits; it is not a signature.
func (f *File) Intact() bool {
	return f.Checksum == "" || f.Checksum == Checksum(f.Divisions)
}

var spaces = regexp.MustCompile(`\s+`)

// CleanName normalises apostrophes and whitespace the same way the generator
// does, so "St Ann’s" and "St Ann's" are one team.
func CleanName(name string) string {
	return strings.TrimSpace(spaces.ReplaceAllString(normalize.Apostrophes(name), " "))
}

// ClubOf derives a club from a team name: "Dyke A" is Dyke, "Park Avenue" is
// Park Avenue.
func ClubOf(team string) string {
	name := CleanName(team)
	if i := strings.LastIndexByte(name, ' '); i > 0 && i == len(name)-2 {
		if c := name[len(name)-1]; c >= 'A' && c <= 'Z' {
			return strings.TrimSpace(name[:i])
		}
	}
	return name
}

// Checksum is FNV-1a (32 bit) over the sorted canonical fixture lines, as
// computed by checksum() in the generator.
func Checksum(divisions []Division) string {
	var lines []string
	for _, d := range divisions {
		for _, f := range d.Fixtures {
			lines = append(lines, fmt.Sprintf("%s|%d|%s|%s|%s",
				CleanName(d.Name), f.Week, f.Date, CleanName(f.Home), CleanName(f.Away)))
		}
	}
	// The generator sorts by UTF-16 code unit; team names are ASCII once
	// apostrophes are normalised, where that and byte order agree.
	sort.Strings(lines)
	h := fnv.New32a()
	_, _ = h.Write([]byte(strings.Join(lines, "\n")))
	return fmt.Sprintf("%08x", h.Sum32())
}

// Parse reads and validates a fixture file. Errors are worded for the person
// who uploaded the file.
func Parse(r io.Reader) (*File, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxSize+1))
	if err != nil {
		return nil, fmt.Errorf("could not read the file: %w", err)
	}
	if len(data) > MaxSize {
		return nil, errors.New("the file is too large to be a fixture file")
	}
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, errors.New("this is not a jim.tennis fixture file (it is not valid JSON)")
	}
	if err := f.Validate(); err != nil {
		return nil, err
	}
	return &f, nil
}

// Validate normalises names and checks that the file describes a sound
// season: every pair of teams in a division meets exactly once at each venue,
// nobody plays twice on one day, and weeks and dates are in range.
func (f *File) Validate() error {
	if f.Format != Format {
		return errors.New("this is not a jim.tennis fixture file")
	}
	if f.Version < 1 || f.Version > Version {
		return fmt.Errorf("this fixture file is version %d; this copy of jim.tennis reads up to version %d", f.Version, Version)
	}
	if f.Season.Year < 1900 || f.Season.Year > 3000 {
		return errors.New("the fixture file has no season year")
	}
	if f.Season.Weeks == 0 {
		f.Season.Weeks = 18
	}
	if f.Season.Weeks < 2 || f.Season.Weeks > 52 {
		return fmt.Errorf("the fixture file has %d weeks in the season", f.Season.Weeks)
	}
	if len(f.Divisions) == 0 {
		return errors.New("the fixture file has no divisions")
	}
	if len(f.Divisions) > 20 {
		return errors.New("the fixture file has too many divisions")
	}

	f.League.Name = CleanName(f.League.Name)
	seenDivision := map[string]bool{}
	seenTeam := map[string]string{}
	for i := range f.Divisions {
		d := &f.Divisions[i]
		d.Name = CleanName(d.Name)
		if d.Name == "" {
			return fmt.Errorf("division %d in the fixture file has no name", i+1)
		}
		if seenDivision[d.Name] {
			return fmt.Errorf("%s appears twice in the fixture file", d.Name)
		}
		seenDivision[d.Name] = true
		if d.Level == 0 {
			d.Level = i + 1
		}
		if len(d.Teams) < 2 {
			return fmt.Errorf("%s needs at least two teams", d.Name)
		}
		if len(d.Teams) > 40 {
			return fmt.Errorf("%s has too many teams", d.Name)
		}

		inDivision := map[string]bool{}
		for j := range d.Teams {
			t := &d.Teams[j]
			t.Name = CleanName(t.Name)
			if t.Name == "" {
				return fmt.Errorf("%s has a team with no name", d.Name)
			}
			if len(t.Name) > 80 {
				return fmt.Errorf("%s has a team name that is too long", d.Name)
			}
			if other, dup := seenTeam[t.Name]; dup {
				return fmt.Errorf("%s is listed in both %s and %s", t.Name, other, d.Name)
			}
			seenTeam[t.Name] = d.Name
			inDivision[t.Name] = true
			if t.Club = CleanName(t.Club); t.Club == "" {
				t.Club = ClubOf(t.Name)
			}
		}

		hosted := map[[2]string]int{}
		playing := map[[2]string]bool{}
		for j := range d.Fixtures {
			fx := &d.Fixtures[j]
			fx.Home, fx.Away = CleanName(fx.Home), CleanName(fx.Away)
			if !inDivision[fx.Home] || !inDivision[fx.Away] {
				return fmt.Errorf("%s: %s v %s involves a team that is not in the division", d.Name, fx.Home, fx.Away)
			}
			if fx.Home == fx.Away {
				return fmt.Errorf("%s: %s is drawn against itself", d.Name, fx.Home)
			}
			if fx.Week < 1 || fx.Week > f.Season.Weeks {
				return fmt.Errorf("%s: %s v %s is in week %d of a %d-week season", d.Name, fx.Home, fx.Away, fx.Week, f.Season.Weeks)
			}
			if _, err := time.Parse(dateLayout, fx.Date); err != nil {
				return fmt.Errorf("%s: %s v %s has no valid date", d.Name, fx.Home, fx.Away)
			}
			hosted[[2]string{fx.Home, fx.Away}]++
			for _, team := range []string{fx.Home, fx.Away} {
				key := [2]string{team, fx.Date}
				if playing[key] {
					return fmt.Errorf("%s: %s has two matches on %s", d.Name, team, fx.Date)
				}
				playing[key] = true
			}
		}
		for _, home := range d.Teams {
			for _, away := range d.Teams {
				if home.Name == away.Name {
					continue
				}
				if n := hosted[[2]string{home.Name, away.Name}]; n != 1 {
					return fmt.Errorf("%s: %s should host %s exactly once (found %d)", d.Name, home.Name, away.Name, n)
				}
			}
		}
	}
	return nil
}
