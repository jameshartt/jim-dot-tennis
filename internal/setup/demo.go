// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package setup

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"jim-dot-tennis/internal/repository"
)

// Demo data lets a new club look around before committing its own data.
//
// Loading it (Service.LoadDemo) is an alternative to the club choice: a
// fictional club, league, divisions, players, availability, selections and a
// few results are inserted, the fictional club becomes the home club and
// setup is recorded as complete, all in one transaction, with the
// repository.SettingDemoData flag set. Clearing it (Service.ClearDemo)
// removes everything in one transaction, drops the home club and the
// completion record, and returns the instance to the club step with the
// administrator account kept.
//
// Guards (all enforced server-side, inside the transaction):
//   - loading needs an administrator, no home club, no setup completion, no
//     demo flag and empty clubs, leagues, seasons, weeks, divisions, teams,
//     players and fixtures tables;
//   - clearing deletes nothing unless the demo flag is set.
//
// Clearing wipes every league table, including anything an administrator
// added while the demo was loaded: a demo instance is a sandbox.

// Errors returned by the demo service.
var (
	// ErrDemoHasData: the instance already has real data; demo data is refused.
	ErrDemoHasData = errors.New("setup: this instance already has data, so demo data cannot be loaded")
	// ErrDemoEnvOverride: HOME_CLUB_ID / HOME_CLUB_NAME fix the home club, so
	// a fictional one cannot take its place.
	ErrDemoEnvOverride = errors.New("setup: HOME_CLUB_ID or HOME_CLUB_NAME is set, so demo data cannot be loaded")
	// ErrNotDemo: the instance is not running on demo data; nothing to clear.
	ErrNotDemo = errors.New("setup: the instance is not running on demo data")
)

// Paths of the demo routes.
const (
	// DemoPath: POST loads the demo data (signed-in administrator, club step).
	DemoPath = Path + "/demo"
	// DemoClearPath: GET asks for confirmation, POST clears the demo data
	// (signed-in administrator, only while demo data is loaded).
	DemoClearPath = DemoPath + "/clear"
)

// clearedTables are emptied by ClearDemo, in an order that suits foreign
// keys (children first; foreign keys are also deferred to commit).
var clearedTables = []string{
	"captain_player_notes", "captains",
	"matchup_players", "matchups", "fixture_players", "fixtures",
	"player_availability", "player_availability_exceptions", "availability_time_slots",
	"player_fixture_availability", "player_general_availability", "player_divisions",
	"player_preferred_partners", "player_tennis_preferences", "preferred_name_requests",
	"player_teams", "players", "fantasy_mixed_doubles",
	"venue_overrides", "teams", "divisions", "weeks", "league_seasons", "leagues", "seasons", "clubs",
}

// keptTables are deliberately not touched by ClearDemo (accounts, sign-in
// state, instance settings, the bundled name lists and tournament set-up).
// A test requires every table to be in exactly one of the two lists, so a new
// migration has to decide.
var keptTables = []string{
	"account_audit_log", "account_links", "app_settings", "login_attempts", "push_subscriptions", "sessions", "sqlite_sequence",
	"tennis_players", "tournament_accounts", "tournament_providers", "tournaments", "user_tournament_providers", "users", "vapid_keys",
	"webauthn_credentials", "webauthn_user_handles",
}

// clearedSettings are the app_settings keys ClearDemo removes: the home club
// and completion record, the demo flag, and the club identity that belongs to
// the demo club. Feature switches and the public URL are kept.
var clearedSettings = []string{
	repository.SettingHomeClubID, repository.SettingSetupCompletedAt, repository.SettingDemoData,
	repository.SettingClubDisplayName, repository.SettingClubLogoFile, repository.SettingClubLogoSize,
	repository.SettingClubPrimaryColour, repository.SettingClubSecondaryColour,
	repository.SettingBHPLTAClubCode, repository.SettingBHPLTAClubName, repository.SettingBHPLTAClubID,
}

