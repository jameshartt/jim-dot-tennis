package admin

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/models"
)

// getUserFromContext is a helper to get the user from request context
func getUserFromContext(r *http.Request) (*models.User, error) {
	user, err := auth.GetUserFromContext(r.Context())
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// parseTemplate loads and parses a template file
func parseTemplate(templateDir, templatePath string) (*template.Template, error) {
	fullPath := filepath.Join(templateDir, templatePath)
	return template.ParseFiles(fullPath)
}

// renderTemplate executes a template with given data
func renderTemplate(w http.ResponseWriter, tmpl *template.Template, data interface{}) error {
	return tmpl.Execute(w, data)
}

// parseIDFromPath extracts an ID from a URL path
// e.g., "/admin/teams/123" -> "123"
func parseIDFromPath(path, prefix string) (uint, error) {
	pathParts := strings.Split(strings.TrimPrefix(path, prefix), "/")
	if len(pathParts) < 1 || pathParts[0] == "" {
		return 0, ErrInvalidID
	}

	id, err := strconv.ParseUint(pathParts[0], 10, 32)
	if err != nil {
		return 0, ErrInvalidID
	}

	return uint(id), nil
}

// renderFallbackHTML renders a simple HTML fallback when templates fail
func renderFallbackHTML(w http.ResponseWriter, title, heading, message, backLink string) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`
	<!DOCTYPE html>
	<html>
	<head><title>` + title + `</title></head>
	<body>
		<h1>` + heading + `</h1>
		<p>` + message + `</p>
		<a href="` + backLink + `">` + backLink + `</a>
	</body>
	</html>
	`))
}

// logAndError logs an error and sends HTTP error response
func logAndError(w http.ResponseWriter, message string, err error, statusCode int) {
	log.Printf("%s: %v", message, err)
	http.Error(w, message, statusCode)
}

// Custom errors
type AdminError string

func (e AdminError) Error() string {
	return string(e)
}

const (
	ErrInvalidID = AdminError("invalid ID")
)
