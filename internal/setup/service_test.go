// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package setup

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/repository"
)

const goodPassword = "correct horse battery"

func newTestService(t *testing.T) (*Service, *database.DB) {
	t.Helper()
	st, db := newTestState(t)
	return NewService(st, nil), db
}

func userCount(t *testing.T, db *database.DB) int {
	t.Helper()
	var n int
	if err := db.Get(&n, `SELECT COUNT(*) FROM users`); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestSetupCodeFormat(t *testing.T) {
	pattern := regexp.MustCompile(`^[2-9A-HJ-NP-Z]{4}-[2-9A-HJ-NP-Z]{4}-[2-9A-HJ-NP-Z]{4}$`)
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		code, err := newSetupCode()
		if err != nil {
			t.Fatal(err)
		}
		if !pattern.MatchString(code) {
			t.Fatalf("code %q has the wrong shape", code)
		}
		if seen[code] {
			t.Fatalf("duplicate code %q", code)
		}
		seen[code] = true
	}
	if normalizeCode(" abcd-efgh jkmn\n") != "ABCDEFGHJKMN" {
		t.Fatalf("normalizeCode: %q", normalizeCode(" abcd-efgh jkmn\n"))
	}
}

func TestIssueCodeStoresOnlyAHash(t *testing.T) {
	ctx := context.Background()
	s, db := newTestService(t)
	code, err := s.IssueCode(ctx)
	if err != nil || code == "" {
		t.Fatalf("IssueCode: %q %v", code, err)
	}
	var values []string
	if err := db.Select(&values, `SELECT value FROM app_settings`); err != nil {
		t.Fatal(err)
	}
	for _, v := range values {
		if strings.Contains(strings.ToUpper(v), normalizeCode(code)) || strings.Contains(v, code) {
			t.Fatalf("setup code stored in clear: %q", v)
		}
	}

	// A restart issues a different code and the old one stops working.
	code2, err := s.IssueCode(ctx)
	if err != nil || code2 == "" || code2 == code {
		t.Fatalf("second IssueCode: %q %v", code2, err)
	}
	if _, err := s.RedeemCode(ctx, code, "198.51.100.1"); !errors.Is(err, ErrWrongCode) {
		t.Fatalf("old code after restart: %v, want wrong code", err)
	}
	if _, err := s.RedeemCode(ctx, strings.ToLower(code2), "198.51.100.1"); err != nil {
		t.Fatalf("new code (lower case): %v", err)
	}
}

func TestIssueCodeOnlyWhenSetupNeeded(t *testing.T) {
	ctx := context.Background()
	s, db := newTestService(t)
	if _, err := db.Exec(`INSERT INTO users (username, password_hash, role) VALUES ('existing', 'x', 'admin')`); err != nil {
		t.Fatal(err)
	}
	code, err := s.IssueCode(ctx)
	if err != nil || code != "" {
		t.Fatalf("with an administrator: code %q err %v, want none", code, err)
	}
}

func TestRedeemCodeIsSingleUse(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestService(t)
	code, _ := s.IssueCode(ctx)
	ticket, err := s.RedeemCode(ctx, code, "198.51.100.1")
	if err != nil || ticket == "" {
		t.Fatalf("redeem: %v", err)
	}
	if !s.TicketValid(ctx, ticket) {
		t.Fatal("ticket not valid after redeeming")
	}
	if s.TicketValid(ctx, ticket+"x") || s.TicketValid(ctx, "") {
		t.Fatal("wrong ticket accepted")
	}
	if _, err := s.RedeemCode(ctx, code, "198.51.100.2"); !errors.Is(err, ErrNoActiveCode) {
		t.Fatalf("second redeem: %v, want no active code", err)
	}
	if !s.CodeWasRedeemed(ctx) {
		t.Fatal("CodeWasRedeemed false after redeeming")
	}

	// A restart cancels the ticket too.
	if _, err := s.IssueCode(ctx); err != nil {
		t.Fatal(err)
	}
	if s.TicketValid(ctx, ticket) {
		t.Fatal("ticket survived a new code")
	}
}

