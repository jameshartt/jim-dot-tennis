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
	"jim-dot-tennis/internal/backup"
	"jim-dot-tennis/internal/config"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
	"jim-dot-tennis/internal/setup"
	"jim-dot-tennis/internal/tournaments"
	"jim-dot-tennis/internal/version"
)

// cliHelp is the text `jim-dot-tennis help` prints. The operator guides
// (docs/accounts-and-recovery.md, docs/backup-and-restore.md, docs/upgrade.md)
// show how to run every command it lists; TestOperatorGuidesCoverEveryCommand
// checks that each listed command appears in one of them.
const cliHelp = `jim-dot-tennis - the club's tennis app

Run it with no command to start the web app. That is what the app's
container does, so you never need to start it by hand.

The commands below are for the person who looks after the club's server.
Run them on the server, inside the app's container. From the folder
holding docker-compose.yml, for example:

    docker compose exec app ./jim-dot-tennis user list

They work while the app is running; there is no need to stop it. The one
exception is "restore", which only runs while the app is stopped.

Accounts
  user list                        Show every account: its role, its tournament
                                   access, whether it can sign in, and when it
                                   last signed in.
  user create <username> --role <role> [--tournaments <access>] [--providers <codes>]
                                   Make a new account. <role> is admin (runs the
                                   instance), captain (runs every team) or
                                   tournaments (Tournaments only: runs the
                                   club's tournaments and nothing else).
                                   <access> is own (their own tournaments and
                                   any they are given) or all (every
                                   tournament). A Tournaments only account
                                   needs one; a captain has none unless you
                                   give it; administrators always have all.
                                   <codes> is for a club that keeps more than
                                   one set of tournaments ("tournaments
                                   status" lists them): the codes, separated
                                   by commas, of the sets the person can
                                   manage. Without it they get the club's own.
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
  user tournaments <username> <access> [--providers <codes>]
                                   Change an account's tournament access: none,
                                   own or all. Not for administrators (they
                                   always have all). A Tournaments only account
                                   keeps own or all; to stop it, disable it.
                                   With --providers, also choose which sets of
                                   tournaments the person can manage (codes
                                   separated by commas), replacing the earlier
                                   choice.
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

Backups
  backup                           Make a backup now: the database and the
                                   uploaded club logo in one file, in the
                                   backups folder (BACKUP_DIR; normally
                                   backups/ next to the database). Safe while
                                   the app is running. The app also makes one
                                   on its own every day (BACKUP_INTERVAL) and
                                   before every upgrade, and keeps the newest
                                   14 (BACKUP_KEEP). With the tournament stack
                                   on, the file also holds the tournament
                                   database; if that cannot be included, no
                                   backup is made and the command says why.
  restore <file>                   Put a backup back: its database and logo
                                   replace the current ones. Refused while the
                                   app is running, so stop the app and run it
                                   in a one-off container (see
                                   docs/backup-and-restore.md, "Put a backup
                                   back (restore)"). A backup from an older
                                   version is upgraded; one from a newer
                                   version is refused. The database it replaces
                                   is kept beside it (tennis.db.replaced-...),
                                   not deleted: remove those copies yourself
                                   once the restore is confirmed good. While
                                   it runs, the other commands refuse.
                                   Everyone has to sign in again afterwards.
                                   A backup with tournament data restores the
                                   tournament database too (stop the
                                   tournament server and identity service
                                   first; a copy of the database it replaces
                                   is kept as courthive.dump.replaced-...).
  restore --tournaments-only <file>
                                   Put back only the tournament data of a
                                   backup (or a courthive.dump.replaced-...
                                   copy). The app's database is not touched.

  version                          Show which version of the app this is, the
                                   commit it was built from and when.
                                   Needs no database.

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
    run until it has (starting the app upgrades the database). "backup" and
    "restore" are the exceptions.
  - A backup holds everyone's personal details, password hashes and passkey
    details, and the app's own secrets. Keep backups where only the people who
    run the app can read them, and keep a copy off this server: a backup on
    the same disk is lost with the server.
`

