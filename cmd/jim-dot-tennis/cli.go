// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	jimdottennis "jim-dot-tennis"
	"jim-dot-tennis/internal/accounts"
	"jim-dot-tennis/internal/config"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/repository"
	"jim-dot-tennis/internal/setup"
)

// cliHelp is the text `jim-dot-tennis help` prints. docs/PRODUCTION_QUICK_REFERENCE.md
// reproduces it; keep the two in step.
const cliHelp = `jim-dot-tennis - the club's tennis app

Run it with no command to start the web app. That is what the app's
container does, so you never need to start it by hand.

The commands below are for the person who looks after the club's server.
Run them on the server, inside the app's container, for example:

    docker exec -it jim-dot-tennis ./jim-dot-tennis user list

They work while the app is running; there is no need to stop it.

Accounts
  user list                        Show every account: its role, whether it can
                                   sign in, and when it last signed in.
  user create <username> --role <role>
                                   Make a new account. <role> is admin (runs the
                                   instance) or captain (runs every team).
                                   Prints an invite link once; the person who
                                   opens it sets up their own passkey or
                                   password. The link works once, for 48 hours.
  user reset <username>            For someone who is locked out (even the last
                                   administrator): prints a recovery link once.
                                   Whoever opens it sets a new passkey or
                                   password for the account and is signed in;
                                   the account is then signed out everywhere
                                   else. The link works once, for 1 hour. Nothing
                                   changes for the account until it is used.
  user disable <username>          Stop an account signing in, and sign it out
                                   everywhere. Refused for the last
                                   administrator who can sign in.
  user enable <username>           Let a disabled account sign in again.
  user role <username> <role>      Change an account's role. Refused if it would
                                   leave no administrator who can sign in.
  user sessions revoke <username>  Sign an account out on every device.

Passkeys for administrators
  admin-passkeys status            Say whether administrators must sign in with
                                   a passkey, and which have none.
  admin-passkeys off               Stop requiring passkeys for administrators,
                                   so they can sign in with a password again
                                   (for when they are locked out).
  admin-passkeys on                Require them. Refused until every active
                                   administrator has a passkey.

First-run setup
  setup code                       Print a new setup code, for creating the first
                                   administrator in the browser. Only works while
                                   the app has no administrator yet. It cancels
                                   any earlier code (including the one in the
                                   app's log) and works once.

  help                             Show this text.

Good to know
  - No command asks for, accepts, shows or sets a password. "user create" and
    "user reset" print a link instead. There is no email: pass the link on
    yourself, privately (in person, by phone or in a private message, not in a
    group chat). Anyone who opens it first gets the account.
  - A new link for an account cancels that account's earlier unused links.
  - Links start with the app's public address (PUBLIC_URL, or the Public URL
    on the Club Settings page). If none is set, only the part after the
    address is printed; put the address people use to reach the app in front
    of it, for example https://tennis.example.org/account/link/....
  - Anyone who can run these commands already controls the server and the
    app's database file. The commands make recovery easier; they do not give
    anyone powers they did not already have.
  - Each command records what it did: the time, the command and the account.
  - If the app was upgraded but has not started since, the commands refuse to
    run until it has (starting the app upgrades the database).
`

// Exit codes for the command line.
const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

// errUsage marks a mistake in how a command was typed; the help is shown.
var errUsage = errors.New("usage")

// cliOut is where a command writes its results (problems go to stderr from
// runCLI; audit records go to stderr through the log package).
type cliOut struct {
	out io.Writer
}

func (c cliOut) printf(format string, a ...any) { fmt.Fprintf(c.out, format, a...) }

// runCLI runs the command in args (os.Args without the program name) and
// returns the process exit code.
func runCLI(args []string, stdout, stderr io.Writer) int {
	// Audit records (accounts.recordAudit) go to the operator's terminal.
	log.SetOutput(stderr)
	log.SetFlags(0)
	c := cliOut{out: stdout}

	cmd, err := parseCommand(args)
	if err != nil {
		if errors.Is(err, errUsage) {
			fmt.Fprintf(stderr, "%s\n\nRun \"jim-dot-tennis help\" to see the commands.\n", strings.TrimPrefix(err.Error(), errUsage.Error()+": "))
			return exitUsage
		}
		fmt.Fprintln(stderr, err)
		return exitUsage
	}
	if cmd.name == "help" {
		fmt.Fprint(stdout, cliHelp)
		return exitOK
	}

	ctx := context.Background()
	db, err := openCLIDatabase(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return exitError
	}
	defer db.Close()

	if err := cmd.run(ctx, c, db); err != nil {
		fmt.Fprintf(stderr, "Not done: %s\n", describeError(err))
		return exitError
	}
	return exitOK
}

