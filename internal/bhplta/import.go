// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package bhplta

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/normalize"
	"jim-dot-tennis/internal/repository"
	"jim-dot-tennis/internal/services"
)

// League naming for the league the import creates when there is none.
const (
	LeagueName   = "Brighton & Hove Parks Tennis League"
	LeagueRegion = "Brighton & Hove"
)

// DefaultWeeks is the number of weeks in a BHPLTA season. The second half
// starts at week DefaultWeeks/2+1 (see admin.TeamEligibilityService).
const DefaultWeeks = 18

// Options tunes an import.
type Options struct {
	// HomeSlug is the BHPLTA club page slug of the home club chosen in
	// setup (one of services.KnownClubSlugs), or "" when the import is not
	// choosing a home club.
	HomeSlug string
	// HomeName is the home club's name if it has to be created and its page
	// could not be read (defaults to the known club's display name).
	HomeName string
	// Weeks is the number of weeks to create for a new season
	// (DefaultWeeks when 0; more if the fixtures run later).
	Weeks int
	// KeepExistingSeason: when the season BHPLTA lists already has
	// divisions here, leave it alone (and still import the clubs) instead
	// of failing with a *SeasonExistsError. Setup uses this.
	KeepExistingSeason bool
}

// SeasonExistsError: the season the fixtures belong to already has
// divisions, so importing it again would duplicate them.
type SeasonExistsError struct {
	SeasonID   uint
	SeasonName string
}

func (e *SeasonExistsError) Error() string {
	return fmt.Sprintf("%s already has divisions; nothing was imported", e.SeasonName)
}

// Summary reports what an import did.
type Summary struct {
	LeagueID      uint
	LeagueName    string
	LeagueCreated bool

	// Season is zero when there were no fixtures to import.
	SeasonID        uint
	SeasonName      string
	SeasonYear      int
	SeasonCreated   bool
	SeasonActivated bool
	SeasonKept      bool // the season already existed with divisions and was left alone

	Divisions int
	Teams     int
	Weeks     int // weeks created
	Fixtures  int
	FirstWeek int // first week number with fixtures on the BHPLTA page
	LastWeek  int // last week number with fixtures on the BHPLTA page

	ClubsCreated int
	ClubsMatched int // clubs that already existed here
	ClubsUpdated int // existing clubs given venue details they lacked

	HomeClubID      uint
	HomeClubName    string
	HomeClubCreated bool // the home club's row was created by this import
	HomeTeams       int

	NoFixtures bool // no season: BHPLTA lists no upcoming fixtures (between seasons)
	// Unmatched lists what could not be matched or read.
	Unmatched []string
	// Notes are things the person should know (no fixtures yet, partial
	// season, season left inactive...).
	Notes []string
}

// clubRow is the part of a clubs row the import reads and fills in.
type clubRow struct {
	ID           uint     `db:"id"`
	Name         string   `db:"name"`
	Address      *string  `db:"address"`
	Website      *string  `db:"website"`
	PhoneNumber  *string  `db:"phone_number"`
	Latitude     *float64 `db:"latitude"`
	Longitude    *float64 `db:"longitude"`
	Postcode     *string  `db:"postcode"`
	AddressLine1 *string  `db:"address_line_1"`
	AddressLine2 *string  `db:"address_line_2"`
	City         *string  `db:"city"`
}

// importer holds the state of one Import call.
type importer struct {
	ctx  context.Context
	tx   *sqlx.Tx
	snap *Snapshot
	opts Options
	now  time.Time
	sum  *Summary

	clubs     []*clubRow
	created   map[uint]bool      // clubs.id created by this import
	pageClub  map[*ClubPage]uint // snapshot club -> clubs.id
	prefixKey map[string]uint    // clubKey(team name prefix) -> clubs.id, from linked teams
}