func TestTicketExpires(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestService(t)
	code, _ := s.IssueCode(ctx)
	ticket, err := s.RedeemCode(ctx, code, "198.51.100.1")
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return time.Now().Add(ticketLifetime + time.Minute) }
	if s.TicketValid(ctx, ticket) {
		t.Fatal("ticket valid after its lifetime")
	}
	if _, err := s.CreateAdmin(ctx, ticket, "first.admin", goodPassword); !errors.Is(err, ErrTicketInvalid) {
		t.Fatalf("CreateAdmin with expired ticket: %v", err)
	}
}

func TestWrongCodesAreThrottled(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestService(t)
	code, _ := s.IssueCode(ctx)
	for i := 0; i < perClientMaxFailures; i++ {
		if _, err := s.RedeemCode(ctx, "AAAA-AAAA-AAAA", "198.51.100.7"); !errors.Is(err, ErrWrongCode) {
			t.Fatalf("guess %d: %v", i, err)
		}
	}
	// Even the right code is refused from that client while throttled.
	if _, err := s.RedeemCode(ctx, code, "198.51.100.7"); !errors.Is(err, ErrThrottled) {
		t.Fatalf("right code while throttled: %v, want throttled", err)
	}
	// Another client is unaffected...
	if _, err := s.RedeemCode(ctx, "AAAA-AAAA-AAAA", "198.51.100.8"); !errors.Is(err, ErrWrongCode) {
		t.Fatalf("other client: %v", err)
	}
	// ...and the throttle lifts after the window.
	s.now = func() time.Time { return time.Now().Add(throttleWindow + time.Second) }
	if _, err := s.RedeemCode(ctx, code, "198.51.100.7"); err != nil {
		t.Fatalf("after the window: %v", err)
	}
}

func TestGlobalThrottle(t *testing.T) {
	th := newCodeThrottle()
	now := time.Now()
	for i := 0; i < globalMaxFailures; i++ {
		client := "203.0.113." + string(rune('a'+i%26)) + string(rune('a'+i/26))
		if th.blocked(client, now) {
			t.Fatalf("blocked after %d failures spread over clients", i)
		}
		th.fail(client, now)
	}
	if !th.blocked("192.0.2.200", now) {
		t.Fatal("fresh client not blocked after the global limit")
	}
	if th.blocked("192.0.2.200", now.Add(throttleWindow+time.Second)) {
		t.Fatal("global block did not lift after the window")
	}
}