// command is a parsed command line.
type command struct {
	name string   // "help", "user list", "user create", ...
	args []string // positional arguments after the command words
	role string   // --role
}

// parseCommand turns args into a command. Flags may come before or after
// the positional arguments.
func parseCommand(args []string) (command, error) {
	var words []string
	var cmd command
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") || a == "-" {
			words = append(words, a)
			continue
		}
		name, value, hasValue := strings.Cut(strings.TrimLeft(a, "-"), "=")
		switch strings.ToLower(name) {
		case "h", "help":
			return command{name: "help"}, nil
		case "role":
			if !hasValue {
				if i+1 >= len(args) {
					return command{}, fmt.Errorf("%w: --role needs a value: admin or captain", errUsage)
				}
				i++
				value = args[i]
			}
			cmd.role = value
		default:
			if strings.Contains(strings.ToLower(name), "pass") {
				return command{}, fmt.Errorf("%w: passwords are never given on the command line. "+
					"\"user create\" and \"user reset\" print a link; the person who opens it chooses how they sign in", errUsage)
			}
			return command{}, fmt.Errorf("%w: unknown option %q", errUsage, a)
		}
	}
	if len(words) == 0 {
		return command{name: "help"}, nil
	}

	need := func(n int, usage string) error {
		if len(cmd.args) != n {
			return fmt.Errorf("%w: usage: jim-dot-tennis %s", errUsage, usage)
		}
		return nil
	}
	noRole := func() error {
		if cmd.role != "" {
			return fmt.Errorf("%w: --role only goes with \"user create\" (to change a role use \"user role <username> <role>\")", errUsage)
		}
		return nil
	}

	switch strings.ToLower(words[0]) {
	case "help":
		return command{name: "help"}, nil
	case "setup":
		if len(words) != 2 || strings.ToLower(words[1]) != "code" {
			return command{}, fmt.Errorf("%w: usage: jim-dot-tennis setup code", errUsage)
		}
		cmd.name = "setup code"
		return cmd, noRole()
	case "admin-passkeys":
		if len(words) != 2 {
			return command{}, fmt.Errorf("%w: usage: jim-dot-tennis admin-passkeys status|on|off", errUsage)
		}
		switch sub := strings.ToLower(words[1]); sub {
		case "status", "on", "off":
			cmd.name = "admin-passkeys " + sub
			return cmd, noRole()
		}
		return command{}, fmt.Errorf("%w: usage: jim-dot-tennis admin-passkeys status|on|off", errUsage)
	case "user":
		if len(words) < 2 {
			return command{}, fmt.Errorf("%w: say what to do, for example: jim-dot-tennis user list", errUsage)
		}
		sub := strings.ToLower(words[1])
		cmd.args = words[2:]
		switch sub {
		case "list":
			cmd.name = "user list"
			if err := need(0, "user list"); err != nil {
				return command{}, err
			}
			return cmd, noRole()
		case "create":
			cmd.name = "user create"
			if err := need(1, "user create <username> --role <admin|captain>"); err != nil {
				return command{}, err
			}
			if cmd.role == "" {
				return command{}, fmt.Errorf("%w: say which role the new account has: --role admin or --role captain", errUsage)
			}
			if _, err := accounts.ParseRole(cmd.role); err != nil {
				return command{}, fmt.Errorf("%w: %v", errUsage, err)
			}
			return cmd, nil
		case "reset", "disable", "enable":
			cmd.name = "user " + sub
			if err := need(1, "user "+sub+" <username>"); err != nil {
				return command{}, err
			}
			return cmd, noRole()
		case "role":
			cmd.name = "user role"
			if err := need(2, "user role <username> <admin|captain>"); err != nil {
				return command{}, err
			}
			if _, err := accounts.ParseRole(cmd.args[1]); err != nil {
				return command{}, fmt.Errorf("%w: %v", errUsage, err)
			}
			return cmd, noRole()
		case "sessions":
			if len(cmd.args) == 0 || strings.ToLower(cmd.args[0]) != "revoke" {
				return command{}, fmt.Errorf("%w: usage: jim-dot-tennis user sessions revoke <username>", errUsage)
			}
			cmd.name = "user sessions revoke"
			cmd.args = cmd.args[1:]
			if err := need(1, "user sessions revoke <username>"); err != nil {
				return command{}, err
			}
			return cmd, noRole()
		}
		return command{}, fmt.Errorf("%w: unknown command \"user %s\"", errUsage, words[1])
	}
	return command{}, fmt.Errorf("%w: unknown command %q", errUsage, words[0])
}

