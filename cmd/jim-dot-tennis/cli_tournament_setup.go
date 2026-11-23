// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"jim-dot-tennis/internal/accounts"
	"jim-dot-tennis/internal/config"
	"jim-dot-tennis/internal/courthive"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/repository"
	"jim-dot-tennis/internal/tournaments"
)

// Where the bundled tournament stack is and how the app signs in to it. Read
// here only (tournamentClient), for the server and the command line alike.
const (
	envCourtHiveAPIURL          = "COURTHIVE_API_URL"
	envCourtHiveIdentityURL     = "COURTHIVE_IDENTITY_URL"
	envCourtHiveServicePassword = "COURTHIVE_SERVICE_PASSWORD"

	defaultCourtHiveAPIURL      = "http://courthive-server:8383"
	defaultCourtHiveIdentityURL = "http://courthive-hiveid:3140"
)

// tournamentConfig reads the tournament stack's settings from the
// environment. An empty password means the stack is not part of this
// deployment; the addresses keep their defaults (the calendar sync of an
// instance that points at CourtHive by hand uses COURTHIVE_API_URL).
func tournamentConfig() courthive.Config {
	return courthive.Config{
		ServerURL:       getEnv(envCourtHiveAPIURL, defaultCourtHiveAPIURL),
		IdentityURL:     getEnv(envCourtHiveIdentityURL, defaultCourtHiveIdentityURL),
		ServicePassword: os.Getenv(envCourtHiveServicePassword),
	}
}

// tournamentClient builds the one tournament client. A password that is set
// but too short, or an address that is not a URL, is an error that names the
// variables and never quotes the password.
func tournamentClient() (*courthive.Client, courthive.Config, error) {
	cfg := tournamentConfig()
	c, err := courthive.New(cfg)
	if err != nil {
		return nil, cfg, fmt.Errorf("the tournament settings cannot be used: %s. Check %s, %s and %s "+
			"(leave %s empty if this deployment has no bundled tournament stack)",
			strings.TrimPrefix(err.Error(), "courthive: "), envCourtHiveAPIURL, envCourtHiveIdentityURL,
			envCourtHiveServicePassword, envCourtHiveServicePassword)
	}
	return c, cfg, nil
}

// cliTournamentsHelp is printed after cliHelp by `jim-dot-tennis help`.
const cliTournamentsHelp = `
Tournaments
  tournaments status               Say whether the bundled tournament software
                                   is switched on in this deployment, whether
                                   the app can reach it, which version it runs,
                                   and whether the club is set up on it. Lists
                                   the club's sets of tournaments with their
                                   codes.
  tournaments provision [--abbreviation <code>]
                                   Set the club up on the bundled tournament
                                   software, as switching Tournaments on in
                                   Club Settings does: connect the app to it,
                                   make the club's place on it, make the
                                   permanent tournament administrator, and add
                                   the club's calendar to the app. Safe to run
                                   again: what already exists is left as it is,
                                   and a set-up that stopped part-way is
                                   finished. <code> is the club's tournament
                                   code (2 to 12 letters or digits); without
                                   it the one the app suggests is used. Once
                                   the club is set up the code cannot change.
                                   Lists tournament accounts the app did not
                                   make; nothing is changed for them.
  tournaments add-provider <code> <name>
                                   For a club that keeps more than one set of
                                   tournaments (for example one for the club
                                   and one for a league cup): add another
                                   beside the club's own. <code> is 2 to 12
                                   letters or digits and cannot change later;
                                   <name> is what people see when they choose
                                   which tournaments to manage. If the
                                   tournament server already has a set with
                                   that code, it is taken on as it is, with
                                   its tournaments. Nobody but administrators
                                   is given it: choose who with "user
                                   tournaments <username> <access> --providers
                                   <codes>" or on the users page. The club
                                   must be set up first. Safe to run again.
  tournaments pre-upgrade          Run by the deployment before the tournament
                                   server starts; you do not need to run it.
                                   When the tournament server image differs
                                   from the one last recorded, it makes a
                                   backup with the tournament data first, and
                                   fails (so the tournament server does not
                                   start) if it cannot. Nothing to do on an
                                   ordinary restart.
`

func runTournamentsCommand(ctx context.Context, c cliOut, db *database.DB, cmd command) error {
	client, cfg, err := tournamentClient()
	if err != nil {
		return err
	}
	svc := tournaments.NewService(db, client)
	if cmd.name == "tournaments status" {
		return tournamentsStatus(ctx, c, svc, cfg)
	}
	if cmd.name == cmdTournamentsAddProvider {
		return tournamentsAddProvider(ctx, c, svc, cmd.args[0], cmd.args[1])
	}
	return tournamentsProvision(ctx, c, db, svc, cmd.abbreviation)
}

func tournamentsAddProvider(ctx context.Context, c cliOut, svc *tournaments.Service, code, name string) error {
	res, err := svc.AddProvider(ctx, accounts.CLIActor(cmdTournamentsAddProvider), code, name)
	if res != nil {
		printSetupResult(c, res)
	}
	var stepErr *tournaments.StepError
	switch {
	case errors.As(err, &stepErr):
		return errors.New("adding the set of tournaments stopped part-way. Run the same command again to finish it; the steps already done are not repeated")
	case errors.Is(err, tournaments.ErrNotSetUp):
		return errors.New("the club is not set up on the tournament software yet. Run \"jim-dot-tennis tournaments provision\" first")
	case errors.Is(err, tournaments.ErrNotConfigured):
		return fmt.Errorf("the bundled tournament software is not switched on in this deployment (%s is empty)", envCourtHiveServicePassword)
	}
	return err
}

