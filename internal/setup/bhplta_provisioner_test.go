// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package setup

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/bhplta"
	"jim-dot-tennis/internal/repository"
)

// The club step with the real provisioner, against the saved BHPLTA pages:
// a failed fetch changes nothing and can be retried from the page.
func TestClubStepImportsFromBHPLTAAndRetries(t *testing.T) {
	ctx := context.Background()
	st, db := newTestState(t)

	var down atomic.Bool
	down.Store(true)
	files := http.FileServer(http.Dir(filepath.Join(findRepoDir(t, "tests"), "e2e", "fixtures", "bhplta")))
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if down.Load() {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		files.ServeHTTP(w, r)
	}))
	t.Cleanup(site.Close)
	b := &bhplta.Bootstrapper{DB: db, Source: &bhplta.Source{BaseURL: site.URL, Client: &http.Client{Timeout: 5 * time.Second}}}
	s := NewService(st, NewBHPLTAProvisioner(b))

	// An administrator signed in on the club step.
	hash, _ := auth.HashPassword(goodPassword)
	if _, err := db.Exec(`INSERT INTO users (username, password_hash, role) VALUES ('first.admin', ?, 'admin')`, hash); err != nil {
		t.Fatal(err)
	}
	browser := newFlow(t, s, db)
	if code, _, _ := browser.do("POST", "/login", url.Values{"username": {"first.admin"}, "password": {goodPassword}}); code != http.StatusSeeOther {
		t.Fatalf("sign in: %d", code)
	}

	// BHPLTA is down: the page reports it, nothing is written.
	if code, loc, _ := browser.do("POST", ClubPath, url.Values{"club": {"st-anns"}}); code != http.StatusSeeOther || loc != ClubPath+"?working=1" {
		t.Fatalf("choose club: %d %q", code, loc)
	}
	job := waitForClubJob(t, s)
	var pe *ProvisionError
	if !errors.As(job.Err, &pe) || !strings.Contains(pe.Message, "fixtures page") {
		t.Fatalf("job error: %v", job.Err)
	}
	code, _, body := browser.do("GET", ClubPath+"?working=1", nil)
	if code != http.StatusOK || !strings.Contains(body, "HTTP status 503") || !strings.Contains(body, "Try again") || !strings.Contains(body, "Nothing was changed") {
		t.Fatalf("club page after failure: %d\n%s", code, body)
	}
	for _, table := range []string{"leagues", "clubs", "seasons", "teams", "fixtures"} {
		var n int
		if err := db.Get(&n, `SELECT COUNT(*) FROM `+table); err != nil || n != 0 {
			t.Fatalf("%s: %d rows after a failed import (err %v)", table, n, err)
		}
	}
	if st.Status(ctx) != StatusNeedsClub {
		t.Fatalf("status after failure: %v", st.Status(ctx))
	}

	// Retry once BHPLTA is back.
	down.Store(false)
	if code, _, _ := browser.do("POST", ClubPath, url.Values{"club": {"st-anns"}}); code != http.StatusSeeOther {
		t.Fatalf("retry: %d", code)
	}
	job = waitForClubJob(t, s)
	if job.Err != nil || job.Result == nil || job.Result.Import == nil {
		t.Fatalf("retry job: %+v", job)
	}
	if code, loc, _ := browser.do("GET", ClubPath+"?working=1", nil); code != http.StatusSeeOther || loc != DonePath {
		t.Fatalf("waiting page after success: %d %q", code, loc)
	}
	code, _, body = browser.do("GET", DonePath, nil)
	if code != http.StatusOK || !strings.Contains(body, `data-count="fixtures">228 fixtures`) || !strings.Contains(body, "41 teams, 4 of them yours") {
		t.Fatalf("done page: %d\n%s", code, body)
	}

	home := st.HomeClub()
	if home.ID() != job.Result.ClubID || home.Club().Name != "St Ann's Tennis" {
		t.Fatalf("home club %d %+v", home.ID(), home.Club())
	}
	// The known BHPLTA identifiers were stored and are live.
	if v, ok, err := repository.NewSettingsRepository(db).GetString(ctx, repository.SettingBHPLTAClubID); err != nil || !ok || v != "10" {
		t.Fatalf("stored BHPLTA club id %q %v %v", v, ok, err)
	}
	if got := home.ClubSettings().BHPLTAClubID(home.Club()); got != 10 {
		t.Fatalf("live BHPLTA club id %d", got)
	}
}
