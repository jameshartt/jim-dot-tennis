// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package main

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	jimdottennis "jim-dot-tennis"
	"jim-dot-tennis/internal/accounts"
	"jim-dot-tennis/internal/admin"
	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/bhplta"
	"jim-dot-tennis/internal/branding"
	"jim-dot-tennis/internal/config"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/fixturegen"
	"jim-dot-tennis/internal/health"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/players"
	"jim-dot-tennis/internal/render"
	"jim-dot-tennis/internal/repository"
	"jim-dot-tennis/internal/secrets"
	"jim-dot-tennis/internal/setup"
	"jim-dot-tennis/internal/version"
	"jim-dot-tennis/internal/webpush"
)

func main() {
	// With arguments this is the server command line (`user ...`, `setup
	// code`, `help`; see cli.go). With none it starts the server, as always.
	if len(os.Args) > 1 {
		os.Exit(runCLI(os.Args[1:], os.Stdout, os.Stderr))
	}

	// Get project root directory
	projectRoot, err := getProjectRoot()
	if err != nil {
		log.Fatalf("Failed to determine project root: %v", err)
	}

	// Say what is wrong with the settings in plain words before anything opens.
	if err := checkEnvironment(); err != nil {
		log.Fatalf("Cannot start: %v", err)
	}

	// Initialize database
	db, err := setupDatabase()
	if err != nil {
		log.Fatalf("Failed to set up database: %v", err)
	}
	defer db.Close()

	// Execute migrations (always the copies built into this binary)
	migrateOrStop(db, jimdottennis.Migrations(), "migrations built into this binary", log.Fatalf)

	// Templates and static files: built in, or from disk with TEMPLATE_RELOAD
	files := loadFiles(projectRoot)

	// Load the embedded fantasy name list on first run (also in setup mode).
	if n, err := repository.NewProTennisPlayerRepository(db).SeedIfEmpty(context.Background()); err != nil {
		log.Fatalf("Failed to load tennis players: %v", err)
	} else if n > 0 {
		log.Printf("Loaded %d tennis players into an empty tennis_players table", n)
	}

	// Instance secrets: refuse known placeholders in production and make sure
	// this instance has its own generated Club Wrapped access code.
	if err := secrets.StartupCheck(context.Background(), db); err != nil {
		log.Fatalf("Startup self-check failed: %v", err)
	}

	// Resolve the home club (HOME_CLUB_ID, then HOME_CLUB_NAME, then the stored
	// setting). With nothing configured the server still starts, in setup mode.
	clubRepo := repository.NewClubRepository(db)
	homeClub, err := config.NewHomeClubHolder(context.Background(), clubRepo, repository.NewSettingsRepository(db))
	if err != nil {
		log.Fatalf("Home club configuration error: %v", err)
	}
	// Production needs a valid https public URL (and no DEV_MODE).
	if err := config.ValidateStartup(homeClub.ClubSettings()); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}
	setupState := setup.NewState(db, homeClub)
	// The club step imports the league, clubs and current season from BHPLTA.
	setupService := setup.NewService(setupState, setup.NewBHPLTAProvisioner(bhplta.NewBootstrapper(db)))
	if code, err := setupService.IssueCode(context.Background()); err != nil {
		log.Printf("Warning: could not issue a setup code: %v", err)
	} else if code != "" {
		setup.LogCode(code)
	}

	// Initialize web push service
	pushService := webpush.New(db)
	pushService.SetSubscriberSource(func() string { return homeClub.ClubSettings().PushSubscriber() })

	// Generate VAPID keys on startup if none exist. No key material is logged.
	if _, _, err := pushService.GenerateVAPIDKeys(); err != nil {
		log.Printf("Warning: Failed to generate VAPID keys: %v", err)
	}

	// Clean up push subscriptions older than 90 days on startup
	if removed, err := pushService.CleanupStaleSubscriptions(90 * 24 * time.Hour); err != nil {
		log.Printf("Warning: Failed to clean up stale push subscriptions: %v", err)
	} else if removed > 0 {
		log.Printf("Cleaned up %d stale push subscriptions", removed)
	}

	// Set up auth service
	authConfig := auth.DefaultConfig()
	// Cookies are Secure unless DEV_MODE is explicitly enabled
	authConfig.CookieSecure = config.SecureCookies()
	authService := auth.NewService(db, authConfig)

	// Set up repositories for fantasy token auth
	playerRepo := repository.NewPlayerRepository(db)
	fantasyMatchRepo := repository.NewFantasyMixedDoublesRepository(db)

	// Set up auth middleware
	authMiddleware := auth.NewMiddleware(authService, playerRepo, fantasyMatchRepo)

	// Set up auth handlers
	templateDir := files.templateDir
	authHandler := auth.NewHandler(authService, templateDir, "/admin/league")

	// Set up admin handlers
	courthiveAPIURL := os.Getenv("COURTHIVE_API_URL")
	if courthiveAPIURL == "" {
		courthiveAPIURL = "http://courthive-server:8383"
	}
	adminHandler := admin.New(db, templateDir, courthiveAPIURL, homeClub, homeClub.BHPLTAClubCode(), pushService)

	// Set up players handlers
	playersHandler := players.New(db, templateDir, homeClub)

	// Set up template functions
	templateFuncs := template.FuncMap{
		"currentYear": func() int {
			return time.Now().Year()
		},
		"appVersion": version.Short,
	}

	// Load templates with functions
	templates, err := loadTemplatesWithFuncs(templateDir, templateFuncs)
	if err != nil {
		log.Fatalf("Failed to load templates: %v", err)
	}

	// Set up routes
	mux := http.NewServeMux()

	// Set up push notification handlers on our mux (broadcast + VAPID reset are admin-only)
	pushService.SetupHandlers(mux, func(next http.Handler) http.Handler {
		return authMiddleware.RequireAuth(authMiddleware.RequireRole("admin")(next))
	})

	// Health check (used by the container health check; served even in setup mode)
	// 200 only while the database answers and its migrations are clean and
	// at the latest this build ships.
	latestMigration, err := database.LatestMigration(jimdottennis.Migrations())
	if err != nil {
		log.Fatalf("Cannot start: %v", err)
	}
	mux.HandleFunc(setup.HealthPath, health.Handler(db, latestMigration))
	// The version number alone, public (commit and date are CLI-only).
	mux.HandleFunc(setup.VersionPath, health.VersionHandler(version.Short()))

	// First-run setup (the rest of the app is gated behind it until a home club is in effect)
	setupHandler := setup.NewHandler(setupService, authService, templateDir)
	setupHandler.RegisterRoutes(mux)

	// Passkeys: bound to the current public URL on every request (it can be
	// changed on the settings page); changes go through the account audit seam.
	passkeys := auth.NewPasskeys(authService, homeClub.ClubSettings(), func() string { return homeClub.Branding().Name })
	passkeyAudit := accounts.New(db)
	passkeys.SetAudit(func(ctx context.Context, actor *models.User, action, detail string) {
		if err := passkeyAudit.RecordEvent(ctx, accounts.UserActor(actor.ID, actor.Username), action, detail); err != nil {
			log.Printf("Failed to record %s: %v", action, err)
		}
	})
	// Administrators' sign-ins (any method) go to the account audit log too.
	authService.SetSignInAudit(func(ctx context.Context, user *models.User, method, address string) {
		if err := passkeyAudit.RecordSignIn(ctx, user.ID, user.Username, method, address); err != nil {
			log.Printf("Failed to record a sign-in: %v", err)
		}
	})
	auth.NewPasskeyHandler(passkeys, templateDir, "/admin/league").RegisterRoutes(mux, authMiddleware)
	authHandler.SetPasskeys(passkeys)
	adminHandler.SetPasskeysAvailable(passkeys.Available)
	setupHandler.OfferPasskeys(passkeys.Available)

	// Auth routes
	authHandler.RegisterRoutes(mux)
	// A user's own password (forms on the sign-in methods page)
	accounts.NewPasswordPage(accounts.New(db), authService, passkeys).RegisterRoutes(mux)
	// Invite and recovery links (no session needed: the link is the authorisation)
	accounts.NewLinkPage(accounts.New(db), authService, passkeys, templateDir, "/admin/league").RegisterRoutes(mux)

	// Admin routes (protected)
	adminHandler.RegisterRoutes(mux, authMiddleware)

	// Public admin-related routes (e.g., club wrapped with lightweight password gate)
	adminHandler.RegisterPublicRoutes(mux)

	// Players routes (protected)
	playersHandler.RegisterRoutes(mux, authMiddleware)

	// Public player-facing routes (standings, etc.)
	playersHandler.RegisterPublicRoutes(mux)

	// Club logo, web app manifests (/static/manifest.json, /api/manifest?start=
	// for per-player PWA install) and the service worker, using the stored
	// club name, logo and colours
	branding.NewHandler(homeClub.ClubSettings(), files.static).RegisterRoutes(mux)

	// Fixture generator (public: reads the league tables, writes nothing)
	fixturegen.New(templates).RegisterRoutes(mux)

	// About page (public, no auth required)
	mux.HandleFunc("/about", func(w http.ResponseWriter, r *http.Request) {
		if err := render.Execute(w, templates, "about.html", nil); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	// Public routes
	tournamentRepo := repository.NewTournamentRepository(db)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			log.Printf("Not found: %s", auth.RedactPath(r.URL.Path))
			http.NotFound(w, r)
			return
		}

		// With tournaments off the home page is the club's own page and
		// nothing links to the tournament software
		if !homeClub.ClubSettings().TournamentsEnabled() {
			if err := render.Execute(w, templates, "club_home.html", map[string]interface{}{"Branding": homeClub.Branding()}); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		var visibleTournaments []models.Tournament
		if tournaments, err := tournamentRepo.FindVisible(r.Context()); err == nil {
			visibleTournaments = tournaments
		}

		data := map[string]interface{}{
			"Tournaments": visibleTournaments,
		}
		if err := render.Execute(w, templates, "index.html", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	// Serve static files (the service worker and manifest are served by the branding handler)
	mux.Handle("/static/", files.staticHandler)

	// Start server with proper timeouts
	port := getPort()
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      auth.NoFraming(auth.CrossOriginGuard(setupState.Gate(setupService.DemoBanner(config.HomeClubMiddleware(homeClub, mux))))),
		ReadTimeout:  30 * time.Second,  // Generous for mobile
		WriteTimeout: 30 * time.Second,  // Generous for mobile
		IdleTimeout:  120 * time.Second, // Keep connections alive
	}
	log.Printf("jim-dot-tennis %s ready: %s, database %s, %s mode",
		version.Long(), "http://localhost"+server.Addr, getEnv("DB_PATH", "./tennis.db"), runMode())
	if err := serve(server, authService.StartCleanup); err != nil {
		db.Close()
		log.Fatalf("Server stopped: %v", err)
	}
	// Normal shutdown: the deferred db.Close() runs and the process exits 0.
	log.Printf("Stopped cleanly")
}

// migrateOrStop applies the migrations and stops the server (through fatalf,
// log.Fatalf in the server) if that fails. A failed migration used to be only
// a warning, from the first commit on and for no recorded reason, and the app
// then ran against a schema older than the code. Sign-in, sessions and the
// audit log all depend on their tables being as the code expects, so that is
// not a state to serve requests in. (The command line never migrates; it
// checks the schema version itself and refuses, see cli.go.)
func migrateOrStop(db *database.DB, migrations fs.FS, label string, fatalf func(format string, args ...any)) {
	if err := db.ExecuteMigrationsFS(migrations, label); err != nil {
		var me *database.MigrationError
		if errors.As(err, &me) {
			// The message names the migration and lists the recovery steps.
			fatalf("Cannot start: %v", err)
			return
		}
		fatalf("Database migration failed, so the app cannot start: %v\n"+
			"Nothing was served. Migrations read from: %s. "+
			"See \"Working with Migrations\" in CLAUDE.md for a migration that stopped part-way.", err, label)
	}
}

// shutdownTimeout is how long requests in flight get to finish after
// SIGTERM/SIGINT. It is shorter than the 10 seconds `docker stop` waits before
// killing the container.
const shutdownTimeout = 8 * time.Second

// serve runs the server and the scheduled cleanup (expired sessions, old
// login attempts, finished links: auth.Service.Cleanup) until SIGINT or
// SIGTERM, then stops both: the cleanup first, then the server, letting
// requests in flight finish. It returns nil after a signal-driven shutdown.
func serve(server *http.Server, startCleanup func(ctx context.Context, firstDelay, interval time.Duration) func()) error {
	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	stopCleanup := startCleanup(ctx, auth.CleanupFirstDelay, auth.CleanupInterval)
	ln, err := net.Listen("tcp", server.Addr)
	if err != nil {
		stopCleanup()
		return fmt.Errorf("cannot listen on %s: %w (is another program using the port? set PORT to change it)", server.Addr, err)
	}
	return runServer(ctx, server, ln, stopCleanup, shutdownTimeout)
}

// runServer serves on ln until ctx is cancelled, then stops the cleanup and
// shuts the server down, waiting up to timeout for requests in flight. A
// clean, signal-driven stop returns nil.
func runServer(ctx context.Context, server *http.Server, ln net.Listener, stopCleanup func(), timeout time.Duration) error {
	failed := make(chan error, 1)
	go func() { failed <- server.Serve(ln) }()
	select {
	case err := <-failed:
		stopCleanup()
		return err
	case <-ctx.Done():
	}

	log.Printf("Shutting down (waiting up to %s for requests in progress)", timeout)
	stopCleanup()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("some requests did not finish in time: %w", err)
	}
	return nil
}

