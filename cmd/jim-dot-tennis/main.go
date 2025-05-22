package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/webpush"
)

func main() {
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

	// Initialize web push service
	pushService := webpush.New(db)

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

	// Set up auth service
	authConfig := auth.DefaultConfig()
	// In development, we can allow insecure cookies
	if os.Getenv("APP_ENV") != "production" {
		authConfig.CookieSecure = false
	}
	authService := auth.NewService(db, authConfig)

	// Set up auth middleware
	authMiddleware := auth.NewMiddleware(authService)

	// Set up auth handlers
	templateDir := filepath.Join(projectRoot, "templates")
	authHandler := auth.NewHandler(authService, templateDir, "/admin")

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

	// Set up push notification handlers
	pushService.SetupHandlers()

	// Set up routes
	mux := http.NewServeMux()

	// Auth routes
	authHandler.RegisterRoutes(mux)

	// Public routes
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if err := templates.ExecuteTemplate(w, "index.html", nil); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	// Admin routes (protected)
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Admin handler called with path: %s", r.URL.Path)

		// Modify this to handle both "/admin" and "/admin/" paths
		if r.URL.Path != "/admin/" && r.URL.Path != "/admin" {
			log.Printf("Not found for path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}

		// Get user from context
		user, err := auth.GetUserFromContext(r.Context())
		if err != nil {
			log.Printf("Failed to get user from context: %v", err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		log.Printf("Admin page requested by user: %s (role: %s)", user.Username, user.Role)

		// Mock stats for the admin page
		mockStats := map[string]int{
			"PlayerCount":  12,
			"FixtureCount": 8,
			"SessionCount": 1,
		}

		// Load the standalone admin template directly (bypass layout)
		adminTemplatePath := filepath.Join(projectRoot, "templates", "admin_standalone.html")
		tmpl, err := template.ParseFiles(adminTemplatePath)
		if err != nil {
			log.Printf("Error parsing standalone admin template: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Execute the standalone template
		if err := tmpl.Execute(w, map[string]interface{}{
			"User":  user,
			"Stats": mockStats,
		}); err != nil {
			log.Printf("Error executing standalone admin template: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	// Fix the admin path handling
	mux.Handle("/admin", authMiddleware.RequireAuth(
		authMiddleware.RequireRole("admin")(adminMux),
	))
	mux.Handle("/admin/", authMiddleware.RequireAuth(
		authMiddleware.RequireRole("admin")(adminMux),
	))

	// Serve static files with special handling for service worker
	staticDir := filepath.Join(projectRoot, "static")
	fs := http.FileServer(http.Dir(staticDir))
	mux.Handle("/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Static file request: %s", r.URL.Path)

		// Add Service-Worker-Allowed header for service worker file
		if r.URL.Path == "/static/service-worker.js" {
			log.Printf("Service worker request detected, setting Service-Worker-Allowed header")
			w.Header().Set("Service-Worker-Allowed", "/")
		}

		http.StripPrefix("/static/", fs).ServeHTTP(w, r)
	}))

	// Start server
	port := getPort()
	log.Printf("Server started at http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
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
	// Get database config from environment variables with defaults
	dbType := getEnv("DB_TYPE", "sqlite3")

	config := database.Config{
		Driver: dbType,
	}

	if dbType == "postgres" {
		config.Host = getEnv("DB_HOST", "localhost")
		config.Port, _ = strconv.Atoi(getEnv("DB_PORT", "5432"))
		config.User = getEnv("DB_USER", "postgres")
		config.Password = getEnv("DB_PASSWORD", "postgres")
		config.DBName = getEnv("DB_NAME", "tennis")
		config.SSLMode = getEnv("DB_SSLMODE", "disable")
	} else {
		// SQLite
		config.FilePath = getEnv("DB_PATH", "./tennis.db")
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