// Exit codes for the command line.
const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

// errUsage marks a mistake in how a command was typed; the help is shown.
// cmdVersion is the command that prints the build identity; it needs no database.
const cmdVersion = "version"

// cmdBackup and cmdRestore make and restore backups (cli_backup.go); they
// open the database themselves.
const (
	cmdBackup  = "backup"
	cmdRestore = "restore"
)

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
		fmt.Fprint(stdout, cliHelp+cliTournamentsHelp)
		return exitOK
	}

	if cmd.name == cmdVersion {
		fmt.Fprintf(stdout, "jim-dot-tennis %s\n", version.Long())
		return exitOK
	}

	ctx := context.Background()
	if cmd.name == cmdRestore {
		if cmd.tournamentsOnly {
			return runRestoreTournaments(ctx, c, stderr, cmd.args[0])
		}
		return runRestore(ctx, c, stderr, cmd.args[0])
	}
	// Every other command uses the database: not while a restore is
	// replacing it.
	release, ok := holdCommandLock()
	if !ok {
		fmt.Fprintf(stderr, "Not done: a restore is in progress in %s. Run this command again once the restore has finished and the app has started.\n",
			backup.Config{DBPath: getEnv("DB_PATH", "./tennis.db")}.DataDir())
		return exitError
	}
	defer release()
	if cmd.name == cmdBackup {
		return runBackup(ctx, c, stderr)
	}
	if cmd.name == cmdTournamentsPreUpgrade {
		return runTournamentsPreUpgrade(ctx, c, stderr)
	}
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
	// tournaments is --tournaments (user create only).
	tournaments string
	// providers is --providers (user create and user tournaments): the
	// codes of the sets of tournaments chosen for the person.
	providers string
	// abbreviation is --abbreviation (tournaments provision only).
	abbreviation string
	// tournamentsOnly is --tournaments-only (restore only).
	tournamentsOnly bool
}

// parseCommand turns args into a command. Flags may come before or after
// the positional arguments.
func parseCommand(args []string) (command, error) {
	cmd, err := parseCommandWords(args)
	if err == nil && cmd.abbreviation != "" && cmd.name != cmdTournamentsProvision {
		return command{}, fmt.Errorf("%w: --abbreviation only goes with \"tournaments provision\"", errUsage)
	}
	if err == nil && cmd.providers != "" && cmd.name != "user create" && cmd.name != "user tournaments" {
		return command{}, fmt.Errorf("%w: --providers only goes with \"user create\" and \"user tournaments\"", errUsage)
	}
	return cmd, err
}

// The tournament set-up commands (cli_tournament_setup.go).
const (
	cmdTournamentsStatus    = "tournaments status"
	cmdTournamentsProvision = "tournaments provision"
	// cmdTournamentsAddProvider takes on a further set of tournaments.
	cmdTournamentsAddProvider = "tournaments add-provider"
	// cmdTournamentsPreUpgrade is run by the deployment's one-shot service
	// before a new tournament server image starts (cli_tournament_upgrade.go).
	cmdTournamentsPreUpgrade = "tournaments pre-upgrade"
)