func redeemed(t *testing.T, s *Service) string {
	t.Helper()
	ctx := context.Background()
	code, err := s.IssueCode(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ticket, err := s.RedeemCode(ctx, code, "198.51.100.1")
	if err != nil {
		t.Fatal(err)
	}
	return ticket
}

func TestCreateAdmin(t *testing.T) {
	ctx := context.Background()
	s, db := newTestService(t)
	ticket := redeemed(t, s)

	// Policy failures are reported against the field and create nothing.
	for _, c := range []struct{ user, pass, field string }{
		{"a", goodPassword, "username"},
		{"has space", goodPassword, "username"},
		{"first.admin", "short", "password"},
		{"first.admin", "first.admin-2024", "password"},
		{"first.admin", "aaaaaaaaaaaa", "password"},
		{"first.admin", strings.Repeat("x", 40) + strings.Repeat("y", 40), "password"},
	} {
		_, err := s.CreateAdmin(ctx, ticket, c.user, c.pass)
		var ie *InputError
		if !errors.As(err, &ie) || ie.Field != c.field {
			t.Errorf("CreateAdmin(%q, %q): %v, want %s error", c.user, c.pass, err, c.field)
		}
	}
	if userCount(t, db) != 0 {
		t.Fatal("rejected input created a user")
	}

	if _, err := s.CreateAdmin(ctx, "not-the-ticket", "first.admin", goodPassword); !errors.Is(err, ErrTicketInvalid) {
		t.Fatalf("wrong ticket: %v", err)
	}

	id, err := s.CreateAdmin(ctx, ticket, "  first.admin ", goodPassword)
	if err != nil {
		t.Fatalf("CreateAdmin: %v", err)
	}
	var row struct {
		Username string `db:"username"`
		Role     string `db:"role"`
		Hash     string `db:"password_hash"`
	}
	if err := db.Get(&row, `SELECT username, role, password_hash FROM users WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}
	if row.Username != "first.admin" || row.Role != "admin" || !strings.HasPrefix(row.Hash, "$2a$12$") {
		t.Fatalf("created user: %+v", row)
	}
	if userCount(t, db) != 1 {
		t.Fatalf("%d users after setup, want 1", userCount(t, db))
	}
	settings := repository.NewSettingsRepository(db)
	if got, ok, _ := settings.GetUint(ctx, settingAdminUserID); !ok || int64(got) != id {
		t.Fatalf("setup_admin_user_id = %d (set %v), want %d", got, ok, id)
	}
	for _, k := range []string{settingTicketHash, settingTicketExpiresAt, settingCodeHash} {
		if _, ok, _ := settings.GetString(ctx, k); ok {
			t.Errorf("%s still set after the administrator was created", k)
		}
	}

	// From now on, code entry and administrator creation are gone.
	if s.state.Status(ctx) != StatusNeedsClub {
		t.Fatalf("status %v, want needs-club", s.state.Status(ctx))
	}
	if _, err := s.CreateAdmin(ctx, ticket, "second.admin", goodPassword); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("second CreateAdmin: %v, want unavailable", err)
	}
	if _, err := s.RedeemCode(ctx, "AAAA-AAAA-AAAA", "198.51.100.1"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("RedeemCode after admin: %v, want unavailable", err)
	}
	if code, _ := s.IssueCode(ctx); code != "" {
		t.Fatal("IssueCode issued a code with an administrator present")
	}
}

// A database with captain or player accounts but no administrator still gets
// its first administrator from the setup code; the existing accounts are kept.
func TestCreateAdminAlongsideNonAdminAccounts(t *testing.T) {
	ctx := context.Background()
	s, db := newTestService(t)
	ticket := redeemed(t, s)
	if _, err := db.Exec(`INSERT INTO users (username, password_hash, role) VALUES ('captain', 'x', 'captain')`); err != nil {
		t.Fatal(err)
	}
	// A username already in use is refused like any other input problem, and
	// nothing in the transaction is kept: the ticket still works.
	var inputErr *InputError
	if _, err := s.CreateAdmin(ctx, ticket, "Captain", goodPassword); !errors.As(err, &inputErr) || inputErr.Field != "username" {
		t.Fatalf("CreateAdmin with a taken username: %v", err)
	}
	if !s.TicketValid(ctx, ticket) {
		t.Fatal("ticket consumed by a failed CreateAdmin")
	}
	if _, err := s.CreateAdmin(ctx, ticket, "first.admin", goodPassword); err != nil {
		t.Fatalf("CreateAdmin with a captain account present: %v", err)
	}
	if userCount(t, db) != 2 {
		t.Fatalf("%d users, want the captain and the new administrator", userCount(t, db))
	}
	if s.state.Status(ctx) != StatusNeedsClub {
		t.Fatalf("status %v, want needs-club", s.state.Status(ctx))
	}
}

// failingProvisioner creates a club and records completion, then fails
// before committing.
type failingProvisioner struct{ db *database.DB }

func (p failingProvisioner) ProvisionHomeClub(ctx context.Context, choice ClubChoice, complete CompleteFunc) (*ProvisionResult, error) {
	tx, err := p.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := findOrCreateClub(ctx, tx, choice.DisplayName)
	if err != nil {
		return nil, err
	}
	if err := complete(ctx, tx, res.ClubID); err != nil {
		return nil, err
	}
	return nil, errors.New("import failed")
}

func TestChooseClub(t *testing.T) {
	ctx := context.Background()
	s, db := newTestService(t)

	if _, err := s.ChooseClub(ctx, "hove-park-tennis-club"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("ChooseClub before an administrator exists: %v", err)
	}
	if _, err := s.CreateAdmin(ctx, redeemed(t, s), "first.admin", goodPassword); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ChooseClub(ctx, "nowhere"); !errors.Is(err, ErrUnknownClub) {
		t.Fatalf("unknown club: %v", err)
	}

	// A failing provisioner leaves nothing behind.
	failing := NewService(s.state, failingProvisioner{db: db})
	if _, err := failing.ChooseClub(ctx, "hove-park-tennis-club"); err == nil {
		t.Fatal("failing provisioner: want error")
	}
	var clubs int
	if err := db.Get(&clubs, `SELECT COUNT(*) FROM clubs`); err != nil || clubs != 0 {
		t.Fatalf("clubs after failed provisioning: %d (%v)", clubs, err)
	}
	if s.state.Status(ctx) != StatusNeedsClub {
		t.Fatal("setup complete after failed provisioning")
	}

	res, err := s.ChooseClub(ctx, "hove-park-tennis-club")
	if err != nil {
		t.Fatalf("ChooseClub: %v", err)
	}
	if !res.CreatedClub || res.ClubName != "Hove Park Tennis Club" {
		t.Fatalf("result %+v", res)
	}
	if s.state.Status(ctx) != StatusReady || s.state.HomeClub().ID() != res.ClubID {
		t.Fatalf("after ChooseClub: status %v home %d", s.state.Status(ctx), s.state.HomeClub().ID())
	}
	if !s.RecentlyCompleted(ctx) {
		t.Fatal("RecentlyCompleted false straight after completion")
	}
	if _, err := s.ChooseClub(ctx, "st-anns"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("ChooseClub after completion: %v", err)
	}
	s.now = func() time.Time { return time.Now().Add(doneWindow + time.Minute) }
	if s.RecentlyCompleted(ctx) {
		t.Fatal("RecentlyCompleted long after completion")
	}
}

func TestFindOrCreateClubReusesExistingRow(t *testing.T) {
	ctx := context.Background()
	_, db := newTestService(t)
	if _, err := db.Exec(`INSERT INTO clubs (id, name) VALUES (7, 'St Ann’s')`); err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	res, err := findOrCreateClub(ctx, tx, "St Ann's Tennis")
	if err != nil {
		t.Fatal(err)
	}
	if res.ClubID != 7 || res.CreatedClub {
		t.Fatalf("result %+v, want existing club 7", res)
	}
}

// --- The pages, end to end -------------------------------------------------

type flowClient struct {
	t      *testing.T
	srv    *httptest.Server
	client *http.Client
}

func newFlow(t *testing.T, s *Service, db *database.DB) *flowClient {
	t.Helper()
	cfg := auth.DefaultConfig()
	cfg.CookieSecure = false
	authSvc := auth.NewService(db, cfg)
	mux := http.NewServeMux()
	NewHandler(s, authSvc, findRepoDir(t, "templates")).RegisterRoutes(mux)
	auth.NewHandler(authSvc, findRepoDir(t, "templates"), "/admin/league").RegisterRoutes(mux)
	mux.HandleFunc("/admin/league/dashboard", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authSvc.SignedInUser(r); !ok {
			http.Error(w, "not signed in", http.StatusUnauthorized)
			return
		}
		_, _ = io.WriteString(w, "dashboard")
	})
	srv := httptest.NewServer(s.state.Gate(mux))
	t.Cleanup(srv.Close)
	jar, _ := cookiejar.New(nil)
	return &flowClient{t: t, srv: srv, client: &http.Client{
		Jar:           jar,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

func (f *flowClient) do(method, path string, form url.Values) (int, string, string) {
	f.t.Helper()
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, _ := http.NewRequest(method, f.srv.URL+path, body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, err := f.client.Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header.Get("Location"), string(b)
}

func TestSetupPagesFlow(t *testing.T) {
	ctx := context.Background()
	s, db := newTestService(t)
	code, _ := s.IssueCode(ctx)
	browser := newFlow(t, s, db)

	// Everything leads to the code page.
	if st, loc, _ := browser.do("GET", "/admin/league/dashboard", nil); st != http.StatusSeeOther || loc != Path {
		t.Fatalf("gate: %d %q", st, loc)
	}
	if st, _, body := browser.do("GET", Path, nil); st != http.StatusOK || !strings.Contains(body, `data-setup-step="code"`) {
		t.Fatalf("code page: %d", st)
	}
	// Steps 2 and 3 send a browser without the code back to step 1.
	for _, p := range []string{AdminPath, ClubPath} {
		if st, loc, _ := browser.do("GET", p, nil); st != http.StatusSeeOther || loc != Path {
			t.Fatalf("%s without code: %d %q", p, st, loc)
		}
	}
	if st, _, body := browser.do("POST", Path, url.Values{"code": {"AAAA-AAAA-AAAA"}}); st != http.StatusUnprocessableEntity || !strings.Contains(body, "doesn&#39;t match") {
		t.Fatalf("wrong code: %d", st)
	}
	if st, loc, _ := browser.do("POST", Path, url.Values{"code": {code}}); st != http.StatusSeeOther || loc != AdminPath {
		t.Fatalf("right code: %d %q", st, loc)
	}

	// Closing the browser and coming back resumes at step 2.
	if st, loc, _ := browser.do("GET", Path, nil); st != http.StatusSeeOther || loc != AdminPath {
		t.Fatalf("resume: %d %q", st, loc)
	}
	// Another browser cannot use the code again or reach step 2.
	other := newFlow(t, s, db)
	if st, _, body := other.do("POST", Path, url.Values{"code": {code}}); st != http.StatusOK || !strings.Contains(body, "no setup code waiting") {
		t.Fatalf("reused code: %d", st)
	}
	if st, loc, _ := other.do("GET", AdminPath, nil); st != http.StatusSeeOther || loc != Path {
		t.Fatalf("other browser on step 2: %d %q", st, loc)
	}

	if st, _, body := browser.do("POST", AdminPath, url.Values{"username": {"first.admin"}, "password": {goodPassword}, "password_confirm": {"different password"}}); st != http.StatusUnprocessableEntity || !strings.Contains(body, "don&#39;t match") {
		t.Fatalf("mismatched passwords: %d", st)
	}
	if st, loc, _ := browser.do("POST", AdminPath, url.Values{"username": {"first.admin"}, "password": {goodPassword}, "password_confirm": {goodPassword}}); st != http.StatusSeeOther || loc != ClubPath {
		t.Fatalf("create admin: %d %q", st, loc)
	}

	// The administrator exists: code entry and administrator creation are 404.
	for _, c := range []struct{ method, path string }{{"POST", Path}, {"GET", AdminPath}, {"POST", AdminPath}} {
		if st, _, _ := browser.do(c.method, c.path, url.Values{}); st != http.StatusNotFound {
			t.Errorf("%s %s after admin: %d, want 404", c.method, c.path, st)
		}
	}
	// A browser that is not signed in is told to sign in, and cannot pick the club.
	if st, _, body := other.do("GET", Path, nil); st != http.StatusOK || !strings.Contains(body, `data-setup-step="signin"`) {
		t.Fatalf("needs-club entry page: %d", st)
	}
	if st, _, _ := other.do("POST", ClubPath, url.Values{"club": {"st-anns"}}); st != http.StatusForbidden {
		t.Fatalf("club step without sign-in: %d, want 403", st)
	}

	// The new administrator is signed in and resumes at step 3.
	if st, loc, _ := browser.do("GET", Path, nil); st != http.StatusSeeOther || loc != ClubPath {
		t.Fatalf("resume at club: %d %q", st, loc)
	}
	if st, _, body := browser.do("GET", ClubPath, nil); st != http.StatusOK || !strings.Contains(body, "Hove Park Tennis Club") {
		t.Fatalf("club page: %d", st)
	}
	if st, _, _ := browser.do("POST", ClubPath, url.Values{"club": {"not-a-club"}}); st != http.StatusUnprocessableEntity {
		t.Fatalf("unknown club: %d", st)
	}
	if st, loc, _ := browser.do("POST", ClubPath, url.Values{"club": {"st-anns"}}); st != http.StatusSeeOther || loc != ClubPath+"?working=1" {
		t.Fatalf("choose club: %d %q", st, loc)
	}
	waitForClubJob(t, s)
	if st, loc, _ := browser.do("GET", ClubPath+"?working=1", nil); st != http.StatusSeeOther || loc != DonePath {
		t.Fatalf("waiting page after the club step: %d %q", st, loc)
	}
	if st, _, body := browser.do("GET", DonePath, nil); st != http.StatusOK || !strings.Contains(body, "St Ann&#39;s Tennis") {
		t.Fatalf("done page: %d", st)
	}
	if st, _, body := browser.do("GET", "/admin/league/dashboard", nil); st != http.StatusOK || body != "dashboard" {
		t.Fatalf("dashboard after setup: %d %q", st, body)
	}

	// Setup is over: every setup page is gone, for everyone.
	for _, f := range []*flowClient{browser, other} {
		for _, p := range []string{Path, AdminPath, ClubPath} {
			for _, m := range []string{"GET", "POST"} {
				if st, _, _ := f.do(m, p, url.Values{}); st != http.StatusNotFound {
					t.Errorf("%s %s after setup: %d, want 404", m, p, st)
				}
			}
		}
	}
	if st, _, _ := other.do("GET", DonePath, nil); st != http.StatusNotFound {
		t.Errorf("done page for a signed-out browser: %d, want 404", st)
	}
	if userCount(t, db) != 1 {
		t.Fatalf("%d users at the end of setup, want 1", userCount(t, db))
	}
}

// A database that already had an administrator before this flow existed
// (needs-club) can finish with a signed-in administrator.
func TestNeedsClubWithExistingAdmin(t *testing.T) {
	ctx := context.Background()
	s, db := newTestService(t)
	hash, _ := auth.HashPassword(goodPassword)
	if _, err := db.Exec(`INSERT INTO users (username, password_hash, role) VALUES ('old.admin', ?, 'admin')`, hash); err != nil {
		t.Fatal(err)
	}
	browser := newFlow(t, s, db)
	if st, loc, _ := browser.do("GET", ClubPath, nil); st != http.StatusSeeOther || loc != Path {
		t.Fatalf("club page signed out: %d %q", st, loc)
	}
	if st, loc, _ := browser.do("GET", "/login", nil); st != http.StatusOK {
		t.Fatalf("login page in needs-club: %d %q", st, loc)
	}
	if st, _, _ := browser.do("POST", "/login", url.Values{"username": {"old.admin"}, "password": {goodPassword}}); st != http.StatusSeeOther {
		t.Fatalf("sign in: %d", st)
	}
	if st, loc, _ := browser.do("GET", Path, nil); st != http.StatusSeeOther || loc != ClubPath {
		t.Fatalf("signed in: %d %q, want club step", st, loc)
	}
	if st, loc, _ := browser.do("POST", ClubPath, url.Values{"club": {"queens-park"}}); st != http.StatusSeeOther || loc != ClubPath+"?working=1" {
		t.Fatalf("choose club: %d %q", st, loc)
	}
	waitForClubJob(t, s)
	if s.state.Status(ctx) != StatusReady {
		t.Fatal("not ready after choosing the club")
	}
}

// waitForClubJob waits for the background club step to finish.
func waitForClubJob(t *testing.T, s *Service) ClubJob {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if job, ok := s.ClubJob(); ok && !job.Running {
			return job
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("club step did not finish")
	return ClubJob{}
}
