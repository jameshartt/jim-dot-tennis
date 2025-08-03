package admin

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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

// parseTemplate loads and parses a template file with helper functions and partials
func parseTemplate(templateDir, templatePath string) (*template.Template, error) {
	fullPath := filepath.Join(templateDir, templatePath)

	// Define template functions
	funcMap := template.FuncMap{
		"lower": func(v interface{}) string {
			switch s := v.(type) {
			case string:
				return strings.ToLower(s)
			case models.AvailabilityStatus:
				return strings.ToLower(string(s))
			default:
				return strings.ToLower(fmt.Sprintf("%v", s))
			}
		},
		"currentYear": func() int {
			return time.Now().Year()
		},
		"add": func(a, b int) int {
			return a + b
		},
		"formatPoints": func(points float64) string {
			// If it's a whole number, show as integer
			if points == float64(int(points)) {
				return fmt.Sprintf("%.0f", points)
			}
			// Otherwise show one decimal place
			return fmt.Sprintf("%.1f", points)
		},
		"until": func(n int) []int {
			result := make([]int, n)
			for i := 0; i < n; i++ {
				result[i] = i
			}
			return result
		},
		"lt": func(a, b int) bool {
			return a < b
		},
		"le": func(a, b int) bool {
			return a <= b
		},
		"ge": func(a, b int) bool {
			return a >= b
		},
	}

	// Parse template with function map
	tmpl := template.New(filepath.Base(templatePath)).Funcs(funcMap)

	// Parse the main template first
	tmpl, err := tmpl.ParseFiles(fullPath)
	if err != nil {
		return nil, err
	}

	// Parse any partials in the admin/partials directory with full path names
	partialsPattern := filepath.Join(templateDir, "admin", "partials", "*.html")
	partialFiles, err := filepath.Glob(partialsPattern)
	if err != nil {
		// If glob fails, continue without partials (not critical)
		log.Printf("Warning: could not find partials at %s: %v", partialsPattern, err)
	} else if len(partialFiles) > 0 {
		// Parse each partial with its desired template name (preserving directory structure)
		for _, partialFile := range partialFiles {
			// Read the file content
			content, err := os.ReadFile(partialFile)
			if err != nil {
				return nil, fmt.Errorf("error reading partial %s: %v", partialFile, err)
			}

			// Get the relative path from templateDir to create the template name
			relPath, err := filepath.Rel(templateDir, partialFile)
			if err != nil {
				return nil, fmt.Errorf("error getting relative path for %s: %v", partialFile, err)
			}

			// Convert path separators to forward slashes for template names (cross-platform)
			templateName := filepath.ToSlash(relPath)

			// Parse the content with the full path as template name
			_, err = tmpl.New(templateName).Parse(string(content))
			if err != nil {
				return nil, fmt.Errorf("error parsing partial %s as %s: %v", partialFile, templateName, err)
			}
		}
	}

	return tmpl, nil
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
