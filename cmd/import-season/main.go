// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

// Command import-season imports the season currently on the BHPLTA website
// into a database, using the same import as setup and the admin "Start a new
// season from BHPLTA" action (internal/bhplta): the league if missing, every
// club (matched, or created with its venue details), and the season with its
// weeks, divisions, teams and fixtures, in one transaction. Optionally it
// copies players and captains from the previous year's teams of the same name.
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	jimdottennis "jim-dot-tennis"
	"jim-dot-tennis/internal/bhplta"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/services"
)

var errDryRun = errors.New("dry run")

func main() {
	dbPath := flag.String("db", "./tennis.db", "Path to SQLite database file")
	baseURL := flag.String("base-url", services.BHPLTABaseURL(), "BHPLTA site root (default: $"+services.BHPLTABaseURLEnv+" or the real site)")
	numWeeks := flag.Int("weeks", bhplta.DefaultWeeks, "Number of weeks in a new season")
	copyPlayers := flag.Bool("copy-players", true, "Copy players and captains from the previous year's teams with the same name")
	dryRun := flag.Bool("dry-run", false, "Fetch and run the import, then roll it back and report what it would have done")
	flag.Parse()

	db, err := database.New(database.Config{Driver: "sqlite3", FilePath: *dbPath})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	if err := db.ExecuteMigrationsFS(jimdottennis.Migrations(), "migrations built into this program"); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	source := bhplta.NewSource()
	source.BaseURL = strings.TrimRight(*baseURL, "/")
	b := &bhplta.Bootstrapper{DB: db, Source: source}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	log.Printf("Reading %s ...", services.BHPLTAFixturesURL(source.BaseURL))
	var copied copyCounts
	sum, err := b.Run(ctx, bhplta.Options{Weeks: *numWeeks}, func(step, total int, what string) {
		log.Printf("  [%d/%d] %s", step, total, what)
	}, func(ctx context.Context, tx *sqlx.Tx, sum *bhplta.Summary) error {
		if *copyPlayers && sum.SeasonID != 0 {
			c, err := copyFromPreviousSeason(ctx, tx, sum)
			if err != nil {
				return err
			}
			copied = c
		}
		if *dryRun {
			printSummary(sum, copied, true)
			return errDryRun
		}
		return nil
	})
	if errors.Is(err, errDryRun) {
		log.Println("Dry run: nothing was written.")
		return
	}
	if err != nil {
		log.Fatalf("Import failed, nothing was written: %s\n  (%v)", bhplta.Describe(err), err)
	}
	printSummary(sum, copied, false)
}

type copyCounts struct{ players, captains int64 }

// copyFromPreviousSeason copies active players and captains from last year's
// teams to this season's teams of the same name.
func copyFromPreviousSeason(ctx context.Context, tx *sqlx.Tx, sum *bhplta.Summary) (copyCounts, error) {
	var prevID uint
	err := tx.GetContext(ctx, &prevID, `SELECT id FROM seasons WHERE year = ? ORDER BY id LIMIT 1`, sum.SeasonYear-1)
	if errors.Is(err, sql.ErrNoRows) {
		return copyCounts{}, nil // no previous season: nothing to copy
	}
	if err != nil {
		return copyCounts{}, fmt.Errorf("find the previous season: %w", err)
	}
	now := time.Now()
	res, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO player_teams (player_id, team_id, season_id, is_active, created_at, updated_at)
		SELECT pt.player_id, nt.id, nt.season_id, TRUE, ?, ?
		FROM player_teams pt
		JOIN teams ot ON ot.id = pt.team_id
		JOIN teams nt ON nt.name = ot.name AND nt.season_id = ?
		JOIN players p ON p.id = pt.player_id AND p.is_active
		WHERE pt.season_id = ? AND ot.season_id = ?`, now, now, sum.SeasonID, prevID, prevID)
	if err != nil {
		return copyCounts{}, fmt.Errorf("copy players: %w", err)
	}
	var c copyCounts
	c.players, _ = res.RowsAffected()
	res, err = tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO captains (player_id, team_id, role, season_id, is_active, created_at, updated_at)
		SELECT ca.player_id, nt.id, ca.role, nt.season_id, TRUE, ?, ?
		FROM captains ca
		JOIN teams ot ON ot.id = ca.team_id
		JOIN teams nt ON nt.name = ot.name AND nt.season_id = ?
		JOIN players p ON p.id = ca.player_id AND p.is_active
		WHERE ca.season_id = ? AND ot.season_id = ?`, now, now, sum.SeasonID, prevID, prevID)
	if err != nil {
		return copyCounts{}, fmt.Errorf("copy captains: %w", err)
	}
	c.captains, _ = res.RowsAffected()
	return c, nil
}

func printSummary(sum *bhplta.Summary, copied copyCounts, dryRun bool) {
	verb := "Imported"
	if dryRun {
		verb = "Would import"
	}
	fmt.Printf("\n%s:\n", verb)
	fmt.Printf("  League:     %s (created: %v)\n", sum.LeagueName, sum.LeagueCreated)
	if sum.SeasonID != 0 {
		fmt.Printf("  Season:     %s (created: %v, active: %v)\n", sum.SeasonName, sum.SeasonCreated, sum.SeasonActivated)
		fmt.Printf("  Weeks:      %d created; fixtures in weeks %d-%d\n", sum.Weeks, sum.FirstWeek, sum.LastWeek)
		fmt.Printf("  Divisions:  %d\n  Teams:      %d\n  Fixtures:   %d\n", sum.Divisions, sum.Teams, sum.Fixtures)
		fmt.Printf("  Players:    %d copied, captains: %d copied\n", copied.players, copied.captains)
	}
	fmt.Printf("  Clubs:      %d created, %d matched, %d given venue details\n", sum.ClubsCreated, sum.ClubsMatched, sum.ClubsUpdated)
	for _, n := range sum.Notes {
		fmt.Printf("  Note: %s\n", n)
	}
	for _, u := range sum.Unmatched {
		fmt.Printf("  Unmatched: %s\n", u)
	}
}