// DemoActive reports whether the instance is running on demo data. It reads an
// in-memory copy of the stored flag, so it is cheap enough for every request.
func (s *Service) DemoActive() bool { return s.demoActive.Load() }

// loadDemoFlag syncs the in-memory flag with app_settings.
func (s *Service) loadDemoFlag(ctx context.Context) {
	on, _, err := repository.NewSettingsRepository(s.db).GetBool(ctx, repository.SettingDemoData)
	s.demoActive.Store(err == nil && on)
}

// LoadDemo loads the fictional demo club and makes it the home club. It is
// only available at the club step (an administrator exists, no home club).
func (s *Service) LoadDemo(ctx context.Context) error {
	if job, ok := s.ClubJob(); ok && job.Running {
		return ErrClubJobRunning
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Status(ctx) != StatusNeedsClub {
		return ErrUnavailable
	}
	if os.Getenv("HOME_CLUB_ID") != "" || os.Getenv("HOME_CLUB_NAME") != "" {
		return ErrDemoEnvOverride
	}
	err := s.inTx(ctx, func(tx *sqlx.Tx) error {
		if err := requireNoRealData(ctx, tx); err != nil {
			return err
		}
		homeClubID, err := insertDemoData(ctx, tx, s.now())
		if err != nil {
			return err
		}
		if err := WriteCompletion(ctx, tx, homeClubID); err != nil {
			return err
		}
		return repository.NewSettingsRepository(tx).SetBool(ctx, repository.SettingDemoData, true)
	})
	if err != nil {
		return err
	}
	s.demoActive.Store(true)
	s.jobMu.Lock()
	s.job = nil
	s.jobMu.Unlock()
	return s.state.Activate(ctx)
}

// ClearDemo removes the demo data in one transaction and returns the instance
// to the club step. It deletes nothing unless the demo flag is set. The
// administrator account (and every other user and session) is kept.
func (s *Service) ClearDemo(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.inTx(ctx, func(tx *sqlx.Tx) error {
		st := repository.NewSettingsRepository(tx)
		on, _, err := st.GetBool(ctx, repository.SettingDemoData)
		if err != nil {
			return err
		}
		if !on {
			return ErrNotDemo
		}
		if _, err := tx.ExecContext(ctx, `PRAGMA defer_foreign_keys = ON`); err != nil {
			return fmt.Errorf("setup: defer foreign keys: %w", err)
		}
		// Accounts stay, but any link to a demo player goes.
		if _, err := tx.ExecContext(ctx, `UPDATE users SET player_id = NULL WHERE player_id IS NOT NULL`); err != nil {
			return fmt.Errorf("setup: unlink users from demo players: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM push_subscriptions WHERE player_token IS NOT NULL`); err != nil {
			return fmt.Errorf("setup: clear player push subscriptions: %w", err)
		}
		for _, table := range clearedTables {
			if _, err := tx.ExecContext(ctx, `DELETE FROM `+table); err != nil {
				return fmt.Errorf("setup: clear %s: %w", table, err)
			}
			// Start numbering afresh so the real club's data is not offset.
			if _, err := tx.ExecContext(ctx, `DELETE FROM sqlite_sequence WHERE name = ?`, table); err != nil {
				return fmt.Errorf("setup: reset %s numbering: %w", table, err)
			}
		}
		for _, key := range clearedSettings {
			if err := st.Delete(ctx, key); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.demoActive.Store(false)
	s.jobMu.Lock()
	s.job = nil
	s.jobMu.Unlock()
	if err := s.state.HomeClub().Refresh(ctx); err != nil {
		return fmt.Errorf("setup: demo data cleared, but refreshing the home club failed: %w", err)
	}
	if s.state.HomeClub().Resolved() {
		return errors.New("setup: demo data cleared, but a home club is still in effect")
	}
	return nil
}

// requireNoRealData fails with ErrDemoHasData unless the database holds no
// league data and no recorded setup completion, and demo data is not already
// loaded.
func requireNoRealData(ctx context.Context, tx *sqlx.Tx) error {
	for _, table := range []string{"clubs", "leagues", "seasons", "weeks", "divisions", "teams", "players", "fixtures"} {
		var n int
		if err := tx.GetContext(ctx, &n, `SELECT COUNT(*) FROM `+table); err != nil {
			return fmt.Errorf("setup: count %s: %w", table, err)
		}
		if n > 0 {
			return ErrDemoHasData
		}
	}
	st := repository.NewSettingsRepository(tx)
	for _, key := range []string{repository.SettingHomeClubID, repository.SettingSetupCompletedAt, repository.SettingDemoData} {
		if _, ok, err := st.GetString(ctx, key); err != nil {
			return err
		} else if ok {
			return ErrDemoHasData
		}
	}
	return nil
}

// txInserter runs inserts on tx, remembering the first error.
type txInserter struct {
	ctx context.Context
	tx  *sqlx.Tx
	err error
}

// insert runs query and returns the new row's ID (0 after an error).
func (t *txInserter) insert(query string, args ...interface{}) int64 {
	if t.err != nil {
		return 0
	}
	res, err := t.tx.ExecContext(t.ctx, query, args...)
	if err != nil {
		t.err = fmt.Errorf("setup: demo data: %w", err)
		return 0
	}
	id, _ := res.LastInsertId()
	return id
}

type demoPlayer struct {
	id     string
	gender string
}

// demoTeam is one fictional team with its four players: women 0 and 1, men 0
// and 1 (rubbers use them as Mens M0+M1, Womens W0+W1, 1st Mixed W0+M0, 2nd
// Mixed W1+M1).
type demoTeam struct {
	id      int64
	clubID  int64
	name    string
	women   [2]demoPlayer
	men     [2]demoPlayer
	players []demoPlayer
}

// insertDemoData inserts the whole demo league and returns the home club ID.
func insertDemoData(ctx context.Context, tx *sqlx.Tx, now time.Time) (uint, error) {
	in := &txInserter{ctx: ctx, tx: tx}

	// Monday of the current week (local time) anchors the season.
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	monday := today.AddDate(0, 0, -((int(today.Weekday()) + 6) % 7))
	weekStart := func(week int) time.Time { return monday.AddDate(0, 0, (week-demoCurrentWeek)*7) }
	const day = 24 * time.Hour

	seasonID := in.insert(`INSERT INTO seasons (name, year, start_date, end_date, is_active, created_at, updated_at) VALUES (?, ?, ?, ?, TRUE, ?, ?)`,
		fmt.Sprintf("Demo Season %d", monday.Year()), monday.Year(), weekStart(1), weekStart(demoWeeks).Add(6*day), now, now)
	leagueID := in.insert(`INSERT INTO leagues (name, type, year, region, created_at, updated_at) VALUES (?, 'Parks', ?, ?, ?, ?)`,
		demoLeagueName, monday.Year(), demoLeagueRegion, now, now)
	in.insert(`INSERT INTO league_seasons (league_id, season_id, created_at, updated_at) VALUES (?, ?, ?, ?)`, leagueID, seasonID, now, now)

	weekIDs := make([]int64, demoWeeks+1)
	for w := 1; w <= demoWeeks; w++ {
		weekIDs[w] = in.insert(`INSERT INTO weeks (week_number, season_id, start_date, end_date, name, is_active, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			w, seasonID, weekStart(w), weekStart(w).Add(6*day), fmt.Sprintf("Week %d", w), w == demoCurrentWeek, now, now)
	}

	divisionIDs := make([]int64, len(demoDivisions))
	for i, d := range demoDivisions {
		divisionIDs[i] = in.insert(`INSERT INTO divisions (name, level, play_day, league_id, season_id, max_teams_per_club, created_at, updated_at) VALUES (?, ?, ?, ?, ?, 2, ?, ?)`,
			d.Name, d.Level, d.PlayDay, leagueID, seasonID, now, now)
	}

	// Clubs, players and teams. teams[division][club].
	teams := make([][]*demoTeam, len(demoDivisions))
	for c, club := range demoClubs {
		clubID := in.insert(`INSERT INTO clubs (name, address, website, phone_number, created_at, updated_at) VALUES (?, ?, '', '', ?, ?)`,
			club.Name, club.Address, now, now)
		var women, men [4]demoPlayer
		for i := 0; i < 4; i++ {
			women[i] = demoPlayer{id: uuid.NewString(), gender: "Women"}
			men[i] = demoPlayer{id: uuid.NewString(), gender: "Men"}
			in.insert(`INSERT INTO players (id, first_name, last_name, club_id, gender, created_at, updated_at) VALUES (?, ?, ?, ?, 'Women', ?, ?)`,
				women[i].id, demoWomen[c*4+i], club.Surname+demoSuffixes[c*4+i], clubID, now, now)
			in.insert(`INSERT INTO players (id, first_name, last_name, club_id, gender, created_at, updated_at) VALUES (?, ?, ?, ?, 'Men', ?, ?)`,
				men[i].id, demoMen[c*4+i], club.Surname+demoSuffixes[(c*4+i+6)%len(demoSuffixes)], clubID, now, now)
		}
		for d := range demoDivisions {
			t := &demoTeam{clubID: clubID, name: fmt.Sprintf("%s %c", strings.TrimSuffix(club.Name, " Tennis Club"), 'A'+rune(d))}
			t.women = [2]demoPlayer{women[d*2], women[d*2+1]}
			t.men = [2]demoPlayer{men[d*2], men[d*2+1]}
			t.players = []demoPlayer{t.women[0], t.women[1], t.men[0], t.men[1]}
			t.id = in.insert(`INSERT INTO teams (name, club_id, division_id, season_id, active, created_at, updated_at) VALUES (?, ?, ?, ?, TRUE, ?, ?)`,
				t.name, clubID, divisionIDs[d], seasonID, now, now)
			for _, p := range t.players {
				in.insert(`INSERT INTO player_teams (player_id, team_id, season_id, is_active, created_at, updated_at) VALUES (?, ?, ?, TRUE, ?, ?)`,
					p.id, t.id, seasonID, now, now)
			}
			teams[d] = append(teams[d], t)
		}
		// The home club's first player in each team captains it.
		if c == 0 {
			for d := range demoDivisions {
				in.insert(`INSERT INTO captains (player_id, team_id, role, season_id, is_active, created_at, updated_at) VALUES (?, ?, 'Team', ?, TRUE, ?, ?)`,
					teams[d][0].women[0].id, teams[d][0].id, seasonID, now, now)
			}
		}
	}

	// Fixtures, with results for the played weeks, selections and availability
	// for the home club's upcoming ones.
	rubbers := []string{"Mens", "Womens", "1st Mixed", "2nd Mixed"}
	played := 0
	for d := range demoDivisions {
		offset := d // Monday for division 1, Tuesday for division 2
		for w, round := range demoRounds {
			week := w + 1
			home, away := teams[d][round[0]], teams[d][round[1]]
			kickoff := weekStart(week).AddDate(0, 0, offset).Add(18*time.Hour + 30*time.Minute)
			status, completed := "Scheduled", (*time.Time)(nil)
			if week < demoCurrentWeek {
				status, completed = "Completed", &kickoff
			}
			fixtureID := in.insert(`INSERT INTO fixtures (home_team_id, away_team_id, division_id, season_id, week_id, scheduled_date, venue_location, status, completed_date, notes, venue_club_id, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, '', ?, ?, ?)`,
				home.id, away.id, divisionIDs[d], seasonID, weekIDs[week], kickoff, demoClubs[round[0]].Name, status, completed, home.clubID, now, now)

			switch {
			case week < demoCurrentWeek:
				result := demoResults[played%len(demoResults)]
				played++
				for r, rubber := range rubbers {
					insertDemoRubber(in, fixtureID, rubber, result[r], home, away, now)
				}
			default:
				for _, rubber := range rubbers {
					in.insert(`INSERT INTO matchups (fixture_id, type, status, home_score, away_score, notes, created_at, updated_at) VALUES (?, ?, 'Pending', 0, 0, '', ?, ?)`,
						fixtureID, rubber, now, now)
				}
				insertDemoAvailability(in, fixtureID, week, d, home, away, round[0] == 0, round[1] == 0, now)
			}
		}
	}
	if in.err != nil {
		return 0, in.err
	}
	return uint(teams[0][0].clubID), nil
}

// insertDemoRubber inserts one finished rubber with its players.
func insertDemoRubber(in *txInserter, fixtureID int64, rubber string, sets demoResult, home, away *demoTeam, now time.Time) {
	var homeSets, awaySets int
	cols := [6]interface{}{}
	for i, s := range sets {
		cols[i*2], cols[i*2+1] = s[0], s[1]
		if s[0] > s[1] {
			homeSets++
		} else {
			awaySets++
		}
	}
	homeScore, awayScore := 0, 2
	if homeSets > awaySets {
		homeScore, awayScore = 2, 0
	}
	id := in.insert(`INSERT INTO matchups (fixture_id, type, status, home_score, away_score, notes, home_set1, away_set1, home_set2, away_set2, home_set3, away_set3, created_at, updated_at)
		VALUES (?, ?, 'Finished', ?, ?, '', ?, ?, ?, ?, ?, ?, ?, ?)`,
		fixtureID, rubber, homeScore, awayScore, cols[0], cols[1], cols[2], cols[3], cols[4], cols[5], now, now)
	pair := func(t *demoTeam) []demoPlayer {
		switch rubber {
		case "Mens":
			return []demoPlayer{t.men[0], t.men[1]}
		case "Womens":
			return []demoPlayer{t.women[0], t.women[1]}
		case "1st Mixed":
			return []demoPlayer{t.women[0], t.men[0]}
		default:
			return []demoPlayer{t.women[1], t.men[1]}
		}
	}
	for _, p := range pair(home) {
		in.insert(`INSERT INTO matchup_players (matchup_id, player_id, is_home, created_at, updated_at) VALUES (?, ?, TRUE, ?, ?)`, id, p.id, now, now)
	}
	for _, p := range pair(away) {
		in.insert(`INSERT INTO matchup_players (matchup_id, player_id, is_home, created_at, updated_at) VALUES (?, ?, FALSE, ?, ?)`, id, p.id, now, now)
	}
}

// insertDemoAvailability gives an upcoming fixture some availability answers
// and, for the home club's team in this week's division 1 fixture, a selection.
func insertDemoAvailability(in *txInserter, fixtureID int64, week, division int, home, away *demoTeam, homeIsClub, awayIsClub bool, now time.Time) {
	record := func(t *demoTeam, shift int) {
		for i, p := range t.players {
			status := demoAvailability[(i+shift+week)%len(demoAvailability)]
			in.insert(`INSERT INTO player_fixture_availability (player_id, fixture_id, status, notes, created_at, updated_at) VALUES (?, ?, ?, '', ?, ?)`,
				p.id, fixtureID, status, now, now)
		}
	}
	if homeIsClub {
		record(home, 0)
	}
	if awayIsClub {
		record(away, 3)
	}
	// Selection: this week's division 1 fixture is fully picked, division 2 is
	// left for the captain to do.
	if week == demoCurrentWeek && division == 0 && homeIsClub {
		for pos, p := range home.players {
			in.insert(`INSERT INTO fixture_players (fixture_id, player_id, is_home, position, created_at, updated_at) VALUES (?, ?, TRUE, ?, ?, ?)`,
				fixtureID, p.id, pos+1, now, now)
		}
	}
}

// --- The "Demo data" banner -------------------------------------------------

// bannerHTML is injected right after the <body> tag of every HTML page while
// demo data is loaded.
const bannerHTML = `<div id="demo-banner" data-demo-banner role="status" style="position:sticky;top:0;z-index:10000;background:#fff3cd;color:#664d03;border-bottom:2px solid #ffc107;padding:0.5rem 1rem;font:0.9rem/1.4 system-ui,sans-serif;text-align:center">` +
	`<strong>Demo data</strong> &ndash; this app is showing invented clubs, players and results. ` +
	`<a href="` + DemoClearPath + `" style="color:inherit;font-weight:600">Clear demo data</a> (administrators)</div>`

// DemoBanner is middleware that, while demo data is loaded, inserts the
// "Demo data" banner after the <body> tag of every full HTML page (admin,
// player, public and setup pages alike, whichever template drew them). Other
// responses (HTMX fragments without a <body>, JSON, assets, redirects) pass
// through untouched. When demo data is not loaded it is a pass-through
// costing one atomic read.
func (s *Service) DemoBanner(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.DemoActive() || r.Method == http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		bw := &bannerWriter{ResponseWriter: w}
		next.ServeHTTP(bw, r)
		bw.finish()
	})
}

// bannerWriter buffers HTML responses so the banner can be inserted; anything
// else is passed straight through.
type bannerWriter struct {
	http.ResponseWriter
	decided bool
	buffer  bool
	status  int
	buf     bytes.Buffer
}

func (b *bannerWriter) decide(sniff []byte) {
	if b.decided {
		return
	}
	b.decided = true
	ct := b.Header().Get("Content-Type")
	if ct == "" && len(sniff) > 0 {
		ct = http.DetectContentType(sniff)
	}
	b.buffer = strings.HasPrefix(ct, "text/html") && b.Header().Get("Content-Encoding") == ""
}

func (b *bannerWriter) WriteHeader(status int) {
	if b.decided && !b.buffer {
		b.ResponseWriter.WriteHeader(status)
		return
	}
	if b.status == 0 {
		b.status = status
	}
	if !b.decided {
		b.decide(nil)
		if !b.buffer {
			b.ResponseWriter.WriteHeader(status)
		}
	}
}

func (b *bannerWriter) Write(p []byte) (int, error) {
	if !b.decided {
		b.decide(p)
		if !b.buffer && b.status != 0 {
			b.ResponseWriter.WriteHeader(b.status)
		}
	}
	if !b.buffer {
		return b.ResponseWriter.Write(p)
	}
	return b.buf.Write(p)
}

// Flush forwards to the underlying writer for unbuffered responses.
func (b *bannerWriter) Flush() {
	if f, ok := b.ResponseWriter.(http.Flusher); ok && (!b.decided || !b.buffer) {
		f.Flush()
	}
}

// Unwrap lets http.ResponseController reach the real writer.
func (b *bannerWriter) Unwrap() http.ResponseWriter { return b.ResponseWriter }

// finish writes the buffered HTML, with the banner after the <body> tag.
func (b *bannerWriter) finish() {
	if !b.decided || !b.buffer {
		return
	}
	body := b.buf.Bytes()
	if i := indexFold(body, "<body"); i >= 0 {
		if j := bytes.IndexByte(body[i:], '>'); j >= 0 {
			at := i + j + 1
			out := make([]byte, 0, len(body)+len(bannerHTML))
			out = append(out, body[:at]...)
			out = append(out, bannerHTML...)
			out = append(out, body[at:]...)
			body = out
		}
	}
	b.Header().Del("Content-Length")
	if b.status != 0 {
		b.ResponseWriter.WriteHeader(b.status)
	}
	_, _ = b.ResponseWriter.Write(body)
}

// indexFold finds the first case-insensitive occurrence of lower-case sub.
func indexFold(b []byte, sub string) int {
	return bytes.Index(bytes.ToLower(b), []byte(sub))
}