func parseCommandWords(args []string) (command, error) {
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
		case cmdVersion:
			return command{name: cmdVersion}, nil
		case "role":
			if !hasValue {
				if i+1 >= len(args) {
					return command{}, fmt.Errorf("%w: --role needs a value: admin or captain", errUsage)
				}
				i++
				value = args[i]
			}
			cmd.role = value
		case "tournaments":
			if !hasValue {
				if i+1 >= len(args) {
					return command{}, fmt.Errorf("%w: --tournaments needs a value: own or all", errUsage)
				}
				i++
				value = args[i]
			}
			cmd.tournaments = value
		case "providers":
			if !hasValue {
				if i+1 >= len(args) {
					return command{}, fmt.Errorf("%w: --providers needs a value: the codes of the sets of tournaments, separated by commas", errUsage)
				}
				i++
				value = args[i]
			}
			if strings.TrimSpace(value) == "" {
				return command{}, fmt.Errorf("%w: --providers needs at least one code", errUsage)
			}
			cmd.providers = value
		case "tournaments-only":
			if hasValue {
				return command{}, fmt.Errorf("%w: --tournaments-only takes no value", errUsage)
			}
			cmd.tournamentsOnly = true
		case "abbreviation":
			if !hasValue {
				if i+1 >= len(args) {
					return command{}, fmt.Errorf("%w: --abbreviation needs a value: the club's tournament code, for example STANNS", errUsage)
				}
				i++
				value = args[i]
			}
			cmd.abbreviation = value
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
	if cmd.tournamentsOnly && strings.ToLower(words[0]) != cmdRestore {
		return command{}, fmt.Errorf("%w: --tournaments-only only goes with \"restore\"", errUsage)
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
		if cmd.tournaments != "" {
			return fmt.Errorf("%w: --tournaments only goes with \"user create\" (to change it use \"user tournaments <username> <access>\")", errUsage)
		}
		return nil
	}

	switch strings.ToLower(words[0]) {
	case "help":
		return command{name: "help"}, nil
	case cmdVersion:
		if len(words) != 1 {
			return command{}, fmt.Errorf("%w: usage: jim-dot-tennis version", errUsage)
		}
		return command{name: cmdVersion}, noRole()
	case "setup":
		if len(words) != 2 || strings.ToLower(words[1]) != "code" {
			return command{}, fmt.Errorf("%w: usage: jim-dot-tennis setup code", errUsage)
		}
		cmd.name = "setup code"
		return cmd, noRole()
	case cmdBackup:
		if len(words) != 1 {
			return command{}, fmt.Errorf("%w: usage: jim-dot-tennis backup", errUsage)
		}
		return command{name: cmdBackup}, noRole()
	case cmdRestore:
		if len(words) != 2 {
			return command{}, fmt.Errorf("%w: usage: jim-dot-tennis restore <backup file>", errUsage)
		}
		return command{name: cmdRestore, args: words[1:], tournamentsOnly: cmd.tournamentsOnly}, noRole()
	case "tournaments":
		usage := fmt.Errorf("%w: usage: jim-dot-tennis tournaments status|provision [--abbreviation <code>]|add-provider <code> <name>|pre-upgrade", errUsage)
		if len(words) >= 2 && strings.ToLower(words[1]) == "add-provider" {
			if len(words) < 4 {
				return command{}, fmt.Errorf("%w: usage: jim-dot-tennis tournaments add-provider <code> <name>", errUsage)
			}
			cmd.name = cmdTournamentsAddProvider
			cmd.args = []string{words[2], strings.Join(words[3:], " ")}
			return cmd, noRole()
		}
		if len(words) != 2 {
			return command{}, usage
		}
		switch sub := strings.ToLower(words[1]); sub {
		case "status", "provision", "pre-upgrade":
			cmd.name = "tournaments " + sub
			return cmd, noRole()
		}
		return command{}, usage
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
			if err := need(1, "user create <username> --role <admin|captain|tournaments> [--tournaments <own|all>]"); err != nil {
				return command{}, err
			}
			if cmd.role == "" {
				return command{}, fmt.Errorf("%w: say which role the new account has: --role admin, --role captain or --role tournaments", errUsage)
			}
			role, err := accounts.ParseAccountType(cmd.role)
			if err != nil {
				return command{}, fmt.Errorf("%w: the role must be admin, captain or tournaments", errUsage)
			}
			access := models.TournamentAccessNone
			if cmd.tournaments != "" {
				if access, err = accounts.ParseTournamentAccess(cmd.tournaments); err != nil {
					return command{}, fmt.Errorf("%w: --tournaments must be own or all", errUsage)
				}
			}
			switch {
			case role == models.RoleTournamentsOnly && access == models.TournamentAccessNone:
				return command{}, fmt.Errorf("%w: a Tournaments only account needs --tournaments own or --tournaments all", errUsage)
			case role == models.RoleAdmin && cmd.tournaments != "":
				return command{}, fmt.Errorf("%w: administrators always have every tournament: leave out --tournaments", errUsage)
			case role == models.RoleAdmin && cmd.providers != "":
				return command{}, fmt.Errorf("%w: administrators always have every set of tournaments: leave out --providers", errUsage)
			case cmd.providers != "" && access == models.TournamentAccessNone:
				return command{}, fmt.Errorf("%w: --providers needs --tournaments own or --tournaments all", errUsage)
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
		case "tournaments":
			cmd.name = "user tournaments"
			if err := need(2, "user tournaments <username> <none|own|all>"); err != nil {
				return command{}, err
			}
			access, err := accounts.ParseTournamentAccess(cmd.args[1])
			if err != nil {
				return command{}, fmt.Errorf("%w: %v", errUsage, err)
			}
			if cmd.providers != "" && access == models.TournamentAccessNone {
				return command{}, fmt.Errorf("%w: --providers goes with own or all, not none", errUsage)
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

// holdCommandLock takes the data folder's restore lock shared
// (backup.AcquireCommandLock) for as long as the command runs, and returns
// how to give it up; ok is false while a restore holds it. Then the command
// is refused,
// rather than writing to a database that is about to be replaced; the
// running server does not hold it, so the commands still work alongside the
// server. Where the lock file cannot be opened at all (no data folder yet)
// the command goes on and reports what is really wrong (no database).
func holdCommandLock() (release func(), ok bool) {
	cfg := backup.Config{DBPath: getEnv("DB_PATH", "./tennis.db")}
	lock, err := backup.AcquireCommandLock(cfg.RestoreLockPath())
	if errors.Is(err, backup.ErrLocked) {
		return nil, false
	}
	if err != nil {
		return func() {}, true
	}
	return lock.Release, true
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
			"(from the folder holding docker-compose.yml: docker compose exec app ./jim-dot-tennis ...), where DB_PATH points at the app's database", path)
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
		return nil, fmt.Errorf("the database at %s is part-way through an upgrade (version %d) and needs attention before anything else changes it; see \"The app keeps restarting after an upgrade\" in docs/troubleshooting.md", path, st.Version)
	case st.Version > latest:
		db.Close()
		return nil, &database.NewerSchemaError{Found: st.Version, Latest: latest}
	case st.Version < latest:
		db.Close()
		return nil, fmt.Errorf("the database is at version %d but this program needs version %d. Start (or restart) the app once so it upgrades the database, then try again", st.Version, latest)
	}
	return db, nil
}

func (cmd command) run(ctx context.Context, c cliOut, db *database.DB) error {
	svc := accounts.New(db)
	// Changes made here reach the bundled tournament stack too, when this
	// deployment has one and its settings are usable.
	if client, _, err := tournamentClient(); err == nil {
		svc.SetTournamentAccessObserver(tournamentAccessObserver(tournaments.NewService(db, client)))
	}
	actor := accounts.CLIActor(cmd.name)
	switch cmd.name {
	case "user list":
		return userList(ctx, c, svc)
	case "user create":
		return userCreate(ctx, c, db, svc, actor, cmd.args[0], cmd.role, cmd.tournaments, cmd.providers)
	case "user reset":
		return userReset(ctx, c, db, svc, actor, cmd.args[0])
	case "user disable", "user enable":
		return userSetActive(ctx, c, svc, actor, cmd.args[0], cmd.name == "user enable")
	case "user role":
		return userRole(ctx, c, svc, actor, cmd.args[0], cmd.args[1])
	case "user tournaments":
		return userTournaments(ctx, c, svc, actor, cmd.args[0], cmd.args[1], cmd.providers)
	case "user sessions revoke":
		return userRevokeSessions(ctx, c, svc, actor, cmd.args[0])
	case "setup code":
		return setupCode(ctx, c, db, svc, actor)
	case "admin-passkeys status", "admin-passkeys on", "admin-passkeys off":
		return adminPasskeys(ctx, c, svc, actor, strings.TrimPrefix(cmd.name, "admin-passkeys "))
	case cmdTournamentsStatus, cmdTournamentsProvision, cmdTournamentsAddProvider:
		return runTournamentsCommand(ctx, c, db, cmd)
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
	fmt.Fprintln(tw, "USERNAME\tROLE\tTOURNAMENTS\tCAN SIGN IN\tLAST SIGNED IN\tSIGNED IN ON\tNOTE")
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
		tournaments := string(a.TournamentAccess)
		if a.Role == models.RoleAdmin {
			tournaments = "all (administrator)"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d device(s)\t%s\n", a.Username, a.RoleLabel(), tournaments, status, last, a.ActiveSessions, note)
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

func userCreate(ctx context.Context, c cliOut, db *database.DB, svc *accounts.Service, actor accounts.Actor, username, roleName, accessName, providerCodes string) error {
	role, err := accounts.ParseAccountType(roleName)
	if err != nil {
		return err
	}
	access := models.TournamentAccessNone
	if accessName != "" {
		if access, err = accounts.ParseTournamentAccess(accessName); err != nil {
			return err
		}
	}
	var (
		a    accounts.Account
		link accounts.IssuedLink
	)
	var sets []uint
	if providerCodes != "" {
		if sets, err = svc.TournamentProviderIDs(ctx, strings.Split(providerCodes, ",")); err != nil {
			return err
		}
	}
	if role != models.RoleTournamentsOnly && access == models.TournamentAccessNone {
		// A plain administrator or captain.
		a, link, err = svc.Invite(ctx, actor, username, role, nil)
	} else {
		a, link, err = svc.InviteAccount(ctx, actor, accounts.NewAccount{Username: username, Role: role, TournamentAccess: access, TournamentProviders: sets})
	}
	if err != nil {
		return err
	}
	kind := a.RoleLabel()
	if a.TournamentAccess != models.TournamentAccessNone {
		kind += ", " + strings.ToLower(a.TournamentAccess.Label())
	}
	if len(sets) > 0 {
		kind += ", in " + strings.ToUpper(strings.ReplaceAll(providerCodes, " ", ""))
	}
	c.printf("Created the account %q (%s). It has no password or passkey yet.\n\n", a.Username, kind)
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

func userTournaments(ctx context.Context, c cliOut, svc *accounts.Service, actor accounts.Actor, username, accessName, providerCodes string) error {
	access, err := accounts.ParseTournamentAccess(accessName)
	if err != nil {
		return err
	}
	a, err := svc.Find(ctx, username)
	if err != nil {
		return err
	}
	var sets []uint
	if providerCodes != "" {
		// Checked before anything changes, so a wrong code changes nothing.
		if a.Role == models.RoleAdmin {
			return accounts.ErrAdminTournamentProviders
		}
		if sets, err = svc.TournamentProviderIDs(ctx, strings.Split(providerCodes, ",")); err != nil {
			return err
		}
	}
	if a.Role != models.RoleAdmin && a.TournamentAccess == access {
		c.printf("%q already has %s; nothing changed.\n", a.Username, strings.ToLower(access.Label()))
	} else {
		previous, err := svc.SetTournamentAccess(ctx, actor, a.ID, access)
		if err != nil {
			return err
		}
		c.printf("Changed the tournament access of %q from %s to %s.\n", a.Username, previous, access)
	}
	if len(sets) > 0 {
		if err := svc.SetTournamentProviders(ctx, actor, a.ID, sets); err != nil {
			return err
		}
		c.printf("%q can now manage these sets of tournaments: %s.\n", a.Username, strings.ToUpper(strings.ReplaceAll(providerCodes, " ", "")))
	}
	if !a.IsActive {
		c.printf("This account is disabled, so it cannot use tournaments until it is enabled:\n    jim-dot-tennis user enable %s\n", a.Username)
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