func tournamentsStatus(ctx context.Context, c cliOut, svc *tournaments.Service, cfg courthive.Config) error {
	st, err := svc.Status(ctx)
	if err != nil {
		return err
	}
	if !st.Configured {
		c.printf("Bundled tournament software: not switched on in this deployment (%s is empty).\n", envCourtHiveServicePassword)
	} else {
		c.printf("Bundled tournament software: switched on.\n")
		reachCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		version, err := svc.EngineVersion(reachCtx)
		cancel()
		if err != nil {
			c.printf("Tournament server (%s): cannot be reached: %v\n", cfg.ServerURL, err)
		} else {
			c.printf("Tournament server (%s): reachable, tournament engine %s.\n", cfg.ServerURL, version)
		}
	}
	switch {
	case st.SetUp:
		c.printf("Club set up: yes, with the tournament code %s.\n", st.Abbreviation)
	case st.Abbreviation != "":
		c.printf("Club set up: no (the last attempt used the code %s).\n", st.Abbreviation)
	default:
		c.printf("Club set up: no.\n")
	}
	if sets, err := svc.ManagedProviders(ctx); err == nil && len(sets) > 0 {
		c.printf("Sets of tournaments:\n")
		for _, p := range sets {
			own := ""
			if p.Own {
				own = " (the club's own)"
			}
			c.printf("  %-12s %s%s\n", p.Abbr, p.Name, own)
		}
	}
	if l := st.Last; l != nil {
		when := l.RanAt.Local().Format("2 Jan 2006 15:04")
		if l.OK {
			c.printf("Last set-up: %s by %s, finished.\n", when, l.By)
		} else {
			c.printf("Last set-up: %s by %s, stopped at %q: %s\n", when, l.By, l.FailedStep, l.Problem)
		}
	}
	return nil
}

func tournamentsProvision(ctx context.Context, c cliOut, db *database.DB, svc *tournaments.Service, abbreviation string) error {
	if !svc.Configured() {
		return fmt.Errorf("the bundled tournament software is not switched on in this deployment (%s is empty), so there is nothing to set up. "+
			"The operations guide's tournaments section says how to switch it on", envCourtHiveServicePassword)
	}
	clubName := config.DefaultDisplayName
	holder, err := config.NewHomeClubHolder(ctx, repository.NewClubRepository(db), repository.NewSettingsRepository(db))
	if err == nil {
		clubName = holder.ClubSettings().DisplayName(holder.Club())
		if club := holder.Club(); club != nil && club.Name != "" {
			clubName = club.Name
		}
	}
	if abbreviation == "" {
		if st, err := svc.Status(ctx); err == nil && !st.SetUp && st.Abbreviation == "" {
			abbreviation = svc.DefaultAbbreviation(ctx, clubName)
			c.printf("Using the suggested tournament code %s (give --abbreviation to choose another).\n", abbreviation)
		}
	}
	res, err := svc.Provision(ctx, accounts.CLIActor("tournaments provision"), tournaments.Request{Abbreviation: abbreviation, Name: clubName})
	if res != nil {
		printSetupResult(c, res)
	}
	var stepErr *tournaments.StepError
	if errors.As(err, &stepErr) {
		return errors.New("tournament set-up stopped part-way. Run \"jim-dot-tennis tournaments provision\" again to finish it; the steps already done are not repeated")
	}
	return err
}

func printSetupResult(c cliOut, res *tournaments.Result) {
	c.printf("\nTournament set-up for %s (%s)", res.Abbreviation, res.ProviderName)
	if res.EngineVersion != "" {
		c.printf(", tournament engine %s", res.EngineVersion)
	}
	c.printf(":\n")
	for _, s := range res.Steps {
		line := fmt.Sprintf("  %-45s %s", s.Label, s.OutcomeText())
		if s.Detail != "" {
			line += " (" + s.Detail + ")"
		}
		c.printf("%s\n", line)
	}
	if !res.OK {
		c.printf("\nStopped at: %s.\n%s\n", res.FailedStep, res.Problem)
		if res.Detail != "" {
			c.printf("Details: %s\n", res.Detail)
		}
	} else if res.Further && res.MadeAnything() {
		c.printf("\nDone. Nobody but administrators has been given it yet. Choose who can manage it with\n" +
			"    jim-dot-tennis user tournaments <username> <own|all> --providers <codes>\nor on the users page.\n")
	} else if res.MadeAnything() {
		c.printf("\nDone. Tournaments are ready to use.\n")
	} else {
		c.printf("\nDone. Everything was already in place; nothing was changed.\n")
	}
	if len(res.Unmanaged) > 0 {
		c.printf("\nTournament accounts the app did not make (nothing was changed for them):\n")
		for _, a := range res.Unmanaged {
			c.printf("  %s  %s\n", a.Email, a.RoleText())
		}
		c.printf("To let these people reach tournament management through the app, give each a\n" +
			"Tournaments only account on the users page, or run\n" +
			"    jim-dot-tennis user create <username> --role tournaments --tournaments own|all\n")
	}
}
