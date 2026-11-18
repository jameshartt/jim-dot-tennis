// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package fixturegen

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func parseFixture(t *testing.T) *Tables {
	t.Helper()
	f, err := os.Open("testdata/league_table_2026.html")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()
	tables, err := ParseTables(f)
	if err != nil {
		t.Fatalf("ParseTables: %v", err)
	}
	return tables
}

func TestParseTablesReadsEveryDivision(t *testing.T) {
	tables := parseFixture(t)

	if tables.Season != 2026 {
		t.Errorf("season: got %d, want 2026", tables.Season)
	}
	wantSizes := []int{10, 10, 10, 12}
	if len(tables.Divisions) != len(wantSizes) {
		t.Fatalf("divisions: got %d, want %d", len(tables.Divisions), len(wantSizes))
	}
	wantDays := []string{"Thursday", "Wednesday", "Tuesday", "Tuesday"}
	for i, d := range tables.Divisions {
		if len(d.Teams) != wantSizes[i] {
			t.Errorf("%s: got %d teams, want %d", d.Name, len(d.Teams), wantSizes[i])
		}
		if d.Level != i+1 || d.PlayDay != wantDays[i] {
			t.Errorf("%s: level %d on %s, want level %d on %s", d.Name, d.Level, d.PlayDay, i+1, wantDays[i])
		}
	}

	top := tables.Divisions[0].Teams[0]
	if top.Name != "Queens" || top.Played != 18 || top.EventsWon != 68 || top.SetsWon != 138 || top.SetsLost != 11 || top.Points != 136 {
		t.Errorf("Division 1 leader parsed wrongly: %+v", top)
	}
	// Half events survive as decimals.
	if hove := tables.Divisions[0].Teams[1]; hove.EventsWon != 63.5 || hove.EventsLost != 8.5 {
		t.Errorf("half events lost: %+v", hove)
	}
	// Apostrophes are normalised so names match across sources.
	if got := tables.Divisions[0].Teams[4].Name; got != "St Ann's" {
		t.Errorf("apostrophe: got %q, want %q", got, "St Ann's")
	}
	// A penalty footnote marker is not part of the points.
	last := tables.Divisions[3].Teams[11]
	if last.Name != "Hove F" || last.Points != 54 || !last.Penalty {
		t.Errorf("penalty row parsed wrongly: %+v", last)
	}
}

func TestParseTablesRejectsPageWithoutTables(t *testing.T) {
	if _, err := ParseTables(strings.NewReader("<html><body><p>Maintenance</p></body></html>")); err == nil {
		t.Fatal("expected an error for a page with no league tables")
	}
}

func TestTablesEndpointCachesAndSurvivesUpstreamFailure(t *testing.T) {
	page, err := os.ReadFile("testdata/league_table_2026.html")
	if err != nil {
		t.Fatal(err)
	}
	hits, failing := 0, false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if failing {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write(page)
	}))
	defer upstream.Close()

	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	h := &Handler{
		templates: template.New(""),
		client:    upstream.Client(),
		tablesURL: upstream.URL,
		now:       func() time.Time { return now },
	}

	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.handleTables(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec
	}

	if rec := get("/fixture-generator/api/tables"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"Queens"`) {
		t.Fatalf("first request: status %d body %.80s", rec.Code, rec.Body.String())
	}
	get("/fixture-generator/api/tables")
	get("/fixture-generator/api/tables?refresh=1") // too soon after the last fetch to go upstream again
	if hits != 1 {
		t.Errorf("upstream hits within the cache window: got %d, want 1", hits)
	}

	now = now.Add(2 * time.Minute)
	get("/fixture-generator/api/tables?refresh=1")
	if hits != 2 {
		t.Errorf("refresh after the floor: got %d upstream hits, want 2", hits)
	}

	// The league site goes down after the cache expires: keep serving what we have.
	failing = true
	now = now.Add(time.Hour)
	if rec := get("/fixture-generator/api/tables"); rec.Code != http.StatusOK {
		t.Errorf("stale copy should be served when upstream fails, got %d", rec.Code)
	}

	// With nothing cached, the failure is reported so the page can fall back.
	cold := &Handler{templates: template.New(""), client: upstream.Client(), tablesURL: upstream.URL, now: time.Now}
	rec := httptest.NewRecorder()
	cold.handleTables(rec, httptest.NewRequest(http.MethodGet, "/fixture-generator/api/tables", nil))
	if rec.Code != http.StatusBadGateway {
		t.Errorf("cold failure: got %d, want %d", rec.Code, http.StatusBadGateway)
	}
}

func TestFetchTablesStampsSource(t *testing.T) {
	page, _ := os.ReadFile("testdata/league_table_2026.html")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(page) }))
	defer upstream.Close()

	tables, err := FetchTables(context.Background(), upstream.Client(), upstream.URL)
	if err != nil {
		t.Fatalf("FetchTables: %v", err)
	}
	if tables.Source != upstream.URL || tables.FetchedAt.IsZero() {
		t.Errorf("source/fetchedAt not set: %q %v", tables.Source, tables.FetchedAt)
	}
}
