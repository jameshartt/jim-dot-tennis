// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"jim-dot-tennis/internal/accounts"
	"jim-dot-tennis/internal/admin"
	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/bhplta"
	"jim-dot-tennis/internal/branding"
	"jim-dot-tennis/internal/config"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/fixturegen"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/players"
	"jim-dot-tennis/internal/render"
	"jim-dot-tennis/internal/repository"
	"jim-dot-tennis/internal/secrets"
	"jim-dot-tennis/internal/setup"
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
	log.Printf("Using project root: %s", projectRoot)

	// Initialize database
	db, err := setupDatabase()
	if err != nil {
		log.Fatalf("Failed to set up database: %v", err)
	}
	defer db.Close()

	// Execute migrations
	migrationsPath := filepath.Join(projectRoot, "migrations")
	if err := db.ExecuteMigrations(migrationsPath); err != nil {
		log.Printf("Warning: Failed to run migrations: %v", err)
	}

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

	// List existing VAPID keys
	if err := pushService.ListVAPIDKeys(); err != nil {
		log.Printf("Warning: Failed to list VAPID keys: %v", err)
	}

	// Generate VAPID keys on startup if none exist
	publicKey, _, err := pushService.GenerateVAPIDKeys()
	if err != nil {
		log.Printf("Warning: Failed to generate VAPID keys: %v", err)
	} else {
		log.Printf("VAPID public key: %s", publicKey)
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
	templateDir := filepath.Join(projectRoot, "templates")
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
	mux.HandleFunc(setup.HealthPath, func(w http.ResponseWriter, r *http.Request) {
		if err := db.PingContext(r.Context()); err != nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintln(w, "ok")
	})

	// First-run setup (the rest of the app is gated behind it until a home club is in effect)
	setup.NewHandler(setupService, authService, templateDir).RegisterRoutes(mux)

	// Auth routes
	authHandler.RegisterRoutes(mux)
	accounts.NewPasswordPage(accounts.New(db), authService, templateDir, "/admin/league").RegisterRoutes(mux)

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
	staticDir := filepath.Join(projectRoot, "static")
	branding.NewHandler(homeClub.ClubSettings(), staticDir).RegisterRoutes(mux)

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
			log.Printf("Not found: %s", r.URL.Path)
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
	fs := http.FileServer(http.Dir(staticDir))
	mux.Handle("/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Static file request: %s", r.URL.Path)
		http.StripPrefix("/static/", fs).ServeHTTP(w, r)
	}))

	// Start server with proper timeouts
	port := getPort()
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      auth.CrossOriginGuard(setupState.Gate(setupService.DemoBanner(config.HomeClubMiddleware(homeClub, mux)))),
		ReadTimeout:  30 * time.Second,  // Generous for mobile
		WriteTimeout: 30 * time.Second,  // Generous for mobile
		IdleTimeout:  120 * time.Second, // Keep connections alive
	}
	log.Printf("Server started at http://localhost:%s", port)
	log.Fatal(server.ListenAndServe())
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
	log.Printf("Loading templates from: %s", templateDir)

	// Create a new template with the function map
	tmpl := template.New("").Funcs(funcMap)

	// Parse all HTML files in the template directory
	pattern := filepath.Join(templateDir, "*.html")
	templates, err := tmpl.ParseGlob(pattern)
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
