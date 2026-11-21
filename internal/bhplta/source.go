// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

// Package bhplta builds a season from the BHPLTA website: the league, its
// divisions, every club and team, the weeks and the fixtures, plus each
// club's venue details from its public club page.
//
// It works in two halves so a slow or failing website never holds a database
// transaction open: Source.Fetch reads everything from the site into a
// Snapshot (no database access), then Import writes a Snapshot inside a
// transaction the caller owns. Bootstrapper.Run does both and commits.
//
// Only public pages are read (the fixtures page and the club pages); nothing
// behind the club password. The site is BHPLTA_BASE_URL when set (tests point
// it at saved pages), else https://www.bhplta.co.uk.
package bhplta

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"jim-dot-tennis/internal/services"
)

// Source reads a season's fixtures and the club pages from the BHPLTA site.
type Source struct {
	// BaseURL is the site root, without a trailing slash.
	BaseURL string
	// Client makes the requests; its timeout bounds each page.
	Client *http.Client
	// Delay is the pause between page requests, to be polite to the site.
	Delay time.Duration
}

// NewSource returns a Source for services.BHPLTABaseURL().
func NewSource() *Source {
	return &Source{
		BaseURL: services.BHPLTABaseURL(),
		Client:  &http.Client{Timeout: 20 * time.Second},
		Delay:   300 * time.Millisecond,
	}
}

// Snapshot is everything a season import needs, read from the site.
type Snapshot struct {
	// Divisions as listed on the fixtures page (empty when NoFixtures).
	Divisions []services.ScrapedDivision
	// NoFixtures: the fixtures page says there are no upcoming fixtures.
	NoFixtures bool
	// Clubs found on the site: the known BHPLTA clubs plus any club a
	// fixture links to.
	Clubs []*ClubPage
	// Warnings are pages that could not be used; the import goes ahead
	// without them and reports them.
	Warnings []string
}

// ClubPage is one club as seen on the site.
type ClubPage struct {
	// Slugs are every club page slug that leads to this club (the known
	// slug, the slug after redirects, and slugs fixtures link to).
	Slugs []string
	// Name is the name to give the club if it is new to this instance.
	Name string
	// Info is the parsed club page, or nil if it could not be read.
	Info *services.ScrapedClubInfo
}

func (c *ClubPage) hasSlug(slug string) bool {
	for _, s := range c.Slugs {
		if s == slug {
			return true
		}
	}
	return false
}

// Progress reports how far a fetch has got: step of total pages, and what is
// being read now. It may be nil.
type Progress func(step, total int, what string)

// FetchError is a page the bootstrap could not do without.
type FetchError struct {
	What string // e.g. "the BHPLTA fixtures page"
	URL  string
	Err  error
}

func (e *FetchError) Error() string { return fmt.Sprintf("%s (%s): %v", e.What, e.URL, e.Err) }
func (e *FetchError) Unwrap() error { return e.Err }

// Fetch reads the fixtures page, then the page of every known BHPLTA club and
// of any other club a fixture links to.
//
// It fails (with a *FetchError) if the fixtures page cannot be read or does
// not look as expected, or if the site stops answering part-way. A single
// club page that is missing or unreadable is only a warning: that club is
// imported without venue details.
func (s *Source) Fetch(ctx context.Context, progress Progress) (*Snapshot, error) {
	if progress == nil {
		progress = func(int, int, string) {}
	}
	snap := &Snapshot{}
	total := 1 + len(services.KnownClubSlugs)

	fixturesURL := services.BHPLTAFixturesURL(s.BaseURL)
	progress(1, total, "the fixtures page")
	divisions, err := services.FetchFixtures(ctx, s.Client, fixturesURL)
	switch {
	case errors.Is(err, services.ErrNoFixturesPublished):
		snap.NoFixtures = true
	case err != nil:
		return nil, &FetchError{What: "the BHPLTA fixtures page", URL: fixturesURL, Err: err}
	default:
		snap.Divisions = divisions
	}

	// Clubs: the known list first, then anything the fixtures link to.
	type pending struct {
		slug, name string
	}
	var queue []pending
	for _, k := range services.KnownClubSlugs {
		queue = append(queue, pending{slug: k.Slug, name: k.DisplayName})
	}
	seen := map[string]bool{}
	for _, k := range services.KnownClubSlugs {
		seen[k.Slug] = true
	}
	for _, d := range snap.Divisions {
		for _, f := range d.Fixtures {
			if f.HomeClubSlug != "" && !seen[f.HomeClubSlug] {
				seen[f.HomeClubSlug] = true
				queue = append(queue, pending{slug: f.HomeClubSlug})
			}
		}
	}
	total = 1 + len(queue)

	for i, p := range queue {
		// A slug may already be known as another club's canonical slug.
		if c := snap.club(p.slug); c != nil {
			continue
		}
		if err := s.pause(ctx); err != nil {
			return nil, &FetchError{What: "the BHPLTA club pages", URL: s.BaseURL, Err: err}
		}
		label := p.name
		if label == "" {
			label = p.slug
		}
		progress(i+2, total, label)
		info, err := services.FetchClubPage(ctx, s.Client, s.BaseURL, p.slug)
		if err != nil {
			var status *services.HTTPStatusError
			if errors.As(err, &status) || errors.Is(err, services.ErrUnexpectedMarkup) {
				snap.Warnings = append(snap.Warnings, fmt.Sprintf("Could not read the BHPLTA page for %s (%v); it was added without venue details.", label, err))
				if p.name != "" {
					snap.addClub(&ClubPage{Slugs: []string{p.slug}, Name: p.name})
				}
				continue
			}
			return nil, &FetchError{What: "the BHPLTA club page for " + label, URL: services.BHPLTAClubURL(s.BaseURL, p.slug), Err: err}
		}
		name := p.name
		if name == "" {
			name = info.Name
		}
		snap.addClub(&ClubPage{Slugs: uniq(p.slug, info.CanonicalSlug), Name: name, Info: info})
	}
	return snap, nil
}

// pause waits Delay (or until ctx is done).
func (s *Source) pause(ctx context.Context) error {
	if s.Delay <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(s.Delay)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// club returns the club reached by slug, if fetched already.
func (snap *Snapshot) club(slug string) *ClubPage {
	for _, c := range snap.Clubs {
		if c.hasSlug(slug) {
			return c
		}
	}
	return nil
}

// addClub adds c, merging it into an existing club with the same canonical
// slug or the same name (St Ann's is reachable as both "st-anns" and
// "st-anns-well-gardens", for example).
func (snap *Snapshot) addClub(c *ClubPage) {
	for _, existing := range snap.Clubs {
		same := false
		for _, s := range c.Slugs {
			if existing.hasSlug(s) {
				same = true
			}
		}
		if !same && clubKey(existing.Name) == clubKey(c.Name) {
			same = true
		}
		if !same && c.Info != nil && existing.Info != nil && clubKey(existing.Info.Name) == clubKey(c.Info.Name) {
			same = true
		}
		if same {
			existing.Slugs = uniq(append(existing.Slugs, c.Slugs...)...)
			if existing.Info == nil {
				existing.Info = c.Info
			}
			return
		}
	}
	snap.Clubs = append(snap.Clubs, c)
}

func uniq(values ...string) []string {
	var out []string
	seen := map[string]bool{}
	for _, v := range values {
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}