// getProjectRoot returns the project root directory
func getProjectRoot() (string, error) {
	// Get the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// If we're running from cmd/jim-dot-tennis, we need to go up two levels
	if filepath.Base(cwd) == "jim-dot-tennis" && filepath.Base(filepath.Dir(cwd)) == "cmd" {
		return filepath.Dir(filepath.Dir(cwd)), nil
	}

	// If we're running from the project root, return the current directory
	return cwd, nil
}

// loadTemplatesWithFuncs loads templates with function map
func loadTemplatesWithFuncs(templateDir string, funcMap template.FuncMap) (*template.Template, error) {
	// Create a new template with the function map (plus the role helpers the
	// admin dashboard, which is among templates/*.html, uses)
	tmpl := template.New("").Funcs(admin.NavTemplateFuncs()).Funcs(funcMap)

	// Parse all HTML files in the template directory
	pattern := filepath.Join(templateDir, "*.html")
	templates, err := render.ParseGlob(tmpl, pattern)
	if err != nil {
		return nil, err
	}

	return templates, nil
}

// setupDatabase initializes the database connection
func setupDatabase() (*database.DB, error) {
	// SQLite is the only supported database.
	dbType := getEnv("DB_TYPE", database.DriverSQLite)
	if err := database.RequireSQLite(dbType); err != nil {
		return nil, err
	}

	config := database.Config{
		Driver:   database.DriverSQLite,
		FilePath: getEnv("DB_PATH", "./tennis.db"),
	}

	return database.New(config)
}

// getPort gets the port from the environment variable or uses the default
func getPort() string {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default port
	}
	return port
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