// openCLIDatabase opens the database the server uses (DB_PATH, as the server
// resolves it) without creating it, migrating it or running the startup
// checks. It refuses a database whose schema is older than this program
// (the server upgrades it on start) or left half-migrated.
func openCLIDatabase(ctx context.Context) (*database.DB, error) {
	path := getEnv("DB_PATH", "./tennis.db")
	if err := database.RequireSQLite(getEnv("DB_TYPE", database.DriverSQLite)); err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("there is no database at %s. Run this command inside the app's container "+
			"(docker exec -it jim-dot-tennis ./jim-dot-tennis ...), where DB_PATH points at the app's database", path)
	}
	// database.New sets WAL and a busy timeout, so this works alongside the
	// running server: a write waits briefly for the server's instead of failing.
	db, err := setupDatabase()
	if err != nil {
		return nil, err
	}

	latest, err := database.LatestMigration(jimdottennis.Migrations())
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("cannot check the database version: %v", err)
	}
	st, err := db.ReadSchemaStatus(ctx)
	if err != nil {
		db.Close()
		return nil, err
	}
	switch {
	case !st.Migrated:
		db.Close()
		return nil, fmt.Errorf("the database at %s has not been set up yet. Start the app once, then try again", path)
	case st.Dirty:
		db.Close()
		return nil, fmt.Errorf("the database at %s is part-way through an upgrade (version %d) and needs attention before anything else changes it; see the migration notes in docs/PRODUCTION_QUICK_REFERENCE.md", path, st.Version)
	case st.Version < latest:
		db.Close()
		return nil, fmt.Errorf("the database is at version %d but this program needs version %d. Start (or restart) the app once so it upgrades the database, then try again", st.Version, latest)
	}
	return db, nil
}

func (cmd command) run(ctx context.Context, c cliOut, db *database.DB) error {
	svc := accounts.New(db)
	actor := accounts.CLIActor(cmd.name)
	switch cmd.name {
	case "user list":
		return userList(ctx, c, svc)
	case "user create":
		return userCreate(ctx, c, db, svc, actor, cmd.args[0], cmd.role)
	case "user reset":
		return userReset(ctx, c, db, svc, actor, cmd.args[0])
	case "user disable", "user enable":
		return userSetActive(ctx, c, svc, actor, cmd.args[0], cmd.name == "user enable")
	case "user role":
		return userRole(ctx, c, svc, actor, cmd.args[0], cmd.args[1])
	case "user sessions revoke":
		return userRevokeSessions(ctx, c, svc, actor, cmd.args[0])
	case "setup code":
		return setupCode(ctx, c, db, svc, actor)
	case "admin-passkeys status", "admin-passkeys on", "admin-passkeys off":
		return adminPasskeys(ctx, c, svc, actor, strings.TrimPrefix(cmd.name, "admin-passkeys "))
	}
	return fmt.Errorf("unknown command %q", cmd.name)
}

