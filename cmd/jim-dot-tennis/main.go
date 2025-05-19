package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/webpush"
)

func main() {
	// Initialize database
	db, err := setupDatabase()
	if err != nil {
		log.Fatalf("Failed to set up database: %v", err)
	}
	defer db.Close()
	
	// Execute migrations if needed
	if err := db.ExecuteMigrations("./migrations"); err != nil {
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
	
	// Set up push notification handlers
	pushService.SetupHandlers()

	// Serve static files with special handling for service worker
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Static file request: %s", r.URL.Path)
		
		// Add Service-Worker-Allowed header for service worker file
		if r.URL.Path == "/static/service-worker.js" {
			log.Printf("Service worker request detected, setting Service-Worker-Allowed header")
			w.Header().Set("Service-Worker-Allowed", "/")
			// Log all headers being sent
			log.Printf("Response headers for service worker:")
			for k, v := range w.Header() {
				log.Printf("  %s: %v", k, v)
			}
		}
		
		// Log the request headers
		log.Printf("Request headers:")
		for k, v := range r.Header {
			log.Printf("  %s: %v", k, v)
		}
		
		http.StripPrefix("/static/", fs).ServeHTTP(w, r)
	}))

	// Serve index.html template
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		
		tmpl, err := template.ParseFiles(filepath.Join("templates", "index.html"))
		if err != nil {
			http.Error(w, "Template error", http.StatusInternalServerError)
			return
		}
		err = tmpl.Execute(w, nil)
		if err != nil {
			http.Error(w, "Template execution error", http.StatusInternalServerError)
		}
	})

	port := getPort()
	log.Printf("Server started at http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
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