// Import writes snap into tx: the league (found or created), every club
// (matched to existing clubs by name, otherwise created, with venue details
// filled in where missing), and, when the snapshot has fixtures, the season
// they belong to with its weeks, divisions, teams and fixtures. It never
// commits or rolls back; on error the caller must roll back.
//
// Clubs are never duplicated: a later import matches the clubs an earlier one
// created. Teams belong to a season, and a season that already has divisions
// is not imported again (see Options.KeepExistingSeason).
func Import(ctx context.Context, tx *sqlx.Tx, snap *Snapshot, opts Options) (*Summary, error) {
	if opts.Weeks <= 0 {
		opts.Weeks = DefaultWeeks
	}
	im := &importer{
		ctx: ctx, tx: tx, snap: snap, opts: opts, now: time.Now(),
		sum:       &Summary{NoFixtures: snap.NoFixtures},
		created:   map[uint]bool{},
		pageClub:  map[*ClubPage]uint{},
		prefixKey: map[string]uint{},
	}
	im.sum.Unmatched = append(im.sum.Unmatched, snap.Warnings...)

	year := im.now.Year()
	if first, _, ok := fixtureDateRange(snap.Divisions); ok {
		year = first.Year()
	}
	if err := im.league(year); err != nil {
		return nil, err
	}
	if err := im.loadClubs(); err != nil {
		return nil, err
	}
	for _, c := range snap.Clubs {
		id, err := im.club(c)
		if err != nil {
			return nil, err
		}
		im.pageClub[c] = id
	}
	if opts.HomeSlug != "" {
		if err := im.homeClub(); err != nil {
			return nil, err
		}
	}

	if snap.NoFixtures || len(snap.Divisions) == 0 {
		// No season: the pages showing this summary say what to do meanwhile.
		im.sum.NoFixtures = true
		im.sum.Notes = append(im.sum.Notes, "BHPLTA has not published any upcoming fixtures yet, so no season was created. "+
			"That is normal between seasons.")
		return im.sum, nil
	}
	if err := im.season(); err != nil {
		return nil, err
	}
	return im.sum, nil
}