func userList(ctx context.Context, c cliOut, svc *accounts.Service) error {
	list, err := svc.List(ctx)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		c.printf("There are no accounts yet.\n")
		c.printf("If the app has not been set up, run \"jim-dot-tennis setup code\" and open the app in a browser.\n")
		return nil
	}
	tw := tabwriter.NewWriter(c.out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "USERNAME\tROLE\tCAN SIGN IN\tLAST SIGNED IN\tSIGNED IN ON\tNOTE")
	admins := 0
	for _, a := range list {
		if a.IsActiveAdmin() {
			admins++
		}
		status := "yes"
		if !a.IsActive {
			status = "no (disabled)"
		}
		last := a.LastLoginAt.Local().Format("2 Jan 2006 15:04")
		if !a.LastLoginAt.After(a.CreatedAt.Add(time.Second)) {
			last = "never"
		}
		note := ""
		if a.SignInMethods() == 0 {
			note = "no way to sign in yet (invite not used)"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d device(s)\t%s\n", a.Username, a.Role, status, last, a.ActiveSessions, note)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	c.printf("\n%d account(s), %d active administrator(s).\n", len(list), admins)
	if admins == 0 {
		c.printf("Nobody can administer the app. Make an account an administrator with\n" +
			"\"jim-dot-tennis user role <username> admin\" (and \"user enable <username>\" if it is disabled).\n")
	}
	return nil
}

func userCreate(ctx context.Context, c cliOut, db *database.DB, svc *accounts.Service, actor accounts.Actor, username, roleName string) error {
	role, err := accounts.ParseRole(roleName)
	if err != nil {
		return err
	}
	a, link, err := svc.Invite(ctx, actor, username, role, nil)
	if err != nil {
		return err
	}
	c.printf("Created the account %q (%s). It has no password or passkey yet.\n\n", a.Username, a.Role)
	printLink(ctx, c, db, link)
	c.printf("Give it to the person who will use %q. When they open it they set up their own\n"+
		"passkey or password and are signed in.\n", a.Username)
	return nil
}

func userReset(ctx context.Context, c cliOut, db *database.DB, svc *accounts.Service, actor accounts.Actor, username string) error {
	a, err := svc.Find(ctx, username)
	if err != nil {
		return err
	}
	link, err := svc.IssueLink(ctx, actor, a.ID, accounts.LinkRecovery)
	if err != nil {
		return err
	}
	c.printf("Made a recovery link for %q (%s). Nothing has changed for the account yet.\n\n", a.Username, a.Role)
	printLink(ctx, c, db, link)
	c.printf("Give it to the person who uses %q. When they open it they set a new passkey or\n"+
		"password and are signed in, and the account is signed out everywhere else.\n"+
		"On that page they can also choose to remove the account's old passkeys and password.\n", a.Username)
	if !a.IsActive {
		c.printf("\nThis account is disabled, so the link will not work yet. To let it, run:\n    jim-dot-tennis user enable %s\n", a.Username)
	}
	return nil
}

// printLink prints a link that was just made, once. With a public URL
// configured it is the full address; without one only the path is printed,
// with how to complete it (a command has no request to take a host from, and
// guessing one could send the person somewhere else).
func printLink(ctx context.Context, c cliOut, db *database.DB, link accounts.IssuedLink) {
	kind, lifetime := "Recovery", "1 hour"
	if link.Kind == accounts.LinkInvite {
		kind, lifetime = "Invite", "48 hours"
	}
	base, err := publicBaseURL(ctx, db)
	switch {
	case base != "":
		c.printf("%s link (shown only this once):\n\n    %s%s\n\n", kind, base, link.Path())
	default:
		c.printf("%s link (shown only this once). This app has no public address set, so this is\n"+
			"only the end of the link:\n\n    %s\n\n", kind, link.Path())
		c.printf("Put the address people use to reach the app in front of it, for example\n"+
			"    https://tennis.example.org%s\n"+
			"To have the whole link printed next time, set PUBLIC_URL in the app's environment\n"+
			"(or the Public URL on the Club Settings page).\n", link.Path())
		if err != nil {
			c.printf("(The public address that is set could not be used: %v)\n", err)
		}
		c.printf("\n")
	}
	c.printf("It works once and stops working after %s (%s).\n", lifetime, link.ExpiresAt.Local().Format("2 Jan 2006 15:04"))
	if link.Superseded > 0 {
		c.printf("An earlier unused link for this account was cancelled.\n")
	}
	c.printf("There is no email: pass it on yourself, privately. Whoever opens it first gets the account.\n")
}

// publicBaseURL is the app's configured public address (PUBLIC_URL, else the
// one stored on the settings page), or "" when none is set or it is invalid.
func publicBaseURL(ctx context.Context, db *database.DB) (string, error) {
	p, ok, err := config.NewClubSettings(ctx, repository.NewSettingsRepository(db)).ResolvePublicURL()
	if err != nil || !ok {
		return "", err
	}
	return p.Base, nil
}

func userSetActive(ctx context.Context, c cliOut, svc *accounts.Service, actor accounts.Actor, username string, active bool) error {
	a, err := svc.Find(ctx, username)
	if err != nil {
		return err
	}
	if a.IsActive == active {
		if active {
			c.printf("%q can already sign in; nothing changed.\n", a.Username)
		} else {
			c.printf("%q is already disabled; nothing changed.\n", a.Username)
		}
		return nil
	}
	after, err := svc.SetActive(ctx, actor, a.ID, active)
	if err != nil {
		return err
	}
	if active {
		c.printf("Enabled %q (%s). It can sign in again.\n", after.Username, after.Role)
		return nil
	}
	c.printf("Disabled %q (%s). It can no longer sign in and was signed out of %d session(s).\n", after.Username, after.Role, a.ActiveSessions)
	return nil
}

func userRole(ctx context.Context, c cliOut, svc *accounts.Service, actor accounts.Actor, username, roleName string) error {
	role, err := accounts.ParseRole(roleName)
	if err != nil {
		return err
	}
	a, err := svc.Find(ctx, username)
	if err != nil {
		return err
	}
	if a.Role == role {
		c.printf("%q is already %s; nothing changed.\n", a.Username, role)
		return nil
	}
	previous, err := svc.SetRole(ctx, actor, a.ID, role)
	if err != nil {
		return err
	}
	c.printf("Changed %q from %s to %s.\n", a.Username, previous, role)
	if !a.IsActive {
		c.printf("This account is disabled. To let it sign in, run:\n    jim-dot-tennis user enable %s\n", a.Username)
	}
	return nil
}

func userRevokeSessions(ctx context.Context, c cliOut, svc *accounts.Service, actor accounts.Actor, username string) error {
	a, err := svc.Find(ctx, username)
	if err != nil {
		return err
	}
	ended, err := svc.RevokeSessions(ctx, actor, a.ID)
	if err != nil {
		return err
	}
	c.printf("Signed %q out of %d session(s). They will need to sign in again on every device.\n", a.Username, ended)
	return nil
}

// setupCode issues a new first-run setup code through the same code as
// server startup (setup.Service.IssueCode): only while the instance has no
// administrator, only the hash is stored, and it replaces any earlier code
// and any browser that had redeemed one. The running server checks codes
// against the database, so the new code works without a restart; its
// wrong-code throttle is in the server's memory and is not reset from here.
func setupCode(ctx context.Context, c cliOut, db *database.DB, svc *accounts.Service, actor accounts.Actor) error {
	holder, err := config.NewHomeClubHolder(ctx, repository.NewClubRepository(db), repository.NewSettingsRepository(db))
	if err != nil {
		return fmt.Errorf("home club configuration: %w", err)
	}
	state := setup.NewState(db, holder)
	status := state.Status(ctx)
	if !status.NeedsFirstAdmin() {
		c.printf("No setup code is needed: this app already has an administrator account.\n" +
			"If nobody can sign in as an administrator, see who can with \"jim-dot-tennis user list\"\n" +
			"and use \"jim-dot-tennis user reset <username>\" (or \"user enable\") instead.\n")
		return nil
	}
	code, err := setup.NewService(state, nil).IssueCode(ctx)
	if err != nil {
		return err
	}
	if code == "" {
		c.printf("No setup code is needed: this app already has an administrator account.\n")
		return nil
	}
	if err := svc.RecordEvent(ctx, actor, accounts.ActionSetupCode, "status "+status.String()); err != nil {
		return err
	}
	c.printf("Setup code (shown only this once):\n\n    %s\n\n", code)
	c.printf("Open the app in a browser and enter this code to create the first administrator.\n" +
		"It works once. It replaces any earlier setup code, including the one in the app's log,\n" +
		"and anyone half-way through setup with an earlier code has to start again.\n")
	return nil
}

// describeError turns an error from a command into a sentence for the
// operator.
func describeError(err error) string {
	if errors.Is(err, accounts.ErrNotFound) {
		return "there is no account with that username. \"jim-dot-tennis user list\" shows them all."
	}
	return err.Error()
}