// league finds the BHPLTA league or creates it.
func (im *importer) league(year int) error {
	var leagues []struct {
		ID   uint   `db:"id"`
		Name string `db:"name"`
	}
	if err := im.tx.SelectContext(im.ctx, &leagues, `SELECT id, name FROM leagues WHERE type = ? ORDER BY year DESC, id DESC`, string(models.ParksLeague)); err != nil {
		return fmt.Errorf("read leagues: %w", err)
	}
	for _, l := range leagues {
		if l.Name == LeagueName {
			im.sum.LeagueID, im.sum.LeagueName = l.ID, l.Name
			return nil
		}
	}
	if len(leagues) > 0 {
		im.sum.LeagueID, im.sum.LeagueName = leagues[0].ID, leagues[0].Name
		return nil
	}
	res, err := im.tx.ExecContext(im.ctx, `
		INSERT INTO leagues (name, type, year, region, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`, LeagueName, string(models.ParksLeague), year, LeagueRegion, im.now, im.now)
	if err != nil {
		return fmt.Errorf("create league: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("create league: %w", err)
	}
	im.sum.LeagueID, im.sum.LeagueName, im.sum.LeagueCreated = uint(id), LeagueName, true
	return nil
}

func (im *importer) loadClubs() error {
	if err := im.tx.SelectContext(im.ctx, &im.clubs, `
		SELECT id, name, address, website, phone_number, latitude, longitude,
		       postcode, address_line_1, address_line_2, city
		FROM clubs ORDER BY id`); err != nil {
		return fmt.Errorf("read clubs: %w", err)
	}
	return nil
}

// findClub returns the existing club matching any of names, or nil.
func (im *importer) findClub(names ...string) *clubRow {
	for _, n := range names {
		for _, c := range im.clubs {
			if n != "" && normalize.Apostrophes(c.Name) == normalize.Apostrophes(n) {
				return c
			}
		}
	}
	for _, n := range names {
		k := clubKey(n)
		if k == "" {
			continue
		}
		for _, c := range im.clubs {
			if clubKey(c.Name) == k {
				return c
			}
		}
	}
	return nil
}

// club returns the clubs.id for a snapshot club, matching an existing club or
// creating one, and fills in venue details the existing club lacks.
func (im *importer) club(c *ClubPage) (uint, error) {
	names := []string{c.Name}
	if c.Info != nil {
		names = append(names, c.Info.Name)
	}
	if row := im.findClub(names...); row != nil {
		im.sum.ClubsMatched++
		if c.Info != nil {
			updated, err := im.fillVenue(row, c.Info)
			if err != nil {
				return 0, err
			}
			if updated {
				im.sum.ClubsUpdated++
			}
		}
		return row.ID, nil
	}
	return im.createClub(c.Name, c.Info)
}

func (im *importer) createClub(name string, info *services.ScrapedClubInfo) (uint, error) {
	row := &clubRow{Name: name}
	if info != nil {
		row.Address = nonEmpty(info.Address)
		row.Website = nonEmpty(info.Website)
		row.PhoneNumber = nonEmpty(info.Phone)
		row.Latitude, row.Longitude = info.Latitude, info.Longitude
		row.Postcode = nonEmpty(info.Postcode)
		row.AddressLine1 = nonEmpty(info.AddressLine1)
		row.AddressLine2 = nonEmpty(info.AddressLine2)
		row.City = nonEmpty(info.City)
	}
	// address / website / phone_number are read into plain strings
	// elsewhere, so store "" rather than NULL.
	res, err := im.tx.ExecContext(im.ctx, `
		INSERT INTO clubs (name, address, website, phone_number, latitude, longitude,
			postcode, address_line_1, address_line_2, city, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.Name, deref(row.Address), deref(row.Website), deref(row.PhoneNumber), row.Latitude, row.Longitude,
		row.Postcode, row.AddressLine1, row.AddressLine2, row.City, im.now, im.now)
	if err != nil {
		return 0, fmt.Errorf("create club %q: %w", name, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("create club %q: %w", name, err)
	}
	row.ID = uint(id)
	im.created[row.ID] = true
	im.clubs = append(im.clubs, row)
	im.sum.ClubsCreated++
	return row.ID, nil
}

// fillVenue copies venue details from info into empty fields of row.
func (im *importer) fillVenue(row *clubRow, info *services.ScrapedClubInfo) (bool, error) {
	changed := false
	fillStr := func(dst **string, v string) {
		if v != "" && (*dst == nil || strings.TrimSpace(**dst) == "") {
			*dst = &v
			changed = true
		}
	}
	fillStr(&row.Address, info.Address)
	fillStr(&row.Website, info.Website)
	fillStr(&row.PhoneNumber, info.Phone)
	fillStr(&row.Postcode, info.Postcode)
	fillStr(&row.AddressLine1, info.AddressLine1)
	fillStr(&row.AddressLine2, info.AddressLine2)
	fillStr(&row.City, info.City)
	if row.Latitude == nil && row.Longitude == nil && info.Latitude != nil && info.Longitude != nil {
		row.Latitude, row.Longitude = info.Latitude, info.Longitude
		changed = true
	}
	if !changed {
		return false, nil
	}
	_, err := im.tx.ExecContext(im.ctx, `
		UPDATE clubs SET address = ?, website = ?, phone_number = ?, latitude = ?, longitude = ?,
			postcode = ?, address_line_1 = ?, address_line_2 = ?, city = ?, updated_at = ?
		WHERE id = ?`,
		deref(row.Address), deref(row.Website), deref(row.PhoneNumber), row.Latitude, row.Longitude,
		row.Postcode, row.AddressLine1, row.AddressLine2, row.City, im.now, row.ID)
	if err != nil {
		return false, fmt.Errorf("update club %q: %w", row.Name, err)
	}
	return true, nil
}

// homeClub resolves Options.HomeSlug to a clubs row, creating it if needed.
func (im *importer) homeClub() error {
	name := im.opts.HomeName
	for _, k := range services.KnownClubSlugs {
		if k.Slug == im.opts.HomeSlug && name == "" {
			name = k.DisplayName
		}
	}
	if c := im.snap.club(im.opts.HomeSlug); c != nil {
		im.sum.HomeClubID = im.pageClub[c]
	} else {
		if name == "" {
			return fmt.Errorf("unknown home club %q", im.opts.HomeSlug)
		}
		if row := im.findClub(name); row != nil {
			im.sum.HomeClubID = row.ID
		} else {
			id, err := im.createClub(name, nil)
			if err != nil {
				return err
			}
			im.sum.HomeClubID = id
		}
	}
	for _, c := range im.clubs {
		if c.ID == im.sum.HomeClubID {
			im.sum.HomeClubName = c.Name
		}
	}
	im.sum.HomeClubCreated = im.created[im.sum.HomeClubID]
	return nil
}

// season writes the season, weeks, divisions, teams and fixtures.
func (im *importer) season() error {
	first, last, ok := fixtureDateRange(im.snap.Divisions)
	if !ok {
		return fmt.Errorf("no fixture dates found on the BHPLTA fixtures page: %w", services.ErrUnexpectedMarkup)
	}
	minWeek, maxWeek := weekRange(im.snap.Divisions)
	if minWeek < 1 {
		return fmt.Errorf("fixtures without a week number on the BHPLTA fixtures page: %w", services.ErrUnexpectedMarkup)
	}
	im.sum.FirstWeek, im.sum.LastWeek = minWeek, maxWeek
	year := first.Year()
	numWeeks := im.opts.Weeks
	if maxWeek > numWeeks {
		numWeeks = maxWeek
	}

	// Season start: the first fixture, or (if BHPLTA no longer lists the
	// early weeks) an estimate a week per missing week earlier.
	start := first.AddDate(0, 0, -7*(minWeek-1))
	end := time.Date(year, time.September, 30, 0, 0, 0, 0, time.UTC)
	if last.After(end) {
		end = last
	}

	seasonID, created, err := im.findOrCreateSeason(year, start, end)
	if err != nil {
		return err
	}
	if im.sum.SeasonKept {
		return nil
	}
	if _, err := im.tx.ExecContext(im.ctx, `
		INSERT OR IGNORE INTO league_seasons (league_id, season_id, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		im.sum.LeagueID, seasonID, im.now, im.now); err != nil {
		return fmt.Errorf("link league to season: %w", err)
	}

	weeks, err := im.weeks(seasonID, created, numWeeks, maxWeek, start, end)
	if err != nil {
		return err
	}
	if minWeek > 1 {
		im.sum.Notes = append(im.sum.Notes, fmt.Sprintf(
			"BHPLTA only lists upcoming fixtures, so fixtures from week %d onwards were imported; weeks 1 to %d have already been played and are not on its website.",
			minWeek, minWeek-1))
	}

	teamIDs := map[string]uint{}
	for i, sd := range im.snap.Divisions {
		divID, err := im.division(sd, i+1, seasonID)
		if err != nil {
			return err
		}
		im.sum.Divisions++
		for _, team := range sd.Teams {
			clubID, err := im.teamClub(team)
			if err != nil {
				return err
			}
			res, err := im.tx.ExecContext(im.ctx, `
				INSERT INTO teams (name, club_id, division_id, season_id, active, created_at, updated_at)
				VALUES (?, ?, ?, ?, TRUE, ?, ?)`, team, clubID, divID, seasonID, im.now, im.now)
			if err != nil {
				return fmt.Errorf("create team %q: %w", team, err)
			}
			id, err := res.LastInsertId()
			if err != nil {
				return fmt.Errorf("create team %q: %w", team, err)
			}
			teamIDs[sd.Name+"\x00"+team] = uint(id)
			im.sum.Teams++
			if clubID == im.sum.HomeClubID && clubID != 0 {
				im.sum.HomeTeams++
			}
		}
		for _, f := range sd.Fixtures {
			week, ok := weeks[f.Week]
			if !ok {
				return fmt.Errorf("fixture %s v %s is in week %d, which the season does not have", f.HomeTeam, f.AwayTeam, f.Week)
			}
			date := f.Date
			if date.IsZero() {
				date = week
				im.sum.Unmatched = append(im.sum.Unmatched, fmt.Sprintf("%s v %s (%s, week %d) had no readable date; it was given the start of its week.", f.HomeTeam, f.AwayTeam, sd.Name, f.Week))
			}
			if _, err := im.tx.ExecContext(im.ctx, `
				INSERT INTO fixtures (home_team_id, away_team_id, division_id, season_id, week_id,
					scheduled_date, venue_location, status, created_at, updated_at)
				VALUES (?, ?, ?, ?, (SELECT id FROM weeks WHERE season_id = ? AND week_number = ?), ?, 'TBD', ?, ?, ?)`,
				teamIDs[sd.Name+"\x00"+f.HomeTeam], teamIDs[sd.Name+"\x00"+f.AwayTeam], divID, seasonID,
				seasonID, f.Week, date, string(models.Scheduled), im.now, im.now); err != nil {
				return fmt.Errorf("create fixture %s v %s: %w", f.HomeTeam, f.AwayTeam, err)
			}
			im.sum.Fixtures++
		}
	}

	if im.opts.HomeSlug != "" && im.sum.HomeTeams == 0 {
		im.sum.Notes = append(im.sum.Notes, fmt.Sprintf("No %s teams were found in BHPLTA's fixtures for %d.", im.sum.HomeClubName, year))
	}
	if !im.sum.SeasonActivated {
		im.sum.Notes = append(im.sum.Notes, fmt.Sprintf("%s was not made the active season because another season is active. Make it active from the Seasons page when you are ready.", im.sum.SeasonName))
	}
	return nil
}

// findOrCreateSeason returns the season for year, creating it if needed.
func (im *importer) findOrCreateSeason(year int, start, end time.Time) (uint, bool, error) {
	var existing struct {
		ID   uint   `db:"id"`
		Name string `db:"name"`
	}
	err := im.tx.GetContext(im.ctx, &existing, `SELECT id, name FROM seasons WHERE year = ? ORDER BY id LIMIT 1`, year)
	switch {
	case err == nil:
		var divisions int
		if err := im.tx.GetContext(im.ctx, &divisions, `SELECT COUNT(*) FROM divisions WHERE season_id = ?`, existing.ID); err != nil {
			return 0, false, fmt.Errorf("check season %s: %w", existing.Name, err)
		}
		im.sum.SeasonID, im.sum.SeasonName, im.sum.SeasonYear = existing.ID, existing.Name, year
		if divisions > 0 {
			if im.opts.KeepExistingSeason {
				im.sum.SeasonKept = true
				im.sum.Notes = append(im.sum.Notes, fmt.Sprintf("%s already has divisions here, so it was left as it is.", existing.Name))
				return existing.ID, false, nil
			}
			return 0, false, &SeasonExistsError{SeasonID: existing.ID, SeasonName: existing.Name}
		}
		var active int
		if err := im.tx.GetContext(im.ctx, &active, `SELECT COUNT(*) FROM seasons WHERE is_active = TRUE`); err != nil {
			return 0, false, fmt.Errorf("check active season: %w", err)
		}
		if active == 0 {
			if _, err := im.tx.ExecContext(im.ctx, `UPDATE seasons SET is_active = TRUE, updated_at = ? WHERE id = ?`, im.now, existing.ID); err != nil {
				return 0, false, fmt.Errorf("activate season: %w", err)
			}
		}
		var isActive bool
		if err := im.tx.GetContext(im.ctx, &isActive, `SELECT is_active FROM seasons WHERE id = ?`, existing.ID); err != nil {
			return 0, false, fmt.Errorf("check season %s: %w", existing.Name, err)
		}
		im.sum.SeasonActivated = isActive
		return existing.ID, false, nil
	case !errors.Is(err, sql.ErrNoRows):
		return 0, false, fmt.Errorf("look up the %d season: %w", year, err)
	}

	var active int
	if err := im.tx.GetContext(im.ctx, &active, `SELECT COUNT(*) FROM seasons WHERE is_active = TRUE`); err != nil {
		return 0, false, fmt.Errorf("check active season: %w", err)
	}
	name := fmt.Sprintf("%d Season", year)
	res, err := im.tx.ExecContext(im.ctx, `
		INSERT INTO seasons (name, year, start_date, end_date, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, name, year, start, end, active == 0, im.now, im.now)
	if err != nil {
		return 0, false, fmt.Errorf("create season %s: %w", name, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, false, fmt.Errorf("create season %s: %w", name, err)
	}
	im.sum.SeasonID, im.sum.SeasonName, im.sum.SeasonYear = uint(id), name, year
	im.sum.SeasonCreated, im.sum.SeasonActivated = true, active == 0
	return uint(id), true, nil
}

// weeks makes sure the season has weeks 1..numWeeks and returns each week's
// start by number. A season that already has weeks keeps them, as long as
// they reach the last fixture week.
func (im *importer) weeks(seasonID uint, created bool, numWeeks, maxWeek int, start, end time.Time) (map[int]time.Time, error) {
	var existing []struct {
		Number int       `db:"week_number"`
		Start  time.Time `db:"start_date"`
	}
	if !created {
		if err := im.tx.SelectContext(im.ctx, &existing, `SELECT week_number, start_date FROM weeks WHERE season_id = ?`, seasonID); err != nil {
			return nil, fmt.Errorf("read weeks: %w", err)
		}
	}
	if len(existing) > 0 {
		byNumber := map[int]time.Time{}
		for _, w := range existing {
			byNumber[w.Number] = w.Start
		}
		for n := 1; n <= maxWeek; n++ {
			if _, ok := byNumber[n]; !ok {
				return nil, fmt.Errorf("the season has no week %d, but BHPLTA's fixtures run to week %d", n, maxWeek)
			}
		}
		return byNumber, nil
	}

	starts := weekStarts(im.snap.Divisions, numWeeks, start)
	out := map[int]time.Time{}
	for n := 1; n <= numWeeks; n++ {
		weekEnd := end
		if n < numWeeks {
			weekEnd = starts[n+1].AddDate(0, 0, -1)
		}
		if weekEnd.Before(starts[n]) {
			weekEnd = starts[n].AddDate(0, 0, 6)
		}
		if _, err := im.tx.ExecContext(im.ctx, `
			INSERT INTO weeks (week_number, season_id, start_date, end_date, name, is_active, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, FALSE, ?, ?)`, n, seasonID, starts[n], weekEnd, fmt.Sprintf("Week %d", n), im.now, im.now); err != nil {
			return nil, fmt.Errorf("create week %d: %w", n, err)
		}
		out[n] = starts[n]
		im.sum.Weeks++
	}
	return out, nil
}

var divisionNumberRe = regexp.MustCompile(`(\d+)`)

// division creates one division. Its level is the number in its name
// ("Division 2" is level 2), else its position on the page; its play day is
// the weekday most of its fixtures fall on.
func (im *importer) division(sd services.ScrapedDivision, position int, seasonID uint) (uint, error) {
	level := position
	if m := divisionNumberRe.FindStringSubmatch(sd.Name); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			level = n
		}
	}
	res, err := im.tx.ExecContext(im.ctx, `
		INSERT INTO divisions (name, level, play_day, league_id, season_id, max_teams_per_club, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 2, ?, ?)`, sd.Name, level, playDay(sd.Fixtures), im.sum.LeagueID, seasonID, im.now, im.now)
	if err != nil {
		return 0, fmt.Errorf("create division %q: %w", sd.Name, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("create division %q: %w", sd.Name, err)
	}
	return uint(id), nil
}

// teamClub finds the club a team belongs to: the club page its home fixtures
// link to, else the club whose other teams share its name prefix, else an
// existing club of that name. If none matches a club is created from the
// name and reported as unmatched.
func (im *importer) teamClub(team string) (uint, error) {
	if len(im.prefixKey) == 0 {
		im.indexTeamLinks()
	}
	prefix := repository.ExtractClubNameFromTeamName(team)
	if slug := im.teamSlug(team); slug != "" {
		if c := im.snap.club(slug); c != nil {
			return im.pageClub[c], nil
		}
	}
	if id, ok := im.prefixKey[clubKey(prefix)]; ok {
		return id, nil
	}
	if row := im.findClub(prefix); row != nil {
		return row.ID, nil
	}
	im.sum.Unmatched = append(im.sum.Unmatched, fmt.Sprintf("No BHPLTA club page was found for the team %q; a club called %q was created for it.", team, prefix))
	id, err := im.createClub(prefix, nil)
	if err != nil {
		return 0, err
	}
	im.prefixKey[clubKey(prefix)] = id
	return id, nil
}

// teamSlug returns the club page slug a team's home fixtures link to.
func (im *importer) teamSlug(team string) string {
	for _, d := range im.snap.Divisions {
		for _, f := range d.Fixtures {
			if f.HomeTeam == team && f.HomeClubSlug != "" {
				return f.HomeClubSlug
			}
		}
	}
	return ""
}

// indexTeamLinks maps the club-name prefix of every linked home team to its
// club, so teams that only appear as away teams can be matched too.
func (im *importer) indexTeamLinks() {
	for _, d := range im.snap.Divisions {
		for _, f := range d.Fixtures {
			if f.HomeClubSlug == "" {
				continue
			}
			if c := im.snap.club(f.HomeClubSlug); c != nil {
				im.prefixKey[clubKey(repository.ExtractClubNameFromTeamName(f.HomeTeam))] = im.pageClub[c]
			}
		}
	}
	for c, id := range im.pageClub {
		im.prefixKey[clubKey(c.Name)] = id
		if c.Info != nil {
			im.prefixKey[clubKey(c.Info.Name)] = id
		}
	}
	delete(im.prefixKey, "")
}

// clubKey reduces a club or team-prefix name to its distinguishing words, so
// "Hove Park Tennis Club", "Hove Park" and "Hove" compare equal, as do
// "St Ann's Tennis", "St Ann’s" and "St Anns".
func clubKey(name string) string {
	var words []string
	for _, w := range strings.Fields(normalize.ForComparison(name)) {
		w = strings.Trim(w, ",&()-")
		switch w {
		case "", "tennis", "club", "lawn", "park", "the", "tc", "ltc":
			continue
		}
		words = append(words, w)
	}
	return strings.Join(words, " ")
}

// fixtureDateRange returns the first and last fixture dates.
func fixtureDateRange(divisions []services.ScrapedDivision) (time.Time, time.Time, bool) {
	var first, last time.Time
	for _, d := range divisions {
		for _, f := range d.Fixtures {
			if f.Date.IsZero() {
				continue
			}
			if first.IsZero() || f.Date.Before(first) {
				first = f.Date
			}
			if f.Date.After(last) {
				last = f.Date
			}
		}
	}
	return first, last, !first.IsZero()
}

func weekRange(divisions []services.ScrapedDivision) (int, int) {
	lo, hi := 0, 0
	for _, d := range divisions {
		for _, f := range d.Fixtures {
			if lo == 0 || f.Week < lo {
				lo = f.Week
			}
			if f.Week > hi {
				hi = f.Week
			}
		}
	}
	return lo, hi
}

// weekStarts returns the start (a Monday) of weeks 1..numWeeks: the Monday of
// the earliest fixture in the week where BHPLTA lists one, otherwise a week
// after the previous week (or, before the first listed week, counted back
// from it). Starts always increase.
func weekStarts(divisions []services.ScrapedDivision, numWeeks int, seasonStart time.Time) map[int]time.Time {
	earliest := map[int]time.Time{}
	for _, d := range divisions {
		for _, f := range d.Fixtures {
			if f.Date.IsZero() || f.Week < 1 {
				continue
			}
			if e, ok := earliest[f.Week]; !ok || f.Date.Before(e) {
				earliest[f.Week] = f.Date
			}
		}
	}
	known := make([]int, 0, len(earliest))
	for n := range earliest {
		known = append(known, n)
	}
	sort.Ints(known)

	starts := map[int]time.Time{}
	anchorWeek, anchor := 1, monday(seasonStart)
	if len(known) > 0 {
		anchorWeek, anchor = known[0], monday(earliest[known[0]])
	}
	for n := 1; n <= numWeeks; n++ {
		var s time.Time
		if e, ok := earliest[n]; ok {
			s = monday(e)
		} else if prev, ok := starts[n-1]; ok && n > anchorWeek {
			s = prev.AddDate(0, 0, 7)
		} else {
			s = anchor.AddDate(0, 0, 7*(n-anchorWeek))
		}
		if prev, ok := starts[n-1]; ok && !s.After(prev) {
			s = prev.AddDate(0, 0, 7)
		}
		starts[n] = s
	}
	return starts
}

// monday returns the Monday on or before t (at midnight UTC).
func monday(t time.Time) time.Time {
	d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	offset := (int(d.Weekday()) + 6) % 7
	return d.AddDate(0, 0, -offset)
}

// playDay returns the weekday most fixtures fall on ("" if none have dates).
func playDay(fixtures []services.ScrapedFixture) string {
	counts := map[time.Weekday]int{}
	for _, f := range fixtures {
		if !f.Date.IsZero() {
			counts[f.Date.Weekday()]++
		}
	}
	best, bestN := time.Weekday(0), 0
	for d := time.Sunday; d <= time.Saturday; d++ {
		if counts[d] > bestN {
			best, bestN = d, counts[d]
		}
	}
	if bestN == 0 {
		return ""
	}
	return best.String()
}

func nonEmpty(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
